[CmdletBinding()]
param(
    [ValidateRange(15,90)][int]$StartupTimeoutSeconds = 25,
    [ValidateRange(30,120)][int]$SessionTimeoutSeconds = 70
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$exe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'
if (!(Test-Path $exe)) { throw "Wails executable not found: $exe (build it first)" }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-output-retry-smoke-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$outputRoot = Join-Path $dataRoot 'photos'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$reportPath = Join-Path $env:TEMP "photobooth-output-retry-smoke-$runId.json"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$app = $null
$failure = $null
$status = 'FAIL'
$sessionId = $null
$blockerPath = $null
$checks = [System.Collections.Generic.List[string]]::new()
$report = [ordered]@{
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'FAIL'
    session_id = $null
    checks = @()
    failure = $null
    synthetic_only = $true
    data_root = $dataRoot
    report_path = $reportPath
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
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$process) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $button = Wait-Button $Pattern $Label $TimeoutSeconds
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Wait-NewSessionDirectory([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $directories = @(Get-ChildItem $sessionsRoot -Directory -ErrorAction SilentlyContinue)
        if ($directories.Count -gt 0) {
            if ($directories.Count -ne 1) { throw "Expected one isolated test session, found $($directories.Count)" }
            return $directories[0].Name
        }
        Start-Sleep -Milliseconds 50
    }
    throw "No session directory appeared. Current contents: $((Get-ChildItem $sessionsRoot -Force -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name) -join ', ')"
}

function Wait-RetryButton([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button 'Retry local save*'
        if ($button) { return $button }
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$process) { throw 'Wails app exited before the retry action became available' }
        Start-Sleep -Milliseconds 200
    }
    throw "Retry local save button did not become available. UI text: $((Get-WindowText) -join ' | ')"
}

try {
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    $cfg = [ordered]@{
        schema_version = 1
        camera = [ordered]@{ provider = 'windowsmf'; width = 1280; height = 720; fps = 30 }
        capture = [ordered]@{ shots = 3; countdown = '5s'; interval = '0s' }
        frame = [ordered]@{ config_path = 'frames/photo-strip-v1.json' }
        storage = [ordered]@{ sessions_dir = 'sessions'; output_dir = 'photos' }
        media_delivery = [ordered]@{ mode = 'local' }
        payment = [ordered]@{ mode = 'disabled' }
    }
    $json = $cfg | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText($configPath, $json, [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')

    Write-Host 'WAILS_OUTPUT_RETRY_SMOKE=START (synthetic camera; isolated temp profile)'
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

    Invoke-Button 'Try without a webcam*' 'start synthetic three-shot session'
    $sessionId = Wait-NewSessionDirectory 15
    $report.session_id = $sessionId
    if ($sessionId -notmatch '^[0-9a-f]{32}$') { throw "Unexpected session ID format: $sessionId" }
    $checks.Add('synthetic session created with valid session ID') | Out-Null

    # Make output/<session-id> a regular file. The workflow can still capture
    # and render normally, but the final local output directory cannot be made.
    $blockerPath = Join-Path $outputRoot $sessionId
    if (Test-Path $blockerPath) { throw "Unexpected pre-existing output path: $blockerPath" }
    [IO.File]::WriteAllText($blockerPath, 'controlled-output-failure')
    Write-Host "OUTPUT_BLOCKER_CREATED=$blockerPath"
    $checks.Add('controlled per-session output obstruction installed') | Out-Null

    $retryButton = Wait-RetryButton $SessionTimeoutSeconds
    $beforeRetryText = Get-WindowText
    if (!($beforeRetryText -contains 'SAVE FAILED' -or ($beforeRetryText -join ' ') -match 'SAVE FAILED')) {
        throw "Result view did not expose SAVE FAILED state. UI text: $($beforeRetryText -join ' | ')"
    }
    $sessionDir = Join-Path $sessionsRoot $sessionId
    $captureDir = Join-Path $sessionDir 'captures'
    $captureFiles = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg')
    $renderedPath = Join-Path $sessionDir 'rendered.jpg'
    $finalPath = Join-Path (Join-Path $outputRoot $sessionId) 'photo.jpg'
    if ($captureFiles.Count -ne 3) { throw "Expected 3 preserved captures before retry, found $($captureFiles.Count)" }
    if (!(Test-Path $renderedPath) -or (Get-Item $renderedPath).Length -le 0) { throw 'Rendered image is missing before retry' }
    if (Test-Path $finalPath) { throw 'Final photo unexpectedly exists before retry' }
    if (!(Test-Path $blockerPath -PathType Leaf)) { throw 'Controlled output obstruction was unexpectedly removed before retry' }
    $checks.Add('Retry button appeared only after 3 captures and rendered image; output absent') | Out-Null
    Write-Host "RETRY_ACTION_VISIBLE=PASS session=$sessionId captures=$($captureFiles.Count)"

    # Remove only the blocker created by this harness; keep all source artifacts.
    Remove-Item -LiteralPath $blockerPath -Force
    if (Test-Path $blockerPath) { throw 'Could not remove the controlled output obstruction' }
    $checks.Add('controlled output obstruction removed') | Out-Null

    Invoke-Button 'Retry local save*' 'retry local save from preserved render'
    [void](Wait-Button 'Take another set*' 'completed result view' 30)
    $afterRetryText = Get-WindowText
    if (($afterRetryText -join ' ') -notmatch 'COMPLETED') {
        throw "The session did not report COMPLETED after retry. UI text: $($afterRetryText -join ' | ')"
    }
    if (($afterRetryText -join ' ') -notmatch [regex]::Escape($sessionId)) {
        throw "The result view does not show the original session ID in its output path. UI text: $($afterRetryText -join ' | ')"
    }

    if (!(Test-Path $finalPath -PathType Leaf)) { throw "Final local photo was not created: $finalPath" }
    $finalInfo = Get-Item $finalPath
    $renderedInfo = Get-Item $renderedPath
    if ($finalInfo.Length -le 0) { throw 'Final local photo is empty' }
    $renderedHash = (Get-FileHash -LiteralPath $renderedPath -Algorithm SHA256).Hash
    $finalHash = (Get-FileHash -LiteralPath $finalPath -Algorithm SHA256).Hash
    if ($renderedHash -ne $finalHash) { throw 'Saved photo does not match the preserved rendered image' }
    $captureFilesAfter = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg')
    if ($captureFilesAfter.Count -ne 3) { throw "Retry changed the capture set: $($captureFilesAfter.Count) files remain" }
    if (!(Test-Path $renderedPath)) { throw 'Retry removed the rendered source file' }
    if (Find-Button 'Retry local save*') { throw 'Retry button remained enabled after successful publication' }

    $checks.Add('retry completed the same session without recapturing or rerendering') | Out-Null
    $checks.Add('final photo SHA256 matches preserved rendered.jpg') | Out-Null
    $checks.Add('all captures and rendered source remain after retry') | Out-Null
    $checks.Add('retry action is no longer enabled after completion') | Out-Null
    $report.output_path = $finalPath
    $report.output_bytes = $finalInfo.Length
    $report.capture_count = $captureFilesAfter.Count
    $report.rendered_sha256 = $renderedHash
    $report.output_sha256 = $finalHash
    $report.status = 'PASS'
    $status = 'PASS'
    Write-Host 'WAILS_OUTPUT_RETRY_UI_SMOKE=PASS'
} catch {
    $failure = $_.Exception.Message
    $report.failure = $failure
    Write-Host 'WAILS_OUTPUT_RETRY_UI_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    if ($sessionId) {
        Write-Host "DEBUG_SESSION=$sessionId"
        Write-Host "DEBUG_UI_TEXT=$((Get-WindowText) -join ' | ')"
    }
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
        }
    }
    if ($blockerPath -and (Test-Path $blockerPath -PathType Leaf)) {
        Remove-Item -LiteralPath $blockerPath -Force -ErrorAction SilentlyContinue
    }
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report.checks = @($checks)
    $report.status = $status
    $report.failure = $failure
    $report | ConvertTo-Json -Depth 8 | Set-Content -Path $reportPath -Encoding utf8
    if (Test-Path $dataRoot) {
        Remove-Item $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    Write-Host "TEST_REPORT=$reportPath"
    Write-Host "TEMP_TEST_DATA_CLEANED=$(!(Test-Path $dataRoot))"
}

if ($status -ne 'PASS') { exit 1 }
exit 0
