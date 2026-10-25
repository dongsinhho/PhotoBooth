# ADR 0010: Independent output targets for local storage and printing

**Status:** Accepted for the output workflow
**Date:** 2026-10-25

## Context

PhotoBooth must be able to save a photo locally and optionally print it. The two operations must not depend on one another: disk/output failure should not suppress a print attempt when configured, and printer failure should not remove the local copy. Performance matters, and a slow physical printer must not block camera capture or UI events.

## Decision

1. Treat the rendered photo as an immutable input to an output-dispatch stage, separate from camera capture, frame rendering, and guest media delivery.
2. Define independent named output targets behind the Go `output.Target` interface. Each target receives a session ID and source-file path and returns a result or an error.
3. Add an `output.Coordinator` that validates the shared job, rejects duplicate target names, caps configured targets at four, runs target attempts concurrently, and preserves configured result order.
4. Keep local storage as one target. A printer will be another target when a concrete model/platform is selected; it should not be implemented as a side effect of local saving.
5. Preserve per-target status, timing, error, output path, and optional job ID. One target failure must not cancel or undo a sibling target.
6. Distinguish `succeeded` from `queued`. A printer adapter may return `queued` with a spool/job ID when the system has accepted the print job; it must not claim physical print completion without reliable confirmation.
7. Add `partial_success` as a terminal session state. All targets completed/queued yields `completed`; at least one completed/queued plus one failed/cancelled/timed-out yields `partial_success`; no successful/queued target yields `failed`.
8. Use a bounded number of concurrent targets. Every target must honor cancellation where possible. Printer work should normally be queued/spooled and return quickly instead of waiting for the physical printer before returning to the session workflow.
9. Keep printing retry/duplicate semantics out of the generic coordinator. They require provider-specific job identity and recovery rules.

## Consequences

- The local photo and printer operations are independently observable and can run concurrently.
- A printer failure will not prevent a local file from being saved; a local-storage failure will not prevent a configured printer target from being attempted.
- UI and logs can show which target succeeded, failed, or accepted a queued job.
- The current implementation includes the coordinator contract and local-storage target, plus synthetic target tests. It does not yet contain a physical printer adapter.
- LAN/cloud guest delivery remains a separate feature and policy.

## Verification

Unit/integration tests cover independent local success when a printer target fails, accepted queued printer output when local storage fails, bounded target configuration, result ordering, concurrent dispatch, and full/partial/failed session outcomes. No physical printer is claimed as tested.
