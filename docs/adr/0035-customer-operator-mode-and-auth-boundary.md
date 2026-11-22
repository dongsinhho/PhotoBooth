# ADR 0035: Customer/Operator modes and authorization boundary

**Status:** Accepted interaction and authorization contract. Initial Wails Preview authentication/controller implementation is recorded below; production runtime and OS kiosk acceptance remain open.
**Date:** 2026-11-21
**Decision owner:** Product owner

## Context

ADR 0034 selected a fullscreen, touchscreen-first Customer Mode and an authenticated Operator Mode. An operator can return to Customer Mode or authenticate and exit PhotoBooth to the logged-in OS desktop. Every process launch must start in Customer Mode.

The current Wails v2 prototype does not enforce this separation:
- The idle screen exposes a direct Settings & frame button.
- Settings reads/writes and frame-package operations are reachable from the Wails frontend.
- The Wails host binds the entire \`App\` object to the WebView (\`Bind: []interface{}{app}\`). Hiding a button or changing frontend navigation is not authorization.
- The current kiosk Escape/F11 behavior deliberately allows leaving fullscreen without authentication.
- Interrupted-session recovery, printer queue inspection/resolution, retention cleanup and SMB remote administration are present in the prototype but have no Customer/Operator authorization boundary.

This ADR defines the boundary and lifecycle. The initial implementation has been added to the Wails v2 Preview; the implementation notes below distinguish automated code/build acceptance from production hardware/runtime acceptance.

## Decisions

### 1. Modes and runtime states

The application has two user modes:

- **Customer Mode:** the only mode entered automatically at process startup. It exposes the guest capture workflow and only the minimum result information needed by the guest.
- **Operator Mode:** a temporary authenticated maintenance mode. It exposes settings, diagnostics, recovery, frame management and administration controls.

The runtime controller uses these states:

| State | Meaning | Allowed next states |
|---|---|---|
| \`starting\` | App is initializing and has not exposed interactive controls | \`customer\`, \`startup_blocked\` |
| \`customer\` | Guest-facing kiosk is active | \`operator_auth_pending\`, \`exiting\` only through an internal shutdown path |
| \`operator_auth_pending\` | Authentication prompt is open; no operator privilege has been granted | \`customer\`, \`operator\` |
| \`operator\` | Current process has a valid authenticated operator session | \`customer\`, \`exiting\` |
| \`startup_blocked\` | A safe guest workflow cannot be offered (for example, an unresolved recovery requiring operator action) | \`operator_auth_pending\` |
| \`exiting\` | Authorized shutdown is quiescing resources before process exit | terminal |

The state is held in the Go application process and is never restored from persisted UI state. A restart, crash or new process always begins at \`starting\` and resolves to Customer Mode unless startup integrity/recovery checks require \`startup_blocked\`.

Photo-session lifecycle (idle/capturing/finalizing/completed/failed/cancelled) is a separate state machine. A session state does not grant operator privileges.

### 2. State transitions

| From | Trigger | Guard/action | To |
|---|---|---|---|
| \`starting\` | startup completed | Load config; perform bounded recovery discovery; do not restore prior operator privilege | \`customer\` or \`startup_blocked\` |
| \`customer\` | operator-entry gesture/action | Open authentication prompt; no privileged data is loaded yet | \`operator_auth_pending\` |
| \`operator_auth_pending\` | cancel/back | Clear credential input and challenge state | \`customer\` |
| \`operator_auth_pending\` | authentication failure | Apply the configured attempt-throttling policy; do not reveal whether an operator account exists | \`operator_auth_pending\` or \`customer\` |
| \`operator_auth_pending\` | authentication success | Create an in-memory operator authorization context; load operator-only views | \`operator\` |
| \`operator\` | Return to booth / Lock | Revoke the authorization context, clear operator UI state and unsaved secrets; restore Customer Mode | \`customer\` |
| \`operator\` | inactivity timeout or OS lock/suspend | Revoke authorization; discard transient privileged state; require fresh authentication after resume | \`customer\` |
| \`operator\` | authenticated Exit application | Verify no unsafe operation is active; stop preview/workers and close resources; invoke native app quit | \`exiting\` |
| any | process restart/crash | All in-memory authorization is lost | new process starts at \`starting\` |

The exact operator-entry gesture is a UI choice and must not be a visible customer navigation button. Do not rely on obscurity as the authorization control; authentication is mandatory after the gesture.

### 3. Customer Mode permissions

Customer Mode may:
- Start a new configured guest photo session.
- Operate the current guest session: view progress, cancel according to the session workflow, continue/retry only the active session's supported recovery action, and view its final photo/result.
- See a safe, guest-oriented message when the booth cannot accept a new session.

Customer Mode must not:
- Read or change station settings, frame paths, camera/printer configuration, retention policy, payment policy or local output roots.
- Browse, install, activate, roll back or remove frame packages.
- Read the list of interrupted/other sessions, their IDs, filesystem paths, manifests or detailed diagnostics.
- Inspect printer queues, reconcile printer locks, resolve print attempts or see spooler/document/job identifiers.
- Preview/apply retention cleanup or delete existing guest data.
- Enable/disable SMB administration or access remote-maintenance configuration.
- Open Explorer/file manager or reveal local photo directories.
- Exit the app, unfullscreen the kiosk or toggle windowed mode through customer-accessible keyboard shortcuts.

Customer-facing DTOs must be allowlisted projections, not the existing operator-rich structs with a few fields hidden in JavaScript. Do not send absolute paths, session UUIDs, local config, printer queue identifiers, lock metadata or internal diagnostic details to Customer Mode.

If startup recovery discovery finds work requiring an operator decision, do not expose recovery cards or old photo/session details to the guest. Enter \`startup_blocked\`, show a generic service-required message and provide only the operator-entry route.

### 4. Operator Mode permissions

An authenticated operator may:
- Read and edit supported station settings.
- Browse, validate, install, activate, roll back and uninstall frame packages.
- Inspect interrupted sessions and invoke the explicitly supported recovery actions.
- Inspect Windows printer queue state and create the existing explicit print resolution records.
- Preview and apply the existing reviewed retention-cleanup flow.
- Inspect and change the existing opt-in remote administration setting, subject to its current OS helper/UAC safeguards.
- Open local output/session locations for maintenance.
- Exit PhotoBooth to the OS desktop.

Operator Mode must not bypass the underlying safety invariants. Existing validation, confirmation, local-path checks, output isolation, printer uncertainty rules, UAC and fail-closed behavior remain in force. Authentication only grants access to the operation; it does not make an unsafe operation valid.

### 5. Backend authorization is authoritative

The Go side owns the mode state and authorization decisions. Frontend route guards, hidden buttons, CSS classes and disabled controls are presentation only.

- Add a centralized authorization guard for each operator-only binding. Every read, write and state-changing method must enforce it at invocation time.
- Do not authorize based on a caller-supplied boolean, role string, frontend route, localStorage value or UI event.
- Operator authorization must be process-local, short-lived and revocable. Do not persist an authenticated flag or a reusable operator session token to disk.
- Locking, timeout, process shutdown and authentication-session invalidation must revoke the authorization context before returning control to Customer Mode.
- Authorization failure must return a generic permission error and must not reveal paths, settings, account existence, recovery metadata or privileged state.
- Internal Go helpers may be used by customer and operator workflows, but exposed Wails binding methods must still guard their public entry point.
- Make authorization checks race-safe with operator lock/exit and ongoing operations. A privileged operation must not continue silently after its authorization context is revoked unless the operation has an explicit, documented safe-completion rule.
- The UI may fetch a minimal mode/status DTO. It must never receive the credential hash, salt, verifier parameters, lockout internals or other authentication secrets.

### 6. Sensitive binding inventory for the first implementation

The following existing Wails methods should be treated as operator-only unless a narrower safe customer projection is explicitly introduced:

- Settings and filesystem selection: \`GetPlayerSettings\`, \`SavePlayerSettings\`, \`BrowseFrameConfig\`.
- Frame packages: \`BrowseFramePackage\`, \`ValidateFramePackage\`, \`InstallFramePackage\`, \`GetFramePackageState\`, \`UninstallFramePackage\`, \`ActivateFramePackage\`, \`RollbackActiveFrame\`.
- Retention: \`PreviewRetentionCleanup\`, \`ApplyRetentionCleanup\`, \`DiscardRetentionPreview\`.
- Session recovery and local-file access: \`ListInterruptedPhotoSessions\`, \`ContinueRecoveredPhotoSession\`, \`RestartRecoveredPhotoSession\`, \`RetryRecoveredPhotoOutput\`, \`OpenPhotoSessionFolder\`, \`OpenPhotoOutputFolder\`.
- Printer maintenance: \`InspectPrinterAttempt\`, \`ResolvePrinterAttempt\`, \`ReconcileOrphanedPrinterLock\`.
- Remote administration: \`GetSmbAdminStatus\`, \`GetSmbAdminInterfaces\`, \`SetSmbAdminEnabled\`.
- Kiosk/host control: \`ToggleKioskMode\`, \`ExitKioskMode\`, and the future authenticated application-quit method.

Customer-facing session operations (start, read current guest progress, cancel current guest session, and view a safe result projection) remain usable without operator authentication. Their arguments and session identity must still be validated by Go. Synthetic/demo entry points are development-only and must not appear in a production Customer Mode build unless explicitly enabled by a separately defined product setting.

This inventory is based on the current Wails spike. Re-audit all exported methods whenever a new binding is added.

### 7. Session-safe mode changes

- Do not enter Operator Mode while a guest camera capture, render/finalization, printer submission or local-output retry is active.
- The UI must explain that the current session must finish or be cancelled before maintenance can begin. Do not silently terminate a guest capture to open Settings.
- Entry is allowed from idle, a completed result, a stable failed/cancelled result, or \`startup_blocked\`, provided all device/output workers have reached their required quiescent state.
- The camera-preview worker must be stopped and released before operator maintenance opens device Settings or other views that can compete for the camera.
- Operator Mode cannot start a guest session. Return to Customer Mode first.
- On return to Customer Mode, close operator-only views, clear sensitive/transient view state and restart the guest preview only after the normal camera lifecycle barrier succeeds.
- A failed transition must leave the previous mode intact and report a safe, non-sensitive reason.

### 8. Authentication and credential handling

This ADR chooses the authorization contract, not the final credential UX. The first implementation must satisfy all of the following:

- Authenticate locally; free/local booth use must not require Internet or a cloud account.
- Never store or log a plaintext password/PIN. Use a reviewed, salted, adaptive password-hashing scheme and protect the verifier file with the logged-in OS user's file permissions. A memory-only secret must be cleared after verification.
- Rate-limit repeated failed attempts. The policy must resist rapid guessing but avoid permanent lockout that would make an offline booth unrecoverable.
- Do not expose whether a credential/account exists. Failed authentication returns one generic result.
- Define a supported credential reset/recovery path that requires control of the operator OS account or another deliberately selected trusted mechanism. Do not provide a customer-visible reset flow or a universal vendor backdoor.
- The authentication verifier and failed-attempt policy must survive ordinary app restarts so restarting the process cannot reset guessing limits.
- Keep operator authorization in memory only. Do not persist the authenticated state.
- Do not use customer photos or session metadata as an authentication factor.
- Authentication should have an accessible touchscreen entry path and must not depend on a physical keyboard being attached.

The first Wails Preview implementation uses one station-level 8-digit PIN entered through a touch keypad. Distinct named operator accounts can be introduced later if operator attribution is required; this single-station credential must not be treated as a meaningful named identity for manual payment audit. The manual payment-confirmation feature still requires a separate identity/audit decision.

**Accepted credential policy:** one station-level local PIN containing exactly 8 ASCII digits; touchscreen keypad in the Wails Preview; Argon2id verifier (64 MiB, 3 iterations, 2 lanes, 32-byte output, unique 16-byte salt); 5-minute inactivity timeout; persisted failure backoff of 2s, 5s, 15s, 30s, 1m, 2m, 5m, 10m, then capped at 15m. No plaintext PIN, authenticated flag or reusable operator token is persisted.

The operator can change the PIN inside authenticated Settings by entering the current PIN. For a forgotten PIN, the trusted recovery path is the interactive CLI under the logged-in operator OS account: close PhotoBooth, then run `go run ./cmd/operator-setup --reset`. The CLI reads the new PIN without echo and can replace only an already-configured credential. It is not exposed as a Wails binding or Customer Mode action. This is OS-account recovery, not a vendor backdoor.

### 9. OS security boundary

- The app-level Operator Mode gate protects PhotoBooth controls and data exposed through its bindings. It does not replace OS account security.
- The initial deployment assumes the operator has already logged in to the OS. Application authentication does not prevent a person with physical keyboard access from using OS-level shortcuts, switching processes or invoking secure-attention functions.
- OS kiosk lockdown/Assigned Access/Shell Launcher remains a separate deployment decision. Do not claim this ADR disables system shortcuts or secures the whole desktop.
- Store credentials and user data under the logged-in OS user's protected profile. Never run the graphical application as root or LocalSystem for convenience.

## Acceptance criteria

1. Fresh launch always enters Customer Mode, including after a previous authenticated operator session and after a crash/restart.
2. Customer UI has no settings/admin/recovery/file-manager/exit path; all guest actions are touch-operable.
3. Direct calls to every operator-only Wails binding fail while in Customer Mode and during an authentication prompt.
4. Failed/cancelled authentication never grants operator access; error messages do not reveal whether an account exists.
5. Successful authentication opens Operator Mode and permits only the explicitly listed operator actions.
6. Lock/return/timeout/OS suspend-resume revokes privileged access; stale requests are rejected after revocation.
7. A customer capture/finalization/output operation cannot be interrupted by an operator-mode transition.
8. Recovery-required startup blocks new guest sessions while exposing only a generic customer message; the operator can authenticate and inspect recovery details.
9. Authenticated Exit closes the app cleanly after camera/preview/workers are quiesced and returns to the OS desktop. Unauthenticated Exit is rejected.
10. No credential plaintext, verifier material, filesystem path, printer identity or old-session metadata appears in customer-facing UI/logs.
11. The authorization boundary works offline and does not depend on SMB Remote administration or any network listener.

## Implementation order

1. Resolve credential type, inactivity timeout and failed-attempt policy.
2. Add a Go-owned mode/authorization controller and unit tests before exposing new UI.
3. Add centralized guards to all operator-only Wails bindings; create allowlisted customer DTOs.
4. Add startup Customer Mode, customer-only routing and a hidden operator-entry challenge flow.
5. Add authenticated Operator Mode, return/lock and authenticated Exit-to-desktop.
6. Add transition handling for camera preview, active sessions, result/failure and startup recovery.
7. Add direct-binding authorization tests, state-transition tests, UI automation and touch-only acceptance.

## Consequences

- The UI cannot be used as the sole authorization boundary; backend enforcement is mandatory.
- The current Wails prototype is not production-safe for booth guests until this ADR's authorization and kiosk criteria are implemented.
- The current Escape/F11 fullscreen escape remains Preview-only behavior and must be redesigned for production Customer Mode.
- The implementation can keep the existing Go core and Wails binding pattern, but all privileged entry points need a Go-side authorization gate.
- Physical printer acceptance, native Ubuntu direct-USB camera acceptance, the production installer, update trust and remote operator configuration remain independent gates.



## Implementation status (2026-11-21)

Initial implementation is complete in the shared Go core and the Wails v2 Preview:

- Go-owned Operator controller with strict/versioned Argon2id credential store, unique salt, atomic write, first-setup no-overwrite, persisted backoff, 5-minute inactivity timeout and in-memory revocable Operator Mode.
- First-time CLI setup and forgotten-PIN OS-account recovery. Recovery command is go run ./cmd/operator-setup --reset and must be run from an interactive terminal after PhotoBooth has been closed. PIN input is not echoed. No Wails or Customer Mode reset binding exists.
- Touch keypad Operator login, Settings PIN change, Return to booth, authenticated Exit application, auto-lock event and clearing of Operator-only frontend state.
- Backend guards on privileged Wails bindings, including legacy DemoSession diagnostic methods. Frontend hiding is not relied upon for authorization.
- Random active-session customer tokens and a custom safe JSON projection. Internal session IDs, local paths, raw errors and printer spool/job data are not sent to Customer Mode.
- Interrupted-session recovery is loaded only after Operator authentication. Operator-selected recovery explicitly transitions to a fresh Customer Mode session.
- Customer/Operator authentication, capture start, preview, cancel, session control and output retry are serialized with the camera lifecycle lock. F11/Escape are consumed in kiosk Customer Mode and only allow window-mode changes in Operator Mode.

**Automated verification on ShinPc (2026-11-21):** root go test ./..., Wails go test ./..., Wails go vet ./..., Wails wails build -clean (Windows/amd64), and Linux/amd64 cross-compile of internal/operatorauth tests plus cmd/operator-setup all passed.

**Not yet production-accepted:** manual touchscreen UI testing on a disposable station profile; real wrong-PIN/backoff/restart and five-minute timeout exercise; actual authenticated Exit-to-desktop; explicit Windows session lock/suspend notification; OS-level kiosk hardening (Assigned Access/Shell Launcher or Linux equivalent); Windows 8 GiB and native Ubuntu 24.04 4 GiB low-RAM acceptance; native Ubuntu 24.04 Wails runtime/package/camera/printer tests. The Linux cross-build above covers the auth package and provisioning CLI only. No PIN was provisioned in the user's real profile by these automated checks.


## Manual UI acceptance follow-up (2026-11-21)

A partial manual smoke was completed on ShinPc with disposable profiles and synthetic-only mode. Customer Mode startup, a hidden five-tap PB-logo gesture opening the Operator keypad, Cancel removing keypad/submit controls from the accessibility tree, and guest Start remaining visible all passed. The guest synthetic capture/render/local-save flow also passed. The preview remained disabled and did not open the physical camera. The disposable profiles, test credential and synthetic photos were deleted afterwards. Authentication/lock Wails bindings return no credential State, and configured/unconfigured failure cases share one generic authentication error.

Wrong/correct PIN UI, authenticated Settings, Change PIN, Return to booth, live five-minute UI lock and authenticated Exit remain pending manual acceptance. Physical camera/printer and OS-level kiosk acceptance are outside this smoke. See HANDOFF_2026-11-21-operator-mode.md.
