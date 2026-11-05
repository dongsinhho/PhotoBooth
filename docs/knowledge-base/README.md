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
12. [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md) - reproducible 100-session B525 capture, process-memory sampling, contention/recovery evidence, and the latest measured image-quality gate.
13. [OUTPUT_TARGETS.md](OUTPUT_TARGETS.md) - independent local/print output targets and lifecycle status.
14. [RENDERING_PERFORMANCE.md](RENDERING_PERFORMANCE.md) - repeatable renderer/output benchmarks and lifecycle test evidence.
15. [UI_SPIKE_RESULTS.md](../../tools/ui-spike/wails-v2/UI_SPIKE_RESULTS.md) - isolated Wails shell bridge performance, resource samples, Go toolchain constraint, and unvalidated areas.
16. [CAMERA_COMPATIBILITY_MATRIX.md](CAMERA_COMPATIBILITY_MATRIX.md) - observed/tested camera, OS, and backend combinations.
17. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment.
18. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - broader theme/catalog package proposal; not all proposed capabilities are implemented.
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
34. [../adr/0013-bounded-camera-stream-recovery.md](../adr/0013-bounded-camera-stream-recovery.md) - camera stream invalidation and bounded provider close/reopen recovery.
35. [../adr/0014-warm-up-webcam-stream.md](../adr/0014-warm-up-webcam-stream.md) - one-second, bounded stream warm-up before the first guest photo.
36. [../../tools/camera-poc/controlled-image-quality.ps1](../../tools/camera-poc/controlled-image-quality.ps1) - repeated B525 captures, measurable darkness/first-shot checks, and JSON report.
37. [../../tools/camera-poc/README.md](../../tools/camera-poc/README.md) - how to rerun the disposable webcam POC.
38. [../../tools/camera-poc/persistent-soak/README.md](../../tools/camera-poc/persistent-soak/README.md) - same-process Go soak, heap/GC measurements, OS-memory sampling and contention/recovery.
39. [../../tools/camera-poc/hotplug-reopen-test/README.md](../../tools/camera-poc/hotplug-reopen-test/README.md) - physical Windows B525 USB unplug/reconnect test with the recovery wrapper.
40. [../../tools/camera-poc/ui-recovery-smoke.ps1](../../tools/camera-poc/ui-recovery-smoke.ps1) - repeated Wails UI contention recovery and fresh-session restart test.
41. [STORE_CONFIG.md](STORE_CONFIG.md) - schema-v1 local station configuration, constraints, CLI precedence and supported modes.
42. [../adr/0015-versioned-local-station-config.md](../adr/0015-versioned-local-station-config.md) - decision for versioned local station config.
43. [../adr/0016-wails-local-settings-screen.md](../adr/0016-wails-local-settings-screen.md) - connect supported local station settings to the Wails prototype.
44. [../../tools/ui-spike/wails-v2/ui-settings-smoke.ps1](../../tools/ui-spike/wails-v2/ui-settings-smoke.ps1) - read-only UI Automation smoke for settings controls and cancel flow.
45. [FRAME_PACKAGE_FORMAT.md](FRAME_PACKAGE_FORMAT.md) - implemented .pbframe schema v1, CLI, validation limits, local installation, activation and rollback.
46. [../adr/0017-local-frame-packages-and-rollback.md](../adr/0017-local-frame-packages-and-rollback.md) - decision and recovery contract for offline frame package lifecycle.
47. [../../tools/ui-spike/wails-v2/ui-frame-package-smoke.ps1](../../tools/ui-spike/wails-v2/ui-frame-package-smoke.ps1) - isolated UI Automation smoke for validate/install/activate/rollback/uninstall.
48. [LOCAL_DATA_RETENTION.md](LOCAL_DATA_RETENTION.md) - opt-in local session/output retention, strict candidate allowlist, two-step plan confirmation and safety limits.
49. [../adr/0018-opt-in-local-data-retention.md](../adr/0018-opt-in-local-data-retention.md) - decision for disabled-by-default local retention and safe CLI apply.
50. [../QUICKSTART_WINDOWS.md](../QUICKSTART_WINDOWS.md) - Windows-first self-build guide based on the recorded B525 reference setup; not a published installer.
51. [../troubleshooting/WINDOWS_TROUBLESHOOTING.md](../troubleshooting/WINDOWS_TROUBLESHOOTING.md) - symptom-based recovery and privacy-safe support diagnostics.
52. [../COMPATIBILITY_REPORT_TEMPLATE.md](../COMPATIBILITY_REPORT_TEMPLATE.md) - exact hardware/OS/provider/result template without photo uploads.
53. [../../tools/self-service/README.md](../../tools/self-service/README.md) and [../../tools/self-service/inspect-environment.ps1](../../tools/self-service/inspect-environment.ps1) - read-only local setup checker.
54. [../../.github/ISSUE_TEMPLATE/](../../.github/ISSUE_TEMPLATE/) - compatibility, bug and feature-request issue forms.
55. [LOCAL_STORAGE_RECOVERY.md](LOCAL_STORAGE_RECOVERY.md) - disk-full/quota failure behavior, artifact preservation and save-only retry command.
56. [../../apps/output-retry/](../../apps/output-retry/) - CLI to save a preserved rendered session without reopening the camera.
57. [../adr/0019-local-output-disk-full-recovery.md](../adr/0019-local-output-disk-full-recovery.md) - local disk-full classification, atomic output publish and save-only recovery decision.
58. [CAMERA_RELIABILITY_RESULTS.md](CAMERA_RELIABILITY_RESULTS.md) - B525 headless/UI hot-plug recovery, idle live-preview handoff, real UI quality batch and warm-up A/B metrics.
59. [../adr/0013-bounded-camera-stream-recovery.md](../adr/0013-bounded-camera-stream-recovery.md) and [../adr/0014-warm-up-webcam-stream.md](../adr/0014-warm-up-webcam-stream.md) - bounded recovery and one-second camera warm-up decisions.
60. [../../tools/ui-spike/wails-v2/ui-camera-preview-smoke.ps1](../../tools/ui-spike/wails-v2/ui-camera-preview-smoke.ps1) - real B525 idle-preview, capture handoff and preview-resume UI smoke.
61. [../adr/0020-idle-only-camera-preview.md](../adr/0020-idle-only-camera-preview.md) - shared-worker, idle-only preview and bounded backpressure decision.
62. [../adr/0023-preview-renderer-and-soak-diagnostics.md](../adr/0023-preview-renderer-and-soak-diagnostics.md) - canvas/ImageBitmap lifecycle, persistent shared-read diagnostics and exact kiosk soak audit.
63. [../../tools/ui-spike/wails-v2/ui-preview-kiosk-soak.ps1](../../tools/ui-spike/wails-v2/ui-preview-kiosk-soak.ps1) - reproducible 10-minute B525 idle-preview/kiosk resource and lifecycle soak with exact JSONL event reconciliation.

## Current status as of 2026-11-05

- Logitech B525 was detected by Windows 11 Pro 64-bit.
- A disposable OpenCV POC read 50/50 frames at 1280x720 through DirectShow and Media Foundation, wrote temporary JPEGs, and rendered a 1200x1800 sample layout JPEG. All temporary image files were deleted.
- Go frame renderer/session/local output have passed tests; after native resource-lifecycle cleanup the experimental Windows Media Foundation provider passed 100 sequential sessions in one Go process (300 captures, 100 strips, 100 local copies). All 500 JPEGs decoded at expected dimensions. Wall time mean 800.73 ms, median 800.30 ms, p95 828.80 ms. Across 813 OS samples, working set averaged 37.08 MB (max 44.33 MB), private bytes averaged 33.04 MB (max 39.94 MB); first-to-last 10% means declined. Same-process contention failed with actionable HRESULT 0xC00D3704 and recovered after release. This is headless runner evidence, not GUI kiosk certification. The expanded same-process recovery window also passed a physical B525 unplug/replug test: provider reopen succeeded on attempt 22 and the same in-flight capture resumed in 23.906 seconds. The built Wails UI has passed real camera capture-render-save and a two-trial recovery/fresh-session suite in one app process; all four completed sessions produced output and test photos were deleted. The first night-time image-quality batch was near-black (mean luminance 2.52-2.54/255). A 2026-10-29 morning retest passed the gross-darkness/first-shot-consistency gate on 9/9 captures, with first-shot drops <=0.122%; visual review still found a dim room without representative booth lighting. One idle-only Wails B525 live-preview/capture/preview-resume UI smoke passed on 2026-10-31 using the same Media Foundation worker, max 5 FPS and one ACK-gated frame. Preview and capture are serialized; the preview stops before capture and resumes on idle. Full-day preview soak, representative booth-lighting quality, repeated Wails USB unplug/replug cycles/ports, actual native-driver blocked-read interruption, printer output, laptop camera and Linux hardware remain untested. One physical USB unplug/replug during an active Wails capture passed on the B525 on 2026-10-31; the same three-shot session auto-recovered and completed output. A current-scene Wails quality batch passed 9/9 captures, and a three-pair warm-up A/B showed only a 0.037 percentage-point first-shot drop improvement with the one-second default. The Windows worker-level blocked-read cancellation/Close-retry regression tests pass. On 2026-10-29, a persistent single-process B525 endurance run passed 1,000/1,000 sessions and verified 5,000/5,000 JPEGs; details are in CAMERA_RELIABILITY_RESULTS.md. The root CLI supports a strict schema-v1 local station config through --config, with CLI override precedence and a sample at examples/store-config-v1.json. The Wails prototype now has a local Settings & frame screen backed by the same config package, plus an offline .pbframe catalog with validation, immutable installation, activation, rollback and guarded uninstall. UI Automation has passed settings-only and full package lifecycle smokes in isolated data roots without opening the camera. See STORE_CONFIG.md, FRAME_PACKAGE_FORMAT.md, LOCAL_DATA_RETENTION.md, ADR 0016, ADR 0017 and ADR 0018.
- Root Go core validation on Go 1.24.8 passed on 2026-10-29 after local retention work: go test -count=1 ./..., go vet ./..., Windows build, Linux/amd64 cross-build (compile-only), and the 15-test Python POC regression suite. The retention core/CLI/config tests cover reviewed-plan digests, dry-run safety, stale candidate rejection, incomplete-session/temporary-output skips and symlinked-root checks; the CLI is not run by player startup. The reusable Windows B525 harness additionally passed 100/100 real-camera sessions, all 500 JPEGs verified, with per-process memory sampling; details are in CAMERA_RELIABILITY_RESULTS.md. The isolated Wails UI spike remains a separate Go 1.25.0 module and does not change the root toolchain.
- Competitor research is based on public vendor claims, not independent product testing.
- The Windows Wails candidate now has a canvas/ImageBitmap idle preview renderer and exact backend ACK/cancellation diagnostics. A repeat B525 fullscreen-kiosk soak on 2026-11-01 passed 10.24 minutes / 195 samples, five Settings cycles and five display cycles, with 2,828/2,828 published-frame ACKs and zero audit gaps/timeouts/decode failures. WebView2 private bytes peaked at 1,254.63 MB (lower than the first independent 10-minute run, but still high for low-RAM PCs). This remains a single-host 10-minute result, not full-day or production certification; see CAMERA_RELIABILITY_RESULTS.md, UI_SPIKE_RESULTS.md and ADR 0023.
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


## Windows Preview release tooling
[tools/release/windows/README.md](../../tools/release/windows/README.md) documents the internal unsigned ZIP package, local immutable-version install, active pointer, launch verification and rollback tooling. This is not a public release.

### Additional update (2026-11-05)

- Internal Windows Preview release tooling now builds a three-file ZIP with a versioned manifest and SHA-256 metadata, installs verified immutable versions side-by-side, switches the active pointer atomically, and supports local rollback. Synthetic lifecycle smoke and a disposable install of the current real Windows binary passed. This is not a signed installer, automatic updater or public distribution; legal terms and production UI selection remain open.
- The 2026-11-05 real B525 Wails quality batch completed capture/render/local-save for 9/9 shots but failed the existing gross-darkness gate: session mean luminance 2.504–2.530/255. First-shot drops remained below 0.3%. The near-black captures and report were retained for visual review; representative fixed-lighting portrait/framing remains open.
