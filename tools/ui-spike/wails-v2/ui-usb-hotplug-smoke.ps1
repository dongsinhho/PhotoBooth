[CmdletBinding()]
param(
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 25,
    [ValidateRange(10,45)][int]$UnplugObservationTimeoutSeconds = 15,
    [ValidateRange(5,45)][int]$ReconnectTimeoutSeconds = 12,
    [ValidateRange(30,120)][int]$SessionTimeoutSeconds = 75
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$exe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'
if (!(Test-Path $exe)) { throw "Wails executable not found: $exe (build it first)" }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-usb-hotplug-smoke-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$outputRoot = Join-Path $dataRoot 'photos'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$reportPath = Join-Path $env:TEMP "photobooth-usb-hotplug-smoke-$runId.json"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$app = $null
$failure = $null
$status = 'FAIL'
$sessionId = $null
$cameraInstanceId = $null
$unplugObserved = $false
$replugObserved = $false
$recoveryEventObserved = $false
$usedContinue = $false
$checks = [System.Collections.Generic.List[string]]::new()
$report = [ordered]@{
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'FAIL'
    camera_name = 'B525 HD Webcam'
    camera_instance_id = $null
    session_id = $null
    usb_unplug_observed = $false
    usb_replug_observed = $false
    recovery_event_observed = $false
    continue_action_used = $false
    capture_count = 0
    checks = @()
    failure = $null
    physical_camera = $true
    data_root = $dataRoot
    report_path = $reportPath
}

function Get-B525 {
    try {
        return @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction Stop |
            Where-Object { $_.FriendlyName -like '*B525*' })
    } catch {
        return @()
    }
}

function Get-Window {
    if (!$app) { return $null }
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $app.Id
    )
    $windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($windows.Count -gt 0) { return $windows.Item(0) }
    return $null
}

function Get-WindowText {
    $window = Get-Window
    if (!$window) { return @() }
    $items = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $window.FindAll(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.Condition]::TrueCondition
    )) {
        if ($element.Current.Name) { $items.Add($element.Current.Name) | Out-Null }
    }
    return $items.ToArray()
}

function Find-Button([string]$Pattern) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    foreach ($button in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)) {
        if ($button.Current.Name -like $Pattern -and $button.Current.IsEnabled) { return $button }
    }
    return $null
}

function Wait-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Pattern
        if ($button) { return $button }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $button = Wait-Button $Pattern $Label $TimeoutSeconds
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Wait-UiText([scriptblock]$Predicate, [int]$TimeoutSeconds, [string]$Label) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        if (& $Predicate $text) { return $text }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 100
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

try {
    $initialCamera = @(Get-B525)
    if ($initialCamera.Count -lt 1) { throw 'B525 HD Webcam is not currently present. Connect it before starting this physical test.' }
    $cameraInstanceId = $initialCamera[0].InstanceId
    $report.camera_instance_id = $cameraInstanceId
    Write-Host "B525_PRESENT=$cameraInstanceId"

    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    $cfg = [ordered]@{
        schema_version = 1
        camera = [ordered]@{ provider = 'windowsmf'; width = 1280; height = 720; fps = 30 }
        capture = [ordered]@{ shots = 3; countdown = '8s'; interval = '2s' }
        frame = [ordered]@{ config_path = 'frames/photo-strip-v1.json' }
        storage = [ordered]@{ sessions_dir = 'sessions'; output_dir = 'photos' }
        media_delivery = [ordered]@{ mode = 'local' }
        payment = [ordered]@{ mode = 'disabled' }
    }
    [IO.File]::WriteAllText($configPath, ($cfg | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')

    Write-Host 'WAILS_USB_HOTPLUG_SMOKE=START (physical B525; isolated temporary profile)'
    Write-Host 'This test captures real images locally in the temporary profile; the script deletes that profile after the run.'
    $app = Start-Process -FilePath $exe -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 200
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited before its window appeared' }
    }
    if (!$window) { throw 'Wails main window did not appear' }
    $checks.Add('isolated Wails window opened') | Out-Null

    Invoke-Button 'Start with webcam*' 'start real B525 session'
    $deadline = (Get-Date).AddSeconds(15)
    $sessionId = $null
    while ((Get-Date) -lt $deadline) {
        $dirs = @(Get-ChildItem $sessionsRoot -Directory -ErrorAction SilentlyContinue)
        if ($dirs.Count -eq 1) { $sessionId = $dirs[0].Name; break }
        Start-Sleep -Milliseconds 100
    }
    if (!$sessionId -or $sessionId -notmatch '^[0-9a-f]{32}$') { throw "Could not identify the isolated B525 session. Session count=$(@(Get-ChildItem $sessionsRoot -Directory -ErrorAction SilentlyContinue).Count)" }
    $report.session_id = $sessionId
    $checks.Add('real B525 session started') | Out-Null
    Write-Host "SESSION_ID=$sessionId"

    # Wait until the first shot is on disk and the second countdown begins.
    # The camera remains open between shots. Pulling the USB cable at this
    # point interrupts an active session and the next frame read exercises
    # the real provider/recovery boundary.
    $captureOne = Join-Path (Join-Path $sessionsRoot $sessionId) 'captures\capture-01.jpg'
    $deadline = (Get-Date).AddSeconds(50)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        if (Test-Path $captureOne) {
            if (($text -join ' ') -match 'Shot 2 of 3 in') { break }
        }
        if (($text -join ' ') -match 'SESSION PAUSED|We lost the camera') { throw "Camera failed before the intentional unplug. UI text: $($text -join ' | ')" }
        Start-Sleep -Milliseconds 100
    }
    if (!(Test-Path $captureOne)) { throw 'The first real B525 capture did not complete within 50 seconds' }
    $checks.Add('first B525 capture completed; second capture countdown started') | Out-Null

    Write-Host ''
    Write-Host '=============================================================='
    Write-Host 'PHYSICAL ACTION: UNPLUG THE B525 USB CABLE NOW.'
    Write-Host 'Leave it unplugged until this process prints RECONNECT_B525_NOW.'
    Write-Host '=============================================================='
    $unplugDeadline = (Get-Date).AddSeconds($UnplugObservationTimeoutSeconds)
    while ((Get-Date) -lt $unplugDeadline) {
        if (@(Get-B525).Count -eq 0) {
            $unplugObserved = $true
            $report.usb_unplug_observed = $true
            $checks.Add('Windows PnP observed the B525 disappear') | Out-Null
            Write-Host 'USB_UNPLUG_OBSERVED=True'
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (!$unplugObserved) {
        throw "Windows did not observe the B525 disappear within $UnplugObservationTimeoutSeconds seconds. Do not report this as a hot-unplug test."
    }

    # Wait until the camera provider surfaces its bounded recovery attempt. Then
    # tell the operator to reconnect, so we know the active path actually saw it.
    $recoveryDeadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $recoveryDeadline) {
        $text = Get-WindowText
        $allText = $text -join ' '
        if ($allText -match 'Waiting for the camera to return|camera disconnected|Camera recovery timed out') {
            $recoveryEventObserved = $true
            $report.recovery_event_observed = $true
            break
        }
        if (Find-Button 'Take another set*') { break }
        Start-Sleep -Milliseconds 100
    }
    Write-Host ''
    Write-Host '=============================================================='
    Write-Host 'RECONNECT_B525_NOW: plug the B525 USB cable back in now.'
    Write-Host 'Keep it connected after Windows detects it.'
    Write-Host '=============================================================='

    $reconnectDeadline = (Get-Date).AddSeconds($ReconnectTimeoutSeconds)
    while ((Get-Date) -lt $reconnectDeadline) {
        if (@(Get-B525).Count -gt 0) {
            $replugObserved = $true
            $report.usb_replug_observed = $true
            $checks.Add('Windows PnP observed the B525 return') | Out-Null
            Write-Host 'USB_REPLUG_OBSERVED=True'
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (!$replugObserved) { throw "Windows did not observe the B525 return within $ReconnectTimeoutSeconds seconds" }

    # Track any surfaced recovery message while waiting for either automatic
    # continuation or an explicit retryable failed state.
    $outcome = $null
    $deadline = (Get-Date).AddSeconds($SessionTimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        $joined = $text -join ' '
        if ($joined -match 'Camera recovery succeeded|Camera reconnected') {
            $recoveryEventObserved = $true
            $report.recovery_event_observed = $true
        }
        if (Find-Button 'Take another set*') {
            $outcome = 'completed_automatically'
            break
        }
        if ((Find-Button 'Reconnect and continue remaining shots*') -or (Find-Button 'Retry camera connection*')) {
            $outcome = 'retryable_failure'
            break
        }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited while waiting for camera recovery/session result' }
        Start-Sleep -Milliseconds 150
    }
    if (!$outcome) { throw "No completed or retryable outcome appeared after reconnect. UI text: $((Get-WindowText) -join ' | ')" }

    if ($outcome -eq 'retryable_failure') {
        $continue = Find-Button 'Reconnect and continue remaining shots*'
        if (!$continue) { $continue = Find-Button 'Retry camera connection*' }
        if (!$continue) { throw 'A retryable failure was reported but no enabled Continue action was available' }
        $continue.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
        $usedContinue = $true
        $report.continue_action_used = $true
        $checks.Add('retryable failure preserved the session and exposed Continue') | Out-Null
        Write-Host 'UI_ACTION=continue_same_session_after_replug'
        [void](Wait-Button 'Take another set*' 'completed B525 result' $SessionTimeoutSeconds)
    }

    $sessionDir = Join-Path $sessionsRoot $sessionId
    $captureDir = Join-Path $sessionDir 'captures'
    $captures = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg')
    $rendered = Join-Path $sessionDir 'rendered.jpg'
    $output = Join-Path (Join-Path $outputRoot $sessionId) 'photo.jpg'
    if ($captures.Count -ne 3) { throw "Expected 3 B525 captures after recovery, found $($captures.Count)" }
    if (!(Test-Path $rendered -PathType Leaf)) { throw 'Rendered image missing after B525 recovery' }
    if (!(Test-Path $output -PathType Leaf)) { throw 'Final local output missing after B525 recovery' }
    if ((Get-Item $output).Length -le 0) { throw 'Final local output is empty after B525 recovery' }
    $checks.Add('all three B525 captures, rendered image and local photo completed') | Out-Null
    if ($recoveryEventObserved) { $checks.Add('Wails UI surfaced camera recovery/reconnection state') | Out-Null }
    if ($usedContinue) { $checks.Add('same-session Continue completed after replug') | Out-Null }

    $report.capture_count = $captures.Count
    $report.output_bytes = (Get-Item $output).Length
    $report.output_path = $output
    $report.recovery_event_observed = $recoveryEventObserved
    $report.status = 'PASS'
    $status = 'PASS'
    Write-Host "WAILS_USB_HOTPLUG_SMOKE=PASS outcome=$outcome recovery_event=$recoveryEventObserved continue=$usedContinue"
} catch {
    $failure = $_.Exception.Message
    $report.failure = $failure
    Write-Host 'WAILS_USB_HOTPLUG_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    if ($sessionId) { Write-Host "DEBUG_SESSION=$sessionId" }
    Write-Host "DEBUG_UI_TEXT=$((Get-WindowText) -join ' | ')"
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
        }
    }
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report.checks = @($checks)
    $report.status = $status
    $report.failure = $failure
    $report | ConvertTo-Json -Depth 8 | Set-Content -Path $reportPath -Encoding utf8
    if (Test-Path $dataRoot) { Remove-Item $dataRoot -Recurse -Force -ErrorAction SilentlyContinue }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    Write-Host "TEST_REPORT=$reportPath"
    Write-Host "TEMP_TEST_DATA_CLEANED=$(!(Test-Path $dataRoot))"
}

if ($status -ne 'PASS') { exit 1 }
exit 0
