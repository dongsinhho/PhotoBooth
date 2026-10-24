# Roadmap and Validation Gates

**Status:** Proposed sequence; dates intentionally unset

## Stage 0 - Self-service readiness and hardware selection
- Select a reference camera/PC/output configuration likely to be available to new booth builders.
- Prepare quickstart, troubleshooting, sample configuration, and voluntary issue/compatibility-report templates.
- Use optional user feedback when it arises; do not require formal interviews or incumbent-store migration.
- Select one camera model, one output path, a Windows baseline, and a specific Linux distribution for the technical spike.
- Verify SDK operations, supported model/OS combinations, license terms, and redistribution rules.
- Validate kiosk behavior, camera trigger/transfer, printing, auto-start, and updates.
- Decide how the pilot will configure local/LAN/cloud delivery and free/paid sessions.
- Choose a verifiable payment-confirmation method before accepting money.

**Exit gate:** self-service setup path is documented, reference hardware/OS test matrix is selected, and a measurable capture-to-output acceptance test is defined. Formal interviews are not required.

## Stage 1 - Reliable local player
- Kiosk startup and session workflow.
- One camera provider and source-image transfer.
- Static frame/render pipeline.
- Preview and one tested output path.
- Local persistence, diagnostics, recovery, and offline operation.
- First deployment package and update/rollback plan.

**Exit gate:** repeated end-to-end sessions pass on target hardware, including failure/recovery tests.

## Stage 2 - Configuration and local output
- Explicit store configuration schema and validation.
- Local-only media delivery and retention/cleanup.
- Disabled/free payment path.
- Local asset/package installation and rollback.
- Payment/media modules remain replaceable and separate from capture.

**Exit gate:** settings can change modes without modifying capture/provider code; local mode never exposes photos on a network accidentally.

## Stage 3 - LAN photo access and optional payment
- Session-scoped LAN download/QR with short-lived authorization.
- Network/interface binding and guest-to-guest access protections.
- One selected payment integration if the pilot requires it.
- Server/provider-verified payment state, idempotency, expiry, duplicate/late callback handling, and reconciliation.

**Exit gate:** tests prove that one session cannot access another session's output and unpaid sessions cannot start when payment is required.

## Stage 4 - Cross-platform validation and cloud delivery
- Linux build and installation on the selected distribution.
- Camera/printer integration checks on Linux; document differences from Windows.
- Optional authenticated cloud upload/download, retention, deletion, quotas, and cost model.
- Content catalog delivery and device sync.

**Exit gate:** Linux compatibility claims cover only tested camera/OS/driver/output combinations; cloud failures never destroy local output.

## Stage 5 - Marketplace and creators
- Creator publishing, validation, previews, versioning, and moderation.
- Free/paid listings, orders, entitlements, refunds, licensing, and creator attribution.
- Payment reconciliation, support, and operational monitoring.

**Exit gate:** purchases, refunds, takedowns, content rollback, and entitlement checks are repeatable and supportable.

## Stage 6 - Growth features
- Multi-booth/store management.
- Advanced theme editor.
- Animated digital exports, promotions, scheduling, and analytics.
- Additional camera/printer providers based on observed demand.

## Cross-stage rules
- Do not start later stages just because they are technically interesting; satisfy current validation gates first.
- Keep options documented, but implement only validated needs.
- Market sizing, pricing, creator economics, cloud cost, and reliability targets should be based on actual evidence.
- Formal interviews are optional. Real hardware tests and observed product behavior are still required.

See [ADR 0005](../adr/0005-greenfield-self-service-adoption.md) for the greenfield/self-service growth strategy.

