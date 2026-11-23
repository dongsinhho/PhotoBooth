# PhotoBooth Handoff - 2026-11-22 - Login startup and fullscreen Customer Mode

## Summary

Implemented the initial cross-platform per-user auto-start groundwork and changed the Wails app to default to fullscreen Customer Mode. Automated tests, isolated auto-start helper tests, and Windows UI smoke passed. This is not a production installer or native Ubuntu 24.04 acceptance.

## Repository

- Path: C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth
- Branch: main
- Last committed HEAD: 9ed9d28 feat(printer): integrate durable Windows print workflow
- No commit or push performed.
- No real HKCU Run key, Startup folder, or real Linux user autostart entry was created on ShinPc.

## User acceptance feedback

The user reported the Customer/Operator authentication UI had been manually tested on ShinPc and felt satisfactory. Record this as user-confirmed overall feedback only; no individual wrong-PIN, backoff, timeout or Exit case is claimed as a separate PASS without a case-specific report.

## Changes implemented

### Fullscreen Customer Mode by default

- Wails fullscreen now defaults on when no kiosk environment override or launch flag is provided.
- Explicit diagnostic opt-out: --windowed / PHOTOBOOTH_KIOSK=0.
- Explicit --kiosk remains accepted.
- Benchmark mode still suppresses fullscreen.
- run-photobooth.ps1 and release Start-PhotoBooth.ps1 now launch fullscreen by default; -Windowed is an explicit diagnostic option.
- F11/Escape remain Operator-only; Customer Mode cannot leave fullscreen via those app shortcuts.

### Windows per-user auto-start

- Added Set-PhotoBoothCurrentUserAutoStart and Remove-PhotoBoothCurrentUserAutoStart in tools/release/windows/Release.Common.ps1.
- Install-PhotoBoothWindowsPackage.ps1 registers the active immutable release's PhotoBooth.exe in the current user's HKCU Run key with --kiosk after activation.
- Upgrades refresh it; rollback points it at the restored release.
- Added -SkipAutoStart for isolated package install/upgrade/rollback tests.
- Added Uninstall-PhotoBoothWindowsPackage.ps1: removes only an auto-start value targeting this install root, removes the executable/release directory, and preserves %LOCALAPPDATA%\PhotoBooth user data.
- Added Test-PhotoBoothCurrentUserAutoStart.ps1; it uses a unique HKCU test subkey, not the real Run key.

### Linux per-user auto-start

- Added tools/release/linux/set-photobooth-autostart.sh with install/remove commands.
- Writes a marked XDG desktop entry under the invoking operator's XDG_CONFIG_HOME/autostart (or HOME/.config/autostart).
- Refuses root, validates an absolute executable path, escapes Desktop Entry values, and does not create a system service.
- Updates/removes only an entry marked X-PhotoBooth-Managed=true; refuses to overwrite/remove an unrelated entry.
- Added test-photobooth-autostart.sh and README.md.
- The .deb installer is not yet implemented; native Ubuntu 24.04 app startup/login integration remains pending.

### UI smoke

- Updated ui-kiosk-mode-smoke.ps1: Customer startup fullscreen, ESC/F11 blocked in Customer Mode, process restart returns to Customer fullscreen.
- Smoke clears PHOTOBOOTH_KIOSK and sends no launch args to prove fullscreen is the actual default.
- It disables camera preview, uses a disposable TEMP profile, and restores environment variables; no physical camera/capture.
- Installed-Package-Kiosk-Smoke uses SkipAutoStart for temporary package install, upgrade and rollback so it does not write TEMP paths into real HKCU Run.

## Verification performed on ShinPc

- Root go test ./...: PASS.
- Wails module go test ./...: PASS.
- Wails go vet ./...: PASS.
- Wails clean Windows build: PASS; PhotoBooth.exe generated at tools/ui-spike/wails-v2/build/bin/PhotoBooth.exe.
- Default-fullscreen UI smoke with PHOTOBOOTH_KIOSK unset and no launch args: PASS.
  - Customer startup fullscreen: PASS.
  - ESC blocked in Customer Mode: PASS.
  - F11 blocked in Customer Mode: PASS.
  - Close/restart returns to Customer fullscreen: PASS.
  - Temporary data cleaned: True.
  - Camera preview disabled during smoke.
- Windows current-user Run helper test: PASS, using a unique test registry subkey. Actual HKCU Run was untouched.
- Linux WSL Ubuntu XDG helper test: PASS, using a temporary HOME and an executable path containing spaces. Install, update-owned-entry, remove, and unowned-entry protection passed.
- PowerShell parser and Bash -n checks: PASS.

## Pending gates

1. No auto-start entry was enabled on the user's real ShinPc account. Actual Windows login/logoff observation on a disposable account remains pending.
2. The current Windows distribution path is an internal-alpha ZIP plus PowerShell install/rollback scripts, not the final signed per-user installer. Genuine publisher signing/trust and installer certification remain pending.
3. The NSIS Wails template still defaults to machine scope; no all-users Startup shortcut was added. Resolve per-user installer scope before using NSIS.
4. Ubuntu .deb packaging and integration of XDG auto-start into that package remain pending.
5. Native Ubuntu 24.04 login-session, fullscreen, Wails runtime, camera, printer and installer tests remain pending. The WSL test covers the helper only.
6. OS-level kiosk/desktop lockdown (Assigned Access/Shell Launcher or Linux desktop policy) is not implemented. App-level ESC/F11 restrictions do not block OS shortcuts.
7. Windows 8 GiB and Ubuntu 24.04 4 GiB low-RAM benchmarks remain pending.

## Commands

Windows default fullscreen build/launch:
    .\run-photobooth.ps1

Windows windowed diagnostic:
    .\run-photobooth.ps1 -Windowed

Linux install (logged-in operator account, never sudo):
    ./tools/release/linux/set-photobooth-autostart.sh install /absolute/path/to/PhotoBooth

Linux remove:
    ./tools/release/linux/set-photobooth-autostart.sh remove

No commit or push was made.


## Follow-up checkpoint - 2026-11-22 - Windows auto-start lifecycle

### Additional implementation and verification

- Restricted the Windows auto-start set/remove helper to registry paths under HKCU:\Software; attempts to target HKLM are rejected.
- Added Test-PhotoBoothAutoStartLifecycle.ps1. It builds two unsigned package versions into a disposable TEMP directory and uses a unique HKCU subkey under Software\PhotoBooth\Tests.
- Full lifecycle smoke passed:
  1. Fresh v1 install registered v1 PhotoBooth.exe --kiosk.
  2. Upgrade to v2 moved the registry value to v2.
  3. Rollback moved it back to v1.
  4. Uninstall removed the owned registry value and release directory.
  5. The separate %LOCALAPPDATA%\PhotoBooth user-data root had the same existence state before/after.
- TEMP install root and test registry subkey were both removed. Test output: PHOTOBOOTH_WINDOWS_AUTOSTART_LIFECYCLE=PASS.
- Test-PhotoBoothCurrentUserAutoStart.ps1 passed normal register/remove and rejection of non-HKCU set/remove paths.
- No actual HKCU Run value was created for the primary ShinPc account.

### Prepared actual logon acceptance kit

- Added Test-PhotoBoothLogonStartup.ps1: read-only check for HKCU Run command, exact executable running in the current interactive session with --kiosk, Customer UI visibility, no public Operator entry and fullscreen primary-display bounds.
- Added Prepare-PhotoBoothLogonAcceptanceKit.ps1 and WINDOWS_LOGON_ACCEPTANCE.md.
- Built an unsigned internal Preview acceptance kit at:
  C:\Users\Public\Documents\PhotoBoothLogonAcceptance-9fd43ab1512e462a93f77b87a51824ee
- Kit files include the versioned package ZIP/checksum, Install/Uninstall scripts, Release.Common.ps1, Test-PhotoBoothLogonStartup.ps1 and README.txt.
- This kit has not been installed or run under another Windows account. The OS test account has not been created. Primary user HKCU Run remains untouched.

### Linux packaging environment check

- WSL distro is Ubuntu 26.04.1 LTS (resolute), amd64, glibc 2.43.
- GTK3 3.24.52, WebKit2GTK 4.1 2.52.6 and dpkg-deb are present; Wails CLI was not found in PATH; WSL Go is 1.24.8.
- This is not the selected Ubuntu 24.04 LTS target. No .deb was built. Do not treat WSL 26.04.1 or its system libraries as Ubuntu 24.04 packaging/runtime acceptance.

### Current next action

Use the acceptance kit from a temporary Standard Windows account:
1. Create a local Standard user (not Administrator) in Windows Settings.
2. Sign into it and run the install command in the kit README with -SkipMachineProvisioning -AllowUnsignedPreview.
3. Sign out and sign back into that account without manually starting PhotoBooth.
4. Run Test-PhotoBoothLogonStartup.ps1 -AllowUnsignedPreview and record the report.
5. Uninstall from that same test account and remove the temporary OS account.

After this, continue the Ubuntu 24.04 packaging path in a matching 24.04 build environment; native Ubuntu login/runtime acceptance remains distinct.

No commit or push was performed.

## Follow-up checkpoint - 2026-11-22 - Administrator account choice and memory baseline

### Windows account choice

The user explicitly selected operation under a Windows Administrator account and declined a separate Standard-account test as unnecessary. This supersedes the earlier temporary Standard-user logon-acceptance instructions in this handoff.

The current login SHINPC\sinhv is a member of the local Administrators group. No new account was created, no sign-out/sign-in test was performed, and the real HKCU Run key was not changed. Account membership does not mean PhotoBooth must request UAC elevation; the app is expected to run in that account's interactive session. WINDOWS_LOGON_ACCEPTANCE.md and the acceptance-kit generator were updated to remove the Standard-account requirement. The unused kit previously generated under Public Documents was deleted. Actual logon observation remains pending.

### Wails preview-only 10-minute baseline

- Added -PreviewOnly to ui-preview-kiosk-soak.ps1, allowing resource telemetry without trying to enter protected Settings or use old diagnostic F11/ESC UI text.
- Updated fullscreen startup verification to use actual window bounds, not the obsolete visible kiosk hint.
- The 10.08-minute run completed 172 resource samples on the B525 with no guest session/capture or printer use.
- Preview mean/min: 4.76 / 4.71 FPS. CPU tree mean/peak: 12.92% / 24%.
- Process-tree private bytes first/last/peak: 327.19 / 678.24 / 900.18 MiB. WebView2 private peak: 873.35 MiB. PhotoBooth.exe private peak: 30.41 MiB.
- Host available RAM minimum: 5,706.58 MiB (36.85%); host commit peak: 74.28%. Paging counters are system-wide and not attributable to PhotoBooth.
- Raw event sequence: 2,903 published / 2,902 ACKed / one final frame in flight at process shutdown. All earlier frames ACKed and there were no duplicate, orphan, invalid or internal-gap events.
- The original report marked FAIL because the final in-flight frame had no terminal record. Updated audit now permits exactly one unmatched final publish only when it is the final event and the final preview state is ready with no ACK/decode failures. Internal gaps still fail.
- A 1.02-minute regression under the new audit passed 23 samples and 300/300 published/ACKed frames. PowerShell AST parse passed.
- TEMP profile cleanup: PASS. No production user data or real auto-start settings were changed.

This is a baseline on the existing 16 GiB ShinPc only. It does not certify the Windows 8 GiB target, approve Wails for low-RAM hardware, or validate Ubuntu 24.04 / 4 GiB.

### Remaining next gates

1. Obtain an actual Windows 11 x64 machine with 8 GiB RAM for target-class UI, camera and memory acceptance; do not emulate the target by changing pagefile or manufacturing system pressure.
2. Validate native Ubuntu 24.04 LTS / 4 GiB and complete its UI, login, camera, printer and .deb gates.
3. Actual Windows sign-out/sign-in auto-start observation under the selected Administrator-group account remains pending. The separate Standard-account test is removed from scope.

No commit or push was performed.
## Final checkpoint and validation before commit - 2026-11-22

The combined Customer/Operator authentication and per-user startup work is ready to commit. The Wails module remains on its established v2.17.0 dependency; a transient downgrade to v2.12.0 was reverted. Root go.mod was tidied so x/crypto and x/term are direct dependencies of the packages that import them.

Final validation:
- Root Go tests (go test -count=1 ./...): PASS.
- Root go vet ./...: PASS.
- Root Windows go build ./...: PASS.
- Wails Go tests (go test -count=1 ./... in tools/ui-spike/wails-v2): PASS.
- Wails go vet ./...: PASS.
- Frontend npm run build: PASS.
- Wails v2.17.0 wails build -clean -platform windows/amd64: PASS.
- run-photobooth.ps1 -NoLaunch Windows build: PASS.
- Wails Customer Mode kiosk UI smoke: PASS (default fullscreen, ESC/F11 blocked, restart returns to fullscreen Customer Mode; disposable profile cleaned).
- Windows auto-start helper tests: PASS (registration/removal and non-HKCU write/removal rejection).
- Windows install -> upgrade -> rollback -> uninstall auto-start lifecycle: PASS (unique HKCU test subkey/temp package root; temp and registry fixtures cleaned; user-data root preserved).
- Linux XDG auto-start test in WSL: PASS (disposable HOME; register/update/remove and protection of unowned entries).
- PowerShell AST parsing of changed release/UI scripts: PASS.
- Bash syntax checks: PASS.
- git diff --check: PASS (only Git line-ending normalization warnings).

No PhotoBooth process remains running. The real HKCU Run value remains absent. No physical camera capture, printer job, installer activation, account sign-out/sign-in or OS-level kiosk policy test was performed as part of this final validation.

The 10.08-minute Wails preview-only 16 GiB baseline and the 1.02-minute 300/300 ACK regression are recorded in UI_SPIKE_RESULTS.md. This does not certify Windows 8 GiB or Ubuntu 24.04 / 4 GiB.

The next gates remain an actual Windows 8 GiB target machine, native Ubuntu 24.04 / 4 GiB acceptance, selected-account Windows sign-out/sign-in auto-start observation, genuine production signing/publisher trust, OS-level kiosk policy and physical printer/camera acceptance where noted above. No remote push is included in this checkpoint.
## Post-commit checkpoint verification - 2026-11-23

Repository state was re-opened after the 2026-11-22 implementation checkpoint.

- Feature commit: ec6b1fbe8a2dcf722c0f75bda310174dbb03e5a1 — feat(kiosk): add authenticated operator mode and login startup.
- Parent: 9ed9d28cdb3533ed66457a65ea47bfad2e975a57.
- The feature commit contains the Customer/Operator authentication implementation, Windows fullscreen/per-user startup and lifecycle scripts, Linux XDG auto-start helper, ADR 0034/0035, updated product docs and the 10-minute Wails preview-only memory baseline.
- At verification time, main and the local origin/main tracking ref both pointed at ec6b1fb; the working tree was clean.
- The feature commit's pre-commit validation is recorded above. No new application changes were necessary for this verification.
- The current user explicitly chose a Windows account in the local Administrators group. No separate Standard account is part of the acceptance plan.
- The actual primary-user HKCU Run value remains absent; no Windows sign-out/sign-in auto-start observation was performed.

Remaining gates: actual Windows 11 8 GiB target-class acceptance; native Ubuntu 24.04 LTS / 4 GiB acceptance; sign-out/sign-in auto-start observation under the selected Administrator-group account; genuine production signing/publisher trust; OS-level kiosk policy; and physical printer/camera acceptance where listed in the product plan.

This entry records the verified commit and repository state. It does not claim that the remaining hardware or OS acceptance gates have passed.