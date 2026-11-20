[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$sourceExe = Join-Path $repoRoot 'tools\ui-spike\wails-v2\build\bin\PhotoBooth.exe'
$sourceHelper = Join-Path $repoRoot 'tools\self-service\manage-smb-remote-admin.ps1'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('photobooth-release-signing-smoke-' + [guid]::NewGuid().ToString('N'))
$certificate = $null
$trustedRootAdded = $false
$trustedPublisherAdded = $false
$checks = [System.Collections.Generic.List[string]]::new()

function Assert-Throws {
    param([scriptblock]$Action, [string]$ExpectedText, [string]$Label)
    $threw = $false
    try { & $Action | Out-Null }
    catch {
        $threw = $_.Exception.Message -match [regex]::Escape($ExpectedText)
        if (!$threw) { throw "$Label failed for an unexpected reason: $($_.Exception.Message)" }
    }
    if (!$threw) { throw "$Label did not fail with expected text '$ExpectedText'." }
}

function Write-UnsignedFixture {
    param([string]$Directory, [string]$Version, [string]$ExecutableSource)
    New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    Copy-Item -LiteralPath $ExecutableSource -Destination (Join-Path $Directory 'PhotoBooth.exe')
    $binary = Join-Path $Directory 'PhotoBooth.exe'
    $manifest = [ordered]@{
        schema_version = 1
        product = $script:PhotoBoothReleaseProduct
        channel = 'internal-alpha'
        version = $Version
        target = 'windows-amd64'
        entrypoint = 'PhotoBooth.exe'
        binary = [ordered]@{
            file = 'PhotoBooth.exe'
            size = [long](Get-Item -LiteralPath $binary).Length
            sha256 = Get-PhotoBoothSha256 -Path $binary
        }
        distribution = [ordered]@{
            signed = $false
            signer_thumbprint = $null
            intended_use = 'temporary unsigned-preview-signing-smoke'
        }
    }
    [IO.File]::WriteAllText(
        (Join-Path $Directory 'release-manifest.json'),
        ($manifest | ConvertTo-Json -Depth 8),
        [Text.UTF8Encoding]::new($false)
    )
}

try {
    if (!(Test-Path -LiteralPath $sourceExe -PathType Leaf)) { throw "Wails executable is missing: $sourceExe" }
    if (!(Test-Path -LiteralPath $sourceHelper -PathType Leaf)) { throw "SMB helper is missing: $sourceHelper" }
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null

    # This certificate is intentionally ephemeral and is NEVER written to the
    # production publisher allowlist. It validates only the signing mechanics.
    $subject = 'CN=PhotoBooth TEST ONLY ' + [guid]::NewGuid().ToString('N')
    $certificate = New-SelfSignedCertificate -Subject $subject -CertStoreLocation 'Cert:\CurrentUser\My' -Type CodeSigningCert -KeyAlgorithm RSA -KeyLength 2048 -HashAlgorithm SHA256 -NotAfter (Get-Date).AddDays(7)
    if (!$certificate -or !$certificate.HasPrivateKey) {
        throw 'Could not create the temporary test-only Code Signing certificate.'
    }
    $certificateFile = Join-Path $tempRoot 'photobooth-test-only-codesign.cer'
    Export-Certificate -Cert $certificate -FilePath $certificateFile -Type CERT | Out-Null
    Import-Certificate -FilePath $certificateFile -CertStoreLocation 'Cert:\CurrentUser\Root' | Out-Null
    $trustedRootAdded = $true
    Import-Certificate -FilePath $certificateFile -CertStoreLocation 'Cert:\CurrentUser\TrustedPublisher' | Out-Null
    $trustedPublisherAdded = $true
    $script:PhotoBoothTrustedPublisherThumbprints = @($certificate.Thumbprint.ToUpperInvariant())
    $checks.Add('create an ephemeral test-only code-signing certificate and temporary trust chain') | Out-Null

    $signedRoot = Join-Path $tempRoot 'signed-release'
    New-Item -ItemType Directory -Path $signedRoot -Force | Out-Null
    $exe = Join-Path $signedRoot 'PhotoBooth.exe'
    $helper = Join-Path $signedRoot 'manage-smb-remote-admin.ps1'
    Copy-Item -LiteralPath $sourceExe -Destination $exe
    Copy-Item -LiteralPath $sourceHelper -Destination $helper

    [void](Set-PhotoBoothAuthenticodeSignature -Path $exe -Certificate $certificate)
    [void](Set-PhotoBoothAuthenticodeSignature -Path $helper -Certificate $certificate)

    $manifest = [ordered]@{
        schema_version = 1
        product = $script:PhotoBoothReleaseProduct
        channel = 'signed-candidate'
        version = '99.99.99-signing-smoke'
        target = 'windows-amd64'
        entrypoint = 'PhotoBooth.exe'
        binary = [ordered]@{
            file = 'PhotoBooth.exe'
            size = [long](Get-Item -LiteralPath $exe).Length
            sha256 = Get-PhotoBoothSha256 -Path $exe
        }
        smb_admin_helper = [ordered]@{
            file = 'manage-smb-remote-admin.ps1'
            size = [long](Get-Item -LiteralPath $helper).Length
            sha256 = Get-PhotoBoothSha256 -Path $helper
        }
        distribution = [ordered]@{
            signed = $true
            signer_thumbprint = $certificate.Thumbprint.ToUpperInvariant()
            signature_type = 'Authenticode-SHA256'
            intended_use = 'test-only; not a production publisher'
        }
    }
    $manifestJson = $manifest | ConvertTo-Json -Depth 8
    $payload = [Convert]::ToBase64String([Text.UTF8Encoding]::new($false).GetBytes($manifestJson))
    $envelope = '$PhotoBoothReleaseManifestBase64 = ''' + $payload + '''' + [Environment]::NewLine
    $manifestPath = Join-Path $signedRoot 'release-manifest.ps1'
    [IO.File]::WriteAllText($manifestPath, $envelope, [Text.UTF8Encoding]::new($false))
    [void](Set-PhotoBoothAuthenticodeSignature -Path $manifestPath -Certificate $certificate)

    $validated = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $signedRoot
    if ($validated.Manifest.version -ne '99.99.99-signing-smoke') {
        throw 'Signed manifest did not round-trip the expected version.'
    }
    $checks.Add('verify signed manifest envelope before trusting metadata') | Out-Null
    $checks.Add('verify Authenticode publisher and SHA-256 for EXE and SMB helper') | Out-Null

    # A modified script must not pass the Authenticode file verification.
    $tamperedHelper = Join-Path $signedRoot 'tampered-helper.ps1'
    Copy-Item -LiteralPath $helper -Destination $tamperedHelper
    [IO.File]::AppendAllText($tamperedHelper, '# tampered')
    Assert-Throws { Assert-PhotoBoothAuthenticodeSignature -Path $tamperedHelper -ExpectedThumbprint $certificate.Thumbprint } 'Authenticode signature is not valid' 'tampered helper verification'
    $checks.Add('reject a tampered signed helper') | Out-Null

    # Production trust is deliberately empty until a genuine publisher is selected.
    $pinned = $script:PhotoBoothTrustedPublisherThumbprints
    try {
        $script:PhotoBoothTrustedPublisherThumbprints = @()
        Assert-Throws { Read-PhotoBoothReleaseManifest -ReleaseDirectory $signedRoot } 'not in the pinned PhotoBooth publisher trust list' 'empty production publisher pin'
        $checks.Add('reject signed packages while the production publisher allowlist is empty') | Out-Null

        # A valid Windows-trusted test certificate is still rejected when its
        # thumbprint is not the one pinned by PhotoBooth.
        $script:PhotoBoothTrustedPublisherThumbprints = @('A' * 40)
        Assert-Throws { Read-PhotoBoothReleaseManifest -ReleaseDirectory $signedRoot } 'not in the pinned PhotoBooth publisher trust list' 'publisher thumbprint pin'
    }
    finally {
        $script:PhotoBoothTrustedPublisherThumbprints = $pinned
    }
    $checks.Add('reject a valid signature from a publisher not pinned by PhotoBooth') | Out-Null

    $unsignedRoot = Join-Path $tempRoot 'unsigned-preview'
    Write-UnsignedFixture -Directory $unsignedRoot -Version '0.1.0-preview-smoke' -ExecutableSource $sourceExe
    Assert-Throws { Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $unsignedRoot } 'pass -AllowUnsignedPreview explicitly' 'unsigned default-deny check'
    [void](Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $unsignedRoot -AllowUnsignedPreview)
    $checks.Add('reject unsigned Preview by default and accept only with explicit opt-in') | Out-Null

    $tamperedManifest = Join-Path $tempRoot 'tampered-manifest'
    Copy-Item -LiteralPath $signedRoot -Destination $tamperedManifest -Recurse
    $tamperedManifestPath = Join-Path $tamperedManifest 'release-manifest.ps1'
    $envelopeLines = [IO.File]::ReadAllLines($tamperedManifestPath)
    $payloadPrefix = '$PhotoBoothReleaseManifestBase64 = '''
    $payloadStart = $payloadPrefix.Length
    $firstLineChars = $envelopeLines[0].ToCharArray()
    $firstLineChars[$payloadStart] = if ($firstLineChars[$payloadStart] -eq 'A') { 'B' } else { 'A' }
    $envelopeLines[0] = -join $firstLineChars
    [IO.File]::WriteAllLines($tamperedManifestPath,$envelopeLines,[Text.UTF8Encoding]::new($false))
    Assert-Throws { Read-PhotoBoothReleaseManifest -ReleaseDirectory $tamperedManifest } 'publisher signature is invalid' 'tampered signed manifest'
    $checks.Add('reject a tampered signed manifest before parsing its release metadata') | Out-Null

    Write-Output 'PHOTOBOOTH_RELEASE_SIGNING_SMOKE=PASS'
    Write-Output ("TEST_CERTIFICATE_THUMBPRINT=" + $certificate.Thumbprint)
    Write-Output 'TEST_CERTIFICATE_PRODUCTION_TRUST_PINNED=False'
    foreach ($check in $checks) { Write-Output ("CHECK=" + $check) }
}
catch {
    Write-Output 'PHOTOBOOTH_RELEASE_SIGNING_SMOKE=FAIL'
    Write-Output ("FAILURE=" + $_.Exception.Message)
    throw
}
finally {
    if ($certificate) {
        if ($trustedPublisherAdded) {
            Remove-Item -LiteralPath ('Cert:\CurrentUser\TrustedPublisher\' + $certificate.Thumbprint) -Force -ErrorAction SilentlyContinue
        }
        if ($trustedRootAdded) {
            Remove-Item -LiteralPath ('Cert:\CurrentUser\Root\' + $certificate.Thumbprint) -Force -ErrorAction SilentlyContinue
        }
        Remove-Item -LiteralPath ('Cert:\CurrentUser\My\' + $certificate.Thumbprint) -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Output 'TEST_CERTIFICATE_AND_TEMP_FILES_REMOVED=True'
}
