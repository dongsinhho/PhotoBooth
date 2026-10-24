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
- The JSON renderer is a disposable prototype, not the production frame schema or renderer.
- Printing, camera disconnect/reconnect, sustained-session stability, UI preview, and kiosk operation are not yet tested.
- The built-in laptop camera requires a separate test on an actual laptop.
- Linux capture/output needs a selected distro and a separate test.
- DSLR/mirrorless remote still capture and original-file download need another provider path.
- Keep all output photos out of version control.
