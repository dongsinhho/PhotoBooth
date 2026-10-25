# Output Targets

**Status:** Coordinator and local-storage target implemented; physical printer adapter not yet implemented.

## Intent

The finalized photo produced by the renderer is one immutable input to the output stage. Storage and printing are independent targets: a failure in one must not prevent the other from trying. Output targets are also separate from guest media delivery through LAN or cloud.

## Current implementation

The Go package is internal/booth/output.

- `Job` identifies the session and read-only rendered source file.
- `Target` exposes a stable name and an `Execute` operation for one destination.
- `Coordinator` validates the common job, rejects duplicate target names, limits configuration to four targets, starts target operations concurrently, and returns per-target results in configuration order.
- The `LocalProvider` implements the target interface and persists the finalized image in a session-scoped folder.
- Target results include status, elapsed duration, error text, optional local output path, and optional external job ID.
- The workflow records all target results and the local output path if local storage succeeded.

The default CLI configures only the local-storage target. Tests use a synthetic printer target; this is not a physical printer integration.

## Status and lifecycle semantics

- `succeeded`: the target completed its synchronous operation. For local storage this means the file was saved.
- `queued`: the target accepted an asynchronous job. For a future printer target this should mean the spooler accepted the job, not that paper has physically emerged. The job ID is recorded when available.
- `failed`, `cancelled`, `timed_out`: the individual target did not complete its operation.

Session outcomes:
- `completed`: every configured output target completed or accepted its job.
- `partial_success`: at least one target completed/queued, while another target failed, was cancelled, or timed out.
- `failed`: no output target completed or accepted its job.

No successful target is rolled back if a sibling target fails. The coordinator does not silently retry printing; duplicate-print policy must be designed with the actual printer/provider because retrying a spool operation can create duplicate paper output.

## Performance and failure behavior

- A maximum of four configured targets bounds concurrent work; the common local-plus-printer use case launches both independently.
- Target results carry per-target durations; the coordinator duration is recorded by the workflow.
- Target implementations must honor the supplied context where possible. Native printer APIs should avoid blocking the capture/UI thread.
- A printer adapter should normally enqueue/spool the job and return a job ID plus `queued` status, rather than holding the capture session open until physical printing finishes.
- The UI event callback runs on the session goroutine. UI adapters must enqueue events and return quickly.
- Target errors are retained in session results and exposed through events. Avoid logging photo contents, credentials, or sensitive printer/network paths unnecessarily.

## Remaining work

1. Select a real printer model and the supported Windows/Linux environments before implementing a native printer adapter.
2. Test the printer's job acceptance/completion signals, cancellation, offline state, paper-out state, and duplicate handling.
3. Decide whether print state should persist across application restart and how to retry or resume pending jobs safely.
4. Add soak/performance tests with simultaneous local storage and print queuing.
5. Keep LAN/cloud photo delivery as a separate module; it must not become another name for local saving or printer output.
