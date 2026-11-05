[CmdletBinding()]
param([string]$InstallRoot)

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
    throw 'No active release pointer exists.'
}
if (!$pointer.previous_version) {
    throw 'No previous release is recorded; rollback is unavailable.'
}

$currentVersion = [string]$pointer.active_version
$currentHash = [string]$pointer.active_sha256
$targetVersion = [string]$pointer.previous_version
$targetHash = [string]$pointer.previous_sha256

$releasesRoot = Join-Path $InstallRoot 'releases'
$currentDirectory = Join-Path $releasesRoot $currentVersion
$targetDirectory = Join-Path $releasesRoot $targetVersion

[void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $currentDirectory -ExpectedVersion $currentVersion -ExpectedSha256 $currentHash)
[void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $targetDirectory -ExpectedVersion $targetVersion -ExpectedSha256 $targetHash)

Write-PhotoBoothActivePointer -InstallRoot $InstallRoot -ActiveVersion $targetVersion -ActiveSha256 $targetHash -PreviousVersion $currentVersion -PreviousSha256 $currentHash

Write-Host "ROLLBACK_FROM=$currentVersion"
Write-Host "ACTIVE_VERSION=$targetVersion"
Write-Host "PREVIOUS_VERSION=$currentVersion"
Write-Host 'Both immutable release folders were preserved.'
