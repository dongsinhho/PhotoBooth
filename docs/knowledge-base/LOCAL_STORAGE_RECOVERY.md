# Local Storage Failure and Disk-Full Recovery

**Status:** Implemented in the local output provider, shared retry API, CLI and Wails prototype\
**Last updated:** 2026-10-31

## Failure behavior

The normal local output provider writes the finalized image into a session-scoped output directory using this sequence:

1. Validate the session ID and rendered source.
2. Create the session output directory.
3. Create a temporary file inside that same directory.
4. Copy the rendered source with context cancellation checks.
5. Sync and close the temporary file.
6. Rename the completed temporary file to photo.jpg.

A partial temporary file is removed on write, sync, close or rename failure. The final photo.jpg is not published until the full temporary file has been synced and closed.

## Insufficient-storage classification

The provider recognizes the operating-system disk-full/quota signals:

- Windows: ERROR_DISK_FULL (112) and ERROR_HANDLE_DISK_FULL (39).
- Non-Windows: ENOSPC and EDQUOT.

These errors wrap the exported output.ErrInsufficientStorage sentinel while preserving the original OS error for Go callers. Other errors, such as access denied, remain ordinary output errors and are not misclassified as disk-full.

The output coordinator retains the typed target cause for programmatic classification; the cause is excluded from JSON serialization. The human-readable per-target error and workflow failure event include operator guidance to free disk space or resolve the quota, then retry. No automatic retry loop is started.

## What is preserved

If local output fails after capture/render:

- All source capture JPEGs remain in the session's captures directory.
- The rendered source remains at sessions/<session-id>/rendered.jpg.
- A partial temp output is removed on a best-effort basis.
- No successful sibling output target is rolled back.
- The session reports a failed or partial result according to the independent target statuses.

The player does not delete a failed session or its render. It does not automatically retry or overwrite an existing completed photo.jpg.

## Retry only the local output (no recapture)

After freeing disk space or resolving the quota, the Wails result screen offers **Retry local save** when every configured capture and rendered.jpg are present but no local photo.jpg was published. The operation reuses the current session and copies the preserved rendered image only; it does not open the camera, recapture, rerender or modify station settings.

The same recovery is available from the repository root using the CLI and the exact station config/session ID:

~~~powershell
go run ./apps/output-retry --config .\examples\store-config-v1.json --session-id 0123456789abcdef0123456789abcdef
~~~

Replace the example session ID with the exact 32-character ID printed by the failed session. For a Wails profile, point --config at %LOCALAPPDATA%\PhotoBooth\store-config-v1.json.

Both UI and CLI use output.RetryLocalFromSession. It:

- Requires explicit storage roots, session ID and configured capture count.
- Verifies that the session directory, captures directory, all configured capture JPEGs and rendered.jpg exist as non-empty regular files.
- Refuses symlinked session/capture/output directories and symlinked source files.
- Refuses overlapping sessions/output roots.
- Refuses to overwrite an existing photo.jpg, including when two retry attempts race to publish.
- Copies only the preserved rendered.jpg to output/<session-id>/photo.jpg; it does not open the camera, recapture, rerender, upload data or delete session files.

Retry uses a same-directory temp file and an atomic no-replace hard link to publish the final name. A filesystem that does not support hard links fails safely rather than replacing an existing image.

A successful CLI retry prints OUTPUT_RETRY_SUCCEEDED and the destination path. The Wails button updates the result view to completed and enables Show saved photo. A failed retry retains the rendered session and shows the last error so the operator can resolve the cause and retry again.

## Validation

Tests inject synthetic disk-full errors at directory creation, temp-file creation, partial write, sync, close and rename. They verify typed error classification, preservation of the underlying OS error, temporary-file cleanup, absence of partial photo.jpg, preservation of source data, and preservation of an older completed output.

Shared retry tests verify success, no-overwrite, concurrent calls, incomplete captures, invalid ID, overlapping roots, symlink rejection and cancellation. Wails tests verify the UI-facing API's session/state guards and save-only behavior. Root and nested Wails test/vet suites, Vite production build and Windows Wails build passed on 2026-10-31. A Windows UI Automation smoke also passed against the built Wails app: the harness used an isolated PHOTOBOOTH_DATA_DIR, captured three synthetic images, introduced a per-session output-path obstruction after session creation, waited for the rendered failure state and Retry local save button, removed only the obstruction, invoked retry through UI Automation, then verified the same session completed and the final photo SHA-256 matched rendered.jpg. It also verified all three captures and rendered source remained, and the retry button was no longer enabled. The disposable profile/session was removed afterward.

No real disk was filled to trigger ERROR_DISK_FULL/ENOSPC and no physical camera was used in this output-retry UI smoke. This is a controlled filesystem-path failure plus an actual operator-button automation test, not a physical disk-exhaustion test.
