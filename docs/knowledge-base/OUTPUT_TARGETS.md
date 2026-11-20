# Output Targets

**Status:** Coordinator and local-storage target implemented; isolated Windows GDI printer target and durable intent/recovery wrapper implemented and tested against a DS-RX1 virtual queue. Physical printer acceptance and guest-session integration remain open.

## Intent

The finalized photo produced by the renderer is one immutable input to the output stage. Storage and printing are independent targets: a failure in one must not prevent the other from trying. Output targets are also separate from guest media delivery through LAN or cloud.

## Current implementation

The Go package is internal/booth/output.

- `Job` identifies the session and read-only rendered source file.
- `Target` exposes a stable name and an `Execute` operation for one destination.
- `Coordinator` validates the common job, rejects duplicate target names, limits configuration to four targets, starts target operations concurrently, and returns per-target results in configuration order.
- The `LocalProvider` implements the target interface and persists the finalized image in a session-scoped folder using a same-directory temporary file, sync/close and final rename.
- Disk-full/quota errors are classified as `output.ErrInsufficientStorage` on Windows (`ERROR_DISK_FULL`/`ERROR_HANDLE_DISK_FULL`) and non-Windows (`ENOSPC`/`EDQUOT`). Partial temp output is removed; captures and `rendered.jpg` are retained. See [LOCAL_STORAGE_RECOVERY.md](LOCAL_STORAGE_RECOVERY.md).
- Target results include status, elapsed duration, human-readable error text, an in-process typed cause (excluded from JSON), optional local output path, and optional external job ID.
- The workflow records all target results and the local output path if local storage succeeded.
- WindowsPrinter uses GDI/Winspool to submit the finalized JPEG through the installed driver; it is an isolated Windows target.
- DurablePrinter persists a manifest-v3 intent before submission and the queued/job ID afterward; known pre-acceptance failures are failed, ambiguous outcomes are uncertain and block resubmission.

The default CLI and Wails guest workflow still configure only the local-storage target. The Windows GDI target has been exercised through a separate synthetic smoke tool against the DS-RX1 virtual queue. This is software/virtual-port evidence only, not physical printer integration.

## Status and lifecycle semantics

- `succeeded`: the target completed its synchronous operation. For local storage this means the file was saved.
- `queued`: the Windows printer target completed EndDoc and Windows returned a spooler job ID. This means the spooler accepted the job, not that paper has physically emerged. DurablePrinter persists the attempt/job ID before returning.
- `failed`, `cancelled`, `timed_out`: the individual target did not complete its operation.

Session outcomes:
- `completed`: every configured output target completed or accepted its job.
- `partial_success`: at least one target completed/queued, while another target failed, was cancelled, or timed out.
- `failed`: no output target completed or accepted its job.

No successful target is rolled back if a sibling target fails. The coordinator does not silently retry printing; duplicate-print policy must be designed with the actual printer/provider because retrying a spool operation can create duplicate paper output. For a failed local save, `go run ./apps/output-retry --config <station-config> --session-id <id>` copies the preserved `rendered.jpg` without reopening the camera; it refuses to overwrite an existing `photo.jpg`. This recovery path is separate from printer retry.

## Performance and failure behavior

- A maximum of four configured targets bounds concurrent work; the common local-plus-printer use case launches both independently.
- Target results carry per-target durations; the coordinator duration is recorded by the workflow.
- Target implementations must honor the supplied context where possible. Native printer APIs should avoid blocking the capture/UI thread.
- A printer adapter should normally enqueue/spool the job and return a job ID plus `queued` status, rather than holding the capture session open until physical printing finishes.
- The UI event callback runs on the session goroutine. UI adapters must enqueue events and return quickly.
- Target errors are retained in session results and exposed through events. Avoid logging photo contents, credentials, or sensitive printer/network paths unnecessarily.

## Remaining work

1. Select/obtain a physical printer model and media kit; the DS-RX1 virtual queue does not close this gate.
2. Run physical acceptance for layout/crop, dimensions, color, margins, cut, repeatability, paper-out/cover-open, disconnect/offline behavior and recovery.
3. Only after physical acceptance, decide whether to wire DurablePrinter into the approved Windows Preview guest workflow. Keep printer configuration disabled by default and do not auto-retry. The Windows version-1 lock reconciliation path is implemented; pre-lease/legacy/corrupt/partial locks remain inspect-only.
4. Evaluate native Ubuntu/Linux printer driver support on a native Ubuntu host separately.
5. Keep LAN/cloud photo delivery as a separate module; it must not become another name for local saving or printer output.


## Hardware inventory update (2026-11-01)

ShinPc currently exposes only Microsoft Print to PDF and Microsoft OneNote virtual printer queues. No physical printer model/driver is installed or available for validation. Do not describe this as physical print support. Local JPEG storage/export remains the tested output path. Select a named printer, connection and Windows driver target before implementing or claiming a physical printer adapter.


## Hardware inventory revalidation and selection research (2026-11-18)

A new read-only inventory of Windows printer queues, PnP Printer-class devices and Win32_Printer on ShinPc again found only Microsoft Print to PDF and two Microsoft OneNote virtual queues. No physical printer was attached/installed and no print job was submitted.

A candidate comparison and proposed first acceptance path are now documented in [PRINTER_SELECTION_AND_ACCEPTANCE.md](PRINTER_SELECTION_AND_ACCEPTANCE.md). HiTi P525L is the preferred first evaluation candidate (Proposed), primarily for its observed lower Vietnam listing prices, booth-oriented formats and listed Linux source-driver package. DNP DS620A is the faster Windows-oriented alternative. Neither model is selected or accepted. Confirm Windows 11 driver, consumables, local service and exact media kit before purchase; Linux support must be validated on Ubuntu 26.04.1, not inferred from a download listing.

**Gate remains blocked:** the operator must select and supply/borrow a physical model. Do not implement a model-specific adapter or claim print support before the physical acceptance path passes.


## DS-RX1 Windows virtual queue and GDI target spike (2026-11-19)

A DNP DS-RX1 x64 printer driver/queue is now installed on ShinPc. This is a **virtual queue**, not a connected physical DS-RX1 printer: it points to the local Multi File Port Monitor named Photobooth-port rather than a USB/printer hardware port. The monitor writes DNP raster spool data to the configured local Printer folder and invokes a local converter that decodes it to JPEG. The converter script was inspected before submitting any job; the synthetic smoke used no guest images or network delivery.

Implemented an isolated Windows GDI target:
- internal/booth/output/printer.go, printer_windows.go and printer_other.go expose NewWindowsPrinter.
- It decodes the finalized JPEG, prepares a 32-bit BGRA DIB, obtains a driver DEVMODE with DocumentPropertiesW, applies orientation to that job only, and submits through CreateDCW / StartDocW / StartPage / StretchDIBits / EndPage / EndDoc.
- The target returns queued with the Windows job ID only after EndDoc succeeds. It does not claim physical print completion.
- FlipVertical is an explicit option. The tested virtual converter required Landscape=true and FlipVertical=true; do not generalize this transform to a physical printer without a physical test.
- The adapter uses the queue's current default form/media configuration. It does not yet expose paper/form selection and is not wired into StoreConfig, delivery.NewDispatcher or the Wails Settings UI.

### Synthetic validation evidence

- A four-variant System.Drawing/GDI+ test identified the correct orientation for the virtual converter: landscape page plus vertical-only source flip. Output JPEG was 1200x1800 and visually reviewed; color patch samples were stable but are not a calibrated color proof.
- The Go target was exercised against DS-RX1 virtual queue. One GDI job returned spooler job ID 6 and generated a 1200x1800 JPEG.
- A 10-job sequential run directly through the printer target passed 10/10 spooler submissions and 10/10 virtual JPEG conversions; all converted images were 1200x1800 with the same SHA-256. Queue returned to Normal / JobCount 0.
- The smoke was then upgraded to use output.NewCoordinator(output.NewLocal(...), WindowsPrinter). A 10-job coordinator run passed 10/10 local saves (each byte-identical to the source fixture), 10/10 queued printer results with job IDs 18-27, and 10/10 converted JPEGs (1200x1800, same SHA-256). Final queue state was Normal / JobCount 0. Report: %TEMP%\\photobooth-dsrx1-coordinator-10copy-20261119.json.
- All jobs used a generated synthetic color/alignment fixture. No physical paper output occurred; physical_print_confirmed=false.

### Integration boundary

The GDI target is available to the Wails Windows Preview only through an opt-in StoreConfig v2 profile. The operator must explicitly enable printer.enabled and supply the selected Windows queue, exact driver media form and per-queue orientation/flip settings. DurablePrinter/manifest v3 adds the audit and kernel-lock recovery boundary, and the recovery card supports queue inspection plus operator resolution. A queued job or a missing queue entry is not proof of physical output. StartDocW/EndDoc are synchronous and cannot be forcibly interrupted while a driver call is blocked. This remains prototype software integration, not physical-device certification.

Physical printer model/media acceptance, color calibration, paper feed/cut, paper-out, disconnection and native Ubuntu printer testing remain OPEN. A virtual DS-RX1 queue does not close those hardware gates.


## Durable printer intent and recovery (2026-11-19)

The Windows printer target has an isolated durable wrapper in internal/booth/output/durable_printer.go. The root CLI remains local-only. The Wails Windows Preview now wires an opt-in local-plus-durable-printer coordinator when operator-managed StoreConfig sets printer.enabled=true; demo sessions and local-output retries remain local-only. Physical acceptance is still pending.

- Session manifest v3 adds an append-only PrintAttempts list (maximum 8). Manifest v1 and v2 remain readable; v2 can be upgraded to v3 on first explicit durable print submission when its immutable frame snapshot is verified. Legacy v1 sessions are not upgraded automatically.
- Each attempt records a random attempt ID, queue name, deterministic document name (session ID + attempt ID), rendered-source SHA-256, requested/updated timestamps, status, spooler job ID and error.
- States: submitting (intent saved before calling driver), queued (Windows accepted and returned a job ID), failed (typed definite pre-acceptance failure), uncertain (the job may have reached the spooler or the result could not be persisted).
- Only the latest failed attempt permits a later explicit attempt. Queued, submitting and uncertain block resubmission. Unknown errors are classified uncertain.
- A per-session .print.lock prevents parallel submissions. New Windows submissions hold a version-1 kernel lease; after an abrupt process exit, an exclusive claim proves that the prior handle was released. Read-only discovery never removes the marker; a separate explicit orphan-reconciliation action handles eligible version-1 locks.
- Windows GDI document name is populated with the durable attempt document name so an operator can search it in the Windows queue.
- Wails recovery DTO displays queue/document/job/status using string-safe timestamps. An explicit Inspect Windows queue action performs a read-only EnumJobsW lookup using the exact persisted document/job identity. It has no print-retry or mark-completed action. Local-output recovery remains independent and never re-submits the printer target.

### Durable virtual smoke evidence

On 2026-11-19 a synthetic manifest-v3 session was dispatched through output.Coordinator with LocalProvider + DurablePrinter against the DS-RX1 virtual queue. Result: local save PASS with source-matching SHA-256; durable attempt persisted as queued with attempt ID 5d3abb11e1b8b90a4c6fa7cba7302040, document name PhotoBooth-e3b06bc2e3319b48f25b692647e3a1b2-5d3abb11e1b8b90a4c6fa7cba7302040, spooler job ID 29; the virtual port generated a 1200x1800 JPEG. Queue returned Normal / JobCount 0.

The smoke tool then added exact read-only queue inspection. A second synthetic manifest-v3 session produced spooler job ID 30 and a 1200x1800 JPEG; immediately afterward EnumJobsW returned Found=false because the virtual converter had already removed the job from the queue. This was correctly treated as "not currently listed", not as printed/completed or failed. The smoke inspection completed without error; the new JPEG was removed after verification. The previously retained representative synthetic image and pre-existing files were left alone. No physical paper was produced.

Fault-injection/unit coverage verifies: durable intent exists before the target is called; definite pre-acceptance error permits a later explicit attempt; unclassified error is uncertain and blocks retries; failure to persist a post-acceptance result leaves the on-disk attempt submitting; stale crash lock prevents another submission; schema-v2 snapshot sessions upgrade to v3 while preserving their prior data; recovery discovery reads attempt/lock metadata without mutation.

**Windows Preview opt-in only; not production-certified:** StoreConfig v2 controls the Windows Preview integration. Printer remains disabled by default and settings are not exposed as guest-facing controls; the operator must edit the local profile explicitly. When enabled, a real-camera session runs LocalProvider and DurablePrinter independently. The demo path and local-output retry cannot submit a printer job. Results distinguish local save from Windows spooler acceptance; queued never means physical paper. The Wails recovery panel supports exact queue inspection, printed/not_printed/unknown closure and kernel-verified orphaned-lock reconciliation. There is no automatic print retry or background queue polling. Physical acceptance, driver/media behavior, duplicate policy under real paper handling and production certification remain pending. See ADR 0033.


### Pre-manifest crash-lock identity

The exclusive .print.lock now stores a bounded JSON record with attempt ID, queue, deterministic document name and creation time before manifest intent publication. The recovery scanner validates and surfaces this data. If the process exits between lock creation and manifest update, the Wails recovery card can still run the exact read-only queue inspection using the lock identity. A corrupt lock stays uncertain but has no Inspect action. Test coverage includes lock-only recovery with no PrintAttempt entry.

A final smoke after this refinement passed through DurablePrinter/Coordinator against DS-RX1 virtual queue: job ID 31, attempt ID 983bdd1b9702439ed04666703969e909, manifest queued, local hash matched source, JPEG 1200x1800; EnumJobsW found no current queue entry after conversion. Queue returned Normal / JobCount 0; no physical paper was produced.


## Operator resolution for queued/uncertain printer attempts (2026-11-19)

Manifest v3 now stores append-only print_resolutions separately from print_attempts. The operator can explicitly record printed, not_printed or unknown after a fresh exact-identity Windows queue inspection.

- printed: matching queue job must be absent, and operator must affirm physical paper was observed. This closes the attempt and blocks any further attempt.
- not_printed: matching queue job must be absent, and operator must affirm the printer/output tray was checked and no paper emerged. Only this decision makes a future, separately invoked DurablePrinter.Execute eligible.
- unknown: can be recorded while the job remains listed or is absent. It permanently blocks retry and does not claim success/failure.

The Wails recovery panel exposes read-only Inspect Windows queue and explicit decisions only for persisted manifest attempts without an active .print.lock. It asks for confirmation and a required note. Resolve re-inspects the exact persisted queue/document/job immediately before saving, reloads the manifest and rechecks the lock. It records an append-only resolution and preserves the original PrintAttempt. It never mutates the Windows queue. No automatic or UI print retry was introduced; DurablePrinter remains disconnected from the standard guest dispatcher.

A lock-only crash with a version-1 Windows lease can now be reconciled after a kernel-exclusive claim proves the old handle is no longer held; the API creates one uncertain attempt if the manifest intent was never written. Pre-lease/unversioned, corrupt/partial and unsupported-platform locks remain inspect-only and are never removed based on PID or queue absence.

This is operator workflow groundwork, not physical printer acceptance. Queue absence remains distinct from physical output. Guest printing remains disabled/unwired.


## Windows printer lock owner verification and reconciliation (2026-11-19)

New durable printer submissions hold a version-1 kernel file lease on .print.lock from before manifest intent publication through result persistence. The lease metadata includes attempt ID, queue/document identity, rendered-source SHA-256 and owner PID; PID is informational, not proof of liveness.

Windows read-only recovery can inspect the metadata concurrently, then probes the kernel lease:
- active: another process still holds the lease; reconciliation is refused.
- orphaned: Windows allowed an exclusive claim, indicating the prior handle is no longer held. The recovery API claims again and retains exclusive ownership during the entire operation.
- unverifiable: legacy (lease version 0/omitted), corrupt/partial or unsupported-platform marker. It remains inspect-only.

Reconcile released printer lock verifies the rendered JPEG hash against the lock metadata, then either marks an existing submitting attempt uncertain, preserves an already persisted queued/uncertain/failed state, or creates exactly one uncertain attempt when the crash occurred before the manifest intent. A verified snapshot-backed v2 manifest is upgraded to v3 only when needed. It persists before deleting the marker through the held Windows handle, so a crash between the two operations can be retried idempotently.

This does not inspect or mutate the printer queue and does not retry printing. The operator must inspect the exact queue/document/job after reconciliation. Physical print outcome remains unknown until separately resolved. The workflow is Windows-only; non-Windows runtime claim/reconciliation is not implemented.


## Windows Preview opt-in printer integration and media-form smoke (2026-11-20)

The Windows GDI target is now wired into the Wails Windows Preview behind the operator-managed StoreConfig v2 `printer.enabled` flag. The default remains false. When explicitly enabled for a real-camera session, the Wails preview constructs `Coordinator(LocalProvider, DurablePrinter)`, so local save and printer submission are independent. Demo sessions always use LocalProvider only. The root CLI remains local-only. Local-output retries (normal and recovered) use the dedicated local-only publisher and cannot submit a printer job.

The output result DTO retains one row per target (name/status/job ID/error/duration). The result screen displays LOCAL + PRINTER when applicable and describes queued as Windows spooler acceptance only. It does not claim physical paper. If local save fails while Windows queues a job, the UI makes the independent outcomes clear and offers only the existing local-save recovery. Ambiguous printer submissions remain uncertain and are not retried automatically.

WindowsPrinterOptions now includes MediaForm. The adapter sets DEVMODEW.dmFormName for the current job only, validates the 31 UTF-16 code-unit limit and invokes DocumentPropertiesW to validate the modified DEVMODE. Vendor-private bytes after dmSize are preserved. StoreConfig validates the same length bound. The form name is driver-specific, not a friendly size alias.

Read-only DS-RX1 virtual driver inspection on ShinPc found exact forms including `(6x4)`, `(6x8)` and `(5x7)`. A single synthetic smoke using `media_form="(6x4)"` was accepted by the driver/spooler as job ID 32. Durable attempt was queued, local output hash matched the synthetic source, and the local converter created a 1200x1800 JPEG. Queue inspection after conversion returned Found=false; this only means the job was no longer listed. `physical_print_confirmed=false`. The new JPEG was verified and removed; all pre-existing output files were preserved. The virtual queue returned Normal with no jobs remaining.

Validation for this software step: config/output tests PASS; Wails dispatcher/demo/default/output-message tests PASS; DS-RX1 virtual media-form smoke PASS. Full root/Wails tests, vet, cross-builds and Wails production build are recorded in the 2026-11-20 handoff continuation.

**Still pending:** physical printer/model and media kit selection, paper/color/cut/repeatability/offline/failure/duplicate acceptance, and native Ubuntu printer-driver runtime evaluation. DS-RX1 virtual evidence does not close any of those gates. Keep `printer.enabled=false` in the default StoreConfig.
