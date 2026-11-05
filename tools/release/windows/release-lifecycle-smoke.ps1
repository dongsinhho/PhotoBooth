[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

$installScript = Join-Path $PSScriptRoot 'Install-PhotoBoothWindowsPackage.ps1'
$startScript = Join-Path $PSScriptRoot 'Start-PhotoBooth.ps1'
$rollbackScript = Join-Path $PSScriptRoot 'Rollback-PhotoBoothWindowsPackage.ps1'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-release-lifecycle-' + [guid]::NewGuid().ToString('N'))
$installRoot = Join-Path $tempRoot 'install'
$packageRoot = Join-Path $tempRoot 'packages'
$checks = [System.Collections.Generic.List[string]]::new()

function Assert-Equal {
    param([object]$Actual, [object]$Expected, [string]$Label)
    if ([string]$Actual -ne [string]$Expected) {
        throw "$Label mismatch. Expected '$Expected', found '$Actual'."
    }
}

function Read-ActiveVersion {
    param([string]$Root)
    $pointerPath = Join-Path $Root 'active.json'
    if (!(Test-Path -LiteralPath $pointerPath -PathType Leaf)) {
        throw 'Smoke test could not find active.json.'
    }
    return (Get-Content -LiteralPath $pointerPath -Raw | ConvertFrom-Json).active_version
}

function New-FixturePackage {
    param(
        [string]$Version,
        [byte]$Marker,
        [string]$Destination,
        [switch]$CorruptManifestHash
    )

    $staging = Join-Path $tempRoot ('fixture-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $staging -Force | Out-Null
    try {
        $bytes = [byte[]]::new(8)
        $bytes[0] = 0x4D
        $bytes[1] = 0x5A
        $bytes[2] = $Marker
        $bytes[3] = 0x11
        $bytes[4] = 0x22
        $bytes[5] = 0x33
        $bytes[6] = 0x44
        $bytes[7] = 0x55
        $exePath = Join-Path $staging 'PhotoBooth.exe'
        [IO.File]::WriteAllBytes($exePath, $bytes)
        $binaryHash = Get-PhotoBoothSha256 -Path $exePath
        $manifestHash = if ($CorruptManifestHash) { '0' * 64 } else { $binaryHash }

        $manifest = [ordered]@{
            schema_version = 1
            product = $script:PhotoBoothReleaseProduct
            channel = $script:PhotoBoothReleaseChannel
            version = $Version
            target = 'windows-amd64'
            entrypoint = 'PhotoBooth.exe'
            binary = [ordered]@{
                file = 'PhotoBooth.exe'
                size = [long](Get-Item -LiteralPath $exePath).Length
                sha256 = $manifestHash
            }
            source = [ordered]@{
                git_revision = 'fixture'
                working_tree_dirty = $true
            }
            build = [ordered]@{
                created_utc = [DateTime]::UtcNow.ToString('o')
                go_version = 'fixture'
                wails_version = '2.17.0'
            }
            distribution = [ordered]@{
                signed = $false
                intended_use = 'synthetic lifecycle test only'
            }
        }

        [IO.File]::WriteAllText(
            (Join-Path $staging 'release-manifest.json'),
            ($manifest | ConvertTo-Json -Depth 8),
            [Text.UTF8Encoding]::new($false)
        )
        [IO.File]::WriteAllText(
            (Join-Path $staging 'README.txt'),
            'Synthetic test fixture. Never launch this file.',
            [Text.UTF8Encoding]::new($false)
        )

        New-Item -ItemType Directory -Path (Split-Path -Parent $Destination) -Force | Out-Null
        Compress-Archive -Path (Join-Path $staging '*') -DestinationPath $Destination -CompressionLevel Optimal
        $zipHash = Get-PhotoBoothSha256 -Path $Destination
        $sidecar = "$zipHash  $([IO.Path]::GetFileName($Destination))" + [Environment]::NewLine
        [IO.File]::WriteAllText($Destination + '.sha256', $sidecar, [Text.UTF8Encoding]::new($false))
        return $Destination
    }
    finally {
        if (Test-Path -LiteralPath $staging) {
            Remove-Item -LiteralPath $staging -Recurse -Force
        }
    }
}

try {
    New-Item -ItemType Directory -Path $tempRoot,$packageRoot -Force | Out-Null
    Assert-PhotoBoothInstallRoot -InstallRoot $installRoot | Out-Null
    $checks.Add('temporary install root is outside operator data') | Out-Null

    $packageV1 = New-FixturePackage -Version '0.1.0-alpha.1' -Marker 1 -Destination (Join-Path $packageRoot 'v1.zip')
    & $installScript -PackagePath $packageV1 -InstallRoot $installRoot | Out-Null
    Assert-Equal (Read-ActiveVersion $installRoot) '0.1.0-alpha.1' 'initial active version'
    $checks.Add('install first immutable release and activate it') | Out-Null

    & $startScript -InstallRoot $installRoot -NoLaunch | Out-Null
    $checks.Add('verify active pointer and binary without launching PhotoBooth') | Out-Null

    $packageV2 = New-FixturePackage -Version '0.2.0-alpha.1' -Marker 2 -Destination (Join-Path $packageRoot 'v2.zip')
    & $installScript -PackagePath $packageV2 -InstallRoot $installRoot | Out-Null
    $pointer = Get-PhotoBoothActivePointer -InstallRoot $installRoot
    Assert-Equal $pointer.active_version '0.2.0-alpha.1' 'updated active version'
    Assert-Equal $pointer.previous_version '0.1.0-alpha.1' 'previous version after update'
    $checks.Add('install update and retain previous release') | Out-Null

    & $rollbackScript -InstallRoot $installRoot | Out-Null
    $pointer = Get-PhotoBoothActivePointer -InstallRoot $installRoot
    Assert-Equal $pointer.active_version '0.1.0-alpha.1' 'active version after rollback'
    Assert-Equal $pointer.previous_version '0.2.0-alpha.1' 'previous version after rollback'
    $checks.Add('rollback atomically to the previous verified release') | Out-Null

    & $startScript -InstallRoot $installRoot -NoLaunch | Out-Null
    $checks.Add('verify rolled-back executable and hash without launching') | Out-Null

    $packageBad = New-FixturePackage -Version '0.3.0-alpha.1' -Marker 3 -Destination (Join-Path $packageRoot 'bad-manifest.zip') -CorruptManifestHash
    $rejected = $false
    try {
        & $installScript -PackagePath $packageBad -InstallRoot $installRoot | Out-Null
    }
    catch {
        $rejected = $true
        Write-Host "EXPECTED_MANIFEST_REJECTION=$($_.Exception.Message)"
    }
    if (!$rejected) {
        throw 'Installer accepted a fixture whose executable hash disagrees with its manifest.'
    }
    Assert-Equal (Read-ActiveVersion $installRoot) '0.1.0-alpha.1' 'active version after rejected manifest'
    $checks.Add('reject binary/manifest hash mismatch without changing active version') | Out-Null

    $badSidecar = Join-Path $packageRoot 'bad-sidecar.zip'
    Copy-Item -LiteralPath $packageV2 -Destination $badSidecar
    [IO.File]::WriteAllText($badSidecar + '.sha256', (('0' * 64) + ' bad-sidecar.zip'), [Text.UTF8Encoding]::new($false))
    $sidecarRejected = $false
    try {
        & $installScript -PackagePath $badSidecar -InstallRoot $installRoot | Out-Null
    }
    catch {
        $sidecarRejected = $true
        Write-Host "EXPECTED_SIDECAR_REJECTION=$($_.Exception.Message)"
    }
    if (!$sidecarRejected) {
        throw 'Installer accepted a package whose SHA-256 sidecar is wrong.'
    }
    Assert-Equal (Read-ActiveVersion $installRoot) '0.1.0-alpha.1' 'active version after rejected sidecar'
    $checks.Add('reject a modified ZIP checksum before extraction or pointer update') | Out-Null

    $protectedRoot = Join-Path $env:LOCALAPPDATA 'PhotoBooth\release-test-must-not-create'
    $protectedRejected = $false
    try {
        Assert-PhotoBoothInstallRoot -InstallRoot $protectedRoot | Out-Null
    }
    catch {
        $protectedRejected = $true
        Write-Host "EXPECTED_DATA_PATH_REJECTION=$($_.Exception.Message)"
    }
    if (!$protectedRejected) {
        throw 'Installer path validation allowed an install root overlapping operator data.'
    }
    $checks.Add('reject installation path overlapping operator data') | Out-Null

    foreach ($version in @('0.1.0-alpha.1','0.2.0-alpha.1')) {
        if (!(Test-Path -LiteralPath (Join-Path (Join-Path $installRoot 'releases') $version) -PathType Container)) {
            throw "Immutable release folder was unexpectedly removed: $version"
        }
    }
    $checks.Add('preserve both release folders after rollback') | Out-Null

    Write-Host 'PHOTOBOOTH_RELEASE_LIFECYCLE_SMOKE=PASS'
    foreach ($check in $checks) { Write-Host "CHECK=$check" }
}
catch {
    Write-Host 'PHOTOBOOTH_RELEASE_LIFECYCLE_SMOKE=FAIL'
    Write-Host "FAILURE=$($_.Exception.Message)"
    throw
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Host 'SYNTHETIC_TEMP_FIXTURES_CLEANED=True'
}
