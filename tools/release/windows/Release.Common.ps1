Set-StrictMode -Version Latest

$script:PhotoBoothReleaseProduct = 'PhotoBooth Windows Preview'
$script:PhotoBoothReleaseChannel = 'internal-alpha'
$script:PhotoBoothReleaseSchema = 1

# Production trust is deliberately empty until the product owner selects and
# obtains the real publisher certificate. Never replace this with a self-signed
# development/test certificate. Pin the approved SHA-1 certificate thumbprint
# here after confirming it out-of-band with the organization that owns the key.
$script:PhotoBoothTrustedPublisherThumbprints = @()

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

    $jsonManifestPath = Join-Path $ReleaseDirectory 'release-manifest.json'
    $signedManifestPath = Join-Path $ReleaseDirectory 'release-manifest.ps1'
    $hasJsonManifest = Test-Path -LiteralPath $jsonManifestPath -PathType Leaf
    $hasSignedManifest = Test-Path -LiteralPath $signedManifestPath -PathType Leaf
    if ($hasJsonManifest -eq $hasSignedManifest) {
        throw 'Release must contain exactly one manifest: release-manifest.json (unsigned Preview) or release-manifest.ps1 (signed publisher envelope).'
    }

    $manifestSignerThumbprint = $null
    try {
        if ($hasSignedManifest) {
            $signature = Get-AuthenticodeSignature -FilePath $signedManifestPath -ErrorAction Stop
            if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid -or !$signature.SignerCertificate) {
                throw "Signed release manifest has no valid Authenticode signature: $($signature.Status) $($signature.StatusMessage)"
            }
            $manifestSignerThumbprint = (($signature.SignerCertificate.Thumbprint -replace '\s', '').ToUpperInvariant())
            $trusted = @($script:PhotoBoothTrustedPublisherThumbprints | ForEach-Object { ($_ -replace '\s', '').ToUpperInvariant() })
            if ($trusted.Count -eq 0 -or $trusted -notcontains $manifestSignerThumbprint) {
                throw "Release manifest signer $manifestSignerThumbprint is not in the pinned PhotoBooth publisher trust list."
            }

            $envelopeText = Get-Content -LiteralPath $signedManifestPath -Raw -ErrorAction Stop
            $firstLine = ($envelopeText -split '\r?\n', 2)[0]
            $prefix = '$PhotoBoothReleaseManifestBase64 = '''
            if (!$firstLine.StartsWith($prefix, [StringComparison]::Ordinal) -or !$firstLine.EndsWith("'")) {
                throw 'Signed release manifest envelope has an invalid payload declaration.'
            }
            $payload = $firstLine.Substring($prefix.Length, $firstLine.Length - $prefix.Length - 1)
            if ([string]::IsNullOrWhiteSpace($payload) -or $payload -notmatch '^[A-Za-z0-9+/=]+$') {
                throw 'Signed release manifest envelope payload is invalid.'
            }
            $manifestBytes = [Convert]::FromBase64String($payload)
            $manifestJson = [Text.UTF8Encoding]::new($false, $true).GetString($manifestBytes)
        }
        else {
            $manifestJson = Get-Content -LiteralPath $jsonManifestPath -Raw -ErrorAction Stop
        }
        $manifest = $manifestJson | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        throw "Release manifest or publisher signature is invalid: $($_.Exception.Message)"
    }

    if ($manifest.schema_version -ne $script:PhotoBoothReleaseSchema) {
        throw "Unsupported release manifest schema: $($manifest.schema_version)"
    }
    if ($manifest.product -ne $script:PhotoBoothReleaseProduct -or
        @('internal-alpha', 'signed-candidate') -notcontains [string]$manifest.channel) {
        throw 'Release manifest product/channel does not match a supported PhotoBooth Windows Preview release channel.'
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
    if ($manifest.PSObject.Properties.Name -contains 'smb_admin_helper') {
        if ($manifest.smb_admin_helper.file -ne 'manage-smb-remote-admin.ps1' -or
            [long]$manifest.smb_admin_helper.size -le 0 -or
            [string]$manifest.smb_admin_helper.sha256 -notmatch '^[0-9a-fA-F]{64}$') {
            throw 'Manifest SMB administration helper file, size or SHA-256 is invalid.'
        }
    }

    if (!$manifest.distribution -or
        $manifest.distribution.signed -isnot [bool]) {
        throw 'Release manifest must declare distribution.signed as a JSON boolean.'
    }
    if ($manifest.distribution.signed) {
        if (!$hasSignedManifest -or [string]$manifest.distribution.signer_thumbprint -notmatch '^[0-9a-fA-F]{40}$') {
            throw 'A signed release must use a signed manifest envelope and declare its 40-character Authenticode signer thumbprint.'
        }
        if (([string]$manifest.distribution.signer_thumbprint).ToUpperInvariant() -ne $manifestSignerThumbprint) {
            throw 'Signed manifest payload signer_thumbprint does not match the Authenticode envelope signer.'
        }
        if ($manifest.channel -ne 'signed-candidate') {
            throw 'A signed publisher package must use the signed-candidate channel; this does not approve production UI selection.'
        }
        Add-Member -InputObject $manifest -NotePropertyName __manifest_signer_thumbprint -NotePropertyValue $manifestSignerThumbprint -Force
    } else {
        if ($hasSignedManifest -or $manifest.distribution.signer_thumbprint) {
            throw 'An unsigned Preview release must use the plain JSON manifest and must not declare a signer thumbprint.'
        }
        if ($manifest.channel -ne 'internal-alpha') {
            throw 'An unsigned package is only valid on the internal-alpha channel.'
        }
    }

    return $manifest
}

function Assert-PhotoBoothReleaseDirectory {
    param(
        [Parameter(Mandatory = $true)][string]$ReleaseDirectory,
        [string]$ExpectedVersion,
        [string]$ExpectedSha256,
        [switch]$AllowUnsignedPreview
    )

    $manifest = Read-PhotoBoothReleaseManifest -ReleaseDirectory $ReleaseDirectory
    $isSigned = [bool]$manifest.distribution.signed
    if ($isSigned) {
        # Read-PhotoBoothReleaseManifest already validated the signed envelope
        # against the pinned publisher before parsing any release metadata.
    } elseif (!$AllowUnsignedPreview) {
        throw 'Unsigned engineering packages are accepted only for internal Preview testing; pass -AllowUnsignedPreview explicitly. Production installation requires a pinned Authenticode publisher.'
    }

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
    if ($isSigned) {
        [void](Assert-PhotoBoothAuthenticodeSignature -Path $exePath -ExpectedThumbprint $manifest.distribution.signer_thumbprint)
    }

    $helperPath = Join-Path $ReleaseDirectory 'manage-smb-remote-admin.ps1'
    if ($manifest.PSObject.Properties.Name -contains 'smb_admin_helper') {
        if (!(Test-Path -LiteralPath $helperPath -PathType Leaf)) {
            throw "Release SMB administration helper is missing: $helperPath"
        }
        $helperFile = Get-Item -LiteralPath $helperPath
        if ([long]$helperFile.Length -ne [long]$manifest.smb_admin_helper.size) {
            throw 'Release SMB administration helper size does not match its manifest.'
        }
        $helperHash = Get-PhotoBoothSha256 -Path $helperPath
        if ($helperHash -ne ([string]$manifest.smb_admin_helper.sha256).ToLowerInvariant()) {
            throw 'Release SMB administration helper SHA-256 does not match its manifest.'
        }
        if ($isSigned) {
            [void](Assert-PhotoBoothAuthenticodeSignature -Path $helperPath -ExpectedThumbprint $manifest.distribution.signer_thumbprint)
        }
    } elseif (Test-Path -LiteralPath $helperPath) {
        throw 'Release contains an unmanaged SMB administration helper that is not declared in its manifest.'
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


function Get-PhotoBoothSigningCertificate {
    param([Parameter(Mandatory = $true)][string]$Thumbprint)

    $normalized = $Thumbprint.Trim().ToUpperInvariant()
    if ($normalized -notmatch '^[0-9A-F]{40}$') {
        throw 'Signing certificate thumbprint must be a 40-character SHA-1 certificate thumbprint.'
    }

    $certificate = $null
    foreach ($store in @('Cert:\CurrentUser\My', 'Cert:\LocalMachine\My')) {
        $candidate = Get-ChildItem -Path $store -ErrorAction SilentlyContinue |
            Where-Object { $_.Thumbprint.ToUpperInvariant() -eq $normalized } |
            Select-Object -First 1
        if ($candidate) {
            $certificate = $candidate
            break
        }
    }
    if (!$certificate) {
        throw "Code-signing certificate $normalized was not found in CurrentUser\My or LocalMachine\My."
    }
    if (!$certificate.HasPrivateKey) {
        throw "Code-signing certificate $normalized does not have an accessible private key."
    }
    $now = [DateTime]::UtcNow
    if ($now -lt $certificate.NotBefore.ToUniversalTime() -or $now -gt $certificate.NotAfter.ToUniversalTime()) {
        throw "Code-signing certificate $normalized is outside its validity period."
    }
    $hasCodeSigningEku = $false
    foreach ($extension in $certificate.Extensions) {
        if ($extension -is [Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]) {
            foreach ($oid in $extension.EnhancedKeyUsages) {
                if ($oid.Value -eq '1.3.6.1.5.5.7.3.3') {
                    $hasCodeSigningEku = $true
                    break
                }
            }
        }
    }
    if (!$hasCodeSigningEku) {
        throw "Certificate $normalized does not contain the Code Signing EKU."
    }
    return $certificate
}

function Set-PhotoBoothAuthenticodeSignature {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][Security.Cryptography.X509Certificates.X509Certificate2]$Certificate,
        [string]$TimestampServer
    )

    $parameters = @{
        FilePath = $Path
        Certificate = $Certificate
        HashAlgorithm = 'SHA256'
        ErrorAction = 'Stop'
    }
    if (![string]::IsNullOrWhiteSpace($TimestampServer)) {
        $parameters.TimestampServer = $TimestampServer
    }
    $result = Set-AuthenticodeSignature @parameters
    if ($result.Status -ne [Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode signing failed for '$Path': $($result.Status) $($result.StatusMessage)"
    }
    if (![string]::IsNullOrWhiteSpace($TimestampServer)) {
        $verifiedSignature = Get-AuthenticodeSignature -FilePath $Path -ErrorAction Stop
        if (!$verifiedSignature.TimeStamperCertificate) {
            throw "Authenticode signature for '$Path' was created without a verifiable timestamp."
        }
    }
    $actualThumbprint = $result.SignerCertificate.Thumbprint.ToUpperInvariant()
    if ($actualThumbprint -ne $Certificate.Thumbprint.ToUpperInvariant()) {
        throw "Authenticode signer mismatch after signing '$Path'."
    }
    return $actualThumbprint
}

function Assert-PhotoBoothAuthenticodeSignature {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$ExpectedThumbprint
    )

    $expected = $ExpectedThumbprint.Trim().ToUpperInvariant()
    if ($expected -notmatch '^[0-9A-F]{40}$') {
        throw 'Expected signer thumbprint is invalid.'
    }
    $signature = Get-AuthenticodeSignature -FilePath $Path -ErrorAction Stop
    if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode signature is not valid for '$Path': $($signature.Status) $($signature.StatusMessage)"
    }
    if (!$signature.SignerCertificate) {
        throw "Authenticode signature has no signer certificate: $Path"
    }
    $actual = $signature.SignerCertificate.Thumbprint.ToUpperInvariant()
    if ($actual -ne $expected) {
        throw "Authenticode signer for '$Path' does not match the release manifest."
    }
    return $actual
}

function Set-PhotoBoothCurrentUserAutoStart {
    param(
        [Parameter(Mandatory = $true)][string]$InstallRoot,
        [Parameter(Mandatory = $true)][string]$ExecutablePath,
        [string]$RegistryPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run',
        [string]$ValueName = 'PhotoBooth'
    )

    if ($RegistryPath -notmatch '^HKCU:\\Software\\') {
        throw 'PhotoBooth auto-start may only write under the current user HKCU Software hive.'
    }
    $fullRoot = Get-PhotoBoothFullPath $InstallRoot
    $fullExecutable = [IO.Path]::GetFullPath($ExecutablePath)
    $releasesRoot = Join-Path $fullRoot 'releases'
    if (!(Test-PhotoBoothPathWithin -Path $fullExecutable -Parent $releasesRoot)) {
        throw 'Auto-start executable must be inside this installation root''s immutable releases directory.'
    }
    if (!(Test-Path -LiteralPath $fullExecutable -PathType Leaf)) {
        throw "Auto-start executable does not exist: $fullExecutable"
    }
    if ([IO.Path]::GetExtension($fullExecutable) -ine '.exe') {
        throw 'Windows auto-start target must be a .exe file.'
    }

    $command = '"' + $fullExecutable + '" --kiosk'
    New-Item -Path $RegistryPath -Force | Out-Null
    New-ItemProperty -Path $RegistryPath -Name $ValueName -Value $command -PropertyType String -Force | Out-Null

    $actual = [string](Get-ItemProperty -Path $RegistryPath -Name $ValueName -ErrorAction Stop).$ValueName
    if ($actual -cne $command) {
        throw 'The current-user PhotoBooth auto-start registry value did not persist exactly.'
    }
    return $command
}

function Remove-PhotoBoothCurrentUserAutoStart {
    param(
        [string]$InstallRoot,
        [string]$RegistryPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run',
        [string]$ValueName = 'PhotoBooth'
    )

    if ($RegistryPath -notmatch '^HKCU:\\Software\\') {
        throw 'PhotoBooth auto-start may only remove values under the current user HKCU Software hive.'
    }
    if (!(Test-Path -LiteralPath $RegistryPath)) {
        return $false
    }
    $property = Get-ItemProperty -Path $RegistryPath -Name $ValueName -ErrorAction SilentlyContinue
    if (!$property) {
        return $false
    }

    $value = [string]$property.$ValueName
    if (![string]::IsNullOrWhiteSpace($InstallRoot)) {
        if ($value -notmatch '^"([^"]+)"\s+--kiosk$') {
            return $false
        }
        $target = [IO.Path]::GetFullPath($Matches[1])
        $fullRoot = Get-PhotoBoothFullPath $InstallRoot
        if (!(Test-PhotoBoothPathWithin -Path $target -Parent $fullRoot)) {
            return $false
        }
    }

    Remove-ItemProperty -Path $RegistryPath -Name $ValueName -ErrorAction Stop
    return $true
}
