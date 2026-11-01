[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PackagePath,
    [ValidateRange(5,45)][int]$StartupTimeoutSeconds = 20
)

$ErrorActionPreference = 'Stop'
$exe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'
if (!(Test-Path $exe)) { throw "Wails executable not found: $exe (build it first)" }
$PackagePath = (Resolve-Path $PackagePath).Path
if ([IO.Path]::GetExtension($PackagePath) -ne '.pbframe') { throw 'PackagePath must point to a .pbframe archive' }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.IO.Compression.FileSystem

$dataRoot = Join-Path $env:TEMP ("photobooth-frame-package-smoke-" + [guid]::NewGuid().ToString('N'))
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
[Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
$app = $null
$status = 'FAIL'
$verified = [System.Collections.Generic.List[string]]::new()
$manifest = $null

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

function Get-WindowText {
    $window = Get-Window
    if (!$window) { return @() }
    $items = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)) {
        if ($element.Current.Name) { $items.Add($element.Current.Name) | Out-Null }
    }
    return $items.ToArray()
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

function Wait-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Pattern
        if ($button) { return $button }
        $proc = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$proc) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for enabled button '$Label'. Controls: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label) {
    $button = Wait-Button $Pattern $Label
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
}

function Wait-Text([string]$Pattern, [int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        if ($text | Where-Object { $_ -match $Pattern }) { return $text }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for text '$Pattern'. Current controls: $((Get-WindowText) -join ' | ')"
}

function Confirm-WebDialog {
    # Try to accept the visible confirmation dialog, logging what Windows exposes
    # so a failed automation attempt is diagnosable instead of silently ignored.
    Start-Sleep -Milliseconds 500
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $allWindows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
    $buttonCondition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    $clicked = $false
    foreach ($win in $allWindows) {
        $windowName = $win.Current.Name
        $buttons = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, $buttonCondition)
        $labels = @($buttons | ForEach-Object { $_.Current.Name })
        foreach ($button in $buttons) {
            if ($button.Current.Name -match '^(OK|Yes)$' -and $button.Current.IsEnabled) {
                Write-Host ('CONFIRM_ACCEPT=' + $button.Current.Name + ' WINDOW=[' + $windowName + ']')
                $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
                $clicked = $true
                break
            }
        }
        if ($clicked) { break }
    }
    if (!$clicked) {
        Write-Host 'CONFIRM_DIALOG_BUTTON_NOT_FOUND; trying Enter on current focus'
        [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
    }
    Start-Sleep -Milliseconds 500
}

function Get-SelectedShotCount {
    $combo = Find-ByAutomationID 'settings-shot-count'
    if (!$combo) { throw 'Shot-count selector is missing' }
    try {
        $value = $combo.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value
        if ($value -match '(\d+)') { return [int]$Matches[1] }
    } catch {}
    try {
        $selection = $combo.GetCurrentPattern([System.Windows.Automation.SelectionPattern]::Pattern).Current.GetSelection()
        if ($selection.Count -gt 0 -and $selection[0].Current.Name -match '^\s*(\d+)') { return [int]$Matches[1] }
    } catch {}
    throw "Could not read selected shot count from UI automation. Control name: $($combo.Current.Name)"
}

try {
    $archive = [System.IO.Compression.ZipFile]::OpenRead($PackagePath)
    try {
        $entry = $archive.GetEntry('manifest.json')
        if (!$entry) { throw 'Fixture archive has no manifest.json' }
        $reader = New-Object System.IO.StreamReader($entry.Open())
        try { $manifest = $reader.ReadToEnd() | ConvertFrom-Json } finally { $reader.Dispose() }
    } finally { $archive.Dispose() }
    if (!$manifest.package_id -or !$manifest.package_version -or !$manifest.name) { throw 'Fixture manifest has incomplete identity fields' }

    Write-Host 'WAILS_FRAME_PACKAGE_SMOKE=START (isolated profile; no camera capture)'
    $app = Start-Process -FilePath $exe -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 250
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails exited before its window appeared' }
    }
    if (!$window) { throw 'Wails main window did not appear' }
    $verified.Add('app opened using isolated data root') | Out-Null

    Invoke-Button 'Settings & frame*' 'Settings & frame'
    [void](Wait-Text 'Frame packages')
    $verified.Add('settings and frame package manager opened') | Out-Null

    $packageInput = Find-ByAutomationID 'frame-package-path'
    if (!$packageInput) { throw 'Frame-package path field missing' }
    $packageInput.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($PackagePath)
    Invoke-Button 'Validate package*' 'Validate package'
    [void](Wait-Text ([regex]::Escape($manifest.name)))
    if (!(Find-Button 'Install locally*')) { throw 'Install button did not become enabled after successful validation' }
    $verified.Add('package validated before install') | Out-Null

    Invoke-Button 'Install locally*' 'Install locally'
    [void](Wait-Text ([regex]::Escape($manifest.name)))
    $installedDir = Join-Path $dataRoot ("frames\packages\" + $manifest.package_id + "\" + $manifest.package_version)
    $installedFrame = Join-Path $installedDir 'definition.json'
    $installedManifest = Join-Path $installedDir 'manifest.json'
    $installDeadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $installDeadline -and (!(Test-Path $installedFrame) -or !(Test-Path $installedManifest))) { Start-Sleep -Milliseconds 200 }
    if (!(Test-Path $installedFrame) -or !(Test-Path $installedManifest)) { throw 'Package files did not appear in the managed local folder after install' }
    $verified.Add('package installed to immutable local version directory') | Out-Null

    Invoke-Button 'Activate' 'Activate frame package'
    Confirm-WebDialog
    $profileAfterActivationPath = Join-Path $dataRoot 'store-config-v1.json'
    if (!(Test-Path $profileAfterActivationPath)) { throw 'Activation did not persist a station profile' }
    $profileAfterActivation = Get-Content $profileAfterActivationPath -Raw | ConvertFrom-Json
    if ([int]$profileAfterActivation.capture.shots -ne @((Get-Content $installedFrame -Raw | ConvertFrom-Json).slots).Count) {
        throw 'Activation did not persist the frame slot count'
    }
    Write-Host ('ACTIVATED_PROFILE_SHOTS=' + $profileAfterActivation.capture.shots)
    [void](Wait-Text '^ACTIVE$')
    # Slot count comes from the installed declarative definition, not a manifest claim.
    $rawFrame = Get-Content $installedFrame -Raw | ConvertFrom-Json
    $expectedSlots = @($rawFrame.slots).Count
    if ((Get-SelectedShotCount) -ne $expectedSlots) {
        throw "Activation selected $((Get-SelectedShotCount)) shots but frame contains $expectedSlots slots"
    }
    if (!(Find-Button 'Roll back frame*')) { throw 'Rollback did not become available after activation' }
    $verified.Add('package activation updated selected shot count and enabled rollback') | Out-Null

    Invoke-Button 'Roll back frame*' 'Roll back frame'
    Confirm-WebDialog
    $deadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $deadline) {
        if ((Get-SelectedShotCount) -eq 3 -and !(Find-Button 'Roll back frame*')) { break }
        Start-Sleep -Milliseconds 200
    }
    if ((Get-SelectedShotCount) -ne 3) { throw 'Rollback did not restore the default three-shot setting' }
    if (Find-Button 'Roll back frame*') { throw 'Rollback stayed enabled after restoring the prior frame' }
    if (!(Test-Path $installedFrame)) { throw 'Rollback deleted the installed package' }
    $config = Get-Content (Join-Path $dataRoot 'store-config-v1.json') -Raw | ConvertFrom-Json
    if ([int]$config.capture.shots -ne 3) { throw "Persisted profile contains $($config.capture.shots) shots after rollback" }
    $restoredFrame = [string]$config.frame.config_path
    if ([string]::IsNullOrWhiteSpace($restoredFrame)) { throw 'Rollback did not persist a restored frame path' }
    $restoredFramePath = if ([IO.Path]::IsPathRooted($restoredFrame)) { $restoredFrame } else { Join-Path $dataRoot $restoredFrame }
    if (!(Test-Path $restoredFramePath)) { throw "Restored frame file does not exist: $restoredFramePath" }
    $verified.Add('rollback restored prior settings, kept installed package and cleared rollback action') | Out-Null

    # An inactive package can only be removed after rollback no longer references it.
    Invoke-Button 'Remove' 'Remove installed frame package'
    Confirm-WebDialog
    [void](Wait-Text 'No installed frame packages yet')
    if (Test-Path $installedDir) { throw 'Uninstall left the immutable package version directory behind' }
    if (Test-Path $installedFrame) { throw 'Uninstall left definition.json behind' }
    $verified.Add('uninstall removed an inactive package after rollback was cleared') | Out-Null

    Invoke-Button 'Cancel' 'Cancel settings'
    [void](Wait-Text 'Settings & frame')
    $status = 'PASS'
    Write-Host ('WAILS_FRAME_PACKAGE_SMOKE=PASS checks=' + $verified.Count)
} catch {
    Write-Host 'WAILS_FRAME_PACKAGE_SMOKE=FAIL'
    Write-Host ('FAILURE=' + $_.Exception.Message)
    Write-Host ('VISIBLE_CONTROLS=' + ((Get-WindowText) -join ' | '))
} finally {
    if ($app) {
        $proc = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($proc) {
            [void]$proc.CloseMainWindow()
            if (!$proc.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
        }
    }
    if (Test-Path $dataRoot) { Remove-Item $dataRoot -Recurse -Force -ErrorAction SilentlyContinue }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
}
if ($status -ne 'PASS') { exit 1 }
exit 0
