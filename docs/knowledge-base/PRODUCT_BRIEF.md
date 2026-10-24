# Product Brief

**Status:** Accepted product requirements; implementation and legal terms not yet finalized
**Last updated:** 2026-10-25

## Vision

Build reliable, cross-platform photobooth software that lets operators reuse cameras, computers and printers they already own. The official app is intended to be free for commercial photobooth operation. Optional paid support and authorized custom engineering are the intended business model.

## Primary users

- **Store owner/operator:** hardware setup, session flow, configuration, delivery/payment policy and retention.
- **Guest:** quick, reliable capture and a clear way to receive the finished photos.
- **Frame/theme creator:** create visual content using supported formats; future distribution terms will define whether/how content may be shared or sold.
- **Support provider:** optional paid setup, troubleshooting, integration and custom engineering.

## Accepted product requirements

- The official application is intended to be free to use for commercial booth sessions, without a mandatory per-booth or per-session usage fee.
- The project does not sell cameras, computers, printers or complete booth kits.
- Operators may self-install and self-operate without purchasing support.
- User customization is limited to supported application-level settings and content such as frames/themes. The standard user grant does not include modifying core source code or redistributing modified core builds.
- Core modification or modified-build distribution requires prior written permission or a separate license, subject to finalized legal terms.
- Media delivery is configurable per store: local-only, controlled LAN access, or optional platform cloud.
- Payment is configurable per store: disabled/free, payment during the guest flow, or verified prepayment.
- Camera integrations are abstracted behind provider interfaces.
- Windows and Linux are target desktop platforms, with support claims limited to a tested compatibility matrix.
- Go is the preferred backend language.
- Capture, render and local output must not depend on Internet connectivity after configuration and asset installation, except when a configured payment flow requires authoritative online verification.

## Product surfaces

- **Player:** kiosk UI, session orchestration, camera capture, render, print/export, optional payment and media delivery.
- **Configuration:** supported settings, store/device policy, installed assets.
- **Content:** frame/theme packages with a documented compatibility contract.
- **Optional cloud services:** catalog, asset delivery, cloud photo delivery, commerce, creator portal and device sync.

## Differentiation hypothesis

The primary value proposition is lowering upfront software cost and allowing operators to reuse existing equipment, while retaining flexible configuration and optional paid expert help. Validate this with real operators; do not assume price alone is sufficient to motivate migration.

## Product principles

1. Free-to-use commercial operation is the intended default.
2. Normal customization happens at the application/content layer, not by editing core code.
3. Support is optional and must not be required for basic operation.
4. Capture is offline-first; optional cloud services must not become hidden requirements.
5. Payment and media delivery are independent modules.
6. Camera and printer integrations use explicit adapters and tested compatibility.
7. Downloaded content is validated declarative data, not arbitrary executable code.
8. Guest photos stay local by default.
9. Do not describe the controlled-core licensing model as OSI open source.
10. Document decisions and distinguish requirements from proposals and implementation facts.
