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
| `storage` | `sessions_dir`, `output_dir`, `session_retention_days`, `output_retention_days` | Required local filesystem paths; retention values are optional integers, 0 disables cleanup, 1–3650 days enables the separate cleanup CLI |
| `media_delivery` | `mode` | Only `local` is implemented |
| `payment` | `mode` | Only `disabled` is implemented; this is the free/no-payment path |
| root | `schema_version` | Required integer; currently must equal `1` |

Relative paths are resolved from the directory containing the config file. Absolute paths are preserved. On Windows, local mode rejects UNC paths and mapped network drives for both session and output roots during validation, load-after-resolution and save-before-write. This is a guard against accidental network storage, not a filesystem sandbox: reparse-point/junction targets and network mounts on Linux/macOS are not yet classified by this check.

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

On Windows, local mode rejects UNC session/output roots and mapped network drive volumes. Validation occurs in StoreConfig.Validate, after relative paths are resolved by Load, and before Save writes the config. This also catches relative roots when the station config itself resides on a UNC share. The Windows tests cover UNC paths; mapped-drive detection uses GetDriveTypeW but has not had a live mapped-drive integration test.

Limitations: junctions/reparse points are not resolved to their final volume, and Linux/macOS network mounts are not classified yet. Do not interpret this as a filesystem sandbox or as proof that local media can never be exposed on a network. The Stage 2 exit gate remains open.
