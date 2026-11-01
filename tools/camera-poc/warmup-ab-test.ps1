[CmdletBinding()]
param(
    [ValidateRange(2,6)][int]$SessionsPerVariant = 3,
    [ValidateRange(3,12)][int]$Shots = 3,
    [string]$Device = 'B525 HD Webcam',
    [ValidateRange(0,255)][double]$MinMeanLuma = 20,
    [ValidateRange(0,100)][double]$MaxFirstShotDropPct = 10,
    [switch]$KeepArtifacts
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$python = Join-Path $repoRoot 'tools\camera-poc\.venv\Scripts\python.exe'
$qualityScript = Join-Path $PSScriptRoot 'python\validate_capture_quality.py'
if (!(Test-Path $python)) { throw "Repository Python venv is missing: $python" }
if (!(Test-Path $qualityScript)) { throw "Capture quality validator is missing: $qualityScript" }

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$tempRoot = Join-Path $env:TEMP "photobooth-warmup-ab-$stamp"
$reportPath = Join-Path $env:TEMP "photobooth-warmup-ab-$stamp.json"
$exe = Join-Path $tempRoot 'booth-player.exe'
$keep = $false
$report = [ordered]@{
    started_utc = [DateTime]::UtcNow.ToString('o')
    status = 'FAIL'
    device = $Device
    sessions_per_variant = $SessionsPerVariant
    shots_per_session = $Shots
    variants = [ordered]@{}
    comparison = $null
    report_path = $reportPath
    artifacts_retained = $false
}

function Invoke-PlayerSession([string]$Name, [bool]$SkipWarmup, [string]$SessionsDir, [string]$OutputDir) {
    $argumentLine = '--camera=windowsmf --device="' + $Device + '" --width=1280 --height=720 --fps=30 --shots=' + $Shots + ' --countdown=0s --interval=0s --sessions-dir="' + $SessionsDir + '" --output-dir="' + $OutputDir + '"'
    if ($SkipWarmup) { $argumentLine += ' --skip-camera-warmup' }
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $exe
    $startInfo.Arguments = $argumentLine
    $startInfo.WorkingDirectory = $repoRoot
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    if (!$process.Start()) { throw "Could not start player for $Name" }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    if (!$process.WaitForExit(30000)) {
        try { $process.Kill() } catch {}
        $process.WaitForExit()
        $stdout = $stdoutTask.Result; $stderr = $stderrTask.Result; $process.Dispose()
        throw "$Name exceeded 30 seconds. Stdout: $stdout Stderr: $stderr"
    }
    $process.WaitForExit()
    $exitCode = $process.ExitCode
    $stdout = $stdoutTask.Result
    $stderr = $stderrTask.Result
    $process.Dispose()
    if ($exitCode -ne 0) { throw "$Name failed (exit=$exitCode). Stdout: $stdout Stderr: $stderr" }
    $captureFiles = @(Get-ChildItem $SessionsDir -Recurse -File -Filter 'capture-*.jpg' -ErrorAction SilentlyContinue)
    $renderedFiles = @(Get-ChildItem $SessionsDir -Recurse -File -Filter 'rendered.jpg' -ErrorAction SilentlyContinue)
    $outputFiles = @(Get-ChildItem $OutputDir -Recurse -File -Filter 'photo.jpg' -ErrorAction SilentlyContinue)
    if ($captureFiles.Count -ne $Shots -or $renderedFiles.Count -ne 1 -or $outputFiles.Count -ne 1) {
        throw "$Name produced captures=$($captureFiles.Count), renders=$($renderedFiles.Count), outputs=$($outputFiles.Count); expected $Shots/1/1."
    }
    return [ordered]@{ name = $Name; session_id = $captureFiles[0].Directory.Parent.Name; captures = $captureFiles.Count; rendered = $true; local_output = $true; player_stdout = $stdout.Trim() }
}

function Get-SignedFirstDrop([object]$Session) {
    $ordered = @($Session.captures | Sort-Object shot)
    $firstRow = $ordered | Where-Object { $_.shot -eq 1 } | Select-Object -First 1
    $first = [double]$firstRow.mean_luma
    $laterValues = @($ordered | Where-Object { $_.shot -ge 2 } | ForEach-Object { [double]$_.mean_luma } | Sort-Object)
    if ($laterValues.Count -eq 0) { return $null }
    $laterMedian = if ($laterValues.Count % 2 -eq 1) { $laterValues[[int][math]::Floor($laterValues.Count / 2)] } else { ($laterValues[($laterValues.Count / 2) - 1] + $laterValues[$laterValues.Count / 2]) / 2 }
    if ($laterMedian -le 0) { return $null }
    return [math]::Round((($laterMedian - $first) / $laterMedian) * 100, 3)
}

try {
    $camera = @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction SilentlyContinue | Where-Object { $_.FriendlyName -like '*B525*' })
    if ($camera.Count -ne 1 -or $camera[0].Status -ne 'OK') { throw "Expected exactly one healthy B525 camera; found $($camera.Count)." }
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    Write-Host "WARMUP_AB_TEST=START device=$Device sessions_per_variant=$SessionsPerVariant shots=$Shots"
    Write-Host 'Keep the scene, lighting, camera aim and distance fixed for the entire alternating run.'
    Push-Location $repoRoot
    try {
        go build -o $exe ./apps/booth-player
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }

    $arms = [ordered]@{
        warmup_disabled = [ordered]@{ skip = $true; root = (Join-Path $tempRoot 'warmup-disabled'); sessions = [System.Collections.Generic.List[object]]::new() }
        warmup_enabled  = [ordered]@{ skip = $false; root = (Join-Path $tempRoot 'warmup-enabled'); sessions = [System.Collections.Generic.List[object]]::new() }
    }
    foreach ($arm in $arms.Values) {
        New-Item -ItemType Directory -Path $arm.root -Force | Out-Null
    }

    for ($index = 1; $index -le $SessionsPerVariant; $index++) {
        # Alternate order by pair to reduce systematic time-of-run bias.
        $order = if ($index % 2 -eq 1) { @('warmup_disabled','warmup_enabled') } else { @('warmup_enabled','warmup_disabled') }
        foreach ($variant in $order) {
            $arm = $arms[$variant]
            $name = "$variant-$index"
            $runRoot = Join-Path $arm.root ("run-{0:D2}" -f $index)
            $sessionsDir = Join-Path $runRoot 'sessions'
            $outputDir = Join-Path $runRoot 'output'
            New-Item -ItemType Directory -Path $sessionsDir,$outputDir -Force | Out-Null
            $result = Invoke-PlayerSession $name $arm.skip $sessionsDir $outputDir
            $arm.sessions.Add($result) | Out-Null
            Write-Host "$name=PASS session=$($result.session_id) captures=$($result.captures)"
        }
    }

    $variantReports = [ordered]@{}
    foreach ($variant in @('warmup_disabled','warmup_enabled')) {
        $arm = $arms[$variant]
        $metricsPath = Join-Path $tempRoot "$variant-metrics.json"
        & $python $qualityScript $arm.root --min-mean-luma $MinMeanLuma --max-first-shot-drop-pct $MaxFirstShotDropPct --json-out $metricsPath | Out-Null
        $validatorExit = $LASTEXITCODE
        if (!(Test-Path $metricsPath)) { throw "Quality validator failed to write metrics for $variant" }
        $metrics = Get-Content $metricsPath -Raw | ConvertFrom-Json
        $signedDrops = @($metrics.sessions | ForEach-Object { Get-SignedFirstDrop $_ } | Where-Object { $_ -ne $null })
        $firstValues = @($metrics.sessions | ForEach-Object { [double](($_.captures | Where-Object { $_.shot -eq 1 } | Select-Object -First 1).mean_luma) })
        $laterValues = @($metrics.sessions | ForEach-Object {
            $v = @($_.captures | Where-Object { $_.shot -ge 2 } | ForEach-Object { [double]$_.mean_luma } | Sort-Object)
            if ($v.Count -eq 2) { ($v[0] + $v[1]) / 2 }
            elseif ($v.Count -gt 0) { $v[[int][math]::Floor($v.Count / 2)] }
        })
        $variantReports[$variant] = [ordered]@{
            warmup_enabled = ($variant -eq 'warmup_enabled')
            sessions = $metrics.sessions
            capture_count = $metrics.capture_count
            quality_gate = $metrics.status
            validator_exit_code = $validatorExit
            mean_first_shot_luma = [math]::Round(($firstValues | Measure-Object -Average).Average,3)
            mean_later_shot_luma = [math]::Round(($laterValues | Measure-Object -Average).Average,3)
            mean_signed_first_shot_drop_pct = [math]::Round(($signedDrops | Measure-Object -Average).Average,3)
            signed_first_shot_drop_pct_by_session = $signedDrops
        }
    }

    $off = [double]$variantReports.warmup_disabled.mean_signed_first_shot_drop_pct
    $on = [double]$variantReports.warmup_enabled.mean_signed_first_shot_drop_pct
    $report.variants = $variantReports
    $report.comparison = [ordered]@{
        warmup_effect_on_first_shot_drop_pct = [math]::Round(($off - $on),3)
        interpretation = 'Positive means the enabled warm-up reduced first-shot darkness relative to later shots; negative means it increased the measured drop. This is a small same-scene diagnostic, not universal image-quality proof.'
        order = 'Alternated warm-up-disabled/enabled order across pairs to reduce run-order bias.'
        countdown = '0s'; interval = '0s'; warmup_enabled_duration = '1s default'
    }
    $report.status = 'COMPLETED'
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report | ConvertTo-Json -Depth 16 | Set-Content -Path $reportPath -Encoding utf8
    Write-Host "WARMUP_AB_TEST=COMPLETED"
    Write-Host "NO_WARMUP_MEAN_FIRST_DROP=$off%"
    Write-Host "ONE_SECOND_WARMUP_MEAN_FIRST_DROP=$on%"
    Write-Host "MEASURED_WARMUP_EFFECT=$([math]::Round(($off - $on),3)) percentage points (positive favours warm-up)"
    Write-Host "REPORT=$reportPath"
    if ($KeepArtifacts) { $keep = $true; $report.artifacts_retained = $true; $report | ConvertTo-Json -Depth 16 | Set-Content -Path $reportPath -Encoding utf8; Write-Host "ARTIFACTS_RETAINED=$tempRoot" }
} catch {
    $failure = $_.Exception.Message
    $report.status = 'FAIL'
    $report.failure = $failure
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    $report | ConvertTo-Json -Depth 16 | Set-Content -Path $reportPath -Encoding utf8
    Write-Host 'WARMUP_AB_TEST=FAIL'
    Write-Host "FAILURE=$failure"
    Write-Host "REPORT=$reportPath"
} finally {
    if (!$keep -and (Test-Path $tempRoot)) { Remove-Item $tempRoot -Recurse -Force -ErrorAction SilentlyContinue }
}

if ($report.status -ne 'COMPLETED') { exit 1 }
exit 0
