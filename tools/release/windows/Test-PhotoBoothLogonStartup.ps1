[CmdletBinding()]
param(
    [string]$InstallRoot,
    [switch]$AllowUnsignedPreview,
    [ValidateRange(10,300)][int]$StartupWaitSeconds = 90
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'Release.Common.ps1')

if ([string]::IsNullOrWhiteSpace($InstallRoot)) {
    $InstallRoot = Join-Path $env:LOCALAPPDATA 'Programs\PhotoBooth'
}
$InstallRoot = Assert-PhotoBoothInstallRoot -InstallRoot $InstallRoot
if (!(Test-Path -LiteralPath $InstallRoot -PathType Container)) {
    throw "PhotoBooth is not installed for this Windows user: $InstallRoot"
}

$pointer = Get-PhotoBoothActivePointer -InstallRoot $InstallRoot
if (!$pointer) { throw 'No active PhotoBooth release pointer exists.' }
$releaseDirectory = Join-Path (Join-Path $InstallRoot 'releases') $pointer.active_version
$release = Assert-PhotoBoothReleaseDirectory -ReleaseDirectory $releaseDirectory -ExpectedVersion $pointer.active_version -ExpectedSha256 $pointer.active_sha256 -AllowUnsignedPreview:$AllowUnsignedPreview

$runPath = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$runValue = Get-ItemProperty -Path $runPath -Name PhotoBooth -ErrorAction Stop
$expectedCommand = '"' + $release.Executable + '" --kiosk'
if ([string]$runValue.PhotoBooth -cne $expectedCommand) {
    throw "Current-user Run value does not match the active release. Expected: $expectedCommand"
}
Write-Host 'HKCU_RUN_VALUE_MATCHES_ACTIVE_RELEASE=PASS'

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms

$sessionId = (Get-Process -Id $PID).SessionId
$deadline = (Get-Date).AddSeconds($StartupWaitSeconds)
$appProcess = $null
$processInfo = $null

while ((Get-Date) -lt $deadline) {
    $candidates = @(Get-CimInstance -ClassName Win32_Process -Filter "Name = 'PhotoBooth.exe'" -ErrorAction Stop |
        Where-Object {
            [int]$_.SessionId -eq $sessionId -and
            [string]::Equals([string]$_.ExecutablePath, $release.Executable, [StringComparison]::OrdinalIgnoreCase) -and
            ([string]$_.CommandLine).Contains('--kiosk')
        })
    if ($candidates.Count -gt 0) {
        $processInfo = $candidates | Select-Object -First 1
        $appProcess = Get-Process -Id ([int]$processInfo.ProcessId) -ErrorAction SilentlyContinue
        if ($appProcess) { break }
    }
    Start-Sleep -Milliseconds 500
}
if (!$appProcess) {
    throw "No PhotoBooth process from the active release was found in this interactive session within $StartupWaitSeconds seconds. Sign out and sign in again; do not launch it manually before rerunning this check."
}
Write-Host 'PHOTOBOOTH_PROCESS_IN_CURRENT_INTERACTIVE_SESSION=PASS'

$condition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ProcessIdProperty,
    [int]$appProcess.Id
)
$window = $null
$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll(
        [System.Windows.Automation.TreeScope]::Children,
        $condition
    )
    if ($windows.Count -gt 0) {
        $window = $windows.Item(0)
        break
    }
    Start-Sleep -Milliseconds 250
}
if (!$window) { throw 'PhotoBooth process exists but its top-level UI window did not appear.' }

$names = [System.Collections.Generic.List[string]]::new()
foreach ($element in $window.FindAll(
    [System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.Condition]::TrueCondition
)) {
    if (![string]::IsNullOrWhiteSpace($element.Current.Name)) {
        $names.Add($element.Current.Name) | Out-Null
    }
}
if ($names -notcontains 'Start with webcam') {
    throw "Customer start action was not found in the UI Automation tree. Visible names: $($names -join ' | ')"
}
if ($names -contains 'Operator access') {
    throw 'Customer Mode unexpectedly exposes a public Operator access entry.'
}
Write-Host 'CUSTOMER_MODE_UI_VISIBLE=PASS'
Write-Host 'NO_PUBLIC_OPERATOR_ENTRY=PASS'

$bounds = $window.Current.BoundingRectangle
$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$fullscreen = (
    [Math]::Abs($bounds.Left - $screen.Left) -le 5 -and
    [Math]::Abs($bounds.Top - $screen.Top) -le 5 -and
    [Math]::Abs($bounds.Right - $screen.Right) -le 5 -and
    [Math]::Abs($bounds.Bottom - $screen.Bottom) -le 5
)
if (!$fullscreen) {
    throw "PhotoBooth is not fullscreen. Window bounds=$($bounds.ToString()); screen=$($screen.ToString())"
}
Write-Host 'CUSTOMER_MODE_FULLSCREEN=PASS'

$parentName = $null
$parent = Get-CimInstance -ClassName Win32_Process -Filter "ProcessId = $($processInfo.ParentProcessId)" -ErrorAction SilentlyContinue
if ($parent) { $parentName = [string]$parent.Name }

$report = [ordered]@{
    status = 'PASS'
    checked_at = (Get-Date).ToString('o')
    user = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    session_id = $sessionId
    process_id = $appProcess.Id
    process_start_time = $appProcess.StartTime.ToString('o')
    parent_process_name = $parentName
    executable = $release.Executable
    command_line = [string]$processInfo.CommandLine
    checks = @(
        'HKCU Run value matches active release and --kiosk',
        'PhotoBooth process exists in current interactive session',
        'Customer start action is visible',
        'No public Operator access entry is exposed',
        'Customer window fills the primary display'
    )
}
$reportPath = Join-Path $env:TEMP ('photobooth-logon-acceptance-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
$report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $reportPath -Encoding UTF8
Write-Host 'PHOTOBOOTH_LOGON_STARTUP_ACCEPTANCE=PASS'
Write-Host "USER=$($report.user)"
Write-Host "PROCESS_ID=$($appProcess.Id)"
Write-Host "PARENT_PROCESS=$parentName"
Write-Host "REPORT=$reportPath"
