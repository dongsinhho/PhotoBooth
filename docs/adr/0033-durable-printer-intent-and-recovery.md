# ADR 0033: Durable printer intent and conservative recovery

**Status:** Durable intent/recovery and Windows Preview opt-in integration implemented; physical printer acceptance and production certification deferred. Printer remains disabled by default.
**Date:** 2026-11-19

## Context

The Windows GDI printer target (ADR 0032) returns a Windows spooler job ID after EndDoc succeeds. That means Windows accepted a job, not that physical paper was printed. A process can crash after spooler acceptance but before PhotoBooth records the returned job ID. Automatically submitting again after that boundary can produce duplicate paper output.

The existing session manifest is atomic, session-scoped and versioned. Schema v1 is legacy and has no immutable frame snapshot; schema v2 adds the immutable frame snapshot and persisted local/free delivery policy. Existing v1/v2 sessions must remain readable.

## Decision

### Manifest schema v3

Bump the session manifest envelope to v3 while preserving read compatibility for v1 and v2:
- v1 remains legacy and retains its existing original-frame requirements.
- v2 remains snapshot-backed and retains its existing media-delivery/payment policy rules.
- v3 requires the same immutable frame snapshot and policy fields as v2, and may additionally store up to eight print attempts.
- A v1 session is not upgraded automatically for printing because it lacks the immutable frame snapshot needed by the current durable-print contract.
- A v2 session is upgraded to v3 only when an operator explicitly starts its first printer submission and its verified frame snapshot is present. Other session data is preserved.

Each PrintAttempt records attempt ID, queue name, unique document name, rendered-source SHA-256, status, optional spool job ID/error, and request/update timestamps. Document name is derived from session ID and attempt ID so it can be searched in the Windows queue.

Allowed states:
- submitting: intent was atomically persisted before invoking the printer target; spool acceptance has not yet been durably recorded.
- queued: the Windows spooler accepted the job and PhotoBooth persisted the job ID. This is not physical-print confirmation.
- failed: the target returned a typed, definite pre-acceptance failure. This is the only state that allows a later explicit attempt.
- uncertain: the outcome may have reached the spooler or the result could not be durably recorded. No new attempt may be submitted automatically.

### Durable output target

The internal/booth/output/durable_printer.go wrapper around a platform target named printer:
1. Validate the local sessions root, session directory identity and exact rendered.jpg path.
2. Hash the rendered JPEG and reject a source that changes between a failed attempt and a later explicit attempt.
3. Acquire an exclusive per-session .print.lock. On Windows, CreateFileW holds a kernel file lease that denies concurrent write/delete access; an in-process mutex also serializes goroutines. A process crash releases the kernel handle automatically, allowing a later exclusive claim to distinguish an orphan without PID checks.
4. Persist a submitting attempt in manifest v3 before calling the platform printer target.
5. Pass the unique document name to the Windows GDI target.
6. Persist queued + spooler job ID only after the platform target reports TargetQueued with a job ID.
7. Persist a definite pre-acceptance failure as failed. Unknown/ambiguous errors and post-acceptance result-persistence failures are uncertain.
8. Remove the lock on normal completion while the lease remains held. If a process crashes, Windows releases the lease but leaves the marker for explicit kernel-verified reconciliation.

The GDI adapter classifies failures before positive StartDocW acceptance as definite pre-acceptance errors. Once StartDocW returns a positive spool job ID, subsequent errors are uncertain and carry that ID; EndPage/EndDoc errors are never treated as safe-to-retry failures.

### Recovery and UI

Read-only session discovery validates manifest v3 PrintAttempts and detects .print.lock. It reports an explanatory reason but does not change the manifest, delete a lock or submit another job.

The Wails recovery DTO converts attempt timestamps to RFC3339 strings. The recovery panel shows queue, document name, job ID and state. It deliberately has no print-retry action:
- queued: Windows accepted the job; physical output is not confirmed.
- submitting / uncertain / stale lock: inspect the Windows queue using the exact document name before any manual decision; never retry automatically.
- failed: the previous attempt was rejected before spooler acceptance; any next attempt remains an explicit future operator action.
- local save recovery stays independent and may retry only the local target without resubmitting the printer job.

## Validation

Synthetic tests cover manifest state transitions, duplicate/invalid attempts, v1/v2 compatibility and read-only recovery. Fault-injection tests cover:
- intent is present before platform target invocation;
- a definite pre-acceptance error is persisted failed and a second explicit attempt is allowed;
- an unclassified error is uncertain and a second submission is blocked;
- if Windows accepted a job but persisting queued/job ID fails, the on-disk state remains submitting and recovery surfaces uncertainty;
- an abandoned lock blocks another submission and remains visible to read-only discovery;
- v2 snapshot-backed sessions can be upgraded to v3 before the first print attempt.

On 2026-11-19, the DS-RX1 virtual smoke was upgraded to dispatch local save plus DurablePrinter through output.Coordinator. One durable job passed:
- attempt ID: 5d3abb11e1b8b90a4c6fa7cba7302040
- document name: PhotoBooth-e3b06bc2e3319b48f25b692647e3a1b2-5d3abb11e1b8b90a4c6fa7cba7302040
- Windows spooler job ID: 29
- manifest print status: queued
- local save SHA-256 matched the synthetic source
- virtual converter created one 1200x1800 JPEG
- queue returned Normal / JobCount 0

The exact read-only queue inspector was exercised by a second durable smoke with spooler job ID 30. The virtual converter had already removed the job by the time EnumJobsW ran, so Found=false; the tool reported no current queue entry without interpreting that as printed/completed or failed. The queue remained Normal / JobCount 0. No physical paper was produced.

See internal/booth/output/durable_printer.go, internal/booth/session/manifest.go, tools/printer-poc/dsrx1-virtual-smoke/README.md and docs/knowledge-base/SESSION_CRASH_RECOVERY.md.

## Not yet production-enabled

- DurablePrinter is a reusable target but is not wired into delivery.NewDispatcher, StoreConfig or Wails guest sessions.
- StoreConfig does not yet express printer enabled/queue/media/orientation settings. Printing must remain disabled by default.
- The UI exposes read-only queue metadata plus explicit operator resolution/closure records (printed/not_printed/unknown). It has no print-send or retry button and does not mutate the spooler.
- An explicit, read-only Windows spooler lookup by the persisted queue + document name + optional job ID is implemented. The Wails recovery card exposes Inspect Windows queue. It reports only jobs still listed by EnumJobsW; a missing job is not proof of success or failure.
- There is no background polling or automatic transition to printed/completed. A separate append-only resolution records an operator's decision and queue observation; printed/not_printed require queue absence plus physical operator confirmation, while unknown remains permanently blocked from retry.
- Pre-lease/unversioned, corrupt/partial or unsupported-platform lock markers remain inspect-only. Newly created Windows version-1 leases can be reconciled only after a kernel exclusive-claim proves the old handle is no longer held, as described below.
- Job ID + queued means spooler acceptance only. It is not durable proof of physical print completion.
- This has been tested only through the local DS-RX1 virtual port converter. Physical printer, paper/cut, color, offline/failure, duplicate-paper and native Ubuntu printer gates remain open.
- StartDocW/EndDoc are synchronous; Go context cancellation cannot forcibly interrupt a blocked driver call.

## Consequences

- The session owns an append-only history of print attempts.
- Ambiguous submissions fail closed and are never retried automatically.
- A local save can succeed/fail independently of the printer job.
- The Wails Windows Preview has an opt-in StoreConfig v2 local-plus-printer path. Production deployment remains gated on physical printer/media acceptance, duplicate-output behavior and device-specific error validation.


### Read-only Windows queue inspection follow-up (2026-11-19)

The Windows adapter now exposes a bounded EnumJobsW snapshot by exact persisted queue + document name + optional job ID. It does not mutate the job. Wails exposes this through Inspect Windows queue for queued/submitting/uncertain attempts and stale locks. When not found, the UI states only that the job is no longer listed; it does not mark it printed or failed.

A second DS-RX1 virtual durable smoke used job ID 30. The queue inspector returned Found=false because the configured virtual converter had already removed the job. The smoke recorded this as not currently listed, while local save and virtual JPEG conversion passed. Queue ended Normal / JobCount 0. No paper output occurred.

Still open: background polling, operator-controlled uncertain-state resolution/closure, StoreConfig printer settings and physical printer acceptance.


### Pre-manifest crash-window refinement (2026-11-19)

The exclusive .print.lock now contains a bounded JSON identity record (attempt ID, queue name, deterministic document name and creation timestamp), written and synced before manifest intent publication. Read-only recovery validates and exposes this identity. Therefore, if the process crashes after creating the lock but before the PrintAttempt is appended to manifest v3, the operator can still invoke Inspect Windows queue using the lock identity. If the lock JSON is corrupt, the session remains uncertain but the Inspect action is withheld because there is no trusted queue/document identity.

A new session test covers a v3 rendered session with a valid crash lock and no manifest PrintAttempt. It verifies discovery recovers the lock identity without modifying the manifest or lock. Wails DTO tests cover safe lock serialization. A final DS-RX1 virtual smoke after this change passed with spooler job ID 31, attempt ID 983bdd1b9702439ed04666703969e909 and a 1200x1800 converted JPEG. EnumJobsW returned Found=false after the virtual converter removed the job; the UI semantics remain "not currently listed", not completed or failed. Queue ended Normal / JobCount 0. No physical paper output occurred.


## Operator-controlled resolution and closure (2026-11-19)

Manifest v3 now contains a separate append-only print_resolutions list. A resolution references one persisted queued/submitting/uncertain PrintAttempt and records:
- operator decision: printed, not_printed or unknown;
- whether the exact matching queue/document/job was still found during a fresh Windows queue inspection;
- queue-check and record timestamps;
- a required bounded operator note.

The original PrintAttempt is never rewritten. Resolution is a separate audit record. Printed/not_printed decisions are accepted only after the matching job is absent from the Windows queue; the operator must additionally confirm the physical result (paper observed or not observed). Queue absence alone never supplies that physical fact. Unknown may be recorded even when a job remains listed; it closes the attempt but permanently blocks retry.

The Wails Windows Preview recovery card now offers Inspect Windows queue followed by three explicit decisions. Each decision requires a native confirmation and a required operator note. ResolvePrinterAttempt re-scans the session, rejects an existing .print.lock, re-inspects the exact persisted queue/document/job at decision time, reloads the manifest, rechecks the lock and appends the resolution atomically. It serializes against station capture/output lifecycle operations. It does not pause/resume/cancel/delete/submit a spooler job.

Only an explicit not_printed decision permits a later separately invoked DurablePrinter.Execute; printed and unknown block further attempts. No automatic retry is added and the Wails card does not provide a print-submission/retry action. DurablePrinter remains unwired from delivery.NewDispatcher and guest sessions.

**Conservative exception:** ResolvePrinterAttempt always refuses while .print.lock exists; a version-1 Windows lease must first pass ReconcileOrphanedPrinterLock. Pre-lease/unversioned, corrupt/partial and unsupported-platform lock markers remain inspect-only because their owner cannot be verified with a kernel-held lease. They are never removed using PID checks or queue absence.

Tests cover physical decisions while a job is still queued (rejected), not_printed unlocking a subsequent explicit attempt, printed/unknown blocking, duplicate/invalid resolutions, manifest round-trip preserving the original attempt, queue reinspection with exact persisted identity, lock refusal without mutation, unknown closure with queue entry still present, and JSON-safe Wails audit presentation.


## Kernel-held Windows lock lease and orphan reconciliation (2026-11-19)

The previous PID/liveness gap is closed for newly created Windows printer locks by a versioned kernel-held file lease. A version-1 .print.lock now includes lease_version, owner_pid (diagnostic only), attempt ID, queue, deterministic document name, rendered-source SHA-256 and creation time.

- DurablePrinter creates the lock with Win32 CreateFileW using exclusive create and a handle that denies other processes write/delete sharing while allowing a specially opened read-only inspection handle. It holds that handle from before manifest intent publication through spooler submission and manifest result persistence.
- The PID is not used as liveness proof. Windows releases the handle when its owning process exits, even after abrupt termination. A second process obtains the exclusive claim only when that kernel handle is no longer held. This avoids PID reuse ambiguity.
- Read-only discovery opens the marker with a Win32 read handle whose sharing flags permit safe concurrent reading, then probes owner state by attempting an exclusive claim. The result is active, orphaned or unverifiable. An orphaned result is only a point-in-time observation; the destructive action reclaims the lease again and holds it through recovery.
- The recovery UI exposes Reconcile released printer lock only for a version-1 lease classified orphaned. The Go method reclaims it exclusively and validates the exact lock/session identity and rendered-source hash.
- If the manifest already contains the matching submitting attempt, reconciliation changes only that attempt to uncertain. If the attempt result had already been persisted (queued/uncertain/failed), it preserves that state. If the process exited after creating the lease but before manifest intent publication, reconciliation verifies/upgrades the snapshot-backed manifest to v3 if required, appends one matching attempt and finalizes it as uncertain. It never guesses that printing succeeded or failed.
- The manifest write happens before lock deletion. The lock is marked for deletion through the exclusively held Windows file handle (SetFileInformationByHandle) before the handle is closed, avoiding a close/delete race. If deletion fails after the uncertain attempt persisted, a later reconciliation is idempotent.
- Reconciliation does not inspect, cancel or submit a spooler job. Afterward, the operator must inspect the exact Windows queue identity and use the existing printed/not_printed/unknown resolution workflow.

**Conservative compatibility:** pre-lease locks (lease_version omitted/zero), corrupt/partial markers and platforms without a verifiable kernel claim remain unverifiable and inspect-only. They are never deleted or upgraded based on PID or queue absence. A partial lock creation interrupted before the complete metadata is synced can also remain inspect-only. Native non-Windows printer lock claiming is not implemented; Linux validation remains compile-only.

Windows tests cover active-holder refusal, orphan claim after a child process exits abruptly without running defers, marker removal while the claim is held, legacy-lock preservation, lock-only crash conversion, an existing submitting-attempt transition without duplicates and active-lock reconciliation refusal. No real queue or physical printer is used by these tests.


## Windows Preview opt-in software integration (2026-11-20)

Wails Windows Preview consumes StoreConfig v2 printer settings only when the operator explicitly sets `printer.enabled=true`. For real-camera sessions the dispatcher is `Coordinator(LocalProvider, DurablePrinter)`; local save and printer submission are independent. Demo sessions are always local-only. The root CLI remains local-only. Both normal and recovered local-output retry paths bypass the dispatcher and cannot re-submit printer jobs. The Settings UI does not expose a guest-facing printer toggle; settings remain operator-managed in the local profile. The canonical defaults/example remain disabled.

PlayerSession snapshots now carry per-target results (name, status, job ID, error and duration). The final result messaging distinguishes local-save success/failure from Windows spooler acceptance and avoids claiming physical print completion. A queued job is still not evidence of paper output.

The GDI adapter supports the exact Windows driver DEVMODEW.dmFormName on a per-job basis. It validates the 31 UTF-16 code-unit limit, preserves the vendor-private DEVMODE tail, and runs DocumentPropertiesW validation before submitting. The DS-RX1 virtual driver on ShinPc exposes form names such as (6x4), (6x8) and (5x7); one single-copy synthetic smoke with (6x4) passed through the driver and virtual Multi File Port Monitor. Job ID 32 was persisted as queued; local output matched source hash; the virtual converter emitted a 1200x1800 JPEG; post-conversion queue inspection returned not-found. physical_print_confirmed=false. The new JPEG was deleted after verification and pre-existing output files were preserved.

The code is still Windows Preview software integration only. Printer remains disabled by default; this does not certify a physical printer, media, color, paper feed/cut, failure states or duplicate behavior. Physical acceptance is pending due to hardware unavailability; native Ubuntu driver acceptance is a separate future gate.
