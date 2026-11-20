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
64. [../QUICKSTART_LINUX_EVALUATION.md](../QUICKSTART_LINUX_EVALUATION.md) - verified WSL2 Ubuntu 26.04.1 Wails build/basic UI smoke plus the blocked V4L2/usbipd device preflight steps.
65. [SESSION_CRASH_RECOVERY.md](SESSION_CRASH_RECOVERY.md) - schema-v2 local session manifests with immutable frame snapshots (legacy v1 remains readable), read-only startup discovery, explicit operator recovery actions and conservative inspection-only cases.
66. [../adr/0028-local-session-crash-recovery.md](../adr/0028-local-session-crash-recovery.md) - decision and safety boundaries for local crash/restart recovery.
67. [../../tools/ui-spike/wails-v2/ui-recovery-smoke.ps1](../../tools/ui-spike/wails-v2/ui-recovery-smoke.ps1) - synthetic-only Windows UI Automation for recovery actions; disables preview and rejects all real-camera capture entry points.
68. [../../tools/ui-spike/wails-v2/cmd/recovery-fixture/](../../tools/ui-spike/wails-v2/cmd/recovery-fixture/) - deterministic temporary session/manifest/JPEG fixtures for the UI smoke.
69. [../adr/0029-media-delivery-and-payment-policy-boundaries.md](../adr/0029-media-delivery-and-payment-policy-boundaries.md) - local delivery factory and disabled-payment gate boundaries; capture remains provider-independent, and LAN/QR modes fail closed until implemented.
70. [NETWORK_EXPOSURE_AUDIT.md](NETWORK_EXPOSURE_AUDIT.md) - read-only Windows host audit for local storage roots, NTFS ACLs, SMB shares, listeners and firewall profile; records the current Stage 2 partial result.
71. [NETWORK_EXPOSURE_AUDIT_RUNBOOK.md](NETWORK_EXPOSURE_AUDIT_RUNBOOK.md) - operator steps for elevated audit and scoped second-device SMB reachability/access checks without opening photo contents.
72. [../adr/0030-linux-v4l2-backend-evaluation.md](../adr/0030-linux-v4l2-backend-evaluation.md) - V4L2 is the selected Linux UVC API; provider implementation waits for a real V4L2 device node in WSL.
73. [../../tools/camera-poc/v4l2-preflight/README.md](../../tools/camera-poc/v4l2-preflight/README.md) - read-only Linux V4L2 device/module/permission preflight; no camera node is opened.
74. [../adr/0031-opt-in-windows-smb-administration.md](../adr/0031-opt-in-windows-smb-administration.md) - accepted policy for SMB Server/admin shares off by default with UAC-controlled opt-in maintenance access.
75. [../../tools/self-service/manage-smb-remote-admin.ps1](../../tools/self-service/manage-smb-remote-admin.ps1) - standalone PowerShell Status/Enable/Disable helper prototype for restricted Windows SMB administration.

## Current status as of 2026-11-08

- Logitech B525 was detected by Windows 11 Pro 64-bit.
- A disposable OpenCV POC read 50/50 frames at 1280x720 through DirectShow and Media Foundation, wrote temporary JPEGs, and rendered a 1200x1800 sample layout JPEG. All temporary image files were deleted.
- Go frame renderer/session/local output have passed tests; after native resource-lifecycle cleanup the experimental Windows Media Foundation provider passed 100 sequential sessions in one Go process (300 captures, 100 strips, 100 local copies). All 500 JPEGs decoded at expected dimensions. Wall time mean 800.73 ms, median 800.30 ms, p95 828.80 ms. Across 813 OS samples, working set averaged 37.08 MB (max 44.33 MB), private bytes averaged 33.04 MB (max 39.94 MB); first-to-last 10% means declined. Same-process contention failed with actionable HRESULT 0xC00D3704 and recovered after release. This is headless runner evidence, not GUI kiosk certification. The expanded same-process recovery window also passed a physical B525 unplug/replug test: provider reopen succeeded on attempt 22 and the same in-flight capture resumed in 23.906 seconds. The built Wails UI has passed real camera capture-render-save and a two-trial recovery/fresh-session suite in one app process; all four completed sessions produced output and test photos were deleted. The first night-time image-quality batch was near-black (mean luminance 2.52-2.54/255). A 2026-10-29 morning retest passed the gross-darkness/first-shot-consistency gate on 9/9 captures, with first-shot drops <=0.122%; visual review still found a dim room without representative booth lighting. One idle-only Wails B525 live-preview/capture/preview-resume UI smoke passed on 2026-10-31 using the same Media Foundation worker, max 5 FPS and one ACK-gated frame. Preview and capture are serialized; the preview stops before capture and resumes on idle. Full-day preview soak, optional portrait-specific face/skin-tone/framing review (separate from the passed current image-quality gate), repeated Wails USB unplug/replug cycles/ports, actual native-driver blocked-read interruption, printer output, laptop camera and Linux hardware remain untested. One physical USB unplug/replug during an active Wails capture passed on the B525 on 2026-10-31; the same three-shot session auto-recovered and completed output. A current-scene Wails quality batch on 2026-11-07 passed the defined image-quality gate (3 sessions / 9 captures, mean luminance 129.207/255, first-shot drop 0.150-0.421%); no camera tuning was indicated. A six-pair warm-up A/B found no meaningful brightness benefit from 1s (median paired effect +0.028 pp against warm-up), so the one-second default is retained as a low-cost stream-settling precaution and is not increased. Portrait-specific face/skin-tone/framing review is a separate optional refinement. Detailed paired metrics are recorded in ADR 0014 and CAMERA_RELIABILITY_RESULTS.md. The Windows worker-level blocked-read cancellation/Close-retry regression tests pass. On 2026-10-29, a persistent single-process B525 endurance run passed 1,000/1,000 sessions and verified 5,000/5,000 JPEGs; details are in CAMERA_RELIABILITY_RESULTS.md. The root CLI supports a strict schema-v1 local station config through --config, with CLI override precedence and a sample at examples/store-config-v1.json. The Wails prototype now has a local Settings & frame screen backed by the same config package, plus an offline .pbframe catalog with validation, immutable installation, activation, rollback and guarded uninstall. UI Automation has passed settings-only and full package lifecycle smokes in isolated data roots without opening the camera. See STORE_CONFIG.md, FRAME_PACKAGE_FORMAT.md, LOCAL_DATA_RETENTION.md, ADR 0016, ADR 0017 and ADR 0018.
- Root Go core validation on Go 1.24.8 passed on 2026-10-29 after local retention work: go test -count=1 ./..., go vet ./..., Windows build, Linux/amd64 cross-build (compile-only), and the 15-test Python POC regression suite. The retention core/CLI/config tests cover reviewed-plan digests, dry-run safety, stale candidate rejection, incomplete-session/temporary-output skips and symlinked-root checks; the CLI is not run by player startup. The reusable Windows B525 harness additionally passed 100/100 real-camera sessions, all 500 JPEGs verified, with per-process memory sampling; details are in CAMERA_RELIABILITY_RESULTS.md. The isolated Wails UI spike remains a separate Go 1.25.0 module and does not change the root toolchain.
- Competitor research is based on public vendor claims, not independent product testing.
- The Windows Wails candidate now has a canvas/ImageBitmap idle preview renderer and exact backend ACK/cancellation diagnostics. A repeat B525 fullscreen-kiosk soak on 2026-11-01 passed 10.24 minutes / 195 samples, five Settings cycles and five display cycles, with 2,828/2,828 published-frame ACKs and zero audit gaps/timeouts/decode failures. WebView2 private bytes peaked at 1,254.63 MB (lower than the first independent 10-minute run, but still high for low-RAM PCs). This remains a single-host 10-minute result, not full-day or production certification; see CAMERA_RELIABILITY_RESULTS.md, UI_SPIKE_RESULTS.md and ADR 0023.
- The Windows Preview uses schema-v2 session manifests for new sessions (schema-v1 remains readable) and read-only crash/restart discovery with explicit Continue / fresh-session restart / local-output retry / inspect actions. New sessions preserve the exact declarative frame JSON as a session-local SHA-256-verified `frame.json`, so Continue remains possible if the operator later edits or removes the current frame file. Synthetic/temp-directory tests cover snapshot immutability/integrity, legacy manifest compatibility, JPEG validation, capture-configuration drift, retention safety, camera cleanup on manifest-write failure and ambiguous output states. A new UI test confirms a snapshot-backed demo session can continue even after the selected frame file is removed, while other station settings errors remain blocking. Physical camera tests were not run for this snapshot change. Linux runtime, USB, soak, portrait-specific refinement and printer gates remain pending. See SESSION_CRASH_RECOVERY.md, ADR 0028 and UI_SPIKE_RESULTS.md.
- No market-adoption or successful self-install results are claimed.
- Linux WSL2 evaluation now passes native root Go tests/vet/build, isolated Wails build and basic WSLg home/Settings/Cancel UI smoke. The V4L2 read-only preflight runs on Ubuntu 26.04.1 but finds no attached camera node: WSL lacks USB/V4L2 sysfs and `/dev/videoN`; `uvcvideo.ko` is available but not loaded, and usbipd-win is not installed. No Linux webcam provider/capture is claimed. See ADR 0030 and the V4L2 preflight README.

## Maintenance rules
- Update the nearest topic document when requirements or implementation change.
- Use ADRs for decisions that affect multiple modules or are expensive to reverse.
- Add source links for vendor SDK support, licensing, security-sensitive behavior, and other changeable facts.
- Keep schemas/API contracts synchronized with implementation and tests.
- Never put secrets, personal data, real guest photos, or credentials here.

## Open questions
- Which camera models, firmware, and connection methods should be added after UVC?
- Which printer/output path should be the first render-to-output target?
- Which production UI framework should be selected after the Windows/Linux evaluation?
- Which Linux camera provider and printer integration should be implemented and validated after the UI shell evaluation?
- Which payment provider and workflow will be used, if any?
- What LAN access, retention, and deletion policy is required?
- What exact end-user license and frame/theme creator terms will be adopted?


## Windows Preview release tooling
[tools/release/windows/README.md](../../tools/release/windows/README.md) documents the internal unsigned ZIP package, local immutable-version install, active pointer, launch verification and rollback tooling. This is not a public release.

### Additional update (2026-11-05)

- Internal Windows Preview release tooling now builds a three-file ZIP with a versioned manifest and SHA-256 metadata, installs verified immutable versions side-by-side, switches the active pointer atomically, and supports local rollback. Synthetic lifecycle smoke and a disposable install of the current real Windows binary passed. This is not a signed installer, automatic updater or public distribution; legal terms and production UI selection remain open.
- The 2026-11-05 real B525 Wails quality batch completed capture/render/local-save for 9/9 shots but failed the existing gross-darkness gate: session mean luminance 2.504–2.530/255. First-shot drops remained below 0.3%. The near-black captures and report were retained for visual review; representative fixed-lighting portrait/framing remains open.


### Additional update (2026-11-05)

- The Windows Preview package workflow has now passed an installed-binary end-to-end smoke: build ZIP → install first version → update to a second version label → rollback → verify active executable/hash → launch installed executable → fullscreen/Esc/F11/Esc checks. No photo capture was started; package/install/player test data was cleaned after graceful exit. This is internal unsigned deployment evidence only.
- Linux evaluation target selected: Ubuntu 26.04.1 LTS x86_64 with Wails v2.17.0 / GTK3 + WebKit2GTK 4.1 (webkit2_41). The target is not yet built or runtime-tested because no accessible Linux environment was available from ShinPc.
- The installed-package smoke now also verifies real Settings persistence across two launches of the installed/rolled-back executable (3 shots / 4s countdown / 750ms interval); both kiosk and settings smokes passed without creating a photo session. See tools/release/windows/README.md and HANDOFF_2026-11-02.md for the exact reports and scope.
- Host preflight for the Ubuntu Linux evaluation is now available at tools/release/windows/linux-evaluation-host-preflight.ps1. It is read-only; current ShinPc result: WSL absent, Docker Linux engine unavailable, so Linux runtime gate is blocked pending elevated WSL setup/reboot or an existing Ubuntu desktop. See HANDOFF_2026-11-02.md.

- Stage 2 policy-boundary follow-up (2026-11-08): full CLI local/free acceptance now verifies three synthetic captures, session-scoped local output and manifest policy metadata; unsupported LAN/cloud/QR profiles fail at entrypoint before session-root creation; payment-gate requests are checked for stable session ID/Resume behavior; Wails saves preserve non-UI policy fields. This adds acceptance evidence but does not close Stage 2 or enable network/payment modes. See STORE_CONFIG.md, ADR 0029 and the latest HANDOFF_2026-11-08.md.


## Continuation status (2026-11-16)

- The WSL2 Ubuntu 26.04.1 local-storage mount/runtime sub-gate has been revalidated against the current source: tmpfs/ext4 paths pass; 9p and symlink redirects to 9p fail closed; direct session-child identity passes while redirected-child identity fails. Root tests, vet and Linux/amd64 compile pass. See [the Linux evaluation runbook](../QUICKSTART_LINUX_EVALUATION.md) and [the roadmap](ROADMAP.md).
- Stage 2 is still OPEN: elevated Windows firewall/SMB evidence and a separate trusted LAN host are missing. Stage 0/1 still need a printer target and acceptance, hardware matrix decision, production UI decision, and signed deployment/update policy. The accepted reference XHC1 recovery campaign is closed for its defined scope; the requested 8 GiB and longer soak tests remain skipped.
- Current checkpoint: [HANDOFF_2026-11-17.md](../../HANDOFF_2026-11-17.md).


## Latest continuation status (2026-11-17)

- ShinPc's SMB remote-administration helper completed one real Enable → authenticated allowlisted SMB share-list → Disable cycle. After Disable, LanmanServer is Stopped/Disabled, AutoShareWks=0, the PhotoBooth Allow rule is absent, three managed inbound Block rules remain effective, and Wi-Fi is restored to Public. The trusted Unix test client then observed TCP 445 fail and smbclient time out.
- This is scoped evidence for the single tested ShinPc/Wi-Fi/Unix path, not disposable-VM, non-allowlisted-source, rollback, reboot-persistence, IPv6 or universal network-boundary certification. Stage 2 remains OPEN pending those gates and provisioning/operator UI integration.
- Latest engineering checkpoint: [HANDOFF_2026-11-17.md](../../HANDOFF_2026-11-17.md).


## SMB operator UI and provisioning integration (2026-11-17)

- The internal Windows Preview Settings & frame screen now contains **Remote administration**, backed by live Windows helper status and active physical adapter/IP discovery. Enable/Disable use the UAC-aware helper; SMB machine state is not persisted in StoreConfig.
- Fresh Windows Preview package installs run helper Disable through UAC before activation. Upgrades preserve an existing operator choice. The release ZIP ships the helper beside PhotoBooth.exe and validates its manifest SHA-256.
- The integrated flow passed Wails Go tests/vet, Windows production build, 10-check Settings UI smoke, synthetic release lifecycle tests and isolated real-binary package/helper-hash install smoke. The exact scope and limitations are in [HANDOFF_2026-11-17.md](../../HANDOFF_2026-11-17.md), [ADR 0031](../adr/0031-opt-in-windows-smb-administration.md), and [Windows release tooling](../../tools/release/windows/README.md).
- Current host remains default-off: LanmanServer Stopped/Disabled, AutoShareWks=0, no PhotoBooth Allow rule, three managed Block rules active, Wi-Fi Public.
- Unlisted-source, reboot-persistence, IPv6 network, disposable-VM and signed-production-installer validation are not claimed. Stage 3 LAN delivery/payment remains deferred.


## Addendum - 2026-11-18

- [PRINTER_SELECTION_AND_ACCEPTANCE.md](PRINTER_SELECTION_AND_ACCEPTANCE.md) records the current ShinPc printer inventory, a research-based HiTi P525L / DNP DS620A shortlist, the still-open hardware-selection gate and the proposed first physical print acceptance sequence. No physical printer support is claimed.


## Addendum - 2026-11-19

- [DS-RX1 virtual smoke README](../../tools/printer-poc/dsrx1-virtual-smoke/README.md) documents the Windows GDI printer target spike, repeatable synthetic smoke command, local+printer coordinator checks and strict virtual-only scope. Current target implementation: internal/booth/output/printer*.go.


- [ADR 0032: Windows GDI printer output target](../adr/0032-windows-gdi-printer-output-target.md) records the internal Windows printer adapter spike, spooler job-ID semantics, virtual-queue evidence and why Wails/session integration is deferred until job-state persistence and duplicate handling are designed.


- [ADR 0033: Durable printer intent and conservative recovery](../adr/0033-durable-printer-intent-and-recovery.md) documents manifest-v3 print attempts, unique Windows document names, crash locks, uncertain-state policy and the remaining production gates.


- [ADR 0033: Durable printer intent and conservative recovery](../adr/0033-durable-printer-intent-and-recovery.md) covers manifest-v3 attempts, crash locks, conservative no-auto-retry states and read-only Windows queue inspection.


- [Store Configuration (schema v2)](STORE_CONFIG.md) defines safe printer defaults, queue/media/orientation/transform fields and in-memory v1-to-v2 migration; guest printing remains disconnected.
