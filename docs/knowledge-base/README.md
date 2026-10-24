# PhotoBooth Knowledge Base

This folder is shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary

- **Proposed** - recommended direction, not yet confirmed by evidence or a product decision.
- **Accepted** - product requirement/decision is agreed; implementation may not exist.
- **Implemented** - exists in code and has been verified.
- **Deprecated** - no longer current; retained only when historical context helps.

Do not turn a proposal into an asserted fact.

## Read in this order

1. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) - end-to-end plan from discovery through engineering, pilot, delivery, and marketing.
2. [LICENSE_STRATEGY.md](LICENSE_STRATEGY.md) - free commercial use, core modification, and optional paid support.
3. [PRODUCT_BRIEF.md](PRODUCT_BRIEF.md) - target users, product value, and configuration principles.
4. [MVP_SCOPE.md](MVP_SCOPE.md) - first milestone, criteria, exclusions, and validation.
5. [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) - player, cloud, and adapter boundaries.
6. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) - domain concepts and relationships.
7. [CAMERA_INTEGRATION.md](CAMERA_INTEGRATION.md) - hardware abstraction and compatibility.
8. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment flows.
9. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - proposed theme/frame package contract.
10. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) - package safety, payment verification, and guest-photo privacy.
11. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) - development and testing practices.
12. [ROADMAP.md](ROADMAP.md) - staged rollout and validation gates.
13. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) - offline-first capture decision.
14. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) - per-store media and payment options.
15. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) - cross-platform and backend direction.
16. [../adr/0004-free-commercial-use-and-paid-support.md](../adr/0004-free-commercial-use-and-paid-support.md) - free commercial use and optional paid support.

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
- What exact software license, source publication policy, and third-party contribution terms will be chosen?
- What are pricing, commission, refund, and creator payout rules?
