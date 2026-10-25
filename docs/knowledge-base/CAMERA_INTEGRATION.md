# Camera Integration

**Status:** Experimental Windows Media Foundation UVC provider integrated. A 100-session B525 reliability batch passed (300/300 captures, 100 strips, 100 local copies), and all 500 JPEG artifacts decoded with expected dimensions. A separate competing-camera test reproduced HRESULT 0xC00D3704 and a fresh session passed after release. Persistent-process memory growth, physical unplug/reconnect, and production UI recovery remain unvalidated.

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

Current reference device: Logitech B525 HD Webcam on Windows 11 Pro 64-bit. A disposable OpenCV POC previously read 50/50 frames through both DirectShow and Media Foundation. The Go Media Foundation player then passed 100 consecutive standalone capture-render-save sessions, with 300/300 captures, 100 strips and 100 local copies. The verifier decoded 500 JPEGs at expected dimensions. A separate contention test failed at frame one with MF_E_HW_MFT_FAILED_START_STREAMING (0xC00D3704), and a fresh session passed after release. Coarse image diagnostics were collected but quality is not certified. This does not establish persistent-process memory stability, controlled-lighting quality, physical hot-unplug recovery, printer output, laptop integrated camera or Linux compatibility. See CAMERA_COMPATIBILITY_MATRIX.md and CAMERA_RELIABILITY_RESULTS.md.

1. Enumerate the B525 and determine the actual negotiated formats/resolutions; a 1280x720 frame was observed, but all supported formats have not been enumerated.
2. Implement an app preview without blocking the UI; the current headless POC reads stream frames but does not implement UI preview.
3. Capture a frame to a timestamped image file.
4. Verify actual dimensions, orientation, color, exposure stability and repeated-capture behavior.
5. Another application holding the camera has been tested: contention can fail at the first stream frame with a resource HRESULT, and a new session succeeds after the holder exits. Physical USB disconnect/reconnect and application cancellation during a blocked native read remain untested.
6. The Go renderer passed unit tests and several short real B525 capture-render-save runs. The stream returned abbreviated MJPEG without Huffman-table markers; the adapter restores standard DHT tables when absent. Printer output remains unimplemented.
7. Record OS/backend/version and exact results in CAMERA_COMPATIBILITY_MATRIX.md.

Laptop integrated camera is also an intended use case. The development machine did not expose a built-in laptop camera in the current device listing, so that case requires a separate test on a laptop. Do not mark it verified from the B525 test.

## Experimental Windows Media Foundation provider

The implementation lives in internal/booth/camera/windowsmf and wraps a locally patched windows-camera-go v0.2.0 copy behind camera.Provider. It keeps Media Foundation calls on a locked OS thread. B525 capture extracts frames from an MJPEG stream, not a dedicated still-shutter trigger. Some frames omit DHT Huffman tables; the adapter inserts standard tables only when absent.

On Windows 11 / Go 1.24.8, 100 consecutive standalone sessions each completed three 1280x720 captures, rendering and local save (300 captures, 100 strips, 100 local copies). The verifier decoded all 500 JPEGs at expected dimensions. Mean wall time was 0.919 s, median 0.910 s, and p95 0.960 s. Per-process working set averaged 37.51 MB (max 41.44 MB); private bytes averaged 35.35 MB (max 36.23 MB). A separate contention test held the camera via OpenCV Media Foundation; the Go session failed at shot 1 with HRESULT 0xC00D3704, and a fresh session passed in 0.89 s after release. These are 100 fresh-process session lifecycles, not a persistent-process memory-leak test. Physical USB hot-unplug, cancellation during a blocked native read, controlled lighting and production UI retry/recovery remain untested. See CAMERA_RELIABILITY_RESULTS.md.

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
