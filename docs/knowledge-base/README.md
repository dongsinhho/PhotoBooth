# PhotoBooth Knowledge Base

This folder is shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary

- **Proposed** - recommended direction, not yet confirmed.
- **Accepted** - product requirement/decision is agreed; implementation may not exist.
- **Implemented** - exists in code and has been verified.
- **Deprecated** - no longer current; retained only when useful for history.

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
11. [CAMERA_INTEGRATION.md](CAMERA_INTEGRATION.md) - hardware abstraction and compatibility.
12. [MEDIA_DELIVERY_AND_PAYMENT.md](MEDIA_DELIVERY_AND_PAYMENT.md) - local/LAN/cloud delivery and optional payment.
13. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) - theme/frame package contract proposal.
14. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) - package safety, payment verification, and guest-photo privacy.
15. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) - development and test practices.
16. [ROADMAP.md](ROADMAP.md) - staged rollout and validation gates.
17. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) - offline-first capture constraint.
18. [../adr/0002-configurable-delivery-and-payment.md](../adr/0002-configurable-delivery-and-payment.md) - store-configurable media/payment.
19. [../adr/0003-cross-platform-player-and-go-backend.md](../adr/0003-cross-platform-player-and-go-backend.md) - cross-platform and backend direction.
20. [../adr/0004-free-commercial-use-and-paid-support.md](../adr/0004-free-commercial-use-and-paid-support.md) - free commercial use and controlled core.
21. [../adr/0005-greenfield-self-service-adoption.md](../adr/0005-greenfield-self-service-adoption.md) - greenfield audience and gradual-growth strategy.

## Current Phase 0 status as of 2026-10-25

- PRD, competitor desk research, optional feedback guide, and self-service adoption log have been prepared.
- Formal interviews are optional and not required for the next phase.
- Competitor research is contextual and based on vendor-published claims; it is not independent product testing.
- The next required evidence is a real camera-to-output technical proof of concept on a selected hardware configuration.
- Product adoption, self-install success, hardware compatibility, and support demand have not yet been validated through actual use.

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
