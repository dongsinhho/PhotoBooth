# ADR 0005: Greenfield Self-Service Adoption

- **Status:** Accepted product/growth direction
- **Date:** 2026-10-25
- **Decision owner:** Product owner

## Context

PhotoBooth is intended to be free for commercial photobooth operation, with optional paid support. The target audience is primarily people setting up a new booth, often reusing available or second-hand equipment. Existing operators may be reluctant to replace a stable production system because switching creates operational risk.

The project has a low-cost maintenance strategy and does not need to win a short-term feature race against established vendors.

## Decision

1. Prioritize greenfield booth builders and DIY/self-build users who can choose software before committing to a vendor workflow.
2. Treat existing operators as a secondary audience for a new booth, spare setup, or later voluntary migration.
3. Formal customer interviews and incumbent migration are not mandatory project gates.
4. Grow gradually through working software, self-service setup, truthful hardware compatibility, clear documentation, optional community feedback, and organic referrals.
5. Do not use competitor feature parity or rapid market-share capture as the primary success criteria.
6. Track useful evidence: successful installations, completed capture-to-output sessions, repeat use, hardware compatibility reports, documentation-resolved issues, support effort, and organic referrals.
7. Competitor research remains useful for context, pricing awareness, and avoiding false claims; it is not a competitive battle plan.
8. Do not treat page views, stars, downloads, or polite interest by themselves as proof of adoption.

## Consequences

### Positive
- The product is optimized around the initial setup decision rather than expensive switching.
- The project can grow at a sustainable pace without a heavy sales/research process.
- Documentation and compatibility reports become core product assets.
- Support load can be kept manageable through self-service.

### Trade-offs
- Organic adoption may be slow and less predictable.
- Less direct user contact can delay discovery of confusing workflows; provide low-friction voluntary feedback.
- The project still needs real hardware validation. Skipping interviews does not mean skipping technical tests.
- The team must resist expanding features based only on competitor checklists.

## Revisit when

Revisit if actual usage shows that the main audience differs, voluntary feedback is insufficient to resolve product risks, or the support burden makes the current strategy unsustainable.
