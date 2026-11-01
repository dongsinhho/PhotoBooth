[CmdletBinding()]
param(
    [ValidateRange(1,1000)][int]$Sessions = 1000,
    [ValidateRange(1,12)][int]$Shots = 3,
    [ValidateRange(10,60)][int]$WatchdogMinutes = 60,
    [string]$Device = 'B525 HD Webcam'
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$tempRoot = Join-Path $env:TEMP "photobooth-endurance-$stamp"
$preflightRoot = Join-Path $tempRoot 'preflight'
$soakRoot = Join-Path $tempRoot 'soak'
$exe = Join-Path $tempRoot 'persistent-soak.exe'
$preflightOut = Join-Path $tempRoot 'preflight.stdout.log'
$preflightErr = Join-Path $tempRoot 'preflight.stderr.log'
$soakOut = Join-Path $tempRoot 'soak.stdout.log'
$soakErr = Join-Path $tempRoot 'soak.stderr.log'
$metricsCsv = Join-Path $tempRoot 'go-session-metrics.csv'
$osCsv = Join-Path $tempRoot 'windows-process-memory.csv'
$heapProfile = Join-Path $tempRoot 'heap-after-soak.pprof'
$summaryJson = Join-Path $tempRoot 'summary.json'
$reportMd = Join-Path $tempRoot 'report.md'
$process = $null
$osWriter = $null
$sampleCount = 0
$peakWorkingSet = [long]0
$peakPrivate = [long]0
$firstWorkingSet = $null
$lastWorkingSet = $null
$firstPrivate = $null
$lastPrivate = $null
$watchdogFired = $false
$exitCode = $null
$startedUtc = [DateTime]::UtcNow
$summary = [ordered]@{
    status = 'FAIL'
    device = $Device
    sessions_requested = $Sessions
    shots_per_session = $Shots
    captures_expected = $Sessions * $Shots
    started_utc = $startedUtc.ToString('o')
    preflight = 'NOT_RUN'
    process_exit_code = $null
    completed_sessions = 0
    failed_sessions = 0
    capture_jpegs_verified = 0
    rendered_jpegs_verified = 0
    local_jpegs_verified = 0
    total_jpegs_verified = 0
    go_session_wall_ms_mean = $null
    go_session_wall_ms_p50 = $null
    go_session_wall_ms_p95 = $null
    heap_after_gc_first_mb = $null
    heap_after_gc_last_mb = $null
    heap_after_gc_delta_mb = $null
    goroutines_first = $null
    goroutines_last = $null
    os_memory_sample_count = 0
    working_set_peak_mb = $null
    private_bytes_peak_mb = $null
    working_set_first_mb = $null
    working_set_last_mb = $null
    private_bytes_first_mb = $null
    private_bytes_last_mb = $null
    watchdog_fired = $false
    go_metrics_csv = $metricsCsv
    os_memory_csv = $osCsv
    go_stdout_log = $soakOut
    go_stderr_log = $soakErr
    heap_profile = $heapProfile
    report_path = $reportMd
    failure = $null
}

function Invoke-OneSessionPreflight {
    $work = Join-Path $preflightRoot 'sessions'
    $out = Join-Path $preflightRoot 'output'
    $csv = Join-Path $preflightRoot 'metrics.csv'
    $stdout = Join-Path $preflightRoot 'stdout.log'
    $stderr = Join-Path $preflightRoot 'stderr.log'
    New-Item -ItemType Directory -Force -Path $preflightRoot | Out-Null
    $args = '--sessions=1 --shots=' + $Shots + ' --device="' + $Device + '" --work-dir="' + $work + '" --output-dir="' + $out + '" --metrics-csv="' + $csv + '"'
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $exe
    $startInfo.Arguments = $args
    $startInfo.WorkingDirectory = $repoRoot
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $p = [System.Diagnostics.Process]::new()
    $p.StartInfo = $startInfo
    if (-not $p.Start()) { throw 'Failed to start persistent-soak preflight executable.' }
    $stdoutTask = $p.StandardOutput.ReadToEndAsync()
    $stderrTask = $p.StandardError.ReadToEndAsync()
    if (-not $p.WaitForExit(90000)) {
        try { $p.Kill() } catch {}
        $p.WaitForExit()
        $stdoutText = $stdoutTask.Result
        $stderrText = $stderrTask.Result
        $p.Dispose()
        throw "One-session camera preflight exceeded 90 seconds. Stdout: $stdoutText; Stderr: $stderrText"
    }
    $p.WaitForExit()
    $exitCode = [int]$p.ExitCode
    $stdoutText = $stdoutTask.Result
    $stderrText = $stderrTask.Result
    Set-Content -Path $stdout -Value $stdoutText -Encoding utf8
    Set-Content -Path $stderr -Value $stderrText -Encoding utf8
    if ($exitCode -ne 0) {
        throw "Camera preflight failed (exit=$exitCode). Stdout: $stdoutText; Stderr: $stderrText"
    }
    if ($stdoutText -notmatch 'PERSISTENT_SOAK_VERIFY successful_sessions=1 capture_jpegs=3 rendered_jpegs=1 local_jpegs=1 total_jpegs=5') {
        throw "Camera preflight exit was zero but expected JPEG verification summary was not found. Log: $stdout"
    }
    $summary.preflight = 'PASS (1 real camera session; 3 captures, render, local output verified)'
    Remove-Item $preflightRoot -Recurse -Force -ErrorAction SilentlyContinue
}

try {
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
    Write-Host "ENDURANCE_START sessions=$Sessions shots_per_session=$Shots device='$Device'"
    Write-Host "Temporary report root: $tempRoot"
    Write-Host 'PRECHECK: camera presence and free disk space.'
    $devices = @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction SilentlyContinue)
    if ($devices.Count -eq 0) {
        $devices = @(Get-PnpDevice -PresentOnly -Class Image -ErrorAction SilentlyContinue)
    }
    if ($devices.Count -eq 0) {
        throw 'Windows reports no present Camera/Image device. Connect the B525 before running the endurance batch.'
    }
    Write-Host ("PRESENT_CAMERA_DEVICES=" + (($devices | ForEach-Object { $_.Status + ':' + $_.FriendlyName }) -join '; '))
    $driveName = [IO.Path]::GetPathRoot($env:TEMP).TrimEnd('\').TrimEnd(':')
    $drive = Get-PSDrive -Name $driveName
    if ($drive.Free -lt 3GB) {
        throw "Only $([math]::Round($drive.Free/1GB,2)) GB is free on drive $driveName; at least 3 GB is required."
    }
    Write-Host ("TEMP_FREE_GB=" + [math]::Round($drive.Free/1GB,2))

    Push-Location $repoRoot
    try {
        Write-Host 'BUILDING persistent-soak.exe'
        go build -o $exe ./tools/camera-poc/persistent-soak
        if ($LASTEXITCODE -ne 0) { throw "go build persistent-soak failed with exit $LASTEXITCODE" }
    } finally { Pop-Location }

    Invoke-OneSessionPreflight
    Write-Host 'PREFLIGHT=PASS; starting persistent single-process endurance run.'

    $workDir = Join-Path $soakRoot 'sessions'
    $outputDir = Join-Path $soakRoot 'output'
    New-Item -ItemType Directory -Force -Path $soakRoot | Out-Null
    $arguments = '--sessions=' + $Sessions + ' --shots=' + $Shots + ' --device="' + $Device + '" --work-dir="' + $workDir + '" --output-dir="' + $outputDir + '" --metrics-csv="' + $metricsCsv + '" --heap-profile="' + $heapProfile + '"'
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $exe
    $startInfo.Arguments = $arguments
    $startInfo.WorkingDirectory = $repoRoot
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    if (-not $process.Start()) { throw 'Failed to start persistent endurance process.' }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $watch = [System.Diagnostics.Stopwatch]::StartNew()
    $lastProgress = [System.Diagnostics.Stopwatch]::StartNew()
    $osWriter = [System.IO.StreamWriter]::new($osCsv, $false, [System.Text.UTF8Encoding]::new($true))
    $osWriter.WriteLine('timestamp_utc,pid,working_set_bytes,private_bytes,virtual_bytes,cpu_seconds,thread_count,handle_count')
    $osWriter.Flush()

    while (-not $process.HasExited) {
        if ($watch.Elapsed.TotalMinutes -ge $WatchdogMinutes) {
            $watchdogFired = $true
            Write-Host "WATCHDOG fired after $WatchdogMinutes minutes; terminating the soak process."
            try { $process.Kill() } catch {}
            $process.WaitForExit()
            break
        }
        $p = Get-Process -Id $process.Id -ErrorAction SilentlyContinue
        if ($p) {
            $p.Refresh()
            $ws = [long]$p.WorkingSet64
            $priv = [long]$p.PrivateMemorySize64
            $virt = [long]$p.VirtualMemorySize64
            $cpu = if ($p.CPU -ne $null) { [double]$p.CPU } else { 0.0 }
            $threads = if ($p.Threads) { $p.Threads.Count } else { 0 }
            $handles = if ($p.HandleCount -ne $null) { $p.HandleCount } else { 0 }
            $osWriter.WriteLine(('{0},{1},{2},{3},{4},{5},{6},{7}' -f [DateTime]::UtcNow.ToString('o'),$process.Id,$ws,$priv,$virt,$cpu,$threads,$handles))
            $osWriter.Flush()
            $sampleCount++
            if ($null -eq $firstWorkingSet) { $firstWorkingSet = $ws; $firstPrivate = $priv }
            $lastWorkingSet = $ws
            $lastPrivate = $priv
            if ($ws -gt $peakWorkingSet) { $peakWorkingSet = $ws }
            if ($priv -gt $peakPrivate) { $peakPrivate = $priv }
        }
        if ($lastProgress.Elapsed.TotalSeconds -ge 20) {
            $lastProgress.Restart()
            $completedRows = 0
            if (Test-Path $metricsCsv) {
                try { $lineCount = (Get-Content $metricsCsv -ErrorAction Stop | Measure-Object).Count; $completedRows = [math]::Max(0, $lineCount - 1) } catch {}
            }
            Write-Host ("PROGRESS elapsed_min={0:N1} metric_rows={1}/{2}" -f $watch.Elapsed.TotalMinutes,$completedRows,$Sessions)
        }
        [void]$process.WaitForExit(250)
    }
    $process.WaitForExit()
    $exitCode = [int]$process.ExitCode
    $stdoutText = $stdoutTask.Result
    $stderrText = $stderrTask.Result
    $process.Dispose()
    Set-Content -Path $soakOut -Value $stdoutText -Encoding utf8
    Set-Content -Path $soakErr -Value $stderrText -Encoding utf8
    $summary.process_exit_code = $exitCode
    $summary.watchdog_fired = $watchdogFired
    $summary.os_memory_sample_count = $sampleCount
    if ($sampleCount -gt 0) {
        $summary.working_set_peak_mb = [math]::Round($peakWorkingSet/1MB,2)
        $summary.private_bytes_peak_mb = [math]::Round($peakPrivate/1MB,2)
        $summary.working_set_first_mb = [math]::Round($firstWorkingSet/1MB,2)
        $summary.working_set_last_mb = [math]::Round($lastWorkingSet/1MB,2)
        $summary.private_bytes_first_mb = [math]::Round($firstPrivate/1MB,2)
        $summary.private_bytes_last_mb = [math]::Round($lastPrivate/1MB,2)
    }

    if ($exitCode -ne 0 -or $watchdogFired) {
        $stdoutText = if (Test-Path $soakOut) { Get-Content $soakOut -Tail 35 | Out-String } else { '' }
        $stderrText = if (Test-Path $soakErr) { Get-Content $soakErr -Raw } else { '' }
        throw "Persistent soak failed or was stopped (exit=$exitCode, watchdog=$watchdogFired). Recent stdout: $stdoutText; stderr: $stderrText"
    }

    $go = @(Import-Csv $metricsCsv)
    $summary.completed_sessions = @($go | Where-Object { $_.state -eq 'completed' -or $_.state -eq 'partial_success' }).Count
    $summary.failed_sessions = @($go | Where-Object { $_.error -ne '' }).Count
    if ($go.Count -lt $Sessions) { throw "Only $($go.Count) Go metric rows found; requested $Sessions sessions." }
    $rows = @($go | Where-Object { [int]$_.index -le $Sessions })
    $wall = @($rows | ForEach-Object { [double]$_.wall_ms } | Sort-Object)
    if ($wall.Count -gt 0) {
        $summary.go_session_wall_ms_mean = [math]::Round((($wall | Measure-Object -Average).Average),3)
        $summary.go_session_wall_ms_p50 = [math]::Round($wall[[math]::Floor(($wall.Count-1)*0.50)],3)
        $summary.go_session_wall_ms_p95 = [math]::Round($wall[[math]::Floor(($wall.Count-1)*0.95)],3)
    }
    $firstRow = $rows | Select-Object -First 1
    $lastRow = $rows | Select-Object -Last 1
    if ($firstRow) {
        $summary.heap_after_gc_first_mb = [math]::Round(([double]$firstRow.heap_alloc_after_gc)/1MB,3)
        $summary.goroutines_first = [int]$firstRow.goroutines_after_gc
    }
    if ($lastRow) {
        $summary.heap_after_gc_last_mb = [math]::Round(([double]$lastRow.heap_alloc_after_gc)/1MB,3)
        $summary.heap_after_gc_delta_mb = [math]::Round(($summary.heap_after_gc_last_mb-$summary.heap_after_gc_first_mb),3)
        $summary.goroutines_last = [int]$lastRow.goroutines_after_gc
    }

    $verifyLine = Select-String -Path $soakOut -Pattern '^PERSISTENT_SOAK_VERIFY ' | Select-Object -Last 1
    if (!$verifyLine) { throw "Soak completed but JPEG verifier summary is missing. Log: $soakOut" }
    $v = @{}
    foreach ($match in [regex]::Matches($verifyLine.Line,'([a-z_]+)=([^ ]+)')) { $v[$match.Groups[1].Value] = $match.Groups[2].Value }
    $summary.capture_jpegs_verified = [int]$v.capture_jpegs
    $summary.rendered_jpegs_verified = [int]$v.rendered_jpegs
    $summary.local_jpegs_verified = [int]$v.local_jpegs
    $summary.total_jpegs_verified = [int]$v.total_jpegs
    if ($summary.completed_sessions -ne $Sessions -or $summary.failed_sessions -ne 0 -or
        $summary.capture_jpegs_verified -ne ($Sessions*$Shots) -or
        $summary.rendered_jpegs_verified -ne $Sessions -or
        $summary.local_jpegs_verified -ne $Sessions -or
        $summary.total_jpegs_verified -ne ($Sessions*($Shots+2))) {
        throw "Verification counts do not match expectation. See $soakOut and $metricsCsv"
    }

    $summary.status = 'PASS'
    Write-Host "ENDURANCE_GATE=PASS sessions=$($summary.completed_sessions) captures=$($summary.capture_jpegs_verified) renders=$($summary.rendered_jpegs_verified) local_outputs=$($summary.local_jpegs_verified)"
} catch {
    $summary.failure = $_.Exception.Message
    Write-Host 'ENDURANCE_GATE=FAIL'
    Write-Host "FAILURE=$($summary.failure)"
} finally {
    if ($osWriter) { $osWriter.Flush(); $osWriter.Dispose() }
    if ($process) {
        try {
            if (-not $process.HasExited) {
                try { $process.Kill() } catch {}
                try { [void]$process.WaitForExit(5000) } catch {}
            }
        } catch {}
        try { $process.Dispose() } catch {}
    }
    $summary.finished_utc = [DateTime]::UtcNow.ToString('o')
    $summary.wall_clock_minutes = [math]::Round(([DateTime]::UtcNow-$startedUtc).TotalMinutes,2)
    $summary | ConvertTo-Json -Depth 8 | Set-Content -Path $summaryJson -Encoding utf8

    # Remove real camera frames on both success and failure; preserve only reports.
    if (Test-Path $preflightRoot) { Remove-Item $preflightRoot -Recurse -Force -ErrorAction SilentlyContinue }
    if (Test-Path $soakRoot) { Remove-Item $soakRoot -Recurse -Force -ErrorAction SilentlyContinue }
    if (Test-Path $exe) { Remove-Item $exe -Force -ErrorAction SilentlyContinue }

    $lines = @(
        '# PhotoBooth B525 long endurance report',
        '',
        "- Status: $($summary.status)",
        "- Device: $Device",
        "- Sessions requested: $Sessions",
        "- Shots per session: $Shots",
        "- Completed sessions: $($summary.completed_sessions)",
        "- Failed sessions: $($summary.failed_sessions)",
        "- Captures verified: $($summary.capture_jpegs_verified)",
        "- Rendered outputs verified: $($summary.rendered_jpegs_verified)",
        "- Local copies verified: $($summary.local_jpegs_verified)",
        "- Total JPEGs verified: $($summary.total_jpegs_verified)",
        "- Wall-clock minutes: $($summary.wall_clock_minutes)",
        "- Mean / P50 / P95 session ms: $($summary.go_session_wall_ms_mean) / $($summary.go_session_wall_ms_p50) / $($summary.go_session_wall_ms_p95)",
        "- Go heap after GC first/last/delta MB: $($summary.heap_after_gc_first_mb) / $($summary.heap_after_gc_last_mb) / $($summary.heap_after_gc_delta_mb)",
        "- Goroutines after GC first/last: $($summary.goroutines_first) / $($summary.goroutines_last)",
        "- Windows process memory samples: $($summary.os_memory_sample_count)",
        "- Working set first/last/peak MB: $($summary.working_set_first_mb) / $($summary.working_set_last_mb) / $($summary.working_set_peak_mb)",
        "- Private bytes first/last/peak MB: $($summary.private_bytes_first_mb) / $($summary.private_bytes_last_mb) / $($summary.private_bytes_peak_mb)",
        "- Watchdog fired: $($summary.watchdog_fired)",
        "- Summary JSON: $summaryJson",
        "- Go session metrics: $metricsCsv",
        "- Windows process memory samples: $osCsv",
        "- Go output log: $soakOut",
        "- Go error log: $soakErr",
        "- Heap profile: $heapProfile",
        "- Failure: $($summary.failure)"
    )
    $lines | Set-Content -Path $reportMd -Encoding utf8
    Write-Host "REPORT_MARKDOWN=$reportMd"
    Write-Host "REPORT_JSON=$summaryJson"
    Write-Host "METRICS_CSV=$metricsCsv"
    Write-Host "OS_MEMORY_CSV=$osCsv"
    Write-Host "TEST_PHOTOS_CLEANED=True"
    Write-Host "REPORT_ROOT=$tempRoot"
}

if ($summary.status -ne 'PASS') { exit 1 }
exit 0
