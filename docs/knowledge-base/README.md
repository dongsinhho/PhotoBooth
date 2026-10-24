# PhotoBooth Knowledge Base

This folder is the shared product and engineering context for humans and AI coding agents. Keep it current and grounded in decisions actually made.

## Status vocabulary

- **Proposed** — recommended, not explicitly accepted or validated.
- **Accepted** — a product decision has been made; implementation may not exist.
- **Implemented** — behavior exists in code and has been verified.
- **Deprecated** — no longer current; retained only when historical context helps.

Never turn a proposal into an asserted fact.

## Read in this order

1. [PRODUCT_BRIEF.md](PRODUCT_BRIEF.md) — users, product value, and principles.
2. [MVP_SCOPE.md](MVP_SCOPE.md) — first milestone, acceptance criteria, and exclusions.
3. [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) — application boundaries and data flows.
4. [DOMAIN_MODEL.md](DOMAIN_MODEL.md) — concepts and relationships.
5. [CONTENT_PACKAGE_SPEC.md](CONTENT_PACKAGE_SPEC.md) — proposed theme/frame package contract.
6. [SECURITY_PRIVACY.md](SECURITY_PRIVACY.md) — package safety and guest-photo privacy.
7. [ENGINEERING_PLAYBOOK.md](ENGINEERING_PLAYBOOK.md) — development and testing practices.
8. [ROADMAP.md](ROADMAP.md) — staged rollout and validation gates.
9. [../adr/0001-offline-first-windows-player.md](../adr/0001-offline-first-windows-player.md) — initial architecture decision.

## Maintenance rules

- Update the nearest topic document when requirements or implementation change.
- Use ADRs for decisions expensive to reverse or affecting multiple modules.
- Add source links for vendor SDK limitations, licenses, security-sensitive behavior, and changeable external facts.
- Keep schemas/API contracts synchronized with implementation and tests.
- Never put secrets, personal data, real customer photos, or credentials here.

## Open questions

- Which camera models and Windows versions must be supported first?
- Which printer models, paper sizes, and output methods are required for the pilot?
- Which camera SDK/provider should be the first integration?
- What are the final client/backend languages and framework versions?
- Which animation and export formats are required?
- Should the first pilot include cloud sign-in/download, or start with local/manual content?
- What are the pricing, commission, refund, and creator payout rules?

