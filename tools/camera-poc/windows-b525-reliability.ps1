[CmdletBinding()]
param(
    [ValidateRange(1, 100)][int]$Sessions = 10,
    [ValidateRange(1, 12)][int]$Shots = 3,
    [string]$Device = "B525 HD Webcam",
    [switch]$TestContention,
    [switch]$KeepArtifacts
)
$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$python = Join-Path $repoRoot "tools\camera-poc\.venv\Scripts\python.exe"
$holderScript = Join-Path $PSScriptRoot "python\hold_camera.py"
$verifyScript = Join-Path $PSScriptRoot "python\verify_soak_artifacts.py"
if (!(Test-Path $python)) { throw "Repository Python venv is missing: $python" }
if (!(Test-Path $verifyScript)) { throw "JPEG verifier is missing: $verifyScript" }
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$root = Join-Path $env:TEMP "photobooth-b525-reliability-$stamp"
$report = Join-Path $env:TEMP "photobooth-b525-reliability-$stamp.csv"
New-Item -ItemType Directory -Force -Path $root | Out-Null
$results = [System.Collections.Generic.List[object]]::new()
$exe = Join-Path $root "booth-player.exe"
$keepRoot = $false

function Invoke-BoothSession {
    param([string]$Name, [string]$BaseDirectory)
    $sessionsDir = Join-Path $BaseDirectory "sessions"
    $outputDir = Join-Path $BaseDirectory "output"
    $stdout = Join-Path $BaseDirectory "stdout.log"
    $stderr = Join-Path $BaseDirectory "stderr.log"
    New-Item -ItemType Directory -Force -Path $BaseDirectory | Out-Null
    $argumentLine = '--camera=windowsmf --device="' + $Device + '" --width=1280 --height=720 --fps=30 --shots=' + $Shots + ' --countdown=0s --interval=0s --sessions-dir="' + $sessionsDir + '" --output-dir="' + $outputDir + '"'
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $exe
    $startInfo.Arguments = $argumentLine
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    if (!$process.Start()) { throw "Failed to start player executable: $exe" }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $peakWorkingSet = 0L
    $peakPrivateBytes = 0L
    $memorySamples = 0
    $monitor = [Diagnostics.Stopwatch]::StartNew()
    $finished = $false
    while ($monitor.Elapsed.TotalSeconds -lt 25) {
        $process.Refresh()
        if ($process.HasExited) { $finished = $true; break }
        try {
            if ($process.WorkingSet64 -gt $peakWorkingSet) { $peakWorkingSet = $process.WorkingSet64 }
            if ($process.PrivateMemorySize64 -gt $peakPrivateBytes) { $peakPrivateBytes = $process.PrivateMemorySize64 }
            $memorySamples++
        } catch {}
        if ($process.WaitForExit(25)) { $process.Refresh(); $finished = $true; break }
    }
    if (-not $finished) {
        try { if (!$process.HasExited) { $process.Kill() } } catch {}
        $process.WaitForExit()
        $clock.Stop()
        [System.IO.File]::WriteAllText($stdout, $stdoutTask.Result)
        [System.IO.File]::WriteAllText($stderr, $stderrTask.Result)
        return [PSCustomObject]@{ Name=$Name; Status="TIMEOUT"; ExitCode=-1; WallSeconds=[math]::Round($clock.Elapsed.TotalSeconds,2); Captures=0; Rendered=0; LocalCopies=0; PeakWorkingSetMB=[math]::Round($peakWorkingSet/1MB,2); PeakPrivateBytesMB=[math]::Round($peakPrivateBytes/1MB,2); MemorySamples=$memorySamples; Error="player exceeded 25 seconds"; BaseDirectory=$BaseDirectory }
    }
    $process.WaitForExit()
    $stdoutText = $stdoutTask.Result
    $stderrText = $stderrTask.Result
    [System.IO.File]::WriteAllText($stdout, $stdoutText)
    [System.IO.File]::WriteAllText($stderr, $stderrText)
    $exitCode = $process.ExitCode
    $reportedExitCode = [string]$exitCode
    $clock.Stop()
    $monitor.Stop()
    $captureCount = @(Get-ChildItem $BaseDirectory -Recurse -File -Filter "capture-*.jpg" -ErrorAction SilentlyContinue).Count
    $renderCount = @(Get-ChildItem $BaseDirectory -Recurse -File -Filter "rendered.jpg" -ErrorAction SilentlyContinue).Count
    $copyCount = @(Get-ChildItem $BaseDirectory -Recurse -File -Filter "photo.jpg" -ErrorAction SilentlyContinue).Count
    $status = "PASS"
    if (($null -ne $exitCode -and $exitCode -ne 0) -or $captureCount -ne $Shots -or $renderCount -ne 1 -or $copyCount -ne 1) { $status = "FAIL" }
    $errorText = ""
    if (Test-Path $stderr) {
        $stderrContent = Get-Content $stderr -Raw -ErrorAction SilentlyContinue
        if (![string]::IsNullOrWhiteSpace($stderrContent)) { $errorText = $stderrContent.Trim() }
    }
    if ($status -eq "FAIL" -and !$errorText -and (Test-Path $stdout)) {
        $stdoutContent = Get-Content $stdout -Raw -ErrorAction SilentlyContinue
        if (![string]::IsNullOrWhiteSpace($stdoutContent)) { $errorText = $stdoutContent.Trim() }
    }
    return [PSCustomObject]@{ Name=$Name; Status=$status; ExitCode=$reportedExitCode; WallSeconds=[math]::Round($clock.Elapsed.TotalSeconds,2); Captures=$captureCount; Rendered=$renderCount; LocalCopies=$copyCount; PeakWorkingSetMB=[math]::Round($peakWorkingSet/1MB,2); PeakPrivateBytesMB=[math]::Round($peakPrivateBytes/1MB,2); MemorySamples=$memorySamples; Error=$errorText; BaseDirectory=$BaseDirectory }
}

try {
    Write-Host "Building standalone PhotoBooth player..."
    Push-Location $repoRoot
    try {
        go build -o $exe ./apps/booth-player
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }

    Write-Host "Running $Sessions real camera sessions ($Shots shots/session) on '$Device'."
    for ($i=1; $i -le $Sessions; $i++) {
        $name = "soak-{0:D3}" -f $i
        $result = Invoke-BoothSession -Name $name -BaseDirectory (Join-Path $root $name)
        $results.Add($result)
        Write-Host ("{0}/{1}: {2}, exit={3}, captures={4}, rendered={5}, local={6}, wall={7:N2}s" -f $i,$Sessions,$result.Status,$result.ExitCode,$result.Captures,$result.Rendered,$result.LocalCopies,$result.WallSeconds)
    }

    if ($TestContention) {
        Write-Host "Starting a separate Media Foundation holder to test contention."
        $holderOut = Join-Path $root "holder.stdout.log"
        $holderErr = Join-Path $root "holder.stderr.log"
        $holderArgs = '"' + $holderScript + '" --backend msmf --seconds 12'
        $holder = Start-Process -FilePath $python -ArgumentList $holderArgs -PassThru -WindowStyle Hidden -RedirectStandardOutput $holderOut -RedirectStandardError $holderErr
        Start-Sleep -Seconds 2
        try {
            $busy = Invoke-BoothSession -Name "contention" -BaseDirectory (Join-Path $root "contention")
            if ($busy.Status -eq "FAIL" -and $busy.Error -match "0xC00D3704|3222091524") { $busy.Status = "EXPECTED_BUSY_ERROR" }
            elseif ($busy.Status -eq "PASS") { $busy.Status = "CONTENTION_NOT_REPRODUCED" }
            $results.Add($busy)
            Write-Host ("Contention attempt: {0}; exit={1}; error={2}" -f $busy.Status,$busy.ExitCode,$busy.Error)
        } finally {
            $holder.WaitForExit(20000) | Out-Null
            if (!$holder.HasExited) { Stop-Process -Id $holder.Id -Force -ErrorAction SilentlyContinue }
            if (Test-Path $holderOut) { Get-Content $holderOut | ForEach-Object { Write-Host "HOLDER: $_" } }
            if (Test-Path $holderErr) { Get-Content $holderErr | ForEach-Object { Write-Host "HOLDER ERROR: $_" } }
        }
        $recovery = Invoke-BoothSession -Name "after-contention-release" -BaseDirectory (Join-Path $root "after-contention-release")
        $results.Add($recovery)
        Write-Host ("After release: {0}, exit={1}, captures={2}, local={3}, wall={4:N2}s" -f $recovery.Status,$recovery.ExitCode,$recovery.Captures,$recovery.LocalCopies,$recovery.WallSeconds)
    }

    $imagePaths = @(Get-ChildItem $root -Recurse -File -Filter "*.jpg" | Where-Object { $_.Name -match "^capture-[0-9]+\.jpg$|^rendered\.jpg$|^photo\.jpg$" })
    if ($imagePaths.Count -eq 0) { throw "No JPEG artifacts were produced; camera or player may not be working." }
    Write-Host "Checking JPEG decode and expected dimensions for $($imagePaths.Count) temporary images."
    & $python $verifyScript $root
    if ($LASTEXITCODE -ne 0) { throw "JPEG validation failed with exit code $LASTEXITCODE" }

    $results | Select-Object Name,Status,ExitCode,WallSeconds,Captures,Rendered,LocalCopies,PeakWorkingSetMB,PeakPrivateBytesMB,MemorySamples,Error | Export-Csv -NoTypeInformation -Encoding utf8 $report
    Write-Host ""
    Write-Host "--- SUMMARY ---"
    $results | Select-Object Name,Status,ExitCode,WallSeconds,Captures,Rendered,LocalCopies,PeakWorkingSetMB,PeakPrivateBytesMB | Format-Table -AutoSize
    $bad = @($results | Where-Object { $_.Status -eq "FAIL" -or $_.Status -eq "TIMEOUT" }).Count
    Write-Host "Failed/timeouts: $bad; CSV report: $report"
    if ($KeepArtifacts) { $keepRoot = $true; Write-Host "Temporary JPEGs/logs retained at: $root" } else { Write-Host "Temporary images/logs will be deleted." }
    if ($bad -gt 0) { exit 1 }
} finally {
    if (!$keepRoot) { Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue }
}
