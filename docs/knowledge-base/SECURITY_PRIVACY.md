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

### Implemented local frame-package controls (2026-10-29)

The first .pbframe implementation is a strict, allowlisted subset rather than a general package/plugin system. The ZIP reader limits archive and per-entry sizes/count, rejects path traversal, duplicate/unknown entries, symlinks and unsupported compression, verifies declared payload sizes and SHA-256 digests, validates optional preview image content/dimensions, and validates definition.json with the production schema-v1 renderer. Installation stages files and writes immutable ID/version directories; activation keeps a digest-verified rollback copy. Full details and limits are in [FRAME_PACKAGE_FORMAT.md](FRAME_PACKAGE_FORMAT.md).

SHA-256 is an integrity control, not a signature: package v1 does not authenticate authors, verify license rights, use publisher signatures, moderate creator content or fetch any network content. Treat packages from unknown sources as untrusted and review the declared license before distributing them.

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
- Document retention and provide configurable cleanup. The implemented local cleanup is opt-in, dry-run by default, requires a reviewed plan digest plus `--apply`, and only targets recognized finalized PhotoBooth layouts. It is not run at player startup; see [LOCAL_DATA_RETENTION.md](LOCAL_DATA_RETENTION.md).
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



## Windows booth SMB server policy (accepted 2026-11-16)

- SMB Server (LanmanServer) and workstation administrative shares are disabled by default for deployed Windows booth images when no local workflow dependency requires them. Do not disable the SMB client service or add outbound SMB blocks as part of this policy.
- The booth's supported media delivery remains local-only. SMB remote administration is a separate operator-maintenance capability, never a guest-photo delivery mechanism.
- Explicit opt-in requires Windows UAC elevation, an already Private-classified physical adapter, and a literal allowlist of individually trusted RFC1918 remote IPv4 sources. The helper must not reclassify a Public network.
- Allow only direct-hosted SMB TCP 445 on the selected local address and Private profile. Legacy NetBIOS TCP 137/139 and UDP 137/138 remain blocked. Do not enable SMB1, guest/anonymous SMB, router port-forwarding, or broad inbound scopes.
- The network allowlist is not authentication or authorization. Existing Windows SMB authentication and share/NTFS ACLs still apply; opening TCP 445 does not grant access to PhotoBooth directories.
- State-changing actions preflight sessions, dependencies, custom shares and conflicting firewall rules; abort on blockers rather than force-stop or alter unrelated policy. The helper owns only its named firewall rules and does not delete custom shares or rewrite ACLs.
- The Server service also supports file, print and named-pipe/RPC sharing. Inventory dependencies and validate that local PhotoBooth capture/render/output works with LanmanServer disabled before deploying this baseline. The helper prototype is not yet VM-validated or integrated into the operator UI/provisioner; see [ADR 0031](../adr/0031-opt-in-windows-smb-administration.md) and [self-service helper docs](../../tools/self-service/README.md).


**2026-11-16 SMB-off local regression:** the real Wails B525 preview/capture smoke passed with LanmanServer Stopped/Disabled. It verified preview before capture, exclusive camera handoff, 3/3 real captures, render, local output and preview restoration. Temporary test data was removed; only the JSON report remained. See NETWORK_EXPOSURE_AUDIT.md and HANDOFF_2026-11-16.md.
