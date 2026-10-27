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
- Camera-busy recovery here is a low-level smoke check only. The production app still needs its own state/retry/error handling.

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
- The Python JSON renderer remains a disposable POC. The Go renderer is the main development path. The experimental Windows Media Foundation provider passed 100 sequential real B525 sessions in one Go process after native resource-lifecycle cleanup (300/300 captures, 100 rendered strips, 100 local copies); all 500 JPEG artifacts had expected dimensions. OS memory sampled across 813 points did not increase in the final 10% of the run. A same-process OpenCV holder triggered HRESULT 0xC00D3704; the same Go process recovered after release. See docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md. Physical USB unplug/reconnect and controlled-lighting image quality remain untested.
- Printing, physical camera disconnect/reconnect, full-day/overnight stability, UI preview, and full kiosk operation are not yet tested.
- The built-in laptop camera requires a separate test on an actual laptop.
- Linux capture/output needs a selected distro and a separate test.
- DSLR/mirrorless remote still capture and original-file download need another provider path.
- Keep all output photos out of version control.


## Repeatable Windows reliability batch

On the development Windows machine with the Logitech B525 connected and the repository Python virtual environment installed, run the script from the repository root:

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\camera-poc\windows-b525-reliability.ps1 -Sessions 100 -Shots 3

The script builds a standalone player, runs sequential fresh-process real-camera sessions, validates JPEG decode and dimensions with verify_soak_artifacts.py, and optionally checks a competing camera holder. The script uses temporary directories for photos/logs and removes them by default; only a CSV summary remains in the user's TEMP directory. Add KeepArtifacts only when local inspection is necessary, and delete the retained images after inspection. For same-process heap/OS-memory measurements and same-process contention recovery, use the separate harness documented in persistent-soak/README.md. Neither test replaces physical USB unplug/reconnect or a full-day/overnight kiosk soak.


## Persistent single-process soak

For a longer test that reuses one Go process and the same camera-provider boundary, see [persistent-soak/README.md](persistent-soak/README.md). It records heap/GC/goroutine metrics, validates all session image files, optionally emits a Go heap profile, and can simulate camera contention followed by recovery in the same process.

This headless harness passed 100 consecutive B525 sessions after native resource-lifecycle cleanup; it is not the final GUI kiosk. USB unplug/reconnect, blocked native-read cancellation, and controlled-lighting image quality remain separate gates.
