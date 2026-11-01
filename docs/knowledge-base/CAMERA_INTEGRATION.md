# Camera Integration

**Status (2026-10-31):** Experimental Windows Media Foundation UVC provider integrated. A 100-session single-process B525 run passed (300/300 captures); all 500 JPEG artifacts decoded with expected dimensions. Same-process contention recovery passed. Physical USB hot-unplug/replug passed at the headless recovery-wrapper level (provider reopened on attempt 22; in-flight capture recovered in 23.906 seconds). A real Wails UI active-session USB unplug/replug test also passed one physical cycle: Windows PnP saw the device disappear and return, Wails surfaced recovery, and the same in-flight three-shot session automatically completed capture/render/local-save without Continue or app restart. The idle-only Wails live preview now uses the same Media Foundation worker as capture, with serialized reads, a one-frame acknowledgement/backpressure gate and a 5 FPS maximum; a real B525 UI smoke passed preview-before-capture, complete capture/render/local-save handoff, and preview-after-capture. A separate three-session Wails image-quality batch completed 9/9 captures at 1280x720; first-shot drop was 0.000%, 0.000% and 0.660%. A small real-camera A/B diagnostic compared warm-up disabled vs. one-second warm-up and measured only a 0.037 percentage-point reduction in mean first-shot drop (0.047% to 0.010%); keep the one-second default and do not infer representative portrait quality from this scene. Production UI selection, full kiosk soak, repeated Wails unplug cycles/USB ports, native reads blocked indefinitely inside a driver, representative fixed-lighting portrait quality, printer output, laptop camera and Linux compatibility remain unvalidated. Worker-level cancellation/deadline simulation and bounded Close retry are covered by passing Windows tests.

## Goal

Reuse one session workflow across multiple camera types while allowing each provider to expose only the capabilities it actually supports. Start with cameras available to the project, without hard-coding the product around webcams forever.

## Camera categories

### 1. UVC webcam / laptop integrated camera

Examples: external Logitech USB webcams and built-in laptop cameras that are exposed by the operating system as supported video-capture devices.

Typical workflow:
1. Enumerate camera devices.
2. Select a device and a supported stream format/resolution/frame rate.
3. Start preview if the UI needs it.
4. Capture a still output by freezing/extracting a frame from the video stream.
5. Save the frame to a local file and pass it to the render pipeline.

Important distinction: this workflow captures a frame from a video stream; it does not necessarily trigger a dedicated still-photo exposure or download a full-resolution original like a DSLR SDK might. It may inherit stream resolution, compression, exposure behavior and auto-white-balance limits.

### 2. DSLR / mirrorless still-camera provider

A future provider may expose:
- Remote still-shutter trigger.
- Transfer/download of the original still image.
- Live view (if supported).
- Camera settings such as shutter, aperture, ISO, focus and white balance (only where the model/SDK supports them).
- Battery/storage status and device diagnostics where available.

Do not assume every camera supports every feature. Vendor SDK, firmware, OS, driver, USB/Wi-Fi mode and licensing restrictions must be tested by exact model.

### 3. Future device categories

Phone cameras, capture cards, HDMI cameras or network cameras may be considered later if there is evidence of demand. They must implement an appropriate provider, not be represented as fully equivalent to UVC or DSLR capture.

## Provider contracts

Prefer a small provider contract plus explicit capability metadata instead of one large interface that pretends every device can do everything.

Core concepts to model:
- EnumerateDevices / SelectDevice.
- Open / Close / Reconnect.
- Capabilities and supported capture modes.
- Optional preview stream.
- Capture requests carry a sequence number, requested dimensions, and explicit local output path; results carry sequence, file path, and actual dimensions. The Windows MF adapter reports stream-frame capture capability, not a dedicated still-shutter capability.
- Cancel/timeouts and stable error categories.
- Device/provider/firmware/OS identifiers for supportability.

Capability examples:
- FrameCaptureFromStream.
- TriggerStillCapture.
- DownloadOriginalStill.
- LivePreview.
- SetResolution / SetFrameRate.
- SetExposure / SetWhiteBalance / SetFocus.
- ReadBatteryOrStorageStatus.

Only report a capability when the provider detects it or the exact combination has been validated. Missing optional capabilities should be exposed as unavailable, not silently ignored.

## Platform backends

For standard UVC cameras:
- Windows: investigate the native Media Foundation capture stack.
- Linux: investigate Video4Linux2 (V4L2), including device permissions and capture format negotiation.
- A cross-platform library (for example OpenCV) may accelerate a proof of concept, but backend/property behavior differs. Do not assume a generic API exposes the same controls or reliability on every OS.

Keep native or library-specific types behind provider boundaries. The domain/session orchestrator must not depend directly on Media Foundation, V4L2, OpenCV, or a vendor SDK.

## First technical spike

Current reference device: Logitech B525 HD Webcam on Windows 11 Pro 64-bit. A disposable OpenCV POC previously read 50/50 frames through DirectShow and Media Foundation. After native resource-lifecycle cleanup in the vendored wrapper, the Go Media Foundation player passed 100 consecutive sessions in one Go process: 300/300 captures, 100 strips and 100 local copies. The verifier decoded 500 JPEGs at expected dimensions. A separate same-process contention test failed at frame one with MF_E_HW_MFT_FAILED_START_STREAMING (0xC00D3704) and then recovered after release. Post-fix OS sampling during the 100-session run did not show rising memory across time. The 100-session run is headless CLI/harness evidence, not GUI kiosk certification. Physical USB hot-unplug/replug passes through the recovery wrapper in a same-process harness; see CAMERA_RELIABILITY_RESULTS.md. The Wails UI has passed real capture and a two-trial recovery/fresh-session suite. Windows worker-level tests simulate cancellation/deadline during a blocked read and bounded Close retry, but real Media Foundation/COM read interruption after USB loss, representative booth-lighting quality, printer output, laptop camera and Linux compatibility remain untested. See CAMERA_COMPATIBILITY_MATRIX.md and CAMERA_RELIABILITY_RESULTS.md.

1. Enumerate the B525 and determine the actual negotiated formats/resolutions; a 1280x720 frame was observed, but all supported formats have not been enumerated.
2. Implement an app preview without blocking the UI; the current headless POC reads stream frames but does not implement UI preview.
3. Capture a frame to a timestamped image file.
4. Verify actual dimensions, orientation, color, exposure stability and repeated-capture behavior.
5. Another application holding the camera has been tested: contention can fail at the first stream frame with a resource HRESULT, and a new session succeeds after the holder exits. A physical USB disconnect/reconnect passes in the headless recovery harness, which closes the invalid provider and reopens it in the same process. A two-trial Wails UI harness passed same-session Continue and fresh-session restart after initial contention, and a post-build rerun passed both flows again in one trial. Windows worker-level tests simulate cancellation/deadline while a read is blocked and verify bounded Close plus successful retry after the read returns. One physical USB unplug/replug during an active Wails capture has now passed on the B525: PnP disappearance/return was observed, Wails surfaced recovery, and the same three-shot session auto-completed. Repeat cycles/ports and behavior when a native Media Foundation/COM read never returns remain untested; this one cycle is not a long-duration kiosk soak.
6. The Go renderer passed unit tests and several short real B525 capture-render-save runs. The stream returned abbreviated MJPEG without Huffman-table markers; the adapter restores standard DHT tables when absent. Printer output remains unimplemented.
7. Record OS/backend/version and exact results in CAMERA_COMPATIBILITY_MATRIX.md.

Laptop integrated camera is also an intended use case. The development machine did not expose a built-in laptop camera in the current device listing, so that case requires a separate test on a laptop. Do not mark it verified from the B525 test.

## Experimental Windows Media Foundation provider

The implementation lives in internal/booth/camera/windowsmf and wraps a locally patched windows-camera-go v0.2.0 copy behind camera.Provider. It keeps Media Foundation calls on a locked OS thread. B525 capture extracts frames from an MJPEG stream, not a dedicated still-shutter trigger. Some frames omit DHT Huffman tables; the adapter inserts standard tables only when absent.

### Initial stream warm-up

After `StartCapture`, the provider consumes and discards approximately one second of stream frames before reporting the camera open. At the default negotiated 30 FPS this is 30 frames; the count is derived from the selected stream's FPS and warm-up duration, never below one frame, and capped at 120 frames. The duration is defaulted to one second and capped at three seconds. This gives UVC auto-exposure/white-balance a chance to settle before the countdown and first saved photo. It does not alter captured pixel values, and the warm-up applies again whenever recovery opens a fresh provider. A stream-read error during warm-up fails that open attempt rather than silently starting capture on an unverified stream. The implementation is unit-tested. On 2026-10-31, a current-scene A/B diagnostic compared three 3-shot B525 sessions with warm-up disabled against three with the one-second default enabled: mean signed first-shot drop changed from 0.047% to 0.010% (0.037 percentage points). Three real Wails sessions at the normal 3s countdown/1s interval also showed first-shot drops of 0.000%, 0.000% and 0.660%. This is a very small current-scene effect, not a fixed-lighting before/after portrait test. Keep the one-second default; representative booth lighting and intended mount distance still require visual assessment.

On Windows 11 / Go 1.24.8, the earlier 100 fresh-process sessions each completed three 1280x720 captures, render and local save (300 captures, 100 strips, 100 local copies). The later same-process soak and post-fix memory comparison are documented in CAMERA_RELIABILITY_RESULTS.md; those measurements are now the main evidence for session-runner lifecycle testing.

## Idle-only live camera preview

The optional `camera.PreviewProvider` extension lets a platform provider return one compressed frame from its active stream without writing a capture artifact. The Windows Media Foundation implementation reads from the same `captureWorker` as `Capture`, through a shared frame-normalization/validation path and the provider's operation mutex. It does not create a second native reader.

The experimental Wails UI starts preview only while the player is idle. It cancels and closes the preview provider before starting a real or demo session, opening operator settings, or shutting down. Capture is allowed to proceed only after the preview stop barrier confirms that the preview goroutine exited; if native close/read cleanup fails or exceeds its 7-second wait budget, the app fails closed rather than opening a second provider. A context cancellation during native Media Foundation startup waits for the startup result and synchronous worker cleanup before `Open` returns. If the native startup/read never returns, the Wails stop barrier times out and refuses capture rather than trusting an untracked cleanup goroutine. Returning to idle restarts preview.

Preview publication is bounded: maximum 5 frames/second, one JPEG frame in flight, and the next frame is not read/published until the WebView acknowledges that the current image loaded/decoded successfully. A decode error is not acknowledged; the 2-second acknowledgement timeout stops preview rather than building an unbounded queue. Stale/duplicate ACKs are rejected. Frames are kept in memory/event transport and are not persisted or uploaded. The UI exposes an explicit retry state for unavailable/stalled preview.

On 2026-10-31, the physical B525 Wails UI smoke passed: a live preview rendered before capture, Start handed exclusive camera ownership to the normal three-shot workflow, all 3 captures/render/local output succeeded, and live preview appeared again after returning to idle. Latest report after lifecycle hardening: `%TEMP%\photobooth-wails-preview-713d7c864cf1438cb4ebd1a21397b379.json` (session `f3d6a96b74a68168ea896435e3997b41`). This is one Windows 11/B525/Wails prototype validation, not a continuous full-day kiosk soak or production UI certification. Other providers/OSes do not yet implement preview.

## Recovery wrapper

The experimental cross-provider recovery decorator lives in `internal/booth/camera/recovery` and is used by the Windows Media Foundation booth-player path.

- A provider wraps native frame-read failures with `camera.ErrStreamUnavailable`; caller cancellation/deadline and post-read validation/output errors do not trigger reopen.
- Recovery closes the invalid provider, removes a stale/partial output at the failed request path, constructs a fresh provider, and retries the same capture request.
- Initial open defaults to three attempts. The Windows CLI allows up to 30 recovery-open attempts with a 250 ms base delay and a 1-second delay cap (about 27.5 seconds of scheduled retry delays, plus native open/close time).
- Each attempt is observable through recovery events. If the device does not return before the recovery budget expires, the active capture/session returns a clear failure; retry is bounded, not infinite.
- Manual hardware harness: `tools/camera-poc/hotplug-reopen-test/README.md`. The expanded-window harness verified that the original in-flight capture resumes after replug: attempts 1-21 could not find the camera; attempt 22 reopened it and the capture succeeded after 23.906 seconds. A two-trial Wails UI suite also passed same-session Continue and fresh-session restart after initial contention. Longer GUI soak, other camera models and native calls blocked indefinitely remain separate validation gates.

## Version and update model

Initially ship camera providers as part of a tested application release rather than loading arbitrary native plugins. A future provider update must be staged, integrity-checked, compatible with the player version, and safely rolled back. Do not update a camera provider during a photo session.

## Compatibility claims

- A device appearing in the OS device list is not proof that capture-to-output works.
- A stream working in a video-call app is not proof of sustained kiosk reliability.
- A UVC device's frame-capture capability is not the same as DSLR remote shutter/original-file support.
- Publish only tested OS/device/backend combinations and state which optional capabilities are unavailable.

## Reference links

- Logitech UVC device guidance: https://support.logi.com/hc/en-001/articles/360023263194-Install-native-UVC-drivers-for-your-Logitech-webcam
- Logitech C925e/C930e/B525 family compatibility note: https://hub.sync.logitech.com/c925e/post/compatibility-and-system-requirements-dndF6e9Q3kDqO6t
- Microsoft Media Foundation video capture: https://learn.microsoft.com/en-us/windows/win32/medfound/video-capture
- Microsoft USB Video Class driver overview: https://learn.microsoft.com/en-us/windows-hardware/drivers/stream/usb-video-class-driver-overview
- Linux V4L2 API: https://docs.kernel.org/userspace-api/media/v4l/v4l2.html
- OpenCV video I/O overview: https://docs.opencv.org/4.x/d0/da7/videoio_overview.html
