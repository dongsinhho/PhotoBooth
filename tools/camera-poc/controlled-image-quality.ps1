[CmdletBinding()]
param(
    [ValidateRange(1,20)][int]$Sessions = 3,
    [ValidateRange(3,12)][int]$Shots = 3,
    [string]$Device = "B525 HD Webcam",
    [ValidateRange(0,255)][double]$MinMeanLuma = 20,
    [ValidateRange(0,100)][double]$MaxFirstShotDropPct = 10,
    [switch]$KeepArtifacts
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$python = Join-Path $repoRoot "tools\camera-poc\.venv\Scripts\python.exe"
$qualityScript = Join-Path $PSScriptRoot "python\validate_capture_quality.py"
if (!(Test-Path $python)) { throw "Repository Python venv is missing: $python" }
if (!(Test-Path $qualityScript)) { throw "Capture quality validator is missing: $qualityScript" }

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$tempRoot = Join-Path $env:TEMP "photobooth-image-quality-$stamp"
$reportPath = Join-Path $env:TEMP "photobooth-image-quality-$stamp.json"
$exe = Join-Path $tempRoot "booth-player.exe"
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
$keep = $false

try {
    Write-Host "PhotoBooth capture quality check"
    Write-Host "Device: $Device; sessions: $Sessions; shots/session: $Shots"
    Write-Host "Keep the camera position, aim, lighting, exposure controls and scene unchanged for this run."
    Write-Host "A mean-luminance floor of $MinMeanLuma is only a gross-dark sentinel, not a universal beauty threshold."
    Write-Host "Report path: $reportPath"

    Push-Location $repoRoot
    try {
        go build -o $exe ./apps/booth-player
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }

    for ($index = 1; $index -le $Sessions; $index++) {
        $name = "session-{0:D2}" -f $index
        $base = Join-Path $tempRoot $name
        $sessionsDir = Join-Path $base "sessions"
        $outputDir = Join-Path $base "output"
        New-Item -ItemType Directory -Force -Path $base | Out-Null

        $argumentLine = '--camera=windowsmf --device="' + $Device + '" --width=1280 --height=720 --fps=30 --shots=' + $Shots + ' --countdown=0s --interval=0s --sessions-dir="' + $sessionsDir + '" --output-dir="' + $outputDir + '"'
        $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
        $startInfo.FileName = $exe
        $startInfo.Arguments = $argumentLine
        $startInfo.UseShellExecute = $false
        $startInfo.CreateNoWindow = $true
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        $process = [System.Diagnostics.Process]::new()
        $process.StartInfo = $startInfo
        if (-not $process.Start()) { throw "Failed to start player executable: $exe" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(30000)) {
            try { $process.Kill() } catch {}
            $process.WaitForExit()
            $stdoutText = $stdoutTask.Result
            $stderrText = $stderrTask.Result
            $process.Dispose()
            throw "Camera session $name exceeded the 30-second limit. Stdout: $stdoutText; Stderr: $stderrText"
        }
        # The blocking wait ensures the final process exit code and redirected
        # output are available before validating the photo artifacts.
        $process.WaitForExit()
        $exitCode = $process.ExitCode
        $stdoutText = $stdoutTask.Result
        $stderrText = $stderrTask.Result
        $process.Dispose()
        $captures = @(Get-ChildItem $sessionsDir -Recurse -File -Filter "capture-*.jpg" -ErrorAction SilentlyContinue)
        $renders = @(Get-ChildItem $sessionsDir -Recurse -File -Filter "rendered.jpg" -ErrorAction SilentlyContinue)
        $outputs = @(Get-ChildItem $outputDir -Recurse -File -Filter "photo.jpg" -ErrorAction SilentlyContinue)
        Write-Host ("{0}/{1}: exit={2}, captures={3}, renders={4}, local outputs={5}" -f $index,$Sessions,$exitCode,$captures.Count,$renders.Count,$outputs.Count)
        if ($exitCode -ne 0 -or $captures.Count -ne $Shots -or $renders.Count -ne 1 -or $outputs.Count -ne 1) {
            throw "Capture-to-output failed in $name (exit=$exitCode, captures=$($captures.Count), renders=$($renders.Count), outputs=$($outputs.Count)). Stdout: $stdoutText; Stderr: $stderrText; Artifacts: $base"
        }
    }

    & $python $qualityScript $tempRoot --min-mean-luma $MinMeanLuma --max-first-shot-drop-pct $MaxFirstShotDropPct --json-out $reportPath
    $qualityExit = $LASTEXITCODE
    Write-Host "Quality report written to: $reportPath"
    if ($KeepArtifacts) {
        $keep = $true
        Write-Host "Capture artifacts retained for visual inspection: $tempRoot"
    } else {
        Write-Host "Temporary capture artifacts will be removed; the JSON metrics report will be retained."
    }
    if ($qualityExit -ne 0) {
        Write-Host "IMAGE_QUALITY_GATE=FAIL. Review lighting/framing and the JSON metrics; thresholds were not relaxed."
        exit 1
    }
    Write-Host "IMAGE_QUALITY_GATE=PASS"
} finally {
    if (!$keep) { Remove-Item $tempRoot -Recurse -Force -ErrorAction SilentlyContinue }
}
