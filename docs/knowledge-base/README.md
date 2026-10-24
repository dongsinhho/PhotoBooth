# PhotoBooth Knowledge Base

This folder is shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary

- **Proposed** — recommended direction, not yet confirmed by evidence or product decision.
- **Accepted** — product requirement/decision is agreed; implementation may not exist.
- **Implemented** — exists in code and has been verified.
- **Deprecated** — no longer current; retain only if historical context helps.

Do not turn a proposal into an asserted fact.

## Read in this order

1. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) — end-to-end plan from discovery through engineering, pilot, delivery, and marketing.
2. [PRODUCT_BRIEF.md](PRODUCT_BRIEF.md) — target users, product value, configuration principles.
3. [MVP_SCOPE.md](MVP_SCOPE.md) — first milestone, criteria, exclusions, and validation.
4. [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) — player, cloud, and adapter boundaries.
5. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) — domain concepts and relationships.
6. [CAMERA_INTEGRATION.md](CAMERA_INTEGRATION.md) — hardware abstraction and compatibility.
7. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) — local/LAN/cloud delivery and optional payment flows.
8. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) — proposed theme/frame package contract.
9. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) — package safety, payment verification, and guest-photo privacy.
10. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) — development and testing practices.
11. [ROADMAP.md](ROADMAP.md) — staged rollout and validation gates.
12. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) — offline-first capture decision.
13. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) — per-store media and payment options.
14. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) — cross-platform and backend direction to validate.

## Maintenance rules

- Update the nearest topic document when requirements or implementation change.
- Use ADRs for decisions that affect multiple modules or are expensive to reverse.
- Add source links for vendor SDK support, licensing, security-sensitive behavior, and other changeable facts.
- Keep schemas/API contracts synchronized with implementation and tests.
- Never put secrets, personal data, real guest photos, or credentials here.

## Open questions

- Which camera models, SDKs, firmware, and connection methods must be supported first?
- Which printer models, paper sizes, and output methods are required for the pilot?
- Which UI framework best balances cross-platform UI reuse and device integration?
- What Linux distribution and kiosk configuration will be supported?
- What are the backend API contracts and final Go framework/library choices?
- Which payment provider and bank-transfer reconciliation workflow will be used?
- What LAN photo access model, security controls, retention, and deletion behavior are required?
- What are pricing, commission, refund, and creator payout rules?
