[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$SkipBuild,
    [string]$OutputDirectory,
    [switch]$AllowUnsignedPreview,
    [string]$SigningCertificateThumbprint,
    [string]$TimestampServer
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')
Assert-PhotoBoothVersion $Version
if ([string]::IsNullOrWhiteSpace($SigningCertificateThumbprint) -and !$AllowUnsignedPreview) {
    throw 'Package creation requires -SigningCertificateThumbprint for a publisher-signed production release or explicit -AllowUnsignedPreview for an unsigned internal Preview.'
}
if (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint) -and $AllowUnsignedPreview) {
    throw 'Choose either a signed production release or an unsigned Preview package, not both.'
}
$signingCertificate = $null
if (![string]::IsNullOrWhiteSpace($SigningCertificateThumbprint)) {
    if ([string]::IsNullOrWhiteSpace($TimestampServer)) {
        throw 'A timestamp server URL is required for production Authenticode signing.'
    }
    $signingCertificate = Get-PhotoBoothSigningCertificate -Thumbprint $SigningCertificateThumbprint
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$uiDirectory = Join-Path $repoRoot 'tools\ui-spike\wails-v2'
$executable = Join-Path $uiDirectory 'build\bin\PhotoBooth.exe'
$smbAdminHelper = Join-Path $repoRoot 'tools\self-service\manage-smb-remote-admin.ps1'
$launcher = Join-Path $repoRoot 'run-photobooth.ps1'

if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path $repoRoot 'artifacts\windows'
}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)

if (!$SkipBuild) {
    if (!(Test-Path -LiteralPath $launcher -PathType Leaf)) {
        throw "Root build launcher is missing: $launcher"
    }
    Write-Host 'Building the isolated Wails Windows Preview through run-photobooth.ps1...'
    & $launcher -NoLaunch
    if ($LASTEXITCODE -ne 0) {
        throw "The PhotoBooth Windows Preview build failed with exit code $LASTEXITCODE."
    }
}
else {
    Write-Host 'Using the existing PhotoBooth.exe because -SkipBuild was supplied.'
}

if (!(Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "Build output is missing: $executable"
}
if (!(Test-Path -LiteralPath $smbAdminHelper -PathType Leaf)) {
    throw "Windows SMB administration helper is missing: $smbAdminHelper"
}
Assert-PhotoBoothPE -Path $executable
Assert-PhotoBoothNotRunning -InstallRoot (Split-Path -Parent $executable)

$packageExecutable = $executable
$packageHelper = $smbAdminHelper
$signingStageRoot = $null
if ($signingCertificate) {
    $signingStageRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-signing-stage-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $signingStageRoot -Force | Out-Null
    $packageExecutable = Join-Path $signingStageRoot 'PhotoBooth.exe'
    $packageHelper = Join-Path $signingStageRoot 'manage-smb-remote-admin.ps1'
    Copy-Item -LiteralPath $executable -Destination $packageExecutable
    Copy-Item -LiteralPath $smbAdminHelper -Destination $packageHelper
    try {
        Write-Host "SIGNING_STAGED_EXECUTABLE=$packageExecutable"
        [void](Set-PhotoBoothAuthenticodeSignature -Path $packageExecutable -Certificate $signingCertificate -TimestampServer $TimestampServer)
        Write-Host "SIGNING_STAGED_HELPER=$packageHelper"
        [void](Set-PhotoBoothAuthenticodeSignature -Path $packageHelper -Certificate $signingCertificate -TimestampServer $TimestampServer)
    }
    catch {
        if (Test-Path -LiteralPath $signingStageRoot) {
            Remove-Item -LiteralPath $signingStageRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
        throw
    }
}
$binaryInfo = Get-Item -LiteralPath $packageExecutable
$binaryHash = Get-PhotoBoothSha256 -Path $packageExecutable
$revision = 'unknown'
$workingTreeDirty = $null
try {
    $revisionOutput = & git -C $repoRoot rev-parse HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $revisionOutput) {
        $revision = ([string]$revisionOutput).Trim()
    }
    $statusOutput = @(& git -C $repoRoot status --porcelain 2>$null)
    if ($LASTEXITCODE -eq 0) {
        $workingTreeDirty = ($statusOutput.Count -gt 0)
    }
}
catch {
    Write-Warning "Could not read Git source metadata: $($_.Exception.Message)"
}

$goVersion = 'unknown'
try {
    Push-Location $uiDirectory
    $goOutput = & go version 2>&1
    if ($LASTEXITCODE -eq 0) {
        $goVersion = ([string]$goOutput).Trim()
    }
}
finally {
    Pop-Location
}

$releaseChannel = if ($signingCertificate) { 'signed-candidate' } else { 'internal-alpha' }
$releaseName = "PhotoBooth-$releaseChannel-$Version-windows-amd64"
$zipPath = Join-Path $OutputDirectory ($releaseName + '.zip')
$hashPath = $zipPath + '.sha256'
if (Test-Path -LiteralPath $zipPath) {
    throw "Refusing to overwrite an existing package: $zipPath"
}
if (Test-Path -LiteralPath $hashPath) {
    throw "Refusing to overwrite an existing checksum file: $hashPath"
}

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$stagingRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-release-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $stagingRoot -Force | Out-Null

try {
    $licenseCollector = Join-Path $PSScriptRoot 'Collect-PhotoBoothThirdPartyNotices.ps1'
    if (!(Test-Path -LiteralPath $licenseCollector -PathType Leaf)) {
        throw "Third-party license collector is missing: $licenseCollector"
    }
    Copy-Item -LiteralPath (Join-Path $repoRoot 'LICENSE') -Destination (Join-Path $stagingRoot 'LICENSE.txt')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'THIRD_PARTY_NOTICES.md') -Destination (Join-Path $stagingRoot 'THIRD_PARTY_NOTICES.md')
    $thirdPartyLicensePath = Join-Path $stagingRoot 'THIRD_PARTY_LICENSES.txt'
    & $licenseCollector -OutputPath $thirdPartyLicensePath
    if ($LASTEXITCODE -ne 0 -or !(Test-Path -LiteralPath $thirdPartyLicensePath -PathType Leaf)) {
        throw 'Third-party license collection failed; refusing to package an incomplete notice bundle.'
    }
    $thirdPartyNoticeFiles = @('LICENSE.txt', 'THIRD_PARTY_NOTICES.md', 'THIRD_PARTY_LICENSES.txt')
    $thirdPartyNotices = @(
        foreach ($noticeFile in $thirdPartyNoticeFiles) {
            $noticePath = Join-Path $stagingRoot $noticeFile
            [ordered]@{
                file = $noticeFile
                size = [long](Get-Item -LiteralPath $noticePath).Length
                sha256 = Get-PhotoBoothSha256 -Path $noticePath
            }
        }
    )

    $manifest = [ordered]@{
        schema_version = 1
        product = $script:PhotoBoothReleaseProduct
        channel = $releaseChannel
        version = $Version
        target = 'windows-amd64'
        entrypoint = 'PhotoBooth.exe'
        binary = [ordered]@{
            file = 'PhotoBooth.exe'
            size = [long]$binaryInfo.Length
            sha256 = $binaryHash
        }
        smb_admin_helper = [ordered]@{
            file = 'manage-smb-remote-admin.ps1'
            size = [long](Get-Item -LiteralPath $packageHelper).Length
            sha256 = Get-PhotoBoothSha256 -Path $packageHelper
        }
        third_party_notices = $thirdPartyNotices
        source = [ordered]@{
            git_revision = $revision
            working_tree_dirty = $workingTreeDirty
        }
        build = [ordered]@{
            created_utc = [DateTime]::UtcNow.ToString('o')
            go_version = $goVersion
            wails_version = '2.17.0'
        }
        distribution = [ordered]@{
            signed = [bool]$signingCertificate
            signer_thumbprint = if ($signingCertificate) { $signingCertificate.Thumbprint.ToUpperInvariant() } else { $null }
            signature_type = if ($signingCertificate) { 'Authenticode-SHA256' } else { $null }
            intended_use = if ($signingCertificate) { 'publisher-signed Windows Preview release candidate; production UI approval remains separate' } else { 'internal Windows Preview evaluation only' }
        }
    }

    $manifestJson = $manifest | ConvertTo-Json -Depth 8
    $unsignedPreviewSwitch = if ($signingCertificate) { '' } else { ' -AllowUnsignedPreview' }
    if ($signingCertificate) {
        $manifestBase64 = [Convert]::ToBase64String([Text.UTF8Encoding]::new($false).GetBytes($manifestJson))
        $envelopePath = Join-Path $stagingRoot 'release-manifest.ps1'
        $envelope = '$PhotoBoothReleaseManifestBase64 = ''' + $manifestBase64 + '''' + [Environment]::NewLine
        [IO.File]::WriteAllText($envelopePath, $envelope, [Text.UTF8Encoding]::new($false))
        [void](Set-PhotoBoothAuthenticodeSignature -Path $envelopePath -Certificate $signingCertificate -TimestampServer $TimestampServer)
    }
    else {
        [IO.File]::WriteAllText(
            (Join-Path $stagingRoot 'release-manifest.json'),
            $manifestJson,
            [Text.UTF8Encoding]::new($false)
        )
    }

    $packageTitle = if ($signingCertificate) { 'PhotoBooth Windows Preview - signed release candidate' } else { 'PhotoBooth Windows Preview - unsigned internal alpha package' }
    $readme = @"
$packageTitle

Version: $Version
Target: Windows 10/11 x64
Executable: PhotoBooth.exe

$(
if ($signingCertificate) {
    'This package has Authenticode signatures for the executable, SMB helper and release-manifest envelope. It is a signed Windows Preview candidate, not proof of production UI approval or universal hardware compatibility.'
} else {
    'This is an unsigned engineering package for internal evaluation only. It is not a production release, signed installer, automatic updater, or compatibility promise for every Windows camera or computer.'
}
)

Requirements:
- Windows 10/11 x64
- Microsoft Edge WebView2 Runtime
- PowerShell 5.1 and Windows NetSecurity module
- A camera/driver combination explicitly validated by the project

First installation:
- Setup asks for explicit confirmation before applying the machine-wide SMB default-off baseline, then requests UAC approval.
- LanmanServer is stopped/disabled and inbound SMB is blocked. Existing shares, including custom shares such as `Users`, become unavailable to network clients while the service is stopped; share definitions, paths, ACLs and user data are not deleted.
- Active SMB sessions and running dependent services remain hard blockers. Custom shares are only allowed to become unserved after the explicit first-install confirmation.
- An SMB provisioning diagnostic log is retained in `%TEMP%` if provisioning fails.
- Upgrades preserve the machine's existing SMB administration choice.

Install:
1. Extract this ZIP to a temporary folder.
2. From the repository checkout, run:
   powershell -ExecutionPolicy Bypass -File tools\release\windows\Install-PhotoBoothWindowsPackage.ps1 -PackagePath <this ZIP>$unsignedPreviewSwitch
3. Launch through:
   powershell -ExecutionPolicy Bypass -File tools\release\windows\Start-PhotoBooth.ps1$unsignedPreviewSwitch

Update and rollback:
- Close PhotoBooth before installing or rolling back.
- Each version is stored in its own immutable releases\<version> folder.
- Installing a new version changes active.json atomically and retains the prior
  version for rollback. Rollback never deletes either release.
- The installation directory is separate from operator data.

Operator data:
- Settings, frames, sessions and photos are stored under:
  %LOCALAPPDATA%\PhotoBooth
- This package workflow does not migrate, overwrite, or delete that directory.
- Back up operator data before testing any preview build.

Integrity:
- Verify the adjacent .sha256 file when transferring the ZIP.
- The embedded release manifest verifies the executable's size and SHA-256.
- SHA-256 detects accidental or untrusted modification only when compared to
  a trusted checksum source; it is not a digital signature or publisher proof.

Licensing:
- PhotoBooth-owned code is licensed under MIT; see LICENSE.txt.
- Third-party dependency licenses and notices are included in THIRD_PARTY_NOTICES.md
  and THIRD_PARTY_LICENSES.txt. Preserve these files when redistributing the package.
- This is a Windows Preview engineering package, not a production compatibility or
  support promise. The MIT license terms apply; optional paid support does not restrict them.
"@
    [IO.File]::WriteAllText(
        (Join-Path $stagingRoot 'README.txt'),
        $readme,
        [Text.UTF8Encoding]::new($false)
    )
    Copy-Item -LiteralPath $packageExecutable -Destination (Join-Path $stagingRoot 'PhotoBooth.exe')
    Copy-Item -LiteralPath $packageHelper -Destination (Join-Path $stagingRoot 'manage-smb-remote-admin.ps1')

    Compress-Archive -Path (Join-Path $stagingRoot '*') -DestinationPath $zipPath -CompressionLevel Optimal
    $zipHash = Get-PhotoBoothSha256 -Path $zipPath
    $checksumLine = "$zipHash  $([IO.Path]::GetFileName($zipPath))" + [Environment]::NewLine
    [IO.File]::WriteAllText($hashPath, $checksumLine, [Text.UTF8Encoding]::new($false))

    Write-Host "PACKAGE_VERSION=$Version"
    Write-Host "PACKAGE_CHANNEL=$releaseChannel"
    Write-Host "PACKAGE_EXE_SHA256=$binaryHash"
    if ($signingCertificate) {
        Write-Host "PACKAGE_PUBLISHER_THUMBPRINT=$($signingCertificate.Thumbprint)"
    }
    Write-Host "PACKAGE_ZIP_SHA256=$zipHash"
    Write-Host "PACKAGE_PATH=$zipPath"
    Write-Host "CHECKSUM_PATH=$hashPath"
    Write-Output $zipPath
}
catch {
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $hashPath -Force -ErrorAction SilentlyContinue
    throw
}
finally {
    if (Test-Path -LiteralPath $stagingRoot) {
        Remove-Item -LiteralPath $stagingRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    if ($signingStageRoot -and (Test-Path -LiteralPath $signingStageRoot)) {
        Remove-Item -LiteralPath $signingStageRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
