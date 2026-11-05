[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(5,30)][int]$TransitionTimeoutSeconds = 12,
    [ValidateRange(5,45)][int]$SettingsTimeoutSeconds = 20
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$buildScript = Join-Path $PSScriptRoot 'New-PhotoBoothWindowsPackage.ps1'
$installScript = Join-Path $PSScriptRoot 'Install-PhotoBoothWindowsPackage.ps1'
$startScript = Join-Path $PSScriptRoot 'Start-PhotoBooth.ps1'
$kioskSmoke = Join-Path $repoRoot 'tools\ui-spike\wails-v2\ui-kiosk-mode-smoke.ps1'
$settingsSmoke = Join-Path $repoRoot 'tools\ui-spike\wails-v2\ui-settings-persist-restart-smoke.ps1'

if (!(Test-Path -LiteralPath $kioskSmoke -PathType Leaf)) {
    throw "Kiosk UI smoke script is missing: $kioskSmoke"
}
if (!(Test-Path -LiteralPath $settingsSmoke -PathType Leaf)) {
    throw "Settings persistence smoke script is missing: $settingsSmoke"
}

$runId = [guid]::NewGuid().ToString('N')
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-installed-kiosk-' + $runId)
$packageDirectory = Join-Path $temporaryRoot 'packages'
$installRoot = Join-Path $temporaryRoot 'install'
$stdoutPath = Join-Path $temporaryRoot 'kiosk-smoke.stdout.log'
$stderrPath = Join-Path $temporaryRoot 'kiosk-smoke.stderr.log'
$settingsStdoutPath = Join-Path $temporaryRoot 'settings-smoke.stdout.log'
$settingsStderrPath = Join-Path $temporaryRoot 'settings-smoke.stderr.log'
$version = '0.0.0-smoke.' + (Get-Date -Format 'yyyyMMdd.HHmmss')
$smokeProcess = $null
$smokeReportPath = $null
$settingsReportPath = $null
$status = 'FAIL'
$failure = $null
$tempCleaned = $false
$checks = [System.Collections.Generic.List[string]]::new()

try {
    New-Item -ItemType Directory -Path $temporaryRoot -Force | Out-Null
    [void](Assert-PhotoBoothInstallRoot -InstallRoot $installRoot)
    Assert-PhotoBoothNotRunning -InstallRoot $installRoot

    $buildArgs = @{
        Version = $version
        OutputDirectory = $packageDirectory
    }
    if ($SkipBuild) { $buildArgs.SkipBuild = $true }

    Write-Host 'STEP=build-unsigned-windows-preview-zip'
    $package = & $buildScript @buildArgs | Select-Object -Last 1
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $package -PathType Leaf)) {
        throw "Windows Preview package build did not produce a ZIP: $package"
    }
    $checks.Add('build versioned package from the current source/binary') | Out-Null

    Write-Host 'STEP=install-package-to-disposable-root'
    & $installScript -PackagePath $package -InstallRoot $installRoot | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'Package installation returned a non-zero exit code.'
    }
    $checks.Add('install package under an isolated TEMP root') | Out-Null

    $updateVersion = $version + '-update.1'
    Write-Host 'STEP=install-real-binary-update'
    $updatePackage = & $buildScript -Version $updateVersion -SkipBuild -OutputDirectory $packageDirectory | Select-Object -Last 1
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $updatePackage -PathType Leaf)) {
        throw "Second version package could not be created: $updatePackage"
    }
    & $installScript -PackagePath $updatePackage -InstallRoot $installRoot | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'Installing the second package version failed.'
    }
    $updatedPointer = Get-PhotoBoothActivePointer -InstallRoot $installRoot
    if ($updatedPointer.active_version -ne $updateVersion -or $updatedPointer.previous_version -ne $version) {
        throw 'Real-binary update did not select the new version and retain the prior version.'
    }
    [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory (Join-Path (Join-Path $installRoot 'releases') $updateVersion) -ExpectedVersion $updateVersion -ExpectedSha256 $updatedPointer.active_sha256)
    $checks.Add('update to a second version using the same real executable') | Out-Null

    Write-Host 'STEP=rollback-real-binary-update'
    & (Join-Path $PSScriptRoot 'Rollback-PhotoBoothWindowsPackage.ps1') -InstallRoot $installRoot | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'Rollback of the real-binary update failed.'
    }
    $pointer = Get-PhotoBoothActivePointer -InstallRoot $installRoot
    if ($pointer.active_version -ne $version -or $pointer.previous_version -ne $updateVersion) {
        throw 'Rollback did not restore the first version as active.'
    }
    $checks.Add('rollback to the first real-binary release and preserve both versions') | Out-Null

    Write-Host 'STEP=verify-active-pointer-and-integrity'
    & $startScript -InstallRoot $installRoot -NoLaunch | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'Installed active pointer or binary integrity verification failed.'
    }
    $releaseDirectory = Join-Path (Join-Path $installRoot 'releases') $version
    $release = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $releaseDirectory -ExpectedVersion $version -ExpectedSha256 $pointer.active_sha256
    $checks.Add('verify active pointer, PE header, size and SHA-256 without launching') | Out-Null

    Write-Host 'STEP=launch-installed-executable-and-test-kiosk'
    $powershellPath = (Get-Process -Id $PID).Path
    if ([string]::IsNullOrWhiteSpace($powershellPath) -or !(Test-Path -LiteralPath $powershellPath -PathType Leaf)) {
        $powershellPath = (Get-Command powershell.exe -ErrorAction Stop).Source
    }

    $arguments = @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', ('"' + $kioskSmoke + '"'),
        '-ExecutablePath', ('"' + $release.Executable + '"'),
        '-StartupTimeoutSeconds', [string]$StartupTimeoutSeconds,
        '-TransitionTimeoutSeconds', [string]$TransitionTimeoutSeconds
    )
    $smokeProcess = Start-Process -FilePath $powershellPath -ArgumentList $arguments -PassThru -Wait -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath

    if (Test-Path -LiteralPath $stdoutPath -PathType Leaf) {
        Get-Content -LiteralPath $stdoutPath | ForEach-Object { Write-Host $_ }
    }
    if (Test-Path -LiteralPath $stderrPath -PathType Leaf) {
        $stderr = Get-Content -LiteralPath $stderrPath -Raw
        if (![string]::IsNullOrWhiteSpace($stderr)) {
            Write-Warning $stderr
        }
    }

    if ($smokeProcess.ExitCode -ne 0) {
        throw "Installed executable kiosk UI smoke failed with exit code $($smokeProcess.ExitCode)."
    }

    $reportLine = Get-Content -LiteralPath $stdoutPath | Where-Object { $_ -match '^REPORT_RETAINED=' } | Select-Object -Last 1
    if (!$reportLine) {
        throw 'Kiosk UI smoke did not report its JSON result path.'
    }
    $smokeReportPath = $reportLine.Substring('REPORT_RETAINED='.Length).Trim()
    if (!(Test-Path -LiteralPath $smokeReportPath -PathType Leaf)) {
        throw "Kiosk UI smoke report is missing: $smokeReportPath"
    }
    $report = Get-Content -LiteralPath $smokeReportPath -Raw | ConvertFrom-Json
    if ($report.status -ne 'PASS' -or !$report.temporary_data_cleaned) {
        throw 'Kiosk UI smoke did not pass or did not clean its isolated player data.'
    }
    foreach ($name in @('fullscreen_on_start','escape_returns_windowed','f11_enters_fullscreen','escape_after_toggle')) {
        if ($report.checks.$name -ne $true) {
            throw "Kiosk UI smoke report did not pass check '$name'."
        }
        $checks.Add("installed executable: $name") | Out-Null
    }

    Write-Host 'STEP=installed-settings-save-and-process-restart'
    $settingsArguments = @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', ('"' + $settingsSmoke + '"'),
        '-ExecutablePath', ('"' + $release.Executable + '"'),
        '-StartupTimeoutSeconds', [string]$StartupTimeoutSeconds,
        '-SettingsTimeoutSeconds', [string]$SettingsTimeoutSeconds
    )
    $settingsProcess = Start-Process -FilePath $powershellPath -ArgumentList $settingsArguments -PassThru -Wait -WindowStyle Hidden -RedirectStandardOutput $settingsStdoutPath -RedirectStandardError $settingsStderrPath

    if (Test-Path -LiteralPath $settingsStdoutPath -PathType Leaf) {
        Get-Content -LiteralPath $settingsStdoutPath | ForEach-Object { Write-Host $_ }
    }
    if (Test-Path -LiteralPath $settingsStderrPath -PathType Leaf) {
        $settingsStderr = Get-Content -LiteralPath $settingsStderrPath -Raw
        if (![string]::IsNullOrWhiteSpace($settingsStderr)) {
            Write-Warning $settingsStderr
        }
    }
    if ($settingsProcess.ExitCode -ne 0) {
        throw "Installed executable settings persistence smoke failed with exit code $($settingsProcess.ExitCode)."
    }

    $settingsReportLine = Get-Content -LiteralPath $settingsStdoutPath | Where-Object { $_ -match '^TEST_REPORT=' } | Select-Object -Last 1
    if (!$settingsReportLine) {
        throw 'Settings persistence smoke did not report its JSON result path.'
    }
    $settingsReportPath = $settingsReportLine.Substring('TEST_REPORT='.Length).Trim()
    if (!(Test-Path -LiteralPath $settingsReportPath -PathType Leaf)) {
        throw "Settings persistence JSON report is missing: $settingsReportPath"
    }
    $settingsReport = Get-Content -LiteralPath $settingsReportPath -Raw | ConvertFrom-Json
    if ($settingsReport.status -ne 'PASS' -or
        !$settingsReport.temporary_profile_cleaned -or
        !$settingsReport.no_photo_session_created -or
        $settingsReport.forced_stop) {
        throw 'Settings persistence smoke did not pass cleanly or did not remove its isolated profile.'
    }
    foreach ($check in $settingsReport.checks) {
        $checks.Add("installed executable settings: $check") | Out-Null
    }

    $status = 'PASS'
    Write-Host "PHOTOBOOTH_INSTALLED_PACKAGE_KIOSK_SMOKE=$status"
    Write-Host "PACKAGE_VERSION=$version"
    Write-Host "INSTALLED_EXECUTABLE=$($release.Executable)"
    Write-Host "EXECUTABLE_SHA256=$($release.Sha256)"
    Write-Host "KIOSK_REPORT=$smokeReportPath"
    Write-Host "SETTINGS_REPORT=$settingsReportPath"
    foreach ($check in $checks) { Write-Host "CHECK=$check" }
}
catch {
    $failure = $_.Exception.Message
    Write-Host 'PHOTOBOOTH_INSTALLED_PACKAGE_KIOSK_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    throw
}
finally {
    $runningApp = @(Get-CimInstance -ClassName Win32_Process -Filter "Name = 'PhotoBooth.exe'" -ErrorAction SilentlyContinue)
    if ($runningApp.Count -gt 0) {
        Write-Warning 'A PhotoBooth.exe process remains. The temporary install root will be preserved; no process will be force-stopped.'
        foreach ($process in $runningApp) {
            Write-Warning "RUNNING_PID=$($process.ProcessId) PATH=$($process.ExecutablePath)"
        }
    }
    else {
        if (Test-Path -LiteralPath $temporaryRoot) {
            Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
        $tempCleaned = !(Test-Path -LiteralPath $temporaryRoot)
    }

    Write-Host "TEMP_INSTALL_AND_PACKAGE_CLEANED=$tempCleaned"
    if ($smokeReportPath) {
        Write-Host "KIOSK_JSON_REPORT_RETAINED=$smokeReportPath"
    }
}
