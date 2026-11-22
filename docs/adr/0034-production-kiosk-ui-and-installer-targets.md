# ADR 0034: Production kiosk UI, operator boundary, and installer targets

**Status:** Product and deployment direction accepted. Production UI framework approval and release certification remain gated on target-hardware validation.
**Date:** 2026-11-21
**Decision owner:** Product owner

## Context

The repository has a runnable Wails v2.17 Windows Preview candidate and a Linux Wails build/UI smoke from WSL2 Ubuntu 26.04.1. The Go core remains isolated on Go 1.24.8; the Wails module uses its own Go 1.25.0 toolchain. Existing UI work demonstrates feasibility, not a production guest/operator interface.

The Wails Windows Preview 10-minute soak on the 16 GiB ShinPc host recorded a process-tree private-byte peak of approximately 1.29 GiB. This is one host/run and is not a low-RAM certification. No actual 8 GiB Windows or 4 GiB Ubuntu 24.04 production target has been accepted. Native Ubuntu direct-USB camera and Linux printer/runtime acceptance also remain pending.

The product owner has selected simultaneous Windows and Linux product targets, a touchscreen-first guest workflow, and installer-based deployment. The product must remain usable for free/local sessions without Internet.

## Decisions

### 1. Initial operating-system targets

- **Windows:** Windows 11 x64; 8 GiB RAM is the initial minimum-hardware target.
- **Linux:** Ubuntu 24.04 LTS x86_64; 4 GiB RAM is the initial minimum-hardware target.
- Windows and Ubuntu are both first-class production targets. A Windows-only release does not close the cross-platform production gate unless the product owner explicitly re-scopes it.
- Other Linux distributions and macOS are not initial release targets. Keep OS-specific behavior behind adapters so macOS remains a possible future target without promising support.
- The existing Ubuntu 26.04.1 WSL2 evaluation remains useful development evidence only. It is not native Ubuntu 24.04 acceptance.

These RAM figures are product targets, not yet verified minimum specifications. Acceptance requires real machines at or near each target, not extrapolation from ShinPc.

### 2. UI and core architecture

- Keep the Go domain/workflow core shared: session lifecycle, capture orchestration, rendering, local storage, output coordination, configuration/policy and recovery contracts.
- OS-dependent camera, printer, startup, packaging, window lifecycle and permission handling stay behind platform adapters.
- Separate UI implementations per OS are explicitly acceptable when they materially improve RAM use, responsiveness or native integration.
- Keep the existing Wails v2.17 shell as the incumbent prototype and first benchmark candidate; do not approve it unconditionally for both OSes and do not rewrite it prematurely.
- Benchmark Wails on the actual Windows 8 GiB and Ubuntu 24.04 4 GiB targets. If a platform cannot meet the agreed memory/headroom and interaction gates, evaluate a native UI for that OS while retaining the shared Go core.

For Wails v2, the Linux target uses GTK3 + WebKit2GTK ABI 4.1 and the repository's existing webkit2_41 build path. The v2.17 documentation lists Ubuntu 22.04+ in the ABI 4.1 family. This is dependency/build guidance, not evidence that this application's complete workflow is production-ready on Ubuntu 24.04.

### 3. Customer Mode and Operator Mode

- The application has one product identity and two permission-separated modes.
- **Customer Mode** is the default mode on every normal application launch. It is fullscreen, touch-first and exposes only the guest workflow. Guests cannot edit station settings, camera/printer configuration, frames, storage, payment policy or administration controls.
- The complete guest workflow must be operable by touchscreen without requiring a physical keyboard or mouse.
- **Operator Mode** is protected by operator authentication. It exposes station settings, diagnostics and maintenance functions. Settings remain operator-controlled; guest input cannot change them.
- Operator authentication must not store a plaintext secret. Credential storage, reset/recovery, rate limiting and lockout behavior require a dedicated security implementation decision before production.
- Returning from Operator Mode to the booth explicitly returns to Customer Mode. If the operator chooses Exit application, the process closes and the OS desktop is shown.
- A subsequent application launch always returns to Customer Mode; it must not restore an unlocked Operator Mode from a previous process.

### 4. Exit and kiosk boundary

- The operator must authenticate before leaving the kiosk application for the OS desktop.
- Customer Mode must not expose an ordinary Exit, Settings or desktop-navigation action.
- The current Preview Escape-to-windowed behavior is prototype behavior, not the final guest-facing exit contract. Production Customer Mode must not offer a guest-accessible keyboard escape path.
- The application is launched inside an already logged-in interactive operator OS account. OS shell replacement/Assigned Access is not a requirement for the first implementation; do not claim OS-level kiosk lockdown.
- After an authenticated exit, the booth is no longer guest-ready until the operator launches/returns the application. This is a maintenance boundary, not a background Customer Mode session.

### 5. Installation, startup and updates

- End-user distribution is **installer-only**. Do not publish a portable ZIP as an alternative end-user installation path.
- Windows should produce a signed per-user installer. The existing versioned release payload, integrity validation, side-by-side activation and rollback can remain the underlying lifecycle; the current internal-alpha ZIP is not itself the final user-facing installer. The exact installer bootstrapper technology remains an implementation choice and must preserve the existing integrity/rollback guarantees.
- Ubuntu 24.04 should initially use a Debian package (.deb) with declared GTK/WebKit runtime dependencies and a desktop launcher. Do not treat the existing WSL build as a package/install test.
- Installer/uninstaller must keep executable files separate from operator settings, frames, sessions and guest photos. Uninstall must not silently delete user data.
- The operator logs into the OS first. The installer configures application auto-start for that user's interactive session. On launch, PhotoBooth enters fullscreen Customer Mode. Do not run the GUI as root or as a pre-login system service.
- The app may check for updates when network access is available and notify the operator. Installation is operator-approved; no silent/background update is allowed in this scope. Offline update checking must never block local booth operation.
- Updates must verify publisher identity and package integrity. The genuine production publisher certificate, timestamp authority and trust pin remain pending; a disposable test certificate is not sufficient.

### 6. Offline operation and optional QR payment

- Free/disabled payment + local capture/render/save remains the default and must work without Internet.
- The product direction supports both:
  1. **Provider-backed VietQR / verified QR payment:** generate a session/order-bound QR and grant the paid session only after a trusted provider/bank transaction signal has been authenticated and reconciled. This path requires Internet/service connectivity where the chosen provider API and callback architecture require it.
  2. **Static QR + local operator confirmation:** display a merchant-configured static QR without requiring the booth itself to contact a payment API. An authenticated operator may confirm the matching session from the local network after checking the payment using an independent source (for example, the operator's banking app). This is a manual operator decision, not automatic bank verification.
- A QR image, a guest screenshot, a redirect, or a guest's claim is never proof of payment.
- Manual confirmation must be explicitly labelled as operator-confirmed, bound to the booth/session/expected amount, time-limited, and audit logged. It must not be represented as a verified VietQR callback.
- Provider-backed confirmation must validate the provider's authenticated callback/signal, transaction identity, amount and session reference; duplicate/late/pending/unknown callbacks must be idempotently reconciled.
- Payment remains separate from camera adapters and capture. Existing disabled/free stays the default. New payment modes remain unsupported until a separate payment ADR, provider selection, secrets handling and end-to-end acceptance are completed.
- The local operator confirmation surface is a separate, narrowly scoped future capability. It does not reopen or expand the SMB-based Remote administration feature, which remains accepted and paused under ADR 0031.

### 7. Remote operator configuration

Remote configuration is a desired future use case, but it is not part of the production UI/deployment implementation slice in this ADR. Define it as a separately authenticated, session/config-scoped capability with explicit network binding and audit behavior. Do not reinterpret the paused SMB Remote administration feature as a remote Settings API or add new SMB scope.

## Implementation and acceptance sequence

1. Specify Customer/Operator mode state transitions, credential boundary and guest-visible navigation.
2. Implement touch-complete Customer Mode, authenticated Operator Mode, protected settings and authenticated Exit-to-desktop.
3. Implement interactive-user auto-start and fullscreen startup for Windows and Ubuntu; verify app restart always returns to Customer Mode.
4. Benchmark the Wails candidate on Windows 11 / 8 GiB and native Ubuntu 24.04 / 4 GiB. Measure whole process tree and host headroom during idle preview, Settings, capture/render/output and a sustained kiosk run. Set numeric process/headroom budgets from the measured target baseline before declaring pass.
5. If the Wails candidate fails either platform's measured target, evaluate an OS-specific native UI for that platform; do not fork the Go domain/workflow core.
6. Build and test a signed Windows installer and Ubuntu .deb: clean install, launch-after-login, offline operation, upgrade, rollback where supported, uninstall/data preservation and recovery from interrupted installation.
7. Add update notification with operator-approved installation after signed release metadata and the production publisher trust root exist.
8. Keep physical printer acceptance, native Ubuntu camera acceptance, payment integration and remote operator configuration as separate gates.

## Current status and explicit non-claims

- Product requirements in this ADR are accepted; implementation is not complete merely because this ADR exists.
- Wails is still a benchmark candidate, not unconditionally approved as the production UI.
- Windows 8 GiB and Ubuntu 24.04 4 GiB are unvalidated hardware targets.
- WSL2 Ubuntu 26.04.1 GUI/build evidence is not native Ubuntu 24.04 kiosk, camera, printer or installer acceptance.
- Windows internal-alpha ZIP tooling and test-certificate signing smokes are not a public production installer or production publisher trust.
- No VietQR integration, static-QR confirmation flow, update service, Ubuntu installer or authenticated remote Settings capability is implemented by this ADR.

## References

- Wails v2.17 Linux distro/runtime dependency guide: https://v2.wails.io/docs/guides/linux-distro-support/
- Wails v2 NSIS installer guide (framework installer capability; project-specific installer still requires implementation): https://v2.wails.io/docs/guides/windows-installer/
- Ubuntu 24.04 release notes and support lifecycle: https://documentation.ubuntu.com/release-notes/24.04/
- VietQR callback/API integration and transaction-sync workflow: https://api-v2.vietqr.vn/api-vietqr-callback and https://api.vietqr.vn/vi/api-vietqr-callback/api-transaction-sync



## Implementation status update (2026-11-22)

Initial ADR 0034 step-3 startup behavior is implemented as follows:

- Normal Wails launch defaults to fullscreen Customer Mode with no environment variable or command-line flag. --windowed / PHOTOBOOTH_KIOSK=0 are explicit diagnostic opt-outs; --kiosk is accepted. Benchmark mode still suppresses fullscreen.
- Windows launcher scripts default to fullscreen. The versioned Windows PowerShell install flow registers the active immutable release's PhotoBooth.exe --kiosk in the current user's HKCU Run key after activation. Upgrades refresh it; rollback points it at the restored release; uninstall removes only a value owned by that install root and preserves %LOCALAPPDATA%\PhotoBooth data.
- Package install/rollback smoke uses SkipAutoStart for TEMP packages so tests do not write temporary paths to the real user's HKCU Run key.
- Linux per-user XDG groundwork is under tools/release/linux. It refuses root, writes a marked desktop entry in the invoking operator's config home, validates/quotes the executable path and refuses to overwrite/remove unowned entries. The future .deb must invoke it in the real operator user's session, not from a root-only postinst script.
- No production startup entry was enabled on the user's real ShinPc account.

Automated acceptance on ShinPc: root/Wails tests, Wails vet, Wails clean Windows build, UI smoke with kiosk env unset and no launch flags (fullscreen Customer startup, ESC/F11 blocked, restart returns Customer fullscreen), isolated Windows Run-key helper test, Linux XDG helper test in disposable WSL HOME (including path with spaces/update/remove/unowned-entry protection), PowerShell parser and Bash syntax checks all passed.

Remaining: real Windows logon/startup observation on a disposable account; native Ubuntu 24.04 login/session test; .deb integration; genuine signed per-user Windows installer; Windows 8 GiB/Ubuntu 4 GiB benchmarks; OS-level kiosk/desktop policy. The current Windows ZIP/PowerShell install path remains internal alpha, not the final signed per-user installer.


## Windows auto-start lifecycle verification update (2026-11-22)

A disposable package lifecycle smoke now passes fresh install, upgrade, rollback and uninstall with auto-start registration enabled against a unique HKCU test subkey. It verifies the stored Run command follows each active immutable release, then verifies uninstall removes the owned value and executable install root without changing the separate %LOCALAPPDATA%\PhotoBooth user-data root. The auto-start helper rejects registry paths outside HKCU:\Software.

A standard Windows account logon acceptance kit is prepared in Public Documents but has not been executed. Actual sign-out/sign-in execution remains pending. The available WSL distro is Ubuntu 26.04.1 LTS, not the Ubuntu 24.04 production target; no .deb build/acceptance is claimed.

## 2026-11-22 - Windows operator account selection

The user selected operation under a Windows account that belongs to the local Administrators group. The current ShinPc login SHINPC\sinhv meets this account-membership requirement. A separate Standard-account acceptance test is not required and is removed from scope.

The app remains a per-user interactive-session application: auto-start is registered in that operator account's HKCU Run key. This decision does not require the built-in local account named Administrator, a Windows service, or forced UAC elevation of PhotoBooth. Actual sign-out/sign-in observation under the selected account remains a separate acceptance check; no real HKCU Run entry has been enabled by this decision.