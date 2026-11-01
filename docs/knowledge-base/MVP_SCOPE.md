# MVP Scope

**Status:** Proposed; hardware and pilot workflows must be validated

## Goal
Prove reliable capture-to-output on real hardware while laying architecture foundations for configurable media delivery, payment, cross-platform builds, and online content.

## Proposed MVP vertical slice
1. Start a kiosk session using local configuration.
2. Show countdown and capture a configured sequence using the first tested UVC webcam provider (Logitech B525 is the current reference candidate). The built-in laptop camera should be tested as a second UVC target; DSLR remote-still support is a separate later provider.
3. Render the source images into a static frame.
4. Preview, then print or export through at least one tested output path.
5. Preserve local session state and recover from common capture/render/output errors.
6. Complete the core capture/render/output workflow without Internet.
7. Validate one OS/hardware combination end-to-end. Define Linux as a target, then run a separate validation spike on an agreed distro and supported camera.
8. Define configuration models for media delivery and payment even if the initial pilot enables only one mode of each.
9. Make free/no-payment sessions work without a payment provider being configured.

## Acceptance checks
- Expected number of captures is produced, or a clear recovery/error state is shown.
- Camera disconnect/timeout does not permanently freeze the UI.
- Frame layout output dimensions and slot positions are deterministic.
- Output failure is recoverable without restarting the whole machine.
- When multiple output targets are configured, each target is attempted independently; one target failing does not cancel a sibling, and target results are retained separately.
- Bundled/installed assets work after restart and while offline.
- Changing media delivery policy does not require changing capture code.
- Changing payment policy does not require changing camera adapter code.
- Automated tests cover session workflow, render geometry, config validation, and independent payment/media state machines where feasible.
- Hardware-dependent tests explicitly record the exact device, OS, SDK/driver, and result.

## Defer until capture reliability is demonstrated
- Public marketplace and creator payouts.
- Full paid checkout and complex refund automation.
- Broad support for every camera/printer.
- Cloud photo delivery as a mandatory service.
- Arbitrary code execution from third-party themes.
- Multi-store analytics and high-volume distributed architecture.
- GIF/video output unless a validated pilot requires it.

## Current implementation status (2026-10-25)

A development-only offline path is implemented and tested: frame preflight (including slot-count matching), countdown events, capture through a provider, Go JSON frame rendering, phase timings, and a bounded output coordinator with an independent local-storage target and per-target status. Session outcomes distinguish completed, partial_success, and failed. The CLI includes an experimental Windows Media Foundation provider; three consecutive real Logitech B525 3-shot sessions passed through capture-render-save. Synthetic printer-target tests pass, but no physical printer adapter exists yet. User-facing kiosk UI, production recovery, and long-run reliability remain unimplemented.

### Local station configuration implementation (2026-10-29)

The root Go core/CLI now supports a versioned schema-v1 station config with camera/capture/frame/storage sections, local media delivery and disabled/free payment policy. The loader rejects unknown fields, unsupported schema versions, oversized or malformed JSON, invalid ranges/durations and unsupported modes; relative paths resolve from the config file's directory. CLI flags override file values only when explicitly passed. The sample is examples/store-config-v1.json and the contract is documented in STORE_CONFIG.md. The Wails prototype now has a limited operator settings screen backed by the same config package, covering shot count, countdown, interval and frame path. A local .pbframe workflow can validate and install immutable frame versions, activate them with verified-slot-count alignment, and roll back to a preserved frame. This is a narrow declarative frame subset, not a full theme marketplace; package signatures, catalog sync and cloud/payment remain unimplemented. A separate opt-in retention CLI supports session/output retention with dry-run by default and a reviewed plan digest required for deletion; it is not wired into player startup or the Wails settings screen. See LOCAL_DATA_RETENTION.md and ADR 0018. The Wails screen is not a production-ready UI.

### Local output disk-full recovery implementation (2026-10-30; Wails retry integration 2026-10-31)

The local output provider classifies Windows disk-full and non-Windows no-space/quota errors, removes partial temporary output, preserves captures/rendered source and retains typed causes through the output coordinator. The shared output.RetryLocalFromSession API is used by the apps/output-retry command and the Wails result-screen **Retry local save** action. It validates all preserved artifacts, refuses symlinked/overlapping paths and publishes without overwriting an existing photo. Failure injection and retry tests cover directory/temp creation, partial write, sync, close, rename, incomplete sessions, no-overwrite and concurrent retries. Root and Wails tests/builds pass. A Windows UI Automation smoke of the Retry local save button passed with three synthetic captures and a controlled output-path obstruction; the disposable test profile was cleaned. A real full-volume/quota test has not been run. See LOCAL_STORAGE_RECOVERY.md and ADR 0019.

The MVP acceptance gate remains open. Real B525 Wails capture/render/local-save and one active-session physical USB unplug/replug recovery cycle have passed on Windows 11; the same session auto-completed after reconnect. The Wails Retry local save UI smoke also passed with a controlled output-path failure. A current-scene Wails quality batch and warm-up A/B diagnostic passed their gross-darkness/relative-consistency gates, but are not representative portrait certification. A further 2026-11-01 Wails batch passed 3/3 sessions and 9/9 1280x720 captures/render/local outputs; mean signed first-shot drop was 0.119% across the three sessions. This confirms current-scene consistency only; controlled portrait lighting/framing and visual inspection remain open. The Wails prototype now includes an idle-only live preview using the same Media Foundation worker as capture; one physical B525 UI smoke verified preview, exclusive capture handoff and preview restart after returning to idle. This does not certify a production UI or full-day preview operation. Remaining gates include production UI selection/full kiosk soak, representative fixed-lighting portrait quality, repeat physical recovery cycles/ports, a real disk-full/quota test, a named physical printer or export workflow, laptop-camera coverage, and the agreed target hardware/OS workflow.

## Optional validation questions
- Which camera models/connectivity and printing hardware dominate target stores?
- Does a booth typically run Windows today, and which Linux stack would be acceptable?
- Is LAN photo download expected to be available to every guest or only by session-specific access?
- Which bank/QR payment method and payment confirmation signal are available?
- Does the store want payment before capture, after capture, or either?
- How long must local photos be retained, and who can delete them?
