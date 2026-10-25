# Camera Compatibility Matrix

**Status:** 100 consecutive CLI-process B525 capture-render-save sessions passed on Windows 11 (300 captures, 100 rendered strips, 100 local copies, all 500 JPEG artifacts dimension-verified). A competing Media Foundation holder reproduced a resource HRESULT followed by a successful new session after release. Persistent-process memory growth, physical hot-unplug, controlled image quality and printing remain untested.
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
| Logitech B525 HD Webcam | Windows 11 Pro 64-bit (10.0.26200) | Go 1.24.8 / Windows Media Foundation / experimental windowsmf.Provider | Detected and selected by friendly name | 100 consecutive sessions; 300/300 frames at 1280x720 | All captured frames and outputs decoded; abbreviated MJPEG without DHT normalized before Go JPEG decode | 100 strips and 100 local copies at 1200x1800; separate recovery session after contention also passed | Competing OpenCV Media Foundation holder caused failure on shot 1 with HRESULT 0xC00D3704; fresh Go session passed after release. Physical hot-unplug and same-process recovery remain untested | 100-process reliability batch passed; experimental, not production-certified |
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
- One temporary JPEG was visually inspected locally and deleted. It was readable at 1280x720, but the current indoor scene produced a soft/noisy image and very close, off-center framing. Focus/exposure under controlled booth lighting, mount distance, and consistent image quality remain unvalidated.
- The disposable Python/OpenCV JSON renderer made a 1200x1800 strip in the initial POC. The Go schema-v1 renderer and experimental Media Foundation provider passed 100 sequential standalone real-camera sessions; see CAMERA_RELIABILITY_RESULTS.md for per-process timing/memory sampling and contention/recovery evidence.
- Camera-busy test: while one process held the camera through Media Foundation, a second process failed to read its first frame with backend error -1072875772. The POC now reports likely causes and next steps. After the holder released the webcam, a new 10-frame capture passed. This validates low-level behavior, not production UI recovery.

## Remaining B525 checklist

- [x] Confirm OS device detection.
- [x] Read 10 consecutive frames through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through Media Foundation.
- [x] Visually inspect one locally captured test image: readable 1280x720, but soft/noisy under current indoor lighting and framed too close/off-center. Temporary file deleted; repeat under controlled lighting and intended camera mounting distance.
- [x] Close/reopen device cycles: 3 cycles of 10 frames each passed using DirectShow and 3 cycles using Media Foundation. This is an initial smoke check, not a soak test.
- [x] Test concurrent access/recovery: a separate OpenCV Media Foundation holder caused the Go player to fail at the first frame with MF_E_HW_MFT_FAILED_START_STREAMING (0xC00D3704); the error now explains likely competing-app/resource causes. After holder release, a fresh Go session passed. Production UI retry and same-process recovery remain unimplemented.
- [ ] Disconnect/reconnect and recover without restarting the computer.
- [x] Real B525 E2E/reliability batch: 100 consecutive standalone processes completed three 1280x720 captures each, rendered the schema-v1 strip and saved local output. The verifier decoded 500 JPEGs (300 capture frames and 200 rendered/local copies) with zero dimension mismatches. Per-session wall time mean 0.919 s, median 0.910 s, p95 0.960 s. Sampled working set mean/max 37.51/41.44 MB; private bytes mean/max 35.35/36.23 MB. A separate contention probe and fresh-session recovery passed.
- [x] Resource sampling during the 100-process batch: working set mean 37.51 MB, max 41.44 MB; private bytes mean 35.35 MB, max 36.23 MB. This does not measure memory growth in a persistent kiosk process.
- [ ] Run a persistent-process soak, capture peak-memory/GC behavior inside a long-lived kiosk process, and measure thermal/resource growth over time.
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
