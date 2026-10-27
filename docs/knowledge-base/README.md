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
12. [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md) - reproducible 100-session B525 capture, process-memory sampling, contention and recovery evidence.
13. [OUTPUT_TARGETS.md](OUTPUT_TARGETS.md) - independent local/print output targets and lifecycle status.
14. [RENDERING_PERFORMANCE.md](RENDERING_PERFORMANCE.md) - repeatable renderer/output benchmarks and lifecycle test evidence.
15. [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md) - isolated Wails shell bridge performance, resource samples, Go toolchain constraint, and unvalidated areas.
16. [CAMERA_COMPATIBILITY_MATRIX.md](CAMERA_COMPATIBILITY_MATRIX.md) - observed/tested camera, OS, and backend combinations.
17. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment.
18. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - theme/frame package contract proposal.
19. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) - package safety, payment verification, and guest-photo privacy.
20. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) - development and test practices.
21. [ROADMAP.md](ROADMAP.md) - staged rollout and validation gates.
22. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) - offline-first capture.
23. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) - store-configurable media/payment.
24. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) - cross-platform and backend direction.
25. [../adr/0004-free-commercial-use-and-paid-support.md](../adr/0004-free-commercial-use-and-paid-support.md) - free commercial use and controlled core.
26. [../adr/0005-greenfield-self-service-adoption.md](../adr/0005-greenfield-self-service-adoption.md) - greenfield audience and gradual-growth strategy.
27. [../adr/0006-webcam-first-technical-spike.md](../adr/0006-webcam-first-technical-spike.md) - webcam-first feasibility decision.
28. [../adr/0007-modular-go-core-and-provider-boundaries.md](../adr/0007-modular-go-core-and-provider-boundaries.md) - modular Go core, session lifecycle, and provider boundary decisions.
29. [../adr/0008-versioned-frame-json-and-local-render-pipeline.md](../adr/0008-versioned-frame-json-and-local-render-pipeline.md) - current frame schema v1 and local render/output path.
30. [../adr/0009-windows-media-foundation-camera-adapter.md](../adr/0009-windows-media-foundation-camera-adapter.md) - experimental native Windows camera adapter, upstream patch, and validation limits.
31. [../adr/0010-independent-output-targets.md](../adr/0010-independent-output-targets.md) - separate local-storage and printer targets, statuses, and session outcomes.
32. [../adr/0011-adaptive-image-scaling-for-renderer.md](../adr/0011-adaptive-image-scaling-for-renderer.md) - adaptive downscaling, pixel-quality regression, and benchmark evidence.
33. [../adr/0012-isolate-wails-ui-spike-toolchain.md](../adr/0012-isolate-wails-ui-spike-toolchain.md) - keep Wails evaluation separate from the Go 1.24.8 core.
34. [../../tools/camera-poc/README.md](../../tools/camera-poc/README.md) - how to rerun the disposable webcam POC.
35. [../../tools/camera-poc/persistent-soak/README.md](../../tools/camera-poc/persistent-soak/README.md) - same-process Go soak, heap/GC measurements, OS-memory sampling and contention/recovery.

## Current status as of 2026-10-27

- Logitech B525 was detected by Windows 11 Pro 64-bit.
- A disposable OpenCV POC read 50/50 frames at 1280x720 through DirectShow and Media Foundation, wrote temporary JPEGs, and rendered a 1200x1800 sample layout JPEG. All temporary image files were deleted.
- Go frame renderer/session/local output have passed tests; after native resource-lifecycle cleanup the experimental Windows Media Foundation provider passed 100 sequential sessions in one Go process (300 captures, 100 strips, 100 local copies). All 500 JPEGs decoded at expected dimensions. Wall time mean 800.73 ms, median 800.30 ms, p95 828.80 ms. Across 813 OS samples, working set averaged 37.08 MB (max 44.33 MB), private bytes averaged 33.04 MB (max 39.94 MB); first-to-last 10% means declined. Same-process contention failed with actionable HRESULT 0xC00D3704 and recovered after release. This is headless runner evidence, not GUI kiosk certification. Physical USB hot-unplug, controlled image quality, printer output, laptop camera and Linux hardware remain untested.
- Root Go core validation on Go 1.24.8 passed after the reliability diagnostic change: go test -count=1 ./..., go vet ./..., Windows build, Linux/amd64 cross-build (compile-only), and the 10-test Python POC regression suite. The reusable Windows B525 harness additionally passed 100/100 real-camera sessions, all 500 JPEGs verified, with per-process memory sampling; details are in CAMERA_RELIABILITY_RESULTS.md. The isolated Wails UI spike remains a separate Go 1.25.0 module and does not change the root toolchain.
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
