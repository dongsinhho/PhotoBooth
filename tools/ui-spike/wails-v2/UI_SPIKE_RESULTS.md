# UI Spike Results

**Date:** 2026-10-25
**Status:** Windows shell/bridge experiment passed; production UI not selected.

## Environment

- OS: Windows 11 Pro 64-bit, build 26200
- CPU: AMD Ryzen 5 8600G with Radeon 760M Graphics
- Wails: v2.17.0
- Isolated module Go toolchain: Go 1.25.0
- Frontend: Vite 7
- WebView2 Runtime: 155.0.4283.62
- Node.js: 22.12.0 / npm 10.9.0
- App binary: approximately 11.0 MB
- Wails doctor reported the Windows environment ready.

The root/core module remains on Go 1.24.8. The UI spike's Wails v2.17.0 dependency requires Go 1.25.0, so the experiment is isolated under this nested Go module and must not drive an implicit upgrade of the core toolchain.

## What was implemented

A touch-first kiosk shell with three views:
- Idle/start screen.
- Simulated three-shot countdown/capture.
- Result screen with simulated photo strip.

A small Go binding owns the demo session state and validates session ID/state transitions. No physical camera, renderer, storage target, printer, payment or cloud service is connected. No real guest photo is created.

A hidden benchmark mode loads the embedded frontend, waits until JavaScript calls the Go Ready binding, makes 100 sequential Ping calls, verifies every echoed sequence value, writes a JSON report, and closes the application. A PowerShell harness samples the executable's process working set and private bytes while it is alive.

## Three hidden-mode runs

| Run | Go-ready after backend startup | Mean round trip | P50 | P95 | Max sampled working set | Max sampled private bytes |
|---|---:|---:|---:|---:|---:|---:|
| 1 | 97.4 ms | 0.240 ms | 0.200 ms | 0.400 ms | 25.93 MB | 17.64 MB |
| 2 | 93.6 ms | 0.304 ms | 0.200 ms | 0.400 ms | 24.88 MB | 18.33 MB |
| 3 | 94.7 ms | 0.331 ms | 0.300 ms | 0.400 ms | 24.67 MB | 17.57 MB |
| Average / maximum | 95.2 ms | 0.292 ms | 0.233 ms* | 0.400 ms* | 25.93 MB | 18.33 MB |

*Mean of each run's p50/p95, not a percentile calculated across pooled raw samples.

Three runs each verified all 100 sequence-echo round trips and wrote a report. Process wall time, including app boot and exit delay, was around 0.88?0.92 seconds. The process exited after the report was written.

## How to interpret the measurements

- `startup_to_ready_ms` measures Wails backend startup callback to the frontend's first successful Ready call. It is not cold OS process creation time and not an end-to-end user-input metric.
- Round-trip latency includes JavaScript → Wails binding → Go method → JavaScript promise resolution. The measured median was around 0.2–0.3 ms; p95 across these small runs was around 0.3–0.4 ms.
- Working set and private bytes are maxima sampled by PowerShell at roughly 15 ms intervals. They are approximate and can miss brief peaks.
- These results are for a hidden WebView in benchmark mode. They do not measure visible rendering rate, touch latency, animation smoothness, preview FPS, production asset memory, or camera/render/output work.
- The UI build works on this Windows host; Linux runtime dependencies, launch, packaging, and hardware behavior have not been tested.

## How to rerun

From this directory:

```powershell
go test -count=1 ./...
& "$((go env GOPATH) + '\bin\wails.exe')" build -clean -nopackage
$env:PHOTOBOOTH_UI_SPIKE_REPORT = Join-Path $env:TEMP 'photobooth-kiosk-ui-spike-report.json'
& '.\build\bin\wails-v2.exe' --benchmark
Get-Content $env:PHOTOBOOTH_UI_SPIKE_REPORT
Remove-Item Env:PHOTOBOOTH_UI_SPIKE_REPORT
Remove-Item $env:TEMP\photobooth-kiosk-ui-spike-report.json
```

## Decision after this spike

Wails v2 is a plausible Windows UI prototype path for further evaluation, not yet the committed product UI. The main trade-off is the WebView2-based desktop shell and the separate Go 1.25+ toolchain requirement for this Wails release. Before adoption, compare the same interaction/session on representative low-cost hardware and test the actual Linux target distribution with GTK3/WebKit2GTK. Keep camera and session logic behind the current Go interfaces.
