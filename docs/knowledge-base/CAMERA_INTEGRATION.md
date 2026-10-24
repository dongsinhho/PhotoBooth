# Camera Integration

**Status:** Required architecture boundary accepted; provider implementations and compatibility matrix are not yet validated.

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
- Capture result with timestamp, media path/reference, actual dimensions, pixel format and diagnostics.
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

Current reference device: Logitech B525 HD Webcam on Windows 11 Pro 64-bit. OS detection and a disposable OpenCV smoke test have passed: DirectShow and Media Foundation each read 50/50 frames at 1280x720, and each backend wrote a temporary JPEG successfully. A separate disposable sample-layout script then rendered a captured 1280x720 source into a 1200x1800 JPEG, which decoded successfully. This does not confirm visual quality, the production frame schema/renderer, printer output, device-loss recovery, sustained operation, or Linux compatibility. See CAMERA_COMPATIBILITY_MATRIX.md for exact evidence.

1. Enumerate the B525 and determine the actual negotiated formats/resolutions; a 1280x720 frame was observed, but all supported formats have not been enumerated.
2. Implement an app preview without blocking the UI; the current headless POC reads stream frames but does not implement UI preview.
3. Capture a frame to a timestamped image file.
4. Verify actual dimensions, orientation, color, exposure stability and repeated-capture behavior.
5. Test disconnect/reconnect, another application holding the camera, and app cancellation.
6. A disposable sample-layout render/export has passed for one captured file; the production render pipeline and printer output remain future tests.
7. Record OS/backend/version and exact results in CAMERA_COMPATIBILITY_MATRIX.md.

Laptop integrated camera is also an intended use case. The development machine did not expose a built-in laptop camera in the current device listing, so that case requires a separate test on a laptop. Do not mark it verified from the B525 test.

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
