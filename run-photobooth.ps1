# Build and launch the Windows PhotoBooth UI candidate.
# The Wails module stays isolated so the root Go core remains on Go 1.24.8.
[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [switch]$NoLaunch,
    [switch]$Dev,
    [switch]$Kiosk,
    [switch]$Windowed
)

$ErrorActionPreference = 'Stop'
if ($Kiosk -and $Windowed) {
    throw '-Kiosk and -Windowed cannot be combined.'
}
# Production/default launches are fullscreen Customer Mode. Windowed is an
# explicit development/diagnostic opt-out; -Kiosk remains for compatibility.
$kioskEnvironmentValue = if ($Windowed) { '0' } else { '1' }
$repoRoot = $PSScriptRoot
$uiDir = Join-Path $repoRoot 'tools\ui-spike\wails-v2'
$binaryDir = Join-Path $uiDir 'build\bin'
$exePath = Join-Path $binaryDir 'PhotoBooth.exe'
$legacyExePath = Join-Path $binaryDir 'wails-v2.exe'

if (-not (Test-Path (Join-Path $uiDir 'go.mod'))) {
    throw "Wails UI module was not found: $uiDir"
}

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCommand) {
    throw 'Go was not found on PATH. Install Go and reopen PowerShell.'
}
if (-not (Get-Command node -ErrorAction SilentlyContinue) -or
    -not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw 'Node.js and npm are required to build the Wails frontend.'
}

Push-Location $uiDir
try {
    # Run inside the nested module: GOTOOLCHAIN=auto selects its Go 1.25
    # toolchain without changing the root/core module's Go 1.24.8 baseline.
    $moduleGo = (& go version 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $moduleGo -notmatch 'go version go([0-9]+\.[0-9]+\.[0-9]+)') {
        throw "Could not determine the Go toolchain for the Wails module. Output: $moduleGo"
    }
    $moduleVersion = [version]$Matches[1]
    if ($moduleVersion -lt [version]'1.25.0') {
        throw "The Wails UI requires Go 1.25.0 or newer; this module selected $moduleVersion."
    }

    $goPath = (& go env GOPATH).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goPath)) {
        throw 'Could not resolve GOPATH.'
    }
    $wails = Join-Path $goPath 'bin\wails.exe'
    if (-not (Test-Path $wails)) {
        $wailsCommand = Get-Command wails -ErrorAction SilentlyContinue
        if ($wailsCommand) {
            $wails = $wailsCommand.Source
        } else {
            throw "Wails CLI v2.17.0 was not found at $wails. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0"
        }
    }

    if ($Dev) {
        if ($SkipBuild -or $NoLaunch) {
            throw '-Dev cannot be combined with -SkipBuild or -NoLaunch.'
        }
        Write-Host "Starting PhotoBooth Windows UI in Wails development mode ($moduleGo)..."
        $previousKioskValue = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_KIOSK', 'Process')
        [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $kioskEnvironmentValue, 'Process')
        try {
            if ($Windowed) { Write-Host 'Windowed diagnostic mode enabled.' }
            else { Write-Host 'Customer Mode starts fullscreen; window mode is Operator-only.' }
            & $wails dev
            if ($LASTEXITCODE -ne 0) {
                throw "Wails dev exited with code $LASTEXITCODE."
            }
        }
        finally {
            [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $previousKioskValue, 'Process')
        }
        return
    }

    if (-not $SkipBuild) {
        Write-Host "Building PhotoBooth Windows UI with $moduleGo..."
        # Use a distinct filename so an older wails-v2.exe that is still open
        # does not prevent the updated candidate from being built.
        & $wails build -nopackage -o PhotoBooth.exe
        if ($LASTEXITCODE -ne 0) {
            throw "Wails build exited with code $LASTEXITCODE."
        }
    }

    if (-not (Test-Path $exePath)) {
        if ($SkipBuild -and (Test-Path $legacyExePath)) {
            throw "The new PhotoBooth.exe has not been built yet. Run .\run-photobooth.ps1 first."
        }
        throw "Build output was not found: $exePath"
    }

    Write-Host "Build ready: $exePath"
    if ($NoLaunch) {
        Write-Host 'Build-only mode: the application was not started.'
        return
    }

    # Do not silently open a second instance against the same local station
    # profile/camera. Never kill an existing process or discard its UI state.
    $running = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('wails-v2.exe', 'PhotoBooth.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath.StartsWith($binaryDir, [StringComparison]::OrdinalIgnoreCase)
    })
    if ($running.Count -gt 0) {
        $pids = ($running | ForEach-Object { $_.ProcessId }) -join ', '
        Write-Warning "An older PhotoBooth UI instance is still open (PID: $pids). Close that window first, then run: .\run-photobooth.ps1 -SkipBuild"
        return
    }

    $previousKioskValue = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_KIOSK', 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $kioskEnvironmentValue, 'Process')
    try {
        $process = Start-Process -FilePath $exePath -WorkingDirectory $repoRoot -PassThru
    }
    finally {
        [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $previousKioskValue, 'Process')
    }
    Write-Host "PhotoBooth Windows Preview started (PID $($process.Id))."
    Write-Host 'The idle screen starts the local webcam preview. Preview stops before capture and while Settings is open.'
    if ($Windowed) {
        Write-Host 'Windowed diagnostic mode is active. Guest mode changes remain restricted.'
    } else {
        Write-Host 'Fullscreen Customer Mode is active. Authenticate as Operator before changing window mode or exiting.'
    }
    Write-Host 'This is an experimental Windows MVP candidate, not a production release.'
}
finally {
    Pop-Location
}
