# Product Plan: From Idea to Commercial Launch

**Status:** Proposed execution plan  
**Last updated:** 2026-10-24  
**Planning horizon:** Approximately 12–16 weeks to a small private pilot, followed by staged public launch. This is a planning estimate, not a delivery commitment. Hardware availability, SDK compatibility, and pilot feedback may change it.

## Executive strategy

Build a dependable local photobooth first, validate it on real store hardware, then add LAN/cloud delivery, configurable payment, online content distribution, and a creator marketplace in stages.

Do not build every feature before the first pilot. Keep interfaces and configuration models extensible, but implement only what a pilot needs and can validate.

## Product positioning hypothesis

Existing software already offers camera control, frame templates, cloud sharing, and QR-based photo delivery. PhotoBooth should not compete on a simple feature checklist.

Validate this combined differentiation hypothesis:
- Operators can choose local-only, LAN, or optional platform-cloud photo delivery.
- Payment is optional and configured independently from camera operations.
- The product targets Windows and Linux with explicit tested hardware/OS compatibility.
- Creators can publish content through a versioned package format and, later, earn from a community marketplace.

These points are hypotheses until validated in discovery interviews. Recheck competitor product capabilities, pricing, and licensing before public launch.

## Target customer for the first pilot

Start with independent photobooth operators or a small operator with one to five booths, provided interviews confirm this segment can give quick product feedback and access to the target hardware.

Do not target every use case at once. Distinguish permanent self-service retail booths, event rental booths, and studio/event workflows; their printer, staffing, payment, and network requirements may differ substantially.

## Phase 0 — Product discovery and validation
**Planning window:** Week 1–2

### Work
1. Interview 10–15 booth owners/operators; observe at least a few real setup and photo sessions if possible.
2. Record the actual camera models, firmware, connection mode, PC specifications, OS, printers, paper sizes, kiosk input, and Internet reliability.
3. Ask how operators currently obtain, design, install, change, and pay for frames/themes.
4. Measure the pain of photo delivery: local retrieval, LAN transfer, cloud sharing, storage cost, and retention.
5. Map payment behavior: free sessions, QR payment during the guest flow, prepayment, manual confirmation, refunds, late/duplicate payment.
6. Compare at least three existing products using a consistent checklist: capture support, printer support, offline behavior, templates/store, QR delivery, payment, Linux support, deployment, pricing, and support.
7. Interview potential frame creators/designers about package production, intellectual property, pricing, and willingness to publish.

### Deliverables
- Product Requirements Document (PRD) with user journeys and non-goals.
- Competitor matrix based on current, source-linked evidence.
- Target operator profile and initial supported-hardware candidate.
- List of the top three pain points ranked by frequency and willingness to pay.
- Initial business hypotheses: pricing model, support model, and cloud-cost assumptions.
- Decision log for accepted requirements and unresolved questions.

### Gate to proceed
- At least three operators agree to be design partners or pilot candidates.
- One camera/printer/PC configuration can be secured for repeatable testing.
- The interviews identify a pain point stronger than “we want another frame app.”
- A plausible first value proposition and pilot scope can be explained in one paragraph.

If these conditions fail, refine the target segment or product positioning before committing to a large build.

## Phase 1 — Technical feasibility spikes
**Planning window:** Week 2–3; can overlap late discovery

This phase prevents an expensive architecture built around unsupported hardware assumptions.

### Camera spike
Using the exact camera candidate chosen with operators:
- Connect/disconnect and recover.
- Trigger still capture and retrieve the original file.
- Test live view only if needed for the first user experience.
- Test repeated captures, timeouts, device-busy states, and camera sleep/reconnection.
- Record model, firmware, OS, SDK/provider version, driver, and unsupported capabilities.
- Check SDK licensing, redistribution terms, and update/support implications.

Compare the official vendor SDK with an open-source provider only for the exact required operations. A model being listed as supported does not prove remote shutter, live view, or setting control all work.

### Cross-platform/UI spike
- Select one Windows baseline and one specific Linux distribution/desktop/session for evaluation.
- Compare candidate desktop UI approaches, initially Avalonia/.NET and Qt, using kiosk startup, touch input, full-screen behavior, rendering performance, packaging, accessibility, and maintenance.
- Build a small proof of concept that can open on both targets. Test the actual camera provider and one output path before declaring Linux viable.
- Compare total cost per booth: hardware, OS/software licenses, setup, device compatibility, remote support, update, and downtime. Do not assume Linux automatically reduces total cost.

### Output/render spike
- Render one sample frame with fixed canvas/slots.
- Check aspect ratio, crop, resolution, image orientation, color, fonts, safe margins, and actual print output.
- Test the intended printer and determine whether driver/CUPS differences affect support.

### Deliverables and gate
- Hardware/OS compatibility matrix.
- Short written decision on the first UI framework and target OS.
- Demonstration: capture one image, retrieve it, render it, and produce one physical or file output.
- Explicit list of blockers and unsupported features.

Do not finalize the UI framework or camera abstraction solely from a paper comparison.

## Phase 2 — Lock MVP scope and contracts
**Planning window:** Week 3

### MVP must include
- Kiosk launch and a small number of session states.
- One supported camera configuration.
- Capture a configured number of photos and handle timeout/disconnection clearly.
- Deterministic static frame rendering.
- Preview and one reliable output path (print or export; support both only if evidence/scope permits).
- Local persistence and installed sample content.
- Recoverable failure states, structured diagnostics, and offline operation.
- Settings and module contracts that keep camera, payment, and media delivery independent.

### MVP must not require
- Marketplace purchases or creator payouts.
- Cloud photo storage.
- Online authentication during each session.
- Support for every camera, printer, or Linux distribution.
- Arbitrary code supplied by a theme creator.
- Microservices, brokers, Kubernetes, or a complex analytics stack without a proven need.

### Contracts to define before parallel AI implementation
- Session state machine and cancellation/deadline rules.
- Camera capability/provider interface.
- Render/frame schema and deterministic output dimensions.
- Print/export result model and recovery behavior.
- Local configuration/versioning model.
- Media delivery policy interface.
- Payment policy/provider interface.
- Logging/privacy rules and error-code conventions.

### Deliverables
- Updated product and technical PRDs.
- Architecture diagrams and accepted ADRs.
- Repository skeleton/module boundaries and test strategy.
- Acceptance criteria for the first end-to-end session.

## Phase 3 — Build the local alpha
**Planning window:** Week 4–7

Build a complete vertical slice instead of many disconnected modules.

### Implementation order
1. Session orchestration and state-machine tests.
2. The first camera provider and hardware diagnostic screen.
3. Original-image transfer and local storage.
4. Static frame render pipeline with sample frames.
5. Preview and print/export adapter.
6. Local operator configuration.
7. Error handling, recovery, logs, cleanup, and disk-full behavior.
8. Package/build scripts and a reproducible development setup.

Keep the player independent of the cloud API. Go is the preferred backend language, but backend infrastructure can wait until local capture has been proven; implement only small contracts/mocks needed for the player.

### Alpha exit criteria
- Repeated end-to-end sessions work on the chosen reference hardware.
- Offline tests prove capture, render, and configured output do not need the network.
- Camera, render, and output errors are understandable and do not permanently freeze the application.
- The same inputs/package version produce the expected deterministic output.
- Unit/integration tests cover session state, rendering geometry, configuration validation, and recoverable failures.
- A clean machine can install and run the alpha using documented steps.

## Phase 4 — Private pilot and reliability hardening
**Planning window:** Week 8–9

Deploy to two or three design-partner booths if possible, beginning with the exact environment that passed alpha testing.

### Pilot instrumentation
Collect only operational information needed to improve reliability:
- Session starts, completions, cancellations, and technical errors.
- Camera provider/model/OS version and anonymized diagnostics.
- Render/output success and duration.
- App restart/recovery and disk usage.
- Operator reports on setup, ease of changing frames, and support burden.

Do not collect or upload guest photos as telemetry. Make any diagnostic upload configurable and redact personal information.

### Test matrix
- Normal sessions and peak repeated use.
- Internet unplugged during capture.
- Camera disconnect, sleep, timeout, and reconnect.
- Printer unavailable, paper/driver failure, and export failure.
- Low disk space and application restart.
- Invalid or missing frame assets.
- Update interruption and rollback, once updating exists.
- LAN/cloud/payment failure only when those features have been included in the pilot.

### Suggested initial quality gates (adjust with pilot baseline)
- At least 200 end-to-end sessions across the pilot, if usage allows.
- At least 99% complete without an unrecoverable application failure.
- Every failed session has an understandable error/recovery path; report hardware failures separately from application defects.
- No session requires the Internet unless the configured payment policy explicitly requires authoritative online verification.
- Operators can activate an installed sample frame without developer intervention.

These are proposed targets for the pilot, not proven current performance.

### Exit gate
Resolve all critical defects; record non-critical known issues and supported limitations. Do not expand hardware/OS claims beyond what was tested.

## Phase 5 — Add optional delivery and payment
**Planning window:** Week 9–11, after basic capture is stable

Implement and test these features as independently selectable modules.

### Media delivery rollout
1. **Local-only first:** reliable local save, quotas, operator access, and retention/cleanup.
2. **LAN download next:** deliver only the requested session output using a short-lived, high-entropy token or equivalent authorization. Never expose the entire image directory to guest devices. Test guest-to-guest isolation.
3. **Platform cloud after cost/privacy design:** authenticated upload/download, quotas, expiry, deletion, retry, cost measurement, and consent/user-facing terms.

Keep capture/output success separate from delivery status. A failed upload must not delete the local output.

### Payment rollout
1. Keep free/disabled mode fully functional without any payment provider.
2. Select one specific provider/flow for the pilot, if operators require it.
3. Use authoritative server/provider confirmation before issuing a paid session permit.
4. Handle duplicate, late, pending, timeout, refund, cancel, and network-loss cases.
5. Test webhook authenticity, idempotency, reconciliation, and the case where payment succeeds after the guest leaves.

Showing a static QR or receiving a client-side success message is not sufficient proof of payment. Evaluate current provider registration, fees, callback/webhook behavior, and go-live conditions before choosing.

### Gate
- Store configuration changes media/payment policies without rewriting camera code.
- Local-only and free modes operate without cloud/payment configuration.
- LAN access cannot retrieve another guest's output.
- Paid sessions cannot be granted on unverified payment.
- Optional media delivery failure cannot lose the only copy of the output.

## Phase 6 — Content catalog and creator beta
**Planning window:** Week 10–13

Start with a curated catalogue, not a full marketplace.

### First content beta
- Finalize and version the declarative theme/frame package schema.
- Build validation for file paths, hashes, package sizes, supported dimensions, license metadata, compatibility, and safe activation/rollback.
- Include a small professionally curated pack. Suggested goal: 20–30 usable frames with consistent preview quality and a range of common concepts.
- Add a read-only catalog, search/categories, preview, download, and local install.
- Seed content with a small group of designers and obtain written permissions/licenses.
- Track downloads and activation counts without collecting guest photos.

### Defer until usage is demonstrated
- Creator payouts and a complex marketplace split.
- Full visual editor.
- Unrestricted uploads and arbitrary plugins.
- Sophisticated ranking/recommendation algorithms.
- Paid subscriptions as a prerequisite to use the player.

### Gate
An operator can discover a compatible frame, inspect its preview/license, download it, install it, activate it safely, and continue using it offline.

## Phase 7 — Release engineering and public beta
**Planning window:** Week 12–15, dependent on prior gates

### Delivery checklist
- Reproducible builds and versioned release artifacts.
- Signed updates where feasible; integrity checks and rollback.
- Installer/uninstaller, auto-start/kiosk configuration, and clear initial setup.
- Configuration export/import and documented backup/restore.
- Structured logs with privacy redaction and support bundle generation.
- Known-issues page and explicit compatibility matrix.
- User guide for camera setup, printer setup, content install, network modes, and troubleshooting.
- Crash/error reporting policy and opt-in diagnostics.
- Support process, security contact, version support policy, and release notes.
- Validate all vendor SDK licenses, third-party licenses, and redistribution requirements.

### Platform policy
Treat Windows and Linux as separate release targets with separate compatibility evidence. If Linux has not passed real camera/printer/kiosk tests, label it experimental rather than claiming production support.

### Exit gate
A new store can install, configure, and run without a developer at the booth; rollback/support procedures have been rehearsed.

## Phase 8 — Marketing and commercial validation
**Starts during discovery; intensifies before public beta**

### Before build completion
- Create a landing page describing the actual problem and invite operators to join the pilot.
- Publish a short demo/prototype and collect feedback; mark non-existent features as planned.
- Recruit three to five design partners.
- Start conversations in relevant Vietnamese photobooth-owner communities, supplier networks, and designer groups. Follow group rules and avoid spam.
- Use interviews and landing-page responses to test positioning, not vanity likes.

### Before public beta
- Prepare short videos demonstrating the complete flow: select frame, take photos, output, receive photos.
- Publish troubleshooting/setup guides that lower migration risk.
- Recruit five or more frame designers for the first content cohort.
- Prepare approximately 20–30 high-quality launch frames, including free samples. This is a suggested launch target, not a requirement.
- Give pilot partners a clear offer: early access, direct support, migration assistance, and a channel to request frames/features.
- Make clear what works offline, what requires Internet, and what costs extra.

### Initial channels
- Facebook/Zalo communities and direct outreach to booth owners.
- Short-form demo video (TikTok, Reels, Shorts).
- Collaborations with frame designers and booth/hardware vendors.
- A simple content page optimized for searches related to photobooth software, frame templates, Linux photobooth, and photo delivery.
- A creator waitlist and free content as an acquisition loop.

### Business model experiments
Test instead of committing early:
- Free player + paid frame/bundle.
- Paid/pro player features for store operations.
- Subscription for multi-booth/cloud/management features.
- Optional paid cloud storage/delivery with transparent quota and costs.

Do not rely on cloud upload as the default business model before estimating storage, transfer, and support cost per booth/session.

### Metrics
Product/reliability:
- Completed sessions divided by started sessions, by hardware/OS.
- Unrecoverable failure rate and average time to recover.
- Time to set up a new booth.
- Time to install/activate content.
- Output/delivery failure and retry rates.
- Cloud cost per booth/month and per delivered photo where applicable.

Adoption:
- Design partners onboarded and still active after pilot.
- Weekly active booths and sessions per booth.
- Percentage of operators who install or switch frames.
- Creator uploads approved and time to publish.

Commercial:
- Paid frame conversion, average order value, repeat purchases.
- Pro conversion and churn if subscription is introduced.
- Creator earnings and contribution margin.
- Support minutes per booth/month.

Set numeric business targets after baselining the pilot, not before.

## Privacy, licensing, and legal readiness
Photobooth images may identify people. Photo delivery, retention, access, deletion, and consent must be designed deliberately. The Vietnamese Personal Data Protection Law (Law 91/2025/QH15) and Decree 356/2025/ND-CP took effect on 2026-01-01 according to [the Government legal document portal](https://vanban.chinhphu.vn/?classid=1&docid=214590&orggroupid=1&pageid=27160) and [the Ministry of Justice legal database entry for Decree 356/2025/ND-CP](https://vbpl.moj.gov.vn/bocongan/Pages/vbpq-thuoctinh.aspx?ItemID=187276). Review current requirements with qualified counsel before commercial release. Do not treat this plan as legal advice.

Every marketplace package needs explicit license/attribution and a takedown process. Review fonts, stickers, characters, logos, music, and creator-submitted assets; do not assume fan art or a public download is commercially redistributable.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Product looks too similar to existing software | Validate a specific underserved operator workflow and test positioning in discovery |
| Camera SDK or model doesn't support required operations | Hardware spike before architecture lock; ship a transparent compatibility matrix |
| Linux saves OS cost but increases support burden | Compare total cost and validate one exact distro + camera + printer combination |
| Payment status can be spoofed or duplicated | Server/provider verification, idempotency, reconciliation, negative tests |
| LAN photo access leaks another guest's images | Session-scoped authorization, short expiry, isolation tests, no directory browsing |
| Cloud costs exceed value | Optional cloud, quotas, retention, per-booth cost monitoring |
| Marketplace launches without good content | Curated launch pack and creator cohort before open publishing |
| Legal/IP issue with guest photos or design assets | Privacy/retention review, contributor license, moderation and takedown process |
| AI produces architecture/code drift | Repository instructions, tests, ADRs, and one reviewed vertical slice at a time |

## First 10 working days — actionable checklist

### Days 1–2
- Prepare a short operator interview script.
- List local booth businesses and relevant communities to contact.
- Ask target operators for camera, PC, OS, printer and paper-size details.

### Days 3–5
- Conduct the first five interviews.
- Build a competitor matrix from direct product/docs/pricing checks.
- Write the problem statement and initial user journey from observed facts.

### Days 6–7
- Select the most promising reference hardware from interview evidence.
- Borrow/buy/access the exact camera and printer.
- Create a compatibility test checklist.

### Days 8–10
- Run the first camera capture/transfer spike on Windows.
- Run a bounded Linux feasibility test on a named distribution if it is part of the target cost case.
- Make a one-page decision: target OS for first pilot, UI candidate, camera provider candidate, and the biggest blockers.
- Update this plan and related ADRs based on evidence.

## Weekly governance

At the end of each week, record:
- What was completed and demonstrated.
- Tests run and actual results.
- What changed in the product assumptions.
- New risks/blockers and decisions needed.
- Next week's one or two highest-value outcomes.

AI can accelerate implementation, research, and test scaffolding, but acceptance gates must be based on observed runs, source evidence, and feedback from real operators—not generated code volume.
