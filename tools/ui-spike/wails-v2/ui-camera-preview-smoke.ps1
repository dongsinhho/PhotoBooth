[CmdletBinding()]
param(
    [string]$Device = 'B525 HD Webcam',
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(30,120)][int]$SessionTimeoutSeconds = 75,
    [ValidateRange(10,45)][int]$PreviewTimeoutSeconds = 25,
    [string]$ExecutablePath = ''
)

$ErrorActionPreference = 'Stop'
$moduleRoot = $PSScriptRoot
$repoRoot = (Resolve-Path (Join-Path $moduleRoot '..\..\..')).Path
if ([string]::IsNullOrWhiteSpace($ExecutablePath)) {
    $candidate = Join-Path $moduleRoot 'build\bin\PhotoBooth.exe'
    if (!(Test-Path $candidate)) { $candidate = Join-Path $moduleRoot 'build\bin\wails-v2.exe' }
    $exe = $candidate
} else {
    $exe = (Resolve-Path $ExecutablePath).Path
}
if (!(Test-Path $exe)) { throw "Wails executable is missing; build the UI candidate first: $exe" }
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-wails-preview-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$outputRoot = Join-Path $dataRoot 'photos'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$reportPath = Join-Path $env:TEMP "photobooth-wails-preview-$runId.json"
$stdoutPath = Join-Path $env:TEMP "photobooth-wails-preview-$runId.stdout.log"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$app = $null
$status = 'FAIL'
$failure = $null
$previewBefore = $false
$previewAfter = $false
$completedSession = $null
$cameraInstanceId = $null
$captureCount = 0
$rendered = $false
$localOutput = $false
$dataCleaned = $false

function Get-B525 {
    try { return @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction Stop | Where-Object { $_.FriendlyName -like '*B525*' }) }
    catch { return @() }
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

function Get-WindowText {
    $window = Get-Window
    if (!$window) { return @() }
    $values = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)) {
        if ($element.Current.Name) { $values.Add($element.Current.Name) | Out-Null }
    }
    return $values.ToArray()
}

function Wait-Text([string]$Pattern, [string]$Label, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $values = @(Get-WindowText)
        foreach ($value in $values) { if ($value -like $Pattern) { return $value } }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Wait-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Pattern
        if ($button) { return $button }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $button = Wait-Button $Pattern $Label $TimeoutSeconds
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Get-SessionIds {
    if (!(Test-Path $sessionsRoot)) { return @() }
    return @((Get-ChildItem $sessionsRoot -Directory | ForEach-Object { $_.Name } | Sort-Object))
}

function Wait-NewSession([string[]]$Before, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $newIds = @(Get-SessionIds | Where-Object { $Before -notcontains $_ })
        if ($newIds.Count -eq 1) { return $newIds[0] }
        if ($newIds.Count -gt 1) { throw "More than one new session appeared: $($newIds -join ', ')" }
        Start-Sleep -Milliseconds 150
    }
    throw 'No new session directory appeared after starting the Wails UI session.'
}

function Wait-CompletedResult([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Find-Button 'Take another set*') {
            $text = Get-WindowText
            if (($text -join ' ') -match 'COMPLETED') { return $text }
        }
        if (Find-Button 'Reconnect and continue*') { throw "Capture entered camera-recovery UI instead of completing. $((Get-WindowText) -join ' | ')" }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited during capture.' }
        Start-Sleep -Milliseconds 200
    }
    throw "Session did not reach the completed result view. UI text: $((Get-WindowText) -join ' | ')"
}

try {
    $camera = @(Get-B525)
    if ($camera.Count -ne 1) { throw "Expected exactly one present B525 camera, found $($camera.Count)." }
    if ($camera[0].Status -ne 'OK') { throw "B525 is not healthy: $($camera[0].Status)" }
    $cameraInstanceId = $camera[0].InstanceId
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    $cfg = [ordered]@{
        schema_version = 1
        camera = [ordered]@{ provider = 'windowsmf'; device = $Device; width = 1280; height = 720; fps = 30 }
        capture = [ordered]@{ shots = 3; countdown = '3s'; interval = '1s' }
        frame = [ordered]@{ config_path = 'frames/photo-strip-v1.json' }
        storage = [ordered]@{ sessions_dir = 'sessions'; output_dir = 'photos' }
        media_delivery = [ordered]@{ mode = 'local' }
        payment = [ordered]@{ mode = 'disabled' }
    }
    [IO.File]::WriteAllText($configPath, ($cfg | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
    Write-Host "WAILS_CAMERA_PREVIEW_SMOKE=START device=$Device"
    Write-Host 'The smoke verifies idle preview, exclusive handoff to capture, and preview restart after returning to idle.'

    $app = Start-Process -FilePath $exe -WorkingDirectory $moduleRoot -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 200
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited before opening its main window.' }
    }
    if (!$window) { throw 'Wails main window did not appear.' }
    Write-Host 'WAILS_WINDOW=READY'

    [void](Wait-Text '*LIVE CAMERA PREVIEW*' 'first live preview frame' $PreviewTimeoutSeconds)
    $previewBefore = $true
    Write-Host 'PREVIEW_BEFORE_CAPTURE=PASS'

    $before = Get-SessionIds
    Invoke-Button 'Start with webcam*' 'start real camera session while preview is active'
    $completedSession = Wait-NewSession $before 15
    [void](Wait-CompletedResult $SessionTimeoutSeconds)
    Write-Host "CAPTURE_SESSION=$completedSession"
    $sessionDir = Join-Path $sessionsRoot $completedSession
    $captureDir = Join-Path $sessionDir 'captures'
    $captures = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg')
    $captureCount = $captures.Count
    $renderedPath = Join-Path $sessionDir 'rendered.jpg'
    $outputPath = Join-Path (Join-Path $outputRoot $completedSession) 'photo.jpg'
    if ($captureCount -ne 3) { throw "Expected 3 capture JPEGs, found $captureCount." }
    if (!(Test-Path $renderedPath -PathType Leaf)) { throw 'The session did not render rendered.jpg.' }
    if (!(Test-Path $outputPath -PathType Leaf)) { throw 'The session did not save photo.jpg locally.' }
    foreach ($file in $captures) { if ($file.Length -le 0) { throw "Empty capture file: $($file.Name)" } }
    $rendered = $true
    $localOutput = $true
    Write-Host 'CAPTURE_HANDOFF=PASS preview stopped before exclusive capture open'
    Write-Host 'CAPTURE_RENDER_LOCAL_SAVE=PASS 3/3'

    Invoke-Button 'Take another set*' 'return to idle and restart preview'
    [void](Wait-Button 'Start with webcam*' 'idle start button' 10)
    [void](Wait-Text '*LIVE CAMERA PREVIEW*' 'live preview after capture session' $PreviewTimeoutSeconds)
    $previewAfter = $true
    Write-Host 'PREVIEW_AFTER_CAPTURE=PASS'
    $status = 'PASS'
} catch {
    $failure = $_.Exception.Message
    Write-Host 'WAILS_CAMERA_PREVIEW_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    Write-Host "UI_TEXT=$((Get-WindowText) -join ' | ')"
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) { [void]$process.CloseMainWindow(); if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue } }
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    if (Test-Path $dataRoot) { Remove-Item $dataRoot -Recurse -Force -ErrorAction SilentlyContinue }
    $dataCleaned = !(Test-Path $dataRoot)
    $report = [ordered]@{
        created_utc = [DateTime]::UtcNow.ToString('o'); status = $status; failure = $failure
        camera = $Device; camera_instance_id = $cameraInstanceId; preview_before_capture = $previewBefore
        preview_after_capture = $previewAfter; session_id = $completedSession; capture_count = $captureCount
        rendered = $rendered; local_output = $localOutput; temp_data_cleaned = $dataCleaned
        capture_mode = 'Wails UI / Windows Media Foundation / shared worker / serialized preview-capture lifecycle'
        preview_max_fps = 5; preview_ack_timeout_seconds = 2; preview_stop_timeout_seconds = 7
    }
    $report | ConvertTo-Json -Depth 8 | Set-Content -Path $reportPath -Encoding utf8
    Remove-Item $stdoutPath -Force -ErrorAction SilentlyContinue
    Write-Host "TEMP_TEST_DATA_CLEANED=$dataCleaned"
    Write-Host "REPORT_RETAINED=$reportPath"
}

if ($status -ne 'PASS') { exit 1 }
Write-Host 'WAILS_CAMERA_PREVIEW_SMOKE=PASS'
exit 0
