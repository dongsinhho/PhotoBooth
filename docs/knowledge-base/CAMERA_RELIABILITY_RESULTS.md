# Windows B525 Reliability Results

**Test dates:** 2026-10-25 and 2026-10-27
**Reference device:** Logitech B525 HD Webcam
**Host:** Windows 11 Pro 64-bit, AMD Ryzen 5 8600G
**Provider:** Experimental Go 1.24.8 / Windows Media Foundation adapter
**Status:** 100 fresh-process sessions and 100 same-process sessions passed. A same-process contention/recovery test also passed after native resource-lifecycle fixes. This is strong headless evidence, not production certification.

## Reproducible command

From the repository root, run this in PowerShell:

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\camera-poc\windows-b525-reliability.ps1 -Sessions 100 -Shots 3

This compiles a standalone player, performs the requested real-camera sessions, optionally tests a competing Media Foundation holder, validates JPEG decode/dimensions, and writes a CSV summary in the user's TEMP directory. Test photos and session logs are deleted by default. Adding the KeepArtifacts switch retains them for local inspection and should be avoided with private surroundings or people in the frame.

The harness requires the repository Python virtual environment at tools/camera-poc/.venv with OpenCV installed.

## Results

### 100-session sequential batch

- Result: 100/100 fresh player processes passed; zero failures/timeouts.
- Each session captured 3 frames at 1280x720, rendered one 1200x1800 strip, and saved one 1200x1800 local copy: 300 capture frames, 100 rendered strips, and 100 local copies.
- The verifier decoded 500 JPEGs and found zero dimension mismatches: 300 capture frames at 1280x720 and 200 render/local files at 1200x1800.
- Session wall time: mean 0.919 s, median 0.910 s, p95 0.960 s. The first session took 1.22 s; subsequent sessions were mostly around 0.90-0.97 s.
- The harness sampled each child process every ~25 ms (21-30 samples/session). Sampled working set: mean 37.51 MB, median 37.57 MB, p95 37.96 MB, maximum 41.44 MB. Private bytes: mean 35.35 MB, p95 35.98 MB, maximum 36.23 MB.
- The 500 test photos and per-session logs were deleted after validation; only a CSV summary remained in the user TEMP directory.

### Separate contention and recovery batch

- A separate OpenCV Media Foundation holder held the B525 for 12 seconds. The Go player failed at shot 1 with exit code 1 and HRESULT 0xC00D3704; it created no partial capture output.
- The provider maps this to MF_E_HW_MFT_FAILED_START_STREAMING, suggesting another app owns the camera or the driver lacks streaming resources. It advises closing other camera apps, waiting for release and starting a new session.
- After release, a fresh Go process passed with 3 captures, render and local copy in 0.89 s.
- The verifier decoded all 55 JPEGs from the 11 successful sessions (the separate 10-session batch plus post-contention recovery), with zero dimension mismatches.
- Windows-only unit tests verify the diagnostic name, HRESULT code, actionable troubleshooting step, and preservation of the underlying HRESULT error.

## Persistent single-process soak after native resource cleanup

The earlier process-level measurements suggested native/COM resource retention during repeated camera open/close cycles. The vendored Windows camera wrapper was updated to release enumerated activation objects, temporary media-type handlers and attributes, the CoTaskMemAlloc array from MFEnumDeviceSources, and the Media Foundation/COM DLL handles in shutdown order.

A 100-session run was repeated in **one persistent Go process** after these changes:

- Result: 100/100 sessions passed, with zero capture/render/save failures. The verifier decoded 500 JPEGs: 300 capture frames at 1280x720, 100 rendered strips and 100 local copies at 1200x1800.
- Session wall time: mean 800.73 ms, median 800.30 ms, p95 828.80 ms; minimum 770.63 ms and maximum 850.60 ms.
- Go heap after GC: 255.3 KB after the first session and 301.1 KB after the 100th (45.8 KB net change); first-ten mean 267.4 KB, last-ten mean 300.3 KB. HeapSys was 19.72 MB after the first session and 23.44 MB after the last. Goroutines returned to 1 after every completed session.
- External OS sampling at approximately 100 ms produced 813 samples over 87.39 seconds. Working set averaged 37.08 MB (p95 43.13 MB, max 44.33 MB); private bytes averaged 33.04 MB (p95 38.79 MB, max 39.94 MB). The first 10% of samples averaged 35.59 MB working set and 31.37 MB private bytes; the last 10% averaged 32.31 MB and 28.74 MB respectively. This run did not show upward OS-memory accumulation over its duration.
- The post-change in-use Go heap profile contained runtime allocations (runtime.allocm and timers); it no longer showed windowsmf.captureWorker.run or wrapper methods among the top sampled retained allocations.
- A separate same-process contention run passed three sessions, failed nine attempts with the expected HRESULT while OpenCV held the camera, then succeeded in an explicit recovery session after release. It verified 20 JPEGs across the four successful sessions and returned to one goroutine after recovery.

A separate pre-cleanup 100-session run had substantially higher sampled OS memory (working set mean/peak 68.21/103.27 MB; private bytes mean/peak 64.44/100.22 MB). The before/after reduction is consistent with the resource-lifecycle fixes helping, but these were two separate runs and the individual fixes were not isolated experimentally. Treat this as encouraging host-specific evidence, not a universal memory bound or production certification.

The persistent soak harness writes per-session Go runtime data to CSV and can emit an in-use heap profile. It does not include a production GUI, printer, image delivery, USB unplug/replug, or blocked-native-read cancellation.

## Coarse image diagnostics

The capture verifier records grayscale mean luminance, grayscale standard deviation, Laplacian variance, and the percentage of near-black/near-white pixels. Across the 300 capture JPEGs in the 100-session batch:

- Mean luminance: 127.05 on the 0-255 scale on average; per-image mean range 116.65-140.68.
- Contrast standard deviation: 64.96 on average.
- Laplacian variance: 109.63 on average; per-image mean range 26.67-279.53.
- Near-black/near-white pixel percentage: 0.19% on average.

These values are diagnostic baselines for this indoor scene only. They are not universal targets, a visual sharpness score, or a quality certification. The earlier visual inspection found soft/noisy framing under current indoor lighting. Controlled booth lighting, intended mounting distance, representative people/skin tones, and repeatability across different scenes still need a visual assessment.

## What this proves and what remains open

The headless runner now repeatedly opens/closes the B525, completes capture-render-save, reports a concrete failure under competing camera access, and successfully retries in the same process after the holder exits. This does not establish production UI retry behavior.

This does not prove thermal reliability, a universal peak-RAM bound, or recovery from a blocked synchronous native read. The measured 100-session run lasted about 87 seconds; much longer soak/overnight runs remain open. The same-process recovery was a headless runner session, not an in-process UI retry. Physical USB unplug/reconnect, controlled-lighting image-quality review, laptop integrated camera, Linux camera/runtime and physical printer remain untested.

## Next reliability gate

1. Exercise the eventual full kiosk application (UI, preview, output and recovery) as one long-lived process. The current headless Go runner has passed 100 same-process sessions, but it is not the production kiosk process.
2. Physically disconnect and reconnect USB, record whether the device is re-enumerated, and confirm recovery without rebooting.
3. Review captures under controlled lighting, fixed camera distance and repeatable framing.
4. Test cancellation while a native frame read is blocked on the selected driver.
5. When a persistent kiosk UI exists, verify a failed session can return to an idle/error screen and start a fresh session without restarting the application.
