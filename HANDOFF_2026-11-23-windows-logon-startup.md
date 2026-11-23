# PhotoBooth Handoff - 2026-11-23 - Selected-account Windows logon startup

## Repository state at start

- Repository: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- Branch: `main`
- Starting HEAD: `727a0b4 docs: finalize November 22 startup checkpoint`
- Starting working tree: clean; `main` was aligned with `origin/main`.
- Selected Windows operator account: `SHINPC\sinhv` (member of local Administrators group). The user declined a separate Standard-account test.

## Work completed on 2026-11-23

1. Re-read the product plan, roadmap, Windows release README and logon acceptance guide. The next gate remained a real sign-out/sign-in auto-start observation under the selected account.
2. Corrected `tools/release/windows/README.md` so it no longer instructs a separate Standard-account test and instead documents the selected Administrator-group account.
3. Built the Wails Windows Preview and a fresh unsigned internal logon acceptance kit using `Prepare-PhotoBoothLogonAcceptanceKit.ps1`.
4. Installed the package under the selected account using `-SkipMachineProvisioning -AllowUnsignedPreview`. No machine-wide SMB provisioning was requested.
5. Verified the real current-user registry value:
   - Path: `HKCU:\Software\Microsoft\Windows\CurrentVersion\Run`
   - Name: `PhotoBooth`
   - Value: `"C:\Users\sinhv\AppData\Local\Programs\PhotoBooth\releases\0.0.0-logon-smoke.20261123205418\PhotoBooth.exe" --kiosk`
6. Verified the installed executable and release files exist. No PhotoBooth process was running after installation.
7. Read-only post-install observation: `LanmanServer` was `Running/Automatic`. The installer used `-SkipMachineProvisioning`; no service, firewall, registry or share setting related to SMB was changed during this task.
8. Updated the Windows acceptance guide and roadmap to record the real installed state and the remaining logon test.

## Acceptance-kit details

- Kit: `C:\Users\Public\Documents\PhotoBoothLogonAcceptance-078355aa51364814a682b2a97e1f66c8`
- Package: `PhotoBooth-internal-alpha-0.0.0-logon-smoke.20261123205418-windows-amd64.zip`
- Package SHA-256: `b9d0aff9ea2ea546938ce1a5c4164d913c5bb663c554cfe34cee7777718db737`
- Executable SHA-256: `b79816283886929a522f4be81db28db2110aec6c953577815a3d6d181b7a642e`
- Package generation and install both completed successfully.
- This is an unsigned internal Preview, not the signed production installer.

## Current status and next action

**Status:** Pending sign-out/sign-in acceptance

The current-user Run entry is enabled, but true Windows logon startup is NOT yet PASS. The remaining test requires an actual OS sign-out/sign-in:

1. Save/close other work and sign out of Windows from the current `SHINPC\sinhv` account. Do not just lock the screen.
2. Sign back into the same account. Do not manually start PhotoBooth.
3. From the acceptance-kit folder, run:

   `powershell -NoProfile -ExecutionPolicy Bypass -File .\Test-PhotoBoothLogonStartup.ps1 -AllowUnsignedPreview`

4. Record the report path and all PASS checks. It must confirm Run-key match, process in the interactive session, Customer UI visible, no public Operator entry and fullscreen bounds.
5. Close PhotoBooth and uninstall from the same account using the kit's `Uninstall-PhotoBoothWindowsPackage.ps1`. Confirm the uninstall prompt.
6. Verify the PhotoBooth-owned Run value is gone while `%LOCALAPPDATA%\PhotoBooth` user data is preserved; sign out/in again to confirm the app no longer starts. Then delete the acceptance-kit directory.

Do not claim acceptance until steps 2-4 pass. If the user is not ready to sign out, leave the installed package and Run value as-is and do not uninstall without an explicit instruction.

## Still separate / pending gates

- Signed production per-user Windows installer, publisher trust and installer certification.
- Native Ubuntu 24.04 LTS build/package/runtime and XDG auto-start integration.
- Windows 11 8 GiB and Ubuntu 24.04 4 GiB target-class acceptance (the 16 GiB ShinPc baseline is not a substitute).
- OS-level kiosk/desktop lockdown; app-level fullscreen and ESC/F11 restrictions are not shell lockdown.
- Physical printer/camera acceptance where still listed in the plan.

No commit or push has yet been performed for the 2026-11-23 documentation changes.


## Follow-up: acceptance passed and Preview removed

After the user reported all checks passed, the actual generated report was inspected: `%TEMP%\\photobooth-logon-acceptance-20261123-210410.json`, status `PASS`, user `SHINPC\\sinhv`, parent `explorer.exe`, interactive session 3. All five checks passed: HKCU Run target, interactive process, Customer UI, no public Operator entry, fullscreen bounds.

The auto-started PhotoBooth process was closed gracefully. `Uninstall-PhotoBoothWindowsPackage.ps1` completed with `CURRENT_USER_AUTO_START=REMOVED` and `PHOTOBOOTH_UNINSTALL=PASS`. Independent checks confirmed install root removed, HKCU Run value absent, no PhotoBooth process running, and `%LOCALAPPDATA%\\PhotoBooth` user data preserved (`frames`, `photos`, `sessions`, `store-config-v1.json`).

**Remaining immediate action:** sign out and sign back in once more to confirm the app does not auto-start after uninstall. Then delete the acceptance kit at `C:\\Users\\Public\\Documents\\PhotoBoothLogonAcceptance-078355aa51364814a682b2a97e1f66c8`. Do not remove the kit before this negative startup check.

Positive Windows logon acceptance is PASS. This is still only unsigned Preview acceptance, not signed production installer certification. Subsequent roadmap gates require real target hardware: Windows 11 8 GiB and Ubuntu 24.04 4 GiB benchmarks; native Ubuntu 24.04 build/UI/camera/printer and .deb; and genuine publisher certificate/TSA for signed Windows production packaging. Current WSL Ubuntu 26.04.1 is not a valid substitute for Ubuntu 24.04.
