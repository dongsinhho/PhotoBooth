[CmdletBinding()]
param(
    [ValidateRange(15,90)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(30,120)][int]$SessionTimeoutSeconds = 60
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$uiDir = $PSScriptRoot
$exe = Join-Path $uiDir 'build\bin\PhotoBooth.exe'
if (!(Test-Path $exe)) { throw "PhotoBooth.exe not found: $exe. Build with .\run-photobooth.ps1 -NoLaunch first." }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-recovery-ui-smoke-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$outputRoot = Join-Path $dataRoot 'photos'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$fixtureReportPath = Join-Path $dataRoot 'recovery-smoke-fixtures.json'
$metricsPath = Join-Path $dataRoot 'preview-metrics.jsonl'
$reportPath = Join-Path $env:TEMP "photobooth-recovery-ui-smoke-$runId.json"
$previousEnv = @{}
foreach ($name in @('PHOTOBOOTH_DATA_DIR','PHOTOBOOTH_DISABLE_CAMERA_PREVIEW','PHOTOBOOTH_SYNTHETIC_ONLY','PHOTOBOOTH_PREVIEW_METRICS_PATH')) {
    $previousEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
$app = $null
$status = 'FAIL'
$failure = $null
$checks = [System.Collections.Generic.List[string]]::new()
$fixture = $null
$report = [ordered]@{
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'FAIL'
    synthetic_only = $true
    physical_camera_allowed = $false
    preview_disabled_env = 'PHOTOBOOTH_DISABLE_CAMERA_PREVIEW=1'
    synthetic_only_env = 'PHOTOBOOTH_SYNTHETIC_ONLY=1'
    data_root = $dataRoot
    checks = @()
    failure = $null
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

function Find-Buttons([string]$Pattern) {
    $window = Get-Window
    if (!$window) { return @() }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    $matches = [System.Collections.Generic.List[System.Windows.Automation.AutomationElement]]::new()
    foreach ($button in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)) {
        if ($button.Current.Name -like $Pattern -and $button.Current.IsEnabled) {
            $matches.Add($button) | Out-Null
        }
    }
    return $matches.ToArray()
}

function Wait-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $buttons = Find-Buttons $Pattern
        if ($buttons.Count -gt 0) { return $buttons[0] }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "Wails app exited while waiting for '$Label'"
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label, [int]$TimeoutSeconds = 15, [int]$Index = 0) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $button = $null
    while ((Get-Date) -lt $deadline) {
        $buttons = Find-Buttons $Pattern
        if ($buttons.Count -gt $Index) {
            $button = $buttons[$Index]
            break
        }
        Start-Sleep -Milliseconds 150
    }
    if (!$button) {
        throw "Could not find enabled button '$Label' at index $Index. UI text: $((Get-WindowText) -join ' | ')"
    }
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Wait-WindowTextContains([string[]]$Values, [string]$Label, [int]$TimeoutSeconds = 20) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        $joined = $text -join ' | '
        $allFound = $true
        foreach ($value in $Values) {
            if ($joined -notmatch [regex]::Escape($value)) { $allFound = $false; break }
        }
        if ($allFound) { return $text }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for '$Label'. UI text: $((Get-WindowText) -join ' | ')"
}

function Wait-Result([string]$ExpectedSessionId, [int]$TimeoutSeconds = 60) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        $joined = $text -join ' | '
        if (($joined -match 'COMPLETED' -or $joined -match "All 3 photos") -and
            ($joined -match [regex]::Escape($ExpectedSessionId))) {
            return $text
        }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited during a synthetic session' }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for completed result for $ExpectedSessionId. UI text: $((Get-WindowText) -join ' | ')"
}

function Wait-IdleRecovery([string]$ExpectedSessionId, [int]$TimeoutSeconds = 20) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = Get-WindowText
        $joined = $text -join ' | '
        if ($joined -match 'Interrupted sessions' -and $joined -match [regex]::Escape($ExpectedSessionId)) {
            return $text
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Idle recovery panel did not return with $ExpectedSessionId. UI text: $((Get-WindowText) -join ' | ')"
}

function Stop-TestApp {
    if (!$app) { return }
    $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
    if ($process) {
        [void]$process.CloseMainWindow()
        if (!$process.WaitForExit(5000)) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(3000)) {
                Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
                throw 'PhotoBooth did not exit gracefully after the second close request.'
            }
        }
    }
}

function Get-FileSha256([string]$Path) {
    if (!(Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Expected file missing: $Path" }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-FixtureHashesUnchanged([string]$SessionId) {
    $sessionDir = Join-Path $sessionsRoot $SessionId
    $keys = @($fixture.original_hashes.PSObject.Properties.Name | Where-Object { $_.StartsWith($sessionDir, [StringComparison]::OrdinalIgnoreCase) })
    if ($keys.Count -lt 3) { throw "No complete original hash set found for inspect-only fixture $SessionId" }
    foreach ($key in $keys) {
        $expected = [string]$fixture.original_hashes.$key
        $actual = Get-FileSha256 $key
        if ($actual -ne $expected) { throw "Restart changed original fixture data: $key" }
    }
}

try {
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    $framePath = Join-Path $repoRoot 'tools\ui-spike\wails-v2\assets\photo-strip-v1.json'
    if (!(Test-Path $framePath -PathType Leaf)) { throw "Synthetic smoke frame is missing: $framePath" }

    $cfg = [ordered]@{
        schema_version = 1
        camera = [ordered]@{ provider = 'windowsmf'; width = 1280; height = 720; fps = 30 }
        capture = [ordered]@{ shots = 3; countdown = '0s'; interval = '0s' }
        frame = [ordered]@{ config_path = $framePath }
        storage = [ordered]@{ sessions_dir = 'sessions'; output_dir = 'photos'; session_retention_days = 0; output_retention_days = 0 }
        media_delivery = [ordered]@{ mode = 'local' }
        payment = [ordered]@{ mode = 'disabled' }
    }
    $configJson = $cfg | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText($configPath, $configJson, [Text.UTF8Encoding]::new($false))

    # Fixture generation is offline and uses synthetic JPEGs. No Wails process
    # exists yet, and no camera provider is constructed by this helper.
    Push-Location $uiDir
    try {
        $seedOutput = & go run ./cmd/recovery-fixture -data-dir $dataRoot 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Recovery fixture generation failed: $($seedOutput -join ' | ')" }
        $seedOutput | ForEach-Object { Write-Host $_ }
    } finally {
        Pop-Location
    }
    $fixture = Get-Content -LiteralPath $fixtureReportPath -Raw | ConvertFrom-Json
    $fixtureIds = @($fixture.continue_id, $fixture.output_pending_id, $fixture.restart_inspect_id, $fixture.hide_inspect_id, $fixture.export_inspect_id)
    $report.fixture_ids = $fixtureIds

    # The three critical controls are inherited by PhotoBooth.exe. The preview
    # guard is checked before profile setup or provider construction.
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DISABLE_CAMERA_PREVIEW', '1', 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_SYNTHETIC_ONLY', '1', 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', $metricsPath, 'Process')

    Write-Host 'WAILS_RECOVERY_UI_SMOKE=START (synthetic fixtures; idle preview disabled; physical camera not permitted)'
    $app = Start-Process -FilePath $exe -WorkingDirectory $repoRoot -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 150
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited before its window appeared' }
    }
    if (!$window) { throw 'Wails main window did not appear' }
    $checks.Add('isolated Wails window opened') | Out-Null

    $startupExpectations = @('Camera preview is disabled for this run', 'Interrupted sessions') + $fixtureIds
    $startupText = Wait-WindowTextContains $startupExpectations 'disabled camera status and all recovery fixture cards' 25
    foreach ($id in $fixtureIds) {
        if (!(($startupText -join ' ') -match [regex]::Escape($id))) { throw "Recovery panel did not show fixture $id" }
    }
    $checks.Add('startup discovered all five fixture sessions without starting camera preview') | Out-Null

    # Prove the visible webcam action is also blocked by the backend guard. A
    # mistaken click must not create a session directory or open Media Foundation.
    Invoke-Button 'Start with webcam*' 'verify physical camera start is rejected in synthetic-only mode'
    [void](Wait-WindowTextContains @('real-camera capture is disabled for this synthetic-only run') 'synthetic-only webcam refusal' 10)
    $afterBlockedCameraStart = @(Get-ChildItem $sessionsRoot -Directory)
    if ($afterBlockedCameraStart.Count -ne 5) { throw "Blocked webcam start changed session directory count to $($afterBlockedCameraStart.Count)" }
    $checks.Add('Start with webcam is rejected by backend and creates no camera session') | Out-Null

    # 1. Continue a partially captured demo session. Only shots 2 and 3 are
    # generated by the fake provider; the original capture remains unchanged.
    $continueDir = Join-Path $sessionsRoot $fixture.continue_id
    $continueCapture = Join-Path (Join-Path $continueDir 'captures') 'capture-01.jpg'
    $continueCaptureHash = Get-FileSha256 $continueCapture
    Invoke-Button 'Continue session*' 'continue synthetic interrupted session'
    $continueResult = Wait-Result $fixture.continue_id $SessionTimeoutSeconds
    $continueSessionDir = Join-Path $sessionsRoot $fixture.continue_id
    $continuedCaptures = @(Get-ChildItem (Join-Path $continueSessionDir 'captures') -Filter 'capture-*.jpg' -File)
    if ($continuedCaptures.Count -ne 3) { throw "Continue expected 3 captures, found $($continuedCaptures.Count)" }
    if ((Get-FileSha256 $continueCapture) -ne $continueCaptureHash) { throw 'Continue modified the preserved first capture' }
    if (!(Test-Path (Join-Path (Join-Path $outputRoot $fixture.continue_id) 'photo.jpg') -PathType Leaf)) { throw 'Continue did not publish local output' }
    $checks.Add('Continue captured only the remaining synthetic shots and preserved the original capture') | Out-Null

    Invoke-Button 'Take another set*' 'return to recovery home after Continue'
    [void](Wait-IdleRecovery $fixture.output_pending_id)

    # 2. Retry local output from the rendered image only.
    $outputDir = Join-Path $sessionsRoot $fixture.output_pending_id
    $renderedPath = Join-Path $outputDir 'rendered.jpg'
    $renderedHash = Get-FileSha256 $renderedPath
    Invoke-Button 'Retry local save*' 'retry output-pending session without camera'
    $outputResult = Wait-Result $fixture.output_pending_id 30
    $recoveredOutputPath = Join-Path (Join-Path $outputRoot $fixture.output_pending_id) 'photo.jpg'
    if ((Get-FileSha256 $recoveredOutputPath) -ne $renderedHash) { throw 'Recovered local output differs from the preserved rendered JPEG' }
    if ((Get-FileSha256 $renderedPath) -ne $renderedHash) { throw 'Output retry modified rendered.jpg' }
    $checks.Add('Retry local save published the preserved rendered JPEG without capture or rendering') | Out-Null

    Invoke-Button 'Take another set*' 'return to recovery home after output retry'
    [void](Wait-IdleRecovery $fixture.restart_inspect_id)

    # 3. Restart an inspect-only demo session with a new session ID. Accept the
    # explicit browser confirmation, then verify every original file is byte-identical.
    Invoke-Button 'Start a new session*' 'confirm fresh synthetic session from inspect-only entry' 15 0
    Start-Sleep -Milliseconds 250
    $shell = New-Object -ComObject WScript.Shell
    [void]$shell.AppActivate($app.Id)
    $shell.SendKeys('{ENTER}')
    $restartDeadline = (Get-Date).AddSeconds($SessionTimeoutSeconds)
    $freshId = $null
    while ((Get-Date) -lt $restartDeadline -and !$freshId) {
        $directories = @(Get-ChildItem $sessionsRoot -Directory | Where-Object { $fixtureIds -notcontains $_.Name })
        if ($directories.Count -gt 0) {
            if ($directories.Count -ne 1) { throw "Expected one new session directory, found $($directories.Count)" }
            $freshId = $directories[0].Name
        } else {
            Start-Sleep -Milliseconds 150
        }
    }
    if (!$freshId -or $freshId -eq $fixture.restart_inspect_id) { throw "Restart did not allocate a fresh session ID: $freshId" }
    [void](Wait-Result $freshId $SessionTimeoutSeconds)
    Assert-FixtureHashesUnchanged $fixture.restart_inspect_id
    $checks.Add('Restart allocated a new synthetic session ID and preserved every original inspect-only file') | Out-Null

    Invoke-Button 'Take another set*' 'return to recovery home after fresh session'
    [void](Wait-IdleRecovery $fixture.hide_inspect_id)

    # 4. Hide is an in-memory UI action only; verify the item disappears from
    # this launch while all of its source bytes remain unchanged.
    Invoke-Button 'Hide for now*' 'hide inspect-only session for this launch' 15 0
    $afterHide = Get-WindowText
    if (($afterHide -join ' ') -match [regex]::Escape($fixture.hide_inspect_id)) {
        throw 'Hide for now did not remove the selected card from the current UI'
    }
    Assert-FixtureHashesUnchanged $fixture.hide_inspect_id
    $checks.Add('Hide for now removed only the card from the current UI and changed no source files') | Out-Null

    # 5. Exercise the inspect/export action on a separate disposable fixture.
    Invoke-Button 'Inspect / export files*' 'open inspect-only session folder' 15 0
    Start-Sleep -Milliseconds 250
    if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'Wails app exited after Inspect / export files' }
    Assert-FixtureHashesUnchanged $fixture.export_inspect_id
    $checks.Add('Inspect / export action completed while the session source remained unchanged') | Out-Null

    # Close the Wails process gracefully so OnShutdown flushes/closes the
    # preview diagnostics handle before the smoke audits the full event file.
    Stop-TestApp

    if ($metricsPath -and (Test-Path $metricsPath -PathType Leaf)) {
        $events = @(Get-Content -LiteralPath $metricsPath | Where-Object { $_.Trim() } | ForEach-Object { $_ | ConvertFrom-Json })
        $report.preview_event_summary = @($events | ForEach-Object { "$($_.event):$($_.state)" })
        $badEvents = @($events | Where-Object { $_.event -eq 'frame_published' -or $_.event -like 'recovery_*' -or ($_.event -eq 'preview_status' -and $_.state -in @('starting','ready','reconnecting')) })
        if ($badEvents.Count -gt 0) { throw "Camera preview worker activity was logged despite preview-off mode: $($badEvents | ConvertTo-Json -Compress)" }
        $disabledEvents = @($events | Where-Object { $_.event -eq 'preview_status' -and $_.state -eq 'disabled' })
        if ($disabledEvents.Count -lt 1) {
            throw "Preview-off mode did not emit its disabled status diagnostic. Observed events: $($report.preview_event_summary -join ', ')"
        }
        $checks.Add('diagnostic log confirms disabled status and contains no preview frame/recovery events') | Out-Null
    } else {
        throw 'Preview diagnostics file was not created; unable to verify the disabled status event'
    }

    # All seeded sessions are still under this disposable root. The UI process
    # is asked to close gracefully before this script removes only its own data.
    $report.status = 'PASS'
    $status = 'PASS'
    Write-Host 'WAILS_RECOVERY_UI_SMOKE=PASS'
} catch {
    $failure = $_.Exception.Message
    $report.failure = $failure
    Write-Host 'WAILS_RECOVERY_UI_SMOKE=FAIL'
    Write-Host "FAILURE=$failure"
    if ($app) { Write-Host "DEBUG_UI_TEXT=$((Get-WindowText) -join ' | ')" }
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(5000)) {
                Write-Warning 'PhotoBooth did not exit gracefully within 5 seconds; sending a normal process close request again.'
                [void]$process.CloseMainWindow()
                if (!$process.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
            }
        }
    }

    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report.checks = @($checks)
    $report.status = $status
    $report.failure = $failure
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding utf8

    if (Test-Path $dataRoot) {
        Remove-Item -LiteralPath $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    foreach ($name in $previousEnv.Keys) {
        [Environment]::SetEnvironmentVariable($name, $previousEnv[$name], 'Process')
    }
    Write-Host "TEST_REPORT=$reportPath"
    Write-Host "TEMP_TEST_DATA_CLEANED=$(!(Test-Path $dataRoot))"
}

if ($status -ne 'PASS') { exit 1 }
exit 0
