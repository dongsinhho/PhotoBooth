# PhotoBooth Windows Preview release tooling

**Status:** Internal alpha packaging and local version-switch prototype. Not a public installer or production updater.

## Scope and safety model

This flow packages the isolated Wails Windows Preview as a ZIP containing exactly:

- PhotoBooth.exe
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
- The installer accepts only three expected root-level ZIP entries; no arbitrary paths, nested files or package scripts are extracted.

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

For isolated tests, pass an explicit temporary -InstallRoot outside %LOCALAPPDATA%\PhotoBooth.

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

This creates synthetic tiny MZ-header fixtures in a unique TEMP folder, builds two valid internal package ZIPs, tests install, active version, update, rollback and active verification, then corrupts a package and verifies that the checksum gate rejects it before the active pointer changes. It never launches PhotoBooth or accesses a camera. All fixture files are removed in a finally block.

## Deliberately deferred

- Signed installer and code signing certificate.
- Public distribution and end-user terms; see docs/knowledge-base/LICENSE_STRATEGY.md.
- Automatic update service, authenticated manifest transport, staged download and background update.
- Database/config migration and rollback policy for future schema changes.
- Windows auto-start, kiosk shell lockdown and production support commitments.
- Linux package/install/update path.
