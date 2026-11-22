$ErrorActionPreference = 'Stop'

$initialLocation = Get-Location
$oldDataDir = $env:PHOTOBOOTH_DATA_DIR
$oldSyntheticOnly = $env:PHOTOBOOTH_SYNTHETIC_ONLY
$process = $null
$profile = $null

try {
    $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
    $appExe = Join-Path $PSScriptRoot 'build\bin\wails-v2.exe'

    if (-not (Test-Path $appExe)) {
        throw "Wails build not found: $appExe. Run 'wails build -clean' from tools/ui-spike/wails-v2 first."
    }

    $profileName = 'PhotoBooth-OperatorAcceptance-' + [Guid]::NewGuid().ToString('N')
    $profile = Join-Path $env:TEMP $profileName
    New-Item -ItemType Directory -Path $profile | Out-Null

    $env:PHOTOBOOTH_DATA_DIR = $profile
    $env:PHOTOBOOTH_SYNTHETIC_ONLY = '1'

    Set-Location $repoRoot

    Write-Host ''
    Write-Host 'PhotoBooth Operator Authentication Acceptance' -ForegroundColor Cyan
    Write-Host '---------------------------------------------'
    Write-Host 'A disposable profile will be created under %TEMP%.'
    Write-Host 'Do NOT enter the real booth PIN. Choose a temporary 8-digit test PIN.'
    Write-Host 'The profile and test credential are deleted when this script exits.'
    Write-Host ''
    Write-Host 'First-time setup: enter and confirm your temporary test PIN below.'
    Write-Host ''

    & go run ./cmd/operator-setup
    if ($LASTEXITCODE -ne 0) {
        throw "operator-setup failed with exit code $LASTEXITCODE"
    }

    Write-Host ''
    Write-Host 'Synthetic-only mode is enabled. The physical camera will not be opened.'
    Write-Host 'IMPORTANT: the app intentionally opens in Customer Mode after every start.'
    Write-Host 'To open Operator PIN, tap/click the purple PB logo at the top-left FIVE times'
    Write-Host 'within 2.5 seconds. The PIN keypad appears only after this hidden gesture.'
    Write-Host ''
    Write-Host 'Manual checks to perform in the app:'
    Write-Host '  1. Five-tap the PB logo, enter an incorrect PIN, and verify the generic error.'
    Write-Host '  2. Close the app, answer Y to restart, repeat the PB gesture and verify backoff persists.'
    Write-Host '  3. Enter the correct test PIN and confirm Operator Settings opens only after authentication.'
    Write-Host '  4. Change the PIN, return to Customer Mode, then authenticate with the new PIN.'
    Write-Host '  5. Leave Operator Mode idle for five minutes and confirm it returns to Customer Mode.'
    Write-Host '  6. Re-enter Operator Mode and test Exit application.'
    Write-Host ''

    do {
        $process = Start-Process -FilePath $appExe -WorkingDirectory (Split-Path $appExe) -PassThru
        Write-Host "PhotoBooth running (PID $($process.Id)). Close the app to continue the acceptance checklist."
        Wait-Process -Id $process.Id
        $process = $null

        $answer = Read-Host 'Restart PhotoBooth with the same disposable credential/profile? (Y/N)'
    } while ($answer -match '^(?i:y|yes)$')
}
finally {
    if ($process -and (Get-Process -Id $process.Id -ErrorAction SilentlyContinue)) {
        Stop-Process -Id $process.Id -Force
    }

    if ($profile -and (Test-Path $profile)) {
        Remove-Item -LiteralPath $profile -Recurse -Force
        Write-Host 'Disposable PhotoBooth profile and test credential removed.' -ForegroundColor Green
    }

    if ($null -eq $oldDataDir) {
        Remove-Item Env:PHOTOBOOTH_DATA_DIR -ErrorAction SilentlyContinue
    } else {
        $env:PHOTOBOOTH_DATA_DIR = $oldDataDir
    }

    if ($null -eq $oldSyntheticOnly) {
        Remove-Item Env:PHOTOBOOTH_SYNTHETIC_ONLY -ErrorAction SilentlyContinue
    } else {
        $env:PHOTOBOOTH_SYNTHETIC_ONLY = $oldSyntheticOnly
    }

    Set-Location $initialLocation
}
