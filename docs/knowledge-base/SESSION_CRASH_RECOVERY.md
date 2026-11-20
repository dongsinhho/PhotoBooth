# Local session crash/restart recovery

**Status:** Implemented in the root Go session/workflow packages and the Windows Preview Wails UI; Go tests and a dedicated synthetic-only Windows Wails UI Automation smoke pass. Physical-camera recovery and production/target-hardware validation remain pending.
**Manifest:** schema version 2 for new sessions; schema version 1 remains readable for legacy sessions
**Last updated:** 2026-11-07

## Scope

The Windows Preview can discover session workspaces left behind by an application/process interruption and offer explicit local recovery actions. Discovery is read-only. It does not open a camera, resume a session, rewrite a manifest, repair a directory, remove temporary files or delete session data.

This is not a database-backed transaction log and does not guarantee that every interruption can be resumed. Ambiguous, legacy or inconsistent data remains available for manual inspection only.

## Durable manifest

Each new workflow session writes `manifest.json` inside its session directory. The bounded JSON manifest is strict (unknown fields and trailing JSON documents are rejected) and records:

- Manifest version and session ID.
- Current phase: `preflight`, `capturing`, `rendered` or `output_finalized`.
- Expected and completed capture counts.
- Original absolute frame-config path, frame SHA-256 and a fingerprint of the capture-affecting run configuration.
- Schema-v2 `frame_snapshot_file` / `frame_snapshot_sha256` fields binding the immutable session-local `frame.json` copy. The snapshot is written, synced and published without replacement before the initial manifest is published.
- Config schema version, camera provider/device and capture dimensions, timing and demo-mode metadata.
- Created, updated, rendered and output-finalized timestamps; final output path after local publication.

Updates are written to a same-directory temporary file, synced and closed before atomic replacement. Windows uses `MoveFileExW(REPLACE_EXISTING | WRITE_THROUGH)`; non-Windows uses rename followed by a parent-directory sync.

Each camera attempt is isolated under `captures/.staging/shot-NN-<random>/`. The workflow fully decodes and syncs the staged JPEG, then publishes it to `capture-NN.jpg` with an atomic no-replace operation (Unix: hard-link publication, sync committed directory, staged-name removal and staging-directory sync; Windows: `MoveFileExW` without `REPLACE_EXISTING`, using `WRITE_THROUGH`). Only after publication does the workflow advance the completed-shot manifest count. A completed capture is never written directly into the committed capture namespace by the camera provider.

## Read-only discovery and classification

On Wails startup, the app inspects only the configured sessions root before the separate idle camera-preview lifecycle starts. Discovery itself never opens a camera or starts a session. Once discovery returns, the normal Windows Preview may open its idle-only camera preview; capture/recovery serializes against and stops that preview before opening the workflow camera provider. Discovery:

- Rejects a sessions root that is not a real local directory or resolves through a symlink/junction.
- Scans at most 4,096 root entries; an oversized root aborts discovery instead of creating an unbounded scan.
- Considers only 32-character lowercase hexadecimal session-directory names.
- Requires each session to be a real direct child and permits only `manifest.json`, `captures/`, `frame.json` (schema-v2 sessions) and `rendered.jpg` in its top level.
- Bounds the session directory to four entries and the captures directory to twelve committed captures plus one optional `.staging/` directory. The staging tree is separately bounded to twelve attempt directories and three files per attempt.
- Validates sequential committed capture names, bounded regular JPEG files, readable JPEG headers, manifest identity and phase/count consistency. A capturing-phase manifest may lag exactly one committed capture after an interrupted post-publication metadata write; larger or otherwise inconsistent gaps remain inspect-only. For schema-v2 sessions, discovery checks the immutable frame snapshot digest and the Wails layer recomputes the current capture-affecting fingerprint using that snapshot; schema-v1 sessions still require the original frame file and digest.
- Capture providers write to a unique `.staging/shot-NN-<random>/capture.jpg` path. Discovery validates known staging names read-only but never removes them. Full JPEG payload decoding is deferred until the operator chooses Continue, where every committed capture is decoded before the camera opens. Output-only retry fully decodes the preserved `rendered.jpg` before publishing it; startup does not decode every full-resolution guest photo.
- Classifies missing/corrupt/unsupported manifests, redirected paths, unknown entries, mismatched counts and invalid images as inspect-only.

A rendered session is separately classified as output-pending. Finalized sessions are omitted from recovery prompts only when the recorded final output path is present and valid. If the matching output directory contains an unexpected file, a temporary file, an invalid image or an ambiguous publication state, the session is inspect-only.

## Operator actions

The idle/home view displays interrupted sessions and offers actions according to their verified state:

- **Continue session:** explicit; re-scans before use and the workflow re-reads the manifest, checks the capture-affecting configuration fingerprint and fully decodes every committed JPEG before opening the camera. Schema-v2 sessions render from their verified `frame.json` snapshot, independent of later changes/removal of the active frame file. If the only Settings warning is that the current frame cannot be loaded or its slot count no longer matches, a verified schema-v2 snapshot may still be continued; other settings errors remain blocking. Schema-v1 sessions keep the legacy original-frame requirement. It removes only a validated interrupted `.staging` tree, then captures missing shots. If a single complete capture was published before its manifest count update, Continue reconciles that count after full decode. If every shot already exists, it skips camera creation and proceeds to rendering.
- **Retry local save:** explicit; only for a fully captured, rendered session with no ambiguous final output. It stops and joins the idle-preview worker before claiming the output operation, fully decodes the preserved `rendered.jpg`, and republishes that file without camera capture or rendering. Output publication is exclusive/no-overwrite. If preview cleanup or JPEG validation fails, no output is published.
- **Start a new session:** explicit confirmation; allocates a new session ID and leaves the interrupted folder unchanged.
- **Inspect / export files:** opens the selected session directory in Explorer on Windows.
- **Hide for now:** hides the card for the current app launch only; it does not alter files.

There is no automatic startup resume, recapture or cleanup. Only an explicit Continue action removes a validated uncommitted `.staging` tree; committed captures, rendered photos, final outputs and whole session folders are never removed by recovery. Existing opt-in retention still requires its separate reviewed-plan workflow.

## Crash boundaries and conservative behavior

- Crash before the first manifest is published: a session folder without a valid manifest (including a snapshot copied but not yet declared) is inspect-only.
- Crash during a camera write: the partial file remains inside a validated staging attempt, never under `capture-NN.jpg`. Discovery stays read-only and may still offer Continue; only after the operator chooses Continue does the workflow remove the validated staging tree and retake that missing shot.
- Crash after atomic capture publication but before its completed-shot count is persisted: exactly one committed JPEG may be ahead of a `capturing` manifest. Discovery offers Continue without mutation; the workflow fully decodes the committed set, reconciles the manifest count, then captures any remaining shots. A gap larger than one, a non-capturing phase or an invalid JPEG remains inspect-only.
- Crash after all captures but before render publication: a valid capturing-phase session may continue and render from preserved captures.
- Crash after render publication and rendered-phase manifest update but before local output completes: output-pending may retry only the local save.
- Crash after local output publication but before output-finalized manifest update: existing output makes the state ambiguous and prevents overwrite; inspect-only is safer than attempting a duplicate publication.
- Crash after finalized manifest and matching output exist: no recovery prompt is shown.

The final numbered capture namespace contains only JPEGs that passed full validation and atomic no-replace publication. Validated staging artifacts are cleaned only during an explicit Continue action. This staging implementation narrows crash ambiguity but does not eliminate the documented filesystem TOCTOU window or certify every filesystem/provider combination.

## Validation

Windows-host synthetic/temp-directory tests cover strict and atomic manifest read/write, phase/timestamp validation, bounded directory enumeration, read-only discovery of known/unknown staging artifacts, legacy/missing manifests, capture-count mismatch and one-capture-ahead reconciliation, config drift, header-only startup discovery versus full JPEG payload validation on Continue, partial capture staging without publishing a final filename, explicit Continue cleanup/retake, no-replace publication collisions, truncated preserved capture rejection before camera open/new capture, camera close when capturing-phase manifest persistence fails, Continue from partial and full-capture states, output-only retry, truncated rendered-image rejection, fail-closed preview cleanup for output retry, new-session restart preservation and ambiguous output folders.

Initial crash-recovery checks on 2026-11-07 and the immutable-frame follow-up were run on ShinPc:
- Root `go test -count=1 ./...`: PASS.
- Wails module `go test -count=1 ./...`: PASS, including preview-off and synthetic-only camera-entry guards.
- Root and Wails `go vet ./...`: PASS.
- Root Windows `go build ./...`: PASS.
- Wails Vite production build and Windows Preview `run-photobooth.ps1 -NoLaunch`: PASS.
- Wails binding generation: PASS.
- `git diff --check`: PASS after normalizing generated blank-line whitespace.
- `ui-recovery-smoke.ps1`: PASS. An isolated TEMP profile seeded five synthetic manifest-backed sessions and exercised startup discovery, explicit rejection of Start with webcam, Continue (remaining shots only), output-only retry, fresh-ID Restart preserving original bytes, Hide, and Inspect/export. `PHOTOBOOTH_DISABLE_CAMERA_PREVIEW=1` and `PHOTOBOOTH_SYNTHETIC_ONLY=1` were inherited before process start. The final preview JSONL contained only `preview_status:disabled`; no frame publication or camera recovery event occurred. The app exited gracefully and the harness deleted only its own TEMP data root. Report: `%TEMP%\\photobooth-recovery-ui-smoke-ea2a9e1c39554ae288602f17c9f01392.json`.

The smoke used generated JPEGs and the fake camera provider only. It did not open the physical camera. Linux runtime, USB recovery, soak, representative image-quality and printer gates were not run.

## Immutable frame snapshot follow-up (2026-11-07)

New sessions use manifest schema v2 and keep a verified copy of the exact frame definition at frame.json in the session directory. The snapshot is capped at 1 MiB, written to a same-directory temporary file and synced, then published without replacement (Unix hard link + temp unlink + directory sync; Windows MoveFileExW without REPLACE_EXISTING and with WRITE_THROUGH). The initial manifest is published only after the snapshot, and it records the fixed filename plus matching SHA-256 digest.

Continue reads and validates the snapshot bytes and renders from that path. It preserves the original configured frame path in the run fingerprint but does not require that source file to remain present for schema-v2 sessions. If the current Settings frame was changed or removed after the interruption, the old session still uses its own snapshot. Wails may bypass only the Settings warning specifically caused by a missing/unreadable current frame or a frame-slot-count mismatch; invalid profile, camera/provider or other station settings warnings still block recovery.

The frame schema v1 is purely declarative and has no external image/font/asset references, so copying the JSON definition preserves the complete render input for the currently supported schema. The rest of the station config remains fingerprinted rather than archived. Legacy manifest v1 sessions remain inspectable/resumable only when their original frame file still exists and matches the recorded digest.

The retention planner now recognizes both manifest.json and frame.json. A manifested session is a session-retention candidate only when its manifest is output_finalized and any declared frame snapshot passes digest verification. Corrupt/undeclared snapshots and sessions that still have a recovery action are skipped; the separate review/digest/apply workflow remains unchanged.

Follow-up regression coverage on 2026-11-07:
- Snapshot publish refuses replacement; a tampered snapshot fails SHA-256 verification.
- Read-only recovery discovery exposes only a verified snapshot path and makes a corrupt declared snapshot inspect-only.
- Workflow Continue completes a partially captured synthetic session after the original frame JSON is removed, rendering and saving from the snapshot.
- Wails discovery/Continue permits that same snapshot-backed flow when the only Settings warning is a missing current frame; other Settings errors remain blocking.
- Retention accepts a finalized session with a matching snapshot and skips one with a corrupt snapshot.


## Printer attempt recovery (manifest v3, 2026-11-19)

Session manifest schema v3 adds an append-only list of printer attempts while preserving read compatibility with schema v1 and v2. Existing v2 immutable frame snapshots remain readable; the durable printer wrapper upgrades a v2 manifest to v3 only when its snapshot is verified immediately before a first explicit print attempt. Legacy v1 sessions are not auto-upgraded for printing.

Each print attempt stores:
- A random attempt ID and a Windows document name derived from session ID + attempt ID.
- Explicit queue name and SHA-256 of the rendered JPEG.
- Request/update timestamps, state, optional spooler job ID and error.

The print state machine is deliberately conservative:
- submitting: the intent was atomically persisted before the Windows printer target is called. If the application exits/crashes before the result is written, discovery treats this as uncertain.
- queued: Windows accepted the job and returned a spooler job ID. It is not proof of paper output.
- failed: only a typed, definite pre-acceptance error. A later attempt may be started explicitly.
- uncertain: the job may have reached the spooler or the queued/job-ID result could not be durably written. No automatic resubmission is allowed.

A per-session .print.lock is created before persisting the print intent. New Windows submissions hold a version-1 kernel file lease until their result is persisted and the marker is removed. If a process exits abruptly, Windows releases the handle. Read-only recovery can classify the marker as active, orphaned or unverifiable; it never removes it during discovery. The Wails recovery card exposes Reconcile released printer lock only for an orphaned version-1 lease. Reconciliation reclaims the lock exclusively and is described below. The UI still has no print retry button. The explicit Inspect Windows queue action uses the exact persisted queue, document name and optional job ID with a read-only EnumJobsW snapshot. If the job is not listed, the UI states that this does not prove it printed or failed. There is no background polling or automatic transition to printed/completed. An explicit append-only operator resolution can now close a persisted manifest attempt as printed, not_printed or unknown; it preserves the original attempt and does not resubmit. A surviving version-1 Windows lease must first pass ReconcileOrphanedPrinterLock; legacy/unversioned, corrupt/partial and unsupported-platform locks remain inspect-only. A separate local-output retry does not re-submit the printer target.

The reusable DurablePrinter output target, StoreConfig v2 policy and Wails Windows Preview opt-in dispatcher are implemented. The root CLI remains local-only; demo sessions and local-output retries remain local-only. Printer stays disabled by default and is configured only through the operator-managed local profile, not the guest Settings screen. Automated/background spooler polling, handling of pre-lease/corrupt/partial locks, a no-duplicate policy verified on physical hardware and physical acceptance remain open. Keep the default profile disabled. See ADR 0033.


### Pre-manifest printer crash window

The .print.lock file now stores a bounded JSON record containing the attempt ID, queue name, deterministic document name and timestamp before the manifest PrintAttempt is written. Recovery validates this record and exposes it through the Wails DTO. This closes the earlier gap where a crash immediately after lock creation but before the manifest write left no document name to inspect. The recovery card can now inspect the exact Windows queue identity from the lock even when no PrintAttempt exists yet. A corrupt/invalid lock remains an uncertainty warning but does not expose an Inspect action without a trusted identity. Read-only discovery never clears the lock or mutates the session.


## Operator printer resolution (manifest v3, 2026-11-19)

Manifest v3 now stores an optional append-only print_resolutions list beside the append-only print_attempts history. Existing v1/v2/v3 manifests without resolutions remain readable; no schema-version bump is required for the optional list.

A resolution records the exact attempt ID, operator decision (printed, not_printed or unknown), whether the matching persisted queue/document/job was found during a fresh read-only Windows inspection, queue-check timestamp, record timestamp and a required operator note. The prior PrintAttempt is left unchanged. This preserves both the original spooler result (queued/submitting/uncertain) and the later operator decision.

The Windows Preview recovery card offers:
1. **Inspect Windows queue**: read-only EnumJobsW against the exact saved queue/document/job identity. A missing queue entry is only "not currently listed"; it does not establish a physical result.
2. **Confirm printed**: available only after the job is absent from the current queue snapshot. The operator must affirm that paper physically emerged.
3. **Confirm not printed**: available only after the job is absent. The operator must affirm that the printer/output tray was inspected and no paper emerged.
4. **Close as unknown · never retry**: available after inspection even if the job remains listed; this is a permanent no-retry closure, not a success/failure claim.

Every decision asks for an explicit confirmation and a required note. The Go API independently re-inspects the exact queue immediately before saving, re-reads the manifest and rechecks for .print.lock. It rejects printed/not_printed while the matching spooler job is still listed. It serializes with station capture/output lifecycle operations and appends the resolution atomically. No spooler job is paused, resumed, cancelled or deleted.

Only a recorded not_printed resolution makes a separate future DurablePrinter.Execute eligible. That is not an automatic retry: the current Wails UI has no print-send/retry button, and DurablePrinter is not wired into delivery.NewDispatcher or guest sessions. printed and unknown remain blocked from another attempt.

**Lock-only crash handling:** a version-1 Windows lease can be reconciled after the kernel proves that its prior handle is no longer held. Reconciliation creates exactly one uncertain PrintAttempt when the crash occurred before manifest intent publication; it does not infer a print outcome. Pre-lease/unversioned, corrupt/partial or unsupported-platform markers remain inspect-only and are never removed based on PID or queue absence.


### Kernel-verified printer lock ownership (Windows, 2026-11-19)

New DurablePrinter lock markers use a version-1 kernel-held Windows file lease. The marker stores the attempt ID, queue, document name, rendered-source SHA-256, creation time, lease version and owner PID. The PID is diagnostic only; the system does not use it to decide whether a process is alive. Win32 CreateFileW sharing rules provide the authority: while the original handle is held, an exclusive claim is denied; after its owning process exits, Windows releases the handle and a new process can claim it.

Read-only session discovery uses a compatible Win32 read-only handle so it can inspect the lock metadata while a submission is active. It then probes an exclusive claim and reports owner state as active, orphaned or unverifiable. The state shown in the UI is only a snapshot. The actual reconciliation method claims the lock again and holds it through validation, manifest persistence and lock deletion.

The Windows Preview offers **Reconcile released printer lock** only for a version-1 lock classified orphaned. Backend reconciliation:
- Verifies the exact session directory and claimed lock identity.
- Fully validates and hashes the session's rendered.jpg and requires it to match the lock's source SHA-256.
- If a matching manifest attempt is still submitting, changes it to uncertain. If it is already queued/uncertain/failed, preserves that state.
- If the lock was created but no PrintAttempt reached the manifest, appends exactly one matching attempt and persists it as uncertain. A snapshot-backed v2 manifest is upgraded to v3 only after its frame snapshot is reverified.
- Persists the manifest before removing the lock marker through the still-exclusive Windows handle. If the process crashes between those operations, the next reconciliation is idempotent.
- Does not query or mutate the Windows spooler and never submits another job. The operator must inspect the queue and then use the separate printed/not_printed/unknown resolution action.

Pre-lease legacy locks (lease version 0/omitted), corrupt/partial lock metadata, and platforms without a supported kernel claim remain unverifiable and inspect-only. They are not removed using PID checks or queue absence. PID reuse therefore does not affect the new Windows lease check. Linux builds are compile-only; this Windows locking behavior is not claimed as a Linux runtime feature.
