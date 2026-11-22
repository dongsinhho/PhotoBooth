[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [string]$InstallRoot,
    [string]$AutoStartRegistryPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    $InstallRoot = Join-Path $env:LOCALAPPDATA 'Programs\PhotoBooth'
}
$InstallRoot = Assert-PhotoBoothInstallRoot -InstallRoot $InstallRoot
if (!(Test-Path -LiteralPath $InstallRoot -PathType Container)) {
    throw "PhotoBooth is not installed at '$InstallRoot'."
}
Assert-PhotoBoothNotRunning -InstallRoot $InstallRoot
$pointer = Get-PhotoBoothActivePointer -InstallRoot $InstallRoot
if (!$pointer) {
    throw 'Install root has no active PhotoBooth release pointer; refusing to remove an unrecognized folder.'
}

if ($PSCmdlet.ShouldProcess($InstallRoot, 'Uninstall PhotoBooth binaries and release history')) {
    $removed = Remove-PhotoBoothCurrentUserAutoStart -InstallRoot $InstallRoot -RegistryPath $AutoStartRegistryPath
    if ($removed) {
        Write-Host 'CURRENT_USER_AUTO_START=REMOVED'
    } else {
        Write-Host 'CURRENT_USER_AUTO_START=UNCHANGED_OR_NOT_PRESENT'
    }

    Remove-Item -LiteralPath $InstallRoot -Recurse -Force
    if (Test-Path -LiteralPath $InstallRoot) {
        throw 'PhotoBooth install directory still exists after uninstall.'
    }
    Write-Host 'PHOTOBOOTH_UNINSTALL=PASS'
    Write-Host 'USER_DATA_PRESERVED=%LOCALAPPDATA%\PhotoBooth'
}
