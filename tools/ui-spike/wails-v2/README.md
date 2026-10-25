# PhotoBooth Kiosk UI Spike (Wails v2)

This is an isolated engineering spike. It is not the production PhotoBooth UI and is not connected to a physical camera, printer, payment service, or the main session runner.

## What it tests

- A touch-first desktop WebView shell with three views: idle, simulated capture, and result.
- A Go binding for the demo session state and a small bridge ping method.
- A benchmark mode that starts a hidden window, performs 100 sequential UI-to-Go round trips, writes a JSON report, then exits.
- External process sampling can record working-set/private-byte high-water marks without changing the core app.
- Camera/render/output ownership remains outside this UI module.

## Toolchain note

This spike uses Wails v2.17.0, which requires a newer Go toolchain than the core's Go 1.24.8 setting. Keep it isolated under this submodule: do not change the root go.mod to make the spike build. Re-evaluate the UI framework/version and toolchain cost before adopting it in the product. Wails v2 Linux builds additionally require GTK3 and WebKit2GTK development/runtime packages; Windows success alone is not Linux UI validation.

## Run a visible demo window

From this directory:

```powershell
wails dev
```

Click **Start a session** to run a purely simulated three-shot flow. No camera is opened and no guest photo is captured.

## Build and measure the bridge

```powershell
wails build -clean
$env:PHOTOBOOTH_UI_SPIKE_REPORT = Join-Path $env:TEMP 'photobooth-kiosk-ui-spike-report.json'
& '.\build\bin\PhotoBoothKioskSpike.exe' --benchmark
Get-Content $env:PHOTOBOOTH_UI_SPIKE_REPORT
```

The benchmark report records the UI-ready time and p50/p95/mean binding latency. The external PowerShell harness should sample the executable's working set and private bytes while it runs. These results measure the shell and binding only; they do not measure the real camera path, frame renderer, printer, or a production kiosk session.
