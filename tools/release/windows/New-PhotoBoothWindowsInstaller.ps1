[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$PackagePath,
    [string]$OutputDirectory,
    [string]$MakensisPath,
    [switch]$AllowUnsignedPreview,
    [string]$SigningCertificateThumbprint,
    [string]$TimestampServer
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
Assert-PhotoBoothVersion $Version

if ($AllowUnsignedPreview -and (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint))) {
    throw 'Choose either -AllowUnsignedPreview or -SigningCertificateThumbprint, not both.'
}
if (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint) -and [string]::IsNullOrWhiteSpace($TimestampServer)) {
    throw 'A timestamp server URL is required when signing the installer.'
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path $repoRoot 'artifacts\windows'
}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null

if ([string]::IsNullOrWhiteSpace($PackagePath)) {
    $channel = if ($AllowUnsignedPreview) { 'internal-alpha' } else { 'signed-candidate' }
    $packageName = "PhotoBooth-$channel-$Version-windows-amd64.zip"
    $PackagePath = Join-Path $OutputDirectory $packageName
    if (!(Test-Path -LiteralPath $PackagePath -PathType Leaf)) {
        $packageBuilder = Join-Path $PSScriptRoot 'New-PhotoBoothWindowsPackage.ps1'
        $builderArgs = @{ Version = $Version; OutputDirectory = $OutputDirectory }
        if ($AllowUnsignedPreview) {
            $builderArgs.AllowUnsignedPreview = $true
        } elseif (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint)) {
            $builderArgs.SigningCertificateThumbprint = $SigningCertificateThumbprint
            $builderArgs.TimestampServer = $TimestampServer
        } else {
            throw 'A signed package requires -SigningCertificateThumbprint and -TimestampServer. Use -AllowUnsignedPreview only for an internal engineering build.'
        }
        & $packageBuilder @builderArgs
        if ($LASTEXITCODE -ne 0) {
            throw "Windows package creation failed with exit code $LASTEXITCODE."
        }
    }
}
$PackagePath = (Resolve-Path -LiteralPath $PackagePath -ErrorAction Stop).Path
if (!(Test-Path -LiteralPath $PackagePath -PathType Leaf)) {
    throw "Windows release package not found: $PackagePath"
}
$checksumPath = $PackagePath + '.sha256'
if (!(Test-Path -LiteralPath $checksumPath -PathType Leaf)) {
    throw "Adjacent package checksum is missing: $checksumPath"
}
$checksumText = (Get-Content -LiteralPath $checksumPath -Raw -ErrorAction Stop).Trim()
if ($checksumText -notmatch '^([0-9a-fA-F]{64})(?:\s+.+)?$') {
    throw 'Adjacent package checksum has an invalid format.'
}
$expectedZipHash = $Matches[1].ToLowerInvariant()
$actualZipHash = Get-PhotoBoothSha256 -Path $PackagePath
if ($actualZipHash -ne $expectedZipHash) {
    throw 'Package ZIP SHA-256 does not match its adjacent checksum.'
}

# Validate the exact release payload before embedding it in the installer.
$validationRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-installer-validation-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $validationRoot -Force | Out-Null
$archive = $null
try {
    $archive = [IO.Compression.ZipFile]::OpenRead($PackagePath)
    $allowed = @('PhotoBooth.exe', 'README.txt', 'release-manifest.json', 'release-manifest.ps1', 'manage-smb-remote-admin.ps1')
    if ($archive.Entries.Count -lt 3 -or $archive.Entries.Count -gt 4) {
        throw 'Release package has an unexpected number of entries.'
    }
    $seen = @{}
    foreach ($entry in $archive.Entries) {
        $name = [string]$entry.FullName
        if ($name.Contains('/') -or $name.Contains('\') -or $name -in @('.', '..') -or $allowed -notcontains $name -or $seen.ContainsKey($name)) {
            throw "Release package contains an unsafe, duplicate or unexpected entry: $name"
        }
        $seen[$name] = $true
        $destination = Join-Path $validationRoot $name
        $sourceStream = $entry.Open()
        $destinationStream = [IO.File]::Open($destination, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        try {
            $sourceStream.CopyTo($destinationStream)
        } finally {
            $destinationStream.Dispose()
            $sourceStream.Dispose()
        }
    }
    [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $validationRoot -ExpectedVersion $Version -AllowUnsignedPreview:$AllowUnsignedPreview)
}
finally {
    if ($archive) { $archive.Dispose() }
    if (Test-Path -LiteralPath $validationRoot) {
        Remove-Item -LiteralPath $validationRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

if ([string]::IsNullOrWhiteSpace($MakensisPath)) {
    $makensisCommand = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($makensisCommand) {
        $MakensisPath = $makensisCommand.Source
    } else {
        throw 'NSIS 3.x compiler was not found. Install NSIS and provide -MakensisPath.'
    }
}
$MakensisPath = (Resolve-Path -LiteralPath $MakensisPath -ErrorAction Stop).Path
$installerName = "PhotoBooth-$Version-Setup.exe"
$installerPath = Join-Path $OutputDirectory $installerName
if (Test-Path -LiteralPath $installerPath) {
    throw "Refusing to overwrite an existing installer: $installerPath"
}

$nsisSource = Join-Path $PSScriptRoot 'PhotoBoothInstaller.nsi'
$defines = @(
    "/DPHOTOBOOTH_PACKAGE_FILE=$PackagePath",
    "/DPHOTOBOOTH_CHECKSUM_FILE=$checksumPath",
    "/DPHOTOBOOTH_INSTALL_SCRIPT=$(Join-Path $PSScriptRoot 'Install-PhotoBoothWindowsPackage.ps1')",
    "/DPHOTOBOOTH_UNINSTALL_SCRIPT=$(Join-Path $PSScriptRoot 'Uninstall-PhotoBoothWindowsPackage.ps1')",
    "/DPHOTOBOOTH_RELEASE_COMMON=$(Join-Path $PSScriptRoot 'Release.Common.ps1')",
    "/DPHOTOBOOTH_VERSION=$Version",
    "/DPHOTOBOOTH_OUTPUT_FILE=$installerPath"
)
if ($AllowUnsignedPreview) {
    $defines += '/DPHOTOBOOTH_ALLOW_UNSIGNED_PREVIEW'
}

Push-Location $PSScriptRoot
try {
    & $MakensisPath '/V3' @defines $nsisSource
    if ($LASTEXITCODE -ne 0) {
        throw "NSIS compilation failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}

if (!(Test-Path -LiteralPath $installerPath -PathType Leaf)) {
    throw 'NSIS returned success but did not create the expected installer.'
}
if (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint)) {
    $certificate = Get-PhotoBoothSigningCertificate -Thumbprint $SigningCertificateThumbprint
    [void](Set-PhotoBoothAuthenticodeSignature -Path $installerPath -Certificate $certificate -TimestampServer $TimestampServer)
}

$installerHash = Get-PhotoBoothSha256 -Path $installerPath
Write-Host "INSTALLER_VERSION=$Version"
Write-Host "INSTALLER_SHA256=$installerHash"
Write-Host "INSTALLER_PATH=$installerPath"
Write-Output $installerPath
