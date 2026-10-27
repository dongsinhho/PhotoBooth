# Product Plan: From Idea to Commercial Launch

Status: Proposed execution plan
Last updated: 2026-10-25
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
- A concurrent reader failed while a second process held the webcam; after release, a fresh capture succeeded. The low-level behavior is documented, but production UI recovery is not implemented.

This section records the first Python/OpenCV webcam POC, not the current full validation state. A later experimental Go Media Foundation provider was integrated with the Go schema-v1 renderer and local output: three consecutive compiled-executable B525 sessions passed, and another end-to-end run passed after the adaptive renderer optimization. The earlier visual inspection still found soft/noisy quality and close/off-center framing under indoor lighting. Controlled booth lighting/mounting, printer hardware, hot-unplug recovery, sustained use, laptop integrated camera, and Linux hardware remain untested.

### Remaining camera tests
1. Test image quality under controlled booth lighting and intended camera mounting distance; the first visual check looked soft/noisy and too close/off-center.
2. The provider now gives actionable diagnostics for the observed camera-busy/resource HRESULT, and a fresh session succeeds after the competing app releases the camera. Production UI retry/recovery and same-process session continuation are not implemented.
3. Physically disconnect/reconnect the USB webcam and record recovery behavior; this requires hardware interaction and remains untested.
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
7. Error recovery, logs, retention/cleanup, and disk-full behavior.
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

## Current execution status (2026-10-25)

Implemented and verified in the repository:
- Go 1.24.8 modular core, explicit session state machine, and camera provider boundary.
- Schema-v1 declarative Go renderer with geometry/config validation, bounded inputs, JPEG/PNG decode and atomic JPEG output.
- Local finalized-output provider, independent from the camera implementation.
- Session runner with preflight, countdown, capture loop, close-camera-before-render, local output, explicit failure/cancel/timeout state, and per-phase capture/render/save timings.
- Output coordinator dispatches configured targets concurrently (bounded to four), records per-target status/duration/error, and treats local storage and printer targets independently. Session state distinguishes completed, partial_success, and total output failure. Local storage is implemented; printer behavior is covered with a synthetic target only.
- Synthetic camera and CLI mode for deterministic offline tests.
- Experimental Windows Media Foundation camera provider behind camera.Provider. The locally patched windows-camera-go v0.2.0 adapter runs without CGO, selects a B525 MJPEG mode, captures stream frames on a thread-locked worker, and normalizes abbreviated MJPEG frames missing DHT tables.
- One hundred consecutive B525 sessions in a single Go process passed after wrapper resource-lifecycle fixes (300/300 captures, 100 rendered strips, 100 local copies); all 500 JPEG artifacts decoded at expected dimensions. Mean session wall time was 800.73 ms, median 800.30 ms, p95 828.80 ms. Across 813 Windows process samples, working set averaged 37.08 MB (max 44.33 MB) and private bytes averaged 33.04 MB (max 39.94 MB); first-to-last 10% memory sample means declined. A separate same-process contention/recovery test passed its expected failure and recovery phases. This is not a test of the future GUI kiosk, physical USB hot-unplug or visual quality under controlled lighting.
- Tests for renderer validation/geometry/resource bounds, adaptive-scaler pixel difference, centered cover crop, session flow, 25 repeated fake-camera sessions, capture/output/preflight errors, deadline cancellation, and MJPEG DHT normalization.
- Renderer benchmark on Windows 11 / Ryzen 5 8600G / Go 1.24.8: same textured fixture compared Catmull-Rom-only at 171.75 ms and 73.1 MB allocated/op versus adaptive scaling at 102.26 ms and 17.3 MB allocated/op. This is allocated bytes, not peak RAM. A post-change B525 session still produced three 1280x720 captures and a valid 1200x1800 output.
- The Windows-only HRESULT regression tests verify actionable MF_E_HW_MFT_FAILED_START_STREAMING diagnostics and preservation of the underlying HRESULT. The 100-session harness and 500-JPEG verifier completed with no failures or dimension mismatches. Details are in docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md.
- ADR 0007 records modular Go core/provider boundaries; ADR 0008 records frame schema v1; ADR 0009 records the experimental Windows camera adapter.
- Separate Wails v2.17 desktop shell experiment lives under tools/ui-spike/wails-v2 in a Go 1.25.0 module; the root core remains Go 1.24.8. Three hidden-mode runs verified 100 bridge calls each (average p50 0.233 ms, p95 0.400 ms; frontend-ready after startup callback averaged 95.2 ms; sampled maximum working set 25.93 MB). This does not measure visible input latency or camera/render integration. See UI_SPIKE_RESULTS.md and ADR 0012.

Observed checks on the current source:
- Full root go test -count=1 ./..., go vet ./..., Windows go build ./..., and Linux/amd64 CGO-disabled cross-build passed after the Windows camera diagnostic changes.
- Root go vet ./..., Windows go build ./..., and Linux/amd64 CGO-disabled cross-build passed on this revision.
- The separate Python/OpenCV POC regression suite passed 10/10 tests. The reusable Windows B525 reliability harness passed 100/100 sessions and verified all 500 JPEGs; the isolated Wails module tests/vet, frontend build and Windows Wails build also pass.

The CLI is a development tool, not a release-ready booth app. For a real B525 test, run the command documented in the root README. Real capture writes camera imagery locally; delete it after test verification.

Still not completed:
- Full GUI/kiosk soak with camera preview and operator/guest UI, cancellation during a blocked native frame read, physical USB unplug/reconnect, UI-level retry/recovery, and controlled-lighting/mounting image-quality checks.
- Production guest/operator UI, real camera preview, full kiosk behavior, and a non-blocking UI event adapter.
- Physical printer adapter and tests for the selected printer model/OS/driver; provider-aware retry and duplicate-print semantics.
- Built-in laptop camera and Linux physical camera/kiosk/output tests. A cross-platform compile check is not hardware validation.
- Optional payment, LAN/cloud media delivery, marketplace, sustained production reliability and user-adoption evidence.

## Weekly review

Record demonstrated outcomes, tests actually run, compatibility changes, issues, user-reported success/failure, decisions, and the next highest-value improvement. AI-generated code volume is not a success metric.
