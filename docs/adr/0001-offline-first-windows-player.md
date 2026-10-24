# ADR 0001: Offline-First Player

- **Status:** Accepted as a product/architecture constraint; implementation not started.
- **Date:** 2026-10-24
- **Decision owner:** Product owner

## Context
Photobooths are on-site experiences. Network access may be unstable, while capture, rendering, and printing must continue predictably. A cloud dependency in the synchronous session path would cause avoidable interruptions.

## Decision
The player must complete a configured session using installed local assets without a live Internet connection, except when a store explicitly requires online payment authorization before the session. Camera capture, rendering, print/export, and session state are local responsibilities. Catalog access, package downloads, optional cloud photo delivery and payment verification are separate workflows.

Downloaded content is validated and staged before activation. Existing installed content remains usable when cloud synchronization fails. If payment is required and authoritative confirmation cannot be obtained, the application must not grant the paid session.

## Consequences

### Positive
- Network outages do not automatically stop unpaid/local sessions.
- Capture latency is more predictable.
- Operators can continue using installed themes and frames.
- Capture incidents can be debugged separately from cloud availability.

### Trade-offs
- The player needs local persistence, cache cleanup, and recovery logic.
- Online payment-required sessions need explicit timeout/reconciliation behavior.
- Media policies need to distinguish local output success from upload/access status.

## Alternatives considered
1. **Cloud-required capture:** rejected because it couples the critical path to network availability.
2. **Web-only player:** not selected as the initial direction because SDK, printer, kiosk, and offline compatibility require validation.
3. **Hybrid local player with optional cloud features:** selected as the intended direction.

## Revisit when
Revisit only if target workflows or hardware materially change. Preserve local capture resilience wherever feasible.

