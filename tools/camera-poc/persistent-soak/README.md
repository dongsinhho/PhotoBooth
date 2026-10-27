# Persistent Go Camera Soak

This tool repeatedly runs the PhotoBooth session workflow inside **one Go process** using the experimental Windows Media Foundation provider. It measures per-session Go heap/GC/goroutine metrics and validates every successful JPEG artifact. It is a headless reliability harness, not the production kiosk UI.

## Requirements

- Windows 11 with a Logitech B525 connected and not already in use.
- Go 1.24.8 (the root module toolchain).
- The repository Python virtual environment with OpenCV installed, for optional contention testing.

## Run 100 sessions in one process

From the repository root, in PowerShell:

    $root = Join-Path $env:TEMP "photobooth-persistent"
    Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue
    go run ./tools/camera-poc/persistent-soak --sessions=100 --shots=3 --device="B525 HD Webcam" --work-dir="$root/sessions" --output-dir="$root/output" --metrics-csv="$root/metrics.csv" --heap-profile="$root/heap.pprof"

This captures 300 frames, renders one strip per completed session, and saves one local copy per completed session. The tool decodes each output and checks capture dimensions 1280x720 and frame-output dimensions 1200x1800. It writes per-session Go runtime measurements to metrics.csv and an optional final in-use Go heap profile to heap.pprof.

The tool explicitly runs GC after collecting each session's before-GC metrics. Session wall-time measurements stop before this diagnostic GC, but the pause between sessions is longer than a production run. Use heap-after-GC as an estimate of retained Go heap rather than production latency.

## Test same-process contention and recovery

    $root = Join-Path $env:TEMP "photobooth-persistent-contention"
    $python = (Resolve-Path "tools/camera-poc/.venv/Scripts/python.exe").Path
    $holder = (Resolve-Path "tools/camera-poc/python/hold_camera.py").Path
    go run ./tools/camera-poc/persistent-soak --sessions=12 --shots=3 --device="B525 HD Webcam" --work-dir="$root/sessions" --output-dir="$root/output" --metrics-csv="$root/metrics.csv" --test-contention --contention-after=3 --holder-seconds=5 --python="$python" --holder-script="$holder"

The test opens the camera for three successful sessions, starts a separate OpenCV holder, checks that attempted sessions fail with HRESULT 0xC00D3704 while the camera is occupied, then waits for release and performs an explicit recovery session in the same Go process.

## Metrics and interpretation

The CSV records session ID/state, wall time, capture count, render/output time, heap before and after forced GC, HeapInuse, HeapSys, TotalAlloc, NumGC, and goroutines. The headless process can therefore be compared between runs.

Go heap does not include all Media Foundation/COM/driver memory. For native resource checks, sample the Windows process WorkingSet64 and PrivateMemorySize64 separately, especially in the first and last portions of a long run. A stable heap is not proof of stable native memory; a rising Working Set alone is not proof of a leak.

All files beneath the supplied work/output directories are real camera images and can contain people or private surroundings. Use TEMP directories, never commit these artifacts, review only what is needed, and delete the test folder after recording metrics.
