# Security and Privacy Baseline

**Status:** Proposed baseline

## Security goals
Protect booth availability, store accounts, transactions, downloaded content, local images, and optional cloud delivery. Treat packages, filenames, metadata, and API responses as untrusted.

## Downloaded package threats
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

## Accounts, payments, and entitlements
For the future marketplace:
- Enforce server-side authorization for every store/creator resource.
- Verify payment provider callbacks; never trust client payment-success claims.
- Make purchase/webhook processing idempotent.
- Use signed or integrity-protected entitlements for offline use and define revocation/grace behavior.
- Rate-limit sensitive endpoints and protect session secrets.

## Update and sync
- Use TLS and authenticated APIs.
- Validate package and application update metadata.
- Download to staging, verify before activation, and support rollback.
- Sync failure must not block local capture/printing.
- Do not overwrite assets in active use.

## Guest-photo privacy
Default policy proposal:
- Photos remain local unless the operator explicitly enables delivery.
- Document local retention behavior and provide configurable cleanup.
- Do not put photos, faces, QR tokens, or guest identifiers into logs, analytics, or crash reports by default.
- For cloud delivery, define access control, token expiry, deletion, storage region, and retention before implementation.
- Collect only the minimum data necessary.
- Review applicable Vietnamese privacy and data-protection obligations against current legal sources before launch.

## Secrets and operations
- Never commit secrets or production settings.
- Use appropriate OS-protected storage for credentials where feasible.
- Redact authorization headers, signed URLs, tokens, and personal information from logs.
- Define log rotation, permissions, backups, incident response, and update policy before commercial release.
- Review security and SDK licensing/support before release.

## Marketplace launch blockers
- Moderation and takedown process.
- Intellectual-property complaint and licensing process.
- Validated upload/download security controls.
- Server-side authorization and payment/entitlement checks.
- Documented guest-photo retention and cloud-delivery policy.
- Update/rollback process for player and content.

