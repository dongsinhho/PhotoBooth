[CmdletBinding()]
param(
    [string]$OutputRoot,
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$buildScript = Join-Path $PSScriptRoot 'New-PhotoBoothWindowsPackage.ps1'
$version = '0.0.0-logon-smoke.' + (Get-Date -Format 'yyyyMMddHHmmss')
$runId = [guid]::NewGuid().ToString('N')

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $commonDocuments = [Environment]::GetFolderPath('CommonDocuments')
    if ([string]::IsNullOrWhiteSpace($commonDocuments)) {
        throw 'Windows Public Documents folder could not be resolved.'
    }
    $OutputRoot = Join-Path $commonDocuments ('PhotoBoothLogonAcceptance-' + $runId)
}
$OutputRoot = [IO.Path]::GetFullPath($OutputRoot)
if (Test-Path -LiteralPath $OutputRoot) {
    throw "Refusing to overwrite an existing acceptance kit folder: $OutputRoot"
}
New-Item -ItemType Directory -Path $OutputRoot -Force | Out-Null

try {
    Write-Host 'STEP=build-unsigned-internal-preview-package'
    $buildArgs = @{
        Version = $version
        AllowUnsignedPreview = $true
        OutputDirectory = $OutputRoot
    }
    if ($SkipBuild) { $buildArgs.SkipBuild = $true }
    $package = & $buildScript @buildArgs | Select-Object -Last 1
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $package -PathType Leaf)) {
        throw "Could not build the acceptance package: $package"
    }
    $packageName = [IO.Path]::GetFileName($package)

    foreach ($name in @(
        'Install-PhotoBoothWindowsPackage.ps1',
        'Uninstall-PhotoBoothWindowsPackage.ps1',
        'Release.Common.ps1',
        'Test-PhotoBoothLogonStartup.ps1'
    )) {
        $source = Join-Path $PSScriptRoot $name
        if (!(Test-Path -LiteralPath $source -PathType Leaf)) {
            throw "Acceptance helper is missing: $source"
        }
        Copy-Item -LiteralPath $source -Destination (Join-Path $OutputRoot $name)
    }

    $readme = @"
PhotoBooth Windows logon auto-start acceptance kit
Version: $version
Channel: unsigned internal Preview - Administrator-group operator account only

This kit validates per-user auto-start under the existing Windows account
selected to operate PhotoBooth. Use the same account for install, sign-in and
uninstall. Do not create a separate Standard account for this acceptance.
No machine-wide SMB provisioning is performed by the documented install.

1. Sign in to the intended operator account (currently SHINPC\sinhv).
   Confirm it is a member of the local Administrators group:
   whoami
   Get-LocalGroupMember -Group 'Administrators'

2. Open PowerShell in this folder under that same account and install:
   powershell -NoProfile -ExecutionPolicy Bypass -File .\Install-PhotoBoothWindowsPackage.ps1 -PackagePath ".\$packageName" -SkipMachineProvisioning -AllowUnsignedPreview

3. Confirm the install command reports CURRENT_USER_AUTO_START=ENABLED.
   Close PhotoBooth gracefully, sign out of Windows, then sign back into the
   same operator account. Do not manually launch PhotoBooth first.

4. After sign-in, run:
   powershell -NoProfile -ExecutionPolicy Bypass -File .\Test-PhotoBoothLogonStartup.ps1 -AllowUnsignedPreview

   Expected: HKCU Run matches the active release, PhotoBooth runs in this
   interactive session with --kiosk, Customer Mode is visible, no public
   Operator access button is present, and the window fills the primary display.

5. Close PhotoBooth gracefully. Uninstall from the same operator account:
   powershell -NoProfile -ExecutionPolicy Bypass -File .\Uninstall-PhotoBoothWindowsPackage.ps1

   Confirm the uninstall prompt. The account's %LOCALAPPDATA%\PhotoBooth
   user data is preserved by design.

6. Sign out and back in once more to verify PhotoBooth no longer auto-starts.
   Delete this acceptance kit folder when finished.

Limitations:
- The package is unsigned and only for internal acceptance.
- This verifies the current user's HKCU Run-key behavior under an
  Administrator-group account, not the final signed production installer.
- Account membership in Administrators does not require PhotoBooth to request
  UAC elevation; it runs in the logged-in interactive user session.
- App-level fullscreen does not block OS shortcuts and is not OS shell lockdown.
"@
    Set-Content -LiteralPath (Join-Path $OutputRoot 'README.txt') -Value $readme -Encoding UTF8
    Write-Host 'PHOTOBOOTH_LOGON_ACCEPTANCE_KIT=READY'
    Write-Host "KIT_PATH=$OutputRoot"
    Write-Host "PACKAGE_PATH=$package"
    Write-Host "PACKAGE_VERSION=$version"
}
catch {
    Remove-Item -LiteralPath $OutputRoot -Recurse -Force -ErrorAction SilentlyContinue
    throw
}
