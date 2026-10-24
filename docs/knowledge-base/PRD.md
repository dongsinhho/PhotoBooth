# Product Requirements Document (PRD)

**Version:** 0.1 — discovery draft  
**Status:** Proposed; customer discovery and hardware validation pending  
**Last updated:** 2026-10-25

## 1. Product summary

PhotoBooth is planned cross-platform photobooth software for operators who want to reuse cameras, computers, and printers they already own. The official app is intended to be free for commercial photobooth operation. Users configure supported behavior and add themes/frames at the application layer; changing the core code is outside the standard user grant.

The project does not sell hardware. Optional paid services may include setup, troubleshooting, training, camera/printer integration, migration, and authorized custom development.

## 2. Problem statement (hypothesis)

Some photobooth solutions are sold as bundled hardware/software/services at a substantial upfront cost. Other operators may already own a suitable camera, PC, and printer but need a workable way to operate them as a booth. We hypothesize they value a no-app-fee, self-installable alternative, provided hardware setup and reliability are manageable.

This problem statement is not yet validated. Do not use it as a proven market fact until operator interviews and a real hardware prototype support it.

## 3. Target users to investigate

### Primary discovery segment
Independent booth operators and small operators with roughly 1–5 booths, especially those who:
- already own or can source used cameras, PCs, and printers;
- can do basic installation/configuration or are willing to learn;
- operate a booth commercially and care about downtime;
- currently pay for software/support or use bundled vendor solutions.

### Secondary segments
- Event-rental operators that bring equipment to different venues.
- DIY/hobby builders who may become commercial operators.
- Frame/theme designers who could make content for the ecosystem.

Do not combine these segments into one persona until their hardware, workflow and support needs are compared.

## 4. Product promise

- Free use of the official player for commercial photobooth sessions is the intended model.
- No mandatory per-booth or per-session software fee is intended.
- Operators may self-install and use the app without purchasing support.
- Normal customization uses supported application settings and frame/theme formats, not core source edits.
- Windows and Linux are target platforms; compatibility claims must match tested combinations.
- Capture, rendering, and configured local output work offline after setup and asset installation, except where an explicitly selected payment flow requires online verification.
- Media delivery can be local-only, controlled LAN download, or optional platform cloud.
- Payment can be disabled/free, required during the guest flow, or required before a session.

## 5. User jobs and candidate requirements

### Setup/operator
- Configure camera, output path, touch/kiosk behavior, and printer.
- Set capture count, countdown, delay, frame, image layout, and supported printing options.
- Understand whether a feature requires LAN or Internet.
- Diagnose unsupported hardware without guesswork.

### Guest
- Start a session with clear instructions.
- Complete capture without UI freeze.
- Receive the photo using the store's configured method.
- Not see or access another guest's images.

### Self-service maintainer
- Install updates and rollback safely.
- Export/restore configuration.
- Find logs and support guidance without exposing guest photos.

### Frame/theme creator
- Create content using a documented format.
- Know compatible player versions, required dimensions, rights, and license terms.
- Preview the result before distributing it.

## 6. MVP candidate scope

### Must demonstrate
- One real camera model and one defined OS/device setup.
- Session start, countdown, configured captures, original-image transfer, and deterministic static-frame rendering.
- Preview and at least one reliable output path (file export or printer).
- Local configuration and persistence.
- Recoverable errors for camera disconnect, timeout, render failure, and output failure.
- Operation without Internet for the configured unpaid/local workflow.
- Configuration of the timing/shot-count values deliberately exposed by the first release.
- A small bundled sample-frame pack and documented content/configuration format.

### Defer
- Public paid marketplace and creator payouts.
- Cloud photo delivery as a required feature.
- General support for every camera, printer, and Linux distribution.
- Arbitrary third-party code/plugins.
- Multi-service cloud architecture.
- Complex analytics, video/GIF output, and advanced creator editor unless research changes priority.

### Scope decision pending
Whether the first public alpha should run on Windows only or include a Linux preview must be decided after the Phase 1 hardware/UI spike. Windows and Linux remain product targets, but untested platforms must not be marketed as production-supported.

## 7. Licensing and business requirements

- Official app use for commercial booth sessions is intended to be free.
- Users may adjust supported application-level settings and create/install supported frame/theme content.
- Standard user rights do not include core source modification or redistribution of a modified core build.
- Core modification/redistribution requires prior written permission or a separate license under final legal terms.
- Support is optional; do not artificially disable normal configuration to force purchases.
- Final user-facing license and content terms require qualified legal review before commercial release.

See [LICENSE_STRATEGY.md](LICENSE_STRATEGY.md) and ADR 0004.

## 8. Non-functional requirements

- **Reliability:** errors must be recoverable or have clear operator actions.
- **Offline:** no cloud dependency in the unpaid local capture/render/output path.
- **Compatibility:** publish a tested camera/OS/driver/printer matrix.
- **Security:** themes and frame packages are declarative data; validate all downloaded files.
- **Privacy:** no default upload of guest photos; define retention and delete behavior for every delivery mode.
- **Usability:** self-install path and operator guide should not require paid support.
- **Maintainability:** keep session/domain logic separated from OS APIs and vendor SDKs.
- **Performance:** determine limits through actual camera, render, and printer tests rather than assumptions.

## 9. Success metrics to instrument during pilot

- Started vs. completed sessions, segmented by OS and camera/provider.
- Unrecoverable application failure rate; hardware failures categorized separately.
- Camera/render/output failure and recovery time.
- New-booth setup time.
- Percentage of operators able to configure a setting or activate a frame without help.
- Optional LAN/cloud delivery success and access-isolation failures.
- Support time per booth.
- Design-partner retention and willingness to recommend/use repeatedly.

Set numeric targets after a baseline exists. Suggested pilot target of 200 sessions and approximately 99% completion without an unrecoverable application failure is a hypothesis, not an existing performance claim.

## 10. Open questions / decision log

- What exact hardware is most common in the first target segment?
- Is the top pain upfront hardware cost, software license cost, support dependency, or lack of control?
- Which printer/output path is required for the first pilot?
- Does the first pilot need LAN access or payment, or should it prove capture first?
- What does “configuration” need to cover before operators consider the app useful?
- Which OS should be the first supported production target based on hardware evidence?
- Which exact commercial license wording is legally acceptable?

