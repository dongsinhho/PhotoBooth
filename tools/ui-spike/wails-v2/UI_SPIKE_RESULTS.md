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

## Portrait-quality harness artifact retention (2026-11-02)

The Wails real-camera quality harness `ui-camera-quality-smoke.ps1` now supports an explicit `-KeepArtifacts` switch for the pending representative portrait/framing review.

Run from `tools/ui-spike/wails-v2` after arranging the intended booth lighting, camera distance and framing:

~~~powershell
.\ui-camera-quality-smoke.ps1 -KeepArtifacts
~~~

Behavior:
- The harness prefers the current launcher output `build/bin/PhotoBooth.exe`; it falls back to `wails-v2.exe` only when the current output is absent.
- Without `-KeepArtifacts`, the isolated TEMP profile is removed as before.
- With `-KeepArtifacts`, the isolated data root is preserved after the app exits. It contains the captured source JPEGs, rendered strip, local output and disposable station config.
- The JSON report includes an artifact manifest with the root/session/output directories and paths for captures, rendered images and local outputs. The inventory also includes partial sessions when a run stops before completing.
- The console prints `ARTIFACT_ROOT`, `SESSION_ARTIFACTS`, `LOCAL_OUTPUT_ARTIFACTS` and the report path when data is retained. Remove the printed artifact root manually after visual review.

Validation on 2026-11-02:
- PowerShell AST parser: PASS; `KeepArtifacts` is present in the parsed parameter block.
- Report metadata serialization: PASS, including the early-failure ordered-dictionary report path.
- Isolated retention simulation: PASS for preserved data when requested and deletion when not requested.
- Artifact inventory simulation: PASS for both completed and partial session directories.
- `git diff --check`: PASS.

No physical-camera quality batch was run as part of this tooling change. The portrait/framing acceptance gate remains **open** until a controlled real B525 capture batch is visually inspected. Passing the numeric validator alone is not portrait certification.


## Alternate XHC1 idle-preview physical recovery (2026-11-02)

A physical same-port unplug/replug cycle was finally observed on the alternate XHC1 path. This run started with the B525 on XHC1 (`PCI(0801)#PCI(0004)#...`), verified that it differed from the ADR 0026 XHC0 reference (`PCI(0801)#PCI(0003)#...`), and required the camera to return to the exact same XHC1 path.

**Physical actions and PnP: PASS.** Initial idle preview reached 6 frames / 5 ACKs (11/11 immediately before unplug). Windows observed physical removal, Wails emitted `reconnecting`, and Windows then observed the same B525 instance returning on the same XHC1 location-path set (`USB_SAME_PORT_RECONNECT=True`).

**Preview recovery: FAIL.** The preview did not return to `ready`. Windows Media Foundation reopen exhausted 30 attempts with `Windows Media Foundation found no camera devices`, then emitted `unavailable`. The event log recorded 31 reconnecting events and no ready-after-reconnecting event. `preview_after_reconnect` is null. Harness exit code was 1.

**Event audit:** 374 JSONL events; 169 published frames = 169 ACKs; zero cancellations, ACK timeouts, decode failures, duplicate publishes/terminal sequences, unmatched published sequences, orphan terminal sequences or invalid JSON lines. This validates ACK accounting only; it does not offset the failed preview recovery.

No photo session was started, the application was not force-stopped, and the disposable TEMP profile was cleaned. No app code was changed as part of this physical test.

Report: `%TEMP%\photobooth-idle-preview-hotplug-35f48692d9e549f38eca89b68d54219d.json`.

Diagnostics: `%TEMP%\photobooth-idle-preview-hotplug-35f48692d9e549f38eca89b68d54219d.jsonl`.

The outcome is specific to one B525 / one Windows 11 host / one XHC1 path / one physical cycle. It does not establish an XHC1 hardware defect or broad camera incompatibility. Compare the Media Foundation provider reopen lifecycle against the passing XHC0 reference run before changing retry policy or claiming a port-level cause.


## Per-attempt preview recovery diagnostics (2026-11-02)

The XHC1 failure showed that the opt-in preview JSONL captured generic `reconnecting` status messages and the final failure string, but dropped the `Err` carried by each `recovery_open_attempt_failed` event. As a result, the previous physical run cannot establish whether all 30 attempts failed identically or whether the failure mode changed during retries.

Updated `cameraPreviewDiagnosticRecord` and `handlePreviewRecoveryEvent`:
- Recovery events (`recovery_started`, `recovery_open_attempt_failed`, `recovery_provider_opened`, `recovery_succeeded`, `recovery_failed`) are now written as explicit JSONL records.
- Each record preserves `attempt`, `total` and the full event error text when provided.
- Existing preview status events and UI messages are unchanged. Logging remains opt-in through `PHOTOBOOTH_PREVIEW_METRICS_PATH`; no normal guest-session log is created by default.
- Retry count/delay, provider open/close behavior, capture warm-up and frame/ACK flow are unchanged.

Validation: `gofmt` completed; `go test ./...` passed from the Wails submodule; root `go test` for `internal/booth/camera/recovery` and `internal/booth/camera/windowsmf` passed; `go vet ./...` passed from the Wails submodule. The new unit test verifies that attempt 7/30 and its exact error are written before the unchanged `preview_status` event.

The instrumented Windows executable has not yet been rebuilt or run through another physical cycle. The next hardware run must use a freshly built `PhotoBooth.exe` and inspect all per-attempt error records before proposing a provider behavior change.


## Media Foundation enumeration diagnostics after XHC1 recovery failures (2026-11-02)

A further observer/control run did not observe physical B525 removal within 90 seconds, so it did **not** exercise the recovery path. While the B525 stayed connected, the independent observer successfully enumerated exactly one `B525 HD Webcam` in all 151 samples (500 ms interval); the Wails preview stayed `ready` with 444 received/decoded frames and 444 ACKs. The app exited normally and its disposable profile was cleaned. Report: `%TEMP%\photobooth-idle-preview-hotplug-f31d4b6a61ca4ab7a0b22cb8ee8437fc.json`.

Additional non-hotplug controls after the XHC1 failure:
- A standalone Media Foundation init/enumeration found one B525 while Windows PnP reported `Status=OK`.
- A second standalone enumeration found one B525 while `PhotoBooth.exe` was actively holding the normal idle preview.
- Twelve consecutive `MF.Init → MF.EnumDevice → MF.Shutdown` cycles on one locked OS thread all found one B525.
- Twelve more cycles, each run on a newly locked worker OS thread, all found one B525.

Code inspection found that the vendored `camera/v1.EnumDevice` released and skipped any activation source when `callGetBaseInfo` failed. If every source failed metadata extraction, it returned an empty list with no error, indistinguishable from a native enumeration that returned zero sources. Implemented a diagnostic distinction:
- `EnumDevice` now returns a contextual joined error when one or more activation sources were returned but metadata extraction failed for every source. The error preserves the failed source index and underlying metadata error.
- A genuine empty result remains an empty list; the Windows Media Foundation provider now reports this explicitly as `MFEnumDeviceSources returned zero video capture sources`.
- Added unit tests for the all-metadata-failed case and for preserving a true empty enumeration.
- Updated `third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md` to document the local compatibility patch.

Validation after the code change: vendored `go test ./camera/v1 ./windows/mf` PASS; root `go test ./...` PASS; root `go vet ./...` PASS; Wails submodule `go test ./...` and `go vet ./...` PASS. A fresh Wails Windows/amd64 production build via `run-photobooth.ps1 -NoLaunch` also PASS; it regenerated bindings and compiled `build/bin/PhotoBooth.exe`. No app was launched by the build.

**Not yet physically validated:** the new classification has not yet been exercised during an XHC1 unplug/replug. The next physical same-port run must use this freshly built executable and inspect whether the retry error is `MFEnumDeviceSources returned zero video capture sources` or a contextual metadata-extraction error. Do not change retry timing or claim an XHC1 hardware/driver cause until this result is known.


**Post-change connected-camera smoke (2026-11-02):** after rebuilding the Wails executable, a standalone Go probe compiled against the updated vendored `camera/v1.EnumDevice` returned `MF_ENUM_COUNT=1`, `MF_DEVICE_0_NAME=B525 HD Webcam` and a non-empty symbolic link while the B525 was attached on XHC1. This confirms the improved error handling preserves normal connected-device enumeration; it does not exercise hotplug recovery.


## Third instrumented XHC1 idle-preview cycle: automatic recovery vs. Retry Connect (2026-11-02)

A third physical same-port unplug/replug cycle on alternate XHC1 was run with the freshly rebuilt PhotoBooth.exe containing per-attempt telemetry and the updated vendored Media Foundation enumeration classification.

**PnP and port identity: PASS.** Windows observed removal and reappearance of the same B525 instance. Both LocationPaths before and after were identical, and the path remained distinct from the XHC0 reference baseline.

**Automatic preview recovery: FAIL.** The initial preview reached 5 frames / 5 ACKs and had reached 10/10 immediately before unplug. The first post-unplug read returned MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED (0xC00D3EA2), entering reconnecting. All 30 automatic reopen attempts then returned the explicit message: Media Foundation MFEnumDeviceSources returned zero video capture sources. The backend emitted recovery_failed and moved to unavailable. This confirms the native zero-source enumeration branch, not the all-activation-metadata-failed branch.

**Manual Retry Connect: PASS.** The operator clicked Retry Connect after automatic recovery had failed. A new preview start appeared at 16:26:34.102 UTC; ready followed at 16:26:36.305 UTC (about 2.2 seconds). Preview resumed with 16 received/decoded frames and 16 ACKs at 5.15 FPS. This proves the manual retry path can restore preview after the automatic recovery window, but it does not satisfy the automatic hotplug-recovery gate. The final automatic attempt was around 16:26:01 UTC, so the manual retry occurred roughly 33 seconds later. This timing motivates a concurrent external enumeration observer; it does not yet prove that extending the retry budget is the right fix.

**Event audit and cleanup: PASS.** The corrected report records 228 JSONL events, zero invalid lines, 79 published frames = 79 ACKs, and zero cancellations, ACK timeouts, decode failures, duplicate publishes/terminal sequences, unmatched published sequences or orphan terminals. No photo session was started, the app was not force-stopped, and the temporary profile was cleaned.

Report: %TEMP%\photobooth-idle-preview-hotplug-02a775bf7a90498798f83990dd6292c6.json (status PARTIAL: automatic recovery failed, operator Retry Connect restored preview).

Diagnostics: %TEMP%\photobooth-idle-preview-hotplug-02a775bf7a90498798f83990dd6292c6.jsonl.

The original harness incorrectly treated any ready event after reconnecting as an automatic-recovery pass. Its JSON report was corrected to PARTIAL. The harness audit now records recovery_started, recovery_open_attempt_failures, recovery_failed, recovery_succeeded, automatic_recovery_status, preview_restarted_after_recovery_failure and ready_after_recovery_failure. It classifies a later restart/ready after recovery_failed as PARTIAL rather than PASS. Existing ACK audit remains mandatory for both PASS and PARTIAL. PowerShell AST parsing, audit against this actual JSONL and a synthetic automatic-recovery-success case passed. The retry policy and camera provider lifecycle were not changed.

**Next diagnostic:** run a separate timestamped Media Foundation enumeration observer using the vendored camera/v1.EnumDevice code at a fixed interval (about 500 ms) concurrently with one more controlled XHC1 cycle. Correlate when the external process first sees the B525 with each in-process reopen attempt. Keep manual Retry Connect untouched until the observer has recorded the transition. If the external process sees the device only after the 30-attempt window, assess a bounded retry adjustment; if it sees the device while PhotoBooth still sees zero sources, investigate per-process/native worker lifecycle and COM/thread ownership. Do not change retry timing before this evidence is available.


## Standalone Media Foundation enumeration observer (2026-11-02)

Added a separate Windows Go diagnostic under tools/diagnostics/mf-enumeration-observer. It polls the vendored windows-camera-go camera/v1 API in an independent process. Each sample performs Init → EnumDevice → Shutdown on a single locked OS thread; it does not open a capture stream or create a PhotoBooth session. JSONL records UTC timestamps, elapsed time, Init/enumeration/sample durations, status, device count, friendly name, symbolic link and distinct initialization/enumeration errors. It defaults to 500 ms, flushes each line, and refuses to overwrite an existing output file.

Validation: go test ./..., go vet ./... and Windows/amd64 build passed; unit cases cover zero-source vs. one/multiple-device status classification. A 5-second connected-camera control on the B525 produced 11/11 camera_found samples, each containing exactly one B525. Mean sample spacing was 500.1 ms; sample work took 2.2–9.2 ms. Report: %TEMP%\photobooth-mf-enumeration-control-20261102-234251.jsonl. This is a stationary-camera control and does not yet establish when Media Foundation enumeration returns after a physical disconnect/reconnect.

Next use: start the observer for 3 minutes before the next physical XHC1 hotplug run, then correlate its timestamped native enumeration samples with the in-process 30 reopen attempts. Do not manually click Retry Connect until the external observer has had time to record the first non-zero result.


## Long-lived observer zero-source state vs. fresh process after XHC1 replug (2026-11-02)

A further physical XHC1 unplug/replug was attempted with the standalone observer running concurrently. The Wails harness started on XHC1 and idle preview reached 6 frames / 5 ACKs, but Windows did not report the B525 instance disappearing within the harness's 90-second unplug-observation window. The harness therefore ended at its unplug precondition (exit code 1), cleaned its disposable profile, and did not exercise PhotoBooth recovery. This must not be counted as another hotplug-recovery failure.

The independent observer continued to run across the physical removal and return:
- It recorded 253 consecutive camera_found samples (one B525) before the first zero_sources sample at 2026-11-02T16:48:11.256Z (sample 254; observer elapsed 126.5 s).
- It then recorded 348 consecutive zero_sources samples through the end of its 5-minute run at 2026-11-02T16:51:04.756Z. Enumeration calls completed in roughly 1.5–7.3 ms; this was not an enumeration timeout.
- After the operator reconnected the same cable, Windows PnP reported the exact MI_02 B525 instance Present=True, Status=OK, Problem=CM_PROB_NONE, with the original XHC1 location paths.
- The long-lived observer process continued to report zero sources after PnP returned. A brand-new observer process started after reconnect immediately found one B525 in all 21/21 samples over 10 seconds. This is strong evidence that the stale enumeration state belongs to the long-lived process/runtime (or its original thread), rather than the camera simply remaining absent at the OS PnP layer. It does not yet distinguish process-scoped state from thread-scoped COM/MF state.

Fresh-process report: %TEMP%\photobooth-mf-enumeration-fresh-process-20261102-235210.jsonl.

The observer has now been extended with an optional -thread-probe-trigger-file mode. While the primary observer goroutine remains locked to its original OS thread and continues sampling, creating the configured trigger file causes one serialized sample to run on a second, distinct locked OS thread in the same process. The JSONL records thread_role=primary_locked_thread or secondary_locked_thread. The sample is serialized against the primary loop because the vendored camera library keeps process-global MF/COM handles. The connected-camera trigger control passed: 17 primary samples and exactly one secondary sample all found one B525. Go tests, vet and Windows build passed.

**Next diagnostic:** repeat one controlled XHC1 hotplug with this two-thread observer active. After reconnect and PnP Present=True, wait for the primary thread to return zero_sources, then create the trigger file. If the secondary thread finds the B525 while the primary remains at zero, the evidence points to thread-scoped state. If both threads remain zero but a fresh process succeeds, the evidence points to state retained at the long-lived process/runtime level. Do not change retry timing or production lifecycle until this comparison is observed.


## 2026-11-03 - Same-process thread comparison and recovery lifecycle fix (physical validation pending)

The enhanced observer completed the primary-vs-secondary thread comparison on one long-lived process. After the exact Logitech B525 MI_02 instance returned to PnP Present=True / Status=OK, the primary locked OS thread remained at zero_sources while one serialized enumeration on a distinct secondary locked OS thread in the same process found one B525 (`camera_found`, sample timestamp `2026-11-03T14:10:21.6721024Z`). This strongly supports thread-scoped COM/Media Foundation state rather than process-wide state or a missing PnP device. The simultaneous Wails harness run did not produce valid recovery telemetry because it stopped on the configured port-identity precondition; it is not counted as another recovery failure.

A narrow Windows MF retry lifecycle implementation now:
- reuses a provider instance across retryable failed Open calls;
- finishes native cleanup on the failed worker, then keeps that worker locked to its OS thread so the next recovery attempt runs on another OS thread;
- releases the previous retained worker only after the replacement worker completes MF startup, or on cancellation/exhaustion/final Close;
- uses atomic retain state and idempotent thread release to avoid cancellation/Close races;
- preserves a provider reference if its final cleanup fails so Close can be retried safely.

The recovery retry count/delay, port validation, and XHC0 behavior were not changed. Tests cover provider reuse, exhaustion cleanup, cleanup retry, thread retention and cancelled startup. Root, vendored camera and Wails test suites and root/Wails vet all pass. The final Windows Wails build was produced with `run-photobooth.ps1 -NoLaunch` using Wails CLI v2.17.0 and Go 1.25.0; output `build/bin/PhotoBooth.exe` is a valid 13,024,768-byte Windows PE executable.

**Not yet physically validated:** the B525 remains disconnected at the latest checkpoint. Reconnect it to the same alternate XHC1 socket, confirm exact PnP identity and LocationPath, preflight the freshly built executable, then run a same-port idle-preview physical cycle. Do not count the prior malformed Wails launch (which failed before preview or unplug) as a recovery failure. Three actual automatic XHC1 recovery failures remain confirmed; the fix must pass a physical cycle before the gate can be marked complete.

**Implementation/validation note (2026-11-03, recovery thread lifecycle follow-up):** the fifth confirmed XHC1 same-port physical cycle still failed automatic preview recovery after PnP returned the exact B525 MI_02 instance on the same XHC1 LocationPaths. All 30 reopen attempts returned MFEnumDeviceSources returned zero video capture sources; the UI entered unavailable. The audit retained 125 received/decoded frames and 125 ACKs with no frame pipeline errors; no photo session started and the app exited gracefully. Reviewing the failure path showed that the recovery decorator closed the original active preview worker via ordinary Close() before retry; that path released the thread, while the prior implementation only retained threads belonging to failed Open() attempts. Added optional RecoveryCloseProvider.CloseForRecovery(); the Windows MF backend now releases native stream resources on the original thread but retains that thread while replacement Open attempts use different OS threads. Retained threads are released on replacement startup success, exhaustion, cancellation or final Close. The recovery decorator falls back to ordinary Close for providers without the optional interface. No retry timing/budget or XHC0 behavior changed. Focused and full Go tests, vendored MF tests, Wails tests, root/Wails vet and git diff --check passed. New binary build hash: 58EEA311A7297B5371C2ED490BD2D6FAD90F1FCD026187B36F193816D9CF3B4B. A new same-port physical cycle is running; keep the result pending until automatic recovery telemetry and resumed frames/ACKs prove it.

### Addendum - 2026-11-03: preserve provider instance across the first recovery Open

- Reviewing the new recovery-close implementation exposed an ownership gap: after CloseForRecovery, the decorator cleared p.active and called openFresh with no candidate, so its first recovery attempt could create a fresh provider instance. That would strand the original provider's retained OS-thread list and prevent that same backend from releasing the original preview thread after a replacement succeeded.
- Added openFreshWithCandidate(ctx, recovering, initialCandidate). Capture and Preview recovery seed it with the provider whose active stream was closed. When that provider implements RetryableOpenProvider, it is reused for the first replacement Open and for subsequent failed-open retries. Context cancellation or exhaustion invokes ordinary Close on that same candidate, releasing retained threads; successful startup releases the retained threads from inside the Windows backend.
- The recovery test now verifies the same provider is reopened (factory called once, provider Open called twice) after CloseForRecovery and returns a fresh preview frame. Windows thread-retention and focused tests pass.
- A pending harness using the previous binary was deliberately stopped before any USB unplug was observed; PhotoBooth was closed with WM_CLOSE and its disposable temp profile was manually cleaned. This is not a physical recovery attempt and does not change the five confirmed failure count.
- Latest full root/vendored/Wails tests, root and Wails vet all pass (CHECKS_FAILED=0). Latest Wails build is 13,029,888 bytes, PE header MZ, SHA-256 BF6B34306D10D70D9C8C414CBB79A994EAA558CF1E8FF6B670110F4E3DB744BB. Preflight passes with B525 MI_02 Status=OK on XHC1, distinct from the XHC0 baseline. No new physical cycle has started yet; camera remains connected.


## Checkpoint - 2026-11-04: close activated Media Foundation source correctly; XHC1 reconnect window inconclusive

### Implementation: explicit IMFMediaSource shutdown

- Investigation after the in-process post-failure probe found that the vendored `Device.CloseDevice()` released `IMFSourceReader`, media type, presentation descriptor and `IMFMediaSource`, but never invoked `IMFMediaSource::Shutdown()` on the activated source.
- Added `Device.CloseDeviceWithError()`: release source reader/media type/presentation descriptor first, call `IMFMediaSource.Shutdown()`, then release the source COM reference and activation reference. Cleanup continues to Release even if Shutdown reports an error; all cleanup errors are preserved with `errors.Join`. Existing `CloseDevice()` remains source-compatible and logs cleanup errors. Partially initialized `OpenDevice()` failures now use the same cleanup path.
- The Windows MF capture worker calls `CloseDeviceWithError()` on its owning locked OS thread before `mf.Shutdown()`; source cleanup errors are logged. Added unit tests proving Shutdown precedes Release and Release still runs when Shutdown fails.
- Important: this is a native lifecycle correction supported by API contract/code inspection, but its physical hotplug benefit is **not yet validated**.

### Production build and test status

- Root `go test -count=1 ./...`: PASS.
- Vendored camera module `go test -count=1 ./camera/v1 ./windows/mf`: PASS.
- Wails submodule `go test -count=1 ./...`: PASS.
- Root and Wails `go vet ./...`: PASS. Direct vendored-module vet still emits five existing `unsafe.Pointer` warnings in `windows/mf/IMFMediaBuffer.go`, `windows/mf/mf.go`, and `camera/v1/IMFSourceReaderCallback.go`; these are outside the lifecycle change.
- `git diff --check`: PASS (Git only warns about its configured LF-to-CRLF conversion).
- Rebuilt using `run-photobooth.ps1 -NoLaunch`, Wails CLI 2.17.0 / Go 1.25.0. Binary: `tools/ui-spike/wails-v2/build/bin/PhotoBooth.exe`, 13,046,784 bytes, PE header `4D-5A-90-00`, SHA-256 `213FD95900E8BF83A974E399BD3EE399C659394953A99574026029DB17E00E02`. Build-only; app not started by build.
- Added an early harness guard that rejects the mutually exclusive flags `-RequireDifferentPort` and `-RequireSamePortOnReconnect`. The guard's explicit negative test passed. A prior mistaken invocation with both flags was stopped before any unplug; PhotoBooth closed gracefully and its temporary profile/log/report were cleaned. It is not a physical test.

### Physical XHC1 cycle after source-shutdown change

- Correct harness invocation used the baseline XHC0 LocationPath plus `-RequireSamePortOnReconnect` only (not `-RequireDifferentPort`). Preflight passed: exact B525 MI_02 was healthy on the alternate XHC1 path, distinct from the XHC0 baseline; preflight did not start the app or open the camera.
- Wails idle preview reached 5 frames / 5 ACKs at 5.95 FPS; before unplug it reached 10 / 10 ACKs at 5.33 FPS. No ACK or decode failures occurred.
- Windows PnP removal was observed, and the backend emitted `reconnecting`. All 30 automatic reopen attempts returned `MFEnumDeviceSources returned zero video capture sources`. The post-failure probe ran on OS thread 5736, different from all 31 retained preview/retry worker threads, and returned `zero_sources devices=0`. Backend entered `unavailable`.
- Harness then waited 60 seconds for PnP to report a healthy B525 but timed out: report fields `physical_replug_observed=false`, `camera_instance_id_after=null`, `camera_topology_after=null`, `same_port_reconnect_verified=false`. Thus the test did **not** establish when the camera reappeared relative to the 30 retries or within that 60-second replug-observation window. Treat this as an inconclusive replug gate, not as a confirmed post-reconnect automatic-recovery failure. No photo session started, app was not force-stopped, and temporary profile was cleaned. The frame audit remained consistent: 114 published = 114 ACKs, zero ACK timeout/decode failures/duplicates/orphans.
- After the harness ended, a fresh PnP query found the exact B525 MI_02 instance `Status=OK`; its current LocationPaths still matched the same XHC1 socket recorded before unplug (`PCI(0004)...USB(4)#USB(3)...` and the ACPI XHC1 path). A fresh standalone Media Foundation observer then found `B525 HD Webcam` in 21/21 samples over 10 seconds (first sample 7.4 ms; subsequent samples roughly 1.6–3.0 ms). This proves the camera was available to PnP and a new MF process after the harness window, but not that it was available during the automatic retry window.
- Report: `%TEMP%\\photobooth-idle-preview-hotplug-224f91e3673c4e7f886c61a19ffd2a0f.json`; JSONL: `%TEMP%\\photobooth-idle-preview-hotplug-224f91e3673c4e7f886c61a19ffd2a0f.jsonl`. Independent observer: `%TEMP%\\photobooth-mf-fresh-after-shutdown-fix-20261104.jsonl`.

### Next diagnostic gate

- Do not claim the source-shutdown fix passed physical hotplug, and do not count this cycle as a confirmed same-port post-reconnect failure: PnP did not return within the harness's 60-second window.
- Camera is currently connected and healthy on the same XHC1 path. Before another unplug, inspect the recorded cleanup warning output if available and verify a fresh Wails process can open the now-present camera. For the next controlled cycle, run an independent MF observer concurrently before unplug and extend the harness reconnect observation to `-ReconnectTimeoutSeconds 120`; correlate the observer's first `camera_found` timestamp with all 30 in-process retries. Keep recovery budget/timing unchanged until this timing is measured.
- All changes remain uncommitted/unpushed. No user changes were reset or discarded.


### Follow-up smoke after the delayed PnP return (2026-11-04)

- After the preceding harness had timed out waiting for reconnect, Windows PnP reported the same exact B525 MI_02 healthy on the XHC1 LocationPaths. A new standalone MF observer found the camera in 21/21 samples.
- A separate fresh PhotoBooth Wails process was then launched against an isolated disposable profile while that same B525 remained on XHC1. It reached `FRESH_PROCESS_PREVIEW_READY=True` with 30 frames / 30 decoded / 30 ACK, 4.99 FPS, zero ACK failures and zero decode failures. No photo session directory was created. The app closed gracefully and the temporary profile was cleaned; smoke process exit code 0.
- This confirms a fresh PhotoBooth process can open a capture stream after the delayed device return. It does not prove the original long-lived process could have recovered automatically, and it does not establish whether the camera returned just after or later than the harness's 60-second replug observation limit.
- The remaining discriminating experiment is one controlled physical cycle with a standalone MF observer started before unplug, recording the first post-return `camera_found` timestamp, and a longer 120-second harness reconnect-observation window. Keep the 30-attempt policy unchanged until the correlated timeline is available.


## Windows Preview packaging and current-scene quality continuation (2026-11-05)

The separate internal release tooling now lives under tools/release/windows; see its README for the package/install/rollback commands. The scripts produce an unsigned Windows x64 ZIP with a manifest and checksum, verify package contents and executable integrity, install immutable version directories, switch the active pointer atomically, and preserve the previous version for rollback. Lifecycle coverage uses synthetic MZ-header fixtures and never launches PhotoBooth. It passed initial install, update, rollback, active version/hash verification, corrupted manifest-binary hash rejection, checksum sidecar rejection, and protected user-data path rejection. A package made from the current real PhotoBooth.exe also passed install/hash verification in a disposable TEMP root without launching the app. This does not constitute a signed installer, public distribution, automatic updates, or production deployment approval.

The 2026-11-05 B525 UI quality smoke completed three sessions / nine captures, render and local output in every session. The unchanged quality validator failed all nine captures for gross darkness (session mean luminance 2.504–2.530/255 against 20/255 minimum). Relative first-shot drops were 0.000–0.296%; this run did not reproduce a material first-shot difference. The inspected first capture was nearly black and all real-photo artifacts were retained under %TEMP%\photobooth-wails-quality-cb632a1a1cd94153b78e20c2c946a07e. This is not portrait/framing acceptance because the scene lighting was not controlled. Do not alter the one-second warm-up based on this result. No recovery or long-run tests were run in this continuation.
