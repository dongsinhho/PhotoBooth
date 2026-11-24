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


## Experimental Wails NSIS per-user prototype (not a booth release)

The Wails shell now has a separate NSIS template smoke path. From `tools/ui-spike/wails-v2`, with Wails v2.17.0 and NSIS 3.13 available:

    wails build -clean -nsis -installscope user -o PhotoBooth -platform windows/amd64

The prototype output is `build/bin/PhotoBoothNSISPreview-amd64-installer.exe`. It is deliberately isolated from the versioned PowerShell package flow:

- Installs to the current user's `%LOCALAPPDATA%\Programs\PhotoBooth NSIS Preview` path.
- Uses the distinct `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\PhotoBoothNSISPreview` value; it does not reuse or replace the versioned package's `PhotoBooth` Run value.
- Refuses machine install scope and administrator execution level at NSIS compile time.
- Refuses to overwrite an existing Preview Run value without a matching ownership record; uninstall removes the value only if it still exactly targets this install root.
- Does not target `%LOCALAPPDATA%\PhotoBooth` operator data. The Wails WebView data directory cleanup is separate from the Go application's PhotoBooth data root.

**Important limitations:** this is only a direct-binary NSIS prototype to validate per-user scope and HKCU startup mechanics. It does not wrap the immutable versioned release ZIP, package-hash verification, side-by-side activation/rollback or first-install SMB default-off provisioning. It is unsigned, has not been installed or uninstalled on a user profile, and is not approved for booth deployment or redistribution. Keep using the existing PowerShell versioned package scripts for internal lifecycle testing until a bootstrapper is implemented around those scripts and the remaining release gates pass.


## Versioned per-user NSIS bootstrapper (internal Preview)

The end-user installer wrapper is `New-PhotoBoothWindowsInstaller.ps1` plus `PhotoBoothInstaller.nsi`. It embeds the exact versioned Windows release ZIP and adjacent SHA-256 sidecar, then invokes the existing `Install-PhotoBoothWindowsPackage.ps1` and `Uninstall-PhotoBoothWindowsPackage.ps1` lifecycle scripts. The underlying lifecycle remains responsible for package/manifest/binary verification, immutable version folders, active-pointer update, rollback metadata, current-user `HKCU` auto-start ownership and user-data separation.

Build an unsigned installer only for internal engineering evaluation:

    .\tools\release\windows\New-PhotoBoothWindowsInstaller.ps1 -Version 0.1.0-nsis-preview -PackagePath .\artifacts\windows\PhotoBooth-internal-alpha-0.1.0-nsis-preview-windows-amd64.zip -OutputDirectory $env:TEMP\PhotoBoothInstallerPreview -MakensisPath "$env:LOCALAPPDATA\Tools\NSIS-3.13\nsis-3.13\makensis.exe" -AllowUnsignedPreview

The NSIS process itself uses `RequestExecutionLevel user` and installs the bootstrapper/uninstaller for the current user. On a **first application install**, the existing PowerShell installer invokes the packaged SMB helper with `-Action Disable -WaitForElevation`; Windows UAC approval is therefore required for the machine-wide default-off SMB baseline. If that approval is cancelled or provisioning fails, the application is not activated. Upgrades preserve the operator's existing SMB maintenance choice and do not repeat first-install provisioning. Do not add `-SkipMachineProvisioning` to the end-user NSIS path.

The unsigned flag is explicit and the installer builder rejects unsigned packages by default. Production output still requires the genuine publisher-owned signing certificate, timestamp service and pinned trust policy. The generated Preview setup has been compiled only; do not treat compile success as install/uninstall acceptance or distribute it. Runtime acceptance must use a disposable Windows profile and deliberately account for the first-install UAC/SMB policy side effect.
