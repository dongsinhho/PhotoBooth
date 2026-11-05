[CmdletBinding()]
param(
    [string]$InstallRoot,
    [switch]$Kiosk,
    [switch]$NoLaunch
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    $InstallRoot = Join-Path $env:LOCALAPPDATA 'Programs\PhotoBooth'
}
$InstallRoot = Assert-PhotoBoothInstallRoot -InstallRoot $InstallRoot
if (!(Test-Path -LiteralPath $InstallRoot -PathType Container)) {
    throw "PhotoBooth is not installed at '$InstallRoot'. Run Install-PhotoBoothWindowsPackage.ps1 first."
}

$pointer = Get-PhotoBoothActivePointer -InstallRoot $InstallRoot
if (!$pointer) {
    throw 'No active PhotoBooth release is selected. Install a package first.'
}

$releaseDirectory = Join-Path (Join-Path $InstallRoot 'releases') $pointer.active_version
$release = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $releaseDirectory -ExpectedVersion $pointer.active_version -ExpectedSha256 $pointer.active_sha256

if ($NoLaunch) {
    Write-Host "ACTIVE_VERSION=$($pointer.active_version)"
    Write-Host "EXECUTABLE=$($release.Executable)"
    Write-Host "SHA256=$($release.Sha256)"
    return
}

Assert-PhotoBoothNotRunning -InstallRoot $InstallRoot

$previousKioskValue = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_KIOSK', 'Process')
$kioskValue = if ($Kiosk) { '1' } else { '0' }
[Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $kioskValue, 'Process')
try {
    $process = Start-Process -FilePath $release.Executable -WorkingDirectory $releaseDirectory -PassThru
}
finally {
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $previousKioskValue, 'Process')
}

Write-Host "PhotoBooth Windows Preview $($pointer.active_version) started (PID $($process.Id))."
Write-Host 'Operator data remains under %LOCALAPPDATA%\PhotoBooth and is not part of the release folder.'
