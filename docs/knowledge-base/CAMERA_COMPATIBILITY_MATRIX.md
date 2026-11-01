# Camera Compatibility Matrix

**Status (last updated 2026-11-01):** The 100-session same-process Go B525 soak passed (300 captures; 500 JPEGs verified), and headless physical USB unplug/replug recovered the same in-flight capture in 23.906 seconds. On 2026-10-31, one physical USB unplug/replug during an active Wails B525 session also passed: Windows PnP observed unplug/replug, the UI surfaced recovery, and all 3 captures/render/local output completed automatically in the same session without Continue or app restart. The Wails idle live preview also passed one B525 UI smoke: a real image was rendered before capture, the same camera was cleanly handed to the three-shot workflow, and preview resumed after returning to idle. The preview uses the same Media Foundation worker and is capped at 5 FPS with one acknowledged frame in flight. Three real Wails 3-shot sessions at the normal 3s countdown/1s interval passed 9/9 captures and output; first-shot drop was 0.000%, 0.000% and 0.660%. A separate current-scene warm-up A/B diagnostic found a small 0.037 percentage-point reduction in average first-shot drop (0.047% disabled vs 0.010% enabled); the one-second default remains unchanged. These image metrics are not representative portrait/controlled-lighting certification. Repeat physical USB cycles/ports, native reads blocked indefinitely, representative booth-lighting quality, physical printing, laptop camera and Linux remain unvalidated. Worker-level cancelled/deadline read and bounded Close-retry tests pass.

**Additional current-scene quality evidence (2026-11-01):** a fresh Wails/B525 batch passed 3 sessions and 9/9 1280x720 captures/render/local outputs. First-shot drops were 0.017%, 0.110% and 0.231% (mean 0.119%); unchanged gross-darkness/relative-consistency validator passed. This does not replace fixed-lighting portrait/framing visual review. Report: `%TEMP%\photobooth-wails-quality-92009cc35d5444d29f236fa624a50ed9.json`.

## Status definitions

- **Detected:** OS lists the device; no claim that the app can capture.
- **Stream POC passed:** a test process opened a stream and read consecutive frames. This is not a full product preview or reliability certification.
- **Capture POC passed:** consecutive frames were read and a still-image file was successfully written.
- **Output passed:** a captured image was rendered into a PhotoBooth frame and exported/printed.
- **Sustained passed:** a defined repeated-session/recovery suite passed for the stated duration and configuration. This is scoped evidence, not blanket production certification.
- **Community reported:** a user supplied a reproducible report, but it was not independently reproduced.
- **Untested:** no evidence yet.
- **Unsupported:** a specific tested combination is known not to work.

## Current matrix

| Camera category/model | OS / version | Backend/provider | Detection | Stream result | Capture result | Render/export | Disconnect/recovery | Overall |
|---|---|---|---|---|---|---|---|---|
| Logitech B525 HD Webcam | Windows 11 Pro 64-bit (10.0.26200) | OpenCV 5.0.0.93 / DirectShow and Media Foundation | Detected by Windows on 2026-10-25 | Both backends read 50/50 frames at 1280x720 | JPEG write passed on both backends; temporary images deleted | JSON-configured 3-photo strip rendered to 1200x1800 JPEG and decoded successfully; no printer test | Concurrent read failed while holder owned device; read recovered after release. Hot-unplug/reconnect untested | Partial POC passed: webcam capture and config-driven file render only |
| Logitech B525 HD Webcam | Windows 11 Pro 64-bit (10.0.26200) | Go 1.24.8 / Windows Media Foundation / experimental windowsmf.Provider + recovery wrapper | Detected by Windows and selected by friendly name | 100-session single-process soak passed (300/300 frames at 1280x720); one idle live preview smoke also passed through the Wails UI using the same provider worker; physical unplug produced 0xC00D3EA2 | All soak captures/outputs decoded; MJPEG without DHT normalized before JPEG decode | 100 strips and 100 local copies at 1200x1800; all 500 soak artifacts verified | Headless same-process unplug/replug recovered in 23.906s. One active Wails UI unplug/replug also passed: PnP disappearance/return and Wails recovery event observed; in-flight three-shot session auto-completed without Continue or app restart. Repeat cycles/ports still required. | CLI and Wails capture/output passed. 3 Wails quality sessions passed 9/9; first-shot drop 0-0.66%. Warm-up A/B effect was very small in the current scene. Representative portrait lighting, printer and broad compatibility remain unverified; experimental, not production-certified |
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
- The disposable Python/OpenCV JSON renderer made a 1200x1800 strip in the initial POC. The Go schema-v1 renderer and experimental Media Foundation provider passed 100 sequential standalone-process sessions and later 100 sessions in one persistent Go process after native resource-lifecycle fixes; see CAMERA_RELIABILITY_RESULTS.md for timings, OS memory sampling and same-process contention/recovery evidence.
- Camera-busy test: while one process held the camera through Media Foundation, a second process failed to read its first frame with backend error -1072875772. The POC now reports likely causes and next steps. After the holder released the webcam, a new 10-frame capture passed. This validates low-level behavior, not production UI recovery.

## Remaining B525 checklist

- [x] Confirm OS device detection.
- [x] Read 10 consecutive frames through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through DirectShow.
- [x] Read 50 consecutive frames and write a JPEG through Media Foundation.
- [x] Visually inspect one locally captured test image: readable 1280x720, but soft/noisy under current indoor lighting and framed too close/off-center. Temporary file deleted; repeat under controlled lighting and intended camera mounting distance.
- [x] Close/reopen device cycles: 3 cycles of 10 frames each passed using DirectShow and 3 cycles using Media Foundation. This is an initial smoke check, not a soak test.
- [x] Test concurrent access/recovery: a separate OpenCV Media Foundation holder caused the Go player to fail at the first frame with MF_E_HW_MFT_FAILED_START_STREAMING (0xC00D3704); diagnostic text explains likely competing-app/resource causes. After holder release, a new process recovered in the earlier test, and the later persistent Go session runner also completed an explicit recovery session in the same process. The two-trial Wails UI harness subsequently passed same-session Continue and fresh-session restart. Longer GUI soak and repeated physical USB unplug cycles/ports remain untested. The headless recovery wrapper and one active-session Wails UI physical unplug/replug cycle have passed.
- [x] Disconnect/reconnect and recover without restarting the computer: headless same-process B525 capture recovered after attempt 22 reopened the provider; elapsed 23.906 seconds. One active Wails UI cycle also passed on 2026-10-31: PnP observed device disappearance/return and Wails auto-completed the original three-shot session after recovery without Continue or app restart. Repeat cycles and USB-port/device combinations before generalizing.
- [x] Repeated Wails UI recovery and fresh-session restart after camera contention: 2/2 trials in one app process; 2/2 Continue flows retained the same session ID and 2/2 fresh-session flows created new IDs. Separately, one physical USB unplug/replug during an active Wails capture passed on 2026-10-31; the same session auto-recovered and completed 3 captures/render/local output. Longer kiosk soak and repeat hot-plug cycles/ports remain open.
- [x] Historical fresh-process B525 E2E batch: 100 standalone processes completed three 1280x720 captures each and rendered/saved local output. The verifier decoded 500 JPEGs with zero dimension mismatches. Per-session wall time mean 0.919 s, median 0.910 s, p95 0.960 s. Sampled working set mean/max 37.51/41.44 MB; private bytes mean/max 35.35/36.23 MB. The newer same-process run is recorded separately below.
- [x] Historical fresh-process resource sampling: working set mean 37.51 MB, max 41.44 MB; private bytes mean 35.35 MB, max 36.23 MB. This is not a persistent-process measurement; see the 2026-10-27 run above.
- [x] Persistent Go session-runner soak after native resource cleanup: 100/100 sessions passed, 500 JPEGs verified. Wall time mean 800.73 ms, median 800.30 ms, p95 828.80 ms. OS working set mean/peak 37.08/44.33 MB; private bytes mean/peak 33.04/39.94 MB across 813 samples. First-vs-last 10% sample means declined for both memory metrics. This is a headless runner, not the future GUI kiosk.
- [x] Run current-scene first-shot diagnostic: Wails UI 3×3 batch passed 9/9, first-shot drop 0.000%, 0.000% and 0.660%. A 3-pair A/B found mean signed first-shot drop 0.047% without warm-up vs. 0.010% with one-second warm-up (0.037 percentage points); no production setting changed.
- [x] Verify idle live preview and exclusive capture handoff on the reference B525: real preview image rendered before capture; normal three-shot session completed capture/render/local-save; preview restarted after returning to idle.
- [x] Physically unplug/replug the B525 while idle live preview was active on ShinPc / Windows 11 build 26200. PnP observed removal and return of the same instance; backend emitted reconnecting then ready; preview resumed to 48 frames / 48 ACKs. Exact JSONL audit: 48 published = 48 acknowledged, zero cancellations, ACK timeouts, decode failures, duplicates, unmatched sequences or invalid lines. No photo session was started and temporary data was deleted. One cycle on one USB port only; see ADR 0026.
- [ ] Validate idle-preview unplug/replug on the alternate XHC1 path (AMD USB 2.0 controller). Windows confirms the current B525 Location Path differs from the ADR 0026 XHC0 path; preflight passes with -BaselineLocationPath. One XHC0 attempt did not observe PnP removal within 45s; two XHC1 attempts did not observe PnP removal within 90s each, so no alternate-port recovery cycle occurred. This is not a recovery failure or pass; repeat when the USB cable can be physically unplugged during the prompt. See ADR 0027.
- [ ] Verify image quality with a representative portrait/booth scene, fixed lighting, camera placement and visual review before claiming a guest-facing quality pass.
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
