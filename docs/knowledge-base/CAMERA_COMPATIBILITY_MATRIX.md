# Camera Compatibility Matrix

**Status:** Partial webcam capture-to-configured-render POC passed; production output and reliability remain untested.
**Last updated:** 2026-10-25

## Status definitions

- **Detected:** OS lists the device; no claim that the app can capture.
- **Stream POC passed:** a test process opened a stream and read consecutive frames. This is not a full product preview or reliability certification.
- **Capture POC passed:** consecutive frames were read and a still-image file was successfully written.
- **Output passed:** a captured image was rendered into a PhotoBooth frame and exported/printed.
- **Sustained passed:** repeated sessions, error recovery, offline behavior, and long-run tests passed.
- **Community reported:** a user supplied a reproducible report, but it was not independently reproduced.
- **Untested:** no evidence yet.
- **Unsupported:** a specific tested combination is known not to work.

## Current matrix

| Camera category/model | OS / version | Backend/provider | Detection | Stream result | Capture result | Render/export | Disconnect/recovery | Overall |
|---|---|---|---|---|---|---|---|---|
| Logitech B525 HD Webcam | Windows 11 Pro 64-bit (10.0.26200) | OpenCV 5.0.0.93 / DirectShow and Media Foundation | Detected by Windows on 2026-10-25 | Both backends read 50/50 frames at 1280x720 | JPEG write passed on both backends; temporary images deleted | JSON-configured 3-photo strip rendered to 1200x1800 JPEG and decoded successfully; no printer test | Concurrent read failed while holder owned device; read recovered after release. Hot-unplug/reconnect untested | Partial POC passed: webcam capture and config-driven file render only |
| Laptop integrated camera | Laptop OS/model not selected | Not selected | Not tested on a laptop | Untested | Untested | Untested | Untested | Untested |
| DSLR/mirrorless reference model | Not selected | Vendor SDK/provider TBD | Untested | Untested | Untested | Untested | Untested | Untested |

## Logitech B525 evidence

Date: 2026-10-25
Machine: Windows 11 Pro 64-bit, version 10.0.26200
Python: 3.12.3 in repository-local virtual environment
OpenCV Python package: 5.0.0.93; cv2.__version__ reports 5.0.0

Observed:
- Windows Plug-and-Play device list includes B525 HD Webcam.
- DirectShow backend at OpenCV camera index 0: 10/10 frames read, 1280x720.
- DirectShow backend at index 0: 50/50 frames read and last frame written successfully to a temporary JPEG, 1280x720. Three additional open/read/save/release cycles of 10 frames each passed.
- Media Foundation backend at index 0: 50/50 frames read and last frame written successfully to a temporary JPEG, 1280x720. Three additional open/read/save/release cycles of 10 frames each passed.
- Temporary images were deleted after collecting file-size/result metadata.
- The POC does not show or visually inspect the saved image; framing, exposure, color, focus, and image quality are not confirmed.
- The JSON-configured three-photo strip was rendered to a 1200x1800 JPEG and read back to verify dimensions. Ten unit tests passed for configuration validation, slot bounds/count, contain/cover fit, and render dimensions. This is a disposable renderer, not the production frame schema.
- Camera-busy test: while one process held the camera through Media Foundation, a second process failed to read its first frame with backend error -1072875772. The POC now reports likely causes and next steps. After the holder released the webcam, a new 10-frame capture passed. This validates low-level behavior, not production UI recovery.

## Remaining B525 checklist

- [x] Confirm OS device detection.
- [x] Read 10 consecutive frames through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through Media Foundation.
- [ ] Visually inspect a locally captured test image for dimensions, orientation, color, focus/exposure, and stale frames.
- [x] Close/reopen device cycles: 3 cycles of 10 frames each passed using DirectShow and 3 cycles using Media Foundation. This is an initial smoke check, not a soak test.
- [x] Test concurrent access: while one MSMF process held the camera, a second capture could not read its first frame; after release, 10-frame capture passed. Production UI/retry handling remains untested.
- [ ] Disconnect/reconnect and recover without restarting the computer.
- [x] Capture three frames, render them into the JSON-configured photo strip, and export a JPEG; output decoded as 1200x1800. This does not validate the production renderer or printer path.
- [ ] Run sustained-session and memory/resource checks.
- [ ] Test a laptop's integrated camera separately.
- [ ] Test Linux on a selected distribution and record the exact backend/provider.

Do not mark a box unless the relevant test was actually executed and its result inspected. Do not commit real guest photos or webcam images as test artifacts; keep them local and delete them after testing.

## Report template

- Test date:
- Tester / app build:
- OS, version, architecture:
- Camera brand/model/firmware:
- Connection/driver/permissions:
- Backend/provider version:
- Formats/resolution/frame rate:
- Test performed:
- Result: pass / fail / partial:
- Reproduction steps:
- Error logs (redacted):
- Limitations:
- Evidence artifact path (avoid storing personal photos):
