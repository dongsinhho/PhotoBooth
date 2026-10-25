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

The MVP acceptance gate is still open until real-camera capture, preview/UI, printer/export choice, physical recovery tests, and the target hardware/OS workflow are validated.

## Optional validation questions
- Which camera models/connectivity and printing hardware dominate target stores?
- Does a booth typically run Windows today, and which Linux stack would be acceptable?
- Is LAN photo download expected to be available to every guest or only by session-specific access?
- Which bank/QR payment method and payment confirmation signal are available?
- Does the store want payment before capture, after capture, or either?
- How long must local photos be retained, and who can delete them?
