[CmdletBinding()]
param(
    [ValidateRange(1,10)][int]$Sessions = 3,
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 25,
    [ValidateRange(30,120)][int]$SessionTimeoutSeconds = 70,
    [string]$Device = 'B525 HD Webcam',
    [ValidateRange(0,255)][double]$MinMeanLuma = 20,
    [ValidateRange(0,100)][double]$MaxFirstShotDropPct = 10,
    [switch]$KeepArtifacts
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$exe = Join-Path $PSScriptRoot 'build\bin\PhotoBooth.exe'
if (!(Test-Path -LiteralPath $exe -PathType Leaf)) {
    # Keep compatibility with older local builds, but prefer the launcher's current output.
    $exe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'
}
$python = Join-Path $repoRoot 'tools\camera-poc\.venv\Scripts\python.exe'
$qualityScript = Join-Path $repoRoot 'tools\camera-poc\python\validate_capture_quality.py'
foreach ($required in @($exe, $python, $qualityScript)) { if (!(Test-Path $required)) { throw "Required file not found: $required" } }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-wails-quality-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$outputRoot = Join-Path $dataRoot 'photos'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$reportPath = Join-Path $env:TEMP "photobooth-wails-quality-$runId.json"
$stdoutPath = Join-Path $env:TEMP "photobooth-wails-quality-$runId.stdout.log"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$app = $null
$status = 'FAIL'
$failure = $null
$checks = [System.Collections.Generic.List[string]]::new()
$completed = [System.Collections.Generic.List[object]]::new()

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
    throw 'No new session directory appeared after starting the UI session.'
}

function Wait-CompletedResult([int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Find-Button 'Take another set*') {
            $text = Get-WindowText
            if (($text -join ' ') -match 'COMPLETED') { return $text }
        }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited during camera capture.' }
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

    Write-Host "WAILS_CAMERA_QUALITY_SMOKE=START sessions=$Sessions shots=3 countdown=3s interval=1s"
    Write-Host "CAMERA=$Device INSTANCE=$cameraInstanceId"
    Write-Host 'Keep the camera aim, lighting and scene unchanged for the entire batch.'
    $app = Start-Process -FilePath $exe -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 200
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited before opening its main window.' }
    }
    if (!$window) { throw 'Wails main window did not appear.' }
    $checks.Add('isolated Wails app opened') | Out-Null

    for ($index = 1; $index -le $Sessions; $index++) {
        if ($index -gt 1) {
            Invoke-Button 'Take another set*' "return to start before session $index"
            [void](Wait-Button 'Start with webcam*' 'start webcam button' 10)
        }
        $before = Get-SessionIds
        Invoke-Button 'Start with webcam*' "start real B525 session $index"
        $sessionId = Wait-NewSession $before 15
        if ($sessionId -notmatch '^[0-9a-f]{32}$') { throw "Invalid session ID: $sessionId" }
        Write-Host "SESSION_$index=$sessionId"
        [void](Wait-CompletedResult $SessionTimeoutSeconds)
        $sessionDir = Join-Path $sessionsRoot $sessionId
        $captureDir = Join-Path $sessionDir 'captures'
        $captures = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg')
        $rendered = Join-Path $sessionDir 'rendered.jpg'
        $output = Join-Path (Join-Path $outputRoot $sessionId) 'photo.jpg'
        if ($captures.Count -ne 3) { throw "Session $index expected 3 captures, found $($captures.Count)." }
        if (!(Test-Path $rendered -PathType Leaf)) { throw "Session $index missing rendered.jpg." }
        if (!(Test-Path $output -PathType Leaf)) { throw "Session $index missing photo.jpg." }
        foreach ($file in $captures) { if ($file.Length -le 0) { throw "Session $index has an empty capture: $($file.Name)" } }
        $completed.Add([ordered]@{
            session_id = $sessionId
            session_dir = $sessionDir
            capture_count = $captures.Count
            capture_paths = @($captures | ForEach-Object { $_.FullName })
            rendered = $true
            rendered_path = $rendered
            local_output = $true
            local_output_path = $output
        }) | Out-Null
        $checks.Add("session $index completed capture/render/local-save through Wails") | Out-Null
        Write-Host "SESSION_$index=PASS captures=3 rendered=True output=True"
    }

    & $python $qualityScript $dataRoot --min-mean-luma $MinMeanLuma --max-first-shot-drop-pct $MaxFirstShotDropPct --json-out $reportPath *> $stdoutPath
    $qualityExit = $LASTEXITCODE
    if (!(Test-Path $reportPath)) { throw "Quality validator did not write a JSON report. Output: $(Get-Content $stdoutPath -Raw -ErrorAction SilentlyContinue)" }
    $metrics = Get-Content $reportPath -Raw | ConvertFrom-Json
    $metrics | Add-Member -NotePropertyName test_setup -NotePropertyValue ([ordered]@{
        camera = $Device; camera_instance_id = $cameraInstanceId; capture_mode = 'Wails UI / Windows Media Foundation'
        sessions = $Sessions; shots_per_session = 3; countdown = '3s'; interval = '1s'; warmup = '1s default in provider'
        started_utc = [DateTime]::UtcNow.ToString('o'); session_ids = @($completed | ForEach-Object { $_.session_id })
        note = 'Diagnostic only. Exposure/lighting/scene are not controlled by this script; it does not certify portrait quality.'
    }) -Force
    $metrics | Add-Member -NotePropertyName ui_workflow -NotePropertyValue ([ordered]@{ completed_sessions = @($completed); status = if ($qualityExit -eq 0) { 'PASS' } else { 'QUALITY_GATE_FAIL' } }) -Force
    $metrics | Add-Member -NotePropertyName executable -NotePropertyValue $exe -Force
    $metrics | ConvertTo-Json -Depth 16 | Set-Content -Path $reportPath -Encoding utf8
    if ($qualityExit -ne 0) {
        Write-Host "WAILS_IMAGE_QUALITY_GATE=FAIL (report retained at $reportPath)"
        $status = 'QUALITY_GATE_FAIL'
        throw 'The UI capture quality gate failed. Review the retained metrics; thresholds were not relaxed.'
    }
    $status = 'PASS'
    Write-Host "JSON_REPORT=$reportPath"
    Write-Host 'WAILS_CAMERA_QUALITY_SMOKE=PASS'
} catch {
    $failure = $_.Exception.Message
    Write-Host 'WAILS_CAMERA_QUALITY_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    Write-Host "UI_TEXT=$((Get-WindowText) -join ' | ')"
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) { [void]$process.CloseMainWindow(); if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue } }
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')

    $artifactSessions = @()
    if (Test-Path -LiteralPath $sessionsRoot -PathType Container) {
        $artifactSessions = @(Get-ChildItem -LiteralPath $sessionsRoot -Directory | Sort-Object Name | ForEach-Object {
            $sessionDir = $_.FullName
            $captureDir = Join-Path $sessionDir 'captures'
            $sessionOutput = Join-Path (Join-Path $outputRoot $_.Name) 'photo.jpg'
            $capturePaths = @()
            if (Test-Path -LiteralPath $captureDir -PathType Container) {
                $capturePaths = @(Get-ChildItem -LiteralPath $captureDir -File -Filter 'capture-*.jpg' | Sort-Object Name | ForEach-Object { $_.FullName })
            }
            [ordered]@{
                session_id = $_.Name
                session_dir = $sessionDir
                capture_paths = $capturePaths
                rendered_path = if (Test-Path -LiteralPath (Join-Path $sessionDir 'rendered.jpg') -PathType Leaf) { Join-Path $sessionDir 'rendered.jpg' } else { $null }
                local_output_path = if (Test-Path -LiteralPath $sessionOutput -PathType Leaf) { $sessionOutput } else { $null }
            }
        })
    }

    $cleaned = $false
    $artifactsRetained = $false
    if ($KeepArtifacts) {
        $artifactsRetained = Test-Path -LiteralPath $dataRoot -PathType Container
    } else {
        if (Test-Path -LiteralPath $dataRoot) {
            Remove-Item -LiteralPath $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
        $cleaned = !(Test-Path -LiteralPath $dataRoot)
    }
    if (Test-Path -LiteralPath $reportPath -PathType Leaf) {
        $metrics = Get-Content -LiteralPath $reportPath -Raw | ConvertFrom-Json
    } else {
        $metrics = [ordered]@{
            status = $status
            failure = $failure
            test_setup = [ordered]@{
                camera = $Device
                capture_mode = 'Wails UI / Windows Media Foundation'
                sessions_requested = $Sessions
                started_utc = [DateTime]::UtcNow.ToString('o')
                executable = $exe
            }
            ui_workflow = [ordered]@{
                completed_sessions = @($completed)
                status = $status
            }
        }
    }
    $metrics | Add-Member -NotePropertyName artifact_manifest -NotePropertyValue ([ordered]@{
        keep_artifacts_requested = [bool]$KeepArtifacts
        retained = $artifactsRetained
        cleaned = $cleaned
        root = $dataRoot
        sessions_root = $sessionsRoot
        output_root = $outputRoot
        sessions = $artifactSessions
    }) -Force
    $metrics | Add-Member -NotePropertyName temp_data_cleaned -NotePropertyValue $cleaned -Force
    $metrics | Add-Member -NotePropertyName temp_data_retained -NotePropertyValue $artifactsRetained -Force
    $metrics | ConvertTo-Json -Depth 16 | Set-Content -Path $reportPath -Encoding utf8

    Remove-Item $stdoutPath -Force -ErrorAction SilentlyContinue
    Write-Host "TEMP_TEST_DATA_CLEANED=$cleaned"
    Write-Host "TEMP_TEST_DATA_RETAINED=$artifactsRetained"
    if ($artifactsRetained) {
        Write-Host "ARTIFACT_ROOT=$dataRoot"
        Write-Host "SESSION_ARTIFACTS=$sessionsRoot"
        Write-Host "LOCAL_OUTPUT_ARTIFACTS=$outputRoot"
    }
    if (Test-Path -LiteralPath $reportPath) { Write-Host "REPORT_RETAINED=$reportPath" }
}

if ($status -ne 'PASS') { exit 1 }
exit 0
