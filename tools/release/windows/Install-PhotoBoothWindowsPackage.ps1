[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PackagePath,
    [string]$InstallRoot,
    [switch]$SkipMachineProvisioning,
    [switch]$SkipAutoStart,
    [string]$AutoStartRegistryPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run',
    [switch]$AllowUnsignedPreview
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    $InstallRoot = Join-Path $env:LOCALAPPDATA 'Programs\PhotoBooth'
}
$InstallRoot = Assert-PhotoBoothInstallRoot -InstallRoot $InstallRoot
$PackagePath = (Resolve-Path -LiteralPath $PackagePath -ErrorAction Stop).Path
if (!(Test-Path -LiteralPath $PackagePath -PathType Leaf)) {
    throw "Package ZIP not found: $PackagePath"
}

Assert-PhotoBoothNotRunning -InstallRoot $InstallRoot

$sidecarPath = $PackagePath + '.sha256'
if (Test-Path -LiteralPath $sidecarPath -PathType Leaf) {
    $sidecar = (Get-Content -LiteralPath $sidecarPath -Raw -ErrorAction Stop).Trim()
    if ($sidecar -notmatch '^([0-9a-fA-F]{64})(?:\s+.+)?$') {
        throw 'Adjacent SHA-256 sidecar has an invalid format.'
    }
    $expectedZipHash = $Matches[1].ToLowerInvariant()
    $actualZipHash = Get-PhotoBoothSha256 -Path $PackagePath
    if ($actualZipHash -ne $expectedZipHash) {
        throw 'Package ZIP SHA-256 does not match its adjacent sidecar.'
    }
}

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-install-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryRoot -Force | Out-Null
$archive = $null

try {
    $archive = [IO.Compression.ZipFile]::OpenRead($PackagePath)
    $requiredNames = @('PhotoBooth.exe', 'README.txt')
    $manifestNames = @('release-manifest.json', 'release-manifest.ps1')
    $allowedNames = @($requiredNames + $manifestNames + 'manage-smb-remote-admin.ps1')
    if ($archive.Entries.Count -lt 3 -or $archive.Entries.Count -gt 4) {
        throw 'Package must contain PhotoBooth.exe, README.txt, exactly one manifest and at most one managed SMB administration helper.'
    }

    $seen = @{}
    foreach ($entry in $archive.Entries) {
        $name = [string]$entry.FullName
        if ($name.Contains('/') -or $name.Contains('\') -or $name -eq '.' -or $name -eq '..') {
            throw "Package contains a path or directory entry that is not allowed: $name"
        }
        if ($allowedNames -notcontains $name) {
            throw "Package contains an unexpected file: $name"
        }
        if ($seen.ContainsKey($name)) {
            throw "Package contains a duplicate entry: $name"
        }
        $seen[$name] = $true

        if ($name -eq 'PhotoBooth.exe' -and ($entry.Length -le 0 -or $entry.Length -gt 512MB)) {
            throw 'Packaged executable has an invalid or excessive uncompressed size.'
        }
        if ($manifestNames -contains $name -and ($entry.Length -le 0 -or $entry.Length -gt 128KB)) {
            throw 'Release manifest has an invalid or excessive size.'
        }
        if ($name -eq 'README.txt' -and ($entry.Length -le 0 -or $entry.Length -gt 64KB)) {
            throw 'Package README has an invalid or excessive size.'
        }
        if ($name -eq 'manage-smb-remote-admin.ps1' -and ($entry.Length -le 0 -or $entry.Length -gt 512KB)) {
            throw 'SMB administration helper has an invalid or excessive size.'
        }
    }
    foreach ($name in $requiredNames) {
        if (!$seen.ContainsKey($name)) {
            throw "Package is missing required entry: $name"
        }
    }
    $presentManifests = @($manifestNames | Where-Object { $seen.ContainsKey($_) })
    if ($presentManifests.Count -ne 1) {
        throw 'Package must contain exactly one release manifest, either JSON Preview or signed Authenticode envelope.'
    }

    foreach ($entry in $archive.Entries) {
        $destination = Join-Path $temporaryRoot $entry.Name
        $sourceStream = $entry.Open()
        $destinationStream = [IO.File]::Open($destination, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        try {
            $sourceStream.CopyTo($destinationStream)
            $destinationStream.Flush()
        }
        finally {
            $destinationStream.Dispose()
            $sourceStream.Dispose()
        }
    }
}
catch {
    if (Test-Path -LiteralPath $temporaryRoot) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    throw
}
finally {
    if ($archive) {
        $archive.Dispose()
    }
}

try {
    $releaseCheck = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $temporaryRoot -AllowUnsignedPreview:$AllowUnsignedPreview
    $manifest = $releaseCheck.Manifest
    $version = [string]$manifest.version
    $sha256 = [string]$manifest.binary.sha256
    $releasesRoot = Join-Path $InstallRoot 'releases'
    $finalRelease = Join-Path $releasesRoot $version

    New-Item -ItemType Directory -Path $releasesRoot -Force | Out-Null
    if (Test-Path -LiteralPath $finalRelease) {
        $existingRelease = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $finalRelease -ExpectedVersion $version -ExpectedSha256 $sha256 -AllowUnsignedPreview:$AllowUnsignedPreview
        $existingManifest = $existingRelease.Manifest
        if ([bool]$existingManifest.distribution.signed -ne [bool]$manifest.distribution.signed) {
            throw "Release version '$version' already exists with a different signature policy; refusing to treat a signed/unsigned collision as an identical release."
        }
        if ([string]$existingManifest.distribution.signer_thumbprint -ne [string]$manifest.distribution.signer_thumbprint) {
            throw "Release version '$version' already exists with a different publisher identity."
        }
        $existingHelperHash = if ($existingManifest.smb_admin_helper) { [string]$existingManifest.smb_admin_helper.sha256 } else { '' }
        $incomingHelperHash = if ($manifest.smb_admin_helper) { [string]$manifest.smb_admin_helper.sha256 } else { '' }
        if ($existingHelperHash -ne $incomingHelperHash) {
            throw "Release version '$version' already exists with a different SMB helper artifact hash."
        }
        Write-Host "RELEASE_ALREADY_INSTALLED=$version"
    }
    else {
        $stagingRelease = Join-Path $releasesRoot ('.staging-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $stagingRelease -Force | Out-Null
        try {
            Copy-Item -LiteralPath (Join-Path $temporaryRoot 'PhotoBooth.exe') -Destination (Join-Path $stagingRelease 'PhotoBooth.exe')
            foreach ($manifestName in $presentManifests) {
                Copy-Item -LiteralPath (Join-Path $temporaryRoot $manifestName) -Destination (Join-Path $stagingRelease $manifestName)
            }
            Copy-Item -LiteralPath (Join-Path $temporaryRoot 'README.txt') -Destination (Join-Path $stagingRelease 'README.txt')
            $temporaryHelper = Join-Path $temporaryRoot 'manage-smb-remote-admin.ps1'
            if (Test-Path -LiteralPath $temporaryHelper -PathType Leaf) {
                Copy-Item -LiteralPath $temporaryHelper -Destination (Join-Path $stagingRelease 'manage-smb-remote-admin.ps1')
            }
            [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $stagingRelease -ExpectedVersion $version -ExpectedSha256 $sha256 -AllowUnsignedPreview:$AllowUnsignedPreview)
            [IO.Directory]::Move($stagingRelease, $finalRelease)
        }
        catch {
            if (Test-Path -LiteralPath $stagingRelease) {
                Remove-Item -LiteralPath $stagingRelease -Recurse -Force -ErrorAction SilentlyContinue
            }
            throw
        }
        Write-Host "RELEASE_INSTALLED=$version"
    }

    $oldPointer = Get-PhotoBoothActivePointer -InstallRoot $InstallRoot

    # A first install establishes the accepted machine-wide default-off baseline.
    # Upgrades deliberately preserve the operator's existing SMB maintenance choice.
    if ($null -eq $oldPointer -and !$SkipMachineProvisioning) {
        $helperPath = Join-Path $finalRelease 'manage-smb-remote-admin.ps1'
        if (!(Test-Path -LiteralPath $helperPath -PathType Leaf)) {
            throw 'A first installation requires the packaged SMB administration helper to establish the default-off Windows baseline. Legacy packages may be upgraded, but cannot be used for a fresh provisioned install.'
        }
        $powerShellExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        $provisionOutput = & $powerShellExe -NoProfile -ExecutionPolicy Bypass -File $helperPath -Action Disable -WaitForElevation 2>&1
        $provisionExitCode = $LASTEXITCODE
        if ($provisionExitCode -ne 0) {
            $details = ($provisionOutput | ForEach-Object { [string]$_ }) -join [Environment]::NewLine
            throw "PhotoBooth could not establish the SMB default-off baseline. Installation was not activated. $details"
        }
        Write-Host 'SMB_DEFAULT_OFF_PROVISIONED=True'
    } elseif ($null -eq $oldPointer -and $SkipMachineProvisioning) {
        Write-Warning 'SKIP_MACHINE_PROVISIONING was explicitly selected; this install does not establish the SMB default-off baseline.'
    }

    if ($oldPointer -and $oldPointer.active_version -eq $version) {
        $previousVersion = $oldPointer.previous_version
        $previousSha256 = $oldPointer.previous_sha256
    }
    elseif ($oldPointer) {
        $previousVersion = $oldPointer.active_version
        $previousSha256 = $oldPointer.active_sha256
    }
    else {
        $previousVersion = $null
        $previousSha256 = $null
    }

    # Recheck immediately before switching the active version. Existing processes are never killed.
    Assert-PhotoBoothNotRunning -InstallRoot $InstallRoot
    $pointerArgs = @{
        InstallRoot = $InstallRoot
        ActiveVersion = $version
        ActiveSha256 = $sha256
        PreviousVersion = $previousVersion
        PreviousSha256 = $previousSha256
    }
    Write-PhotoBoothActivePointer @pointerArgs

    if (!$SkipAutoStart) {
        $activeExecutable = Join-Path $finalRelease 'PhotoBooth.exe'
        $autoStartCommand = Set-PhotoBoothCurrentUserAutoStart -InstallRoot $InstallRoot -ExecutablePath $activeExecutable -RegistryPath $AutoStartRegistryPath
        Write-Host 'CURRENT_USER_AUTO_START=ENABLED'
        Write-Host "AUTO_START_COMMAND=$autoStartCommand"
    } else {
        Write-Host 'CURRENT_USER_AUTO_START=SKIPPED (explicit test/maintenance option)'
    }

    Write-Host "ACTIVE_VERSION=$version"
    Write-Host "PREVIOUS_VERSION=$previousVersion"
    Write-Host "INSTALL_ROOT=$InstallRoot"
    Write-Output $finalRelease
}
finally {
    if (Test-Path -LiteralPath $temporaryRoot) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
