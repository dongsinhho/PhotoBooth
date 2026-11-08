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
