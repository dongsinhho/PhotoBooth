[CmdletBinding()]
param(
    [string]$ReportPath = (Join-Path $env:TEMP ("photobooth-linux-host-preflight-{0}.json" -f (Get-Date -Format 'yyyyMMdd-HHmmss')))
)

$ErrorActionPreference = 'Continue'

function Get-CommandPath([string]$Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    return $null
}

$os = Get-CimInstance Win32_OperatingSystem
$computer = Get-CimInstance Win32_ComputerSystem
$processor = Get-CimInstance Win32_Processor | Select-Object -First 1
$systemDrive = $env:SystemDrive.TrimEnd(':')
$volume = Get-Volume -DriveLetter $systemDrive -ErrorAction SilentlyContinue
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
$isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

$wslStatus = (& wsl.exe --status 2>&1 | Out-String).Trim()
$wslStatusExitCode = $LASTEXITCODE
$wslList = (& wsl.exe --list --verbose 2>&1 | Out-String).Trim()
$wslListExitCode = $LASTEXITCODE
$wslInstalled = ($wslStatusExitCode -eq 0 -and
    $wslListExitCode -eq 0 -and
    $wslStatus -notmatch 'WSL.*not installed|Windows Subsystem for Linux is not installed')
$featureStates = @{}
foreach ($feature in @('Microsoft-Windows-Subsystem-Linux', 'VirtualMachinePlatform')) {
    try {
        $state = Get-WindowsOptionalFeature -Online -FeatureName $feature -ErrorAction Stop
        $featureStates[$feature] = [string]$state.State
    } catch {
        $featureStates[$feature] = 'UNKNOWN_REQUIRES_ELEVATION_OR_QUERY_FAILED'
    }
}

$dockerPath = 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
$dockerVersion = $null
if (Test-Path -LiteralPath $dockerPath) {
    $dockerVersion = (Get-Item -LiteralPath $dockerPath).VersionInfo.ProductVersion
}
$dockerService = Get-Service -Name 'com.docker.service' -ErrorAction SilentlyContinue
$dockerInfo = (& docker info --format '{{.OSType}}|{{.ServerVersion}}' 2>&1 | Out-String).Trim()
$dockerEngineAvailable = $LASTEXITCODE -eq 0 -and $dockerInfo -match '^(linux|windows)\|'

$report = [ordered]@{
    generated_at = (Get-Date).ToString('o')
    host = @{
        os = $os.Caption
        os_version = $os.Version
        architecture = $os.OSArchitecture
        cpu = $processor.Name
        total_ram_gb = [math]::Round($computer.TotalPhysicalMemory / 1GB, 1)
        free_ram_gb = [math]::Round($os.FreePhysicalMemory / 1MB, 1)
        system_drive_free_gb = if ($volume) { [math]::Round($volume.SizeRemaining / 1GB, 1) } else { $null }
        hypervisor_present = [bool]$computer.HypervisorPresent
        virtualization_firmware_enabled = [bool]$processor.VirtualizationFirmwareEnabled
        vm_monitor_mode_extensions = [bool]$processor.VMMonitorModeExtensions
        slat_extensions = [bool]$processor.SecondLevelAddressTranslationExtensions
        current_process_is_admin = $isAdmin
    }
    wsl = @{
        installed = $wslInstalled
        status = $wslStatus
        distributions = $wslList
        optional_features = $featureStates
    }
    docker = @{
        desktop_version = $dockerVersion
        service_status = if ($dockerService) { [string]$dockerService.Status } else { 'NOT_FOUND' }
        engine_available = $dockerEngineAvailable
        engine_info = $dockerInfo
    }
    assessment = @{
        enough_disk_for_initial_eval = ($null -ne $volume -and $volume.SizeRemaining -ge 35GB)
        enough_ram_for_initial_eval = ($computer.TotalPhysicalMemory -ge 12GB)
        admin_elevation_available_in_current_process = $isAdmin
        linux_environment_ready = ($wslInstalled -or $dockerEngineAvailable)
    }
}

$parent = Split-Path -Parent $ReportPath
if (!(Test-Path -LiteralPath $parent)) {
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
}
$report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $ReportPath -Encoding UTF8

Write-Output 'PHOTOBOOTH_LINUX_HOST_PREFLIGHT'
Write-Output "OS=$($report.host.os) $($report.host.os_version) $($report.host.architecture)"
Write-Output "CPU=$($report.host.cpu)"
Write-Output "RAM_GB=$($report.host.total_ram_gb); FREE_RAM_GB=$($report.host.free_ram_gb)"
Write-Output "SYSTEM_DRIVE_FREE_GB=$($report.host.system_drive_free_gb)"
Write-Output "HYPERVISOR_PRESENT=$($report.host.hypervisor_present)"
Write-Output "VIRTUALIZATION_FIRMWARE_ENABLED=$($report.host.virtualization_firmware_enabled)"
Write-Output "VM_MONITOR_MODE_EXTENSIONS=$($report.host.vm_monitor_mode_extensions)"
Write-Output "SLAT_EXTENSIONS=$($report.host.slat_extensions)"
Write-Output "CURRENT_PROCESS_ADMIN=$isAdmin"
Write-Output "WSL_INSTALLED=$wslInstalled"
Write-Output "WSL_FEATURE=$($featureStates['Microsoft-Windows-Subsystem-Linux'])"
Write-Output "VIRTUAL_MACHINE_PLATFORM=$($featureStates['VirtualMachinePlatform'])"
Write-Output "DOCKER_DESKTOP_VERSION=$dockerVersion"
Write-Output "DOCKER_SERVICE=$($report.docker.service_status)"
Write-Output "DOCKER_LINUX_ENGINE_AVAILABLE=$dockerEngineAvailable"
Write-Output "ENOUGH_DISK_FOR_INITIAL_EVAL=$($report.assessment.enough_disk_for_initial_eval)"
Write-Output "ENOUGH_RAM_FOR_INITIAL_EVAL=$($report.assessment.enough_ram_for_initial_eval)"
Write-Output "LINUX_ENVIRONMENT_READY=$($report.assessment.linux_environment_ready)"
Write-Output "REPORT=$ReportPath"
