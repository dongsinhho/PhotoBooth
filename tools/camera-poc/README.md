# Webcam Camera POC

This is a disposable technical spike to assess generic UVC webcam capture and config-driven frame rendering. It is **not production PhotoBooth code** and does not establish laptop-camera or Linux compatibility.

## Current environment

- Windows 11 Pro 64-bit, version 10.0.26200.
- OS device list detected a Logitech B525 HD Webcam.
- OpenCV Python package 5.0.0.93 is installed only in the local POC virtual environment.
- DirectShow and Media Foundation both read 50 of 50 frames at 1280x720 and wrote temporary JPEGs.
- Three open/read/save/release cycles of 10 frames passed per backend.
- A disposable JSON-configured three-photo strip was rendered to 1200x1800 and read back successfully.
- When a second process attempted to read while another process held the camera, the second reader failed on its first frame. After the holder released the camera, reads succeeded again.
- Camera-busy recovery here is a low-level smoke check. The Go booth-player now has a bounded provider recovery wrapper; physical USB unplug/replug was verified in a same-process headless harness, but production kiosk UI/session re-entry still needs a dedicated test.

## Setup (Windows PowerShell)

From repository root:

    py -3.12 -m venv tools/camera-poc/.venv
    tools/camera-poc/.venv/Scripts/python.exe -m pip install --upgrade pip
    tools/camera-poc/.venv/Scripts/python.exe -m pip install -r tools/camera-poc/requirements.txt

## Capture scan

    tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/capture_test.py --scan --backend dshow

If DirectShow does not find frames, retry using --backend msmf and then --backend auto.

## Capture and render using the JSON frame

The bundled sample frame at tools/camera-poc/sample-frame.json defines a 1200x1800 canvas, a border, text, and three photo slots. The renderer validates the JSON, fits each source into its assigned slot, and exports one image.

The following captures three test images and creates a temporary output. It does not upload any images. Delete these outputs after local verification; they may include people or private surroundings.

    $python = 'tools/camera-poc/.venv/Scripts/python.exe'
    $capture = 'tools/camera-poc/python/capture_test.py'
    $render = 'tools/camera-poc/python/render_frame_test.py'
    $config = 'tools/camera-poc/sample-frame.json'
    $inputs = 1..3 | ForEach-Object { Join-Path $env:TEMP "pb-frame-$_.jpg" }
    $output = Join-Path $env:TEMP 'pb-photo-strip.jpg'
    try {
      for ($i = 0; $i -lt 3; $i++) {
        & $python $capture --index 0 --backend msmf --frames 10 --output $inputs[$i]
        if ($LASTEXITCODE -ne 0) { throw "Capture $i failed" }
      }
      & $python $render --config $config --input $inputs[0] --input $inputs[1] --input $inputs[2] --output $output
      if ($LASTEXITCODE -ne 0) { throw 'Render failed' }
      Get-Item $output | Select-Object FullName,Length
    } finally {
      Remove-Item $inputs -Force -ErrorAction SilentlyContinue
      Remove-Item $output -Force -ErrorAction SilentlyContinue
    }

## Unit tests

    tools/camera-poc/.venv/Scripts/python.exe -m unittest discover -s tools/camera-poc/tests -p 'test_*.py' -v

The tests cover configuration validation, canvas bounds, color formats, contain/cover fitting, input-slot count, and three-photo rendering. They do not test the camera SDK or printer.

## Optional camera-busy probe

In one PowerShell window, run:

    tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/hold_camera.py --index 0 --backend msmf --seconds 20

While it is holding the webcam, run a capture attempt from another window. The second process may fail to read frames because the camera is already in use. Close the holder, then repeat the capture test and confirm that it succeeds. This is expected to characterize the current OS/backend behavior, not a production error-handling test.

## Known gaps

- One temporary B525 image was visually inspected and deleted: readable but soft/noisy under current indoor lighting and framed too close/off-center. Repeat the quality check with controlled booth lighting and intended mount distance; stale-frame and long-run quality tests remain pending.
- The Python JSON renderer remains a disposable POC. The Go renderer is the main development path. The experimental Windows Media Foundation provider passed 100 sequential real B525 sessions in one Go process after native resource-lifecycle cleanup (300/300 captures, 100 rendered strips, 100 local copies); all 500 JPEG artifacts had expected dimensions. OS memory sampled across 813 points did not increase in the final 10% of the run. A same-process OpenCV holder triggered HRESULT 0xC00D3704; the same Go process recovered after release. See docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md. Extended-window in-flight recovery after physical USB replug passed in the single-process manual harness. A Wails UI capture-output smoke test and one forced initial-contention/retry journey have since passed. The original night scene was grossly dark, but a 2026-10-31 Wails UI batch in the current scene passed 9/9 captures at 1280x720; first-shot drops were 0.000%, 0.000% and 0.660%. A three-pair warm-up A/B found only a 0.037 percentage-point reduction in first-shot drop with the one-second warm-up; keep the default unchanged. Representative controlled-lighting portrait quality is still untested. One active-session physical B525 USB unplug/replug through Wails also passed once; repeat cycles/ports remain open.
- Physical USB unplug/replug passed with the expanded recovery window: provider reopen succeeded on attempt 22 after 21 unavailable-device attempts, and the same in-flight capture recovered in 23.906 seconds without a process restart. A separate physical unplug/replug during an active Wails UI session passed once on 2026-10-31; Windows PnP and Wails recovery were observed and the same three-shot session completed without app restart or Continue. Repeat UI cycles/ports, printing, full-day/overnight stability, live UI preview and full kiosk operation remain untested.
- The built-in laptop camera requires a separate test on an actual laptop.
- Linux capture/output needs a selected distro and a separate test.
- DSLR/mirrorless remote still capture and original-file download need another provider path.
- Keep all output photos out of version control.


## Read-only Windows camera-control diagnostic

To inspect the current B525 control values without changing them, run this Windows-only command from the repository root while no other app is using the camera:

```powershell
go run ./tools/camera-poc/camera-controls --device="B525 HD Webcam"
```

The default command opens the selected 1280x720 MJPEG stream, reads the driver's current exposure, focus, brightness, gamma, white-balance, backlight-compensation and gain values/ranges plus raw auto/manual flags, writes a JSON report, and closes the camera without changing settings. Values are driver-specific and are not directly comparable across camera models. Do not treat control readback as proof that lighting or photo quality is adequate; if a setting reports an unknown mode, keep that ambiguity rather than inferring that automatic exposure is enabled.

The optional `--probe-auto-gain` and `--probe-max-gain` modes temporarily attempt a bounded gain change, advance up to 30 stream frames to allow frame-boundary application, capture three diagnostic frames only if the driver reports the requested mode/value, and verify the original value/flags before returning. They fail rather than saving samples if readback never matches. On 2026-10-29, this B525 remained at gain 128/manual after both the Auto request and a manual 255 request, even after 30 frames; no diagnostic JPEGs were saved. An independent read-only query confirmed the original 128/manual state after each probe. The control wrapper also now treats non-`S_OK` set HRESULTs as errors. Do not use these probes alongside another camera application or assume other models/drivers behave the same way.

## Repeated Wails UI recovery and fresh-session test

Run this on the connected Windows test machine with the B525 attached and no other capture application running. The harness deliberately uses a separate OpenCV Media Foundation holder to create the camera-unavailable state, invokes the real Wails UI through Windows UI Automation, tests Continue with the same session ID, and then tests Start a fresh session with a new ID. Two trials exercise both recovery paths repeatedly within one app process:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\camera-poc\ui-recovery-smoke.ps1 -Trials 2
```

The harness builds on the already-built Wails executable, closes the UI when done, deletes all newly created session/output folders (these are real camera frames and may contain private surroundings), and retains only a JSON summary report in `%TEMP%`. It does not unplug USB hardware or replace a long-duration kiosk soak.

## Controlled capture image-quality batch

Use this after setting the intended camera aim/distance and a fixed lighting arrangement. Do not move the camera or change the scene between sessions in the same comparison batch. The script builds the Go player, runs bounded real B525 sessions, verifies that every session produced the requested number of capture JPEGs plus a rendered strip and local output, then measures dimensions, grayscale mean luminance, contrast standard deviation, Laplacian variance and near-black/clipped pixel percentages.

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\camera-poc\controlled-image-quality.ps1 -Sessions 3 -Shots 3
```

The JSON metrics report is retained in `%TEMP%`; capture JPEGs and session folders are deleted by default. Add `-KeepArtifacts` only when local visual inspection is required, and delete the photos afterward. The default mean-luminance floor of 20/255 is a gross-dark sentinel, not a universal photo-quality threshold; the relative check flags shot 1 when its mean luminance is more than 10% below the median of shots 2 and 3. Change thresholds only for a justified test scene and keep the original measurements. This checker does not replace visual review or controlled lighting.

The initial 2026-10-28 three-session batch captured/rendered/saved all nine frames, but failed the darkness gate: mean luminance was 2.52-2.54/255, with 99.97%-100% of pixels at grayscale <=15. The first shot was not darker than the later shots in those sessions. This is evidence that the current scene is too dark, not proof that camera exposure quality is acceptable. A controlled-lighting re-test remains open.

## Repeatable Windows reliability batch

On the development Windows machine with the Logitech B525 connected and the repository Python virtual environment installed, run the script from the repository root:

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\camera-poc\windows-b525-reliability.ps1 -Sessions 100 -Shots 3

The script builds a standalone player, runs sequential fresh-process real-camera sessions, validates JPEG decode and dimensions with verify_soak_artifacts.py, and optionally checks a competing camera holder. The script uses temporary directories for photos/logs and removes them by default; only a CSV summary remains in the user's TEMP directory. Add KeepArtifacts only when local inspection is necessary, and delete the retained images after inspection. For same-process heap/OS-memory measurements and same-process contention recovery, use the separate harness documented in persistent-soak/README.md. Neither test replaces physical USB unplug/reconnect or a full-day/overnight kiosk soak.


## Persistent single-process soak

For a longer test that reuses one Go process and the same camera-provider boundary, see [persistent-soak/README.md](persistent-soak/README.md). It records heap/GC/goroutine metrics, validates all session image files, optionally emits a Go heap profile, and can simulate camera contention followed by recovery in the same process.

This headless harness passed 100 consecutive B525 sessions after native resource-lifecycle cleanup; it is not the final GUI kiosk. The expanded USB unplug/replug recovery window passed a physical test in the same process, recovering the in-flight capture after 23.906 seconds. Blocked native-read cancellation and controlled-lighting image quality remain separate gates.


## Physical USB unplug/reconnect with the Go recovery wrapper

The Windows booth-player wraps the Media Foundation provider with bounded close/reopen recovery. To verify a real unplug/replug in one process, follow [hotplug-reopen-test/README.md](hotplug-reopen-test/README.md) and run:

```powershell
Set-Location .\tools\camera-poc\hotplug-reopen-test
go run .
```

The integrated recovery test now uses a longer, bounded recovery window so it can keep the current capture waiting for the device to reappear. Do not unplug a camera used by another application or with unsaved work. Test photos are created only in TEMP and must not be committed.


## Wails real-camera quality and warm-up A/B diagnostics (2026-10-31)

### Wails UI cadence quality check

Run this on the connected Windows B525 machine with the intended scene kept still for the full batch:

~~~powershell
Set-Location .\tools\ui-spike\wails-v2
.\ui-camera-quality-smoke.ps1 -Sessions 3
~~~

The script launches the built Wails executable with a disposable `PHOTOBOOTH_DATA_DIR`, a real Windows Media Foundation B525 camera and the normal station cadence (3-second countdown, 1-second interval, three shots). It uses Windows UI Automation to start three real sessions, verifies all captures/render/local outputs, measures dimensions/luminance/contrast/sharpness/dark/clipped pixels, retains only a JSON report and deletes temporary photos/profile. It is a diagnostic, not a fixed-lighting portrait certification.

Run on 2026-10-31: 3/3 sessions completed and 9/9 JPEGs were 1280x720; all render and local-output checks passed. First-shot drop against median of shots 2/3 was 0.000%, 0.000% and 0.660%. Mean luminance per session was 129.282, 130.572 and 130.334. Report: `%TEMP%\photobooth-wails-quality-7a8bef8468e04c7691e3da93e39a6cf6.json`.

### Warm-up A/B diagnostic

To compare the current one-second provider warm-up with no warm-up in the root CLI, run:

~~~powershell
Set-Location .\tools\camera-poc
.\warmup-ab-test.ps1 -SessionsPerVariant 3 -Shots 3
~~~

The harness alternates variant order, uses the same B525/scene, and sets countdown/interval to 0s to isolate stream startup warm-up. It builds `apps/booth-player`, captures three sessions per arm, validates three captures/render/local output, measures each JPEG and removes all temporary photo artifacts by default. It retains a JSON metrics report under `%TEMP%`.

`--skip-camera-warmup` is intentionally diagnostic-only. It is not in station config and must not be used for normal booth sessions. Ordinary provider and Wails paths continue to discard the default one second (bounded at three seconds).

Run on 2026-10-31: all 6 sessions/18 captures and outputs passed. Mean signed first-shot drop was 0.047% without warm-up and 0.010% with the one-second default, a 0.037 percentage-point difference. This small current-scene difference does not justify increasing warm-up or changing gain/exposure. Report: `%TEMP%\photobooth-warmup-ab-20261031-185921.json`.

Representative portrait quality still requires fixed lighting, camera placement/distance, a subject and visual review. Do not infer beauty/portrait quality from global luma thresholds alone.
