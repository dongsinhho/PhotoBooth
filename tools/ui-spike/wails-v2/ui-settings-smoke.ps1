[CmdletBinding()]
param(
    [ValidateRange(5,45)][int]$StartupTimeoutSeconds = 20
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$exe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'
if (!(Test-Path $exe)) {
    throw "Wails executable not found: $exe (run wails build -clean -nopackage first)"
}

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$dataRoot = Join-Path $env:TEMP ("photobooth-settings-smoke-" + [guid]::NewGuid().ToString('N'))
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
[Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$defaultFramePath = Join-Path $dataRoot 'frames\photo-strip-v1.json'
$profileExistedBefore = Test-Path $configPath
$frameExistedBefore = Test-Path $defaultFramePath
$configBefore = if ($profileExistedBefore) { [IO.File]::ReadAllBytes($configPath) } else { $null }

$app = $null
$status = 'FAIL'
$failure = $null
$verified = [System.Collections.Generic.List[string]]::new()

function Get-Window {
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $app.Id
    )
    $windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($windows.Count -gt 0) { return $windows.Item(0) }
    return $null
}

function Find-ByAutomationID([string]$AutomationID) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::AutomationIdProperty, $AutomationID
    )
    return $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
}

function Find-Button([string]$NamePattern) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    foreach ($button in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)) {
        if ($button.Current.Name -like $NamePattern -and $button.Current.IsEnabled) { return $button }
    }
    return $null
}

function Wait-Button([string]$NamePattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $NamePattern
        if ($button) { return $button }
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$process) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 250
    }
    $names = @()
    $window = Get-Window
    if ($window) {
        $condition = New-Object System.Windows.Automation.PropertyCondition(
            [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
            [System.Windows.Automation.ControlType]::Button
        )
        $names = @($window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition) | ForEach-Object { $_.Current.Name })
    }
    throw "Could not find enabled button '$Label' (pattern $NamePattern). Accessible buttons: $($names -join ' | ')"
}

function Invoke-Button([string]$NamePattern, [string]$Label) {
    $button = Wait-Button $NamePattern $Label
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
}

function Wait-Text([string]$ExpectedText, [int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $window = Get-Window
        if ($window) {
            $condition = New-Object System.Windows.Automation.PropertyCondition(
                [System.Windows.Automation.AutomationElement]::NameProperty,
                $ExpectedText
            )
            $element = $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
            if ($element) { return $element }
        }
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$process) { throw "Wails app exited while waiting for UI text '$ExpectedText'" }
        Start-Sleep -Milliseconds 150
    }
    $visible = @()
    $window = Get-Window
    if ($window) {
        $visible = @($window.FindAll(
            [System.Windows.Automation.TreeScope]::Descendants,
            [System.Windows.Automation.Condition]::TrueCondition
        ) | ForEach-Object { $_.Current.Name } | Where-Object { $_ })
    }
    throw "Timed out waiting for UI text '$ExpectedText'. Current UI: $($visible -join ' | ')"
}

try {
    Write-Host 'WAILS_SETTINGS_SMOKE=START (no camera capture; no Save action)'
    $app = Start-Process -FilePath $exe -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 250
        $window = Get-Window
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$process) { throw 'Wails app exited before its main window appeared' }
    }
    if (!$window) { throw "Wails main window did not appear within $StartupTimeoutSeconds seconds" }
    $verified.Add('main window opened') | Out-Null

    Invoke-Button 'Settings & frame*' 'Settings & frame'
    [void](Wait-Text 'OPERATOR CONFIGURATION' 15)
    [void](Wait-Text 'Frame packages' 15)

    $expected = @(
        @{ id = 'settings-shot-count'; type = [System.Windows.Automation.ControlType]::ComboBox; label = 'shot-count selector' },
        @{ id = 'settings-countdown'; type = [System.Windows.Automation.ControlType]::Edit; label = 'countdown field' },
        @{ id = 'settings-interval'; type = [System.Windows.Automation.ControlType]::Edit; label = 'interval field' },
        @{ id = 'settings-frame-path'; type = [System.Windows.Automation.ControlType]::Edit; label = 'frame path field' }
    )
    foreach ($item in $expected) {
        $element = Find-ByAutomationID $item.id
        if (!$element) { throw "Settings UI missing $($item.label) (AutomationId=$($item.id))" }
        if ($element.Current.ControlType -ne $item.type) { throw "$($item.label) has unexpected control type $($element.Current.ControlType.ProgrammaticName)" }
        try { $value = $element.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value }
        catch { $value = $element.Current.Name }
        if ([string]::IsNullOrWhiteSpace($value)) { throw "$($item.label) has no accessible current value" }
        Write-Host ("SETTINGS_FIELD=" + $item.id + " VALUE=" + $value)
        $verified.Add($item.label) | Out-Null
    }

    foreach ($pattern in @('Browse*','Save settings*','Cancel','Validate package*','Refresh')) {
        if (!(Find-Button $pattern)) { throw "Settings UI missing enabled button pattern '$pattern'" }
    }
    foreach ($id in @('frame-package-path','install-frame-package','rollback-frame','frame-package-list')) {
        if (!(Find-ByAutomationID $id)) { throw "Frame-package UI missing control (AutomationId=$id)" }
    }
    $verified.Add('Browse, Save, Cancel, package validation and refresh controls exposed') | Out-Null

    $allText = @()
    foreach ($element in (Get-Window).FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)) {
        if ($element.Current.Name) { $allText += $element.Current.Name }
    }
    if (!($allText -match 'OPERATOR CONFIGURATION')) { throw 'Settings heading is not exposed to UI Automation' }
    if (!($allText -match 'store-config-v1\.json')) { throw 'Local configuration path is not shown in the settings view' }
    if (!($allText -match 'Frame packages')) { throw 'Local frame-package manager heading is missing' }
    if (!($allText -match 'Installed frames')) { throw 'Installed frame-package list heading is missing' }
    $verified.Add('settings heading, config path and local package library displayed') | Out-Null

    Invoke-Button 'Cancel' 'Cancel settings'
    Start-Sleep -Milliseconds 300
    if (!(Find-Button 'Settings & frame*')) { throw 'Cancel did not return to the home screen' }
    $verified.Add('Cancel returns home without saving') | Out-Null

    if ($profileExistedBefore) {
        $configAfter = [IO.File]::ReadAllBytes($configPath)
        if ([Convert]::ToBase64String($configAfter) -ne [Convert]::ToBase64String($configBefore)) {
            throw 'The existing station configuration changed even though Save was not clicked'
        }
        $verified.Add('existing configuration unchanged') | Out-Null
    }

    $status = 'PASS'
    Write-Host ('WAILS_SETTINGS_SMOKE=PASS checks=' + $verified.Count)
} catch {
    $failure = $_.Exception.Message
    Write-Host 'WAILS_SETTINGS_SMOKE=FAIL'
    Write-Host ('FAILURE=' + $failure)
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
        }
    }
    # The smoke uses a unique PHOTOBOOTH_DATA_DIR; it never touches the operator's real profile.
    if (Test-Path $dataRoot) {
        Remove-Item $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
}
if ($status -ne 'PASS') { exit 1 }
exit 0
