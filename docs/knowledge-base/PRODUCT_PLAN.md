# Product Plan: From Idea to Commercial Launch

Status: Proposed execution plan
Last updated: 2026-10-31
Planning horizon: 12-16 weeks to a private pilot is an initial estimate, not a commitment. Hardware and SDK issues may change it.

## Strategy

Build a dependable local photobooth, make the official application free for commercial use, and grow gradually through usefulness, self-service documentation, truthful hardware compatibility, and community contributions.

Do not make persuading existing stores to migrate or beating competitors feature-for-feature a core success metric.

## Target audience

### Primary: new booth builders
People setting up their first booth or adding a new booth, especially DIY users who want to reuse a camera, PC, or printer they already own or source second-hand. They can choose software before committing to an incumbent workflow.

### Secondary: existing operators
Existing stores may try PhotoBooth on a new booth, spare/test rig, or voluntary later migration. Replacing a stable production system is not the primary go-to-market motion because switching can be risky and support-intensive.

## Product positioning

PhotoBooth is software-only. The project does not sell cameras, computers, printers, or booth kits. The official application is intended to be free for commercial photobooth operation and self-installation. Users configure supported application settings and add frame/theme content without editing core code.

Optional paid services may include setup, troubleshooting, training, camera/printer integration, migration, and authorized custom core work.

This is a product direction, not proof of market demand. Validate gradually through successful self-installations, tested hardware combinations, repeat usage, voluntary issue reports, and organic referrals. Formal interviews are optional, not a gate.

## Phase 0 - Self-service readiness

Planning window: about one week; no interview phase required.

Work:
1. Keep the one-sentence promise clear: free commercial use, reuse existing hardware, self-install, app-level configuration, and optional support.
2. Publish an honest project overview distinguishing implemented, planned, and experimental features.
3. Create a reproducible installation path, quickstart, troubleshooting guide, sample configuration, and sample frame.
4. Define a compatibility matrix with statuses: tested, community-reported, untested, and unsupported.
5. Create low-friction feedback channels: issue templates, compatibility report, feature request, and setup-troubleshooting templates.
6. Keep the interview guide as an optional tool if users volunteer feedback; do not require interviews or design-partner commitments.
7. Use competitor research as context and to avoid false claims, not as a feature-parity checklist.

Exit gate:
- Product and licensing positioning is documented.
- A reference camera/PC/output configuration is selected.
- A repeatable test and user-report path exists.
- Formal interviews and incumbent migrations are not required.

Do not treat stars, page views, or downloads alone as proof of adoption. Prefer successful first installs, completed capture-to-output workflows, repeat use, and voluntary hardware reports.

## Phase 1 - Technical feasibility

The first reference camera is the Logitech B525 HD Webcam available on the Windows 11 Pro 64-bit development machine.

### Observed results on 2026-10-25
- OpenCV Python package 5.0.0.93 was installed only in the local POC virtual environment.
- DirectShow at camera index 0 read 50 of 50 frames at 1280x720 and wrote a temporary JPEG.
- Media Foundation at camera index 0 read 50 of 50 frames at 1280x720 and wrote a temporary JPEG.
- Each backend was reopened three times and read 10 of 10 frames on each cycle.
- A JSON-configured three-photo strip was rendered to a 1200x1800 JPEG, which OpenCV decoded successfully; all temporary images were deleted.
- Ten renderer unit tests passed, including malformed configuration, bounds, fit modes, and three-slot rendering.
- A concurrent reader failed while a second process held the webcam; after release, a fresh capture succeeded. At this 2026-10-25 POC baseline, the low-level behavior was documented but production UI recovery had not yet been implemented; the later Wails recovery integration is recorded below.

This section records the first Python/OpenCV webcam POC, not the current full validation state. A later experimental Go Media Foundation provider was integrated with the Go schema-v1 renderer and local output: three consecutive compiled-executable B525 sessions passed, the 100-session B525 harness passed, and bounded provider hot-unplug/replug recovery passed in one Go process (reopen succeeded at attempt 22, followed by a successful capture). The earlier visual inspection still found soft/noisy quality and close/off-center framing under indoor lighting. Controlled booth lighting/mounting, Wails active-capture physical unplug/replug behavior, printer hardware, laptop integrated camera, and Linux hardware remain untested.

### Remaining camera tests
1. Test image quality under controlled booth lighting and intended camera mounting distance; the first visual check looked soft/noisy and too close/off-center.
2. The provider gives actionable diagnostics for the observed camera-busy/resource HRESULT. The Wails prototype now has real same-session Continue and fresh-session Restart flows; the recorded UI automation passed 2/2 trials for each after forcing initial camera contention. These automated contention tests are distinct from physically unplugging the webcam during an active Wails capture.
3. Provider-level B525 USB hot-unplug/replug recovery has passed in a headless Go process. Still test physical disconnect/reconnect during an active Wails capture and record the GUI recovery behavior.
4. Test the built-in camera on a real laptop separately; do not infer this from the B525 test.
5. Compare optimized renderer output under controlled booth lighting and intended mounting distance; test a physical printer after selecting its model and OS/driver target.
6. Test Linux on one named distribution and record the exact camera/backend combination.

### Cross-platform spike
- Investigate Windows Media Foundation and Linux Video4Linux2 for the generic UVC path.
- OpenCV is currently a disposable POC tool. A separate Wails v2.17 Windows shell spike now builds and measures WebView-to-Go calls, but the production UI framework remains undecided because Wails v2.17 requires Go 1.25 and adds a WebView runtime; Linux UI/runtime and hardware are not tested.
- Select one Windows baseline and one specific Linux distribution for an actual kiosk prototype.
- Test startup, full-screen mode, input, packaging/update, camera integration, output, and support burden.
- Treat each OS/camera/printer combination as a separate compatibility claim.

Exit gate: demonstrate capture, file transfer/save, render, and output on exact reference hardware. Record blockers and unsupported capabilities. A successful build or 50-frame smoke test is not proof of broad compatibility.

## Phase 2 - Lock MVP scope and contracts

The MVP should include:
- Kiosk launch and explicit session states.
- One tested UVC webcam provider.
- Configured capture count and countdown.
- Static, deterministic frame rendering.
- Preview and one reliable output path.
- Local settings/session storage and sample content.
- Recoverable camera, render, and output errors.
- Offline operation for unpaid/local sessions.
- App-level configuration for the timing and shot-count values deliberately exposed.
- Clear installation and troubleshooting documentation.

Defer:
- Public marketplace commerce and creator payouts.
- Cloud photo delivery as a requirement.
- Universal camera/printer or generic Linux support.
- Arbitrary executable theme plugins.
- Microservices and complex cloud infrastructure.
- Features without evidence of need, such as advanced analytics or GIF/video output.

Define the session state machine, camera capability/provider API, render/frame schema, output error model, versioned settings, media-delivery policy, payment policy, logging/privacy rules, and extension/license boundary before parallel AI implementation.

## Phase 3 - Local alpha

Implement one complete vertical slice:
1. Session orchestrator and tests.
2. One UVC webcam provider and hardware diagnostic view.
3. Original frame capture and local storage.
4. Static frame render pipeline.
5. Preview and print/export adapter.
6. Local configuration.
7. Error recovery, logs, retention/cleanup, and disk-full behavior. The local output provider classifies disk-full/quota errors, removes partial temporary output and preserves captures/rendered source. Save-only retry is available through the CLI and the Wails result-screen button, backed by one shared validated API and no-overwrite publication. Automated core/Wails tests and Windows builds pass; manual UI button smoke and physical full-disk hardware testing remain unrun.
8. Reproducible build/install instructions.

Exit criteria:
- Repeated end-to-end sessions work on the exact reference hardware.
- Unpaid/local capture and output work without Internet.
- Camera/render/output failures do not permanently freeze the app.
- Render output is deterministic.
- Tests cover session state, geometry, settings validation, and recoverable errors.
- A clean setup can install and run the alpha using documented steps.

## Phase 4 - Reliability pilot

Use a few volunteer/new-build users if available, but do not make interviews a prerequisite. Start with the exact tested configuration.

Test normal/repeated sessions, offline operation, camera disconnect, timeout, printer unavailable, low disk, app restart, corrupt frame, and output failure. Add LAN/payment/cloud tests only when those features exist.

A possible pilot target is 200 sessions and approximately 99% completed without an unrecoverable application failure. Adjust this proposed goal after a real baseline; it is not an existing performance claim.

Track setup time, successful first run, session completion, failure/recovery, support hours, and repeat usage. Do not upload guest photos as telemetry.

## Phase 5 - Optional delivery and payment

- Local-only: reliable local storage, retention/cleanup, and operator retrieval.
- LAN: short-lived authorization scoped to one output; never expose the entire photo directory.
- Cloud: only after clear quota, cost, access, consent, retention, and deletion policies.
- Payment-disabled/free mode must work without a payment provider.
- For paid flows, verify success through a trusted server/provider signal; QR display or a client-side claim is not payment proof.
- Handle duplicate/late/pending payment, timeout, refund, cancellation, and network loss.
- Media-delivery failure must not delete the only local output.

## Phase 6 - Curated content catalog and creator beta

Start with a small curated set and a versioned declarative package format. Validate file paths, size, hashes, compatibility, licenses, and safe activation/rollback. Publish sample frames and instructions before building a full visual editor or open publishing marketplace.

Success means a self-builder can discover, preview, download, install, and use a compatible frame offline without modifying core code.

## Phase 7 - Release engineering

- Reproducible release builds and installer/uninstaller.
- Kiosk startup and supported OS configuration.
- Staged update, integrity checks, rollback, and release notes.
- Backup/restore of settings.
- Logs/support bundle with privacy redaction.
- Quickstart, hardware matrix, troubleshooting, and known issues.
- Dependency/license audit and reviewed end-user terms.
- Linux stays experimental until actual camera/printer/kiosk combinations pass.

## Phase 8 - Gradual marketing and community

- Maintain a landing page that distinguishes available and planned features.
- Publish build/setup tutorials and real capture-to-output demos.
- Share useful solutions in relevant self-build, hardware, and photobooth communities without spam.
- Invite voluntary compatibility reports and contributions to public documentation/sample content where allowed.
- Use curated free frames to demonstrate the workflow.
- Let community adoption grow organically; do not make existing-store migration a growth goal.
- Track successful installs, repeat use, issue resolution through documentation, organic referrals, and active hardware configurations rather than vanity metrics.

## Commercial model

Basic commercial use and self-install are intended to be free. Support, setup, training, hardware integration, and authorized core modification can be paid. Do not deliberately cripple normal settings or frame/theme customization to sell support.

See LICENSE_STRATEGY.md and ADR 0004. Exact legal terms are pending review.

## Current execution status (initial baseline: 2026-10-25; reviewed 2026-11-01)

Implemented and verified in the repository:
- Go 1.24.8 modular core, explicit session state machine, and camera provider boundary.
- Schema-v1 declarative Go renderer with geometry/config validation, bounded inputs, JPEG/PNG decode and atomic JPEG output.
- Local finalized-output provider, independent from the camera implementation.
- Session runner with preflight, countdown, capture loop, close-camera-before-render, local output, explicit failure/cancel/timeout state, and per-phase capture/render/save timings.
- Output coordinator dispatches configured targets concurrently (bounded to four), records per-target status/duration/error and typed causes, and treats local storage and printer targets independently. Session state distinguishes completed, partial_success, and total output failure. Local storage uses a same-directory temporary file, sync/close and atomic publish; disk-full/quota failures are classified, partial temp files are cleaned up, and source captures/rendered output are preserved. A shared `output.RetryLocalFromSession` API powers both `apps/output-retry` and the Wails result-screen **Retry local save** button. It validates preserved artifacts and refuses overwrite, including concurrent retry publication. Core and Wails tests cover the retry paths; Windows UI Automation smoke of the retry button has now passed with a disposable synthetic session and controlled output-path obstruction; physical full-disk testing is still pending. Printer behavior is covered with a synthetic target only.
- Synthetic camera and CLI mode for deterministic offline tests.
- Experimental Windows Media Foundation camera provider behind camera.Provider. The locally patched windows-camera-go v0.2.0 adapter runs without CGO, selects a B525 MJPEG mode, captures stream frames on a thread-locked worker, and normalizes abbreviated MJPEG frames missing DHT tables.
- One hundred consecutive B525 sessions in a single Go process passed after wrapper resource-lifecycle fixes (300/300 captures, 100 rendered strips, 100 local copies); all 500 JPEG artifacts decoded at expected dimensions. Mean session wall time was 800.73 ms, median 800.30 ms, p95 828.80 ms. Across 813 Windows process samples, working set averaged 37.08 MB (max 44.33 MB) and private bytes averaged 33.04 MB (max 39.94 MB); first-to-last 10% memory sample means declined. A separate same-process contention/recovery test passed its expected failure and recovery phases. This is not a test of the future GUI kiosk. A separate physical B525 hot-unplug/replug during an active Wails UI session passed one cycle on 2026-10-31; repeated cycles/ports and representative visual quality under controlled lighting remain untested.
- Tests for renderer validation/geometry/resource bounds, adaptive-scaler pixel difference, centered cover crop, session flow, 25 repeated fake-camera sessions, capture/output/preflight errors, deadline cancellation, and MJPEG DHT normalization.
- Renderer benchmark on Windows 11 / Ryzen 5 8600G / Go 1.24.8: same textured fixture compared Catmull-Rom-only at 171.75 ms and 73.1 MB allocated/op versus adaptive scaling at 102.26 ms and 17.3 MB allocated/op. This is allocated bytes, not peak RAM. A post-change B525 session still produced three 1280x720 captures and a valid 1200x1800 output.
- The Windows-only HRESULT regression tests verify actionable MF_E_HW_MFT_FAILED_START_STREAMING diagnostics and preservation of the underlying HRESULT. The 100-session harness and 500-JPEG verifier completed with no failures or dimension mismatches. Details are in docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md.
- ADR 0007 records modular Go core/provider boundaries; ADR 0008 records frame schema v1; ADR 0009 records the experimental Windows camera adapter.
- Separate Wails v2.17 desktop shell experiment lives under tools/ui-spike/wails-v2 in a Go 1.25.0 module; the root core remains Go 1.24.8. It has the real camera/workflow integration, camera-failure Continue/Restart UI, operator settings/frame-package screens, a save-only local-output retry action and an idle-only live camera preview using the same Windows Media Foundation worker as capture. Three hidden-mode runs verified 100 bridge calls each (average p50 0.233 ms, p95 0.400 ms; frontend-ready after startup callback averaged 95.2 ms; sampled maximum working set 25.93 MB). Latest Wails tests/vet, Vite build and Windows build passed on 2026-10-31. This does not measure visible input latency or certify the UI as the production interface. The output-retry button passed a disposable UI Automation smoke; one active-session physical B525 hot-unplug/replug cycle through Wails also passed. Real disk-full, repeat physical recovery cycles/ports and broader camera-quality validation remain open. See UI_SPIKE_RESULTS.md and ADR 0012/0019.

Observed checks on the current source:
- On 2026-10-31: full root `go test -count=1 ./...`, `go vet ./...`, Windows `go build ./...`, Linux/amd64 CGO-disabled compile and the 15-test Python/OpenCV camera-POC suite passed. A later diagnostic warm-up option was added and the full root test/vet/Windows build/Linux compile matrix was run again; all passed.
- On 2026-10-31: isolated Wails `go test -count=1 ./...`, `go vet ./...`, Vite production build and Wails Windows production build passed; binding generation added `RetryPhotoOutput` and `OutputRetrying`.
- On 2026-11-01: a fresh Wails current-scene quality batch passed 3/3 real B525 sessions and 9/9 1280x720 captures/render/local outputs. Session first-shot drops were 0.017%, 0.110% and 0.231% (mean 0.119%); all sessions passed the unchanged gross-darkness/relative-consistency validator. Temporary photo data was removed; report: `%TEMP%\photobooth-wails-quality-92009cc35d5444d29f236fa624a50ed9.json`. This is not fixed-lighting portrait certification or visual inspection. ShinPc has only Microsoft Print to PDF and OneNote virtual queues; a physical printer model is not available for adapter/hardware validation.
- On 2026-11-01: Wails operator Settings UI smoke passed after its harness was changed to wait for the Settings view after the idle-preview stop barrier; 8 checks passed and Cancel left the existing operator config unchanged. A temporary .pbframe UI smoke passed 7 checks through validate/install/activate/rollback/uninstall. A new two-process settings persistence smoke saved countdown 4s / interval 750ms / 3 shots, exited the first process gracefully, restarted PhotoBooth.exe against the same disposable profile, and verified all values reloaded; no photo session was created and the profile was cleaned. Report: `%TEMP%\photobooth-settings-restart-6d4b3d0f640f448aa8ec9345a4194462.json`. These are experimental Wails prototype acceptance checks, not installer-upgrade/schema-migration or production kiosk certification.
- On 2026-10-31: Wails idle live-preview UI smoke passed on the physical B525: preview image rendered on idle, the same camera was closed before capture, the three-shot session completed all captures/render/local output, then preview restarted after returning to idle. Latest report: `%TEMP%\photobooth-wails-preview-713d7c864cf1438cb4ebd1a21397b379.json`. Preview is capped at 5 FPS with one ACK-gated frame in flight; stale/duplicate ACKs are rejected, decode failures are not ACKed, and cancellation during native startup now waits for cleanup before `Open` returns. This is one functional smoke, not a full-day preview soak. Wails UI active-session physical B525 USB unplug/replug also passed one cycle; Windows PnP disappearance/return and UI recovery were observed, then the same session auto-completed 3 captures/render/local output without Continue or app restart. The three-session Wails image-quality batch passed 9/9 captures at 1280x720; first-shot drop was 0.000%, 0.000% and 0.660%. A separate 3-pair warm-up A/B gave mean signed first-shot drop 0.047% without warm-up vs. 0.010% with the one-second default (0.037 percentage points); default remains unchanged. See CAMERA_RELIABILITY_RESULTS.md.

- On 2026-11-01: `ui-preview-kiosk-soak.ps1` passed for 10.26 minutes on the reference B525/Windows 11 kiosk UI. It collected 195 two-second samples, 2,825 effective ACKs, five Settings stop/resume cycles and five display-mode cycles. Mean/min FPS was 4.71/4.07; no ACK timeout or decode failure. CPU tree mean/peak 15.28/25% of one logical core. PhotoBooth.exe private bytes stayed near 29 MB; WebView2 private bytes peaked at 1,418 MB and ended at 1,315 MB. The WebView2 footprint was high but varied around 1.3–1.45 GB after startup rather than growing without bound during this run. The 3 GB safety guard was not reached; the temporary profile was cleaned. Up to three backend JSONL ACK audit gaps were observed, with later frame publication confirming the serial pump consumed the ACKs; the missing audit records remain an observability issue. Full-day/overnight and low-RAM target validation remain open. See `tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md` and `docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md`.

The CLI and Wails shell remain development/prototype paths, not a release-ready booth app. Real camera validation has been performed on the reference B525, but the current visual quality evidence is only a coarse darkness/first-shot consistency gate and is not a representative booth portrait. Real capture writes camera imagery locally; delete it after test verification.

Still not completed:
- Real disk-full/quota hardware test; the Wails Retry local save button passed a disposable Windows UI Automation smoke with a controlled output-path obstruction.
- A 10.26-minute B525/Windows 11 fullscreen kiosk + idle-preview soak passed, including five Settings stop/resume and five fullscreen/windowed cycles. Full-day/overnight preview soak, low-RAM target validation, repeat physical USB recovery across ports/cycles, and representative booth-lighting/mounting image-quality certification remain open. The measured WebView2 footprint is high (peak 1,418 MB private bytes), so memory optimization/target-hardware validation is still needed. Provider-level B525 hot-plug and Wails camera-busy Continue/Restart UI automation are separate tests and have passed as described above.
- Production guest/operator UI and secured kiosk deployment. The Wails candidate now displays idle live preview, latest captured still, and passed a 10.26-minute application-level kiosk/preview soak, but is not the production UI selection. Windows Assigned Access/Shell Launcher, login auto-start, OS shortcut policy, low-RAM target validation, and full-day soak remain open.
- Physical printer adapter and tests for the selected printer model/OS/driver; provider-aware retry and duplicate-print semantics.
- Built-in laptop camera and Linux physical camera/kiosk/output tests. A cross-platform compile check is not hardware validation.
- Optional payment, LAN/cloud media delivery, marketplace, sustained production reliability and user-adoption evidence.

## Weekly review

Record demonstrated outcomes, tests actually run, compatibility changes, issues, user-reported success/failure, decisions, and the next highest-value improvement. AI-generated code volume is not a success metric.
