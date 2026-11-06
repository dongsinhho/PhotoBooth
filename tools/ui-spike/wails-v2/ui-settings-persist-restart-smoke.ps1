[CmdletBinding()]
param(
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(5,45)][int]$SettingsTimeoutSeconds = 20,
    [string]$ExecutablePath
)

$ErrorActionPreference = 'Stop'
$moduleRoot = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ExecutablePath)) {
    $exe = Join-Path $moduleRoot 'build\bin\PhotoBooth.exe'
} else {
    $exe = (Resolve-Path -LiteralPath $ExecutablePath -ErrorAction Stop).Path
}
if (!(Test-Path -LiteralPath $exe -PathType Leaf)) {
    throw "PhotoBooth executable was not found: $exe"
}
$exeWorkingDirectory = Split-Path -Parent $exe

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-settings-restart-$runId"
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$sessionsRoot = Join-Path $dataRoot 'sessions'
$reportPath = Join-Path $env:TEMP "photobooth-settings-restart-$runId.json"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR','Process')
$app = $null
$status = 'FAIL'
$failure = $null
$forcedStop = $false
$dataCleaned = $false
$checks = [System.Collections.Generic.List[string]]::new()
$startedAt = [DateTime]::UtcNow
$firstRunValues = $null
$restartValues = $null

function Get-Window {
    if (!$app) { return $null }
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty,[int]$app.Id
    )
    $windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children,$condition)
    if ($windows.Count -gt 0) { return $windows.Item(0) }
    return $null
}

function Get-WindowText {
    $window = Get-Window
    if (!$window) { return @() }
    return @($window.FindAll(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.Condition]::TrueCondition
    ) | ForEach-Object { $_.Current.Name } | Where-Object { $_ })
}

function Find-ByAutomationID([string]$Id) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::AutomationIdProperty,$Id
    )
    return $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition)
}

function Find-Button([string]$Pattern) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    foreach ($button in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants,$condition)) {
        if ($button.Current.Name -like $Pattern -and $button.Current.IsEnabled) { return $button }
    }
    return $null
}

function Wait-Button([string]$Pattern,[string]$Label,[int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Pattern
        if ($button) { return $button }
        if (!$app -or !(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "PhotoBooth exited while waiting for '$Label'."
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for button '$Label'. UI: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern,[string]$Label) {
    $button = Wait-Button $Pattern $Label
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Wait-Text([string]$ExpectedText,[int]$TimeoutSeconds = $SettingsTimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $window = Get-Window
        if ($window) {
            $condition = New-Object System.Windows.Automation.PropertyCondition(
                [System.Windows.Automation.AutomationElement]::NameProperty,$ExpectedText
            )
            if ($window.FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition)) { return }
        }
        if (!$app -or !(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "PhotoBooth exited while waiting for UI text '$ExpectedText'."
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for UI text '$ExpectedText'. UI: $((Get-WindowText) -join ' | ')"
}

function Read-Value([string]$Id) {
    $element = Find-ByAutomationID $Id
    if (!$element) { throw "Settings field '$Id' is missing from the accessibility tree." }
    return [string]$element.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value
}

function Set-Value([string]$Id,[string]$Value) {
    $element = Find-ByAutomationID $Id
    if (!$element) { throw "Settings field '$Id' is missing from the accessibility tree." }
    $element.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($Value)
}

function Start-PhotoBooth {
    $script:app = Start-Process -FilePath $exe -WorkingDirectory $exeWorkingDirectory -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-Window) {
            Wait-Text 'GO BINDING READY' $StartupTimeoutSeconds
            $checks.Add('PhotoBooth main window and Go bridge ready') | Out-Null
            return
        }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw 'PhotoBooth exited before its main window appeared.'
        }
        Start-Sleep -Milliseconds 150
    }
    throw "PhotoBooth main window did not appear within $StartupTimeoutSeconds seconds."
}

function Open-Settings {
    Invoke-Button 'Settings & frame*' 'open Settings & frame'
    Wait-Text 'OPERATOR CONFIGURATION' $SettingsTimeoutSeconds
    Wait-Text 'Frame packages' $SettingsTimeoutSeconds
    Wait-Text 'Review old local data' $SettingsTimeoutSeconds
    $checks.Add('Settings view fully loaded after preview stop barrier') | Out-Null
}

function Close-PhotoBooth {
    if (!$app) { return }
    $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
    if (!$process) { return }
    [void]$process.CloseMainWindow()
    if (!$process.WaitForExit(7000)) {
        $script:forcedStop = $true
        Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
    }
    $script:app = $null
}

try {
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR',$dataRoot,'Process')
    Write-Host 'WAILS_SETTINGS_PERSIST_RESTART=START (no photo session/capture)'
    Write-Host "TEMP_PROFILE=$dataRoot"

    # First process: edit two app-level timing settings and persist them.
    Start-PhotoBooth
    Open-Settings
    $shotCount = Read-Value 'settings-shot-count'
    $sessionRetentionDefault = Read-Value 'settings-session-retention'
    $outputRetentionDefault = Read-Value 'settings-output-retention'
    if ($shotCount -notmatch '3') { throw "Expected default three-shot setting, found '$shotCount'." }
    if ($sessionRetentionDefault -ne '0' -or $outputRetentionDefault -ne '0') {
        throw "Retention must default to disabled (0/0), found sessions='$sessionRetentionDefault' outputs='$outputRetentionDefault'."
    }
    Set-Value 'settings-countdown' '4s'
    Set-Value 'settings-interval' '750ms'
    Set-Value 'settings-session-retention' '30'
    Set-Value 'settings-output-retention' '45'
    $countdownBeforeSave = Read-Value 'settings-countdown'
    $intervalBeforeSave = Read-Value 'settings-interval'
    $sessionRetentionBeforeSave = Read-Value 'settings-session-retention'
    $outputRetentionBeforeSave = Read-Value 'settings-output-retention'
    Write-Host ("UIA_VALUES_BEFORE_SAVE countdown={0} interval={1} session_retention_days={2} output_retention_days={3}" -f $countdownBeforeSave,$intervalBeforeSave,$sessionRetentionBeforeSave,$outputRetentionBeforeSave)
    Invoke-Button 'Save settings*' 'save local station settings'
    $deadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $deadline -and !(Test-Path -LiteralPath $configPath -PathType Leaf)) {
        Start-Sleep -Milliseconds 100
    }
    if (!(Test-Path -LiteralPath $configPath -PathType Leaf)) {
        throw 'Save settings did not persist store-config-v1.json.'
    }
    $persisted = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
    if ([int]$persisted.capture.shots -ne 3 -or
        [string]$persisted.capture.countdown -ne '4s' -or
        [string]$persisted.capture.interval -ne '750ms' -or
        [int]$persisted.storage.session_retention_days -ne 30 -or
        [int]$persisted.storage.output_retention_days -ne 45) {
        Write-Host ('PERSISTED_CONFIG=' + ($persisted | ConvertTo-Json -Depth 8 -Compress))
        throw ("Persisted config mismatch. UIA before save countdown='{0}' interval='{1}'." -f $countdownBeforeSave,$intervalBeforeSave)
    }
    $firstRunValues = [ordered]@{
        shot_count = [int]$persisted.capture.shots
        countdown = [string]$persisted.capture.countdown
        interval = [string]$persisted.capture.interval
        session_retention_days = [int]$persisted.storage.session_retention_days
        output_retention_days = [int]$persisted.storage.output_retention_days
    }
    $checks.Add('Save settings persisted shot count, countdown and interval to local schema-v1 config') | Out-Null
    Write-Host 'SETTINGS_SAVE_PERSISTED=True countdown=4s interval=750ms shots=3 session_retention_days=30 output_retention_days=45'
    Invoke-Button 'Preview cleanup' 'run read-only retention preview'
    Wait-Text 'No expired PhotoBooth data matched the enabled policies.' $SettingsTimeoutSeconds
    if (Find-Button 'Apply reviewed cleanup*') {
        throw 'Apply cleanup must remain disabled when the preview contains zero candidates.'
    }
    $checks.Add('read-only retention preview renders and disables Apply when no candidates exist') | Out-Null
    Close-PhotoBooth
    if ($forcedStop) { throw 'First PhotoBooth process required forced termination; restart persistence is not considered clean.' }
    $checks.Add('first PhotoBooth process exited gracefully') | Out-Null

    # Second process: load the same station profile and assert the saved values
    # are visible in the settings controls after a real process restart.
    Start-PhotoBooth
    Open-Settings
    $restartValues = [ordered]@{
        shot_count = Read-Value 'settings-shot-count'
        countdown = Read-Value 'settings-countdown'
        interval = Read-Value 'settings-interval'
        session_retention_days = Read-Value 'settings-session-retention'
        output_retention_days = Read-Value 'settings-output-retention'
    }
    if ($restartValues.shot_count -notmatch '3' -or
        $restartValues.countdown -ne '4s' -or
        $restartValues.interval -ne '750ms' -or
        $restartValues.session_retention_days -ne '30' -or
        $restartValues.output_retention_days -ne '45') {
        throw ("Settings did not survive app restart. Values: {0}" -f ($restartValues | ConvertTo-Json -Compress))
    }
    $checks.Add('second PhotoBooth process reloaded saved settings after restart') | Out-Null
    if (!(Test-Path -LiteralPath $configPath -PathType Leaf)) { throw 'Station config disappeared after restart.' }
    $sessionDirs = @(Get-ChildItem -LiteralPath $sessionsRoot -Directory -ErrorAction SilentlyContinue)
    if ($sessionDirs.Count -ne 0) { throw "Unexpected photo session directories were created: $($sessionDirs.Count)" }
    $checks.Add('no photo session/capture was created by this smoke') | Out-Null
    Close-PhotoBooth
    if ($forcedStop) { throw 'Second PhotoBooth process required forced termination.' }
    $status = 'PASS'
    Write-Host 'WAILS_SETTINGS_PERSIST_RESTART=PASS'
} catch {
    $failure = $_.Exception.Message
    Write-Host 'WAILS_SETTINGS_PERSIST_RESTART=FAIL'
    Write-Host "FAILURE=$failure"
    Write-Host "UI_TEXT=$((Get-WindowText) -join ' | ')"
} finally {
    Close-PhotoBooth
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR',$previousDataRoot,'Process')
    if (Test-Path -LiteralPath $dataRoot) {
        Remove-Item -LiteralPath $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    $dataCleaned = !(Test-Path -LiteralPath $dataRoot)
    $report = [ordered]@{
        started_utc = $startedAt.ToString('o')
        finished_utc = [DateTime]::UtcNow.ToString('o')
        status = $status
        failure = $failure
        first_run_values = $firstRunValues
        restart_values = $restartValues
        checks = @($checks)
        forced_stop = $forcedStop
        temporary_profile_cleaned = $dataCleaned
        no_photo_session_created = (!(Test-Path -LiteralPath $sessionsRoot) -or @(Get-ChildItem -LiteralPath $sessionsRoot -Directory -ErrorAction SilentlyContinue).Count -eq 0)
        report_path = $reportPath
    }
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host "TEMP_PROFILE_CLEANED=$dataCleaned"
    Write-Host "TEST_REPORT=$reportPath"
}
if ($status -ne 'PASS') { exit 1 }
exit 0
