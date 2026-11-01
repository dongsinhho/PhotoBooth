# ADR 0020: Idle-only camera preview serialized with capture

**Status:** Accepted for the experimental Windows Wails player; one real Logitech B525 UI preview/capture/restart smoke passed on 2026-10-31. This is not a production UI or broad device-support decision.
**Date:** 2026-10-31

## Context

The Wails prototype previously showed only the latest captured still on the capture screen. A continuous camera preview is needed on the idle/ready screen so an operator or guest can frame the shot before starting a session.

The Windows Media Foundation provider currently owns a single capture worker and one active native stream. Opening a second camera/provider for preview could contend with the capture provider, produce an exclusive-device error, or complicate unplug/reconnect recovery. Preview must not compete with the session runner for camera reads, delay capture, or accumulate old frames when the WebView is busy.

## Decision

1. Add an optional `camera.PreviewProvider` extension. Keep the core `camera.Provider` contract unchanged for providers that do not support preview.
2. Windows Media Foundation preview reads from the exact same `captureWorker` and negotiated MJPEG stream used by capture. Frame normalization, size checks, and dimension validation are shared. A provider-level mutex serializes `Preview` and `Capture`; no second native reader or stream is opened.
3. In the Wails prototype, run preview only while the player is idle. It is stopped and the provider is closed before a real or demo session, Settings, or application shutdown can proceed. Capture begins only after the stop barrier confirms the preview worker exited. If close fails or the native read does not return within the shutdown budget, fail closed and refuse to open another provider. A context cancellation during synchronous Media Foundation startup must not return while cleanup runs in an untracked goroutine: `windowsBackend.Open` waits for the startup result and closes the worker before returning, or returns a cleanup error while retaining the worker reference so Close can retry.
4. The recovery decorator exposes optional preview and applies the existing bounded close/reopen policy to stream-invalidated preview reads. Cancellation and non-stream errors do not trigger recovery.
5. Publish at most one preview frame at a time. The Wails event carries a sequence and JPEG data URL; the frontend acknowledges only after image decode/load succeeds. Decode errors are not acknowledged. The backend waits for that exact acknowledgement before reading the next frame. Duplicate, stale and already-consumed ACKs are rejected. A 200 ms minimum inter-frame delay caps publication at 5 FPS; a 2-second acknowledgement timeout stops preview if the WebView stops responding.
6. Keep preview local. Frames remain in memory/event transport and are not written to the session directory or finalized output. The UI labels the live image as local-only.
7. Stop preview when opening operator Settings or starting a session. Restart it when the UI returns to idle. If preview startup or recovery fails, show an actionable unavailable/reconnect state and allow an explicit retry.

## Consequences

- The reference B525 preview can reuse its active Media Foundation stream without contending with session capture.
- Capture has priority because preview is fully stopped and closed before the workflow opens the camera. Preview and capture are not simultaneous in this implementation.
- Slow UI rendering cannot create an unbounded queue: there is at most one unacknowledged frame, and a stalled UI stops preview.
- Idle preview keeps the webcam active/streaming and may keep its camera indicator light on. The UI visibly identifies the preview and does not upload or persist frames.
- Preview support is currently Windows Media Foundation only. The Wails UI prototype is not selected as the production UI, and laptop cameras, other webcam models, Linux, and long-running kiosk behavior are not validated.

## Validation

- Root Go `go test -count=1 ./...`, `go vet ./...`, Windows `go build ./...`, and Linux/amd64 CGO-disabled compile-only build passed on 2026-10-31.
- Wails Go `go test -count=1 ./...`, `go vet ./...`, and `wails build -clean -nopackage` passed; Wails regenerated the preview method bindings and compiled the Vite frontend.
- PowerShell UI smoke: `tools/ui-spike/wails-v2/ui-camera-preview-smoke.ps1` passed on the physical Logitech B525 / Windows 11 host. It observed a real idle preview before capture, started the three-shot Wails session while preview was active, verified all three captures, render and local output completed (demonstrating exclusive handoff), returned to idle and observed the live preview again.
- Latest report after all ACK, startup-cleanup and frontend decode-failure handling changes: `%TEMP%\photobooth-wails-preview-713d7c864cf1438cb4ebd1a21397b379.json`. Session `f3d6a96b74a68168ea896435e3997b41`. `preview_before_capture=true`, `preview_after_capture=true`, `capture_count=3`, `rendered=true`, `local_output=true`, `temp_data_cleaned=true`. The isolated test profile and test photos were deleted.
- Unit tests cover preview recovery after stream invalidation, no recovery for cancellation/non-stream errors, exact frame acknowledgement, stale/duplicate acknowledgement rejection, fail-closed handling of native close errors, and cancellation during native startup waiting for worker cleanup before `Open` returns.
- On 2026-11-01, one physical Logitech B525 unplug/replug while idle preview was active passed on ShinPc / Windows 11 build 26200. PnP observed removal and return of the same instance; the backend emitted reconnecting then ready; preview advanced from 11 baseline frames/ACKs to 48/48 after reconnect. The exact JSONL audit found 48 publications = 48 ACKs and no timeout, decode failure, duplicate, unmatched sequence or invalid line. No photo session was started; the app exited gracefully and the disposable profile was removed. This is one camera/port/host cycle only; see [ADR 0026](0026-idle-preview-physical-usb-recovery-validation.md).

## Open validation

- Sustained visible preview over a full booth operating window and measured CPU/memory on target hardware.
- Repeat physical idle-preview USB unplug/replug over additional cycles; test another USB port and camera model before generalizing compatibility. One physical B525 cycle on the reference port passed; see ADR 0026.
- Visual review of representative portrait framing and lighting, including how preview orientation/crop maps to the final photo.
- Linux V4L2 provider and preview support, if Linux becomes an accepted target.
