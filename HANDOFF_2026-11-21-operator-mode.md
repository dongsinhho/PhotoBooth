# PhotoBooth Handoff - 2026-11-21 - Operator Authentication implementation

## Checkpoint summary

Initial Customer Mode / Operator Mode authentication implementation is complete in the shared Go auth package and Wails v2 Preview. Automated tests and builds pass on ShinPc. This is an uncommitted implementation checkpoint, not production certification.

## Repository

- Path: C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth
- Branch: main
- Last known committed HEAD before this work: 9ed9d28 feat(printer): integrate durable Windows print workflow
- No commit or push was performed.
- User's real station profile was not provisioned with a test/default PIN.

## Decisions now implemented

- One station-level 8-digit ASCII PIN.
- Argon2id verifier, unique 16-byte salt, 64 MiB memory, 3 iterations, 2 lanes, 32-byte hash.
- Persistent failed-attempt/backoff: 2s, 5s, 15s, 30s, 1m, 2m, 5m, 10m, then 15m cap.
- 5-minute Operator inactivity timeout.
- Interactive touchscreen keypad; PIN not echoed or stored in plaintext.
- Change PIN inside authenticated Settings requires current PIN.
- Forgotten-PIN recovery requires control of the logged-in OS account: close PhotoBooth, run go run ./cmd/operator-setup --reset in an interactive terminal, and enter/confirm the new PIN. CLI reset is not a Wails binding.
- Every new application process starts in Customer Mode.
- Wails Go bindings enforce Operator authorization for settings, frame packages, retention, recovery, printer maintenance, SMB admin, kiosk/window mode, preview stop, application exit and legacy DemoSession diagnostics.
- Operator entry is a hidden five-tap gesture on the PB brand mark; there is no public Operator navigation button on the guest screen. The gesture only opens the PIN challenge and grants no privilege by itself.
- The WebView does not receive credential-existence/configuration status. AuthenticateOperator and LockOperatorMode return no auth State; unconfigured, wrong-PIN, corrupt-store and backoff failures are normalized to one generic authentication error.
- Interrupted-session recovery data is loaded only after Operator authentication and shown in Operator Settings.
- Capture, preview, output retry and Customer/Operator transitions are serialized with the camera lifecycle lock. Operator authentication refuses while guest capture/finalization/output is active.
- F11/Escape are consumed in fullscreen Customer Mode; only Operator Mode can request window-mode changes. This is app-level behavior, not whole-OS kiosk lockdown.

## Main code changes

- internal/operatorauth/: controller, strict credential store, atomic platform-specific writes/permissions, unit tests.
- cmd/operator-setup/main.go: first setup and --reset forgotten-PIN CLI recovery.
- tools/ui-spike/wails-v2/operator_auth.go: Wails mode/auth/exit bindings.
- tools/ui-spike/wails-v2/customer_session.go: customer-safe JSON projection and random token validation.
- tools/ui-spike/wails-v2/operator_auth_test.go, customer_session_test.go: authorization, mode transition and data-projection regression tests.
- Wails app.go, player.go, preview.go, recovery.go, settings/package/retention/SMB/kiosk files: backend guards and session lifecycle transition changes.
- Frontend frontend/src/main.js and frontend/src/app.css: keypad, login/lock, Change PIN, authenticated Exit, Operator-only recovery/settings loading, customer-safe session fields and shortcut behavior.
- Dependencies pinned for root Go 1.24.8 compatibility: golang.org/x/crypto v0.41.0, golang.org/x/term v0.35.0, golang.org/x/sys v0.36.0.

## Verification on ShinPc (2026-11-21)

- Root go test ./...: PASS.
- Wails module go test ./...: PASS.
- Wails module go vet ./...: PASS.
- Wails wails build -clean: PASS, Windows/amd64; binding generation, Vite frontend compile and application compile passed.
- Linux/amd64 cross-compile of internal/operatorauth tests and cmd/operator-setup: PASS.
- Partial manual UI acceptance was performed on ShinPc using isolated %TEMP% profiles and PHOTOBOOTH_SYNTHETIC_ONLY=1. UI Automation confirmed fresh Customer Mode, the guest idle screen exposes only guest actions (no public Operator button), five taps on the PB mark open the 8-digit PIN challenge, and Cancel removes the keypad/submit controls from the accessibility tree while guest Start remains visible. The camera preview displayed the synthetic-only disabled state and did not open the physical webcam.
- The guest "Try without a webcam" workflow completed 3 synthetic captures, rendered a 1200x1800 JPEG and published a local photo. UI showed COMPLETED / "Your photo has been saved on this booth" / "Your photo strip is ready". Captures, rendered.jpg, photo.jpg and manifests were verified under the disposable profile. The app closed gracefully and the entire temporary profile was deleted.
- PIN wrong/correct entry, successful Operator Settings load, Change PIN, authenticated Exit, and live 5-minute UI timeout were not completed manually. Backend tests cover credential/backoff/timeout behavior; do not call these UI checks PASS. No real camera or printer acceptance was performed.
- Added tools/ui-spike/wails-v2/run-operator-auth-acceptance.ps1: it creates a random disposable %TEMP% profile, invokes the secure interactive first-time setup CLI so the operator enters a temporary PIN locally, launches the synthetic-only Wails app, supports restarting against the same profile, and removes the profile/credential/photos on exit. PowerShell parser validation passed; the script has not yet been run interactively by the user.
- Latest Wails generated signatures confirm AuthenticateOperator and LockOperatorMode both return Promise<void>; no credential State is returned to the WebView. Root tests, Wails tests, Wails vet and Wails clean Windows build passed again after this change.

## Explicit limitations / next acceptance gates

1. Run powershell -ExecutionPolicy Bypass -File .\tools\ui-spike\wails-v2\run-operator-auth-acceptance.ps1 from the repo root. The script provisions a fresh disposable test PIN through the interactive CLI, launches synthetic-only mode and cleans the profile on exit. Manually verify wrong PIN, backoff persistence after restart, correct PIN -> Settings, Change PIN, Return to booth, five-minute lock and authenticated Exit-to-desktop. Startup Customer Mode, hidden five-tap gesture, Cancel hiding the keypad, and synthetic guest capture/render/local save already passed.
2. Verify Customer Mode cannot reveal Settings/recovery/local file paths through keyboard shortcuts or direct Wails binding attempts. Alt+Tab, Windows key, secure attention and other OS-level shortcuts are not blocked by this app-level implementation.
3. Implement/test actual Windows session lock/suspend notifications; currently inactivity timer and backend expiry are implemented, but no explicit WTS session lock/suspend listener has been accepted.
4. Evaluate Windows Assigned Access/Shell Launcher or an equivalent supported OS kiosk configuration separately; this app does not lock down the desktop.
5. Run low-RAM tests on a real Windows 8 GiB target and native Ubuntu 24.04 LTS 4 GiB target.
6. Validate native Ubuntu 24.04 Wails runtime, installer, camera and printer separately. The Linux cross-build above covers auth package/CLI only.
7. Physical printer/media acceptance, native Ubuntu direct-USB camera, production publisher certificate/trust and approved release installer remain independent pending gates.

## Current operational instructions

First-time setup (run as the logged-in operator OS account, interactive terminal):
go run ./cmd/operator-setup

Forgotten PIN (close PhotoBooth first; run under the operator OS account):
go run ./cmd/operator-setup --reset

Optional data root (must match the app's configured data directory):
$env:PHOTOBOOTH_DATA_DIR = 'C:\path\to\PhotoBooth-data'

Do not provision the user's real booth PIN from this checkpoint; owner should perform first-time setup locally.
