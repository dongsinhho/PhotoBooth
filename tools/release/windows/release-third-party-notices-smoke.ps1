 [CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$packageBuilder = Join-Path $PSScriptRoot 'New-PhotoBoothWindowsPackage.ps1'
$installScript = Join-Path $PSScriptRoot 'Install-PhotoBoothWindowsPackage.ps1'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-third-party-notices-smoke-' + [guid]::NewGuid().ToString('N'))
$checks = [System.Collections.Generic.List[string]]::new()
$version = '0.0.0-notices-smoke.' + [guid]::NewGuid().ToString('N')
$installRoot = Join-Path $tempRoot 'install'
$outputDirectory = Join-Path $tempRoot 'packages'

try {
    if (Get-Process -Name PhotoBooth -ErrorAction SilentlyContinue) {
        throw 'PhotoBooth is running; close it before the package notice smoke test.'
    }

    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
    & $packageBuilder -Version $version -SkipBuild -AllowUnsignedPreview -OutputDirectory $outputDirectory | Out-Null
    if (!$?) {
        throw 'The Windows release package builder failed.'
    }
    $zipName = "PhotoBooth-internal-alpha-$version-windows-amd64.zip"
    $zipPath = Join-Path $outputDirectory $zipName
    if (!(Test-Path -LiteralPath $zipPath -PathType Leaf)) {
        throw "The generated package ZIP is missing: $zipPath"
    }
    $checks.Add('build package and collect a manifest-pinned third-party notice bundle') | Out-Null

    Add-Type -AssemblyName System.IO.Compression
    $archive = [IO.Compression.ZipFile]::OpenRead($zipPath)
    try {
        $zipNames = @($archive.Entries | ForEach-Object { [string]$_.FullName })
        foreach ($name in @('LICENSE.txt', 'THIRD_PARTY_NOTICES.md', 'THIRD_PARTY_LICENSES.txt')) {
            if ($zipNames -notcontains $name) {
                throw "Release ZIP is missing third-party notice file: $name"
            }
        }
        if ($zipNames.Count -ne 7) {
            throw "Expected seven flat release ZIP entries (including three license notices), found $($zipNames.Count)."
        }
    }
    finally {
        $archive.Dispose()
    }
    $checks.Add('include root MIT license and all third-party notices in release ZIP') | Out-Null

    & $installScript -PackagePath $zipPath -InstallRoot $installRoot -SkipMachineProvisioning -SkipAutoStart -AllowUnsignedPreview | Out-Null
    if (!$?) {
        throw 'The isolated Windows package install failed.'
    }
    $releaseDirectory = Join-Path (Join-Path $installRoot 'releases') $version
    foreach ($name in @('LICENSE.txt', 'THIRD_PARTY_NOTICES.md', 'THIRD_PARTY_LICENSES.txt')) {
        if (!(Test-Path -LiteralPath (Join-Path $releaseDirectory $name) -PathType Leaf)) {
            throw "Installed immutable release is missing license notice: $name"
        }
    }
    $checks.Add('install and retain all three notice files in the immutable release directory') | Out-Null

    . (Join-Path $PSScriptRoot 'Release.Common.ps1')
    $validated = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $releaseDirectory -AllowUnsignedPreview
    if (@($validated.Manifest.third_party_notices).Count -ne 3) {
        throw 'Installed release manifest does not declare exactly three third-party notice files.'
    }
    $checks.Add('verify installed notice metadata, file sizes and SHA-256 digests') | Out-Null

    $tamperedPath = Join-Path $releaseDirectory 'THIRD_PARTY_LICENSES.txt'
    $originalBytes = [IO.File]::ReadAllBytes($tamperedPath)
    try {
        [IO.File]::AppendAllText($tamperedPath, [Environment]::NewLine + '# tampered fixture')
        $tamperRejected = $false
        try {
            [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $releaseDirectory -AllowUnsignedPreview)
        }
        catch {
            $tamperRejected = $_.Exception.Message -match '(size does not match its manifest|SHA-256 does not match its manifest)'
            if (!$tamperRejected) {
                throw "Tampered notice was rejected for an unexpected reason: $($_.Exception.Message)"
            }
        }
        if (!$tamperRejected) {
            throw 'The release validator accepted a modified third-party notice file.'
        }
    }
    finally {
        [IO.File]::WriteAllBytes($tamperedPath, $originalBytes)
    }
    $checks.Add('reject a tampered third-party notice file using the manifest digest') | Out-Null

    Write-Output 'PHOTOBOOTH_THIRD_PARTY_NOTICES_SMOKE=PASS'
    foreach ($check in $checks) { Write-Output "CHECK=$check" }
}
catch {
    Write-Output 'PHOTOBOOTH_THIRD_PARTY_NOTICES_SMOKE=FAIL'
    Write-Output "FAILURE=$($_.Exception.Message)"
    throw
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -Confirm:$false -ErrorAction SilentlyContinue
    }
    Write-Output 'THIRD_PARTY_NOTICES_SMOKE_TEMP_FILES_CLEANED=True'
}
