[CmdletBinding()]
param(
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$buildScript = Join-Path $PSScriptRoot 'New-PhotoBoothWindowsPackage.ps1'
$installScript = Join-Path $PSScriptRoot 'Install-PhotoBoothWindowsPackage.ps1'
$rollbackScript = Join-Path $PSScriptRoot 'Rollback-PhotoBoothWindowsPackage.ps1'
$uninstallScript = Join-Path $PSScriptRoot 'Uninstall-PhotoBoothWindowsPackage.ps1'

$runId = [guid]::NewGuid().ToString('N')
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-autostart-lifecycle-' + $runId)
$packageDirectory = Join-Path $tempRoot 'packages'
$installRoot = Join-Path $tempRoot 'install'
$registryPath = "HKCU:\Software\PhotoBooth\Tests\AutoStartLifecycle-$runId"
$userDataRoot = Join-Path $env:LOCALAPPDATA 'PhotoBooth'
$userDataExistedBefore = Test-Path -LiteralPath $userDataRoot
$version = '0.0.0-autostart.' + (Get-Date -Format 'yyyyMMddHHmmss')
$updateVersion = $version + '-update.1'
$status = 'FAIL'
$failure = $null
$checks = [System.Collections.Generic.List[string]]::new()

function Assert-AutoStartTargetsVersion {
    param([Parameter(Mandatory=$true)][string]$ExpectedVersion)

    $pointer = Get-PhotoBoothActivePointer -InstallRoot $installRoot
    if (!$pointer -or $pointer.active_version -ne $ExpectedVersion) {
        throw "Active pointer mismatch. Expected '$ExpectedVersion'."
    }

    $expectedExecutable = Join-Path (Join-Path (Join-Path $installRoot 'releases') $ExpectedVersion) 'PhotoBooth.exe'
    $expectedCommand = '"' + $expectedExecutable + '" --kiosk'
    $stored = [string](Get-ItemProperty -Path $registryPath -Name PhotoBooth -ErrorAction Stop).PhotoBooth
    if ($stored -cne $expectedCommand) {
        throw "Auto-start registry command mismatch. Expected '$expectedCommand', found '$stored'."
    }

    [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory (Split-Path -Parent $expectedExecutable) -ExpectedVersion $ExpectedVersion -ExpectedSha256 $pointer.active_sha256 -AllowUnsignedPreview)
}

try {
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    [void](Assert-PhotoBoothInstallRoot -InstallRoot $installRoot)
    Assert-PhotoBoothNotRunning -InstallRoot $installRoot

    if (!$SkipBuild) {
        Write-Host 'STEP=build-current-windows-executable'
        & (Join-Path $repoRoot 'run-photobooth.ps1') -NoLaunch
        if ($LASTEXITCODE -ne 0) {
            throw 'Building PhotoBooth.exe failed.'
        }
    }

    Write-Host 'STEP=build-version-1-package'
    $package = & $buildScript -Version $version -SkipBuild -AllowUnsignedPreview -OutputDirectory $packageDirectory | Select-Object -Last 1
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $package -PathType Leaf)) {
        throw "Version 1 package was not created: $package"
    }

    Write-Host 'STEP=install-version-1-and-register-test-autostart'
    & $installScript -PackagePath $package -InstallRoot $installRoot -SkipMachineProvisioning -AutoStartRegistryPath $registryPath -AllowUnsignedPreview | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Version 1 installation failed.' }
    Assert-AutoStartTargetsVersion -ExpectedVersion $version
    $checks.Add('fresh install registers the active immutable release in the supplied current-user registry path') | Out-Null

    Write-Host 'STEP=build-version-2-package'
    $updatePackage = & $buildScript -Version $updateVersion -SkipBuild -AllowUnsignedPreview -OutputDirectory $packageDirectory | Select-Object -Last 1
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $updatePackage -PathType Leaf)) {
        throw "Version 2 package was not created: $updatePackage"
    }

    Write-Host 'STEP=upgrade-and-refresh-test-autostart'
    & $installScript -PackagePath $updatePackage -InstallRoot $installRoot -SkipMachineProvisioning -AutoStartRegistryPath $registryPath -AllowUnsignedPreview | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Version 2 upgrade failed.' }
    Assert-AutoStartTargetsVersion -ExpectedVersion $updateVersion
    $checks.Add('upgrade moves auto-start to the newly active immutable release') | Out-Null

    Write-Host 'STEP=rollback-and-refresh-test-autostart'
    & $rollbackScript -InstallRoot $installRoot -AutoStartRegistryPath $registryPath -AllowUnsignedPreview
    if ($LASTEXITCODE -ne 0) { throw 'Rollback failed.' }
    Assert-AutoStartTargetsVersion -ExpectedVersion $version
    $checks.Add('rollback moves auto-start back to the restored immutable release') | Out-Null

    Write-Host 'STEP=uninstall-and-remove-owned-test-autostart'
    & $uninstallScript -InstallRoot $installRoot -AutoStartRegistryPath $registryPath -Confirm:$false
    if ($LASTEXITCODE -ne 0) { throw 'Uninstall failed.' }
    if (Test-Path -LiteralPath $installRoot) {
        throw 'Uninstall left the executable/release install directory behind.'
    }
    $remainingValue = Get-ItemProperty -Path $registryPath -Name PhotoBooth -ErrorAction SilentlyContinue
    if ($remainingValue) {
        throw 'Uninstall left the PhotoBooth test auto-start value behind.'
    }
    if ((Test-Path -LiteralPath $userDataRoot) -ne $userDataExistedBefore) {
        throw 'Uninstall changed the existence of the separate LOCALAPPDATA PhotoBooth user-data root.'
    }
    $checks.Add('uninstall removes the owned auto-start value and release files without changing user-data root') | Out-Null

    $status = 'PASS'
    Write-Host 'PHOTOBOOTH_WINDOWS_AUTOSTART_LIFECYCLE=PASS'
    foreach ($check in $checks) { Write-Host "CHECK=$check" }
}
catch {
    $failure = $_.Exception.Message
    Write-Host 'PHOTOBOOTH_WINDOWS_AUTOSTART_LIFECYCLE=FAIL'
    Write-Host "FAILURE=$failure"
    throw
}
finally {
    $running = @(Get-CimInstance -ClassName Win32_Process -Filter "Name = 'PhotoBooth.exe'" -ErrorAction SilentlyContinue)
    if ($running.Count -gt 0) {
        Write-Warning 'A PhotoBooth.exe process remains; preserving the temporary install root for safety.'
        foreach ($process in $running) {
            Write-Warning "RUNNING_PID=$($process.ProcessId) PATH=$($process.ExecutablePath)"
        }
    } else {
        if (Test-Path -LiteralPath $tempRoot) {
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    Remove-Item -LiteralPath $registryPath -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "TEMP_ROOT_CLEANED=$(!(Test-Path -LiteralPath $tempRoot))"
    Write-Host "TEST_REGISTRY_CLEANED=$(!(Test-Path -LiteralPath $registryPath))"
}
if ($status -ne 'PASS') { exit 1 }
