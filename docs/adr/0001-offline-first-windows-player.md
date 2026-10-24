# ADR 0001: Offline-First Windows Player

- **Status:** Accepted as a product/architecture constraint; implementation not started.
- **Date:** 2026-10-24
- **Decision owner:** Product owner

## Context
Photobooths are on-site experiences. Network access may be unstable, while capture, rendering, and printing must continue predictably. A cloud dependency in the synchronous session path would cause avoidable interruptions.

## Decision
The player must complete a configured session using installed local assets without a live Internet connection. Camera capture, rendering, print/export, and session state are local responsibilities. Cloud features are limited to non-critical workflows such as account/catalog access, package downloads, license refresh, and optional photo delivery.

Downloaded content is validated and staged locally before activation. Existing installed content remains usable when cloud synchronization fails.

## Consequences

### Positive
- Network outages do not automatically stop sessions.
- Capture latency is more predictable.
- Operators can continue using installed themes and frames.
- Capture incidents can be debugged separately from cloud availability.

### Trade-offs
- The player needs local persistence, cache cleanup, and recovery logic.
- Licenses and updates need an offline grace/revalidation policy.
- Cloud desired state can differ temporarily from the active local state.
- Cloud photo delivery needs explicit privacy and retention design.

## Alternatives considered
1. **Cloud-required capture:** rejected because it couples the critical path to network availability.
2. **Web-only player:** not selected as the initial direction because SDK, printer, kiosk, and offline compatibility with target hardware require validation. It may remain suitable for some surfaces.
3. **Hybrid local player with cloud content delivery:** selected as the intended direction.

## Revisit when
Revisit only if target workflows or hardware materially change. Preserve local capture resilience wherever feasible.

