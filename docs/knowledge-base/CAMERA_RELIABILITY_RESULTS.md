# Windows B525 Reliability Results

**Test date:** 2026-10-25
**Reference device:** Logitech B525 HD Webcam
**Host:** Windows 11 Pro 64-bit, AMD Ryzen 5 8600G
**Provider:** Experimental Go 1.24.8 / Windows Media Foundation adapter
**Status:** 100 sequential CLI-process sessions and a competing-camera recovery test passed. Not a same-process soak or production certification.

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

## Coarse image diagnostics

The capture verifier records grayscale mean luminance, grayscale standard deviation, Laplacian variance, and the percentage of near-black/near-white pixels. Across the 300 capture JPEGs in the 100-session batch:

- Mean luminance: 127.05 on the 0-255 scale on average; per-image mean range 116.65-140.68.
- Contrast standard deviation: 64.96 on average.
- Laplacian variance: 109.63 on average; per-image mean range 26.67-279.53.
- Near-black/near-white pixel percentage: 0.19% on average.

These values are diagnostic baselines for this indoor scene only. They are not universal targets, a visual sharpness score, or a quality certification. The earlier visual inspection found soft/noisy framing under current indoor lighting. Controlled booth lighting, intended mounting distance, representative people/skin tones, and repeatability across different scenes still need a visual assessment.

## What this proves and what remains open

This is stronger than the earlier three-session smoke test: the adapter repeatedly opens and closes the B525, completes capture-render-save, reports a concrete failure under competing camera access, and works again in a new process after the holder exits.

It does not prove a long-duration soak, bounded peak RAM, thermal reliability, or recovery from a blocked synchronous native read. The current command-line runner exits when a session fails; the successful recovery was a fresh session process, not an in-process UI retry. Physical USB unplug/reconnect was not performed. A controlled-lighting image-quality review, built-in laptop camera, Linux camera/runtime and physical printer remain untested.

## Next reliability gate

1. Exercise a persistent kiosk process for repeated sessions and sample its working set/heap over time; the 100-session batch launches a new process each time.
2. Physically disconnect and reconnect USB, record whether the device is re-enumerated, and confirm recovery without rebooting.
3. Review captures under controlled lighting, fixed camera distance and repeatable framing.
4. Test cancellation while a native frame read is blocked on the selected driver.
5. When a persistent kiosk UI exists, verify a failed session can return to an idle/error screen and start a fresh session without restarting the application.
