# Product Requirements Document (PRD)

**Version:** 0.2 â€” self-service/greenfield direction
**Status:** Proposed requirements; hardware validation pending
**Last updated:** 2026-10-25

## 1. Product summary

PhotoBooth is planned cross-platform software for people setting up a new photobooth, particularly DIY builders who want to reuse equipment they own or source second-hand. The official app is intended to be free for commercial booth operation. Users customize supported settings and add frames/themes at the application layer; editing core code is outside the standard user grant.

The project does not sell hardware. Optional paid services may include setup, troubleshooting, training, camera/printer integration, migration, and authorized custom development.

## 2. Problem hypothesis

People building their first booth may prefer assembling a solution from available or used hardware rather than purchasing a complete bundled system. They need software that is possible to install and configure themselves, with honest hardware compatibility and reliable photo output.

This is a hypothesis, not a proven market fact. The project will use technical proof, voluntary setup feedback, successful installations, and repeat usage to validate it. Formal interviews and conversion from incumbent software are not required gates.

## 3. Target users

### Primary: new booth builders
People creating their first booth or adding a new booth, especially self-builders using a camera, PC, and printer they already own or source second-hand.

### Secondary: existing operators
May use PhotoBooth on a new booth, spare/test rig, or eventual voluntary migration. Replacing a stable running system is not the core go-to-market strategy.

### Community participants
Technically curious users who report tested hardware configurations and create frames/themes using supported formats. Core modification remains outside the standard user grant under the intended controlled-core model.

## 4. Product promise

- Free use of the official player for commercial photobooth sessions is the intended model.
- No mandatory per-booth or per-session software fee is intended.
- Operators can self-install and use the app without paid support.
- Normal customization uses supported application settings and frame/theme content, not core source edits.
- Windows and Linux are target platforms; compatibility claims match tested combinations.
- Capture/render/configured local output work offline after setup, except where an explicitly chosen payment flow requires online verification.
- Media delivery can be local-only, controlled LAN download, or optional platform cloud.
- Payment can be disabled/free, required during the guest flow, or required before a session.

## 5. User jobs

### Self-builder/operator
- Check before installation whether their exact camera/OS/output combination is tested.
- Configure camera, output, kiosk behavior, and printer.
- Change supported countdown, shot count, delays, frame/theme, and print/export settings.
- Understand which features need LAN or Internet.
- Diagnose unsupported hardware without guesswork.
- Export/restore configuration and collect a privacy-safe support log.

### Guest
- Start a session with clear instructions.
- Complete capture without UI freeze.
- Receive the finished image using the store's selected method.
- Never access another guest's images.

### Content creator
- Create content using a documented format.
- Know player compatibility and content usage/license requirements.
- Preview the result before distributing it.

## 6. MVP candidate scope

- One real camera model and defined OS configuration.
- Session start, countdown, configured captures, original transfer, and deterministic static-frame render.
- Preview and one reliable output path.
- Local settings/session storage.
- Recoverable errors for camera disconnect, timeout, render, and output failures.
- Offline workflow for unpaid/local use.
- Configuration for the timing/shot-count settings intentionally exposed in the first version.
- Bundled sample frame and documented customization/compatibility path.
- Quickstart and troubleshooting sufficient for a self-install attempt.

Defer marketplace commerce/payouts, mandatory cloud delivery, arbitrary plugin code, universal camera/printer support, complex analytics, and features without evidence of need.

Windows and Linux remain targets; decide which is production-alpha first after technical feasibility. Untested operating systems must not be marketed as supported.

## 7. License and business requirements

- Official app commercial use is intended to be free.
- Users may self-install and configure supported application-level settings/content.
- Core modification and redistribution of modified core builds require separate authorization/license under final terms.
- Support is optional.
- Final user-facing license and content terms need legal review before commercial release.

See LICENSE_STRATEGY.md and ADR 0004.

## 8. Non-functional requirements

- Reliability: recoverable errors and clear operator action.
- Offline: no cloud dependency in unpaid local capture/render/output.
- Compatibility: publish a matrix of tested camera, OS, SDK/driver, and output.
- Security: theme/frame packages are declarative data, not arbitrary code.
- Privacy: no default guest-photo upload.
- Self-service: clear install, troubleshoot, restore, and update instructions.
- Maintainability: session/domain logic separate from OS/vendor APIs.
- Performance: establish limits through actual hardware tests.

## 9. Adoption and reliability metrics

Track:
- Successful first installs and first capture-to-output demos.
- Started/completed sessions and unrecoverable application failure, segmented by hardware/OS.
- Camera/render/output failure and recovery time.
- Time to set up a new booth.
- How often users configure settings/install a new frame without help.
- Issues solved by docs versus requiring direct support.
- Repeat usage, organic referrals, voluntary compatibility reports, and content contributions.
- Support hours per installation and the nature of support requests.

Downloads/stars/views alone do not prove adoption. Set numeric goals after the first real baseline. A suggested pilot target of 200 sessions and approximately 99% completion without an unrecoverable application failure is a proposed goal, not an existing claim.

## 10. Open questions

- Which camera/OS/output combination should be the first reference setup?
- Which settings are essential for the first self-service release?
- What level of Linux support is realistic after hardware validation?
- Which LAN/payment features should be implemented only after local alpha?
- How will third-party frames/themes be licensed and distributed?
