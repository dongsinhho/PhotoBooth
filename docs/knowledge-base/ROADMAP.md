# Roadmap and Validation Gates

**Status:** Proposed sequence; dates intentionally unset

## Stage 0 — Validate product and hardware assumptions
- Interview target booth operators and inspect workflows.
- Select an initial camera model, Windows environment, and printer/output path.
- Verify SDK availability, model support, redistribution rules, and connection stability.
- Collect representative frame layouts and expected output quality.
- Decide whether the pilot needs cloud sign-in/download or can start locally.

**Exit gate:** exact hardware configuration and measurable capture-to-output acceptance test agreed.

## Stage 1 — Reliable local player
- Kiosk startup and session workflow.
- One camera adapter and source-image handling.
- Static frame/render pipeline.
- Preview and one tested output path.
- Local persistence, diagnostics, recovery, and offline operation.
- Automated tests for domain/render/package boundaries.

**Exit gate:** repeated end-to-end sessions pass on target hardware, including documented failure/recovery scenarios.

## Stage 2 — Local content management
- Formal manifest and frame schema.
- Bundled/sample package loading.
- Package validation, installation, activation, and rollback.
- Theme presentation schema and a small set of built-in themes.

**Exit gate:** content can be switched without code changes or destabilizing a running session.

## Stage 3 — Cloud catalog and delivery
- Account/store/device model as required.
- Catalog and content metadata APIs.
- Object storage/CDN delivery.
- Authenticated downloads, integrity validation, versioning, and offline cache behavior.

**Exit gate:** operator can discover/install content online and continue using it offline.

## Stage 4 — Marketplace and creators
- Creator publishing portal and moderation.
- Free/paid listings, payment, orders, entitlements, refunds, and licensing.
- Creator attribution and basic download/sales reporting.
- Security, privacy, and operational review.

**Exit gate:** test purchases, takedowns, and refunds are repeatable and supportable.

## Stage 5 — Growth features
- Multiple booth/store management.
- Advanced theme editor.
- Animated digital exports, promotions, scheduling, and analytics.
- Additional camera/printer integrations based on validated demand.

## Cross-stage rules
- Do not start later stages before current validation gates are met.
- Update product/architecture docs as decisions become accepted or implementation changes.
- Base market sizing, pricing, conversion, creator economics, and reliability targets on pilot evidence.

