# PhotoBooth Knowledge Base

This folder is shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary
- Proposed: recommended direction, not yet confirmed.
- Accepted: product requirement/decision is agreed; implementation may not exist.
- Implemented: exists in code and has been verified.
- Deprecated: no longer current; retained only when useful for history.

Never turn a proposal into an asserted fact.

## Read in this order
1. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) - end-to-end plan from discovery through engineering, pilot, delivery, and marketing.
2. [PRD.md](PRD.md) - draft requirements, user jobs, MVP candidates, and adoption metrics.
3. [COMPETITOR_RESEARCH.md](COMPETITOR_RESEARCH.md) - public-source research for context, with caveats.
4. [DISCOVERY_LOG.md](DISCOVERY_LOG.md) - self-service adoption hypotheses and evidence tracker.
5. [DISCOVERY_INTERVIEW_GUIDE.md](DISCOVERY_INTERVIEW_GUIDE.md) - optional feedback guide; interviews are not a gate.
6. [LICENSE_STRATEGY.md](LICENSE_STRATEGY.md) - free commercial use, app-level customization, core modification rights, and optional support.
7. [PRODUCT_BRIEF.md](PRODUCT_BRIEF.md) - target users, value proposition, and accepted requirements.
8. [MVP_SCOPE.md](MVP_SCOPE.md) - first milestone, acceptance criteria, and exclusions.
9. [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) - player, cloud, and adapter boundaries.
10. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) - domain concepts and relationships.
11. [CAMERA_INTEGRATION.md](CAMERA_INTEGRATION.md) - provider capabilities and platform boundaries.
12. [OUTPUT_TARGETS.md](OUTPUT_TARGETS.md) - independent local/print output targets and lifecycle status.
13. [CAMERA_COMPATIBILITY_MATRIX.md](CAMERA_COMPATIBILITY_MATRIX.md) - observed/tested camera, OS, and backend combinations.
14. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment.
15. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - theme/frame package contract proposal.
16. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) - package safety, payment verification, and guest-photo privacy.
17. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) - development and test practices.
18. [ROADMAP.md](ROADMAP.md) - staged rollout and validation gates.
19. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) - offline-first capture.
20. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) - store-configurable media/payment.
21. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) - cross-platform and backend direction.
22. [../adr/0004-free-commercial-use-and-paid-support.md](../adr/0004-free-commercial-use-and-paid-support.md) - free commercial use and controlled core.
23. [../adr/0005-greenfield-self-service-adoption.md](../adr/0005-greenfield-self-service-adoption.md) - greenfield audience and gradual-growth strategy.
24. [../adr/0006-webcam-first-technical-spike.md](../adr/0006-webcam-first-technical-spike.md) - webcam-first feasibility decision.
25. [../adr/0007-modular-go-core-and-provider-boundaries.md](../adr/0007-modular-go-core-and-provider-boundaries.md) - modular Go core, session lifecycle, and provider boundary decisions.
26. [../adr/0008-versioned-frame-json-and-local-render-pipeline.md](../adr/0008-versioned-frame-json-and-local-render-pipeline.md) - current frame schema v1 and local render/output path.
27. [../adr/0009-windows-media-foundation-camera-adapter.md](../adr/0009-windows-media-foundation-camera-adapter.md) - experimental native Windows camera adapter, upstream patch, and validation limits.
28. [../adr/0010-independent-output-targets.md](../adr/0010-independent-output-targets.md) - separate local-storage and printer targets, statuses, and session outcomes.
29. [../../tools/camera-poc/README.md](../../tools/camera-poc/README.md) - how to rerun the disposable webcam POC.

## Current status as of 2026-10-25

- Logitech B525 was detected by Windows 11 Pro 64-bit.
- A disposable OpenCV POC read 50/50 frames at 1280x720 through DirectShow and Media Foundation, wrote temporary JPEGs, and rendered a 1200x1800 sample layout JPEG. All temporary image files were deleted.
- Go frame renderer/session/local output have passed synthetic tests; an experimental Windows Media Foundation provider passed three consecutive compiled-executable B525 sessions on Windows 11. The independent output coordinator and local-storage target pass tests for concurrent dispatch, retained per-target outcomes, and partial success when a sibling target fails. A physical printer adapter is not implemented. Controlled image quality, hot-unplug recovery, soak use, built-in laptop camera, and Linux hardware remain untested.
- The offline Go slice passes on Go 1.24.8, and three consecutive compiled-executable sessions pass with the real B525. Next evidence: controlled image quality and recovery/soak tests, then a UI spike and a real-printer adapter after choosing a printer model.
- Competitor research is based on public vendor claims, not independent product testing.
- No market-adoption or successful self-install results are claimed.

## Maintenance rules
- Update the nearest topic document when requirements or implementation change.
- Use ADRs for decisions that affect multiple modules or are expensive to reverse.
- Add source links for vendor SDK support, licensing, security-sensitive behavior, and other changeable facts.
- Keep schemas/API contracts synchronized with implementation and tests.
- Never put secrets, personal data, real guest photos, or credentials here.

## Open questions
- Which camera models, firmware, and connection methods should be added after UVC?
- Which printer/output path should be the first render-to-output target?
- Which UI framework and Linux distribution should be supported first?
- Which payment provider and workflow will be used, if any?
- What LAN access, retention, and deletion policy is required?
- What exact end-user license and frame/theme creator terms will be adopted?
