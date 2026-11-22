[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')
$runId = [guid]::NewGuid().ToString('N')
$root = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-autostart-test-' + $runId)
$registryPath = "HKCU:\Software\PhotoBooth\Tests\AutoStart-$runId"
$status = 'FAIL'
try {
    $release = Join-Path (Join-Path $root 'releases') '0.0.1-test'
    New-Item -ItemType Directory -Path $release -Force | Out-Null
    $exe = Join-Path $release 'PhotoBooth.exe'
    [IO.File]::WriteAllBytes($exe, [byte[]](0x4D,0x5A,0x00,0x00))
    $value = Set-PhotoBoothCurrentUserAutoStart -InstallRoot $root -ExecutablePath $exe -RegistryPath $registryPath
    $expected = '"' + $exe + '" --kiosk'
    if ($value -cne $expected) { throw "Auto-start command mismatch: $value" }
    $stored = [string](Get-ItemProperty -Path $registryPath -Name PhotoBooth).PhotoBooth
    if ($stored -cne $expected) { throw "Registry value mismatch: $stored" }
    Write-Host 'REGISTER_CURRENT_USER_AUTO_START=PASS'
    $removed = Remove-PhotoBoothCurrentUserAutoStart -InstallRoot $root -RegistryPath $registryPath
    if (!$removed) { throw 'Auto-start registry value was not removed.' }
    if ((Get-ItemProperty -Path $registryPath -Name PhotoBooth -ErrorAction SilentlyContinue)) {
        throw 'Auto-start registry value still exists after removal.'
    }
    Write-Host 'REMOVE_CURRENT_USER_AUTO_START=PASS'

    $writeBlocked = $false
    try {
        Set-PhotoBoothCurrentUserAutoStart -InstallRoot $root -ExecutablePath $exe -RegistryPath 'HKLM:\Software\PhotoBooth\Tests\MustNotBeCreated' | Out-Null
    } catch {
        if ($_.Exception.Message -notlike '*may only write under the current user HKCU Software hive*') { throw }
        $writeBlocked = $true
    }
    if (!$writeBlocked) { throw 'Set helper accepted a non-HKCU registry path.' }
    Write-Host 'NON_HKCU_WRITE_REJECTED=PASS'

    $removeBlocked = $false
    try {
        Remove-PhotoBoothCurrentUserAutoStart -InstallRoot $root -RegistryPath 'HKLM:\Software\PhotoBooth\Tests\MustNotBeCreated' | Out-Null
    } catch {
        if ($_.Exception.Message -notlike '*may only remove values under the current user HKCU Software hive*') { throw }
        $removeBlocked = $true
    }
    if (!$removeBlocked) { throw 'Remove helper accepted a non-HKCU registry path.' }
    Write-Host 'NON_HKCU_REMOVE_REJECTED=PASS'
    $status = 'PASS'
}
finally {
    Remove-Item -LiteralPath $registryPath -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
Write-Host "PHOTOBOOTH_WINDOWS_AUTOSTART_TEST=$status"
if ($status -ne 'PASS') { exit 1 }
