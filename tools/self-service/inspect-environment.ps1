[CmdletBinding()]
param(
    [string]$ConfigPath = "examples\store-config-v1.json",
    [switch]$CheckCamera
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$checks = [System.Collections.Generic.List[object]]::new()

function Add-Check {
    param([string]$Name, [ValidateSet('PASS','WARN','FAIL')][string]$Status, [string]$Details)
    $checks.Add([pscustomobject]@{ Check = $Name; Status = $Status; Details = $Details })
}

function Resolve-ConfigPath {
    param([string]$Value, [string]$Base)
    if ([IO.Path]::IsPathRooted($Value)) { return [IO.Path]::GetFullPath($Value) }
    return [IO.Path]::GetFullPath((Join-Path $Base $Value))
}

Push-Location $repoRoot
try {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if (!$go) {
        Add-Check 'Go toolchain' 'FAIL' 'go was not found on PATH'
    } else {
        $versionText = (& go version 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) {
            Add-Check 'Go toolchain' 'FAIL' $versionText
        } else {
            Add-Check 'Go toolchain' 'PASS' $versionText
        }
    }

    $configFullPath = Resolve-ConfigPath -Value $ConfigPath -Base $repoRoot
    if (!(Test-Path -LiteralPath $configFullPath -PathType Leaf)) {
        Add-Check 'Station config' 'FAIL' "Config file not found: $configFullPath"
    } else {
        try {
            $config = Get-Content -LiteralPath $configFullPath -Raw | ConvertFrom-Json
            if ([int]$config.schema_version -ne 1) { throw 'schema_version must equal 1' }
            if ([int]$config.capture.shots -lt 1 -or [int]$config.capture.shots -gt 12) { throw 'capture.shots must be 1..12' }
            if ($config.camera.provider -notin @('fake','windowsmf')) { throw 'camera.provider must be fake or windowsmf' }
            if ($config.media_delivery.mode -ne 'local') { throw 'only local media delivery is currently implemented' }
            if ($config.payment.mode -ne 'disabled') { throw 'only disabled/free payment is currently implemented' }
            Add-Check 'Station config' 'PASS' "Schema v1; provider=$($config.camera.provider); shots=$($config.capture.shots); media=local; payment=disabled"

            $configBase = Split-Path -Parent $configFullPath
            $framePath = Resolve-ConfigPath -Value ([string]$config.frame.config_path) -Base $configBase
            if (!(Test-Path -LiteralPath $framePath -PathType Leaf)) {
                Add-Check 'Frame file' 'FAIL' "Frame file not found: $framePath"
            } else {
                try {
                    $frame = Get-Content -LiteralPath $framePath -Raw | ConvertFrom-Json
                    $slotCount = @($frame.slots).Count
                    if ($frame.schema_version -ne 1) { throw 'Frame schema_version must equal 1' }
                    if ($slotCount -ne [int]$config.capture.shots) {
                        throw "Configured shots=$($config.capture.shots), frame slots=$slotCount"
                    }
                    Add-Check 'Frame file' 'PASS' "Schema v1; $slotCount slot(s); shot count matches"
                } catch {
                    Add-Check 'Frame file' 'FAIL' $_.Exception.Message
                }
            }

            foreach ($item in @(
                @{ Name='Session storage path'; Value=[string]$config.storage.sessions_dir },
                @{ Name='Output storage path'; Value=[string]$config.storage.output_dir }
            )) {
                if ([string]::IsNullOrWhiteSpace($item.Value)) {
                    Add-Check $item.Name 'FAIL' 'Path is empty'
                } else {
                    $resolved = Resolve-ConfigPath -Value $item.Value -Base $configBase
                    $parent = Split-Path -Parent $resolved
                    if (Test-Path -LiteralPath $resolved -PathType Container) {
                        Add-Check $item.Name 'PASS' "Existing directory: $resolved"
                    } elseif (Test-Path -LiteralPath $parent -PathType Container) {
                        Add-Check $item.Name 'WARN' "Directory does not exist yet; parent is writable only after an actual write check: $resolved"
                    } else {
                        Add-Check $item.Name 'WARN' "Directory and parent do not exist yet: $resolved"
                    }
                }
            }
        } catch {
            Add-Check 'Station config' 'FAIL' $_.Exception.Message
        }
    }

    if ($CheckCamera) {
        try {
            $devices = @(Get-CimInstance Win32_PnPEntity -ErrorAction Stop |
                Where-Object {
                    $_.Name -match '(?i)camera|webcam|B525|imaging device' -and
                    $_.Name -notmatch '(?i)microphone|audio'
                } |
                Select-Object -ExpandProperty Name -Unique)
            if ($devices.Count -gt 0) {
                Add-Check 'Windows camera enumeration' 'PASS' ($devices -join '; ')
            } else {
                Add-Check 'Windows camera enumeration' 'WARN' 'No matching PnP device name was found. This check does not open the camera or prove Media Foundation capture.'
            }
        } catch {
            Add-Check 'Windows camera enumeration' 'WARN' "PnP enumeration unavailable: $($_.Exception.Message)"
        }
    } else {
        Add-Check 'Windows camera enumeration' 'WARN' 'Not checked. Use -CheckCamera for read-only PnP name enumeration; it does not open the camera.'
    }

    Write-Host ''
    Write-Host 'PhotoBooth read-only environment inspection'
    Write-Host "Repository: $repoRoot"
    Write-Host "Config:     $configFullPath"
    Write-Host 'No files were created, modified, uploaded, or deleted.'
    Write-Host ''
    $checks | Format-Table -AutoSize -Wrap
    $failed = @($checks | Where-Object Status -eq 'FAIL').Count
    $warnings = @($checks | Where-Object Status -eq 'WARN').Count
    Write-Host ("SUMMARY: failed={0} warnings={1}" -f $failed,$warnings)
    if ($failed -gt 0) { exit 1 }
} finally {
    Pop-Location
}
