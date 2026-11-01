# UI Spike Results

**Initial spike date:** 2026-10-25
**Latest settings/package/output-recovery integration check:** 2026-10-31
**Status:** Windows shell/bridge experiment, local settings/frame-package lifecycle, and save-only output-retry API/button pass automated tests and Windows builds. The new retry button also passed a Windows UI Automation smoke with a controlled output-path obstruction. Real disk-full and active-capture USB-unplug tests remain pending. Production UI not selected.

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

A touch-first kiosk shell with four views:
- Idle/start screen with real webcam and synthetic-camera choices.
- Three-shot capture workflow with countdown, progress, and latest captured still.
- Recoverable camera-error screen with session/shot progress and operator actions.
- Result screen with the rendered photo strip, local output path, and Explorer action.

The recovery screen preserves the session ID and completed shots. Continue retries the same session and resumes from the next uncaptured shot; the core validates and reuses the previous capture files. Restart creates a clean session with a new ID, and Return to start clears the current UI state without deleting saved files.

The Wails Windows app now uses the root Go workflow runner. Start with webcam uses the experimental Windows Media Foundation camera provider plus bounded recovery, then the frame renderer and local-storage output target. The separate synthetic-camera action exercises the same render/save pipeline without opening hardware. Real webcam capture through the UI, repeated recovery and fresh-session restart have now been exercised by a two-trial automation harness in one Wails process. The night-time image-quality batch failed the gross-darkness gate; the morning retest passed the coarse darkness/first-shot-consistency gate on 9/9 captures, but a sample remains a dim room scene, not a representative photobooth portrait. The stage shows the latest still, not continuous live video. Printer, payment and cloud services are not connected.

A hidden benchmark mode loads the embedded frontend, waits until JavaScript calls the Go Ready binding, makes 100 sequential Ping calls, verifies every echoed sequence value, writes a JSON report, and closes the application. A PowerShell harness samples the executable's process working set and private bytes while it is alive.

### Real capture and recovery integration (2026-10-28)

The Windows Wails app now invokes the root Go workflow runner for real webcam capture, frame rendering and local output. Its synthetic-camera mode goes through that same render/save pipeline without accessing the webcam. Progress is sent asynchronously to the frontend; captured JPEGs appear as shot previews, and the final rendered strip is displayed on the result screen with the saved path and an Explorer action.

If capture/reconnect exhausts its retry budget, the failed-session screen offers:
- Continue after reconnect: keep the current session ID and completed JPEGs, then capture only the remaining shots.
- Start a fresh session: create a new session ID and start from shot one.
- Return to the start screen: clear the current UI state without deleting saved files.

Verification on 2026-10-29: root Go tests/vet, nested Wails tests, frontend production build and Wails Windows build passed after the camera-control wrapper fix. A two-trial UI Automation harness ran in one Wails process and passed 2/2 same-session Continue flows and 2/2 fresh-session restart flows after forcing initial camera contention; all four completed sessions produced 3 captures, render and local output. The six new session/output folders were deleted afterward. The night-time quality batch was near-black (mean luminance 2.52-2.54/255); a morning retest passed the gross-darkness/first-shot-consistency gate on 9/9 shots (session means 49.108, 50.051 and 52.515; first-shot drops <=0.122%). Visual inspection still shows a dim room rather than a representative booth/portrait scene, so photo quality is not yet certified. The UI displays the latest captured still, not continuous live video; physical unplug during active capture, actual Media Foundation read interruption, long soak and production UI selection remain open. Windows worker-level cancellation/deadline tests now verify prompt caller return, bounded Close timeout and successful second Close once a simulated native read returns.

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
## Local operator settings integration (2026-10-29)

The Wails prototype now exposes a **Settings & frame** screen backed by the root schema-v1 station-config package. The screen edits shot count (1–12), countdown, inter-shot interval, and a local frame JSON path. A native Browse dialog chooses a file without uploading it. The capture progress list and session total are dynamic; a saved two-shot profile was exercised through a two-slot synthetic capture → render → local-save workflow.

Save validates duration strings, supported station modes, the frame schema, and exact equality between shot count and frame slot count. Saving is rejected during a running session. The profile is stored in `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json` on Windows and survives restarts. A corrupt/unsupported existing profile is not silently replaced: the UI shows a repair warning and the player rejects capture until a valid profile is saved. No camera is opened by viewing settings or by the UI settings smoke test.

Validation on 2026-10-29:
- Wails tests passed, including 5 repeated executions of persistence, mismatch/repair guards and the two-shot workflow; the two-shot end-to-end workflow additionally passed 20 consecutive runs.
- Wails `go vet ./...`, frontend `npm run build` and `wails build -clean -nopackage` passed; the Wails build regenerated bindings for the new settings methods.
- `ui-settings-smoke.ps1` passed on the built Windows app. UI Automation found the shot selector (3 photos), countdown (3s), interval (1s), frame path, Browse, Save and Cancel; Cancel returned home and the existing settings profile was not modified.
- Root Go tests/vet and platform build checks are recorded separately in the core validation report.

**Scope:** this is operator configuration for the Wails prototype, not a production UI decision. It does not offer arbitrary frame editing, camera/device options, LAN/cloud delivery, payment, hot reload or live camera preview. A frame must already be a valid local schema-v1 JSON with exactly the selected number of slots. Physical camera capture and printer behavior are not exercised by the settings smoke test.
## Local frame-package lifecycle integration (2026-10-29)

The root Go core includes a limited .pbframe schema-v1 package manager and the Wails prototype exposes it under **Settings & frame → Frame packages**. The .pbframe archive contains manifest.json, one validated frame JSON definition, and at most one preview PNG/JPEG plus optional LICENSE.txt. The parser rejects path traversal, directories/symlinks, duplicate/unknown entries, malformed or unknown JSON fields, invalid digest/size/media type and over-limit archives. Payload sizes and SHA-256 are verified and the real frame renderer validates the definition before install. Package versions are immutable; re-installing the same manifest/version is idempotent, while a different manifest for an existing version is rejected.

UI lifecycle: validate package (writes nothing) → install locally (does not activate) → explicitly activate (creates previous-frame backup and pending activation state, updates config and shot count to the actual slot count) → optional rollback (verifies backup SHA-256 and schema, saves a separate restored frame file, restores profile and clears rollback metadata). The list re-verifies installed versions. Package/settings changes are rejected while a capture session is running. Profile changes through the ordinary frame-path field are not allowed to bypass package activation/rollback for managed package paths.

A real temporary two-slot .pbframe created by the CLI passed the built Windows UI Automation flow: validation, immutable local install, activation to 2 shots, and rollback to 3 shots. The package remained installed and the test used a unique PHOTOBOOTH_DATA_DIR which was removed afterward. No physical camera was opened. The screen stayed on Settings after activation/rollback.

During regression, the existing Wails demo sometimes exposed a terminal session state before its final JPEG preview data URL had been attached. The event bridge now keeps the UI in a non-terminal finalizing state until runPhotoSession publishes the final session snapshot. A dedicated terminal-state test covers all terminal states and the capture-render-save regression passed 50 consecutive executions after the fix.

Validation on 2026-10-29: root Go tests/vet, Windows build, Linux/amd64 compile-only build and 15 Python camera POC tests passed. Frame-package unit tests and CLI tests passed. Wails tests/vet, frontend production build, Windows production build, settings-only UI smoke and full frame-package UI smoke passed. The package UI smoke does not capture photos and uses an isolated temporary profile.

**Trust boundary:** SHA-256 verifies archive contents against the archive manifest, not publisher identity or legal licensing. Packages remain unsigned; full themes, arbitrary asset folders, network downloads, catalog/marketplace, payment, moderation, in-app deletion UI and production UI certification remain out of scope. Exact format is documented in docs/knowledge-base/FRAME_PACKAGE_FORMAT.md and ADR 0017.

## Guarded local frame-package uninstall (2026-10-29)

The operator can now remove an installed version from **Settings & frame → Frame packages**. This is a distinct action from rollback and deletes only the selected immutable version directory, not source frame JSON, photos, rendered strips or station settings. The backend refuses to remove the current active frame, a package referenced by pending activation, or either side of a committed rollback transition. If the rollback record is invalid, removal fails closed rather than assuming the package is unused. The package tree and rollback-state directory are checked for symlinked parent directories before activation/state reads or uninstall.

Unit coverage includes successful removal of an unused version, refusing active/pending/rollback-referenced versions, retaining data on rejected removal, removing after rollback is cleared, refusing a symlinked package parent, and rejecting symlinked rollback state. Wails API tests verify the active-package guard, session-running guard and post-rollback uninstall. The fresh Windows build passed the 8-check settings UI smoke and 7-check full package UI flow: validate → install → activate to two shots → rollback to three shots → uninstall. The smoke uses a disposable PHOTOBOOTH_DATA_DIR, does not open a camera, and removes all test artifacts.


## Local output retry in the result screen (2026-10-31)

The Wails result view now exposes **Retry local save** for a terminal session that has captured every configured shot and produced its rendered JPEG but has no published local output. The action calls the shared core `output.RetryLocalFromSession` API, so the CLI and UI use the same session/path/file checks. It never reopens the camera, captures another photo or invokes the renderer. While it runs, the result view shows a saving state and disables starting another session; backend guards reject overlapping retry, capture, settings and frame-package operations.

The shared retry API validates a 32-character lowercase hexadecimal session ID, configured shot count, sessions/output root separation, real non-symlink session/capture directories, every expected non-empty capture, and the preserved non-empty rendered.jpg. It refuses an existing photo.jpg and uses the local provider's exclusive no-replace publication (same-directory temp file, sync/close, then hard link to the final name) to protect against concurrent retries.

Validation on 2026-10-31:
- Shared core tests: success from preserved render, preserve captures/render, missing capture, invalid ID/count, overlapping roots, symlinked session, cancelled context, existing output refusal and concurrent two-call publication (exactly one success).
- Wails API tests: complete failed session retries to completed using the same ID and all original captures; rejects wrong ID, already-completed retry and incomplete captures; prevents a second retry or fresh session during retry.
- Root `go test -count=1 ./...`, `go vet ./...`, Windows build, Linux/amd64 CGO-disabled compile and the 15-test Python camera POC suite passed.
- Wails `go test -count=1 ./...`, `go vet ./...`, Vite production build and `wails build -clean -nopackage` passed. The build regenerated `RetryPhotoOutput` Go/TypeScript bindings.

**Not yet verified:** manually clicking the new retry button in a running Wails window, real disk-full/quota exhaustion, and hard-link support on non-NTFS filesystems. The Wails screen is still an experiment, not the production UI.


## Windows UI Automation smoke for local output retry (2026-10-31)

Script: `ui-output-retry-smoke.ps1`. The test data is isolated under `%TEMP%` and is automatically deleted after the run; the JSON result report is retained. Run from this module using PowerShell:

~~~powershell
.\ui-output-retry-smoke.ps1
~~~

The script creates a GUID-scoped `PHOTOBOOTH_DATA_DIR` below the Windows temp directory and a synthetic three-shot station profile (10-second session countdown to allow controlled observation). It opens the freshly built Wails executable and starts **Try without a webcam**. Once the new session directory exists, it creates a regular file at `photos/<session-id>`, deliberately preventing the output provider from creating the per-session directory while leaving the session workspace writable. Capture and rendering can finish; final local output fails with a path-type error rather than consuming disk space.

The script waits for the result view's enabled **Retry local save** button, confirms there are three captures and a non-empty `rendered.jpg` but no `photo.jpg`, removes only the obstruction it created, and invokes the retry button through Windows UI Automation. It then verifies the same session reports COMPLETED, the output exists, the final output SHA-256 equals the preserved render SHA-256, all captures/rendered.jpg remain, and the retry button is no longer enabled. It closes the app and deletes the entire isolated test data directory in a finally block.

Run on 2026-10-31: **PASS**. Session `4f988fc35b89d3201003c63800bb44c6`; three synthetic captures; output size 139,879 bytes; rendered/output SHA-256 `18E6D86D0909380256D0DD2DFD86D3F95ACE5C1766ED967F81B47D26CFE33FCC`; temp test profile was removed. Full report was written under `%TEMP%\photobooth-output-retry-smoke-349431a8a6334ddc90a461ad51b59bb2.json`.

**Interpretation:** this is a real Windows GUI/button smoke, but the induced failure is a controlled filesystem path obstruction, not an actual OS disk-full/quota condition. No physical camera was opened. Real ERROR_DISK_FULL/ENOSPC and active Wails-camera USB unplug/replug remain pending.


## Physical B525 hot-plug UI smoke harness (2026-10-31)

Script: ui-usb-hotplug-smoke.ps1. It creates a disposable PHOTOBOOTH_DATA_DIR, launches the Wails app with the real Windows Media Foundation camera provider, waits for a first B525 capture and the second-shot countdown (the camera remains open and the photo session is still active), then asks the operator to unplug the B525 USB cable. It waits for Windows PnP to report the device absent and for the Wails recovery/failure state before prompting the operator to reconnect. It then verifies automatic recovery or a retryable failure followed by same-session Continue, and requires three captures, rendered output and local photo before reporting PASS. All test photo data is removed in a finally block; the JSON diagnostic report is retained.

Run from this module:

~~~powershell
.\ui-usb-hotplug-smoke.ps1 -UnplugObservationTimeoutSeconds 45
~~~

**Execution status on 2026-10-31:** harness syntax/preflight passed and Windows detected exactly one B525 device (USB\VID_046D&PID_0836&MI_02\8&195090D4&0&0002, status OK). Two interactive attempts were started; neither attempt observed the device disappear (first timeout 15s, second timeout 45s). Both app processes exited, both disposable data roots were removed, and the B525 remained present/OK. Therefore, **no physical unplug/replug recovery result has been obtained**; this is not a camera recovery failure. The next run requires a person at ShinPc to unplug the B525 when the script prints PHYSICAL ACTION and reconnect it when it prints RECONNECT_B525_NOW.


## Active-session physical USB recovery and current-scene quality checks (2026-10-31)

One physical B525 unplug/replug during an active Wails capture passed. The result screen surfaced the recovery state; Windows PnP observed the device disappear and return; the existing three-shot session auto-completed render and local output without pressing Continue or restarting Wails. Session `82d6d3bdd45d682019bd4407310bc212`; report `%TEMP%\photobooth-usb-hotplug-smoke-0b988aaca6fb4c65acd9633f4ee5e849.json`. This is one cycle for the B525/Windows host and not a repeated-cycle or broad compatibility certification.

Three real Wails camera sessions at the normal 3-second countdown / 1-second interval completed 9/9 captures at 1280x720 plus render/local save. First-shot drop was 0.000%, 0.000% and 0.660%; report `%TEMP%\photobooth-wails-quality-7a8bef8468e04c7691e3da93e39a6cf6.json`. A separate warm-up A/B diagnostic compared 3 sessions without warm-up against 3 with the one-second default (18 captures total): mean signed first-shot drop 0.047% vs. 0.010%, a small 0.037 percentage-point difference. The default warm-up remains unchanged; fixed-lighting portrait quality is still unvalidated. See `docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md` and ADR 0013/0014.


## Idle live camera preview with exclusive capture handoff (2026-10-31)

The experimental Wails home/idle screen now renders live B525 preview from the same Windows Media Foundation `captureWorker` used by session capture. There is no second camera stream. The optional `camera.PreviewProvider` extension returns a validated JPEG from the active stream; Media Foundation preview and capture share the same frame read/normalization path and operation mutex.

Preview runs only while idle and is stopped/closed before real or demo capture, operator Settings, and application shutdown. The capture start path waits for the preview stop barrier. A close failure remains latched and blocks future camera sessions rather than opening a second provider whose ownership is uncertain. Returning to idle restarts preview.

To avoid WebView backlog, at most one frame is published at a time. Each frame has a sequence; the frontend decodes the JPEG to a temporary `ImageBitmap`, draws into a persistent canvas, explicitly closes the bitmap, then acknowledges. The next frame is read only after that ACK and a 200ms inter-frame delay (5 FPS maximum). If the UI does not ACK within 2 seconds, preview stops and presents Retry. The canvas is not stored in session/output directories.

**Physical validation on 2026-10-31:** `ui-camera-preview-smoke.ps1` passed on Logitech B525 / Windows 11. The Wails UI rendered a real preview before capture; pressing Start while preview was active handed exclusive ownership to the normal workflow; the same app completed 3/3 capture, rendered the strip and saved local output; selecting Take another set returned to idle and preview rendered again.

- Report: `%TEMP%\photobooth-wails-preview-2f6b66c9f67e4e55a66bf190ba6ffa1e.json`.
- Session: `e0e7d34c6cee2ef62df8db51bff05d81`.
- `preview_before_capture=true`, `preview_after_capture=true`, `capture_count=3`, `rendered=true`, `local_output=true`, `temp_data_cleaned=true`.
- Preview cap/limits: 5 FPS max, 2-second ACK timeout, 7-second stop barrier.

**Scope:** one functional preview/capture/preview-return UI smoke with the reference B525. This does not validate multi-hour preview stability, a different camera or USB port, Linux, production UI selection, or representative portrait lighting. The implementation deliberately does not run preview and capture simultaneously; capture has priority.


## Final smoke rerun after ACK and startup-cancellation hardening (2026-10-31)

After the latest source changes (duplicate-ACK guard, no ACK on WebView decode failure, and synchronous cleanup when Media Foundation startup is cancelled), the Wails production executable was rebuilt and the physical B525 preview smoke was run again.

- Result: PASS.
- Report: `%TEMP%\photobooth-wails-preview-713d7c864cf1438cb4ebd1a21397b379.json`.
- Session: `f3d6a96b74a68168ea896435e3997b41`.
- `preview_before_capture=true`, `preview_after_capture=true`, `capture_count=3`, `rendered=true`, `local_output=true`, `temp_data_cleaned=true`.
- Preview max rate 5 FPS, 2-second ACK timeout, 7-second stop barrier.
- The isolated profile and real camera test images were removed; only the JSON report remains.

This is the final smoke evidence for the latest build. It verifies one functional idle-preview / exclusive-capture-handoff / return-to-idle cycle on B525, not a multi-hour preview soak, other webcams/ports, or production UI certification.


## Windows fullscreen kiosk mode (2026-10-31)

The Windows Wails candidate now supports an explicit kiosk launch path without changing the default windowed workflow.

- Launch fullscreen: from the repository root run `./run-photobooth.ps1 -Kiosk`.
- Exit fullscreen without quitting PhotoBooth: press **Esc**.
- Toggle fullscreen from either windowed or fullscreen mode: press **F11**.
- The on-screen hint `KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE` is visible only while fullscreen is active.
- The root launcher sets `PHOTOBOOTH_KIOSK` for the child process and restores its own PowerShell environment after process creation. Normal launch explicitly selects windowed mode; benchmark mode cannot accidentally start fullscreen.
- This uses Wails native fullscreen/unfullscreen APIs. It does not disable Windows system shortcuts, Alt+Tab, Ctrl+Alt+Del, taskbar behavior, account access or the Windows shell; it is not Windows Assigned Access/Shell Launcher.

**Windows UI smoke on 2026-10-31:** `ui-kiosk-mode-smoke.ps1` passed on ShinPc / Windows 11 / primary display. It verified fullscreen startup, Escape to windowed, F11 to fullscreen and Escape again to windowed. It did not start a capture session and produced no guest photos. Temporary profile data was deleted. Report: `%TEMP%\photobooth-kiosk-smoke-61b1b02bca074449b0c3cee0c1fbecc9.json`.

**Root-launcher handoff check:** `run-photobooth.ps1 -SkipBuild -Kiosk` also started the built `PhotoBooth.exe` fullscreen; the persistent kiosk hint was detected; Escape returned the app to windowed mode and the same process remained running.

The smoke covers one Windows host/display. Multi-monitor targeting, touch-only exit/maintenance UX, Windows login auto-start, OS-level shell lockdown, long-duration kiosk/preview soak and production installer/update remain open. See [ADR 0022](../../../docs/adr/0022-windows-kiosk-mode.md).


## Ten-minute idle preview / kiosk soak (2026-11-01)

Added `ui-preview-kiosk-soak.ps1` as the repeatable long-lived UI test. It defaults to 10 minutes in fullscreen kiosk mode, samples every 2 seconds, and performs a Settings stop/resume plus F11/windowed/fullscreen round-trip every 2 minutes. It launches with a unique temporary station profile, refuses to run beside another PhotoBooth UI process, performs no capture, and removes the temporary profile after graceful shutdown. A 3,072 MB process-tree private-bytes guard is recorded and stops the run if the test threatens to consume excessive memory.

**Physical result on ShinPc / Windows 11 / Logitech B525: PASS**
- Actual duration 10.26 minutes; 195 resource samples.
- The initial progress counter showed 2,825 ACKs; a later exact JSONL audit found 2,882 published / 2,872 ACK events / 10 unmatched publishes (no timeout/decode failures). The progress counter undercounted across display-mode lifecycle resets and must not be treated as the exact backend total. Average preview rate 4.71 FPS, minimum sampled 4.07 FPS.
- No ACK timeout or frame-decode failure.
- Five Settings open/Cancel cycles passed, including preview stop before Settings and preview resumption on return to Idle.
- Five F11 fullscreen/windowed cycles passed in the same app process.
- Maximum observed ACK p95 9.17 ms; maximum frame-gap p95 215.22 ms.
- CPU process-tree mean/peak 15.28% / 25% of one logical core.
- Whole process-tree working set first/last/peak 517 / 1,288 / 1,434 MB; private bytes 331 / 1,344 / 1,448 MB.
- `PhotoBooth.exe` working set first/last/peak 45 / 46 / 48 MB; private bytes 28.9 / 29.1 / 31.1 MB.
- WebView2 working set first/last/peak 472 / 1,241 / 1,388 MB; private bytes 302 / 1,315 / 1,418 MB.
- WebView2 memory increased during initial allocation, then varied around 1.3–1.45 GB during the remaining run. This did not show unbounded growth during this 10-minute sample, but it is a high memory footprint and needs review before claiming suitability for low-RAM booth PCs.
- Post-run exact JSONL audit: 2,882 published / 2,872 ACK / 0 cancellation events, leaving 10 published sequences without a matching ACK or cancellation record. No timeout/decode events were present. Do not infer these missing audit records as successful ACKs; this was the observability defect addressed by the later renderer/diagnostics hardening.
- Temporary profile cleaned: true; no guest images/session were created.

Reports retained under `%TEMP%\photobooth-preview-soak-1b9787abe95f4b978f08339cb895c715\`: `summary.json`, `report.md`, `process-tree-resources.csv`, `process-details.csv`, and `preview-events.jsonl`.

**Scope:** one Windows host, one B525, one 10-minute run. This does not replace a full-day/overnight soak, representative portrait/lighting evaluation, other cameras/USB ports, printer testing, Linux runtime testing, or production UI selection. The 10-minute run is a positive first soak gate, not Stage 1 exit-gate certification.


## Preview renderer and diagnostics hardening (2026-11-01)

The 10-minute baseline exposed a high WebView2 memory footprint and incomplete ACK audit records. The renderer and soak instrumentation were hardened before a same-host rerun.

- The idle preview now uses one persistent canvas. A JPEG is decoded to an `ImageBitmap`, checked again after async decode so a late frame cannot draw after leaving Idle, copied into the canvas, and explicitly closed before the backend ACK.
- Canvas dimensions are changed only if the camera resolution changes; stop/unavailable clears the backing store.
- The backend reuses one mutex-serialized append-only diagnostics handle for the opt-in soak. It closes on path change, failed write or app shutdown; failed opens/writes are counted and a later record retries the open.
- Windows opens the JSONL with read/write/delete sharing; the PowerShell sampler reads point-in-time snapshots using `FileShare.ReadWrite`. Unix-like builds retain an append-only `os.OpenFile` path.
- Preview cancellation emits `frame_cancelled` unless an ACK was already queued; cancellation does not masquerade as timeout or an ACK failure.
- The soak never infers ACK success from a later sequence. It reconciles every published sequence against ACK/cancel records after graceful shutdown; unmatched sequences, orphan/duplicate events and invalid JSON lines fail the run. Summary ACK totals are derived from exact backend ACK events rather than lifecycle-relative progress counters.

### Repeat 10-minute physical soak

**Result: PASS** on ShinPc / Windows 11 / Logitech B525, fullscreen kiosk.

- Actual duration 10.24 minutes; 195 resource samples at 2-second intervals.
- Exact backend event audit: 2,828 published / 2,828 ACK / 0 cancelled. Zero unmatched publishes, orphan ACKs, duplicate events, invalid JSON lines, ACK timeouts or decode failures.
- Five Settings stop/resume cycles and five fullscreen/windowed cycles passed.
- Preview mean/min FPS 4.73 / 4.70; final preview 4.78 FPS.
- CPU process-tree mean/peak 14.50% / 22% of one logical core.
- Whole process-tree private bytes first/last/peak 344.54 / 1,130.56 / 1,285.69 MB.
- WebView2 private bytes first/last/peak 315.20 / 1,098.87 / 1,254.63 MB.
- PhotoBooth.exe private bytes first/last/peak 29.33 / 31.69 / 31.86 MB.
- Temporary profile removed; no photo session or guest images were created.

Compared with the previous independent 10.26-minute baseline on the same host/camera and with the same sample/cycle schedule, the new run observed an 11.5% lower WebView2 private-byte peak (1,254.63 vs. 1,418.22 MB) and an 11.2% lower process-tree private-byte peak (1,285.69 vs. 1,447.70 MB). CPU mean was 14.50% vs. 15.28% of one logical core. This comparison is indicative, not a controlled A/B; each value comes from one run and startup/allocation timing can vary. The >1.2 GB WebView2 peak remains high for low-RAM target PCs.

Report: `%TEMP%\photobooth-preview-soak-d9ce78c26007467fac0cee06e3f4ebb1\summary.json`. Resource CSV, per-process CSV and full JSONL event stream are retained alongside it.

**Next validation:** repeat the soak on an actual low-RAM target PC, then run a full-day/overnight soak. The 10-minute gate is a positive single-host milestone, not production certification or the Stage 1 exit gate. See [ADR 0023](../../../docs/adr/0023-preview-renderer-and-soak-diagnostics.md).


### Final soak-harness audit verification (2026-11-01)

After moving the final PASS/FAIL decision until after shutdown and exact JSONL reconciliation, the updated harness was run once more for a 3.15-minute verification on the physical B525:

- 73 resource samples, fullscreen kiosk, three Settings stop/resume cycles and three fullscreen/windowed cycles.
- 868 frame-published events = 868 ACK events; zero cancellation, unmatched publish, orphan ACK, duplicate event or invalid JSON line.
- Final preview FPS 4.92; mean/min 4.79 / 4.76 FPS.
- CPU process-tree mean/peak 11.86% / 21% of one logical core.
- Process-tree private bytes peak 1,010.39 MB; WebView2 private bytes peak 981.21 MB; PhotoBooth.exe private bytes peak 29.72 MB.
- Temporary profile cleaned; exit code 0.

Report: `%TEMP%\photobooth-preview-soak-076f91305c8e433fbc0dd804a7b8e382\summary.json`.

This short run verifies the final report/audit path; it does not replace the 10.24-minute memory comparison or a full-day soak.

## WebView2 subprocess memory profiling and GPU diagnostic toggle (2026-11-01)

The preview soak harness now records each WebView2 subprocess role (browser, renderer, GPU, utility and other), PID/parent PID, and per-role private-byte samples. Role first/last/peak aggregates are included in the JSON/Markdown summary as well as the per-process and resource CSVs. Private bytes are process allocations; they are not dedicated GPU memory/VRAM.

The Wails Windows options expose a diagnostic-only switch: `PHOTOBOOTH_DISABLE_WEBVIEW_GPU=1`, or `-DisableWebViewGPU` on `ui-preview-kiosk-soak.ps1`. It maps to Wails v2.17.0 `windows.Options.WebviewGpuIsDisabled`. The normal app and root launcher keep hardware acceleration enabled unless this environment variable is explicitly set; no user-facing setting was added.

A same-host B525/Windows 11 comparison passed the exact preview event audit in both modes:

| Metric | GPU disabled | GPU enabled (default) |
|---|---:|---:|
| Duration / resource samples | 3.15 min / 70 | 3.06 min / 69 |
| Settings / display cycles | 3 / 3 | 2 / 2 |
| Preview FPS mean / min | 4.75 / 4.71 | 4.75 / 4.71 |
| CPU mean / peak (% of one logical core) | 13.96 / 27 | 13.78 / 22 |
| Process-tree private bytes peak | 612.53 MB | 1,003.94 MB |
| WebView2 private bytes peak | 582.54 MB | 977.52 MB |
| GPU-process private bytes peak | 33.20 MB | 576.38 MB |
| Renderer private bytes peak | 493.21 MB | 479.01 MB |
| Published / ACK / cancelled | 865 / 863 / 2 | 848 / 848 / 0 |
| Audit gaps | 0 | 0 |

The diagnostic run substantially reduced the measured total private-byte peak, while renderer private bytes remained around 0.5 GB. FPS and average CPU were similar in these short samples. They are not a controlled benchmark: initial allocation baselines differed, the exact durations and lifecycle-cycle counts differed, and only one host/camera was tested. This does not establish a leak, a minimum RAM requirement, or suitability for low-RAM PCs. Keep GPU acceleration enabled by default until the behavior is checked on representative target hardware and the full operator/capture experience is reviewed.

See [ADR 0024](../../../docs/adr/0024-webview2-preview-memory-profiling.md) and the retained reports under the host's `%TEMP%\photobooth-preview-soak-…` directories.

## Low-RAM host preflight and system memory-pressure telemetry (2026-11-01)

The preview soak harness now records the host hardware/OS and samples system-wide memory counters alongside the PhotoBooth/WebView2 process tree:
- Visible physical RAM, available RAM in MB and as a percentage of total.
- Committed bytes, commit limit and commit percentage.
- `PagesInputPersec` and `PageReadsPersec` from `Win32_PerfFormattedData_PerfOS_Memory`.

The report writes these values per sample in `process-tree-resources.csv` and summarizes low-water/peak values in `summary.json` and `report.md`. If the Windows memory performance class is unavailable, the script records available RAM from `Win32_OperatingSystem` and leaves unavailable commit/page-read counters blank.

The intended next test profile is a Windows 11 x64 booth PC with 8 GiB RAM. This is a validation target, not an official minimum specification. Use:
```powershell
.\ui-preview-kiosk-soak.ps1 -PreflightOnly
.\ui-preview-kiosk-soak.ps1 -DurationMinutes 10 -RequireLowRamHost
```
`-RequireLowRamHost` refuses to start the camera/app when Windows reports more than 8,192 MiB visible physical memory. The threshold can be explicitly changed with `-LowRamMaxPhysicalMemoryMB` for a separately documented hardware class. Preflight is read-only and does not open the camera.

On 2026-11-01, ShinPc (ASRock X600-ITX, Ryzen 5 8600G/Radeon 760M, Windows 11 Pro build 26200) reported 15,484 MiB visible RAM and was correctly classified `above-low-ram-target`. The low-RAM guard rejected it before camera/app startup.

A 1.02-minute instrumentation smoke on ShinPc recorded 23 samples and preview metrics averaging 4.72 FPS with no ACK/decode failures. The regular soak gate reported FAIL because this short duration did not complete the required Settings and display lifecycle cycles; it is not a passing soak. System-wide host metrics during the sample showed available RAM first/min 1,699.20/1,361.88 MB (8.80% minimum), commit peak 96.67%, `PagesInputPersec` peak 4,585 and `PageReadsPersec` peak 47. These counters are host-wide and do not attribute all memory pressure to PhotoBooth. The temporary profile was cleaned.

Report: `%TEMP%\photobooth-preview-soak-db22bee67dbb4b3480b4289eb859213f\summary.json`.

**Not validated:** an actual 8 GiB host, low-RAM UI responsiveness, capture/render/local-output under memory pressure, or a final minimum hardware specification. Do not use ShinPc as a substitute for that hardware test. See [ADR 0025](../../../docs/adr/0025-low-ram-host-readiness-and-memory-pressure.md).

## Physical USB recovery during idle live preview (2026-11-01)

Harness: `ui-idle-preview-hotplug-smoke.ps1`. Unlike the active-capture hot-plug test, this test starts the experimental Wails UI with an isolated local profile, waits for acknowledged idle preview frames, and asks the operator to physically unplug/replug the B525. It does not start a photo session or save guest photos. It observes the original Windows PnP instance, reads backend preview state transitions from the shared-read JSONL, verifies preview frame/ACK progress after reconnect, closes the app gracefully, and audits every published frame sequence after shutdown.

Run:
```powershell
.\ui-idle-preview-hotplug-smoke.ps1 -PreflightOnly
.\ui-idle-preview-hotplug-smoke.ps1 -UnplugObservationTimeoutSeconds 45
```
The full run requires a person at the machine to unplug the B525 when prompted and reconnect it only after `RECONNECT_B525_NOW` is printed. It refuses to run beside another PhotoBooth UI process.

**Physical run on 2026-11-01 — PASS:** ShinPc / Windows 11 Pro build 26200 / Logitech B525 on the reference USB port. PnP observed the original B525 instance disappear and return with the same instance ID. Wails emitted seven `reconnecting` status events and then a `ready` status after reconnect. Preview advanced from 11 baseline frames/ACKs to 48 frames / 48 ACKs after reconnect; sampled resumed FPS during the short window was 2.32. Post-shutdown exact audit: 48 published = 48 ACKs; zero cancellations, ACK timeouts, decode failures, invalid JSON lines, duplicate publishes/terminal sequences, unmatched publishes or orphan terminals. No photo session was created; the app exited gracefully and the temporary profile was removed. The camera was enumerated healthy after the run.

Report: `%TEMP%\photobooth-idle-preview-hotplug-c151fb720c924e19857e3013a1a4bb36.json`.

**Harness note:** the first physical attempt detected PnP removal and its JSONL recorded reconnecting/ready events, but the test failed because UI Automation did not expose the transient reconnect label. It was not counted as a hardware failure or a pass. The harness was corrected to wait for backend `preview_status` events in JSONL and the second physical attempt passed. The status-event log, not a UIA text scrape, is the authoritative state-transition assertion.

**Scope:** one physical cycle, one B525, one USB port, one Windows 11 host, experimental Wails build. This does not certify repeated cycles, additional ports/cameras, capture during preview (capture is exclusive and was not run), full-day operation, portrait quality, Linux or production UI readiness. Repeat cycles and other hardware remain open.

## USB topology-aware validation of an alternate controller path (2026-11-01)

The idle-preview hotplug harness now records Windows PnP location paths, location info, USB parent and host-controller identity. It supports:
- `-BaselineLocationPath`: requires the camera's initial attached path to differ from a previously validated port.
- `-RequireSamePortOnReconnect`: requires the device to return to that same alternate path after unplug.
- `-RequireDifferentPort`: separately requires the post-unplug path itself to differ from the pre-unplug path.

The last passing physical idle-preview hotplug (ADR 0026) used:
- XHC0 / AMD USB 3.10 host controller.
- `PCIROOT(0)#PCI(0801)#PCI(0003)#USBROOT(0)#USB(1)#USB(2)#USBMI(2)`.

Windows later reported the B525 attached to a distinct path:
- XHC1 / AMD USB 2.0 host controller.
- `PCIROOT(0)#PCI(0801)#PCI(0004)#USBROOT(0)#USB(1)#USB(4)#USB(3)#USBMI(2)`.

Preflight with the original XHC0 path supplied to `-BaselineLocationPath` passed and reported `baseline_port_different=true`. The script can now run the recovery test on XHC1 while requiring replug on that same alternate path.

**Alternate-path recovery is not yet validated:** one XHC0 run reached idle preview but Windows did not report PnP removal within 45 seconds. After the B525 was attached on XHC1, two XHC1 runs reached idle preview (5–6 frames/ACKs) but Windows did not report PnP removal within 90 seconds each. All three attempts were marked FAIL at the unplug precondition; neither XHC1 run reached recovery or replug. The reports confirm no photo session, graceful app shutdown, temporary-profile cleanup and B525 still healthy on XHC1. These are not recovery failures and do not prove a port incompatibility.

Reports:
- 45-second attempt: `%TEMP%\photobooth-idle-preview-hotplug-5c75e6ecadc44920af1b0c6d0ca96433.json`
- 90-second attempt: `%TEMP%\photobooth-idle-preview-hotplug-9010cd33183642a1852ca0815925894b.json`
- XHC1 same-port reconnect attempt: `%TEMP%\photobooth-idle-preview-hotplug-cf6df3a56cf7406f906840c028e7f64c.json`

Use the current camera path as test baseline, and when ready physically unplug the USB cable from the PC port (not the webcam-side connector). Reconnect to the same XHC1 socket after the script prints `RECONNECT_B525_NOW`.

See [ADR 0027](../../../docs/adr/0027-usb-topology-aware-port-validation.md).

## Wails quality rerun and current regression/build matrix (2026-11-01)

A fresh real B525 Wails UI batch completed 3/3 sessions and 9/9 1280x720 captures. Every session rendered and saved `photo.jpg` locally. The unchanged quality validator passed:
- Session mean luminance: 122.758 / 122.661 / 122.738.
- First-shot drop: 0.017% / 0.110% / 0.231% (mean approximately 0.119%).
- Dark-pixel fraction: 0.151–0.169%; clipped-pixel fraction 0.654–0.681%.
- Laplacian variance: 12.720–13.095 (descriptive only; no sharpness gate is defined).
- Temporary photos/session profile were removed; metrics-only report remains at `%TEMP%\photobooth-wails-quality-92009cc35d5444d29f236fa624a50ed9.json`.

This indicates stable exposure across the three shots in the current scene. It does not certify portrait quality or framing: lighting/subject/distance were not instrument-controlled and the captured images were deleted without human visual review.

Regression/build verification on 2026-11-01 after the user confirmed Retry local save was stable:
- Root `go test -count=1 ./...` — PASS.
- Root `go vet ./...` — PASS.
- Wails module `go test -count=1 ./...` — PASS.
- Wails module `go vet ./...` — PASS.
- Root Windows `go build ./...` — PASS.
- Root Linux/amd64 `CGO_ENABLED=0 go build ./...` — PASS (compile-only; no Linux camera/kiosk runtime claim).
- Frontend `npm.cmd run build` — PASS.
- Wails v2.17 Windows/amd64 production build — PASS; `build/bin/PhotoBooth.exe` refreshed.

Retry local save is treated as operationally stable per user confirmation; no retry-specific code changes were made in this batch. Real physical full-volume/quota exhaustion remains a separate validation item. The current Desktop Commander PowerShell process is not elevated (not Administrator), so a bounded VHD-based Windows disk-full integration test cannot be created/mounted safely from this session. No system-volume quota or free-space settings were altered.

Windows printer inventory: only Microsoft Print to PDF and Microsoft OneNote virtual queues are present. There is no physical printer model/driver to target, so no physical printer adapter or print result is claimed.


## Operator settings and frame-package UI validation (2026-11-01)

The Wails Settings smoke was hardened to wait for the actual `OPERATOR CONFIGURATION` and `Frame packages` headings after opening Settings. This replaces a fixed 400 ms delay that raced the idle-preview stop/close barrier and could query the accessibility tree before the Settings view was rendered. It still does not click Save or modify the operator's real profile. The rerun passed 8 checks: settings controls and current values were exposed, local config path and package manager were displayed, and Cancel returned home without saving.

A temporary `.pbframe` fixture was generated from `examples/frames/photo-strip-v1.json` using the local frame-package CLI, then validated. The Wails UI package smoke passed 7 checks through validate → install immutable version → activate (slot count persisted) → rollback (default 3-shot frame restored) → uninstall inactive package. No camera capture was started; disposable profile and package fixture were removed.

## Station settings persistence across application restart (2026-11-01)

Added `ui-settings-persist-restart-smoke.ps1`. It uses a temporary `PHOTOBOOTH_DATA_DIR`, changes countdown to 4s and interval to 750ms (three shots), saves the schema-v1 station config, closes the first `PhotoBooth.exe` gracefully, starts a second process with the same profile, opens Settings and confirms the values were reloaded. Result PASS: shot count 3 / countdown 4s / interval 750ms; both processes exited gracefully; no photo session was created; temporary profile removed. Metrics/report: `%TEMP%\photobooth-settings-restart-6d4b3d0f640f448aa8ec9345a4194462.json`.

This validates local configuration persistence across an application process restart in the experimental Wails player. It does not test installer upgrades, migration between schema versions, Windows login auto-start or a production kiosk deployment.
