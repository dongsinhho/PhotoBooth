[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$OutputPath
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$uiDirectory = Join-Path $repoRoot 'tools\ui-spike\wails-v2'
$manifestPath = Join-Path $repoRoot 'third_party\licenses\manifest.json'

if (!(Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    throw "Third-party license manifest is missing: $manifestPath"
}

$manifest = Get-Content -LiteralPath $manifestPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
if ($manifest.schema_version -ne 1 -or !$manifest.modules) {
    throw 'Third-party license manifest has an unsupported schema or an empty module list.'
}

Push-Location $uiDirectory
try {
    $moduleLines = @(& go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}{{end}}' . 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Could not enumerate the Windows application Go dependency closure: $($moduleLines -join [Environment]::NewLine)"
    }
}
finally {
    Pop-Location
}

$actualModules = @{}
$projectModulePrefix = 'github.com/dongsinhho/PhotoBooth'
foreach ($line in $moduleLines) {
    $value = ([string]$line).Trim()
    if ([string]::IsNullOrWhiteSpace($value)) { continue }
    $parts = $value -split '\|', 2
    if ($parts.Count -ne 2) { throw "Unexpected go list module record: $value" }
    $modulePath = $parts[0]
    if ($modulePath -eq $projectModulePrefix -or $modulePath.StartsWith($projectModulePrefix + '/', [StringComparison]::Ordinal)) {
        continue
    }
    $actualModules[$modulePath] = $parts[1]
}

$expectedModules = @{}
foreach ($module in $manifest.modules) {
    $key = [string]$module.path
    if ($expectedModules.ContainsKey($key)) {
        throw "Duplicate module path in third-party license manifest: $key"
    }
    $expectedModules[$key] = [string]$module.version
}

$missing = @($actualModules.Keys | Where-Object { !$expectedModules.ContainsKey($_) } | Sort-Object)
$stale = @($expectedModules.Keys | Where-Object { !$actualModules.ContainsKey($_) } | Sort-Object)
$versionMismatches = @()
foreach ($modulePath in $actualModules.Keys) {
    if ($expectedModules.ContainsKey($modulePath) -and $expectedModules[$modulePath] -ne $actualModules[$modulePath]) {
        $versionMismatches += "$modulePath expected=$($expectedModules[$modulePath]) actual=$($actualModules[$modulePath])"
    }
}
if ($missing.Count -gt 0 -or $stale.Count -gt 0 -or $versionMismatches.Count -gt 0) {
    $details = @()
    if ($missing.Count -gt 0) { $details += 'Unlisted runtime modules: ' + ($missing -join ', ') }
    if ($stale.Count -gt 0) { $details += 'Manifest modules not in this build: ' + ($stale -join ', ') }
    if ($versionMismatches.Count -gt 0) { $details += 'Version mismatches: ' + ($versionMismatches -join '; ') }
    throw "Third-party license manifest does not match the current Windows application dependency closure. $($details -join ' | ')"
}

$builder = [Text.StringBuilder]::new()
[void]$builder.AppendLine('PhotoBooth third-party license notices')
[void]$builder.AppendLine('Generated from third_party/licenses/manifest.json after validating the Windows Wails application dependency closure.')
[void]$builder.AppendLine('Preserve this file and the original notices when redistributing PhotoBooth binaries.')
[void]$builder.AppendLine()

foreach ($module in ($manifest.modules | Sort-Object path)) {
    [void]$builder.AppendLine(('=' * 78))
    [void]$builder.AppendLine("$($module.path) $($module.version)")
    [void]$builder.AppendLine("License: $($module.license)")
    if ($module.PSObject.Properties.Name -contains 'notes' -and ![string]::IsNullOrWhiteSpace([string]$module.notes)) { [void]$builder.AppendLine("Notes: $($module.notes)") }
    [void]$builder.AppendLine('-' * 78)

    foreach ($relativePath in $module.files) {
        $sourcePath = Join-Path $repoRoot ([string]$relativePath)
        if (!(Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
            throw "Required third-party license/notice file is missing: $relativePath"
        }
        $content = [IO.File]::ReadAllText($sourcePath)
        if ([string]::IsNullOrWhiteSpace($content)) {
            throw "Third-party license/notice file is empty: $relativePath"
        }
        [void]$builder.AppendLine("Source notice file: $relativePath")
        [void]$builder.AppendLine($content.TrimEnd())
        [void]$builder.AppendLine()
    }
}

$outputFullPath = [IO.Path]::GetFullPath($OutputPath)
$outputParent = Split-Path -Parent $outputFullPath
New-Item -ItemType Directory -Path $outputParent -Force | Out-Null
$finalText = $builder.ToString().TrimEnd([char[]]@([char]13, [char]10)) + [Environment]::NewLine
[IO.File]::WriteAllText($outputFullPath, $finalText, [Text.UTF8Encoding]::new($false))

Write-Output "THIRD_PARTY_LICENSE_MODULES=$($manifest.modules.Count)"
Write-Output "THIRD_PARTY_LICENSE_OUTPUT=$outputFullPath"
Write-Output "THIRD_PARTY_LICENSE_SHA256=$((Get-FileHash -LiteralPath $outputFullPath -Algorithm SHA256).Hash.ToLowerInvariant())"
