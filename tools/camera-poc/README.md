# Webcam Camera POC

This is a disposable technical spike to assess the generic UVC webcam path on the current development machine. It is **not production PhotoBooth code** and does not establish laptop-camera or Linux compatibility.

## Current environment

- Windows 11 Pro 64-bit, version 10.0.26200.
- OS device list detected a Logitech B525 HD Webcam on 2026-10-25.
- Detection is confirmed. On 2026-10-25, OpenCV 5.0.0.93 read 50/50 frames at 1280x720 using both DirectShow and Media Foundation and wrote a temporary JPEG through each backend. This is only a stream/frame-capture smoke test; reconnect, image quality, frame rendering and print/export remain untested.
- The current machine's device list did not show an integrated laptop camera, so that use case needs a separate laptop test.

## Run the POC (Windows PowerShell)

From repository root:

    py -3.12 -m venv tools/camera-poc/.venv
    tools/camera-poc/.venv/Scripts/python.exe -m pip install --upgrade pip
    tools/camera-poc/.venv/Scripts/python.exe -m pip install -r tools/camera-poc/requirements.txt
    tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/capture_test.py --scan --backend dshow

If DirectShow does not find frames, retry using --backend msmf and then --backend auto.

After identifying a working camera index (this POC only knows indices, not friendly device names), capture 50 frames and save the final frame to a temporary file:

    $out = Join-Path $env:TEMP 'photobooth-b525-poc.jpg'
    tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/capture_test.py --index 0 --backend dshow --frames 50 --output $out
    Get-Item $out | Select-Object FullName,Length

Replace index 0 with the index that worked. Verify the image locally, then delete it. A webcam image may include people or private surroundings; the POC does not upload the file.

## Results so far (2026-10-25)

- Windows 11 Pro 64-bit, build/version 10.0.26200; Logitech B525 detected by OS.
- OpenCV package 5.0.0.93 (cv2 reports 5.0.0), local virtual environment.
- DirectShow, camera index 0: 10/10 frames in initial scan; then 50/50 frames at 1280x720 and JPEG write succeeded.
- Media Foundation, camera index 0: 50/50 frames at 1280x720 and JPEG write succeeded.
- Temporary capture JPEGs were deleted. Their visual quality was not inspected, and no webcam image was added to the repository.
- A separate disposable render test composed one captured frame into a 1200x1800 sample JPEG; OpenCV read it back at the expected dimensions. The temporary source and rendered files were deleted. This is not the production renderer and is not a printer test.
- Laptop integrated camera and Linux remain untested.

## Capture-to-render command example

Use a temporary directory and delete both image files after verifying the output:

    $source = Join-Path $env:TEMP 'photobooth-source.jpg'
    $output = Join-Path $env:TEMP 'photobooth-frame.jpg'
    try {
      tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/capture_test.py --index 0 --backend msmf --frames 10 --output $source
      tools/camera-poc/.venv/Scripts/python.exe tools/camera-poc/python/render_frame_test.py --input $source --output $output
      Get-Item $output | Select-Object FullName,Length
    } finally {
      Remove-Item $source,$output -Force -ErrorAction SilentlyContinue
    }

## Test sequence

1. Close other apps that may be using the webcam.
2. Scan camera indices with DirectShow; try Media Foundation if necessary.
3. Read 50 consecutive frames and save only the final frame.
4. Check dimensions, orientation, color, focus/exposure, and whether the image is stale.
5. Close and reopen the camera several times.
6. Test behavior when another application already owns the camera.
7. Disconnect/reconnect the B525 and repeat the scan.
8. A one-off sample-layout render has passed. The production renderer, frame schema, print path, and visual quality remain to be validated.
9. Repeat on an actual laptop for the integrated camera. Do not mark this case passed based on the B525 result.

## Interpretation

- Scan pass means the OS/backend can stream frames at that moment, not that a full session is reliable.
- A 50-frame pass is an early sanity check, not a soak test.
- OpenCV is used here to get an initial answer quickly. It is not yet the production backend decision.
- Production evaluation still needs Windows and Linux behavior, packaging, camera permissions, device-loss recovery, and sustained-session stability.
- Keep all output images out of version control.
