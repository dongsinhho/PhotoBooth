[CmdletBinding()]
param(
    [ValidateRange(1,3)][int]$Trials = 2,
    [ValidateRange(60,300)][int]$HolderSeconds = 180
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$python = Join-Path $repoRoot 'tools\camera-poc\.venv\Scripts\python.exe'
$holderScript = Join-Path $repoRoot 'tools\camera-poc\python\hold_camera.py'
$exe = Join-Path $repoRoot 'tools\ui-spike\wails-v2\build\bin\wails-v2.exe'
$sessionRoot = Join-Path $env:LOCALAPPDATA 'PhotoBooth\sessions'
$outputRoot = Join-Path $env:LOCALAPPDATA 'PhotoBooth\photos'
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$tempRoot = Join-Path $env:TEMP "photobooth-ui-recovery-$stamp"
$reportPath = Join-Path $env:TEMP "photobooth-ui-recovery-$stamp.json"
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null

if (!(Test-Path $python)) { throw "Python virtualenv missing: $python" }
if (!(Test-Path $holderScript)) { throw "Camera holder script missing: $holderScript" }
if (!(Test-Path $exe)) { throw "Built Wails app missing: $exe (build it first)" }

$baselineSessions = @()
$baselineOutputs = @()
if (Test-Path $sessionRoot) { $baselineSessions = @(Get-ChildItem $sessionRoot -Directory | ForEach-Object { $_.Name }) }
if (Test-Path $outputRoot) { $baselineOutputs = @(Get-ChildItem $outputRoot -Directory | ForEach-Object { $_.Name }) }

$app = $null
$holder = $null
$failure = $null
$results = [System.Collections.Generic.List[object]]::new()
$summary = [ordered]@{
    started_utc = [DateTime]::UtcNow.ToString('o')
    trials_requested = $Trials
    recovery_cycles_passed = 0
    fresh_session_cycles_passed = 0
    completed_sessions = @()
    status = 'FAIL'
    failure = $null
    report_path = $reportPath
}

function Get-SessionDirectoryNames {
    if (!(Test-Path $sessionRoot)) { return @() }
    return @(Get-ChildItem $sessionRoot -Directory | ForEach-Object { $_.Name })
}

function Get-Window {
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $app.Id
    )
    $wins = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($wins.Count -gt 0) { return $wins.Item(0) }
    return $null
}

function Find-Button([string[]]$Patterns) {
    $window = Get-Window
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    $buttons = $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)
    foreach ($button in $buttons) {
        foreach ($pattern in $Patterns) {
            if ($button.Current.Name -like $pattern -and $button.Current.IsEnabled) { return $button }
        }
    }
    return $null
}

function Wait-Button([string[]]$Patterns, [int]$TimeoutSeconds, [string]$Label) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Patterns
        if ($button) { return $button }
        $proc = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if (!$proc) { throw "Wails app exited while waiting for '$Label'" }
        Start-Sleep -Milliseconds 250
    }
    $window = Get-Window
    $names = @()
    if ($window) {
        $condition = New-Object System.Windows.Automation.PropertyCondition(
            [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
            [System.Windows.Automation.ControlType]::Button
        )
        $names = @($window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition) | ForEach-Object { $_.Current.Name })
    }
    throw "Timed out waiting for '$Label'. Buttons now: $($names -join ' | ')"
}

function Invoke-Button([string[]]$Patterns, [int]$TimeoutSeconds, [string]$Label) {
    $button = Wait-Button $Patterns $TimeoutSeconds $Label
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "UI_ACTION=$Label"
}

function Start-CameraHolder([string]$Tag) {
    $stdout = Join-Path $tempRoot "holder-$Tag.stdout.log"
    $stderr = Join-Path $tempRoot "holder-$Tag.stderr.log"
    $args = '"' + $holderScript + '" --backend msmf --seconds ' + $HolderSeconds
    $script:holder = Start-Process -FilePath $python -ArgumentList $args -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr -WindowStyle Hidden
    $deadline = (Get-Date).AddSeconds(12)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 250
        if ((Test-Path $stdout) -and ((Get-Content $stdout -Raw -ErrorAction SilentlyContinue) -match 'HOLDER_OPENED')) {
            Write-Host "CAMERA_HOLDER=OPEN ($Tag)"
            return
        }
        if ($script:holder.HasExited) { break }
    }
    $stdoutText = if (Test-Path $stdout) { Get-Content $stdout -Raw } else { '' }
    $stderrText = if (Test-Path $stderr) { Get-Content $stderr -Raw } else { '' }
    throw "Could not acquire the B525 for forced UI failure ($Tag). Stdout: $stdoutText Stderr: $stderrText"
}

function Stop-CameraHolder {
    if ($script:holder) {
        Stop-Process -Id $script:holder.Id -Force -ErrorAction SilentlyContinue
        Wait-Process -Id $script:holder.Id -Timeout 5 -ErrorAction SilentlyContinue
        $script:holder = $null
        Start-Sleep -Milliseconds 600
        Write-Host 'CAMERA_HOLDER=RELEASED'
    }
}

function Wait-NewSessionId([string[]]$Before, [int]$TimeoutSeconds, [string]$Label) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $newIds = @(Get-SessionDirectoryNames | Where-Object { $Before -notcontains $_ })
        if ($newIds.Count -gt 0) {
            $dir = Get-ChildItem $sessionRoot -Directory | Where-Object { $newIds -contains $_.Name } | Sort-Object CreationTimeUtc -Descending | Select-Object -First 1
            if ($dir) { return $dir.Name }
        }
        Start-Sleep -Milliseconds 250
    }
    throw "No new session directory appeared for '$Label'"
}

function Wait-SessionArtifacts([string]$SessionId, [int]$TimeoutSeconds) {
    $sessionDir = Join-Path $sessionRoot $SessionId
    $rendered = Join-Path $sessionDir 'rendered.jpg'
    $output = Join-Path (Join-Path $outputRoot $SessionId) 'photo.jpg'
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $captures = @()
        $captureDir = Join-Path $sessionDir 'captures'
        if (Test-Path $captureDir) { $captures = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg') }
        if ($captures.Count -eq 3 -and (Test-Path $rendered) -and (Test-Path $output)) {
            return [ordered]@{
                session_id = $SessionId
                capture_count = $captures.Count
                rendered = $true
                local_output = $true
                output_bytes = (Get-Item $output).Length
            }
        }
        Start-Sleep -Milliseconds 300
    }
    $count = 0
    $captureDir = Join-Path $sessionDir 'captures'
    if (Test-Path $captureDir) { $count = @(Get-ChildItem $captureDir -File -Filter 'capture-*.jpg').Count }
    throw "Session $SessionId did not complete capture/render/save within $TimeoutSeconds seconds (captures=$count, rendered=$(Test-Path $rendered), output=$(Test-Path $output))"
}

try {
    Write-Host "PhotoBooth repeated Wails recovery smoke test: $Trials trial(s)"
    Write-Host 'Each trial forces camera contention twice: continue same session, then restart from a fresh session.'
    $app = Start-Process -FilePath $exe -PassThru
    Start-Sleep -Seconds 2
    if (!(Get-Window)) { throw 'Wails window did not appear' }

    for ($trial = 1; $trial -le $Trials; $trial++) {
        if ($trial -gt 1) {
            Invoke-Button @('Take another set*') 10 "return-to-start trial $trial"
        }

        # Cycle A: force initial camera-open failure, then continue the same session.
        $beforeA = Get-SessionDirectoryNames
        Start-CameraHolder "t$trial-a"
        Invoke-Button @('Start with webcam*') 10 "start-webcam trial $trial / continue"
        [void](Wait-Button @('Retry camera connection*', 'Reconnect and continue remaining shots*') 45 "camera failure trial $trial / continue")
        $failedA = Wait-NewSessionId $beforeA 5 "failure session A trial $trial"
        Write-Host "FAILED_SESSION_A=$failedA"
        Stop-CameraHolder
        Invoke-Button @('Retry camera connection*', 'Reconnect and continue remaining shots*') 10 "continue-existing-session trial $trial"
        [void](Wait-Button @('Take another set*') 60 "result after continue trial $trial")
        $doneA = Wait-SessionArtifacts $failedA 5
        $results.Add([ordered]@{ trial=$trial; flow='continue_same_session'; failed_session_id=$failedA; completed=$doneA }) | Out-Null
        $summary.recovery_cycles_passed++
        Write-Host "CONTINUE_SAME_SESSION=PASS trial=$trial id=$failedA"

        # Cycle B: force a second failure, then use Start a fresh session.
        Invoke-Button @('Take another set*') 10 "return-to-start before fresh restart trial $trial"
        $beforeB = Get-SessionDirectoryNames
        Start-CameraHolder "t$trial-b"
        Invoke-Button @('Start with webcam*') 10 "start-webcam trial $trial / fresh restart"
        [void](Wait-Button @('Retry camera connection*', 'Reconnect and continue remaining shots*') 45 "camera failure trial $trial / fresh restart")
        $failedB = Wait-NewSessionId $beforeB 5 "failure session B trial $trial"
        Write-Host "FAILED_SESSION_B=$failedB"
        Stop-CameraHolder
        $beforeRestart = Get-SessionDirectoryNames
        Invoke-Button @('Start a fresh session') 10 "start-fresh-session trial $trial"
        [void](Wait-Button @('Take another set*') 60 "result after fresh restart trial $trial")
        $freshId = Wait-NewSessionId $beforeRestart 5 "fresh session trial $trial"
        if ($freshId -eq $failedB) { throw "Fresh session reused failed session ID $failedB" }
        $doneB = Wait-SessionArtifacts $freshId 5
        $results.Add([ordered]@{ trial=$trial; flow='fresh_session_after_failure'; failed_session_id=$failedB; completed=$doneB; new_id_verified=($freshId -ne $failedB) }) | Out-Null
        $summary.fresh_session_cycles_passed++
        Write-Host "FRESH_SESSION=PASS trial=$trial old=$failedB new=$freshId"
    }

    $summary.completed_sessions = @($results)
    $summary.status = 'PASS'
    Write-Host 'WAILS_UI_RECOVERY_AND_FRESH_SESSION_GATE=PASS'
} catch {
    $failure = $_.Exception.Message
    $summary.failure = $failure
    $summary.completed_sessions = @($results)
    Write-Host "WAILS_UI_RECOVERY_AND_FRESH_SESSION_GATE=FAIL"
    Write-Host "FAILURE=$failure"
} finally {
    Stop-CameraHolder
    if ($app) {
        $proc = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($proc) {
            [void]$proc.CloseMainWindow()
            if (!$proc.WaitForExit(3000)) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
        }
    }

    # Photos created by this test may include private surroundings. Delete every
    # session/output created after the baseline regardless of pass or failure.
    $newSessionIds = @(Get-SessionDirectoryNames | Where-Object { $baselineSessions -notcontains $_ })
    foreach ($id in $newSessionIds) {
        Remove-Item (Join-Path $sessionRoot $id) -Recurse -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path $outputRoot) {
        Get-ChildItem $outputRoot -Directory | Where-Object { $baselineOutputs -notcontains $_.Name } | ForEach-Object {
            Remove-Item $_.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    $summary.finished_utc = [DateTime]::UtcNow.ToString('o')
    $summary.report_path = $reportPath
    $summary | ConvertTo-Json -Depth 10 | Set-Content -Path $reportPath -Encoding utf8
    Write-Host "TEST_REPORT=$reportPath"
    Write-Host "TEST_SESSION_ARTIFACTS_CLEANED=$($newSessionIds.Count)"
    Remove-Item $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

if ($summary.status -ne 'PASS') { exit 1 }
exit 0
