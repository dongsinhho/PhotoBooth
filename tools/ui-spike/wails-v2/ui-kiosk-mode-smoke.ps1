[CmdletBinding()]
param(
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(5,30)][int]$TransitionTimeoutSeconds = 12,
    [string]$ExecutablePath
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
if ([string]::IsNullOrWhiteSpace($ExecutablePath)) {
    $exe = Join-Path $PSScriptRoot 'build\bin\PhotoBooth.exe'
} else {
    $exe = (Resolve-Path -LiteralPath $ExecutablePath -ErrorAction Stop).Path
}
if (!(Test-Path -LiteralPath $exe -PathType Leaf)) { throw "PhotoBooth executable was not found: $exe" }
$exeWorkingDirectory = Split-Path -Parent $exe

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName Microsoft.VisualBasic

if (-not ('PhotoBoothKioskNative' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class PhotoBoothKioskNative {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }

    [DllImport("user32.dll", SetLastError=true)]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")]
    public static extern bool ShowWindow(IntPtr hWnd, int command);
    [DllImport("user32.dll")]
    public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern IntPtr SetFocus(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hWnd, IntPtr processId);
    [DllImport("kernel32.dll")]
    public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll")]
    public static extern bool AttachThreadInput(uint attach, uint attachTo, bool attachInput);
    [DllImport("user32.dll")]
    public static extern void keybd_event(byte virtualKey, byte scan, uint flags, UIntPtr extraInfo);
}
'@
}

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-kiosk-smoke-$runId"
$reportPath = Join-Path $env:TEMP "photobooth-kiosk-smoke-$runId.json"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$previousKiosk = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_KIOSK', 'Process')
$app = $null
$window = $null
$status = 'FAIL'
$failure = $null
$checks = [ordered]@{}
$dataCleaned = $false

function Get-AppWindow {
    if (!$app) { return $null }
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $app.Id
    )
    $windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($windows.Count -gt 0) { return $windows.Item(0) }
    return $null
}

function Get-WindowText {
    $current = Get-AppWindow
    if (!$current) { return @() }
    $values = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $current.FindAll(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.Condition]::TrueCondition
    )) {
        if ($element.Current.Name) { $values.Add($element.Current.Name) | Out-Null }
    }
    return $values.ToArray()
}

function Wait-Window([int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $current = Get-AppWindow
        if ($current) { return $current }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw 'PhotoBooth exited before creating its main window.'
        }
        Start-Sleep -Milliseconds 200
    }
    throw 'PhotoBooth main window did not appear before the startup timeout.'
}

function Wait-Text([string]$Text, [bool]$Present, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $found = (@(Get-WindowText) -contains $Text)
        if ($found -eq $Present) { return }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "PhotoBooth exited while waiting for UI text: $Text"
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for text '$Text' present=$Present. Current UI: $((Get-WindowText) -join ' | ')"
}

function Get-AppBounds {
    $current = Get-AppWindow
    if (!$current) { throw 'PhotoBooth main window is unavailable.' }
    $rect = New-Object PhotoBoothKioskNative+RECT
    if (![PhotoBoothKioskNative]::GetWindowRect($current.Current.NativeWindowHandle, [ref]$rect)) {
        throw 'Could not read the PhotoBooth window bounds.'
    }
    return [pscustomobject]@{
        Left = $rect.Left; Top = $rect.Top
        Right = $rect.Right; Bottom = $rect.Bottom
        Width = $rect.Right - $rect.Left
        Height = $rect.Bottom - $rect.Top
    }
}

function Test-FillsPrimaryScreen {
    $bounds = Get-AppBounds
    $screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    return (
        [Math]::Abs($bounds.Left - $screen.Left) -le 5 -and
        [Math]::Abs($bounds.Top - $screen.Top) -le 5 -and
        [Math]::Abs($bounds.Right - $screen.Right) -le 5 -and
        [Math]::Abs($bounds.Bottom - $screen.Bottom) -le 5
    )
}

function Send-WindowKey([string]$Key) {
    $current = Get-AppWindow
    if (!$current) { throw 'PhotoBooth window is unavailable for keyboard input.' }
    $handle = [IntPtr]$current.Current.NativeWindowHandle
    if ($handle -eq [IntPtr]::Zero) { throw 'PhotoBooth window has no native handle.' }

    $virtualKey = switch ($Key) {
        '{ESC}' { 0x1B; break }
        '{F11}' { 0x7A; break }
        default { throw "Unsupported smoke-test key: $Key" }
    }
    $currentThread = [PhotoBoothKioskNative]::GetCurrentThreadId()
    $targetThread = [PhotoBoothKioskNative]::GetWindowThreadProcessId($handle, [IntPtr]::Zero)
    $foreground = [PhotoBoothKioskNative]::GetForegroundWindow()
    $foregroundThread = if ($foreground -ne [IntPtr]::Zero) {
        [PhotoBoothKioskNative]::GetWindowThreadProcessId($foreground, [IntPtr]::Zero)
    } else { 0 }
    $attachedTarget = $false
    $attachedForeground = $false

    try {
        if ($targetThread -ne 0 -and $targetThread -ne $currentThread) {
            $attachedTarget = [PhotoBoothKioskNative]::AttachThreadInput($currentThread, $targetThread, $true)
        }
        if ($foregroundThread -ne 0 -and $foregroundThread -ne $currentThread -and $foregroundThread -ne $targetThread) {
            $attachedForeground = [PhotoBoothKioskNative]::AttachThreadInput($currentThread, $foregroundThread, $true)
        }

        [void][PhotoBoothKioskNative]::ShowWindow($handle, 9)
        [void][PhotoBoothKioskNative]::BringWindowToTop($handle)
        [void][PhotoBoothKioskNative]::SetForegroundWindow($handle)
        [void][PhotoBoothKioskNative]::SetFocus($handle)
        Start-Sleep -Milliseconds 250
        [PhotoBoothKioskNative]::keybd_event([byte]$virtualKey, [byte]0, [uint32]0, [UIntPtr]::Zero)
        [PhotoBoothKioskNative]::keybd_event([byte]$virtualKey, [byte]0, [uint32]2, [UIntPtr]::Zero)
        Start-Sleep -Milliseconds 150
    }
    finally {
        if ($attachedForeground) {
            [void][PhotoBoothKioskNative]::AttachThreadInput($currentThread, $foregroundThread, $false)
        }
        if ($attachedTarget) {
            [void][PhotoBoothKioskNative]::AttachThreadInput($currentThread, $targetThread, $false)
        }
    }
}

try {
    Write-Host 'WAILS_KIOSK_SMOKE=START (no capture; disposable local settings only)'
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', '1', 'Process')

    $app = Start-Process -FilePath $exe -WorkingDirectory $exeWorkingDirectory -PassThru
    $window = Wait-Window $StartupTimeoutSeconds
    Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' $true $StartupTimeoutSeconds
    Wait-Text 'Start with webcam' $true $StartupTimeoutSeconds
    $checks['fullscreen_on_start'] = Test-FillsPrimaryScreen
    if (!$checks['fullscreen_on_start']) {
        throw "Kiosk startup did not fill the primary screen. Bounds: $((Get-AppBounds | ConvertTo-Json -Compress))"
    }
    Write-Host 'KIOSK_STARTUP_FULLSCREEN=PASS'

    Send-WindowKey '{ESC}'
    Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' $false $TransitionTimeoutSeconds
    $checks['escape_returns_windowed'] = -not (Test-FillsPrimaryScreen)
    if (!$checks['escape_returns_windowed']) {
        throw 'Escape hid the kiosk hint but the window still fills the primary screen.'
    }
    Write-Host 'ESCAPE_WINDOWED=PASS'

    Send-WindowKey '{F11}'
    Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' $true $TransitionTimeoutSeconds
    $checks['f11_enters_fullscreen'] = Test-FillsPrimaryScreen
    if (!$checks['f11_enters_fullscreen']) {
        throw 'F11 displayed the kiosk hint but did not fill the primary screen.'
    }
    Write-Host 'F11_FULLSCREEN=PASS'

    Send-WindowKey '{ESC}'
    Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' $false $TransitionTimeoutSeconds
    $checks['escape_after_toggle'] = -not (Test-FillsPrimaryScreen)
    if (!$checks['escape_after_toggle']) {
        throw 'Escape did not restore the window after the F11 toggle.'
    }
    Write-Host 'ESCAPE_AFTER_TOGGLE=PASS'

    $status = 'PASS'
}
catch {
    $failure = $_.Exception.Message
    Write-Error $failure
}
finally {
    if ($app -and (Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
        try {
            $proc = Get-Process -Id $app.Id -ErrorAction Stop
            [void]$proc.CloseMainWindow()
            if (-not $proc.WaitForExit(7000)) {
                Write-Warning "PhotoBooth process $($app.Id) did not close gracefully. No force-kill was issued."
            }
        } catch {
            Write-Warning "Could not close the disposable smoke window gracefully: $($_.Exception.Message)"
        }
    }

    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $previousKiosk, 'Process')

    if ($app -and !(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
        if (Test-Path $dataRoot) {
            Remove-Item -LiteralPath $dataRoot -Recurse -Force
        }
        $dataCleaned = !(Test-Path $dataRoot)
    }

    $report = [ordered]@{
        status = $status
        created_at = (Get-Date).ToString('o')
        executable = $exe
        checks = $checks
        failure = $failure
        temporary_data_cleaned = $dataCleaned
        temporary_data_path = if ($dataCleaned) { $null } else { $dataRoot }
    }
    $report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host "TEMP_TEST_DATA_CLEANED=$dataCleaned"
    Write-Host "REPORT_RETAINED=$reportPath"
    Write-Host "WAILS_KIOSK_SMOKE=$status"
}

if ($status -ne 'PASS') { exit 1 }
