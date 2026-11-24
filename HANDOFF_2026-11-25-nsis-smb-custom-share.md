# PhotoBooth Handoff - 2026-11-25 - NSIS SMB custom-share acceptance

## Repository

- Path: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- Branch: `main`
- Base HEAD before this work: `ea2479a build(windows): add versioned per-user NSIS installer`
- Working tree had five source/doc modifications before checkpoint; inspect final status/commit at handoff.
- Remote device: ShinPc (`871fb565-63c5-49a0-864d-922768f83762`).

## Why the 0.1.0 setup aborted

The first run extracted and integrity-validated the versioned release, but the elevated SMB helper blocked first-install provisioning:

`Custom SMB share(s) exist: Users. The helper will not expose or disrupt them implicitly.`

UAC was accepted. The error was a deliberate helper safeguard, not a UAC refusal or an NSIS extract/hash problem.

Elevated read-only audit:
- `Users` SMB share at `C:\Users`, `Special=False`, `CurrentUsers=0`.
- Share ACL: `BUILTIN\Administrators` Full; `Everyone` Full. Do not modify the share ACL or delete its definition/path/data.
- No active SMB sessions, no open files, no running LanmanServer dependent services.
- Existing SMB allow rule allows source `192.168.1.20` to local `192.168.1.11` on Private profile.
- Baseline at the last safe preflight: `LanmanServer` Running/Automatic, SMB1 false, SMB2 true, `AutoShareWks=1`, PhotoBooth allow rule enabled.
- The aborted first run had not created active pointer or HKCU Run. `%LOCALAPPDATA%\PhotoBooth` user data remains untouched.

The user explicitly authorized disconnecting the existing SMB link from the Raspbian host. The intended effect is that all SMB shares stop being served when LanmanServer is stopped, while persistent share definitions, paths, ACLs and user data remain preserved. This is not a deletion of `C:\Users` or the shared files.

## Implementation changes (uncommitted at checkpoint creation)

1. `tools/self-service/manage-smb-remote-admin.ps1`
   - Added `-AllowStoppingCustomShares`, restricted to `-Action Disable`; default behavior still blocks custom shares.
   - If the explicit flag is passed on Disable, logs/warns the names/paths that will become unserved; no share definition or ACL is deleted.
   - Active SMB sessions and running dependent services remain hard blockers.
   - Added `-SuppressConfirmation` for the post-wizard-confirmation provisioning flow. This avoids the PowerShell 5.1 `-Confirm:$false` string-binding problem.
   - Added optional `-ResultLogPath` and log records for action/admin state, custom shares, success, and exception/stack trace. UAC elevation forwards all options.
2. `tools/release/windows/Install-PhotoBoothWindowsPackage.ps1`
   - On first install passes `-AllowStoppingCustomShares -SuppressConfirmation -WaitForElevation -ResultLogPath` to packaged helper.
   - On failure collects helper process output and elevated result log, throws before active pointer/auto-start activation and keeps diagnostic path.
3. `tools/release/windows/PhotoBoothInstaller.nsi`
   - Asks a Yes/No first-install warning before the script runs, explicitly describing service shutdown and network unavailability of shares while preserving definitions/ACLs/data.
   - Uses NSIS `nsExec::ExecToLog` to show PowerShell output in Details pane for both signed/unsigned branches.
4. `tools/release/windows/New-PhotoBoothWindowsPackage.ps1` and `tools/release/windows/README.md`
   - Generated package/readme and release docs describe the custom share behavior, hard blockers and diagnostics.

## Safety test results before runtime retry

- Elevated helper run with `-Action Disable -AllowStoppingCustomShares -SuppressConfirmation -WhatIf -ResultLogPath ...` returned exit code 0.
- It confirmed Admin token and warned `Users [C:\Users]` would become unserved.
- No-Active-SMB-Session and no-dependent-service guards passed.
- WhatIf output stated it would apply the default-off SMB disable action.
- Read-back after WhatIf proved no change: LanmanServer Running/Automatic, AutoShareWks 1, Users share still present and allow rule still enabled.
- Initial preflight without the new switches had exited 1 on custom Users share as intended.
- An intermediate preflight exposed the bad `-Confirm:$false` binding, which has been removed and replaced by the explicit switch.
- PowerShell parser checks passed for helper, install script, package builder, installer builder.
- `git diff --check` passed.

## Rebuilt package/installer (unsigned internal Preview)

- Version `0.1.1-nsis-preview`.
- Package: `artifacts/windows/PhotoBooth-internal-alpha-0.1.1-nsis-preview-windows-amd64.zip` (ignored by Git).
- ZIP SHA-256 `1AEE383DF5B05449785EE59ACFA277955058D4230AB9731B54DCF279A3F7FEBB`.
- Packaged helper and source helper SHA-256 match: `6120ca4d319142ab240549b3ce56b12a458d7adddfbc61b6ab8c807f6126dd35`.
- Setup: `%TEMP%\PhotoBoothInstallerPreview-0.1.1\PhotoBooth-0.1.1-nsis-preview-Setup.exe`.
- Size: 5,517,940 bytes.
- Setup SHA-256: `7129B48B47B199388D82C6CCD7881C2AB77F46C23CFD32B2224CED41FBA70225`.
- Built with NSIS 3.13. Authenticode `NotSigned`; internal Preview only.

## Current runtime state at checkpoint

**The 0.1.1 setup has NOT yet been launched.** No real SMB setting has been altered. Prior safe readback:
- LanmanServer Running/Automatic.
- AutoShareWks=1.
- `Users` share at C:\Users is still present with original ACL.
- PhotoBooth allow firewall rule still enabled for 192.168.1.20 -> 192.168.1.11.
- No active pointer/HKCU Run created for this install attempt.
- User data at `%LOCALAPPDATA%\PhotoBooth` preserved.

## Next action

Launch exactly the 0.1.1 installer above on ShinPc. User has already authorized disconnection of current SMB remote administration path. The wizard first displays a clear Yes/No warning about stopping LanmanServer and rendering all SMB shares unavailable to network clients; user should accept. Then accept the separate Windows UAC prompt. Do not run installer a second time if it fails; read NSIS Details and the `PhotoBooth-SmbProvisioning-*.log` in `%TEMP%`.

After completion verify:
1. `LanmanServer` Stopped/Disabled.
2. `AutoShareWks` exists as DWORD 0.
3. PhotoBooth SMB allow rule absent.
4. SMB 445 and NetBIOS inbound block rules effective.
5. Persistent `Users` share definition and ACL still intact (service stopped; not served).
6. No active pointer/Run until provisioning passes; after pass, active.json points 0.1.1 and HKCU Run points to its PhotoBooth.exe `--kiosk`.
7. `%LOCALAPPDATA%\PhotoBooth` user data unchanged.

The actual installer runtime acceptance is still PENDING. If successful, continue with logon, upgrade/rollback and safe uninstall without deleting user data. The 0.1.0 aborted release directory is an inert orphan under `%LOCALAPPDATA%\Programs\PhotoBooth\releases`; do not delete it until 0.1.1 is active and the release lifecycle is verified.

## Other gates

- Real Windows 11 <=8 GiB and native Ubuntu 24.04 / 4 GiB target acceptance remains PENDING by user instruction.
- Genuine signed production installer/publisher trust remains PENDING.
- Ubuntu .deb/native camera and printer gates remain PENDING.
- Physical printer acceptance remains deferred.


## Upgrade/rollback runtime acceptance and uninstall hardening (2026-11-25)

### Runtime results

- NSIS setup `0.1.2-nsis-preview` compiled and the user completed the upgrade wizard from active `0.1.1`.
- Independent post-upgrade verification: active pointer `0.1.2`, previous `0.1.1`, HKCU Run points to `0.1.2` executable with `--kiosk`; LanmanServer Stopped/Disabled, AutoShareWks DWORD 0, three active inbound block rules; operator data still present.
- Integrity validated for both releases using `Assert-PhotoBoothReleaseDirectory -AllowUnsignedPreview`.
- Ran `Rollback-PhotoBoothWindowsPackage.ps1 -AllowUnsignedPreview`. Script printed rollback complete and updated pointer/Run to `0.1.1`, previous `0.1.2`. A shell wrapper then threw due to stale `$LASTEXITCODE`; the rollback itself had succeeded. Independent checks then passed pointer values, HKCU Run, hashes of both releases, unchanged SMB state, all user-data items and app closed.
- User's current release was restored to `0.1.1-nsis-preview`.

### Fix for actual NSIS uninstall

Inspection found `PhotoBoothInstaller.nsi` invoked the uninstaller script with `-Confirm:$$false` through Windows PowerShell `-File`. Replaced this with `-SuppressConfirmation` and `nsExec::ExecToLog`, plus `Pop $0`/exit-code check for visible diagnostics. `Uninstall-PhotoBoothWindowsPackage.ps1` now accepts `[switch]$SuppressConfirmation`; it sets `$ConfirmPreference='None'` only for explicit NSIS invocation. Direct use keeps native confirmation behavior.

Tested the updated source script on the real install with `-SuppressConfirmation -WhatIf`: it did not prompt, reported the expected uninstall action, and the active pointer, HKCU Run and user data were unchanged.

### New setup build

- Version `0.1.3-nsis-preview`.
- Package ZIP SHA-256 `58625E5931B05D3E5E899210A6A2F58A4DCC774A33852C6448DF43EB725D9F73`.
- Setup path `%TEMP%\PhotoBoothInstallerPreview-0.1.3\PhotoBooth-0.1.3-nsis-preview-Setup.exe`.
- 5,521,859 bytes; SHA-256 `464F8B8EE9FE2AF85C40D3CC79AC253AA297F2AEA9B6A8B9A26DE93DC6D380DD`.
- NSIS 3.13 compile passed; Authenticode `NotSigned`.
- Package/setup are ignored/outside Git artifacts.

### Current machine state / next action

Current app install is still active at `0.1.1-nsis-preview` with `0.1.2` previous; application is closed. SMB stays Stopped/Disabled, AutoShareWks 0, all three inbound blocks active, Users share definition still at `C:\Users`, operator data remains in `%LOCALAPPDATA%\PhotoBooth`.

Next: upgrade through the `0.1.3` NSIS setup (existing active install means no first-install SMB dialog/UAC should be needed), verify active pointer and SMB unchanged; then run the installed `PhotoBooth Setup\\uninstall.exe` in normal user session. If UI asks for confirmation, proceed. Verify there is no PhotoBooth process, `%LOCALAPPDATA%\\Programs\\PhotoBooth` absent, HKCU Run value absent, setup root and HKCU bootstrapper/uninstall registrations absent, while `%LOCALAPPDATA%\\PhotoBooth` four operator data items remain and SMB stays Disabled. This closes the full install → upgrade → rollback → uninstall lifecycle if all pass.
