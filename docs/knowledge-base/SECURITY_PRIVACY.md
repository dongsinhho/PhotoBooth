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


## Customer / Operator mode access boundary (2026-11-21)

PhotoBooth's guest-facing Customer Mode and authenticated Operator Mode are separate application authorization states. See [ADR 0035](../adr/0035-customer-operator-mode-and-auth-boundary.md).

- A fresh process always enters Customer Mode. Never restore authenticated operator state from a settings file, localStorage, a persisted boolean or a reusable session token.
- Go owns authorization. Hiding a frontend button or view is not a security control because the Wails WebView is bound to the Go App object.
- All settings, frame-package management, retention, interrupted-session inspection/recovery, printer queue resolution, local filesystem opening, SMB remote-administration controls and application exit must enforce a Go-side operator authorization check at the binding entry point.
- Customer UI receives only allowlisted result/status DTOs. Do not send local paths, session identifiers, printer queue/document/job metadata, old-session recovery details or authentication verifier material to Customer Mode.
- Operator credentials must never be stored/logged in plaintext. Use a reviewed salted adaptive password hash, protect the verifier using the logged-in OS user's file permissions and rate-limit guessing attempts. Recovery requires control of the operator OS account or another explicitly selected trusted mechanism; no universal backdoor.
- Operator authorization is process-local and revocable. Return/lock, inactivity timeout, OS lock/suspend, shutdown and restart remove privileged access.
- Do not allow mode transitions to interrupt capture, finalization, printer submission or output retry. Require safe worker quiescence.
- The app-level gate does not disable OS shortcuts or replace Windows Assigned Access / Linux desktop kiosk hardening. Do not claim whole-desktop kiosk security based on this mode boundary.
- The credential form, inactivity timeout and failed-attempt/backoff numeric policy must be selected before implementation. Five minutes is the current timeout proposal, not yet an approved value.


## Operator credential implementation update (2026-11-21)

The initial local Operator Authentication implementation is in internal/operatorauth and the Wails v2 Preview. See ADR 0035.

- One station-level 8-digit PIN; Argon2id verifier (64 MiB memory, 3 iterations, 2 lanes, 32-byte output) with a unique 16-byte salt.
- Credential store is strict/versioned, written atomically, and created without overwriting an existing credential. No plaintext PIN or logged-in Operator flag is persisted.
- Failed attempts and next-allowed timestamp persist across restart. Backoff increases from 2 seconds to a 15-minute cap; there is no permanent lockout.
- Operator access is process-local and automatically expires after 5 minutes without operator activity. Touch/keyboard activity refreshes it through a throttled backend heartbeat.
- Authenticated PIN change is available inside Operator Settings. Forgotten-PIN recovery requires control of the logged-in operator OS account: close PhotoBooth and run go run ./cmd/operator-setup --reset from an interactive terminal. The CLI never echoes the PIN. No reset binding exists in Wails or Customer Mode.
- Windows credential data is stored under the current user's profile cache directory (normally LocalAppData); Linux uses the current user's cache directory with owner-only filesystem permissions. The Windows profile DACL is relied upon; portable Go FileMode does not inspect its effective DACL.
- Customer-facing session projections omit internal session IDs, paths, raw errors and printer spool/job metadata; guest controls use a random active-session token.

Automated Go tests, Wails tests/vet/build and Linux auth/CLI cross-build passed on 2026-11-21. Manual runtime UI/security acceptance, OS session lock/suspend notification and whole-desktop kiosk hardening remain open. Do not represent the Wails Preview as a production-certified kiosk.


## Manual Customer/Operator UI smoke (2026-11-21)

On ShinPc, disposable profiles were used with synthetic-only mode. Customer Mode startup, hidden five-tap PB-logo gesture, Operator keypad visibility, Cancel removing keypad controls, and the guest synthetic capture/render/local-save path passed. The WebView no longer receives credential-existence status or State objects from the authentication/lock bindings; configuration absence, wrong PIN, corrupt store and backoff use one generic authentication error. The camera preview stayed disabled and the physical webcam was not opened. Three synthetic captures and the rendered/final JPEGs were verified, then the complete profile and test credential were deleted.

The PIN verification flow (wrong/correct entry), authenticated Settings, Change PIN, Return, five-minute UI inactivity lock and authenticated Exit remain pending manual acceptance. Backend authorization and credential tests passed separately. No real camera or printer acceptance was part of this smoke.


The Wails authentication and lock bindings return no credential State to the WebView. Unconfigured store, wrong PIN, corrupt store and persisted backoff are normalized to one generic authentication failure. A regression test verifies that configured and unconfigured cases produce the same binding error. Customer Mode has no public Operator navigation button; a hidden five-tap PB-logo gesture only opens the PIN challenge.


Per-user auto-start uses the logged-in operator account: Windows HKCU Run or Linux XDG autostart. No root/system GUI service is created. Windows removal only removes an entry whose target belongs to the chosen install root; Linux modifies only entries carrying the PhotoBooth ownership marker. Fullscreen Customer Mode is the default, and ESC/F11 cannot change window mode before Operator authentication. These controls do not replace OS-level kiosk policy.


## Auto-start lifecycle hardening (2026-11-22)

Windows auto-start registration/removal helpers reject registry paths outside the current user's HKCU Software hive. Removal also checks that the stored --kiosk executable target is within the specified PhotoBooth install root. Lifecycle tests use a unique HKCU test subkey and verify install/upgrade/rollback/uninstall without writing the primary user's real Run key. Actual Windows Explorer sign-in execution remains to be observed under the selected Administrator-group operator account; a separate Standard account is not part of the acceptance scope.
