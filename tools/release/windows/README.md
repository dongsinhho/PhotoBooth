# Windows per-user launch and auto-start

PhotoBooth's normal launch path is fullscreen Customer Mode. The Wails executable defaults to fullscreen even if PHOTOBOOTH_KIOSK is unset. The --windowed flag (or -Windowed in the repository launcher scripts) is reserved for explicit development/diagnostic use. A benchmark launch also suppresses fullscreen.

## Versioned Windows package flow

Install/upgrade a previously built versioned package:

    .\tools\release\windows\Install-PhotoBoothWindowsPackage.ps1 -PackagePath .\artifacts\windows\PhotoBooth-internal-alpha-0.1.0-windows-amd64.zip -AllowUnsignedPreview

The default install root is %LOCALAPPDATA%\Programs\PhotoBooth. After the active release pointer is switched, the installer writes a current-user Run value at:

    HKCU\Software\Microsoft\Windows\CurrentVersion\Run

The value points to the active immutable release's PhotoBooth.exe and passes --kiosk. Upgrades rewrite it to the newly active version. Rollback also changes it to the restored version.

Uninstall while PhotoBooth is closed:

    .\tools\release\windows\Uninstall-PhotoBoothWindowsPackage.ps1

Uninstall removes the PhotoBooth-owned auto-start value and the executable/release directory. It preserves %LOCALAPPDATA%\PhotoBooth operator settings, credentials, frames, sessions and guest photos.

The installer and rollback scripts expose -SkipAutoStart only for isolated tests/maintenance. The package smoke uses this switch so it never writes a TEMP install path into the real user's HKCU Run key.

## Development launcher

- Default: .\run-photobooth.ps1 — build and launch fullscreen Customer Mode.
- Explicit diagnostic: .\run-photobooth.ps1 -Windowed.
- .\run-photobooth.ps1 -NoLaunch builds only.

The current versioned package is still an internal-alpha ZIP plus PowerShell install/rollback scripts, not the final signed per-user Windows installer. Genuine publisher signing and installer certification remain separate gates.


## Selected-account Windows logon acceptance

The selected operator account is the existing SHINPC\sinhv account in the local Administrators group. Do not create a separate Standard account for this acceptance. Prepare a fresh internal acceptance kit under the same account:

    .\tools\release\windows\Prepare-PhotoBoothLogonAcceptanceKit.ps1

Follow tools/release/windows/WINDOWS_LOGON_ACCEPTANCE.md and the generated kit README. The documented install uses -SkipMachineProvisioning so it does not change machine-wide SMB policy, but it enables only the current user's HKCU Run entry. After installation, sign out and back into the same account without manually launching PhotoBooth, then run Test-PhotoBoothLogonStartup.ps1 -AllowUnsignedPreview. The final signed production installer remains a separate release gate.
