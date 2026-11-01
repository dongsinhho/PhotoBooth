# Roadmap and Validation Gates

**Status:** Proposed sequence; dates intentionally unset

## Stage 0 - Self-service readiness and hardware selection
- Select a reference camera/PC/output configuration likely to be available to new booth builders.
- Prepare quickstart, troubleshooting, sample configuration, and voluntary issue/compatibility-report templates.
- Use optional user feedback when it arises; do not require formal interviews or incumbent-store migration.
- Select one camera model, one output path, a Windows baseline, and a specific Linux distribution for the technical spike.
- Verify SDK operations, supported model/OS combinations, license terms, and redistribution rules.
- Validate kiosk behavior, camera trigger/transfer, printing, auto-start, and updates.
- Decide how the pilot will configure local/LAN/cloud delivery and free/paid sessions.
- Choose a verifiable payment-confirmation method before accepting money.

**Exit gate:** self-service setup path is documented, reference hardware/OS test matrix is selected, and a measurable capture-to-output acceptance test is defined. Formal interviews are not required.

**Implementation note (2026-10-30):** added a Windows-first quickstart, symptom-based troubleshooting, a read-only environment inspector, a compatibility report template and GitHub issue forms for compatibility/bug/feature reports. The docs distinguish the exact experimental B525/Windows 11 reference from untested laptop cameras, printers and Linux. This improves the self-service/documentation/reporting portion of Stage 0; it does not complete the stage exit gate. A production installer/update path, a chosen Linux distro and printer hardware/driver target still require explicit selection and validation. See [QUICKSTART_WINDOWS.md](../QUICKSTART_WINDOWS.md), [WINDOWS_TROUBLESHOOTING.md](../troubleshooting/WINDOWS_TROUBLESHOOTING.md), [COMPATIBILITY_REPORT_TEMPLATE.md](../COMPATIBILITY_REPORT_TEMPLATE.md) and [inspect-environment.ps1](../../tools/self-service/inspect-environment.ps1).

## Stage 1 - Reliable local player
- Kiosk startup and session workflow.
- One camera provider and source-image transfer.
- Static frame/render pipeline.
- Preview and one tested output path.
- Local persistence, diagnostics, recovery, and offline operation.
- First deployment package and update/rollback plan.

**Exit gate:** repeated end-to-end sessions pass on target hardware, including failure/recovery tests.

**Implementation note (2026-10-31):** the Wails prototype now has an idle-only B525 live preview using the same Windows Media Foundation worker as capture. The UI smoke saw a real preview image, then a clean stop/close before the same three-shot session opened the camera, completed capture/render/local-save, and restarted preview when returning to idle. Preview is capped at 5 FPS with one acknowledged frame in flight and fails closed if worker cleanup is uncertain. The reference B525 also completed one physical unplug/replug during an active Wails UI session; PnP disappearance/return and the Wails recovery state were observed, and the same three-shot session auto-completed render/local-save without Continue or app restart. Three real Wails image-quality sessions also passed 9/9 captures at 1280x720. Repeat recovery cycles/ports, representative fixed-lighting portrait quality, full-day preview/kiosk soak and production UI selection, printer/export selection and laptop/Linux hardware remain open. A root-level `run-photobooth.ps1` now builds and launches the Wails-based Windows MVP UI candidate as `PhotoBooth.exe`, while keeping its Go 1.25.0 module separate from the Go 1.24.8 core. The app is clearly labeled Windows Preview; this makes the latest GUI path directly runnable but does not approve production adoption. See [ADR 0021](../adr/0021-windows-mvp-ui-candidate.md). An opt-in native Wails fullscreen mode is now available through `run-photobooth.ps1 -Kiosk`; `Esc` exits fullscreen without closing the app and `F11` toggles it. The Windows display smoke passed startup-fullscreen, Escape-to-windowed, F11-to-fullscreen and Escape-after-toggle, with the disposable profile removed. This is application fullscreen only, not Windows shell lockdown, auto-start or a secured kiosk account; see [ADR 0022](../adr/0022-windows-kiosk-mode.md). A small current-scene warm-up A/B showed only a 0.037 percentage-point relative first-shot improvement from the one-second default; keep it unchanged and see [ADR 0014](../adr/0014-warm-up-webcam-stream.md).

**Implementation note (2026-11-01):** the B525/Windows 11 Wails idle-preview kiosk candidate passed a 10.26-minute soak with 195 two-second samples, 2,825 ACKs in the initial progress counter (not the exact backend event total; this counter was later found to undercount across display-mode lifecycle resets), five Settings stop/resume cycles and five F11 fullscreen/windowed cycles. Mean/min preview rate was 4.71/4.07 FPS; there were no ACK timeout or decode-failure events. CPU process-tree mean/peak was 15.28/25% of one logical core. PhotoBooth.exe private bytes stayed around 29 MB; WebView2 private bytes peaked at 1,418 MB and ended at 1,315 MB. The WebView2 footprint rose during initial allocation then varied around 1.3–1.45 GB during the remaining run: no unbounded growth was observed in this 10-minute sample, but the footprint is high for low-RAM booth PCs and should be optimized/validated on target hardware. A post-run exact JSONL audit found 2,882 published / 2,872 ACK events (10 unmatched published sequences), despite zero ACK-timeout/decode-failure events. The old summary progress counter undercounted ACKs across lifecycle resets (reported 2,825), and the old harness inferred success from later publications; that inference was removed. The finding triggered a persistent shared-read diagnostics writer, explicit cancellation events and a stricter final JSONL audit. This initial run is superseded by the rerun documented below. The disposable profile was removed and no photo session was created. Full-day/overnight soak, low-RAM target, repeated USB recovery cycles/ports, representative portrait lighting, printer, Linux and production UI selection remain open. See [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md) and [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md).

**Implementation note (2026-11-01, rerun after preview/diagnostics hardening):** the frontend now decodes each preview JPEG to an `ImageBitmap`, draws into one persistent canvas and explicitly closes the bitmap before ACK; the canvas backing store is resized only when dimensions change. The backend keeps one mutex-serialized JSONL append handle with Windows read sharing, records explicit in-flight cancellation and closes the log on shutdown. The soak harness reads concurrent snapshots with `FileShare.ReadWrite` and performs an exact post-shutdown sequence audit; no ACK is inferred from later publications. The 10.24-minute B525 fullscreen-kiosk rerun passed 195 samples, five Settings cycles and five display cycles: 2,828 frame publications = 2,828 ACK events; zero cancellations, timeouts, decode failures, unmatched sequences, orphan ACKs, duplicates or invalid JSON lines. Average/min preview FPS 4.73/4.70; CPU process-tree mean/peak 14.50/22% of one logical core. Process-tree private bytes first/last/peak 344.54/1,130.56/1,285.69 MB; WebView2 private bytes 315.20/1,098.87/1,254.63 MB; PhotoBooth.exe private bytes peak 31.86 MB. Compared with the earlier independent 10.26-minute baseline, WebView2 private-byte peak was 11.5% lower and process-tree peak 11.2% lower; this is indicative, not a controlled A/B, and WebView2 still uses over 1.2 GB at peak. Report: `%TEMP%\photobooth-preview-soak-d9ce78c26007467fac0cee06e3f4ebb1\summary.json`. This is one 10-minute single-host/single-camera run, not full-day/low-RAM target certification. See [ADR 0023](../adr/0023-preview-renderer-and-soak-diagnostics.md).

**Implementation note (2026-11-01, WebView2 subprocess memory profiling):** the preview soak harness now classifies WebView2 subprocesses and records per-role private bytes. A diagnostic-only Wails option can disable GPU acceleration via `PHOTOBOOTH_DISABLE_WEBVIEW_GPU=1` / `-DisableWebViewGPU`; normal launches retain GPU acceleration. On ShinPc / Windows 11 / B525, one 3.15-minute GPU-disabled run recorded a 612.53 MB process-tree private-byte peak and 582.54 MB WebView2 peak, versus 1,003.94 MB and 977.52 MB in a 3.06-minute GPU-enabled run. FPS and mean CPU were similar; GPU-process private bytes fell substantially, but renderer private bytes remained about 493 MB. Both exact event audits passed and disposable profiles were removed. This is a short, single-host diagnostic comparison, not controlled benchmarking, leak diagnosis, low-RAM certification or a production default decision. Keep GPU on by default; Stage 1 remains open pending representative low-RAM hardware and broader acceptance. See [ADR 0024](../adr/0024-webview2-preview-memory-profiling.md) and [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md).

**Implementation note (2026-11-01, low-RAM validation tooling):** the Wails preview soak harness now has a read-only host preflight, an 8 GiB visible-RAM candidate filter (`-RequireLowRamHost`) and host-wide available-memory, commit and page-read telemetry in CSV/JSON/Markdown. ShinPc reports 15,484 MiB visible RAM and is correctly rejected as above the low-RAM target. A 1.02-minute instrumentation smoke verified 23 samples, but the normal soak gate failed because the short run did not complete Settings/display lifecycle cycles; it is not a passing soak. During the sample, host-wide available RAM minimum was 1,361.88 MB, commit peaked at 96.67%, `PagesInputPersec` at 4,585 and `PageReadsPersec` at 47. These counters are system-wide, not PhotoBooth-specific. An actual 8 GiB host is not connected/validated; the 8 GiB profile is a test target, not a declared product minimum. See [ADR 0025](../adr/0025-low-ram-host-readiness-and-memory-pressure.md) and [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md).

**Implementation note (2026-11-01, physical idle-preview USB recovery):** one physical Logitech B525 unplug/replug was tested while the Wails player was actively showing idle preview on ShinPc / Windows 11 build 26200. Windows PnP observed removal and return of the same device instance; the backend emitted seven `reconnecting` statuses followed by `ready`; preview advanced from 11 baseline frames/ACKs to 48/48 after reconnect. The exact post-shutdown JSONL audit found 48 frame publications = 48 ACKs, with zero cancellations, ACK timeouts, decode failures, invalid lines, duplicates, unmatched publishes or orphan terminals. No photo session was started; app shutdown was graceful and the temporary profile was cleaned. The first harness attempt had a UI Automation false negative for the transient reconnect label; the corrected harness now asserts the backend status event log. This is one reference camera/USB port/host cycle, not repeated-reliability or broad compatibility certification. See [ADR 0026](../adr/0026-idle-preview-physical-usb-recovery-validation.md), [ADR 0020](../adr/0020-idle-only-camera-preview.md) and [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md).

**Implementation note (2026-11-01, alternate USB controller coverage):** the idle-preview hotplug harness now captures Windows USB LocationPaths and parent host-controller identity. It supports `-BaselineLocationPath` to assert the current port differs from a previously validated port and `-RequireSamePortOnReconnect` to isolate recovery on that same alternate path. The original passing cycle used XHC0 / AMD USB 3.10 at `PCI(0801)#PCI(0003)#...`; Windows now reports the B525 on XHC1 / AMD USB 2.0 at `PCI(0801)#PCI(0004)#...`. Preflight confirms the paths differ. One XHC0 attempt timed out after 45s without PnP removal; two XHC1 attempts reached idle preview but Windows did not report PnP removal within 90s each, so neither exercised XHC1 recovery. Apps exited and temporary profiles were cleaned. Do not mark XHC1 recovery as pass or fail: the physical unplug precondition was not observed. See [ADR 0027](../adr/0027-usb-topology-aware-port-validation.md) and [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md).

**Implementation note (2026-11-01, post-retry next-gate validation):** the user confirmed Retry local save is operationally stable; no further retry implementation was performed. A fresh real B525 Wails batch passed 3/3 sessions, 9/9 1280x720 captures, 3/3 renders and 3/3 local outputs. First-shot drops were 0.017%, 0.110% and 0.231% (mean approximately 0.119%); the existing gross-darkness/relative-consistency validator passed without threshold changes. This is current-scene evidence only, not representative portrait/framing certification; temporary photos were deleted. Root and Wails Go tests/vet, root Windows build, root Linux/amd64 CGO-disabled compile, Vite production build and Wails Windows/amd64 production build all passed. Linux validation is compile-only. ShinPc exposes only PDF/OneNote virtual printers, so physical printing remains blocked until a target printer is selected. A real disk-full/quota VHD test was not run because the connected PowerShell identity is not Administrator; no system volume/quota settings were altered. The user-requested 8 GiB host test remains skipped and is not the next gate. See [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md), [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md) and [ADR 0019](../adr/0019-local-output-disk-full-recovery.md).

**Implementation note (2026-11-01, settings/package/restart acceptance):** the Wails Settings UI smoke now waits for `OPERATOR CONFIGURATION` and `Frame packages` after opening Settings instead of using a fixed delay that raced the idle-preview stop barrier; 8 UI assertions passed and Cancel did not save or alter the operator profile. A temporary local `.pbframe` UI workflow passed validate → install immutable version → activate / update shot count → rollback to the prior 3-shot frame → uninstall inactive package (7 checks). A two-process settings persistence smoke saved 3 shots / 4s countdown / 750ms interval, closed PhotoBooth gracefully, relaunched against the same disposable profile and verified the values reloaded in Settings. Both processes exited gracefully, no photo session was created, and temp profile/fixture were cleaned. This closes the prototype's basic settings and local frame-package UI acceptance path, and demonstrates config persistence across app restart; it does not validate installer upgrades/schema migration or production kiosk deployment. See [STORE_CONFIG.md](STORE_CONFIG.md) and [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md).

## Stage 2 - Configuration and local output
- Explicit store configuration schema and validation.
- Local-only media delivery and opt-in retention/cleanup with dry-run by default.
- Disabled/free payment path.
- Local asset/package installation and rollback.
- Payment/media modules remain replaceable and separate from capture.

**Exit gate:** settings can change modes without modifying capture/provider code; local mode never exposes photos on a network accidentally.

**Implementation note (2026-10-31):** local output now has explicit disk-full/quota classification, temporary-file cleanup and a shared save-only recovery API used by both `apps/output-retry` and the Wails **Retry local save** result-screen action. The retry republishes the preserved render without camera access, uses no-overwrite publication, and is covered by core/Wails API tests and fresh Windows builds. The Wails Retry local save button passed a disposable Windows UI Automation smoke; real ERROR_DISK_FULL/ENOSPC hardware testing remains pending. The B525 also passed one active-session USB unplug/replug cycle through Wails, while representative booth-lighting quality and repeated unplug cycles remain open. See [LOCAL_STORAGE_RECOVERY.md](LOCAL_STORAGE_RECOVERY.md), [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md) and [ADR 0019](../adr/0019-local-output-disk-full-recovery.md).

**Implementation note (2026-10-29):** schema-v1 local station configuration and CLI loading are implemented in the root Go core. Local .pbframe validate/install/activate/rollback is available through the package manager and Wails prototype. Opt-in session/output retention is implemented as a separate CLI with dry-run-by-default and a reviewed plan digest required for apply; it does not run during player startup. Full themes/catalog/signatures, LAN delivery, payment and production UI adoption remain separate work. See [STORE_CONFIG.md](STORE_CONFIG.md), [FRAME_PACKAGE_FORMAT.md](FRAME_PACKAGE_FORMAT.md), [LOCAL_DATA_RETENTION.md](LOCAL_DATA_RETENTION.md), [ADR 0015](../adr/0015-versioned-local-station-config.md), [ADR 0017](../adr/0017-local-frame-packages-and-rollback.md) and [ADR 0018](../adr/0018-opt-in-local-data-retention.md).

## Stage 3 - LAN photo access and optional payment
- Session-scoped LAN download/QR with short-lived authorization.
- Network/interface binding and guest-to-guest access protections.
- One selected payment integration if the pilot requires it.
- Server/provider-verified payment state, idempotency, expiry, duplicate/late callback handling, and reconciliation.

**Exit gate:** tests prove that one session cannot access another session's output and unpaid sessions cannot start when payment is required.

## Stage 4 - Cross-platform validation and cloud delivery
- Linux build and installation on the selected distribution.
- Camera/printer integration checks on Linux; document differences from Windows.
- Optional authenticated cloud upload/download, retention, deletion, quotas, and cost model.
- Content catalog delivery and device sync.

**Exit gate:** Linux compatibility claims cover only tested camera/OS/driver/output combinations; cloud failures never destroy local output.

## Stage 5 - Marketplace and creators
- Creator publishing, validation, previews, versioning, and moderation.
- Free/paid listings, orders, entitlements, refunds, licensing, and creator attribution.
- Payment reconciliation, support, and operational monitoring.

**Exit gate:** purchases, refunds, takedowns, content rollback, and entitlement checks are repeatable and supportable.

## Stage 6 - Growth features
- Multi-booth/store management.
- Advanced theme editor.
- Animated digital exports, promotions, scheduling, and analytics.
- Additional camera/printer providers based on observed demand.

## Cross-stage rules
- Do not start later stages just because they are technically interesting; satisfy current validation gates first.
- Keep options documented, but implement only validated needs.
- Market sizing, pricing, creator economics, cloud cost, and reliability targets should be based on actual evidence.
- Formal interviews are optional. Real hardware tests and observed product behavior are still required.

See [ADR 0005](../adr/0005-greenfield-self-service-adoption.md) for the greenfield/self-service growth strategy.


See [ADR 0006](../adr/0006-webcam-first-technical-spike.md) for the webcam-first technical spike.
