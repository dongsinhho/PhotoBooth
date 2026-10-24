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

This is a successful initial webcam capture-to-file-render proof of concept, not production readiness. Image quality was not visually inspected. The production frame schema/renderer, printer path, device-loss recovery, sustained operation, integrated laptop camera, and Linux remain untested.

### Next camera tests
1. Visually inspect a locally captured test image for orientation, focus/exposure, color, and stale frames.
2. Design production session/UI behavior for camera-busy errors; the low-level failure and recovery-after-release have been observed in the POC, but app-level recovery is not implemented.
3. Disconnect/reconnect and record recovery behavior.
4. Test the built-in camera on a real laptop separately; do not infer this from the B525 test.
5. Validate the actual product frame schema/renderer and printer or chosen output path.
6. Test Linux on one named distribution and record the exact camera/backend combination.

### Cross-platform spike
- Investigate Windows Media Foundation and Linux Video4Linux2 for the generic UVC path.
- OpenCV is currently a disposable POC tool; the production backend and UI framework remain undecided.
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

## Current execution status

Completed:
- Product/discovery documents and self-service growth strategy.
- Logitech B525 device detection.
- 50 of 50 frames at 1280x720 through DirectShow and Media Foundation.
- Three open/read/save/release cycles per backend using 10 frames per cycle.
- Disposable sample-layout render to a 1200x1800 JPEG and successful read-back.

Not yet completed:
- Visual image-quality inspection.
- Camera-busy and disconnect/reconnect recovery.
- Production frame schema/renderer and printer output.
- Integrated laptop camera test.
- Linux camera/kiosk/output test.
- Sustained production reliability and user adoption evidence.

## Weekly review

Record demonstrated outcomes, tests actually run, compatibility changes, issues, user-reported success/failure, decisions, and the next highest-value improvement. AI-generated code volume is not a success metric.
