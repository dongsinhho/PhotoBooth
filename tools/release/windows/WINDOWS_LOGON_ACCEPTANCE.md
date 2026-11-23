# Windows interactive logon auto-start acceptance

The selected Windows operating account is an account in the local Administrators group. Do not create a separate Standard account for this acceptance.

The current ShinPc login SHINPC\sinhv is a member of the local Administrators group. This is an account-membership choice, not a requirement to launch PhotoBooth with a UAC elevation prompt. PhotoBooth is expected to run in the logged-in interactive user session and use that user's HKCU Run entry.

## Prepare an internal acceptance kit

From the repository root, with the latest Windows Preview already built:

    .\tools\release\windows\Prepare-PhotoBoothLogonAcceptanceKit.ps1 -SkipBuild

The script writes a uniquely named kit under Windows Public Documents. It contains an unsigned internal Preview ZIP, package install/uninstall scripts, Release.Common.ps1, a read-only post-logon verifier and a README.

## Acceptance procedure

1. Sign in to the intended operator account (currently SHINPC\sinhv) and confirm it belongs to the local Administrators group:

       whoami
       Get-LocalGroupMember -Group 'Administrators'

2. From the kit folder, install under that same account using the README command. -SkipMachineProvisioning avoids changing machine-wide SMB policy.

3. Confirm CURRENT_USER_AUTO_START=ENABLED, close PhotoBooth gracefully, sign out, then sign back into the same account without manually launching the app.

4. Run Test-PhotoBoothLogonStartup.ps1 -AllowUnsignedPreview from the kit. The verifier checks:
   - The current user's HKCU Run value matches the active release executable and --kiosk.
   - That executable is running in the current interactive session with --kiosk.
   - Customer Mode is visible and the public Operator access entry is absent.
   - The main window fills the primary display.

5. Close PhotoBooth and uninstall from the same account. Confirm the uninstall prompt. User data under %LOCALAPPDATA%\PhotoBooth is preserved.

6. Sign out and back in once more to verify the app no longer auto-starts, then delete the acceptance kit.

## Scope and limitations

- This uses an unsigned internal Preview package and does not certify the final signed production installer.
- No separate Standard account is required.
- The account runs PhotoBooth in its interactive session; do not add a machine-wide service or forced UAC elevation for this purpose.
- App fullscreen is not OS shell lockdown; Windows shortcuts such as Alt+Tab and the Windows key remain OS-level behavior.
- On 2026-11-23, an unsigned internal Preview was built and installed under the selected SHINPC\sinhv account. The user's HKCU Run value is now enabled and points to the active immutable release with `--kiosk`. Actual sign-out/sign-in observation remains pending; do not launch PhotoBooth manually before running the verifier.
- The install used `-SkipMachineProvisioning`; it did not change machine-wide SMB policy. The current read-only observation after install was LanmanServer Running/Automatic.