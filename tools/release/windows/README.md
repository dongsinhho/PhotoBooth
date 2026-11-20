# PhotoBooth Windows Preview release tooling

**Status:** Internal-alpha ZIP packaging, explicit unsigned Preview opt-in, and Authenticode signed-candidate verification are implemented. No genuine publisher certificate is pinned; this is not a public installer, production-approved release or automatic updater.

## Scope and safety model

This flow packages the isolated Wails Windows Preview as a strict root-only ZIP containing PhotoBooth.exe, README.txt, exactly one release manifest, and the optional manage-smb-remote-admin.ps1 helper. Unsigned Preview packages use release-manifest.json; signed candidates use release-manifest.ps1 (an Authenticode-signed envelope containing base64 JSON). The installer rejects mixed manifests, nested paths and unexpected entries.

The package records the explicit alpha version, Windows x64 target, source Git revision, dirty-working-tree flag, Go/Wails toolchain metadata, executable size, and SHA-256. An adjacent .sha256 file detects ZIP corruption during transfer when checked against a trusted copy.

Installation is offline and side-by-side:

- Each version is staged and verified before being moved into releases/<version>.
- active.json is replaced atomically only after the new release is verified.
- The previous active version remains installed and can be restored.
- The installer never terminates an existing process.
- The scripts reject an installation path that overlaps %LOCALAPPDATA%\PhotoBooth.
- The app's settings, frames, sessions and photos remain in their separate user-data directory.
- The installer accepts PhotoBooth.exe, README.txt, exactly one manifest (unsigned JSON Preview or signed Authenticode envelope), and at most the managed SMB helper; no arbitrary paths, nested files or package scripts are extracted.
- The helper's size and SHA-256 are included in the release manifest; signed candidates also require its Authenticode signer to match the pinned publisher.
- On a first install, the installer invokes helper Disable through UAC and does not activate the release unless the default-off state is applied. Existing sessions, dependent services or custom shares cause the operation to stop for review rather than be force-changed.
- On upgrades, the installer preserves the machine's existing SMB administration choice; it does not silently disable a maintenance configuration the operator deliberately enabled.

The unsigned internal-alpha flow does not authenticate a publisher. Signed-candidate packages use an Authenticode-signed manifest envelope plus signed EXE/helper artifacts and a pinned publisher thumbprint. The current production trust allowlist is intentionally empty until a genuine publisher certificate is selected and verified. This does not add download/update transport, automatic updates, config migration, or production UI certification.

## Build a package

From the repository root:

~~~powershell
.\tools\release\windows\New-PhotoBoothWindowsPackage.ps1 -Version 0.1.0-alpha.1 -AllowUnsignedPreview
~~~

The script uses run-photobooth.ps1 -NoLaunch by default and writes a new, non-overwriting ZIP plus checksum under artifacts\windows\.

Use -SkipBuild only when intentionally packaging the already-built executable. The manifest captures whether the source tree was dirty; a dirty build is an internal checkpoint, not a reproducible source release.

## Install or update

~~~powershell
.\tools\release\windows\Install-PhotoBoothWindowsPackage.ps1 -PackagePath .\artifacts\windows\PhotoBooth-internal-alpha-0.1.0-alpha.1-windows-amd64.zip -AllowUnsignedPreview
~~~

Default install root: %LOCALAPPDATA%\Programs\PhotoBooth.

For isolated tests, pass an explicit temporary -InstallRoot outside %LOCALAPPDATA%\PhotoBooth. Synthetic installer tests can use `-SkipMachineProvisioning`; this switch is for test fixtures only and must not be used for a real first booth installation.

## Launch the selected version

~~~powershell
.\tools\release\windows\Start-PhotoBooth.ps1 -AllowUnsignedPreview
# or
.\tools\release\windows\Start-PhotoBooth.ps1 -Kiosk -AllowUnsignedPreview
~~~

Use -NoLaunch to verify the active pointer and executable without starting the app.

## Roll back

Close PhotoBooth gracefully, then run:

~~~powershell
.\tools\release\windows\Rollback-PhotoBoothWindowsPackage.ps1 -AllowUnsignedPreview
~~~

Rollback validates both release folders and their hashes before atomically switching active/previous versions. It does not uninstall or delete either version or user data.

## Local lifecycle test

~~~powershell
.\tools\release\windows\release-lifecycle-smoke.ps1
.\tools\release\windows\release-signing-smoke.ps1
~~~

The lifecycle smoke creates synthetic MZ-header fixtures in a unique TEMP folder and tests unsigned default-deny, explicit Preview opt-in, helper-less first-install refusal, update, rollback, active verification, binary/manifest hash rejection and ZIP sidecar rejection. It uses the explicit test-only machine-provisioning bypass, never launches PhotoBooth or accesses a camera, and removes fixture files in a finally block.

The separate signing smoke creates a disposable TEST-ONLY Code Signing certificate, signs copies of the EXE/helper plus the signed-manifest envelope, validates pinned-publisher checks, tamper rejection and unsigned default-deny. It removes the test certificate from current-user stores and all temporary artifacts. The test thumbprint is never added to the production allowlist.

## Deliberately deferred

- Select and obtain the genuine publisher Code Signing certificate, verify the timestamp authority, and pin the publisher thumbprint; current signed packages are candidates only, not production-approved releases.
- Public distribution and end-user terms; see docs/knowledge-base/LICENSE_STRATEGY.md.
- Automatic update service, authenticated manifest transport, staged download and background update.
- Database/config migration and rollback policy for future schema changes.
- Windows auto-start, kiosk shell lockdown and production support commitments.
- Linux package/install/update path.


## Installed package update / rollback / kiosk smoke

From the repository root:

~~~powershell
.\tools\release\windows\installed-package-kiosk-smoke.ps1 -AllowUnsignedPreview
~~~

The wrapper builds the current unsigned Windows Preview (requires `-AllowUnsignedPreview`), creates a versioned ZIP, installs it under a unique TEMP root, creates a second package label from the same executable, updates the active pointer, rolls back to the first version, verifies the restored binary/hash, then launches the installed executable through the kiosk UI smoke. The UI smoke accepts -ExecutablePath so it tests the installed release rather than the build-tree copy.

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


## Release authenticity and publisher trust (2026-11-20)

Unsigned Preview packages are now explicit opt-in only:
- Build an unsigned internal package with `-AllowUnsignedPreview`.
- Install, start or roll back an unsigned package only by also passing `-AllowUnsignedPreview` to that operation.
- The installer rejects unsigned packages before activation when the switch is absent.
- Unsigned package manifests must remain on the `internal-alpha` channel and use `release-manifest.json`.

Signed release candidates:
- The builder signs staged copies of `PhotoBooth.exe` and `manage-smb-remote-admin.ps1` with Authenticode SHA-256; source/build-tree files are not modified by signing.
- It embeds the JSON release manifest as a base64 payload in `release-manifest.ps1` and Authenticode-signs that envelope. The installer verifies the envelope signature and pinned publisher before parsing the metadata, then checks the signed EXE/helper, file size and SHA-256.
- Build command (replace values only after a genuine organization-owned Code Signing certificate and timestamp authority are available):

~~~powershell
.\tools\release\windows\New-PhotoBoothWindowsPackage.ps1 -Version 0.1.0-rc.1 -SigningCertificateThumbprint <40-character-thumbprint> -TimestampServer <RFC3161-timestamp-url>
~~~

- Pin the verified publisher thumbprint in `$script:PhotoBoothTrustedPublisherThumbprints` in `Release.Common.ps1` only after independently confirming it with the legal product owner / publisher. The current allowlist is intentionally empty. Consequently, signed packages are rejected until this external trust decision is made.
- Signed packages use the `signed-candidate` channel. They are not declared production approved; production UI, legal/distribution and hardware acceptance remain separate.
- SHA-256 sidecars remain transfer-corruption checks, not publisher proof.

Validation scripts:
- `release-lifecycle-smoke.ps1` verifies unsigned default-deny, explicit Preview opt-in, immutable install/update/rollback and checksum/manifest rejection without launching PhotoBooth.
- `release-signing-smoke.ps1` creates a temporary TEST-ONLY certificate in the current-user stores, signs staged copies and the manifest envelope, verifies the publisher pin and tamper detection, and removes the certificate and temporary artifacts. It does not alter the production trust allowlist.
- `installed-package-kiosk-smoke.ps1` launches the app to test the installed kiosk UI; because it uses an unsigned Preview fixture, call it explicitly with `-AllowUnsignedPreview`.

No production publisher certificate has been selected or pinned yet. Do not use a self-signed/test certificate as a production publisher.
