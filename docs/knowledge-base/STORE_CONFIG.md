# Versioned Store Configuration (Schema v1)

**Status:** Implemented in the root Go CLI/core and the isolated Wails Windows Preview player (shot count, timing, frame path, and opt-in session/output retention policy).

## Run with the sample config

From the repository root:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json
```

The sample intentionally selects `fake` as the camera provider. It is safe for a synthetic demo and does not access the physical webcam.

For the reference B525 on Windows, explicitly override the provider and desired timings:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json --camera=windowsmf --device="B525 HD Webcam" --countdown=0s --interval=0s
```

Existing CLI flags still work without a config file. If a config file is supplied, only flags explicitly present on the command line override file settings; flags omitted from the command preserve config values.

## Shape

The example at `examples/store-config-v1.json` is the canonical sample. Schema v1 contains:

| Section | Fields | Current validation |
|---|---|---|
| `camera` | `provider`, optional `device`, `width`, `height`, `fps` | Provider `fake` or `windowsmf`; dimensions 1–4096; FPS greater than 0 and at most 120 |
| `capture` | `shots`, `countdown`, `interval` | 1–12 shots; Go duration strings from `0s` through 5 minutes; shot count must match frame slots at workflow preflight |
| `frame` | `config_path` | Required non-empty local frame JSON path; renderer performs frame-schema validation |
| `storage` | `sessions_dir`, `output_dir`, `session_retention_days`, `output_retention_days` | Required filesystem roots; Windows rejects UNC/network drives and follows existing junctions; Linux classifies the backing mount; retention values are optional integers, 0 disables cleanup, 1–3650 days enables cleanup |
| `media_delivery` | `mode` | Only `local` is implemented |
| `payment` | `mode` | Only `disabled` is implemented; this is the free/no-payment path |
| root | `schema_version` | Required integer; currently must equal `1` |

Relative paths are resolved from the directory containing the config file. Absolute paths are preserved. Windows local-mode checks follow existing junction/symlink targets and reject UNC/network drives; Linux checks the backing mount from /proc/self/mountinfo and rejects known network filesystem types. Load and Save validate resolved roots, and LocalProvider revalidates output directories immediately before writing. See the limitations in the Local-mode network path guard section below.

## Defaults and validation

Missing fields inherit the schema-v1 defaults from `internal/booth/config.Defaults()`. Defaults use the synthetic camera, three shots, a 3-second countdown, one-second inter-shot delay, local media delivery, and disabled payment. Both retention values default to `0` (disabled), including older schema-v1 profiles that omit the fields. Storage and frame paths are relative and become absolute when loaded from a file.

The loader rejects files over 64 KiB, unknown JSON fields, missing or unsupported schema versions, invalid ranges/durations, malformed JSON and unsupported mode values. It does not execute code or fetch remote resources. Credentials and guest photo data do not belong in this config.

Unimplemented modes such as cloud delivery or QR payment fail validation rather than silently falling back to local/free behavior.

## Wails player settings integration

The Wails Windows prototype now reads and writes a station profile through the same root `internal/booth/config` package. Its profile is stored at `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json`. The first launch installs the embedded sample frame at `%LOCALAPPDATA%\PhotoBooth\frames\photo-strip-v1.json` and initializes local `sessions` and `photos` folders.

The **Settings & frame** screen exposes shot count (1–12), countdown, interval, a local frame JSON path, and independent session/output retention days (0 disables; 1–3650 enables). Browse opens a native local file picker. Saving is disabled while a session/output retry or retention preview/apply runs, validates durations and the selected frame schema, and requires the frame's slot count to match the configured shot count. Saving a retention policy never deletes data; the separate Review old local data panel must build a read-only candidate preview before a one-time, explicit Apply confirmation. Settings persist across restarts; invalid existing config is not silently overwritten, shows a warning, and blocks sessions until the operator saves a valid replacement.

This is wired into the Wails **prototype only**. It does not add live-preview, arbitrary frame editing, camera-control options, cloud/LAN delivery, payment processing, hot reload, or production UI certification. The UI settings page does not upload photos or frame files.

## Current scope

Implemented: strict JSON load/save, safe defaults, validation, relative path resolution, root CLI integration and explicit CLI override precedence; Wails prototype settings for shot count, countdown, interval and frame path; local .pbframe create/validate/install/list; and Wails package validate/install/activate/rollback with local integrity checks and recovery metadata. See [FRAME_PACKAGE_FORMAT.md](FRAME_PACKAGE_FORMAT.md).

Opt-in local session/output retention is implemented in the core and CLI, and surfaced through the Wails Windows Preview Settings UI. CLI cleanup defaults to read-only dry-run and requires a reviewed plan digest plus `--apply`; UI cleanup requires a fully displayed preview, the latest one-use digest and explicit confirmation. Neither path is triggered automatically by the player. See [LOCAL_DATA_RETENTION.md](LOCAL_DATA_RETENTION.md).

Not implemented yet: file-watcher/hot-reload, automatic migration from older schema versions, package signatures/publisher trust, full theme packages, catalog/download sync, arbitrary assets/fonts/animation, LAN/cloud delivery, payment processing, or a policy for locking down config paths on a multi-user PC.


## Restart persistence smoke (2026-11-01)

A new isolated Wails UI smoke, `tools/ui-spike/wails-v2/ui-settings-persist-restart-smoke.ps1`, verifies persistence across two separate PhotoBooth.exe processes. It uses a disposable `PHOTOBOOTH_DATA_DIR`, saves shot count 3, countdown 4s and interval 750ms, closes the first process gracefully, starts a second process against the same local profile, opens Settings, and verifies all three values are reloaded. The smoke passed on 2026-11-01: both processes started and exited normally, the second process showed shot count 3 / countdown 4s / interval 750ms, no photo session or capture was created, and the temporary profile was removed. Report: `%TEMP%\photobooth-settings-restart-6d4b3d0f640f448aa8ec9345a4194462.json`. This validates the schema-v1 settings screen persistence path in the experimental Wails player; it is not a production-install/update test.

## Local-mode network path guard (2026-11-06)

On Windows, local mode rejects UNC roots, mapped network drives and existing directory junction/symlink paths whose resolved target is network-backed. It follows the nearest existing ancestor with CreateFileW/GetFinalPathNameByHandleW, preserving not-yet-created child components, then checks GetDriveTypeW. On Linux, it resolves symlinks at the nearest existing ancestor and classifies the longest matching mountpoint from /proc/self/mountinfo. Known network filesystem types include NFS/NFS4, CIFS/SMB, SSHFS, 9p, Ceph, GlusterFS, Lustre, AFS, Coda, DAVFS and selected remote FUSE drivers. The mountinfo parser handles optional fields and escaped mountpoint paths.

StoreConfig.Validate checks direct paths; Load checks again after resolving config-relative paths; Save checks resolved paths before replacing config. LocalProvider also checks the output root before directory creation and re-checks the concrete session directory after creation but before creating a temporary photo file. Tests cover Windows UNC rejection, a real local directory junction, final-path normalization, mountinfo parsing/type classification and injected network rejection before output bytes are written.

Limitations: Linux mount classification has only been cross-compiled in this environment; no Linux runtime/mount integration test was run. The network filesystem list is explicit, so unknown FUSE/network providers may not be recognized. macOS/other platforms retain the unclassified fallback. A mount or reparse target can still change in the narrow interval after validation, so this is a best-effort guard, not a filesystem sandbox or a guarantee against all network exposure. Stage 2 exit gate remains open.

## Session and output storage isolation (2026-11-06)

The session workspace root and finalized photo output root must be distinct, non-overlapping trees. The check canonicalizes both paths before comparing them, follows existing Windows junction/symlink and Linux symlink path components where those platform resolvers are implemented, and rejects identical/nested roots or aliases that resolve into the same tree. This rule is enforced by StoreConfig.Validate, Load after resolving config-relative paths, and Save before replacing the profile. It also runs when building a retention plan, including when only one retention policy is enabled.

A session ID owns exactly one direct child directory under the sessions root, and the same ID owns one direct child under the output root. Workflow, output retry and LocalProvider validate that the resolved child remains directly beneath its assigned root and that the resolved basename still matches the session ID. Re-directed child junctions/symlinks are rejected. LocalProvider normal dispatcher execution uses exclusive publication: an existing photo for a session ID is never overwritten.

A new workflow session claims its workspace with atomic directory creation and refuses an existing session ID. Continuing a failed session is an explicit resume operation: it requires the existing workspace to contain only its captures directory, and that directory to contain exactly the expected completed capture files. An empty capture directory is valid for resuming a failure before the first completed shot. Unknown/partial files, symlinks, unexpected rendered artifacts or mismatched capture counts fail closed. A normal new session never reuses a prior workspace.

The Wails player snapshots station settings, frame path, storage roots, capture count and timing under the app lock when a session starts or continues. Later edits cannot redirect a running session's renderer, camera parameters or output into a different profile. Continue also refuses to start while station settings are invalid or a settings save is in progress.

Validation on the Windows reference host includes root overlap tests (identical/nested/sibling and symlink aliases), live mklink /J tests for root and session-ID redirection, stale-session-ID refusal with previous bytes preserved, empty-workspace resume, unexpected-entry resume refusal, retry identity checks, normal output no-overwrite, and an immutable Wails runtime-config snapshot test. This is best-effort path identity enforcement, not an OS-level sandbox; a narrow TOCTOU interval remains between the final path check and the filesystem operation.
