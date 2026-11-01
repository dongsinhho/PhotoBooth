# PhotoBooth Kiosk UI Spike (Wails v2)

This is an isolated engineering spike, not the selected production UI framework. The Windows build now connects the webcam button to the real Go session runner, Media Foundation camera provider, frame renderer and local-storage output. The UI remains a prototype; physical printer, payment and cloud services are not connected.

## What it tests

- A touch-first desktop WebView shell with idle, real/synthetic capture, recoverable camera-error, and result views.
- A Go binding for real and synthetic photo sessions, progress/recovery events, and session controls.
- An operator **Settings & frame** view backed by the versioned root config package: shot count, countdown, inter-shot interval and local frame JSON path.
- A local **Frame packages** manager for .pbframe validation, immutable install, activation, and rollback; installation never activates a frame implicitly.
- Save-time frame-schema and frame-slot/count validation, local atomic persistence, and visible repair warnings for invalid station profiles.
- A benchmark mode that starts a hidden window, performs 100 sequential UI-to-Go round trips, writes a JSON report, then exits.
- External process sampling can record working-set/private-byte high-water marks without changing the core app.
- Camera, workflow, rendering and local output are owned by the root Go core; the UI calls those packages through a narrow Wails binding.

## Toolchain note

This spike uses Wails v2.17.0, which requires a newer Go toolchain than the core's Go 1.24.8 setting. Keep it isolated under this submodule: do not change the root go.mod to make the spike build. Re-evaluate the UI framework/version and toolchain cost before adopting it in the product. Wails v2 Linux builds additionally require GTK3 and WebKit2GTK development/runtime packages; Windows success alone is not Linux UI validation.

## Run a visible demo window

From this directory:

```powershell
wails dev
```

Click **Settings & frame** from the home screen to configure 1–12 shots, countdown, inter-shot interval and the local frame JSON. The frame must validate and contain exactly the configured number of slots; use **Browse…** to choose a local JSON file. Save is rejected during a running session, and invalid saved settings are never silently overwritten. The Wails profile is stored at `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json`; the first launch installs the embedded sample frame under `%LOCALAPPDATA%\PhotoBooth\frames\`. Settings persist after restart. Cloud/LAN and payment settings are not offered because those modes are not implemented.

Click **Start with webcam** to open a Windows Media Foundation camera and run the count/timing defined by the saved profile. The result screen displays the saved strip and can reveal the file in Explorer. If capture/reconnect exhausts its retry budget, use Continue to preserve existing shots and retry only unfinished shots, or start a fresh session. The idle/home screen shows a live B525 preview on the Windows Media Foundation path. Preview uses the same worker as capture, is capped at 5 FPS with a single acknowledged frame in flight, and is fully stopped/closed before a session starts or operator Settings opens; it restarts when returning to idle. The capture stage displays the latest captured still.

Optional Windows kiosk mode is enabled by launching the root script with `-Kiosk`, which sets `PHOTOBOOTH_KIOSK=1` for that process only. Kiosk startup is fullscreen; **Esc** exits fullscreen without closing the app, and **F11** toggles fullscreen from either windowed or fullscreen mode. The persistent on-screen hint explains the shortcuts. Normal windowed launch remains the default. The UI mode is not an OS shell replacement: Windows task switching, security shortcuts, taskbar behavior and account policy remain controlled by Windows. Use **Try without a webcam** to exercise the same workflow with synthetic images; these are visibly marked as demo mode.

The same screen has a **Frame packages** area. Choose a .pbframe file, validate it, install it locally, then explicitly activate a listed package. Activation adjusts the shot count to the validated frame slot count and preserves the prior frame for rollback. Rollback restores a verified copy while keeping installed package versions. Remove deletes only an inactive package version, and the core refuses it while the version is needed by the active profile, a pending activation, or rollback. Package v1 is offline and declarative only; it has no scripts, themes, remote downloads, signatures or marketplace. See [FRAME_PACKAGE_FORMAT.md](../../../docs/knowledge-base/FRAME_PACKAGE_FORMAT.md).

Settings-screen UI smoke (opens the settings view, checks accessible controls, then cancels without saving or capturing photos):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\ui-settings-smoke.ps1
```

The smoke sets a unique temporary PHOTOBOOTH_DATA_DIR and removes it on exit; it does not touch the operator's real profile or open a camera. It waits for the Settings heading after opening the view so the idle-preview stop/close barrier has completed before accessibility assertions run.

To verify local settings persistence across a real app process restart (save in the first PhotoBooth.exe process, close it, start a second process against the same disposable profile, and verify values reload):
~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\ui-settings-persist-restart-smoke.ps1
~~~

This test saves 3 shots / 4s countdown / 750ms interval, verifies the schema-v1 JSON, then confirms the same values are visible in Settings after restart. It does not start a photo session, does not touch the real user profile, and removes its temporary profile. The JSON report remains under %TEMP%.

Full local frame package UI smoke (pass a valid test .pbframe; the script validates, installs, activates, rolls back and uninstalls in a disposable profile without capture):

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\ui-frame-package-smoke.ps1 -PackagePath C:\path\to\sample.pbframe
~~~

The package smoke also uses a unique temporary PHOTOBOOTH_DATA_DIR and cleans it on exit. Create fixtures with the root CLI, documented in docs/knowledge-base/FRAME_PACKAGE_FORMAT.md.

## Build and measure the bridge

```powershell
wails build -clean
$env:PHOTOBOOTH_UI_SPIKE_REPORT = Join-Path $env:TEMP 'photobooth-kiosk-ui-spike-report.json'
& '.\build\bin\PhotoBoothKioskSpike.exe' --benchmark
Get-Content $env:PHOTOBOOTH_UI_SPIKE_REPORT
```

The benchmark report records the UI-ready time and p50/p95/mean binding latency. The external PowerShell harness should sample the executable's working set and private bytes while it runs. These results measure the shell and binding only; they do not measure the real camera path, frame renderer, printer, or a production kiosk session.


## Real B525 camera quality smoke

Run three real Windows Media Foundation B525 sessions from the built Wails UI, at the normal 3-second countdown and 1-second inter-shot interval:

~~~powershell
.\ui-camera-quality-smoke.ps1 -Sessions 3
~~~

The script uses an isolated temporary station profile, Windows UI Automation and a real B525. It checks all 3 captures, render and local output per session; measures image dimensions/luminance/contrast/sharpness/dark/clipped pixels; retains the JSON metrics under `%TEMP%`; and deletes temporary images and profile. Keep camera aim, lighting and scene unchanged during the run. The diagnostic thresholds are not a portrait-quality certification.

On 2026-10-31, 3 sessions / 9 captures passed. All captures were 1280x720; all renders and local outputs succeeded; first-shot drop was 0.000%, 0.000% and 0.660%. Report: `%TEMP%\photobooth-wails-quality-7a8bef8468e04c7691e3da93e39a6cf6.json`.

Physical B525 unplug/replug during an active Wails session also passed one cycle on 2026-10-31; Windows PnP and Wails recovery state were observed and the same three-shot session auto-completed without Continue. Repeat cycles/ports and representative booth portrait lighting remain open; see [CAMERA_RELIABILITY_RESULTS.md](../../../docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md).


## Windows kiosk-mode smoke

Build the current executable from the repository root, then run from this directory:

~~~powershell
.\ui-kiosk-mode-smoke.ps1
~~~

The smoke launches an isolated fullscreen instance, verifies that its window fills the primary display, sends **Esc** and checks that the app returns to a windowed state, sends **F11** and checks fullscreen again, then exits with **Esc**. It does not start a photo session or create guest photos. It uses a unique temporary `PHOTOBOOTH_DATA_DIR` and deletes it after a graceful app shutdown; only a JSON report remains in `%TEMP%`.

This validates application fullscreen and the keyboard escape path on the tested Windows display. It does not disable Windows hotkeys, prevent Alt+Tab or Ctrl+Alt+Del, configure Windows Assigned Access/Shell Launcher, auto-start on login, or certify a tamper-resistant kiosk.

## Idle preview / kiosk soak

Build the current Windows executable from the repository root, connect the reference Logitech B525, then run from this directory:

~~~powershell
.\ui-preview-kiosk-soak.ps1
~~~

Defaults: 10 minutes, fullscreen kiosk, 2-second process/resource samples, and one lifecycle cycle every 2 minutes. Each cycle opens operator Settings (the preview must stop and release the camera), cancels back to Idle (preview must resume), then toggles F11 to windowed and back to fullscreen. The script does **not** start a photo session or capture guest images.

The soak refuses to start beside another PhotoBooth/Wails UI process because the preview owns the reference camera exclusively. It creates a disposable station profile, checks that the same B525 remains present, records the PhotoBooth process and WebView2 process tree separately, measures CPU/working set/private bytes, and validates preview FPS, frame progress, ACK timeouts, decode failures, and the one-frame backpressure bound. A 3,072 MB process-tree private-bytes safety stop closes the test app gracefully if memory keeps growing. The temporary profile is removed after a graceful exit; only the JSON/Markdown summary, resource CSVs, and preview JSONL diagnostics remain under %TEMP%.

Useful variants:

~~~powershell
.\ui-preview-kiosk-soak.ps1 -Windowed
.\ui-preview-kiosk-soak.ps1 -DurationMinutes 2 -LifecycleCycleMinutes 1
.\ui-preview-kiosk-soak.ps1 -DurationMinutes 15 -MaxProcessTreePrivateMB 4096
~~~

ACK accounting uses the serial preview pump: a later frame cannot be published until the previous ACK has been consumed. If a separate JSONL ACK audit record is missing but later sequence publication continues, the harness records an audit gap rather than misclassifying the frame as stuck. A frame timeout, decode failure, missing progress, more than one in-flight frame, or low FPS still fails the gate. The JSONL ACK-audit gaps should be investigated independently; they are not silently counted as errors or discarded from the report.

**Reference run (ShinPc / Windows 11 / Logitech B525, 2026-11-01):** 10.26 minutes, 195 samples, 2,825 effective ACKs, five Settings cycles and five fullscreen/windowed cycles passed. Mean preview FPS was 4.71 (minimum 4.07); no ACK timeout or decode failure occurred. CPU tree mean/peak was 15.28/25% of one logical core. PhotoBooth.exe private bytes stayed around 29 MB; WebView2 private bytes peaked at 1,418 MB and ended at 1,315 MB. Memory rose during initial WebView2 allocation then varied around 1.3–1.45 GB for the remainder; this is a high footprint, not a low-memory certification. Maximum backend ACK-audit gaps: 3. Report and limitations: [UI_SPIKE_RESULTS.md](UI_SPIKE_RESULTS.md) and [CAMERA_RELIABILITY_RESULTS.md](../../../docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md).

This is a single-host 10-minute smoke/soak, not an overnight/full-day soak, a multi-camera test, Linux validation, printer validation, or production UI certification.

## Idle live camera preview smoke

Build the Wails executable, connect the reference Logitech B525, then run:

~~~powershell
.\ui-camera-preview-smoke.ps1
~~~

The smoke runs the real idle-preview path through the Wails UI, waits for an actual decoded preview image, starts one real three-shot session while preview is active, verifies capture/render/local output, returns to idle and verifies that preview resumes. It uses a unique temporary `PHOTOBOOTH_DATA_DIR`, deletes all test photos/profile data, and retains only a JSON summary under `%TEMP%`.

Architecture: preview is idle-only and shares the exact Windows Media Foundation worker used by capture. The UI acknowledges a frame only after image load/decode succeeds; a decode error is not ACKed. The backend does not read/publish the next frame until that acknowledgement. Stale/duplicate ACKs are rejected. The interval caps publication at 5 FPS and a 2-second timeout stops a stalled preview. Preview closes before capture, Settings or app shutdown; if native cleanup cannot be confirmed within 7 seconds, capture fails closed rather than opening another camera provider. The same UI screen exposes a retry state when the camera is unavailable.

On 2026-10-31, the physical B525 smoke passed: preview rendered before capture, one three-shot capture session completed with 3/3 captures, render and local output, then preview rendered again after returning to idle. Report: `%TEMP%\photobooth-wails-preview-2f6b66c9f67e4e55a66bf190ba6ffa1e.json`. This is one functional UI smoke; it is not a full-day preview soak or production UI certification. See [ADR 0020](../../../docs/adr/0020-idle-only-camera-preview.md).
