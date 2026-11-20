# Versioned Store Configuration (Schema v2)

**Status:** Implemented in the root Go CLI/core and the isolated Wails Windows Preview player. Schema v2 adds operator-managed printer queue/media/orientation settings. The Wails Windows Preview now consumes them for an opt-in local-plus-durable-printer output path when `printer.enabled=true`; defaults remain disabled. Physical printer/media acceptance is PENDING because no physical unit is available.

## Run with the sample config

From the repository root:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v2.json
```

The sample intentionally selects `fake` as the camera provider. It is safe for a synthetic demo and does not access the physical webcam.

The legacy `examples/store-config-v1.json` remains available as a schema-v1 migration example. The loader upgrades it in memory to schema v2 and never rewrites the file unless an explicit Save occurs.

For the reference B525 on Windows, explicitly override the provider and desired timings:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v2.json --camera=windowsmf --device="B525 HD Webcam" --countdown=0s --interval=0s
```

Existing CLI flags still work without a config file. If a config file is supplied, only flags explicitly present on the command line override file settings; flags omitted from the command preserve config values.

## Shape

The canonical current sample is `examples/store-config-v2.json`. Schema v2 contains:

| Section | Fields | Current validation |
|---|---|---|
| `camera` | `provider`, optional `device`, `width`, `height`, `fps` | Provider `fake` or `windowsmf`; dimensions 1–4096; FPS greater than 0 and at most 120 |
| `capture` | `shots`, `countdown`, `interval` | 1–12 shots; Go duration strings from `0s` through 5 minutes; shot count must match frame slots at workflow preflight |
| `frame` | `config_path` | Required non-empty local frame JSON path; renderer performs frame-schema validation |
| `storage` | `sessions_dir`, `output_dir`, `session_retention_days`, `output_retention_days` | Required filesystem roots; Windows rejects UNC/network drives and follows existing junctions; Linux classifies the backing mount; retention values are optional integers, 0 disables cleanup, 1–3650 days enables cleanup |
| `media_delivery` | `mode` | Only `local` is implemented |
| `payment` | `mode` | Only `disabled` is implemented; this is the free/no-payment path |
| `printer` | `enabled`, `queue_name`, `media_form`, `orientation`, `transform.flip_vertical` | Operator-managed Windows Preview policy. Defaults to disabled/empty queue/empty form/portrait/no flip. If enabled, queue and exact driver DEVMODEW form name are required. Demo sessions remain local-only. Enabling this software path is not physical printer acceptance. |
| root | `schema_version` | Required integer; v1 is migrated in memory to current v2; current schema is `2` |

Relative paths are resolved from the directory containing the config file. Absolute paths are preserved. Windows local-mode checks follow existing junction/symlink targets and reject UNC/network drives; Linux checks the backing mount from /proc/self/mountinfo and rejects known network filesystem types. Load and Save validate resolved roots, and LocalProvider revalidates output directories immediately before writing. See the limitations in the Local-mode network path guard section below.

## Defaults and validation

Missing fields inherit the schema-v2 defaults from `internal/booth/config.Defaults()`. Defaults use the synthetic camera, three shots, a 3-second countdown, one-second inter-shot delay, local media delivery, disabled payment and disabled printer. Both retention values default to `0` (disabled). Printer defaults are `enabled=false`, empty `queue_name`, empty `media_form`, `orientation=portrait` and `transform.flip_vertical=false`. Storage and frame paths are relative and become absolute when loaded from a file.

The loader rejects files over 64 KiB, unknown JSON fields, missing or unsupported schema versions, invalid ranges/durations, malformed JSON and unsupported mode values. Schema-v1 files migrate to v2 in memory; `Load` does not rewrite the source. The next explicit `Save` writes schema v2. For enabled printer configuration, queue name and media form are required; queue/form strings are bounded and reject control characters; media form must fit the 31 UTF-16 code units in DEVMODEW.dmFormName; orientation must be `portrait` or `landscape`. `media_form` is the exact form name accepted by the selected Windows driver, not a friendly alias like “6x4”. On the DS-RX1 virtual queue currently installed on ShinPc, read-only driver/registry inspection found names such as `(6x4)`, `(6x8)` and `(5x7)`; these names are evidence for that virtual driver only and must not be copied to a different physical driver without checking its forms. It does not execute code or fetch remote resources. Credentials and guest photo data do not belong in this config.

Unimplemented modes such as cloud delivery or QR payment fail validation rather than silently falling back to local/free behavior.

## Wails player settings integration

The Wails Windows prototype now reads and writes a station profile through the same root `internal/booth/config` package. Its profile remains at the existing path `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json` for location compatibility; the JSON schema stored in that file is now v2. The first launch installs the embedded sample frame at `%LOCALAPPDATA%\PhotoBooth\frames\photo-strip-v1.json` and initializes local `sessions` and `photos` folders.

The **Settings & frame** screen exposes shot count (1–12), countdown, interval, a local frame JSON path, and independent session/output retention days (0 disables; 1–3650 enables). Browse opens a native local file picker. Saving is disabled while a session/output retry or retention preview/apply runs, validates durations and the selected frame schema, and requires the frame's slot count to match the configured shot count. Saving a retention policy never deletes data; the separate Review old local data panel must build a read-only candidate preview before a one-time, explicit Apply confirmation. Settings persist across restarts; invalid existing config is not silently overwritten, shows a warning, and blocks sessions until the operator saves a valid replacement.

Printer settings remain operator-managed StoreConfig fields and are deliberately not exposed as guest-facing controls on the Wails Settings screen. Saving capture/frame/retention settings preserves the printer policy. In the Windows Preview, real camera sessions use an independent LocalProvider plus DurablePrinter only when an operator explicitly sets `printer.enabled=true`, a Windows queue name and that driver's exact media-form name. Demo sessions are always local-only. Local-output retry uses its dedicated local-only path and can never submit a printer job. The result screen distinguishes a Windows queued job from physical paper output. No automatic print retry, spooler mutation, printer-status polling, or physical-completion claim is added. Printer remains disabled by default. This is prototype software integration, not physical acceptance or production UI certification. It does not add live-preview, arbitrary frame editing, camera-control options, cloud/LAN delivery, payment processing, hot reload, or photo/frame uploads from the UI.

## Current scope

Implemented: strict JSON load/save, safe defaults, validation, relative path resolution, root CLI integration and explicit CLI override precedence; Wails prototype settings for shot count, countdown, interval and frame path; local .pbframe create/validate/install/list; and Wails package validate/install/activate/rollback with local integrity checks and recovery metadata. See [FRAME_PACKAGE_FORMAT.md](FRAME_PACKAGE_FORMAT.md).

Opt-in local session/output retention is implemented in the core and CLI, and surfaced through the Wails Windows Preview Settings UI. CLI cleanup defaults to read-only dry-run and requires a reviewed plan digest plus `--apply`; UI cleanup requires a fully displayed preview, the latest one-use digest and explicit confirmation. Neither path is triggered automatically by the player. See [LOCAL_DATA_RETENTION.md](LOCAL_DATA_RETENTION.md).

Not implemented yet: file-watcher/hot-reload, package signatures/publisher trust, full theme packages, catalog/download sync, arbitrary assets/fonts/animation, LAN/cloud delivery, payment processing, a printer settings UI, physical printer/media acceptance and a policy for locking down config paths on a multi-user PC.


## Restart persistence smoke (2026-11-01)

A new isolated Wails UI smoke, `tools/ui-spike/wails-v2/ui-settings-persist-restart-smoke.ps1`, verifies persistence across two separate PhotoBooth.exe processes. It uses a disposable `PHOTOBOOTH_DATA_DIR`, saves shot count 3, countdown 4s and interval 750ms, closes the first process gracefully, starts a second process against the same local profile, opens Settings, and verifies all three values are reloaded. The smoke passed on 2026-11-01: both processes started and exited normally, the second process showed shot count 3 / countdown 4s / interval 750ms, no photo session or capture was created, and the temporary profile was removed. Report: `%TEMP%\photobooth-settings-restart-6d4b3d0f640f448aa8ec9345a4194462.json`. This validates the original schema-v1 settings screen persistence path in the experimental Wails player; schema-v1 profiles now migrate in memory to schema v2 when loaded. It is not a production-install/update test.

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

## Policy boundaries (2026-11-07)

The schema-v1 values remain unchanged: media_delivery.mode defaults to local and payment.mode defaults to disabled. The root CLI and Wails player now resolve these settings through separate policy boundaries:

- The delivery factory maps local to the existing output.Dispatcher contract. The local provider and its no-overwrite/session-isolation behavior remain unchanged. Unsupported LAN/cloud values fail closed; they do not construct a network listener or silently fall back.
- The payment gate is called by the workflow after frame/config validation but before it claims the session workspace or opens the camera. Disabled/free mode does not make network requests. A future provider adapter receives the stable session ID and whether this is a resume, so permit reuse/idempotency can be implemented without coupling a provider to camera capture.
- The workflow depends on interfaces only. CLI and Wails construct the selected policies from the loaded StoreConfig and pass them to the workflow.
- New schema-v2 session manifests persist media_delivery_mode and payment_mode and include them in the run-config fingerprint. Continue refuses policy drift. Legacy schema-v1 manifests remain readable.

No LAN delivery or payment processing is implemented by this change. QR/paid modes, cloud, network listeners, payment intents, webhooks and provider credentials remain out of scope; unsupported config values continue to be rejected. Stage 2 exit gate remains OPEN. See [ADR 0029](../adr/0029-media-delivery-and-payment-policy-boundaries.md).

## Policy-boundary acceptance follow-up (2026-11-08)

The local/offline policy path now has an entrypoint-level acceptance test in addition to unit tests. A temporary schema-v1 profile with the fake camera, local delivery and disabled payment completes a three-shot CLI run. The test verifies all captures, a local output under the session-scoped output directory, and a finalized schema-v2 manifest containing media_delivery_mode=local and payment_mode=disabled.

Externally supplied profiles selecting LAN, cloud or QR are rejected by the complete CLI entrypoint before it reaches the run/capture path or creates the sessions root. Config Save also rejects these unsupported values rather than persisting a profile that could imply a feature is available. Workflow tests verify the payment gate receives the stable session ID and Resume flag on both new and continued sessions; a policy change to an unsupported mode fails before camera open and preserves the prior manifest unchanged.

The Wails Settings API continues to expose capture/frame/retention fields only. Regression tests ensure saving those fields preserves media-delivery and payment policy values, and that a started session uses the policy snapshot it captured rather than later mutable app state.

These are synthetic/fake-camera and temporary-directory tests. They do not prove Linux mount behavior at runtime, nor do they constitute a packet-capture audit of a deployed station. Only local delivery and disabled/free payment remain supported; Stage 2 remains OPEN.


## Windows local-network exposure audit (2026-11-08)

Added tools/self-service/audit-local-network-exposure.ps1, documented in [NETWORK_EXPOSURE_AUDIT.md](NETWORK_EXPOSURE_AUDIT.md). It reviews local/fixed storage roots, broad NTFS allow principals, overlapping SMB shares and their share ACLs, SMB1/SMB2, LanmanServer, TCP listeners on 445/139, active network categories, effective firewall defaults, named SMB-related inbound rules and current SMB sessions where permissions allow.

ShinPc's actual profile selected local delivery and disabled payment. Both storage roots were distinct directories on fixed local drive C:, with no Everyone/Authenticated Users/BUILTIN Users/Domain Users allow ACE in their returned NTFS ACLs. No ordinary named SMB share overlapped either root. The default C$ administrative share does cover both roots and reports Full access for Administrators, Backup Operators and INTERACTIVE; this remains a WARN for explicit review rather than being treated as a guest-facing PhotoBooth share. SMB1 is disabled; SMB2 and LanmanServer are enabled; listeners exist on 445/139. The active Public firewall profile is enabled with default inbound Block, and no enabled named SMB/File and Printer Sharing/NetBIOS allow rule matching the active profile and ports was found. Current SMB sessions could not be enumerated because the audit identity received Access Denied.

The initial run completed with 0 FAIL / 3 WARN. A follow-up expanded the firewall check to inspect all effective inbound SMB port filters and any Any-port/LanmanServer rules; on the non-elevated account, Windows returned Access Denied and the audit correctly reported this as WARN, producing 0 FAIL / 4 WARN. SMB session enumeration was also denied. This is partial host-local evidence, not proof of remote unreachability or Stage 2 completion. Repeat elevated to review the effective port-filter inventory, administrative-share ACL and sessions, perform a separate trusted-device reachability check, and complete Linux runtime/mount validation. No shares, firewall rules, services, profile settings, session contents or photos were changed or opened.
