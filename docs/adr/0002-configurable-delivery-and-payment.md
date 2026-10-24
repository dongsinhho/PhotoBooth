# ADR 0002: Configurable Media Delivery and Payment

- **Status:** Accepted product requirement; implementation not started.
- **Date:** 2026-10-24
- **Decision owner:** Product owner

## Context
Different photobooth stores have different budgets and workflows. Some want local storage and LAN downloads to avoid cloud costs; others want cloud delivery. Some offer free sessions, while others collect payment before or during the guest flow.

## Decision
Media delivery and payment are separate, per-store configurable policies.

Media delivery modes:
1. Local-only.
2. LAN download with controlled, session-scoped access.
3. Optional platform cloud service.

Payment modes:
1. Disabled/free.
2. Payment QR shown during the guest flow.
3. Verified prepayment before a session starts.

Implement these as independent modules/adapters. Session orchestration selects the configured policy but does not contain provider-specific payment or network storage implementation.

## Security and reliability consequences
- Local-only must not expose photo files on a network.
- LAN mode must authorize access to only the requested output; the network is not assumed trusted.
- Cloud mode needs explicit retention, deletion, access, quota, and cost policies.
- Payment success must be authoritative, not inferred from showing a QR or receiving a client-side claim.
- When payment is mandatory and verification is unavailable, fail closed with clear recovery; do not grant an unpaid session.
- Optional media delivery can fail after capture without deleting the local result.

## Alternatives considered
1. **Always upload photos to the platform cloud:** rejected as a default due to cost and privacy preferences.
2. **Always require payment:** rejected because free and store-specific models are required.
3. **Put payment logic in camera/session code:** rejected because provider and policy changes would couple unrelated modules.

## Open details
Payment provider, QR format, LAN access mechanism, cloud retention/price model, per-session override permissions, and exact policy schema remain undecided.

