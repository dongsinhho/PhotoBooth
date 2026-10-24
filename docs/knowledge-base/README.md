# PhotoBooth Knowledge Base

This folder is the shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary

- **Proposed** - recommended direction, not yet confirmed.
- **Accepted** - product requirement/decision is agreed; implementation may not exist.
- **Implemented** - exists in code and has been verified.
- **Deprecated** - no longer current; retained only when useful for history.

Never turn a proposal into an asserted fact.

## Read in this order

1. [PRODUCT_PLAN.md](PRODUCT_PLAN.md) - end-to-end plan from discovery through engineering, pilot, delivery, and marketing.
2. [LICENSE_STRATEGY.md](LICENSE_STRATEGY.md) - free commercial use, app-level customization, core modification rights, and optional support.
3. [PRODUCT_BRIEF.md](PRODUCT_BRIEF.md) - target users, value proposition, and accepted requirements.
4. [MVP_SCOPE.md](MVP_SCOPE.md) - first milestone, acceptance criteria, and exclusions.
5. [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) - player, cloud, and adapter boundaries.
6. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) - domain concepts and relationships.
7. [CAMERA_INTEGRATION.md](CAMERA_INTEGRATION.md) - hardware abstraction and compatibility.
8. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment.
9. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - theme/frame package contract proposal.
10. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) - package safety, payment verification, and guest-photo privacy.
11. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) - development and test practices.
12. [ROADMAP.md](ROADMAP.md) - staged rollout and validation gates.
13. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) - offline-first capture constraint.
14. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) - store-configurable media/payment.
15. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) - cross-platform and backend direction.
16. [../adr/0004-free-commercial-use-and-paid-support.md](../adr/0004-free-commercial-use-and-paid-support.md) - free commercial use, app-level customization, and controlled core.

## Maintenance rules

- Update the nearest topic document when requirements or implementation change.
- Use ADRs for decisions that affect multiple modules or are expensive to reverse.
- Add source links for vendor SDK support, licensing, security-sensitive behavior, and other changeable facts.
- Keep schemas/API contracts synchronized with implementation and tests.
- Never put secrets, personal data, real guest photos, or credentials here.

## Open questions

- Which camera models, SDKs, firmware, and connection methods are first priority?
- Which printer models, paper sizes, and output methods are required for the pilot?
- Which UI framework and Linux distribution should be supported first?
- Which payment provider and payment-confirmation workflow will be used, if any?
- What LAN access, retention, and deletion policy is required?
- What exact end-user license, warranty/liability terms, and core-change authorization flow will be adopted?
- What permissions will apply to creating, distributing, or selling frame/theme packages?
