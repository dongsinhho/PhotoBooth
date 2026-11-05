Set-StrictMode -Version Latest

$script:PhotoBoothReleaseProduct = 'PhotoBooth Windows Preview'
$script:PhotoBoothReleaseChannel = 'internal-alpha'
$script:PhotoBoothReleaseSchema = 1

function Assert-PhotoBoothVersion {
    param([Parameter(Mandatory = $true)][string]$Version)

    if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$') {
        throw "Invalid release version '$Version'. Use MAJOR.MINOR.PATCH with an optional prerelease suffix."
    }
}

function Get-PhotoBoothFullPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    return [IO.Path]::GetFullPath($Path).TrimEnd([IO.Path]::DirectorySeparatorChar)
}

function Test-PhotoBoothPathWithin {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Parent
    )

    $fullPath = Get-PhotoBoothFullPath $Path
    $fullParent = Get-PhotoBoothFullPath $Parent
    if ($fullPath.Equals($fullParent, [StringComparison]::OrdinalIgnoreCase)) {
        return $true
    }

    $prefix = $fullParent + [IO.Path]::DirectorySeparatorChar
    return $fullPath.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)
}

function Assert-PhotoBoothInstallRoot {
    param([Parameter(Mandatory = $true)][string]$InstallRoot)

    $fullRoot = Get-PhotoBoothFullPath $InstallRoot
    $volumeRoot = [IO.Path]::GetPathRoot($fullRoot).TrimEnd([IO.Path]::DirectorySeparatorChar)
    if ($fullRoot.Equals($volumeRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'InstallRoot cannot be a drive root.'
    }

    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        throw 'LOCALAPPDATA is unavailable; cannot protect the user-data directory.'
    }

    $userDataRoot = Join-Path $env:LOCALAPPDATA 'PhotoBooth'
    if ((Test-PhotoBoothPathWithin -Path $fullRoot -Parent $userDataRoot) -or
        (Test-PhotoBoothPathWithin -Path $userDataRoot -Parent $fullRoot)) {
        throw "InstallRoot '$fullRoot' overlaps the user-data directory '$userDataRoot'. Choose a separate installation folder."
    }

    return $fullRoot
}

function Assert-PhotoBoothNotRunning {
    param([Parameter(Mandatory = $true)][string]$InstallRoot)

    $fullRoot = Get-PhotoBoothFullPath $InstallRoot
    $processes = @(Get-CimInstance -ClassName Win32_Process -Filter "Name = 'PhotoBooth.exe'" -ErrorAction Stop)
    foreach ($process in $processes) {
        $processPath = [string]$process.ExecutablePath
        if ([string]::IsNullOrWhiteSpace($processPath)) {
            throw "Cannot determine the executable path for running PhotoBooth PID $($process.ProcessId); refusing to install or launch while app state is uncertain."
        }
        throw "PhotoBooth is already running (PID $($process.ProcessId), path '$processPath'). Close it gracefully before installing, rolling back or launching another copy. Target install: $fullRoot"
    }
}

function Get-PhotoBoothSha256 {
    param([Parameter(Mandatory = $true)][string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant()
}

function Assert-PhotoBoothPE {
    param([Parameter(Mandatory = $true)][string]$Path)

    $stream = [IO.File]::OpenRead($Path)
    try {
        if ($stream.Length -lt 2) {
            throw "Executable is too small: $Path"
        }
        $first = $stream.ReadByte()
        $second = $stream.ReadByte()
        if ($first -ne 0x4D -or $second -ne 0x5A) {
            throw "Executable does not have a Windows PE/DOS MZ header: $Path"
        }
    }
    finally {
        $stream.Dispose()
    }
}

function Read-PhotoBoothReleaseManifest {
    param([Parameter(Mandatory = $true)][string]$ReleaseDirectory)

    $manifestPath = Join-Path $ReleaseDirectory 'release-manifest.json'
    if (!(Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "Release manifest is missing: $manifestPath"
    }

    try {
        $manifest = Get-Content -LiteralPath $manifestPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        throw "Release manifest is invalid JSON: $($_.Exception.Message)"
    }

    if ($manifest.schema_version -ne $script:PhotoBoothReleaseSchema) {
        throw "Unsupported release manifest schema: $($manifest.schema_version)"
    }
    if ($manifest.product -ne $script:PhotoBoothReleaseProduct -or $manifest.channel -ne $script:PhotoBoothReleaseChannel) {
        throw 'Release manifest product/channel does not match this internal Windows Preview installer.'
    }
    Assert-PhotoBoothVersion ([string]$manifest.version)
    if ($manifest.target -ne 'windows-amd64' -or $manifest.entrypoint -ne 'PhotoBooth.exe') {
        throw 'Release target or entrypoint is not supported.'
    }
    if ($manifest.binary.file -ne 'PhotoBooth.exe') {
        throw 'Manifest binary file must be PhotoBooth.exe.'
    }
    if ([long]$manifest.binary.size -le 0 -or [string]$manifest.binary.sha256 -notmatch '^[0-9a-fA-F]{64}$') {
        throw 'Manifest binary size or SHA-256 is invalid.'
    }

    return $manifest
}

function Assert-PhotoBoothReleaseDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$ReleaseDirectory,
        [string]$ExpectedVersion,
        [string]$ExpectedSha256
    )

    $manifest = Read-PhotoBoothReleaseManifest -ReleaseDirectory $ReleaseDirectory
    if ($ExpectedVersion -and $manifest.version -ne $ExpectedVersion) {
        throw "Release version mismatch. Expected '$ExpectedVersion', found '$($manifest.version)'."
    }
    if ($ExpectedSha256 -and $manifest.binary.sha256 -ne $ExpectedSha256) {
        throw 'Release SHA-256 does not match the active pointer.'
    }

    $exePath = Join-Path $ReleaseDirectory 'PhotoBooth.exe'
    if (!(Test-Path -LiteralPath $exePath -PathType Leaf)) {
        throw "Release executable is missing: $exePath"
    }
    $file = Get-Item -LiteralPath $exePath
    if ([long]$file.Length -ne [long]$manifest.binary.size) {
        throw 'Release executable size does not match its manifest.'
    }
    Assert-PhotoBoothPE -Path $exePath
    $actualHash = Get-PhotoBoothSha256 -Path $exePath
    if ($actualHash -ne ([string]$manifest.binary.sha256).ToLowerInvariant()) {
        throw 'Release executable SHA-256 does not match its manifest.'
    }

    return [pscustomobject]@{
        Manifest = $manifest
        Executable = $exePath
        Sha256 = $actualHash
    }
}

function Get-PhotoBoothActivePointer {
    param([Parameter(Mandatory = $true)][string]$InstallRoot)

    $pointerPath = Join-Path $InstallRoot 'active.json'
    if (!(Test-Path -LiteralPath $pointerPath -PathType Leaf)) {
        return $null
    }

    try {
        $pointer = Get-Content -LiteralPath $pointerPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        throw "Active release pointer is invalid JSON: $($_.Exception.Message)"
    }

    if ($pointer.schema_version -ne $script:PhotoBoothReleaseSchema -or
        $pointer.product -ne $script:PhotoBoothReleaseProduct) {
        throw 'Active release pointer has an unsupported schema or product.'
    }

    Assert-PhotoBoothVersion ([string]$pointer.active_version)
    if ([string]$pointer.active_sha256 -notmatch '^[0-9a-fA-F]{64}$') {
        throw 'Active release pointer SHA-256 is invalid.'
    }

    if ($pointer.previous_version) {
        Assert-PhotoBoothVersion ([string]$pointer.previous_version)
        if ([string]$pointer.previous_sha256 -notmatch '^[0-9a-fA-F]{64}$') {
            throw 'Previous release pointer SHA-256 is invalid.'
        }
    }

    return $pointer
}

function Write-PhotoBoothActivePointer {
    param(
        [Parameter(Mandatory = $true)][string]$InstallRoot,
        [Parameter(Mandatory = $true)][string]$ActiveVersion,
        [Parameter(Mandatory = $true)][string]$ActiveSha256,
        [string]$PreviousVersion,
        [string]$PreviousSha256
    )

    Assert-PhotoBoothVersion $ActiveVersion
    if ($ActiveSha256 -notmatch '^[0-9a-fA-F]{64}$') {
        throw 'Active SHA-256 must be 64 hexadecimal characters.'
    }
    if ($PreviousVersion) {
        Assert-PhotoBoothVersion $PreviousVersion
        if ($PreviousSha256 -notmatch '^[0-9a-fA-F]{64}$') {
            throw 'Previous SHA-256 must be 64 hexadecimal characters.'
        }
    }

    $pointer = [ordered]@{
        schema_version = $script:PhotoBoothReleaseSchema
        product = $script:PhotoBoothReleaseProduct
        active_version = $ActiveVersion
        active_sha256 = $ActiveSha256.ToLowerInvariant()
        previous_version = $PreviousVersion
        previous_sha256 = if ($PreviousVersion) { $PreviousSha256.ToLowerInvariant() } else { $null }
        updated_utc = [DateTime]::UtcNow.ToString('o')
    }

    $json = $pointer | ConvertTo-Json -Depth 6
    $destination = Join-Path $InstallRoot 'active.json'
    $temporary = Join-Path $InstallRoot ('.active-' + [guid]::NewGuid().ToString('N') + '.tmp')
    $backup = Join-Path $InstallRoot ('.active-backup-' + [guid]::NewGuid().ToString('N') + '.tmp')
    [IO.File]::WriteAllText($temporary, $json, [Text.UTF8Encoding]::new($false))

    try {
        if (Test-Path -LiteralPath $destination -PathType Leaf) {
            [IO.File]::Replace($temporary, $destination, $backup)
        }
        else {
            [IO.File]::Move($temporary, $destination)
        }
    }
    finally {
        foreach ($stalePath in @($temporary, $backup)) {
            if (Test-Path -LiteralPath $stalePath) {
                Remove-Item -LiteralPath $stalePath -Force -ErrorAction SilentlyContinue
            }
        }
    }
}
