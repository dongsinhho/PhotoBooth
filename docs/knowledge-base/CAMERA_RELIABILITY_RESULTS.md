# Windows B525 Reliability Results

**Test dates:** 2026-10-25, 2026-10-27, 2026-10-28 and 2026-10-29
**Reference device:** Logitech B525 HD Webcam
**Host:** Windows 11 Pro 64-bit, AMD Ryzen 5 8600G
**Provider:** Experimental Go 1.24.8 / Windows Media Foundation adapter
**Status:** 100 fresh-process sessions, 100 same-process sessions, and a 1,000-session persistent same-process endurance run passed. Same-process camera-contention recovery and physical USB unplug/replug recovery have also passed with the headless wrapper. This is strong experimental evidence, not production certification.

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

### 1,000-session persistent endurance run (2026-10-29)

- **Result: PASS** in one persistent Go process on the B525: 1,000/1,000 sessions completed; zero session failures; 3 captures/session.
- Artifact verifier decoded all **5,000/5,000 JPEGs**: 3,000 1280x720 captures, 1,000 1200x1800 rendered strips and 1,000 1200x1800 local copies. No decode or dimension failures.
- Runtime: 2,972.17 seconds (49.54 minutes). Session wall-time mean 2,900.7 ms; p50 2,901.229 ms; p95 2,914.511 ms. The current one-second camera warm-up is included in each session.
- Go heap allocation after forced GC grew from 0.255 MB after session 1 to 0.381 MB after session 1,000 (+0.126 MB); the maximum observed post-GC heap was 0.381 MB. The number of Go goroutines after GC remained exactly 1 for every session.
- Windows process sampling collected 11,241 samples. Working Set peak was 41.93 MB and Private Bytes peak 42.52 MB. First-decile averages were 28.71/28.79 MB (Working Set/Private Bytes); last-decile averages were 31.61/31.99 MB, an increase of about 2.90/3.20 MB. Memory oscillated during the run rather than rising monotonically; however, this is not enough to prove that all native/driver allocations are leak-free.
- Handle count was variable (peak 523) and did not rise monotonically through the run; threads peaked at 27. Repeat on the actual booth PC/USB topology before setting production memory bounds.
- Test artifacts containing real camera frames were removed after validation. The TEMP report retains the Go metrics CSV, Windows process-memory CSV, stdout/stderr logs and heap profile. Summary: `C:\\Users\\sinhv\\AppData\\Local\\Temp\\photobooth-endurance-20261029-095355\\summary.json`; report: `C:\\Users\\sinhv\\AppData\\Local\\Temp\\photobooth-endurance-20261029-095355\\report.md`.
- The Go soak itself emitted `failed=0` and `verified_jpegs=5000`. The first run of the surrounding PowerShell harness returned exit code 1 after the PASS due to calling `WaitForExit()` on a disposed `Process` object in the finalizer. That wrapper bug has been fixed and the harness is now testable with a one-session smoke run. This finalization error did not alter the Go soak result or artifact verification.

**Scope:** this is a headless persistent camera → capture → render → local-save soak. It does not test the Wails UI, a printer, camera contention recovery, or physical USB unplug during active capture; those remain separate reliability gates.

## Physical USB hot-unplug/replug recovery (2026-10-27 to 2026-10-28)

Test device: Logitech B525 HD Webcam on Windows 11 Pro 64-bit, using the Go Windows Media Foundation provider.

Observed manual runs:
- A 120-attempt harness was unplugged after the first 83 successful captures. Subsequent reads returned the device-invalidated HRESULT; the process remained alive and closed cleanly.
- A persistent-provider run accumulated 43 successful captures before unplug, then continued to receive errors after replug. Windows PnP later reported the B525 as present, but the original Media Foundation session did not recover. It ended with 557 failed calls and a clean Close().
- A manual teardown/reopen experiment detected disconnect, waited for the operator to reconnect, closed the old provider (nil), opened a new provider on the first attempt in the same Go process, and completed 10/10 post-reopen captures. No process restart was required.
- The initial integrated recovery-wrapper harness also detected MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED (0xC00D3EA2), closed the invalid provider and tried reopening. When the first bounded reopen window expired while the device was absent, a later capture call successfully reopened and captured again in the same process (595 successes, 5 failed calls, clean final close). This exposed a workflow edge: the active session can fail if the camera remains absent until the short recovery budget expires.
- The recovery wrapper has separate initial-open and recovery-attempt budgets. The Windows CLI and manual harness are configured for up to 30 reconnect attempts with capped linear backoff (approximately 28 seconds of scheduled retry delays, plus native open/close time). A later 600/600 run confirmed steady-state capture. The decisive physical test then passed with the extended window: after unplug, open attempts 1-21 reported no camera device; attempt 22 reopened the provider. The same in-flight Capture call emitted AUTO_RECOVERY_SUCCEEDED and CAPTURE_OK_AFTER_RECOVERY at harness attempt 35, with elapsed time 23.906 seconds. The process was not restarted.

The provider maps the observed disconnect HRESULT to MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED and classifies failed native stream reads as camera.ErrStreamUnavailable. A missing device is not retried forever: cancellation remains responsive between open attempts, and exhausting the configured recovery budget returns a clear error. The production GUI/session recovery policy still needs validation.

## Blocked-read cancellation and bounded worker shutdown (2026-10-29)

The Windows Media Foundation capture worker now uses the extracted `serveCaptureCommands` loop with a buffered reply per command. If the caller's context is cancelled or expires while the synchronous native read is still running, `nextFrame` can return promptly without requiring the native call to be interruptible. If `Close` cannot send its shutdown command or finish resource release within the configured timeout, it returns a descriptive timeout and retains the worker reference so a caller can retry `Close` rather than opening another provider over live native resources. The timeout used in the diagnostic now matches the configured timeout value.

Two Windows-only regression tests simulate a native read blocked on a channel: one cancels the capture context, and the other lets a deadline expire. Both verify the caller returns the right context error, first `Close` fails with the bounded timeout while the read remains blocked, and a second `Close` succeeds once the simulated read is released. The targeted Windows camera package passed `go test -count=20 ./internal/booth/camera/windowsmf`; no worker goroutine remained after each simulated test.

**Limit:** these tests exercise the worker command/cancellation contract with a deliberately blocking test read; they do not prove a real Media Foundation driver or COM call will eventually unblock after USB removal. Physical USB unplug during active capture and behaviour of a native read that never returns remain hardware/driver validation gates.

## First-photo exposure stabilization (2026-10-28)

The UI/camera workflow was observed to sometimes produce a first photo that is slightly darker than subsequent shots. The Windows Media Foundation provider now discards a bounded one-second warm-up stream before the camera is considered ready, so auto-exposure and white-balance can settle before the guest countdown and first saved photo. The warm-up is also performed on fresh providers created by the recovery wrapper. This change is covered by Windows-only unit tests for frame-count calculation, bounds, exact discard count, early failure and error preservation.

One physical B525 run after the change completed 3/3 real captures, rendered the strip and saved a local output. OpenCV decoded all three 1280x720 JPEGs. Grayscale mean luminance was 133.07, 133.10 and 133.07 (0-255); grayscale standard deviation was 75.24, 75.25 and 75.21. The frames were very similar within that one session. The artifacts were kept in TEMP only during verification and removed afterward.

**Validation gap:** this is not a controlled before/after experiment: lighting and scene were not standardized against a pre-warm-up baseline. The result is encouraging evidence of within-session consistency, not proof that the warm-up solved the first-photo darkness issue. Re-test with fixed camera position and lighting, compare the first shot against shots 2/3, and inspect exposure visually. Do not compensate by brightening only the first photo in the renderer.

### Read-only camera-control diagnosis and reversible gain probe (2026-10-28)

Added `tools/camera-poc/camera-controls` to query the selected Media Foundation stream and its current camera-control ranges without changing settings by default. The current B525 readback at 1280x720 / 30 FPS was:

- Exposure: Auto, current `-4`, range `-11..-2`, driver default `-6`.
- White balance: Auto, current `6210`, range `2800..6500`, driver default `5500`.
- Brightness: Manual, current `128/255`, which equals its driver default.
- Backlight compensation: Manual, current `1/1`, which equals its driver default.
- Gain: Manual, current `128/255`, driver default `64`.
- Gamma: not exposed by this driver.

Because gain is manual, two bounded probes attempted to switch it to Auto and to maximum manual value `255`. The updated probe advances up to 30 stream frames before checking readback, to allow a driver to apply a control on a frame boundary. On this B525, both requests still read back as `128 / Manual` after 30 frames, so the helper stopped without saving diagnostic images. Independent read-only queries after both attempts confirmed `128 / Manual`; the original state was unchanged. The vendor control wrapper now reports non-`S_OK` HRESULTs as errors rather than swallowing `ERROR_NOT_FOUND`, and a four-case regression test covers `S_OK`, `ERROR_NOT_FOUND`, `E_FAIL` and `S_FALSE`. For this specific max/auto test, the setter returned without an error but readback never reflected the requested state. This establishes that the selected control path does not apply those changes while the stream is open; it does not prove no other driver/control path can change gain. The app should not silently force camera controls without a supported and verified configuration path.

The 2026-10-29 morning batch passed the current gross-darkness/relative-consistency gate, but the inspected frame still shows a dim room with no representative subject. The next image-quality test should use stable, sufficient booth lighting and the intended camera aim/distance. If that representative scene remains too dark, continue investigating supported camera-control integration rather than brightening only the first photo or lowering the gate.

### Wails UI capture-to-output smoke test (2026-10-28)

UI Automation clicked **Start with webcam** in the built Windows Wails app. The app completed three real captures, rendered the photo strip and wrote the final local output; the three capture JPEGs decoded at 1280x720. Their grayscale mean luminance values were 2.82, 2.74 and 2.73, with standard deviations 3.73, 3.08 and 2.92. Visual inspection showed a predominantly near-black scene with a small distant display. A follow-up CLI run against the same B525 and Media Foundation provider, outside the Wails UI, was also near-black (mean luminance 2.63, 2.63 and 2.64). This indicates the dark result was not isolated to the UI integration; the current scene itself was extremely dim. It is still an image-quality failure for a guest-facing booth, not a successful quality test. The temporary session folders and photo artifacts were deleted after inspection.

This smoke test validates that a guest can start a real webcam session from the Wails UI and have the three frames rendered and saved. It does not validate suitable lighting or image quality. The subsequent two-trial recovery/fresh-session test below strengthens UI recovery evidence, but does not establish long-duration kiosk behavior. Repeat with camera aim and booth lighting deliberately set before treating the quality gate as complete.

### Wails UI initial-camera-contention recovery (2026-10-28)

A competing OpenCV Media Foundation process held the B525 before the Wails app started a new photo session. UI Automation clicked **Start with webcam** and observed the recoverable camera-error screen with the **Retry camera connection** action. The holder was stopped, then UI Automation clicked retry/continue. The same UI session ID subsequently completed all three captures, rendered the strip and wrote its local output without restarting Wails. The test artifacts were removed afterward.

This validates one initial-open/contention failure followed by a successful manual retry from the UI. It does not test physical USB unplug during an active capture, multiple reconnect cycles, blocked native-read cancellation, or long-running kiosk stability.

### Repeatable image-quality gate (2026-10-28)

Added `tools/camera-poc/controlled-image-quality.ps1` and `tools/camera-poc/python/validate_capture_quality.py`, with unit tests in `tools/camera-poc/tests/test_capture_quality.py`. The PowerShell harness runs a configurable number of bounded real-camera sessions, checks capture/render/local-output counts, and leaves a JSON report in TEMP. It deletes capture artifacts by default. The Python gate checks expected dimensions, JPEG decoding, gross darkness (default mean luminance floor 20/255), and the first shot's luminance drop against the median of shots 2+ (default maximum 10%). The thresholds are diagnostics, not universal aesthetic rules.

The first measured batch used three sessions with three captures each. All 3 sessions and all 9 capture JPEGs passed decode/dimension/capture-render-save checks, but the image-quality gate failed:

- Session 1 mean luminance: 2.524; shot means 2.527, 2.523, 2.522.
- Session 2 mean luminance: 2.544; shot means 2.545, 2.545, 2.542.
- Session 3 mean luminance: 2.544; shot means 2.544, 2.547, 2.540.
- Across those images, 99.97%-100% of pixels had grayscale intensity <=15; contrast standard deviation was about 0.53-0.67 and Laplacian variance about 0.38-0.48.
- The relative first-shot drop was 0% in each session at the tool's three-decimal reported precision. This does not establish exposure adequacy because all shots were uniformly too dark.

The same-camera CLI and Wails captures were visually near-black in that night-time scene. This batch demonstrates why a successful JPEG/render/save pipeline must remain separate from an image-quality pass. The subsequent morning retest is recorded below; a representative static booth/portrait scene with fixed lighting and camera position/aim is still required. Do not relax the darkness sentinel just to turn a batch green.

### Morning image-quality retest (2026-10-29)

The same three-session/three-shot harness was rerun during daylight on the current camera setup. It passed the configured diagnostic gate across all 9 captures:

- Session mean luminance: 49.108, 50.051 and 52.515 on the 0-255 scale.
- Shot 1 luminance drop vs. median of shots 2-3: 0.054%, 0% and 0.122% respectively; all below the configured 10% limit.
- All 9 captures were 1280x720 and were rendered/saved successfully. Per-shot contrast standard deviation was about 31.7-32.2; Laplacian variance was about 16.1-16.6; pixels at grayscale <=15 ranged from 16.0%-19.4%.

I inspected a sample frame: it shows a real room with limited subject/booth lighting, substantial dark areas and visible low-light noise. This run is a meaningful improvement over the near-black night run and supports first-shot consistency after the one-second warm-up, but the gate is intentionally only a gross-darkness and relative-consistency diagnostic. A representative guest/portrait scene with the intended booth lighting and mount distance remains necessary before claiming acceptable guest-facing image quality. The 9 test images were inspected locally and the retained photo directory was deleted; only the metrics JSON remains in TEMP.

### Repeated Wails UI recovery and fresh-session test (2026-10-29)

Added `tools/camera-poc/ui-recovery-smoke.ps1`, a Windows UI Automation harness that deliberately holds the B525 from a separate Media Foundation process to force an initial camera-open failure. It runs the recovery and fresh-session paths in the same Wails process and cleans all new session/output folders at exit. A two-trial run passed:

- 2/2 **Continue same session** flows: the app displayed a recoverable camera failure, the competing holder was released, Continue was invoked, the original session ID was retained, and 3 captures/render/local output completed.
- 2/2 **Start a fresh session** flows: after a second forced failure, the holder was released and the action created a different session ID, then completed 3 captures/render/local output.
- 4 completed sessions total; 6 newly created session directories (including the two failed placeholders) and corresponding outputs were removed after verification. The JSON report is `photobooth-ui-recovery-20261029-082054.json` in TEMP.

This strengthens evidence for the Wails prototype's recovery/re-entry behavior. After the worker cancellation/Close-retry change was built, the UI harness was run once more against the new executable: same-session Continue passed, then Start a fresh session produced a distinct ID and passed capture/render/save. Three newly created session/output folders were deleted; the report is `photobooth-ui-recovery-20261029-092845.json` in TEMP. These are controlled contention/retry tests, not a full-day soak or physical USB unplug during an active capture. Next, validate with representative booth lighting, then test physical hot-unplug and native-driver behaviour when possible.

## Coarse image diagnostics

The capture verifier records grayscale mean luminance, grayscale standard deviation, Laplacian variance, and the percentage of near-black/near-white pixels. Across the 300 capture JPEGs in the 100-session batch:

- Mean luminance: 127.05 on the 0-255 scale on average; per-image mean range 116.65-140.68.
- Contrast standard deviation: 64.96 on average.
- Laplacian variance: 109.63 on average; per-image mean range 26.67-279.53.
- Near-black/near-white pixel percentage: 0.19% on average.

These values are diagnostic baselines for this indoor scene only. They are not universal targets, a visual sharpness score, or a quality certification. The earlier visual inspection found soft/noisy framing under current indoor lighting. Controlled booth lighting, intended mounting distance, representative people/skin tones, and repeatability across different scenes still need a visual assessment.

## What this proves and what remains open

The headless runner now repeatedly opens/closes the B525, completes capture-render-save, reports a concrete failure under competing camera access, and successfully retries in the same process after the holder exits. Separately, one forced initial-contention test through the Wails UI showed the failure screen and completed the same session after Continue once the competing process exited. Repeated UI recovery and long-running production kiosk behavior remain untested.

This does not prove thermal reliability, a universal peak-RAM bound, or recovery from a blocked synchronous native read. The measured 100-session run lasted about 87 seconds; much longer soak/overnight runs remain open. USB unplug/reconnect recovery passed in a headless same-process harness, but active-session USB unplug through the UI remains untested. Controlled-lighting image-quality review, laptop integrated camera, Linux camera/runtime and physical printer remain untested.

## Next reliability gate

1. Exercise the eventual full kiosk application (UI, preview, output and recovery) as one long-lived process. The current headless Go runner has passed 100 same-process sessions, but it is not the production kiosk process.
2. Repeat the physical unplug/replug recovery test across multiple cycles and device/USB-port combinations; the B525 on the current Windows host has passed one extended-window cycle.
3. Review captures under controlled lighting, fixed camera distance and repeatable framing.
4. Test cancellation while a native frame read is blocked on the selected driver.
5. Repeat the UI recovery journey across multiple trials, including Return to start and Start a fresh session after an error, without restarting the application.


## Wails UI active-session USB hot-unplug/replug (2026-10-31)

Test device: Logitech B525 HD Webcam, Windows 11 Pro 64-bit, experimental Wails v2.17 shell and Go Windows Media Foundation provider.

One real physical UI Automation run passed with the following sequence:
- Wails opened an isolated station profile and started a real B525 three-shot session.
- The first capture completed; the USB cable was physically unplugged during the second-shot countdown while the session/provider remained active.
- Windows PnP reported the B525 absent, then present again after reconnection.
- The Wails UI surfaced the camera recovery/reconnection state.
- The provider recovered automatically inside the in-flight capture; no explicit Continue action was used.
- The original session completed all 3 captures, rendered the strip, and saved the local output without restarting Wails.
- The disposable profile and test photos were deleted after verification; the JSON report was retained in TEMP.

Report: `%TEMP%\photobooth-usb-hotplug-smoke-0b988aaca6fb4c65acd9633f4ee5e849.json`. Session: `82d6d3bdd45d682019bd4407310bc212`; output size 228,474 bytes; `usb_unplug_observed=true`, `usb_replug_observed=true`, `recovery_event_observed=true`, `continue_action_used=false`, `capture_count=3`.

**Scope:** this is one successful physical B525 unplug/replug cycle through the Wails UI on this Windows host. It strengthens the earlier headless same-process recovery evidence, but does not establish repeated-cycle/device/port reliability, other camera models, full kiosk soak, or recovery from a native driver read that never returns.

## Wails camera image-quality and warm-up comparison (2026-10-31)

### Real Wails capture cadence

Added `tools/ui-spike/wails-v2/ui-camera-quality-smoke.ps1`. It launches the built Wails app with a disposable profile, uses the real B525/Media Foundation provider, captures three real UI sessions at the default `3s` countdown and `1s` interval, validates that each session produced 3 captures/render/output, then measures the capture JPEGs. Images/profile are deleted; only the metrics JSON remains.

Three Wails UI sessions completed (9/9 JPEGs at 1280x720), rendered and saved locally. First-shot luminance drop against the median of shots 2/3 was 0.000%, 0.000% and 0.660%. Session mean luminance was 129.282, 130.572 and 130.334 (0-255 grayscale). All sessions passed the existing gross-darkness and relative-consistency diagnostic gate. Report: `%TEMP%\photobooth-wails-quality-7a8bef8468e04c7691e3da93e39a6cf6.json`.

### Warm-up A/B diagnostic

Added a diagnostic-only `--skip-camera-warmup` option to the root `booth-player` CLI, backed by `windowsmf.Options.SkipWarmupForDiagnostics`. It is not part of station config or the Wails UI; ordinary provider/player paths continue to discard the default one second of warm-up frames. The flag prints a warning and is accepted only for `--camera=windowsmf`.

Added `tools/camera-poc/warmup-ab-test.ps1`. It alternates three warm-up-disabled and three warm-up-enabled real B525 sessions (3 shots each), with zero countdown/interval to isolate provider startup warm-up. All 6 sessions (18/18 JPEGs) completed capture/render/local output and passed the existing gross-dark/relative-first-shot validator.

- Warm-up disabled: mean signed first-shot drop 0.047%; mean first-shot luminance 131.278; mean later-shot luminance 131.340.
- One-second warm-up enabled: mean signed first-shot drop 0.010%; mean first-shot luminance 130.537; mean later-shot luminance 130.550.
- The measured reduction in relative first-shot drop was 0.037 percentage points, a very small difference in this scene. Overall luma between the arms also differed slightly, so scene/ambient drift remains a confounder.

Interpretation: current results do not justify increasing warm-up duration, changing camera gain/exposure, or altering the first photo in the renderer. Keep the one-second default. The A/B data is same-host/same-scene diagnostic evidence, not a controlled studio/portrait-lighting validation or proof the warm-up is universally effective. A fixed lighting, distance, framing and representative subject before/after test remains open. The diagnostic `--skip-camera-warmup` option must not be used for normal booth sessions.

Report: `%TEMP%\photobooth-warmup-ab-20261031-185921.json`. Capture data was removed; the report retains per-capture metrics and the signed A/B comparison.

## Idle live preview / capture handoff (2026-10-31)

Added the optional `camera.PreviewProvider` interface without changing the required `camera.Provider` contract. The Windows Media Foundation provider implements Preview using the same capture worker and shared JPEG normalization/validation helper used by Capture; a provider operation mutex serializes native reads. The recovery decorator handles stream-invalidated preview reads with the bounded close/reopen policy.

The Wails prototype renders preview only on the idle/home screen. Before Start session, demo session, operator Settings and app shutdown, it cancels preview and waits for provider close. If the native worker does not stop inside the 7-second shutdown budget or Close reports an error, the app refuses to open another camera provider. A native close error remains latched so repeated Start/Capture attempts stay fail-closed. When returning to idle, preview starts again.

Backpressure: the backend publishes a JPEG data URL with an increasing sequence, then waits for the WebView ACK. The frontend ACKs after image decode/load. Only one frame can be in flight, with a 200ms minimum interval (5 FPS maximum); an ACK timeout of 2 seconds stops preview rather than queueing frames indefinitely. Frames are not written to session or output directories.

Physical B525 UI smoke passed on 2026-10-31. Report: `%TEMP%\photobooth-wails-preview-2f6b66c9f67e4e55a66bf190ba6ffa1e.json`. It confirmed: real preview before capture; original three-shot workflow captured 3/3, rendered and saved local output while preview had been stopped; after selecting Take another set, the live preview was rendered again. Session `e0e7d34c6cee2ef62df8db51bff05d81`. The disposable photo/profile directory was deleted; only JSON metrics remain.

Limits: one real reference-device smoke only. It does not establish long-running preview stability, other cameras/ports, Linux, production UI, or portrait quality under booth lighting. Preview and capture are serialized and not simultaneous by design.


## Preview smoke rerun after lifecycle hardening (2026-10-31)

After adding the `previewAckPending` duplicate-ACK guard, making WebView decode failures non-acknowledging, and removing the untracked native-startup cleanup goroutine from the Media Foundation `Open(ctx)` cancellation path, the Wails Windows production build and real B525 UI smoke were rerun.

Latest report: `%TEMP%\photobooth-wails-preview-713d7c864cf1438cb4ebd1a21397b379.json`. Session `f3d6a96b74a68168ea896435e3997b41`. The report has `status=PASS`, `preview_before_capture=true`, `preview_after_capture=true`, `capture_count=3`, `rendered=true`, `local_output=true`, and `temp_data_cleaned=true`. The test JPEG/session profile were deleted.

Additional unit tests verify that cancellation while native startup is in progress waits until the worker reports startup success/failure and completes its cleanup before the cancellation path returns. If native startup does not return, Wails' 7-second stop barrier times out and prevents capture from opening a second worker.


## Ten-minute Wails idle-preview / kiosk soak (2026-11-01)

The reusable harness `tools/ui-spike/wails-v2/ui-preview-kiosk-soak.ps1` now exercises the long-lived UI rather than only the headless Go runner. It runs the real Windows Media Foundation B525 idle preview in fullscreen kiosk mode, samples CPU and working-set/private-byte values for PhotoBooth.exe and its WebView2 process tree every 2 seconds, checks preview progress/ACK timeouts/decode failures, and repeats the Settings stop/resume and fullscreen/windowed lifecycle. It uses an isolated temporary profile and no photo session.

**Result:** PASS on ShinPc / Windows 11 / Logitech B525. Duration 10.26 minutes, 195 samples, 2,825 effective ACKs, five Settings cycles and five display-mode cycles. Mean/min preview FPS 4.71 / 4.07. No ACK timeout or decode failure. CPU tree mean/peak 15.28% / 25% of one logical core. PhotoBooth.exe private bytes stayed around 29 MB (peak 31.12 MB); WebView2 private bytes peaked at 1,418 MB and ended at 1,315 MB. WebView2 allocation rose during startup and then fluctuated around 1.3–1.45 GB for the rest of this single run. That is not evidence of an unbounded leak within the measured window, but it remains a significant memory footprint for low-cost PCs. The test's 3,072 MB process-tree private-bytes safety stop was not reached.

A post-run exact JSONL audit found 2,882 frame publications, 2,872 ACK events and zero cancellation events: 10 published sequences had no matching ACK/cancel record. No timeout/decode failure was emitted, but later frame publication is not accepted as proof that the missing audit record existed. This is an observability defect addressed by the subsequent canvas/diagnostics hardening and the exact-audit rerun below.

Report: `%TEMP%\photobooth-preview-soak-1b9787abe95f4b978f08339cb895c715\summary.json`; supporting CSV/JSONL/Markdown files are alongside it. The temporary station profile was cleaned.

**Limits:** one 10-minute run on one Windows host and one camera. Full-day/overnight behavior, memory behavior on a low-RAM target, other camera models/ports, representative image quality, printer, Linux and production UI certification remain open.


## Preview canvas / diagnostics hardening and repeat kiosk soak (2026-11-01)

The first 10-minute UI soak reported a WebView2 private-byte peak of 1,418 MB and missing JSONL ACK audit records. The Wails idle preview was changed from replacing a data-URL `img` on every frame to a single canvas backed by a temporary `ImageBitmap`; the bitmap is closed after drawing and before ACK. The canvas backing store remains stable at 1280x720 and is reset when preview stops. The backend now reuses a serialized diagnostics append handle; Windows share-read/write flags allow the sampler to read the file concurrently, and shutdown flushes/closes it. A frame that is still in flight at cancellation is recorded as `frame_cancelled`; a queued ACK is consumed first where available.

The updated kiosk soak uses exact post-shutdown JSONL sequence reconciliation. A later frame no longer counts as proof that an ACK log event exists. The final gate rejects unmatched publishes, orphan/duplicate ACKs, duplicate publishes/cancellations, ACK/cancel overlap, invalid lines, timeouts and decode failures.

**Repeat result: PASS** on B525 / Windows 11 / ShinPc:
- 10.24 minutes; 195 samples; fullscreen kiosk; five Settings stop/resume cycles and five fullscreen/windowed cycles.
- 2,828 published frame events = 2,828 ACK events; zero cancellations, timeouts, decode errors, unmatched publishes, orphan ACKs, duplicate events or invalid lines.
- Mean/min FPS 4.73 / 4.70; final 4.78 FPS.
- CPU process-tree mean/peak 14.50% / 22% of one logical core.
- Process-tree private bytes 344.54 / 1,130.56 / 1,285.69 MB (first/last/peak).
- WebView2 private bytes 315.20 / 1,098.87 / 1,254.63 MB (first/last/peak); PhotoBooth.exe private bytes peak 31.86 MB.
- The initial 10.26-minute baseline's WebView2/process-tree peaks were 1,418.22 / 1,447.70 MB. The repeat was lower by approximately 11.5% / 11.2%, but this is only an indicative two-run comparison, not a controlled A/B. WebView2 still has a high low-RAM footprint.
- Temporary profile cleaned; no photo session or test guest images were created.

Report: `%TEMP%\photobooth-preview-soak-d9ce78c26007467fac0cee06e3f4ebb1\summary.json`. See [ADR 0023](../adr/0023-preview-renderer-and-soak-diagnostics.md).

The result remains a 10-minute single-host test. Full-day/overnight, low-RAM target hardware, other cameras/ports, Linux, printer and representative portrait lighting remain open.

## Wails current-scene capture-quality rerun (2026-11-01)

Harness: `tools/ui-spike/wails-v2/ui-camera-quality-smoke.ps1`. Real Logitech B525 / Windows Media Foundation through the Wails UI. Three normal sessions, each with three captures, 3-second countdown and 1-second interval. Camera aim, scene and lighting were kept unchanged during the batch as instructed by the harness. The script verified each session produced 3 captures, `rendered.jpg` and local `photo.jpg`, then ran the existing quality validator.

**Result: PASS**
- 3/3 sessions completed; 9/9 captures were 1280x720.
- All 3 sessions rendered and produced a local output.
- Session 1: mean luma 122.758; later-shot median 122.764; first-shot drop 0.017%.
- Session 2: mean luma 122.661; later-shot median 122.706; first-shot drop 0.110%.
- Session 3: mean luma 122.738; later-shot median 122.833; first-shot drop 0.231%.
- Mean signed first-shot drop across the three sessions: approximately 0.119%.
- Per-capture dark-pixel fraction was 0.151–0.169%; clipped-pixel fraction was 0.654–0.681%; Laplacian variance was 12.720–13.095. These are descriptive metrics only; no sharpness acceptance threshold is defined.
- Validator thresholds remained unchanged: minimum session mean luminance 20/255, maximum first-shot drop 10%. No threshold was relaxed.
- Temporary photo/session data was removed; only the JSON metrics report remains.

Report: `%TEMP%\photobooth-wails-quality-92009cc35d5444d29f236fa624a50ed9.json`.

**Interpretation and limits:** the batch shows consistent exposure across the three shots in this current scene and no material first-shot-darkness signal. It does not prove portrait quality: exposure/lighting were not instrument-controlled, there was no representative subject/framing review, and the capture images were deleted rather than retained for visual inspection. Keep the one-second warm-up default unchanged. A fixed-lighting, intended camera distance/framing and human visual review remains open.

## Physical printer inventory on ShinPc (2026-11-01)

Windows reports only Microsoft Print to PDF and two Microsoft OneNote virtual queues. No physical printer model/driver/USB or network printer queue is installed. Therefore no physical print test or model-specific Windows printer adapter can be validated on this host. Local JPEG export remains the currently tested output path; a physical printer adapter should wait until the target model, connection and supported OS/driver are selected.


## Wails current-scene quality batch (2026-11-05)

Harness: tools/ui-spike/wails-v2/ui-camera-quality-smoke.ps1, using the current PhotoBooth.exe and real Logitech B525 / Windows Media Foundation through the Wails UI. Three sessions of three captures each were completed with the existing 3-second countdown and 1-second interval. The harness used an isolated temporary profile and retained its data root only because -KeepArtifacts was supplied.

**Capture workflow: PASS; image-quality gate: FAIL**
- 3/3 sessions completed; 9/9 captures were 1280x720.
- All 3 sessions rendered and produced a local output.
- Session mean luminance: 2.520, 2.530 and 2.504 (0-255 scale).
- First-shot relative drops: 0.040%, 0.296% and 0.000%.
- Every capture was below the existing 20/255 gross-darkness threshold; the inspected capture image was nearly black.
- Thresholds were not relaxed. The first-shot consistency signal remained small (under 0.3%) and did not reproduce a material first-shot-darkness difference in this batch.

Report: %TEMP%\photobooth-wails-quality-cb632a1a1cd94153b78e20c2c946a07e.json.

Retained artifacts root: %TEMP%\photobooth-wails-quality-cb632a1a1cd94153b78e20c2c946a07e. It contains real camera captures, rendered photos and local outputs; treat it as private photo data. The operator should remove it manually after review when no longer needed.

**Interpretation and limits:** this is a failed current-scene darkness gate, not a portrait/framing acceptance. Lighting and scene exposure were not instrument-controlled, so the result does not isolate a camera-driver/provider defect. Do not change the one-second warm-up or exposure policy based on this batch. Repeat the short quality review only after arranging a representative fixed-lighting scene and intended camera framing. No USB recovery or long-run test was performed in this continuation.
