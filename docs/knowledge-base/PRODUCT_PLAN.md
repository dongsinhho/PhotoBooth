# Product Plan: From Idea to Commercial Launch

**Status:** Proposed execution plan
**Last updated:** 2026-10-25
**Planning horizon:** Approximately 12–16 weeks to a private pilot is an initial estimate, not a delivery commitment. Hardware/SDK issues may change it.

## Executive strategy

Build a dependable local photobooth, make the official application free for commercial use, and grow adoption gradually through product usefulness, self-service documentation, transparent hardware compatibility, and community contributions.

Do not make persuading existing stores to migrate or beating competitors feature-for-feature a core success metric.

## Target audience

### Primary: greenfield booth builders
People setting up their first booth or adding a new booth, especially DIY users who want to reuse a camera, PC, or printer they already own or source second-hand. They can choose software before committing to an incumbent workflow.

### Secondary: existing operators
Existing stores may try PhotoBooth on a new booth, spare/test rig, or later voluntary migration. Replacing a stable production setup is not the primary go-to-market motion because the switching risk and support burden can be high.

## Positioning hypothesis

PhotoBooth is software-only. The project does not sell cameras, computers, printers, or booth kits. The official application is intended to be free for commercial photobooth operation and self-installation. Users configure supported application settings and add frame/theme content without editing core code. Optional paid services include installation, troubleshooting, training, camera/printer integration, migration, and authorized custom core work.

This is a product direction, not proof of market demand. Validate gradually through successful self-installations, tested hardware combinations, repeat usage, voluntary issue reports, and organic referrals. Formal interviews are optional, not a gate.

## Phase 0 — Product readiness and lightweight validation
**Planning window:** About one week; no interview phase required.

### Work
1. Keep the one-sentence promise clear: free for commercial use, reuse existing hardware, self-install, configure at the app layer, optional support.
2. Publish an honest project overview that distinguishes implemented, planned, and experimental features.
3. Create a reproducible installation path, quickstart, troubleshooting guide, sample configuration, and sample frame.
4. Define a compatibility matrix with statuses: tested, community-reported, untested, and unsupported.
5. Create low-friction feedback channels: issue templates, compatibility-report template, feature-request template, and setup-troubleshooting template.
6. Keep the interview guide as an optional tool if users volunteer feedback; do not require 10–15 interviews or design-partner commitments to proceed.
7. Compare competitors as context and to avoid false claims, not to build a feature-parity checklist or a campaign to displace incumbent users.

### Deliverables
- PRD.
- Competitor snapshot with sources and caveats.
- Self-service installation and feedback plan.
- Compatibility-report template.
- Hypothesis register.
- Clear separation between current implementation and roadmap.

### Gate to proceed
- Product and licensing positioning is documented.
- A camera/PC/output configuration is selected for a technical proof of concept.
- A repeatable test and user-report path exists.
- Formal interviews and incumbent migrations are not required.

Do not treat stars, likes, page views, or downloads alone as proof of adoption. Prefer successful first installs, completed capture-to-output workflows, repeat use, and voluntary hardware reports.

## Phase 1 — Technical feasibility

Select an exact camera model and a printer or file-export path based on equipment likely to be available to self-builders.

### Camera spike
- Connect/disconnect and recover.
- Trigger still capture and retrieve the original image.
- Test live view only if needed for the first experience.
- Test repeated captures, timeouts, camera busy/sleep, and reconnection.
- Record model, firmware, OS, SDK/provider version, drivers, capabilities, and licensing constraints.

### Cross-platform spike
- Select one Windows baseline and one specific Linux distribution for evaluation.
- Compare candidate UI frameworks through a real kiosk prototype: input, full-screen/auto-start, packaging/update, camera integration, and output.
- Treat every OS/camera/printer combination as a separate compatibility claim.
- Compare total cost including setup, remote maintenance, support, and downtime. Linux does not automatically mean lower total cost.

### Gate
Demonstrate a real end-to-end path: capture -> transfer -> render -> output. Record blockers and unsupported capabilities. Do not claim broad compatibility based on a single successful build.

## Phase 2 — Lock MVP scope and contracts

### MVP must include
- Kiosk launch and explicit session states.
- One tested camera provider/model.
- Configured capture count and countdown.
- Static, deterministic frame rendering.
- Preview and one reliable output path.
- Local settings/session storage and sample content.
- Recoverable camera, render, and output errors.
- Offline operation for unpaid/local sessions.
- App-level settings for the timing and shot-count values deliberately exposed.
- Clear installation and troubleshooting documentation.

### Defer
- Public marketplace payments and creator payouts.
- Cloud photo delivery as a requirement.
- Universal camera/printer or generic Linux support.
- Arbitrary executable theme plugins.
- Microservices and complex cloud infrastructure.
- Features without evidence of need, such as advanced analytics or GIF/video output.

Define session state machine, camera capability/provider API, render/frame schema, output error model, versioned settings, media-delivery policy, payment policy, logging/privacy rules, and the extension/license boundary before parallel AI implementation.

## Phase 3 — Local alpha

Implement one complete vertical slice rather than many disconnected modules:
1. Session orchestrator and tests.
2. One camera provider and hardware diagnostic view.
3. Original-image transfer and local storage.
4. Static frame-render pipeline.
5. Preview and print/export adapter.
6. Local configuration.
7. Error recovery, logs, retention/cleanup, and disk-full handling.
8. Reproducible build/install instructions.

### Exit criteria
- Repeated end-to-end sessions work on exact reference hardware.
- Unpaid/local capture and output work without Internet.
- Camera/render/output failures do not permanently freeze the app.
- Render output is deterministic.
- Tests cover session state, geometry, settings validation, and recoverable errors.
- A clean setup can install and run the alpha using documented steps.

## Phase 4 — Reliability pilot

Use a few volunteer/new-build users if available, but do not make interviews a prerequisite. Start with the exact tested configuration.

Test normal/repeated sessions, offline operation, camera disconnect, timeout, printer unavailable, low disk, app restart, corrupt frame, and output failure. Add LAN/payment/cloud tests only when those features exist.

Possible pilot target: 200 sessions and approximately 99% completed without an unrecoverable application failure. This is a proposed goal to adjust after a real baseline, not an existing performance claim.

Track setup time, successful first run, session completion, failure/recovery, support hours, and repeat usage. Do not upload guest photos as telemetry.

## Phase 5 — Optional delivery and payment

- Local-only: reliable local storage, retention/cleanup, and operator retrieval.
- LAN: short-lived authorization scoped to one output; never expose the entire photo directory.
- Cloud: only after clear quota, cost, access, consent, retention, and deletion policies.
- Payment-disabled/free mode must work without a payment provider.
- For paid flows, verify success through a trusted server/provider signal; QR display or a client-side claim is not proof of payment.
- Handle duplicate/late/pending payment, timeout, refund, cancellation, and network loss.
- Media-delivery failure must not delete the only local output.

## Phase 6 — Curated content catalog and creator beta

Start with a small curated set and a versioned declarative package format. Validate file paths, size, hashes, compatibility, licenses, and safe activation/rollback. Publish sample frames and instructions before building a full visual editor or open publishing marketplace.

Success means a self-builder can discover, preview, download, install, and use a compatible frame offline without modifying core code.

## Phase 7 — Release engineering

- Reproducible release builds and installer/uninstaller.
- Kiosk startup and supported OS configuration.
- Staged update, integrity checks, rollback, and release notes.
- Backup/restore of settings.
- Logs/support bundle with privacy redaction.
- Quickstart, hardware matrix, troubleshooting, and known issues.
- Dependency/license audit and reviewed end-user terms.
- Linux stays experimental until actual camera/printer/kiosk combinations pass.

## Phase 8 — Gradual marketing and community

- Maintain a landing page distinguishing available and planned features.
- Publish build/setup tutorials and real capture-to-output demos.
- Share useful solutions in relevant self-build, hardware, and photobooth communities without spam.
- Invite voluntary compatibility reports and contributions to public documentation/sample content where allowed.
- Use curated free frames to demonstrate the workflow.
- Let community adoption grow organically; there is no goal of forcing existing stores to switch.
- Track successful installs, repeat use, issue resolution through documentation, organic referrals, and active hardware configurations rather than vanity metrics.

## Commercial model

Basic commercial use and self-install are intended to be free. Support, setup, training, hardware integration, and authorized core modification can be paid. Do not deliberately cripple normal settings or frame/theme customization to sell support. See LICENSE_STRATEGY.md and ADR 0004. Exact legal terms are pending review.

## Current execution status

Prepared: PRD, competitor snapshot, optional feedback guide, and hypothesis register. Not yet done: selected-hardware compatibility test, successful self-install data, and actual user-adoption evidence. Formal interviews are optional. Document preparation does not mean validation is complete.

## Weekly review

Record demonstrated outcomes, tests actually run, compatibility changes, new issues, user-reported success/failure, decisions, and the next highest-value improvement. AI-generated code volume is not a success metric.
