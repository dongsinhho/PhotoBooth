# Product Brief

**Status:** Accepted requirements with open implementation choices
**Last updated:** 2026-10-24

## Vision
Build a reliable, configurable photobooth platform for store-operated booths. Guests take photos through a local player; operators control how images are stored/shared and whether a session requires payment. Online themes/frames and a community marketplace extend the product over time.

## Users
- **Store owner/operator:** hardware setup, session flow, store policy, content, payment, photo access, and retention.
- **Guest:** quick, attractive, predictable capture and a clear route to obtain the result.
- **Creator:** publish versioned, licensed themes/frames and eventually earn revenue.
- **Platform operator:** operate cloud services, content catalog, commerce, updates, moderation, and support.

## Accepted product requirements
- Media delivery is configurable per store: local-only, LAN access, or optional platform cloud.
- Payment is configurable per store: disabled/free, QR payment in the guest flow, or verified prepayment before a session starts.
- Camera integrations are abstracted behind provider interfaces to allow new models/providers in future releases.
- Windows and Linux are target desktop platforms; actual hardware support is a tested compatibility matrix.
- Go is the preferred backend language. Backend architecture/framework choices remain open.
- Capture, rendering, and local output must not depend on Internet connectivity after the booth is configured and required assets are installed.

## Product surfaces
- **Player:** kiosk UI, session orchestration, capture, render, print/export, payment integration, media delivery.
- **Store configuration:** media policy, payment policy, device configuration, installed assets.
- **Marketplace:** catalog, preview, free/paid content, purchase/download.
- **Creator portal:** upload, validation, preview, versioning, publishing, moderation.
- **Cloud media delivery:** optional service; not mandatory for every store or booth.

## Differentiation hypothesis
A smooth lifecycle for new visual content—discover, preview, install, activate, update—combined with configuration flexibility may be a differentiator. Validate market demand, hardware mix, and willingness to pay with real operators and creators.

## Principles
1. Offline-first capture.
2. Optional features must remain genuinely optional, not hidden cloud dependencies.
3. Separation of presentation, capture, payment, media access, and printing.
4. Adapter boundaries for replaceable hardware/payment/storage integrations.
5. Downloaded content is declarative and validated, not arbitrary executable code.
6. Privacy by default and explicit image retention/deletion policy.
7. Cross-platform claims are backed by platform-specific tests.
8. Document decisions and label unresolved assumptions.

## Distribution and support model

- The project does not sell hardware or bundled booth kits.
- The target offer is a free-to-use player for commercial photobooth operation, including self-installation.
- Supported configuration, frame/theme packages, and documented extension points should permit normal booth customization without core-code edits.
- Optional paid support and authorized custom engineering are the intended primary revenue sources.
- Core modification and redistribution rights must be stated clearly in the final license. The source license has not been selected; do not describe the software as OSI open source if modifications/derivatives require additional permission.
- A public marketplace or paid content is a possible future addition, not a prerequisite to use the basic player.
