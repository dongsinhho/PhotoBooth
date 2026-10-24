# Security and Privacy Baseline

**Status:** Proposed baseline

## Security goals
Protect booth availability, store accounts, transactions, downloaded content, local images, and optional cloud delivery. Treat packages, filenames, metadata, payment callbacks, and API responses as untrusted.

## Downloaded packages
Threats include path traversal, decompression bombs, malicious media, oversized assets, deceptive metadata, and code execution attempts.

Controls:
- Use an allowlisted declarative schema.
- Do not execute package scripts or load arbitrary external URLs.
- Enforce archive/path/size/file-count/image-dimension limits.
- Verify hashes and add publisher signatures when signing infrastructure exists.
- Stage and validate before activation; preserve a known-good version.
- Review media parsers, dependencies, and licenses.

## Hardware and local availability
Camera/printer disconnections, blocked I/O, full disk, driver crashes, and partial output can disrupt sessions.

Controls:
- Use timeouts, cancellation, bounded retries, and explicit state transitions.
- Keep device operations off the UI thread.
- Detect disk/output failures and show actionable recovery steps.
- Avoid deleting files until session/output state is known.

## Payment and entitlement
- Enforce server-side authorization for every store/creator resource.
- Verify provider callbacks or query trusted provider APIs; never trust a client payment-success claim.
- Make purchase and webhook processing idempotent.
- Define late payment, duplicate notification, timeout, refund, cancellation, and reconciliation paths.
- Issue a session-scoped permit only after authoritative confirmation.
- Use signed or integrity-protected entitlement data for legitimate offline use and define expiry/revocation rules.
- Never store customer banking credentials or raw payment secrets in the player.

## Media delivery security
- Local-only mode must not expose the photo directory through an unauthenticated network server.
- LAN mode must share only explicitly selected outputs, never the full directory or other sessions by default.
- Use high-entropy, short-lived, revocable tokens bound to one output/session; consider TLS, rate limits, and network binding.
- Do not assume LAN equals trusted; protect against guest-to-guest access.
- Cloud mode uses authenticated short-lived access, restricted object permissions, quotas, and explicit deletion/retention behavior.
- Cloud upload failure must not delete the only local copy.

## Guest-photo privacy
Default policy:
- Photos remain local unless the operator explicitly enables another delivery mode.
- Document retention and provide configurable cleanup.
- Do not place photos, faces, QR tokens, or guest identifiers in logs/analytics by default.
- For cloud delivery, define who can access content, token expiry, deletion behavior, storage region, and retention before implementation.
- Collect only minimum required data.
- Review Vietnamese privacy/data-protection obligations against current legal sources before launch.

## Updates and secrets
- Use authenticated transport and protect credentials with OS-appropriate secure storage where feasible.
- Never commit secrets or production settings.
- Redact authorization headers, signed URLs, tokens, and personal information from logs.
- Stage downloads, verify integrity, and preserve rollback for player/provider/content updates.
- Never load arbitrary native plugins from the marketplace in the first version.

## Marketplace launch blockers
- Moderation/takedown and intellectual-property complaint procedures.
- Validated content upload/download controls.
- Server-side authorization and payment/entitlement verification.
- Documented guest-photo retention and delivery policy.
- Player, provider and content update/rollback process.

