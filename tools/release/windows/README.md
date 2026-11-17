# PhotoBooth Windows Preview release tooling

**Status:** Internal alpha packaging and local version-switch prototype. Not a public installer or production updater.

## Scope and safety model

This flow packages the isolated Wails Windows Preview as a ZIP containing exactly four root-level files:

- PhotoBooth.exe
- manage-smb-remote-admin.ps1
- release-manifest.json
- README.txt

The package records the explicit alpha version, Windows x64 target, source Git revision, dirty-working-tree flag, Go/Wails toolchain metadata, executable size, and SHA-256. An adjacent .sha256 file detects ZIP corruption during transfer when checked against a trusted copy.

Installation is offline and side-by-side:

- Each version is staged and verified before being moved into releases/<version>.
- active.json is replaced atomically only after the new release is verified.
- The previous active version remains installed and can be restored.
- The installer never terminates an existing process.
- The scripts reject an installation path that overlaps %LOCALAPPDATA%\PhotoBooth.
- The app's settings, frames, sessions and photos remain in their separate user-data directory.
- The installer accepts only the three required root-level files and the optional managed SMB helper; no arbitrary paths, nested files or package scripts are extracted.
- The helper's size and SHA-256 are included in the release manifest and checked before activation.
- On a first install, the installer invokes helper Disable through UAC and does not activate the release unless the default-off state is applied. Existing sessions, dependent services or custom shares cause the operation to stop for review rather than be force-changed.
- On upgrades, the installer preserves the machine's existing SMB administration choice; it does not silently disable a maintenance configuration the operator deliberately enabled.

SHA-256 is not a digital signature. This flow does not authenticate the publisher, download updates, auto-update, migrate config schemas, or certify the Wails UI as production-ready.

## Build a package

From the repository root:

~~~powershell
.\tools\release\windows\New-PhotoBoothWindowsPackage.ps1 -Version 0.1.0-alpha.1
~~~

The script uses run-photobooth.ps1 -NoLaunch by default and writes a new, non-overwriting ZIP plus checksum under artifacts\windows\.

Use -SkipBuild only when intentionally packaging the already-built executable. The manifest captures whether the source tree was dirty; a dirty build is an internal checkpoint, not a reproducible source release.

## Install or update

~~~powershell
.\tools\release\windows\Install-PhotoBoothWindowsPackage.ps1 -PackagePath .\artifacts\windows\PhotoBooth-WindowsPreview-0.1.0-alpha.1-windows-amd64.zip
~~~

Default install root: %LOCALAPPDATA%\Programs\PhotoBooth.

For isolated tests, pass an explicit temporary -InstallRoot outside %LOCALAPPDATA%\PhotoBooth. Synthetic installer tests can use `-SkipMachineProvisioning`; this switch is for test fixtures only and must not be used for a real first booth installation.

## Launch the selected version

~~~powershell
.\tools\release\windows\Start-PhotoBooth.ps1
# or
.\tools\release\windows\Start-PhotoBooth.ps1 -Kiosk
~~~

Use -NoLaunch to verify the active pointer and executable without starting the app.

## Roll back

Close PhotoBooth gracefully, then run:

~~~powershell
.\tools\release\windows\Rollback-PhotoBoothWindowsPackage.ps1
~~~

Rollback validates both release folders and their hashes before atomically switching active/previous versions. It does not uninstall or delete either version or user data.

## Local lifecycle test

~~~powershell
.\tools\release\windows\release-lifecycle-smoke.ps1
~~~

This creates synthetic tiny MZ-header fixtures in a unique TEMP folder, tests legacy three-file package install with the explicit test-only provisioning bypass, update, rollback and active verification, then corrupts a package and verifies that the checksum gate rejects it before the active pointer changes. The second install follows the upgrade path and does not reapply machine provisioning. It never launches PhotoBooth or accesses a camera. All fixture files are removed in a finally block.

## Deliberately deferred

- Signed installer and code signing certificate.
- Public distribution and end-user terms; see docs/knowledge-base/LICENSE_STRATEGY.md.
- Automatic update service, authenticated manifest transport, staged download and background update.
- Database/config migration and rollback policy for future schema changes.
- Windows auto-start, kiosk shell lockdown and production support commitments.
- Linux package/install/update path.


## Installed package update / rollback / kiosk smoke

From the repository root:

~~~powershell
.\tools\release\windows\installed-package-kiosk-smoke.ps1
~~~

The wrapper builds the current Windows Preview, creates a versioned ZIP, installs it under a unique TEMP root, creates a second package label from the same executable, updates the active pointer, rolls back to the first version, verifies the restored binary/hash, then launches the installed executable through the kiosk UI smoke. The UI smoke accepts -ExecutablePath so it tests the installed release rather than the build-tree copy.

It verifies:
- Package creation and installation in a disposable root.
- Real-binary update from version 1 to version 2 and rollback to version 1; both immutable release folders remain.
- Active pointer, PE header, size and SHA-256 after rollback.
- Kiosk startup fullscreen, Esc to windowed, F11 to fullscreen and Esc back to windowed.
- Settings persistence on the installed executable: save 3 shots / 4s countdown / 750ms interval, close the first process, launch a second process against the same disposable profile, and verify the values reload.
- No photo capture/session is started. The UI smokes use disposable PHOTOBOOTH_DATA_DIR roots; package, install and temporary player data are removed after graceful app exit. The kiosk and settings JSON reports are retained in TEMP.

This is a Windows Preview deployment/kiosk functional smoke, not physical camera image-quality acceptance, a printer test, recovery test, long-run soak, signed release, Windows shell lockdown, or production certification.

## SMB remote administration in the Windows Preview

Open **Settings & frame -> Remote administration**. The panel reads the current LanmanServer state, AutoShareWks value and PhotoBooth-owned effective firewall rules. It also lists active physical adapters and their IPv4 addresses. It does not infer machine state from StoreConfig.

- The recommended booth baseline is **DEFAULT OFF**: Server Stopped/Disabled, AutoShareWks=0, PhotoBooth inbound Block rules active, and no PhotoBooth Allow rule.
- Enable requires an already-Private physical adapter, a selected local IPv4 and one or more individually trusted RFC1918 source IPv4 addresses. The UI shows a machine-wide confirmation and the helper requests UAC approval.
- Disable requests UAC and returns to the default-off state. Existing custom shares or active SMB sessions make the helper abort; it does not delete unrelated shares or ACLs.
- The UI refreshes status from Windows after either action. A failed refresh is reported as unverified rather than claiming the change succeeded.
- The helper sits beside PhotoBooth.exe in the release folder and its hash is covered by the manifest. Source-checkout builds locate the same helper under tools\self-service.
- Fresh package installation applies Disable before active.json is switched. Upgrades preserve the previous operator choice.

Read-only verification commands from the repository root:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\manage-smb-remote-admin.ps1 -Action Status -JsonOutput
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\manage-smb-remote-admin.ps1 -Action ListInterfaces -JsonOutput
~~~

The UI acceptance is included in [ui-settings-smoke.ps1](../../tools/ui-spike/wails-v2/ui-settings-smoke.ps1). It verifies the live default-off status and controls without pressing Enable/Disable, saving station settings, or creating a photo session.
