# ADR 0019: Local output disk-full recovery

- **Status:** Accepted and implemented in the root Go core/CLI and Wails prototype; user confirmed Retry local save is now stable on 2026-11-01
- **Date:** 2026-10-30
- **Implementation update:** 2026-10-31

## Context

Phase 3 of the product plan requires recoverable local output failures, including disk-full behavior. Captures and the rendered image are valuable session artifacts; a failed local copy must not leave a partial final photo or force the operator to repeat a successful camera session unnecessarily.

A free-space preflight is not a reliable correctness mechanism because available space can change between the check and the write. The actual filesystem operation must remain authoritative.

## Decision

The normal local output provider continues to use a same-directory temporary file, context-aware copy, sync, close and final rename. It recognizes disk-full/quota errors at directory creation, temp-file creation, write, sync, close and rename:

- Windows: ERROR_DISK_FULL (112) and ERROR_HANDLE_DISK_FULL (39).
- Non-Windows: ENOSPC and EDQUOT.

These wrap output.ErrInsufficientStorage and retain the original OS error. Generic I/O errors are not classified as disk-full.

A failed temp output is removed on a best-effort basis. The final photo.jpg is not published until the temp file has been fully copied, synced and closed. The session capture JPEGs and rendered.jpg remain in the work directory. The output coordinator retains the target's typed cause in-process while excluding it from JSON; the human-readable per-target error remains available to the UI/log.

The shared output.RetryLocalFromSession API is used by both the apps/output-retry CLI and Wails. It requires explicit storage roots/session ID/capture count, validates the real session/capture directories and every expected non-empty capture plus rendered.jpg, rejects overlapping storage roots and symlinked entries, and refuses to overwrite an existing local photo.jpg. It republishes the preserved rendered image only; it does not open the camera, recapture or render again.

Retry publication uses SaveExclusive: the fully copied/synced same-directory temp file is linked to photo.jpg, which provides atomic no-replace behavior on filesystems supporting hard links. A competing retry or existing photo cannot silently replace the destination. Normal Save retains its existing temp-file/rename contract.

The Wails result screen now exposes **Retry local save** when all configured shots and rendered.jpg exist but no local output path was published. It reports the retry state and disables conflicting session/settings/frame changes while the save is running. The retry API returns the refreshed session state and retains an actionable diagnostic on failure.

## Consequences

- On 2026-11-01, the user confirmed the Retry local save feature is now stable and asked to move to the next planned work. No further retry-specific implementation is scheduled unless a reproducible regression appears.
- This operational confirmation does not by itself establish that a real full-volume/quota exhaustion test was performed; the recorded synthetic failure-injection and controlled output-path-obstruction tests remain the explicit technical evidence below.
- A failed disk-full local target is reported as failed with actionable instructions to free space/resolve quota.
- A failed copy does not publish a partial final JPEG; previously completed output remains unchanged.
- Operators can retry local saving from rendered.jpg without reopening the camera, taking new photographs or rerendering.
- The retry action is available in the Wails result screen and through the explicit CLI.
- Retry does not run automatically and does not delete preserved failed sessions.
- There is no free-space estimate, secure erasure or automatic cleanup of preserved sessions.
- The hard-link no-replace publication depends on filesystem hard-link support; unsupported destinations fail safely without replacing an existing photo.
- Real full-disk/quota exhaustion was not induced on the development machine; failure-injection tests simulate OS error codes. The Wails result-screen action passed a UI Automation smoke on 2026-10-31 with synthetic captures and a controlled output-path obstruction, but this does not simulate ERROR_DISK_FULL/ENOSPC.

## Validation

Temporary fixture tests inject disk-full at directory creation, temporary-file creation, partial write, sync, close and rename. They verify typed error classification, original cause preservation, temp-file cleanup, absence of partial photo.jpg, retention of the rendered source and captures, preservation of an old output, and successful save after the error condition is removed.

Shared retry API tests verify successful save-only retry, source/capture preservation, wrong ID and incomplete-session rejection, overlapping-root/symlink refusal, cancellation, no-overwrite behavior and concurrent retries publishing at most once. CLI tests exercise the shared API. Wails API tests verify successful retry from a failed session, same session identity, completed output state, rejection of wrong/already-completed sessions, incomplete capture rejection and conflicting-operation guards.

On 2026-10-31, root full tests/vet, Windows build, Linux/amd64 compile-only, Python camera-POC tests, nested Wails tests/vet, Vite production build and Wails Windows build passed. Wails generated bindings include RetryPhotoOutput. A manual Wails UI Automation smoke of the new result-screen button passed on 2026-10-31 using a disposable profile/session and a controlled output-path obstruction (not a real disk-full signal). A real disk-full/quota hardware test remains outstanding.
