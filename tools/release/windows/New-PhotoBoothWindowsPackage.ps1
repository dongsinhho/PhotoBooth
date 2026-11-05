[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$SkipBuild,
    [string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')
Assert-PhotoBoothVersion $Version

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$uiDirectory = Join-Path $repoRoot 'tools\ui-spike\wails-v2'
$executable = Join-Path $uiDirectory 'build\bin\PhotoBooth.exe'
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
Assert-PhotoBoothPE -Path $executable
Assert-PhotoBoothNotRunning -InstallRoot (Split-Path -Parent $executable)

$binaryInfo = Get-Item -LiteralPath $executable
$binaryHash = Get-PhotoBoothSha256 -Path $executable
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

$releaseName = "PhotoBooth-WindowsPreview-$Version-windows-amd64"
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
    $manifest = [ordered]@{
        schema_version = 1
        product = $script:PhotoBoothReleaseProduct
        channel = $script:PhotoBoothReleaseChannel
        version = $Version
        target = 'windows-amd64'
        entrypoint = 'PhotoBooth.exe'
        binary = [ordered]@{
            file = 'PhotoBooth.exe'
            size = [long]$binaryInfo.Length
            sha256 = $binaryHash
        }
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
            signed = $false
            intended_use = 'internal Windows Preview evaluation only'
        }
    }

    $manifestJson = $manifest | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText(
        (Join-Path $stagingRoot 'release-manifest.json'),
        $manifestJson,
        [Text.UTF8Encoding]::new($false)
    )

    $readme = @"
PhotoBooth Windows Preview - internal alpha package

Version: $Version
Target: Windows 10/11 x64
Executable: PhotoBooth.exe

This is an unsigned engineering package for internal evaluation. It is not a
production release, signed installer, automatic updater, or compatibility
promise for every Windows camera or computer.

Requirements:
- Windows 10/11 x64
- Microsoft Edge WebView2 Runtime
- A camera/driver combination explicitly validated by the project

Install:
1. Extract this ZIP to a temporary folder.
2. From the repository checkout, run:
   powershell -ExecutionPolicy Bypass -File tools\release\windows\Install-PhotoBoothWindowsPackage.ps1 -PackagePath <this ZIP>
3. Launch through:
   powershell -ExecutionPolicy Bypass -File tools\release\windows\Start-PhotoBooth.ps1

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
- This internal alpha package does not define or replace end-user license terms.
- Do not redistribute publicly. Product/legal terms and third-party dependency
  redistribution review remain open.
"@
    [IO.File]::WriteAllText(
        (Join-Path $stagingRoot 'README.txt'),
        $readme,
        [Text.UTF8Encoding]::new($false)
    )
    Copy-Item -LiteralPath $executable -Destination (Join-Path $stagingRoot 'PhotoBooth.exe')

    Compress-Archive -Path (Join-Path $stagingRoot '*') -DestinationPath $zipPath -CompressionLevel Optimal
    $zipHash = Get-PhotoBoothSha256 -Path $zipPath
    $checksumLine = "$zipHash  $([IO.Path]::GetFileName($zipPath))" + [Environment]::NewLine
    [IO.File]::WriteAllText($hashPath, $checksumLine, [Text.UTF8Encoding]::new($false))

    Write-Host "PACKAGE_VERSION=$Version"
    Write-Host "PACKAGE_EXE_SHA256=$binaryHash"
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
}
