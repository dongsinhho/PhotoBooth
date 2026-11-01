# ADR 0013: Bounded camera stream recovery

**Status:** Accepted for experimental Windows player implementation; one physical B525 recovery cycle through the Wails UI passed on 2026-10-31; production certification remains open
**Date:** 2026-10-27

## Context

Physical USB testing of the Logitech B525 on Windows 11 exposed a failure mode that a normal repeated-session soak does not cover. Unplugging the webcam causes the active Media Foundation stream to return HRESULT 0xC00D3EA2 (MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED). Repeated Capture calls on the same provider never recovered, even after Windows re-enumerated the device. Closing the invalid provider and opening a fresh provider restored capture in the same Go process.

The first recovery-wrapper experiment also showed that a short reopen budget could expire while the camera was absent, returning an error before the operator reconnected it. A later capture call could then open a fresh provider, but a one-shot session workflow may already have terminated. The recovery window therefore needs to be long enough to cover normal operator reconnect time while remaining bounded and cancellable.

## Decision

1. Keep recovery outside platform-specific providers in `internal/booth/camera/recovery`; keep the platform-neutral `camera.Provider` contract unchanged apart from a sentinel error category.
2. Providers wrap a failed native stream read with `camera.ErrStreamUnavailable`. Context cancellation/deadline, request validation, JPEG normalization, and file-output errors must not automatically trigger camera reopen.
3. On an eligible stream failure, close the invalid provider before constructing a new one. Remove any stale output for the failed capture request before retrying it.
4. Distinguish initial-open attempts from mid-session recovery attempts. Initial opening fails quickly; recovery uses a separately configurable bounded budget and capped linear backoff so reconnect can occur during the same capture call.
5. The Windows CLI experimental defaults are three initial-open attempts and up to 30 recovery attempts with a 250 ms base delay capped at four increments (approximately 28 seconds of scheduled retry delays, plus native open/close time).
6. Emit recovery events for start, failed open attempt, successful provider open, successful capture recovery, and recovery exhaustion. Event handlers must return promptly.
7. Preserve native cleanup failures and do not create a second provider if the old provider may still own native resources. Honor the caller's context between attempts; do not retry forever.
8. Do not restart/replay the entire photo session automatically after the capture workflow has definitively failed. Production UI/session policy must decide whether to show a reconnect prompt and allow the operator to restart the session safely.

## Evidence

- Repeated calls against the original provider did not recover after a physical USB disconnect/reconnect.
- Manual close/open in the same process recovered on the first provider-open attempt and captured 10/10 frames afterward.
- The initial integrated wrapper recovered in the same process on a subsequent capture call after the first reopen budget expired. This established that creating a new provider works, while exposing the too-short in-flight recovery window.
- Physical validation of the expanded 30-attempt window passed on the Logitech B525. After USB unplug, provider open attempts 1-21 reported that no camera device was available; attempt 22 reopened the provider. The same in-flight call then reported AUTO_RECOVERY_SUCCEEDED and CAPTURE_OK_AFTER_RECOVERY at harness attempt 35, completing in 23.906 seconds without restarting the process.
- On 2026-10-31, a real B525 USB unplug/replug was also exercised while a Wails UI session was active. Windows PnP observed device disappearance and return; the Wails UI surfaced recovery/reconnection; the in-flight session automatically completed all three captures, render and local output without a process restart or Continue action. The disposable profile/photos were deleted. Report: `%TEMP%\photobooth-usb-hotplug-smoke-0b988aaca6fb4c65acd9633f4ee5e849.json`. This is one successful UI cycle for this exact camera/host/backend, not repeated-cycle or production certification.

## Consequences

- The Windows CLI can wait for a temporarily disconnected webcam and retry the same capture request within the configured recovery window.
- If the camera is not available before the bounded recovery budget expires, the capture/session fails with a diagnostic error rather than retrying forever.
- Headless same-process recovery is supplemented by one successful active-session Wails UI USB unplug/replug cycle on the B525. This still does not certify repeated cycles/USB ports, native reads blocked indefinitely inside a driver, other camera models, Linux, or full-day kiosk reliability.
