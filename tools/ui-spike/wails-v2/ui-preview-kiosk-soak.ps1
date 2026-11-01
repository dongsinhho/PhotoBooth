[CmdletBinding()]
param(
    [ValidateRange(1,60)][int]$DurationMinutes = 10,
    [ValidateRange(1,10)][int]$SampleIntervalSeconds = 2,
    [ValidateRange(1,30)][int]$LifecycleCycleMinutes = 2,
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(20,120)][int]$PreviewTimeoutSeconds = 60,
    [ValidateRange(5,30)][int]$TransitionTimeoutSeconds = 12,
    [ValidateRange(512,8192)][int]$MaxProcessTreePrivateMB = 3072,
    [string]$Device = 'B525 HD Webcam',
    [ValidateRange(4096,32768)][int]$LowRamMaxPhysicalMemoryMB = 8192,
    [switch]$Windowed,
    [switch]$DisableWebViewGPU,
    [switch]$PreflightOnly,
    [switch]$RequireLowRamHost
)

$ErrorActionPreference = 'Stop'
$moduleRoot = $PSScriptRoot
$repoRoot = (Resolve-Path (Join-Path $moduleRoot '..\..\..')).Path
$exe = Join-Path $moduleRoot 'build\bin\PhotoBooth.exe'
if (!(Test-Path $exe)) { throw "Build the current Windows UI candidate first: $exe" }

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms

if (-not ('PhotoBoothSoakNative' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class PhotoBoothSoakNative {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int command);
    [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr SetFocus(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, IntPtr processId);
    [DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll")] public static extern bool AttachThreadInput(uint attach, uint attachTo, bool attachInput);
    [DllImport("user32.dll")] public static extern void keybd_event(byte virtualKey, byte scan, uint flags, UIntPtr extraInfo);
}
'@
}

$runId = [guid]::NewGuid().ToString('N')
$reportRoot = Join-Path $env:TEMP "photobooth-preview-soak-$runId"
$dataRoot = Join-Path $reportRoot 'data'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$resourcesCsv = Join-Path $reportRoot 'process-tree-resources.csv'
$processDetailsCsv = Join-Path $reportRoot 'process-details.csv'
$reportJson = Join-Path $reportRoot 'summary.json'
$reportMd = Join-Path $reportRoot 'report.md'
$diagnosticsPath = Join-Path $reportRoot 'preview-events.jsonl'
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$previousKiosk = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_KIOSK', 'Process')
$previousDiagnosticsPath = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', 'Process')
$previousDisableWebViewGpu = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DISABLE_WEBVIEW_GPU', 'Process')
$app = $null
$watch = $null
$status = 'FAIL'
$failure = $null
$dataCleaned = $false
$cameraInstanceId = $null
$sampleCount = 0
$settingsCycles = 0
$fullscreenCycles = 0
$totalAcknowledgedFrames = 0
$totalCancelledFrames = 0
$diagnosticAckLogGapsMax = 0
$lastDiagnosticAckLogGap = 0
$lastFrameCount = 0
$lastFrameProgress = [DateTime]::UtcNow
$hostHardware = $null
$hostInitialMemory = $null
$lastCpuTotal = $null
$lastCpuSampleAt = $null
$cpuSamples = [System.Collections.Generic.List[double]]::new()
$workingSetSamples = [System.Collections.Generic.List[double]]::new()
$privateSamples = [System.Collections.Generic.List[double]]::new()
$appWorkingSetSamples = [System.Collections.Generic.List[double]]::new()
$appPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewWorkingSetSamples = [System.Collections.Generic.List[double]]::new()
$webViewPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewBrowserPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewRendererPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewGpuPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewUtilityPrivateSamples = [System.Collections.Generic.List[double]]::new()
$webViewOtherPrivateSamples = [System.Collections.Generic.List[double]]::new()
$hostAvailableMemorySamples = [System.Collections.Generic.List[double]]::new()
$hostAvailableMemoryPercentSamples = [System.Collections.Generic.List[double]]::new()
$hostCommitPercentSamples = [System.Collections.Generic.List[double]]::new()
$hostPagesInputSamples = [System.Collections.Generic.List[double]]::new()
$hostPageReadsSamples = [System.Collections.Generic.List[double]]::new()
$frameRates = [System.Collections.Generic.List[double]]::new()
$ackP95Samples = [System.Collections.Generic.List[double]]::new()
$ackMaxSamples = [System.Collections.Generic.List[double]]::new()
$gapP95Samples = [System.Collections.Generic.List[double]]::new()
$startedAt = [DateTime]::UtcNow
$finalMetrics = $null

function Get-B525 {
    try { return @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction Stop | Where-Object { $_.FriendlyName -like '*B525*' }) }
    catch { return @() }
}

function Get-HostHardwareSnapshot {
    $computer = Get-CimInstance Win32_ComputerSystem -ErrorAction Stop
    $os = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
    $cpus = @(Get-CimInstance Win32_Processor -ErrorAction Stop | Select-Object -ExpandProperty Name -Unique)
    $gpus = @(Get-CimInstance Win32_VideoController -ErrorAction Stop | Select-Object -ExpandProperty Name -Unique)
    $physicalMemoryMB = [Math]::Round(([double]$os.TotalVisibleMemorySize / 1024), 0)
    [pscustomobject]@{
        manufacturer = [string]$computer.Manufacturer
        model = [string]$computer.Model
        cpu = ($cpus -join '; ')
        gpu = ($gpus -join '; ')
        os_caption = [string]$os.Caption
        os_version = [string]$os.Version
        os_build = [string]$os.BuildNumber
        architecture = [string]$os.OSArchitecture
        total_physical_memory_mb = $physicalMemoryMB
        low_ram_threshold_mb = $LowRamMaxPhysicalMemoryMB
        low_ram_candidate = ($physicalMemoryMB -le $LowRamMaxPhysicalMemoryMB)
        memory_profile = if ($physicalMemoryMB -le $LowRamMaxPhysicalMemoryMB) { 'low-ram-candidate' } else { 'above-low-ram-target' }
    }
}

function Get-HostMemorySnapshot {
    $totalMB = [double]$hostHardware.total_physical_memory_mb
    try {
        $memory = Get-CimInstance Win32_PerfFormattedData_PerfOS_Memory -ErrorAction Stop
        if (!$memory) { throw 'Windows memory performance counters returned no instance.' }
        $availableMB = [double]$memory.AvailableBytes / 1MB
        $committedMB = [double]$memory.CommittedBytes / 1MB
        $commitLimitMB = [double]$memory.CommitLimit / 1MB
        return [pscustomobject]@{
            available_mb = [Math]::Round($availableMB,2)
            available_percent = if ($totalMB -gt 0) { [Math]::Round(($availableMB / $totalMB) * 100,2) } else { $null }
            committed_mb = [Math]::Round($committedMB,2)
            commit_limit_mb = [Math]::Round($commitLimitMB,2)
            commit_percent = if ($commitLimitMB -gt 0) { [Math]::Round(($committedMB / $commitLimitMB) * 100,2) } else { $null }
            pages_input_per_sec = [double]$memory.PagesInputPersec
            page_reads_per_sec = [double]$memory.PageReadsPersec
            source = 'Win32_PerfFormattedData_PerfOS_Memory'
        }
    } catch {
        $os = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
        $availableMB = [double]$os.FreePhysicalMemory / 1024
        return [pscustomobject]@{
            available_mb = [Math]::Round($availableMB,2)
            available_percent = if ($totalMB -gt 0) { [Math]::Round(($availableMB / $totalMB) * 100,2) } else { $null }
            committed_mb = $null
            commit_limit_mb = $null
            commit_percent = $null
            pages_input_per_sec = $null
            page_reads_per_sec = $null
            source = 'Win32_OperatingSystem fallback; commit/page-read counters unavailable'
        }
    }
}

function Get-AppWindow {
    if (!$app) { return $null }
    $root = [System.Windows.Automation.AutomationElement]::RootElement
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ProcessIdProperty, [int]$app.Id
    )
    $windows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    if ($windows.Count -gt 0) { return $windows.Item(0) }
    return $null
}

function Get-WindowText {
    $window = Get-AppWindow
    if (!$window) { return @() }
    $values = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)) {
        if ($element.Current.Name) { $values.Add($element.Current.Name) | Out-Null }
    }
    return $values.ToArray()
}

function Wait-Text([string]$Pattern, [string]$Label, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        foreach ($value in @(Get-WindowText)) { if ($value -like $Pattern) { return $value } }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "PhotoBooth exited while waiting for '$Label'." }
        Start-Sleep -Milliseconds 200
    }
    throw "Timed out waiting for '$Label'. UI: $((Get-WindowText) -join ' | ')"
}

function Find-Button([string]$Pattern) {
    $window = Get-AppWindow
    if (!$window) { return $null }
    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Button
    )
    foreach ($button in $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)) {
        if ($button.Current.Name -like $Pattern -and $button.Current.IsEnabled -and !$button.Current.IsOffscreen) { return $button }
    }
    return $null
}

function Wait-Button([string]$Pattern, [string]$Label, [int]$Seconds = 10) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $button = Find-Button $Pattern
        if ($button) { return $button }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw "PhotoBooth exited while waiting for visible button '$Label'." }
        Start-Sleep -Milliseconds 200
    }
    $buttons = @()
    $window = Get-AppWindow
    if ($window) {
        $condition = New-Object System.Windows.Automation.PropertyCondition(
            [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
            [System.Windows.Automation.ControlType]::Button
        )
        $buttons = @($window.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition) | ForEach-Object {
            "$($_.Current.Name) [enabled=$($_.Current.IsEnabled),offscreen=$($_.Current.IsOffscreen)]"
        })
    }
    throw "Visible button '$Label' not found (pattern '$Pattern'). Buttons: $($buttons -join ' | ')"
}

function Invoke-Button([string]$Pattern, [string]$Label) {
    $button = Wait-Button $Pattern $Label
    $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    Write-Host "LIFECYCLE_ACTION=$Label"
}

function Get-Percentile([double[]]$Values, [double]$Percentile) {
    if (!$Values -or $Values.Count -eq 0) { return 0.0 }
    $sorted = @($Values | Sort-Object)
    $index = [Math]::Max(0, [Math]::Min($sorted.Count - 1, [int][Math]::Ceiling($Percentile * $sorted.Count) - 1))
    return [double]$sorted[$index]
}

function Read-PreviewMetrics {
    if (!(Test-Path $diagnosticsPath -PathType Leaf)) { return $null }
    $records = [System.Collections.Generic.List[object]]::new()
    # The Go process keeps one append handle open for the diagnostic stream.
    # Share both read and write while taking a point-in-time line snapshot.
    $stream = [IO.FileStream]::new(
        $diagnosticsPath,
        [IO.FileMode]::Open,
        [IO.FileAccess]::Read,
        [IO.FileShare]::ReadWrite
    )
    try {
        $reader = [IO.StreamReader]::new($stream)
        try {
            while ($null -ne ($line = $reader.ReadLine())) {
                if ([string]::IsNullOrWhiteSpace($line)) { continue }
                try { $records.Add(($line | ConvertFrom-Json -ErrorAction Stop)) | Out-Null } catch {}
            }
        }
        finally {
            $reader.Dispose()
        }
    }
    finally {
        $stream.Dispose()
    }
    if ($records.Count -eq 0) { return $null }

    $lastReadyIndex = -1
    $state = 'starting'
    for ($i = 0; $i -lt $records.Count; $i++) {
        if ($records[$i].event -eq 'preview_status') {
            $state = [string]$records[$i].state
            if ($state -eq 'ready') { $lastReadyIndex = $i }
        }
    }
    if ($lastReadyIndex -lt 0) { return $null }

    $run = @($records | Select-Object -Skip ($lastReadyIndex + 1))
    $published = @($run | Where-Object { $_.event -eq 'frame_published' })
    $acked = @($run | Where-Object { $_.event -eq 'frame_acknowledged' })
    $cancelled = @($run | Where-Object { $_.event -eq 'frame_cancelled' })
    $timeouts = @($run | Where-Object { $_.event -eq 'frame_ack_timeout' })
    $decodeFailures = @($run | Where-Object { $_.event -eq 'frame_decode_failed' })
    $ackedSequenceSet = [System.Collections.Generic.HashSet[long]]::new()
    $cancelledSequenceSet = [System.Collections.Generic.HashSet[long]]::new()
    foreach ($event in $acked) { [void]$ackedSequenceSet.Add([long]$event.sequence) }
    foreach ($event in $cancelled) { [void]$cancelledSequenceSet.Add([long]$event.sequence) }
    $unackedPublished = @($published | Where-Object {
        -not $ackedSequenceSet.Contains([long]$_.sequence) -and
        -not $cancelledSequenceSet.Contains([long]$_.sequence)
    })
    $latestPublishedSequence = if ($published.Count) { [long]$published[-1].sequence } else { 0L }
    $unackedNonLatest = @($unackedPublished | Where-Object { [long]$_.sequence -ne $latestPublishedSequence })
    $latencies = @($acked | ForEach-Object { [double]$_.latency_ms })
    $timestamps = @($published | ForEach-Object { [DateTime]::Parse($_.timestamp_utc).ToUniversalTime() })
    $gaps = @()
    for ($i = 1; $i -lt $timestamps.Count; $i++) {
        $gaps += ($timestamps[$i] - $timestamps[$i - 1]).TotalMilliseconds
    }
    $latestPublishedAcked = $false
    if ($published.Count -gt 0) {
        $latestSequence = [long]$published[-1].sequence
        $latestPublishedAcked = $ackedSequenceSet.Contains($latestSequence)
        $latestPublishedCancelled = $cancelledSequenceSet.Contains($latestSequence)
    } else {
        $latestPublishedCancelled = $false
    }
    # The pump is serial. Missing ACK records are never silently inferred as
    # successful: only an explicit ACK increments the count, explicit stop
    # cancellations are excluded, and one final ready-state frame may remain
    # in flight at the snapshot boundary.
    $pendingFrames = if ($state -eq 'ready' -and $published.Count -gt 0 -and !$latestPublishedAcked -and !$latestPublishedCancelled) { 1 } else { 0 }
    $effectiveAckCount = [Math]::Max(0, $published.Count - $pendingFrames - $cancelled.Count)
    $fps = 0.0
    if ($timestamps.Count -gt 0) {
        $elapsed = ([DateTime]::UtcNow - $timestamps[0]).TotalSeconds
        if ($elapsed -gt 0) { $fps = $effectiveAckCount / $elapsed }
    }
    $lastAck = $acked | Select-Object -Last 1
    $lastAckUtc = if ($lastAck) { [DateTime]::Parse($lastAck.timestamp_utc).ToUniversalTime() } else { [DateTime]::MinValue }
    $latencyMean = if ($latencies.Count) { ($latencies | Measure-Object -Average).Average } else { 0.0 }

    return [pscustomobject]@{
        state = $state
        frames_received = $published.Count
        frames_decoded = $effectiveAckCount
        acks = $effectiveAckCount
        ack_failed = $timeouts.Count
        decode_failed = $decodeFailures.Count
        cancelled_frames = $cancelled.Count
        # Non-latest gaps are missing audit records: the serial pump published
        # a later sequence, but neither an ACK nor a stop-cancellation record
        # exists for this earlier frame.
        unacked_non_latest = $unackedNonLatest.Count
        backend_ack_count = $acked.Count
        backend_ack_timeout_events = $timeouts.Count
        backend_ack_p95_ms = [Math]::Round((Get-Percentile $latencies 0.95), 2)
        fps = [Math]::Round($fps, 2)
        ack_mean_ms = [Math]::Round($latencyMean, 2)
        ack_p50_ms = [Math]::Round((Get-Percentile $latencies 0.50), 2)
        ack_p95_ms = [Math]::Round((Get-Percentile $latencies 0.95), 2)
        ack_max_ms = if ($latencies.Count) { [Math]::Round(($latencies | Measure-Object -Maximum).Maximum, 2) } else { 0.0 }
        decode_p95_ms = $null
        frame_gap_p95_ms = [Math]::Round((Get-Percentile $gaps 0.95), 2)
        frame_gap_max_ms = if ($gaps.Count) { [Math]::Round(($gaps | Measure-Object -Maximum).Maximum, 2) } else { 0.0 }
        last_ack_utc = $lastAckUtc
        frame_bytes_mean = if ($published.Count) { [Math]::Round((($published | Measure-Object -Property frame_bytes -Average).Average), 0) } else { 0 }
    }
}

function Wait-PreviewReady([int]$MinimumFrames, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $metrics = Read-PreviewMetrics
        if ($metrics -and $metrics.state -eq 'ready' -and $metrics.frames_received -ge $MinimumFrames) { return $metrics }
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'PhotoBooth exited while waiting for live preview.' }
        Start-Sleep -Milliseconds 200
    }
    throw "Preview telemetry did not become ready. UI: $((Get-WindowText) -join ' | ')"
}

function Get-ProcessTree {
    $all = @(Get-CimInstance Win32_Process)
    $ids = [System.Collections.Generic.HashSet[int]]::new()
    [void]$ids.Add([int]$app.Id)
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($item in $all) {
            if ($ids.Contains([int]$item.ParentProcessId) -and !$ids.Contains([int]$item.ProcessId)) {
                [void]$ids.Add([int]$item.ProcessId)
                $changed = $true
            }
        }
    }
    $processInfoById = @{}
    foreach ($nativeProcess in $all) {
        $processInfoById[[int]$nativeProcess.ProcessId] = $nativeProcess
    }

    $result = [System.Collections.Generic.List[object]]::new()
    foreach ($processId in $ids) {
        $p = Get-Process -Id $processId -ErrorAction SilentlyContinue
        if ($p) {
            $p.Refresh()
            $nativeProcess = $processInfoById[[int]$processId]
            $processType = if ($p.ProcessName -eq 'msedgewebview2') { 'browser' } else { 'app' }
            if ($p.ProcessName -eq 'msedgewebview2' -and $nativeProcess.CommandLine -match '(?:^|\s)--type=([^\s]+)') {
                $processType = $Matches[1]
                if ($processType -eq 'utility' -and $nativeProcess.CommandLine -match '--utility-sub-type=([^\s]+)') {
                    $processType = "utility:$($Matches[1])"
                }
            }
            $result.Add([pscustomobject]@{
                id = [int]$p.Id
                parent_id = if ($nativeProcess) { [int]$nativeProcess.ParentProcessId } else { 0 }
                name = [string]$p.ProcessName
                process_type = $processType
                cpu = if ($null -ne $p.CPU) { [double]$p.CPU } else { 0.0 }
                working_set = [long]$p.WorkingSet64
                private_bytes = [long]$p.PrivateMemorySize64
                threads = if ($p.Threads) { [int]$p.Threads.Count } else { 0 }
                handles = if ($null -ne $p.HandleCount) { [int]$p.HandleCount } else { 0 }
            }) | Out-Null
        }
    }
    return $result.ToArray()
}

function Get-WindowBounds {
    $window = Get-AppWindow
    if (!$window) { throw 'PhotoBooth top-level window is unavailable.' }
    $handle = [IntPtr]$window.Current.NativeWindowHandle
    $rect = New-Object PhotoBoothSoakNative+RECT
    if (![PhotoBoothSoakNative]::GetWindowRect($handle, [ref]$rect)) { throw 'Could not read PhotoBooth window bounds.' }
    return [pscustomobject]@{
        left=$rect.Left; top=$rect.Top; right=$rect.Right; bottom=$rect.Bottom
    }
}

function Test-Fullscreen {
    $bounds = Get-WindowBounds
    $screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    return (
        [Math]::Abs($bounds.left - $screen.Left) -le 5 -and
        [Math]::Abs($bounds.top - $screen.Top) -le 5 -and
        [Math]::Abs($bounds.right - $screen.Right) -le 5 -and
        [Math]::Abs($bounds.bottom - $screen.Bottom) -le 5
    )
}

function Send-WindowKey([string]$Key) {
    $window = Get-AppWindow
    if (!$window) { throw 'PhotoBooth window is unavailable for keyboard input.' }
    $handle = [IntPtr]$window.Current.NativeWindowHandle
    if ($handle -eq [IntPtr]::Zero) { throw 'PhotoBooth window has no native handle.' }
    $virtualKey = switch ($Key) { 'F11' { 0x7A; break } 'ESC' { 0x1B; break } default { throw "Unsupported key '$Key'." } }
    $currentThread = [PhotoBoothSoakNative]::GetCurrentThreadId()
    $targetThread = [PhotoBoothSoakNative]::GetWindowThreadProcessId($handle, [IntPtr]::Zero)
    $foreground = [PhotoBoothSoakNative]::GetForegroundWindow()
    $foregroundThread = if ($foreground -ne [IntPtr]::Zero) { [PhotoBoothSoakNative]::GetWindowThreadProcessId($foreground, [IntPtr]::Zero) } else { 0 }
    $attachedTarget = $false
    $attachedForeground = $false
    try {
        if ($targetThread -and $targetThread -ne $currentThread) { $attachedTarget = [PhotoBoothSoakNative]::AttachThreadInput($currentThread, $targetThread, $true) }
        if ($foregroundThread -and $foregroundThread -ne $currentThread -and $foregroundThread -ne $targetThread) { $attachedForeground = [PhotoBoothSoakNative]::AttachThreadInput($currentThread, $foregroundThread, $true) }
        [void][PhotoBoothSoakNative]::ShowWindow($handle, 9)
        [void][PhotoBoothSoakNative]::BringWindowToTop($handle)
        [void][PhotoBoothSoakNative]::SetForegroundWindow($handle)
        [void][PhotoBoothSoakNative]::SetFocus($handle)
        Start-Sleep -Milliseconds 200
        [PhotoBoothSoakNative]::keybd_event([byte]$virtualKey, [byte]0, [uint32]0, [UIntPtr]::Zero)
        [PhotoBoothSoakNative]::keybd_event([byte]$virtualKey, [byte]0, [uint32]2, [UIntPtr]::Zero)
    } finally {
        if ($attachedForeground) { [void][PhotoBoothSoakNative]::AttachThreadInput($currentThread, $foregroundThread, $false) }
        if ($attachedTarget) { [void][PhotoBoothSoakNative]::AttachThreadInput($currentThread, $targetThread, $false) }
    }
    Start-Sleep -Milliseconds 250
}

function Save-ResourceSample([double]$ElapsedSeconds, $Metrics) {
    $tree = @(Get-ProcessTree)
    if ($tree.Count -eq 0) { throw 'PhotoBooth process tree disappeared during the soak.' }
    $cpuTotal = [double](($tree | Measure-Object -Property cpu -Sum).Sum)
    $wsTotal = [long](($tree | Measure-Object -Property working_set -Sum).Sum)
    $privateTotal = [long](($tree | Measure-Object -Property private_bytes -Sum).Sum)
    $threads = [int](($tree | Measure-Object -Property threads -Sum).Sum)
    $handles = [int](($tree | Measure-Object -Property handles -Sum).Sum)
    $appProcess = $tree | Where-Object { $_.id -eq $app.Id } | Select-Object -First 1
    $webViewProcesses = @($tree | Where-Object { $_.name -eq 'msedgewebview2' })
    $webViewWorkingSet = [long](($webViewProcesses | Measure-Object -Property working_set -Sum).Sum)
    $webViewPrivate = [long](($webViewProcesses | Measure-Object -Property private_bytes -Sum).Sum)
    $webViewBrowserPrivate = [long](($webViewProcesses | Where-Object { $_.process_type -eq 'browser' } | Measure-Object -Property private_bytes -Sum).Sum)
    $webViewRendererPrivate = [long](($webViewProcesses | Where-Object { $_.process_type -eq 'renderer' } | Measure-Object -Property private_bytes -Sum).Sum)
    $webViewGpuPrivate = [long](($webViewProcesses | Where-Object { $_.process_type -eq 'gpu-process' } | Measure-Object -Property private_bytes -Sum).Sum)
    $webViewUtilityPrivate = [long](($webViewProcesses | Where-Object { $_.process_type -like 'utility*' } | Measure-Object -Property private_bytes -Sum).Sum)
    $webViewOtherPrivate = [Math]::Max(0, $webViewPrivate - $webViewBrowserPrivate - $webViewRendererPrivate - $webViewGpuPrivate - $webViewUtilityPrivate)
    $hostMemory = Get-HostMemorySnapshot
    $now = [DateTime]::UtcNow
    $cpuOneCore = 0.0
    if ($null -ne $script:lastCpuSampleAt -and $null -ne $script:lastCpuTotal) {
        $seconds = ($now - $script:lastCpuSampleAt).TotalSeconds
        if ($seconds -gt 0) { $cpuOneCore = [Math]::Max(0, (($cpuTotal - $script:lastCpuTotal) / $seconds) * 100) }
    }
    $script:lastCpuTotal = $cpuTotal
    $script:lastCpuSampleAt = $now
    $script:cpuSamples.Add($cpuOneCore) | Out-Null
    $script:workingSetSamples.Add($wsTotal / 1MB) | Out-Null
    $script:privateSamples.Add($privateTotal / 1MB) | Out-Null
    $script:appWorkingSetSamples.Add($appProcess.working_set / 1MB) | Out-Null
    $script:appPrivateSamples.Add($appProcess.private_bytes / 1MB) | Out-Null
    $script:webViewWorkingSetSamples.Add($webViewWorkingSet / 1MB) | Out-Null
    $script:webViewPrivateSamples.Add($webViewPrivate / 1MB) | Out-Null
    $script:webViewBrowserPrivateSamples.Add($webViewBrowserPrivate / 1MB) | Out-Null
    $script:webViewRendererPrivateSamples.Add($webViewRendererPrivate / 1MB) | Out-Null
    $script:webViewGpuPrivateSamples.Add($webViewGpuPrivate / 1MB) | Out-Null
    $script:webViewUtilityPrivateSamples.Add($webViewUtilityPrivate / 1MB) | Out-Null
    $script:webViewOtherPrivateSamples.Add($webViewOtherPrivate / 1MB) | Out-Null
    $script:hostAvailableMemorySamples.Add($hostMemory.available_mb) | Out-Null
    if ($null -ne $hostMemory.available_percent) { $script:hostAvailableMemoryPercentSamples.Add($hostMemory.available_percent) | Out-Null }
    if ($null -ne $hostMemory.commit_percent) { $script:hostCommitPercentSamples.Add($hostMemory.commit_percent) | Out-Null }
    if ($null -ne $hostMemory.pages_input_per_sec) { $script:hostPagesInputSamples.Add($hostMemory.pages_input_per_sec) | Out-Null }
    if ($null -ne $hostMemory.page_reads_per_sec) { $script:hostPageReadsSamples.Add($hostMemory.page_reads_per_sec) | Out-Null }
    $script:frameRates.Add($Metrics.fps) | Out-Null
    $script:ackP95Samples.Add($Metrics.ack_p95_ms) | Out-Null
    $script:ackMaxSamples.Add($Metrics.ack_max_ms) | Out-Null
    $script:gapP95Samples.Add($Metrics.frame_gap_p95_ms) | Out-Null
    $row = [pscustomobject]@{
        timestamp_utc=$now.ToString('o')
        elapsed_seconds=[Math]::Round($ElapsedSeconds,2)
        cpu_tree_percent_one_core=[Math]::Round($cpuOneCore,2)
        working_set_tree_mb=[Math]::Round($wsTotal/1MB,2)
        private_bytes_tree_mb=[Math]::Round($privateTotal/1MB,2)
        app_working_set_mb=[Math]::Round($appProcess.working_set/1MB,2)
        app_private_bytes_mb=[Math]::Round($appProcess.private_bytes/1MB,2)
        webview_process_count=$webViewProcesses.Count
        webview_working_set_mb=[Math]::Round($webViewWorkingSet/1MB,2)
        webview_private_bytes_mb=[Math]::Round($webViewPrivate/1MB,2)
        webview_gpu_disabled=$DisableWebViewGPU
        host_physical_memory_total_mb=$hostHardware.total_physical_memory_mb
        host_memory_profile=$hostHardware.memory_profile
        host_memory_available_mb=$hostMemory.available_mb
        host_memory_available_percent=$hostMemory.available_percent
        host_committed_mb=$hostMemory.committed_mb
        host_commit_limit_mb=$hostMemory.commit_limit_mb
        host_commit_percent=$hostMemory.commit_percent
        host_pages_input_per_sec=$hostMemory.pages_input_per_sec
        host_page_reads_per_sec=$hostMemory.page_reads_per_sec
        webview_browser_private_bytes_mb=[Math]::Round($webViewBrowserPrivate/1MB,2)
        webview_renderer_private_bytes_mb=[Math]::Round($webViewRendererPrivate/1MB,2)
        webview_gpu_private_bytes_mb=[Math]::Round($webViewGpuPrivate/1MB,2)
        webview_utility_private_bytes_mb=[Math]::Round($webViewUtilityPrivate/1MB,2)
        webview_other_private_bytes_mb=[Math]::Round($webViewOtherPrivate/1MB,2)
        process_count=$tree.Count
        thread_count=$threads
        handle_count=$handles
        preview_state=$Metrics.state
        frames_received=$Metrics.frames_received
        frames_decoded=$Metrics.frames_decoded
        acknowledgements=$Metrics.acks
        ack_failures=$Metrics.ack_failed
        decode_failures=$Metrics.decode_failed
        cancelled_frames=$Metrics.cancelled_frames
        unacked_non_latest=$Metrics.unacked_non_latest
        fps=$Metrics.fps
        ack_mean_ms=$Metrics.ack_mean_ms
        ack_p50_ms=$Metrics.ack_p50_ms
        ack_p95_ms=$Metrics.ack_p95_ms
        ack_max_ms=$Metrics.ack_max_ms
        decode_p95_ms=$Metrics.decode_p95_ms
        frame_gap_p95_ms=$Metrics.frame_gap_p95_ms
        frame_gap_max_ms=$Metrics.frame_gap_max_ms
    }
    $row | Export-Csv -Path $resourcesCsv -NoTypeInformation -Append -Encoding UTF8
    foreach ($item in $tree) {
        [pscustomobject]@{
            timestamp_utc=$now.ToString('o')
            elapsed_seconds=[Math]::Round($ElapsedSeconds,2)
            process_id=$item.id
            parent_process_id=$item.parent_id
            process_name=$item.name
            process_type=$item.process_type
            cpu_seconds=$item.cpu
            working_set_mb=[Math]::Round($item.working_set/1MB,2)
            private_bytes_mb=[Math]::Round($item.private_bytes/1MB,2)
            thread_count=$item.threads
            handle_count=$item.handles
        } | Export-Csv -Path $processDetailsCsv -NoTypeInformation -Append -Encoding UTF8
    }

    # Protect the development host if WebView2 memory keeps growing during
    # the soak. The resource sample above is persisted before stopping.
    $privateMB = $privateTotal / 1MB
    if ($privateMB -gt $MaxProcessTreePrivateMB) {
        throw ("Process-tree private bytes exceeded the safety guard: {0:N0} MB > {1:N0} MB. The last sample was saved; stopping the soak." -f $privateMB,$MaxProcessTreePrivateMB)
    }
}

$hostHardware = Get-HostHardwareSnapshot
$hostInitialMemory = Get-HostMemorySnapshot
Write-Host ("HOST_PREFLIGHT manufacturer={0} model={1} ram_mb={2} profile={3} threshold_mb={4}" -f
    $hostHardware.manufacturer,$hostHardware.model,$hostHardware.total_physical_memory_mb,$hostHardware.memory_profile,$LowRamMaxPhysicalMemoryMB)
if ($PreflightOnly) {
    [pscustomobject]@{
        host = $hostHardware
        initial_memory = $hostInitialMemory
        preflight_only = $true
        low_ram_requirement_met = [bool]$hostHardware.low_ram_candidate
    } | ConvertTo-Json -Depth 5
    if ($RequireLowRamHost -and !$hostHardware.low_ram_candidate) {
        throw ("Low-RAM validation requires <= {0} MB visible physical memory; this host has {1} MB. No camera or app was started." -f $LowRamMaxPhysicalMemoryMB,$hostHardware.total_physical_memory_mb)
    }
    return
}
if ($RequireLowRamHost -and !$hostHardware.low_ram_candidate) {
    throw ("Refusing low-RAM soak on a non-target host: {0} MB visible physical memory exceeds {1} MB. Use -PreflightOnly to inspect the host; no camera or app was started." -f $hostHardware.total_physical_memory_mb,$LowRamMaxPhysicalMemoryMB)
}

try {
    Write-Host "PREVIEW_KIOSK_SOAK=START duration_min=$DurationMinutes sample_s=$SampleIntervalSeconds cycle_min=$LifecycleCycleMinutes"
    Write-Host ("HOST_PROFILE={0} total_ram_mb={1} initial_available_mb={2} initial_commit_percent={3}" -f
        $hostHardware.memory_profile,$hostHardware.total_physical_memory_mb,$hostInitialMemory.available_mb,$hostInitialMemory.commit_percent)
    Write-Host 'No photo session will be started. Only a disposable local station profile will be created.'

    # Preview is an exclusive camera owner. Never start a soak instance beside
    # an operator window or another Wails prototype using the same webcam.
    $existingApps = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('PhotoBooth.exe', 'wails-v2.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath -like '*PhotoBooth*tools\ui-spike\wails-v2\build\bin*'
    })
    if ($existingApps.Count -gt 0) {
        $details = ($existingApps | ForEach-Object { "PID=$($_.ProcessId) EXE=$($_.ExecutablePath)" }) -join '; '
        throw "Another PhotoBooth UI process is already running; close it before the exclusive-camera soak. $details"
    }

    $camera = @(Get-B525)
    if ($camera.Count -ne 1 -or $camera[0].Status -ne 'OK') { throw "Expected exactly one healthy B525 camera; found $($camera.Count)." }
    $cameraInstanceId = $camera[0].InstanceId
    New-Item -ItemType Directory -Path $dataRoot -Force | Out-Null
    $cfg = [ordered]@{
        schema_version = 1
        camera = [ordered]@{ provider = 'windowsmf'; device = $Device; width = 1280; height = 720; fps = 30 }
        capture = [ordered]@{ shots = 3; countdown = '3s'; interval = '1s' }
        frame = [ordered]@{ config_path = 'frames/photo-strip-v1.json' }
        storage = [ordered]@{ sessions_dir = 'sessions'; output_dir = 'photos' }
        media_delivery = [ordered]@{ mode = 'local' }
        payment = [ordered]@{ mode = 'disabled' }
    }
    [IO.File]::WriteAllText($configPath, ($cfg | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    $header = 'timestamp_utc,elapsed_seconds,cpu_tree_percent_one_core,working_set_tree_mb,private_bytes_tree_mb,app_working_set_mb,app_private_bytes_mb,webview_process_count,webview_working_set_mb,webview_private_bytes_mb,webview_gpu_disabled,host_physical_memory_total_mb,host_memory_profile,host_memory_available_mb,host_memory_available_percent,host_committed_mb,host_commit_limit_mb,host_commit_percent,host_pages_input_per_sec,host_page_reads_per_sec,webview_browser_private_bytes_mb,webview_renderer_private_bytes_mb,webview_gpu_private_bytes_mb,webview_utility_private_bytes_mb,webview_other_private_bytes_mb,process_count,thread_count,handle_count,preview_state,frames_received,frames_decoded,acknowledgements,ack_failures,decode_failures,cancelled_frames,unacked_non_latest,fps,ack_mean_ms,ack_p50_ms,ack_p95_ms,ack_max_ms,decode_p95_ms,frame_gap_p95_ms,frame_gap_max_ms' + [Environment]::NewLine
    [IO.File]::WriteAllText($resourcesCsv, $header, [Text.UTF8Encoding]::new($false))
    $detailsHeader = 'timestamp_utc,elapsed_seconds,process_id,parent_process_id,process_name,process_type,cpu_seconds,working_set_mb,private_bytes_mb,thread_count,handle_count' + [Environment]::NewLine
    [IO.File]::WriteAllText($processDetailsCsv, $detailsHeader, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText($diagnosticsPath, '', [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $(if ($Windowed) { '0' } else { '1' }), 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', $diagnosticsPath, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DISABLE_WEBVIEW_GPU', $(if ($DisableWebViewGPU) { '1' } else { '0' }), 'Process')
    $app = Start-Process -FilePath $exe -WorkingDirectory $moduleRoot -PassThru

    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    while ((Get-Date) -lt $deadline -and !(Get-AppWindow)) {
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'PhotoBooth exited during startup.' }
        Start-Sleep -Milliseconds 200
    }
    if (!(Get-AppWindow)) { throw 'PhotoBooth window did not appear.' }
    [void](Wait-Text 'Start with webcam' 'Idle screen' $StartupTimeoutSeconds)
    $initial = Wait-PreviewReady 5 $PreviewTimeoutSeconds
    if (!$Windowed) {
        [void](Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' 'fullscreen kiosk hint' $StartupTimeoutSeconds)
        if (!(Test-Fullscreen)) { throw 'Kiosk startup did not fill the primary display.' }
    }
    Write-Host "PREVIEW_READY=PASS frames=$($initial.frames_received) state=$($initial.state) fps=$($initial.fps) ack_p95_ms=$($initial.ack_p95_ms)"
    Write-Host "KIOSK_START_MODE=$(if($Windowed){'WINDOWED'}else{'FULLSCREEN'})"

    $watch = [System.Diagnostics.Stopwatch]::StartNew()
    $nextLifecycle = [double]($LifecycleCycleMinutes * 60)
    $lastFrameCount = $initial.frames_received
    $lastFrameProgress = [DateTime]::UtcNow
    $lastMetricAcks = $initial.acks
    $totalAcknowledgedFrames = $initial.acks
    $lastProgressLog = [System.Diagnostics.Stopwatch]::StartNew()

    while ($watch.Elapsed.TotalMinutes -lt $DurationMinutes) {
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) { throw 'PhotoBooth exited during preview soak.' }
        $metrics = Read-PreviewMetrics
        if (!$metrics) { throw 'Machine-readable preview telemetry became unavailable.' }
        if ($metrics.state -ne 'ready') { throw "Live preview entered unexpected state '$($metrics.state)' during soak." }
        if ($metrics.ack_failed -ne 0 -or $metrics.decode_failed -ne 0) {
            throw "Preview reported ack_failed=$($metrics.ack_failed), decode_failed=$($metrics.decode_failed)."
        }
        if ($metrics.unacked_non_latest -gt $diagnosticAckLogGapsMax) {
            $diagnosticAckLogGapsMax = $metrics.unacked_non_latest
        }
        if ($metrics.unacked_non_latest -gt 0 -and $metrics.unacked_non_latest -ne $lastDiagnosticAckLogGap) {
            Write-Warning "Backend JSONL audit currently has $($metrics.unacked_non_latest) publish records without an ACK or cancellation event. The final audit will fail unless the event record is present."
            $lastDiagnosticAckLogGap = $metrics.unacked_non_latest
        }
        # ADR 0020 permits exactly one published frame to remain in flight.
        # The backend pump is serial: it cannot publish N+1 until it consumes
        # the ACK for N. Any received/ACK difference larger than one is invalid.
        if (($metrics.frames_received - $metrics.acks) -gt 1 -or
            ($metrics.frames_decoded - $metrics.acks) -gt 1 -or
            $metrics.frames_decoded -gt $metrics.frames_received -or
            $metrics.acks -gt $metrics.frames_decoded) {
            throw "Frame accounting mismatch (max one in-flight/ACK frame allowed): received=$($metrics.frames_received), decoded=$($metrics.frames_decoded), ACKs=$($metrics.acks)."
        }
        if ($metrics.acks -ge $lastMetricAcks) { $totalAcknowledgedFrames += ($metrics.acks - $lastMetricAcks) }
        else { $totalAcknowledgedFrames += $metrics.acks }
        $lastMetricAcks = $metrics.acks

        if ($metrics.frames_received -gt $lastFrameCount) {
            $lastFrameCount = $metrics.frames_received
            $lastFrameProgress = [DateTime]::UtcNow
        } elseif (([DateTime]::UtcNow - $lastFrameProgress).TotalSeconds -gt 5) {
            throw 'No new preview frame for more than 5 seconds.'
        }

        Save-ResourceSample $watch.Elapsed.TotalSeconds $metrics
        $sampleCount++
        if ($lastProgressLog.Elapsed.TotalSeconds -ge 20) {
            Write-Host ("SOAK_PROGRESS elapsed_min={0:N1}/{1} acked_frames={2} fps={3:N2} cpu_one_core={4:N1}% ws_tree={5:N1}MB private_tree={6:N1}MB" -f
                $watch.Elapsed.TotalMinutes,$DurationMinutes,$totalAcknowledgedFrames,$metrics.fps,$cpuSamples[$cpuSamples.Count-1],$workingSetSamples[$workingSetSamples.Count-1],$privateSamples[$privateSamples.Count-1])
            $lastProgressLog.Restart()
        }

        if ($watch.Elapsed.TotalSeconds -ge $nextLifecycle) {
            $settingsCycles++
            Write-Host "LIFECYCLE_PHASE=settings_open cycle=$settingsCycles"
            Invoke-Button 'Settings & frame*' 'open Settings'
            [void](Wait-Text '*OPERATOR CONFIGURATION*' 'Settings view' $TransitionTimeoutSeconds)
            Write-Host 'SETTINGS_VIEW=VISIBLE'

            # Preview must be stopped while Settings owns the operator workflow.
            # Do not classify this intentional state transition as an unhealthy preview.
            $stoppingDeadline = (Get-Date).AddSeconds($TransitionTimeoutSeconds)
            $sawStopping = $false
            while ((Get-Date) -lt $stoppingDeadline) {
                $settingsMetrics = Read-PreviewMetrics
                if ($settingsMetrics -and $settingsMetrics.state -eq 'stopping') { $sawStopping = $true; break }
                Start-Sleep -Milliseconds 100
            }
            if (!$sawStopping) { throw 'Opening Settings did not produce the expected preview stop event.' }
            $totalCancelledFrames += $settingsMetrics.cancelled_frames
            Write-Host "PREVIEW_STOP_FOR_SETTINGS=PASS cancelled_in_flight=$($settingsMetrics.cancelled_frames)"

            Invoke-Button 'Cancel' 'cancel Settings (exact visible button)'
            Start-Sleep -Milliseconds 300
            [void](Wait-Text 'Start with webcam' 'return to Idle' $TransitionTimeoutSeconds)
            Write-Host 'SETTINGS_CANCEL=INVOKED'

            $afterSettings = Wait-PreviewReady 3 $PreviewTimeoutSeconds
            if ($afterSettings.state -ne 'ready') { throw "Preview did not resume after Settings; state=$($afterSettings.state)." }
            $lastFrameCount = $afterSettings.frames_received
            $lastFrameProgress = [DateTime]::UtcNow
            $lastMetricAcks = $afterSettings.acks
            $totalAcknowledgedFrames += $afterSettings.acks
            Write-Host "SETTINGS_CYCLE=$settingsCycles PASS preview_resumed_frames=$($afterSettings.frames_received)"

            if (!$Windowed) {
                Write-Host 'LIFECYCLE_PHASE=fullscreen_to_windowed'
                Send-WindowKey 'F11'
                [void](Wait-Text 'Start with webcam' 'windowed Idle' $TransitionTimeoutSeconds)
                if (Test-Fullscreen) { throw 'F11 did not switch fullscreen to windowed mode.' }
                [void](Wait-PreviewReady 3 $PreviewTimeoutSeconds)
                Send-WindowKey 'F11'
                [void](Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' 'fullscreen hint' $TransitionTimeoutSeconds)
                if (!(Test-Fullscreen)) { throw 'F11 did not restore fullscreen kiosk mode.' }
            } else {
                Write-Host 'LIFECYCLE_PHASE=windowed_to_fullscreen_to_windowed'
                Send-WindowKey 'F11'
                [void](Wait-Text 'KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE' 'fullscreen hint from windowed launch' $TransitionTimeoutSeconds)
                if (!(Test-Fullscreen)) { throw 'F11 did not enter fullscreen mode.' }
                Send-WindowKey 'F11'
                if (Test-Fullscreen) { throw 'F11 did not return to windowed mode.' }
            }
            $afterToggle = Wait-PreviewReady 3 $PreviewTimeoutSeconds
            $lastFrameCount = $afterToggle.frames_received
            $lastFrameProgress = [DateTime]::UtcNow
            # Fullscreen changes do not restart preview. Count only ACKs that
            # arrived since the post-Settings checkpoint, not the full run.
            $toggleAckDelta = if ($afterToggle.acks -ge $lastMetricAcks) {
                $afterToggle.acks - $lastMetricAcks
            } else {
                $afterToggle.acks
            }
            $totalAcknowledgedFrames += $toggleAckDelta
            $lastMetricAcks = $afterToggle.acks
            $fullscreenCycles++
            Write-Host "KIOSK_TOGGLE_CYCLE=$fullscreenCycles PASS preview_resumed_frames=$($afterToggle.frames_received)"
            $nextLifecycle += ($LifecycleCycleMinutes * 60)
        }

        Start-Sleep -Seconds $SampleIntervalSeconds
    }

    $finalMetrics = Read-PreviewMetrics
    if (!$finalMetrics -or $finalMetrics.state -ne 'ready') { throw 'Final preview state was not ready.' }
    if ($finalMetrics.ack_failed -ne 0 -or $finalMetrics.decode_failed -ne 0) { throw 'Final preview had an ACK/decode failure.' }
    if ($finalMetrics.unacked_non_latest -gt 0) {
        throw "Preview diagnostics omitted $($finalMetrics.unacked_non_latest) ACK record(s) for frames that were followed by later publications."
    }
    if ($finalMetrics.unacked_non_latest -gt $diagnosticAckLogGapsMax) {
        $diagnosticAckLogGapsMax = $finalMetrics.unacked_non_latest
    }
    if (($finalMetrics.frames_received - $finalMetrics.acks) -gt 1 -or
        ($finalMetrics.frames_decoded - $finalMetrics.acks) -gt 1 -or
        $finalMetrics.frames_decoded -gt $finalMetrics.frames_received -or
        $finalMetrics.acks -gt $finalMetrics.frames_decoded) {
        throw "Final frame accounting mismatch (max one in-flight/ACK frame allowed): received=$($finalMetrics.frames_received), decoded=$($finalMetrics.frames_decoded), ACKs=$($finalMetrics.acks)."
    }
    if ($finalMetrics.fps -lt 1.5) { throw "Final FPS below 1.5: $($finalMetrics.fps)." }
    if ($sampleCount -lt 5) { throw "Insufficient resource samples: $sampleCount." }
    if ($settingsCycles -lt 1 -or $fullscreenCycles -lt 1) { throw 'Settings and fullscreen/windowed lifecycle cycles are required.' }
    $cameraEnd = @(Get-B525 | Where-Object { $_.InstanceId -eq $cameraInstanceId -and $_.Status -eq 'OK' })
    if ($cameraEnd.Count -ne 1) { throw 'B525 was no longer present/healthy at the end of the soak.' }
    $status = 'PASS'
    # The authoritative PASS/FAIL is emitted from finally after the complete
    # flushed JSONL sequence audit; do not announce PASS before that audit.
}
catch {
    $failure = $_.Exception.Message
    Write-Host "UI_TEXT=$((Get-WindowText) -join ' | ')"
}
finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(10000)) {
                Write-Warning 'PhotoBooth did not close gracefully. It is left running to avoid force-closing a camera owner.'
            }
        }
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_KIOSK', $previousKiosk, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', $previousDiagnosticsPath, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DISABLE_WEBVIEW_GPU', $previousDisableWebViewGpu, 'Process')
    $appAlive = $app -and (Get-Process -Id $app.Id -ErrorAction SilentlyContinue)
    if (!$appAlive -and (Test-Path $dataRoot)) {
        Remove-Item -LiteralPath $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    $dataCleaned = !(Test-Path $dataRoot)

    # Reconcile the complete flushed JSONL log after graceful app shutdown.
    # Progress counters can cross preview lifecycle resets; the final report
    # must use exact backend events rather than inferred frontend totals.
    $publishedSequences = [System.Collections.Generic.HashSet[long]]::new()
    $acknowledgedSequences = [System.Collections.Generic.HashSet[long]]::new()
    $cancelledSequences = [System.Collections.Generic.HashSet[long]]::new()
    $publishedEventCount = 0
    $acknowledgedEventCount = 0
    $cancelledEventCount = 0
    $invalidDiagnosticLineCount = 0
    $orphanAckEventCount = 0
    $overlappingTerminalEventCount = 0
    if (Test-Path $diagnosticsPath -PathType Leaf) {
        $logStream = [IO.FileStream]::new($diagnosticsPath,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
        try {
            $logReader = [IO.StreamReader]::new($logStream)
            try {
                while ($null -ne ($line = $logReader.ReadLine())) {
                    if ([string]::IsNullOrWhiteSpace($line)) { continue }
                    try {
                        $event = $line | ConvertFrom-Json -ErrorAction Stop
                        switch ([string]$event.event) {
                            'frame_published' {
                                $publishedEventCount++
                                [void]$publishedSequences.Add([long]$event.sequence)
                            }
                            'frame_acknowledged' {
                                $acknowledgedEventCount++
                                [void]$acknowledgedSequences.Add([long]$event.sequence)
                            }
                            'frame_cancelled' {
                                $cancelledEventCount++
                                [void]$cancelledSequences.Add([long]$event.sequence)
                            }
                        }
                    } catch {
                        $invalidDiagnosticLineCount++
                    }
                }
            }
            finally { $logReader.Dispose() }
        }
        finally { $logStream.Dispose() }
    }
    $unmatchedPublishCount = @($publishedSequences | Where-Object {
        -not $acknowledgedSequences.Contains($_) -and -not $cancelledSequences.Contains($_)
    }).Count
    $orphanAckEventCount = @($acknowledgedSequences | Where-Object { -not $publishedSequences.Contains($_) }).Count
    $overlappingTerminalEventCount = @($acknowledgedSequences | Where-Object { $cancelledSequences.Contains($_) }).Count
    $duplicatePublishedEventCount = $publishedEventCount - $publishedSequences.Count
    $duplicateAckEventCount = $acknowledgedEventCount - $acknowledgedSequences.Count
    $duplicateCancelledEventCount = $cancelledEventCount - $cancelledSequences.Count
    $totalAcknowledgedFrames = $acknowledgedEventCount
    $totalCancelledFrames = $cancelledEventCount
    $diagnosticAckLogGapsMax = $unmatchedPublishCount
    if ($invalidDiagnosticLineCount -gt 0 -or $unmatchedPublishCount -gt 0 -or
        $orphanAckEventCount -gt 0 -or $overlappingTerminalEventCount -gt 0 -or
        $duplicatePublishedEventCount -gt 0 -or $duplicateAckEventCount -gt 0 -or
        $duplicateCancelledEventCount -gt 0) {
        $status = 'FAIL'
        $failure = "Final JSONL audit failed: invalid_lines=$invalidDiagnosticLineCount, unmatched_publish=$unmatchedPublishCount, orphan_ack=$orphanAckEventCount, ACK/cancel_overlap=$overlappingTerminalEventCount, duplicate_publish=$duplicatePublishedEventCount, duplicate_ack=$duplicateAckEventCount, duplicate_cancel=$duplicateCancelledEventCount."
    }

    $stats = [ordered]@{
        cpu_one_core_mean = if ($cpuSamples.Count) { [Math]::Round(($cpuSamples | Measure-Object -Average).Average,2) } else { $null }
        cpu_one_core_peak = if ($cpuSamples.Count) { [Math]::Round(($cpuSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        working_set_first_mb = if ($workingSetSamples.Count) { [Math]::Round($workingSetSamples[0],2) } else { $null }
        working_set_last_mb = if ($workingSetSamples.Count) { [Math]::Round($workingSetSamples[$workingSetSamples.Count-1],2) } else { $null }
        working_set_peak_mb = if ($workingSetSamples.Count) { [Math]::Round(($workingSetSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        private_bytes_first_mb = if ($privateSamples.Count) { [Math]::Round($privateSamples[0],2) } else { $null }
        private_bytes_last_mb = if ($privateSamples.Count) { [Math]::Round($privateSamples[$privateSamples.Count-1],2) } else { $null }
        private_bytes_peak_mb = if ($privateSamples.Count) { [Math]::Round(($privateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        app_working_set_first_mb = if ($appWorkingSetSamples.Count) { [Math]::Round($appWorkingSetSamples[0],2) } else { $null }
        app_working_set_last_mb = if ($appWorkingSetSamples.Count) { [Math]::Round($appWorkingSetSamples[$appWorkingSetSamples.Count-1],2) } else { $null }
        app_working_set_peak_mb = if ($appWorkingSetSamples.Count) { [Math]::Round(($appWorkingSetSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        app_private_bytes_first_mb = if ($appPrivateSamples.Count) { [Math]::Round($appPrivateSamples[0],2) } else { $null }
        app_private_bytes_last_mb = if ($appPrivateSamples.Count) { [Math]::Round($appPrivateSamples[$appPrivateSamples.Count-1],2) } else { $null }
        app_private_bytes_peak_mb = if ($appPrivateSamples.Count) { [Math]::Round(($appPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_working_set_first_mb = if ($webViewWorkingSetSamples.Count) { [Math]::Round($webViewWorkingSetSamples[0],2) } else { $null }
        webview_working_set_last_mb = if ($webViewWorkingSetSamples.Count) { [Math]::Round($webViewWorkingSetSamples[$webViewWorkingSetSamples.Count-1],2) } else { $null }
        webview_working_set_peak_mb = if ($webViewWorkingSetSamples.Count) { [Math]::Round(($webViewWorkingSetSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_private_bytes_first_mb = if ($webViewPrivateSamples.Count) { [Math]::Round($webViewPrivateSamples[0],2) } else { $null }
        webview_private_bytes_last_mb = if ($webViewPrivateSamples.Count) { [Math]::Round($webViewPrivateSamples[$webViewPrivateSamples.Count-1],2) } else { $null }
        webview_private_bytes_peak_mb = if ($webViewPrivateSamples.Count) { [Math]::Round(($webViewPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_browser_private_first_mb = if ($webViewBrowserPrivateSamples.Count) { [Math]::Round($webViewBrowserPrivateSamples[0],2) } else { $null }
        webview_browser_private_last_mb = if ($webViewBrowserPrivateSamples.Count) { [Math]::Round($webViewBrowserPrivateSamples[$webViewBrowserPrivateSamples.Count-1],2) } else { $null }
        webview_browser_private_peak_mb = if ($webViewBrowserPrivateSamples.Count) { [Math]::Round(($webViewBrowserPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_renderer_private_first_mb = if ($webViewRendererPrivateSamples.Count) { [Math]::Round($webViewRendererPrivateSamples[0],2) } else { $null }
        webview_renderer_private_last_mb = if ($webViewRendererPrivateSamples.Count) { [Math]::Round($webViewRendererPrivateSamples[$webViewRendererPrivateSamples.Count-1],2) } else { $null }
        webview_renderer_private_peak_mb = if ($webViewRendererPrivateSamples.Count) { [Math]::Round(($webViewRendererPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_gpu_private_first_mb = if ($webViewGpuPrivateSamples.Count) { [Math]::Round($webViewGpuPrivateSamples[0],2) } else { $null }
        webview_gpu_private_last_mb = if ($webViewGpuPrivateSamples.Count) { [Math]::Round($webViewGpuPrivateSamples[$webViewGpuPrivateSamples.Count-1],2) } else { $null }
        webview_gpu_private_peak_mb = if ($webViewGpuPrivateSamples.Count) { [Math]::Round(($webViewGpuPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_utility_private_first_mb = if ($webViewUtilityPrivateSamples.Count) { [Math]::Round($webViewUtilityPrivateSamples[0],2) } else { $null }
        webview_utility_private_last_mb = if ($webViewUtilityPrivateSamples.Count) { [Math]::Round($webViewUtilityPrivateSamples[$webViewUtilityPrivateSamples.Count-1],2) } else { $null }
        webview_utility_private_peak_mb = if ($webViewUtilityPrivateSamples.Count) { [Math]::Round(($webViewUtilityPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        webview_other_private_first_mb = if ($webViewOtherPrivateSamples.Count) { [Math]::Round($webViewOtherPrivateSamples[0],2) } else { $null }
        webview_other_private_last_mb = if ($webViewOtherPrivateSamples.Count) { [Math]::Round($webViewOtherPrivateSamples[$webViewOtherPrivateSamples.Count-1],2) } else { $null }
        webview_other_private_peak_mb = if ($webViewOtherPrivateSamples.Count) { [Math]::Round(($webViewOtherPrivateSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        host_memory_available_first_mb = if ($hostAvailableMemorySamples.Count) { [Math]::Round($hostAvailableMemorySamples[0],2) } else { $null }
        host_memory_available_last_mb = if ($hostAvailableMemorySamples.Count) { [Math]::Round($hostAvailableMemorySamples[$hostAvailableMemorySamples.Count-1],2) } else { $null }
        host_memory_available_min_mb = if ($hostAvailableMemorySamples.Count) { [Math]::Round(($hostAvailableMemorySamples | Measure-Object -Minimum).Minimum,2) } else { $null }
        host_memory_available_percent_min = if ($hostAvailableMemoryPercentSamples.Count) { [Math]::Round(($hostAvailableMemoryPercentSamples | Measure-Object -Minimum).Minimum,2) } else { $null }
        host_commit_percent_peak = if ($hostCommitPercentSamples.Count) { [Math]::Round(($hostCommitPercentSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        host_pages_input_per_sec_peak = if ($hostPagesInputSamples.Count) { [Math]::Round(($hostPagesInputSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        host_page_reads_per_sec_peak = if ($hostPageReadsSamples.Count) { [Math]::Round(($hostPageReadsSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        preview_fps_mean = if ($frameRates.Count) { [Math]::Round(($frameRates | Measure-Object -Average).Average,2) } else { $null }
        preview_fps_min = if ($frameRates.Count) { [Math]::Round(($frameRates | Measure-Object -Minimum).Minimum,2) } else { $null }
        ack_p95_ms_max = if ($ackP95Samples.Count) { [Math]::Round(($ackP95Samples | Measure-Object -Maximum).Maximum,2) } else { $null }
        ack_max_ms_max = if ($ackMaxSamples.Count) { [Math]::Round(($ackMaxSamples | Measure-Object -Maximum).Maximum,2) } else { $null }
        frame_gap_p95_ms_max = if ($gapP95Samples.Count) { [Math]::Round(($gapP95Samples | Measure-Object -Maximum).Maximum,2) } else { $null }
    }
    $report = [ordered]@{
        status = $status
        started_utc = $startedAt.ToString('o')
        finished_utc = [DateTime]::UtcNow.ToString('o')
        requested_duration_minutes = $DurationMinutes
        actual_duration_minutes = if ($watch) { [Math]::Round($watch.Elapsed.TotalMinutes,2) } else { 0 }
        sample_interval_seconds = $SampleIntervalSeconds
        max_process_tree_private_mb = $MaxProcessTreePrivateMB
        sample_count = $sampleCount
        camera = $Device
        camera_instance_id = $cameraInstanceId
        launch_mode = if ($Windowed) { 'windowed' } else { 'fullscreen kiosk' }
        webview_gpu_disabled = [bool]$DisableWebViewGPU
        host = [ordered]@{
            manufacturer = $hostHardware.manufacturer
            model = $hostHardware.model
            cpu = $hostHardware.cpu
            gpu = $hostHardware.gpu
            os_caption = $hostHardware.os_caption
            os_version = $hostHardware.os_version
            os_build = $hostHardware.os_build
            architecture = $hostHardware.architecture
            total_physical_memory_mb = $hostHardware.total_physical_memory_mb
            low_ram_threshold_mb = $hostHardware.low_ram_threshold_mb
            low_ram_candidate = $hostHardware.low_ram_candidate
            memory_profile = $hostHardware.memory_profile
            initial_memory = $hostInitialMemory
        }
        low_ram_required = [bool]$RequireLowRamHost
        settings_cycles = $settingsCycles
        fullscreen_windowed_cycles = $fullscreenCycles
        published_frame_events = $publishedEventCount
        total_acknowledged_frames = $totalAcknowledgedFrames
        total_cancelled_in_flight_frames = $totalCancelledFrames
        unmatched_published_frames = $unmatchedPublishCount
        orphan_ack_events = $orphanAckEventCount
        invalid_diagnostic_lines = $invalidDiagnosticLineCount
        overlapping_ack_cancel_events = $overlappingTerminalEventCount
        duplicate_publish_events = $duplicatePublishedEventCount
        duplicate_ack_events = $duplicateAckEventCount
        duplicate_cancel_events = $duplicateCancelledEventCount
        diagnostic_ack_log_gaps_max = $diagnosticAckLogGapsMax
        final_preview_metrics = $finalMetrics
        resources = $stats
        failure = $failure
        temporary_profile_cleaned = $dataCleaned
        temporary_profile_path = if ($dataCleaned) { $null } else { $dataRoot }
        metrics_csv = $resourcesCsv
        process_details_csv = $processDetailsCsv
        diagnostics_jsonl = $diagnosticsPath
    }
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportJson -Encoding UTF8
    $lines = @(
        '# PhotoBooth idle preview / kiosk soak report',
        '',
        "- Status: $status",
        "- Camera: $Device",
        "- Requested / actual duration (min): $DurationMinutes / $($report.actual_duration_minutes)",
        "- Resource samples: $sampleCount at $SampleIntervalSeconds-second intervals",
        "- Process-tree private-bytes safety stop (MB): $MaxProcessTreePrivateMB",
        "- Launch mode: $($report.launch_mode)",
        "- WebView2 GPU acceleration disabled (diagnostic only): $DisableWebViewGPU",
        "- Settings cycles: $settingsCycles",
        "- Fullscreen/windowed cycles: $fullscreenCycles",
        "- Acknowledged frames observed: $totalAcknowledgedFrames",
        "- In-flight frames cancelled during Settings/shutdown: $totalCancelledFrames",
        "- Backend JSONL published / ACK / cancelled events: $publishedEventCount / $totalAcknowledgedFrames / $totalCancelledFrames",
        "- Unmatched publishes / orphan ACKs / invalid lines: $unmatchedPublishCount / $orphanAckEventCount / $invalidDiagnosticLineCount",
        "- Duplicate publish / ACK / cancel events: $duplicatePublishedEventCount / $duplicateAckEventCount / $duplicateCancelledEventCount",
        "- Final preview FPS: $(if($finalMetrics){$finalMetrics.fps}else{'n/a'})",
        "- CPU tree mean/peak (% of one logical core): $($stats.cpu_one_core_mean) / $($stats.cpu_one_core_peak)",
        "- Working set first/last/peak (MB): $($stats.working_set_first_mb) / $($stats.working_set_last_mb) / $($stats.working_set_peak_mb)",
        "- Private bytes tree first/last/peak (MB): $($stats.private_bytes_first_mb) / $($stats.private_bytes_last_mb) / $($stats.private_bytes_peak_mb)",
        "- PhotoBooth.exe working set first/last/peak (MB): $($stats.app_working_set_first_mb) / $($stats.app_working_set_last_mb) / $($stats.app_working_set_peak_mb)",
        "- PhotoBooth.exe private bytes first/last/peak (MB): $($stats.app_private_bytes_first_mb) / $($stats.app_private_bytes_last_mb) / $($stats.app_private_bytes_peak_mb)",
        "- WebView2 working set first/last/peak (MB): $($stats.webview_working_set_first_mb) / $($stats.webview_working_set_last_mb) / $($stats.webview_working_set_peak_mb)",
        "- WebView2 private bytes first/last/peak (MB): $($stats.webview_private_bytes_first_mb) / $($stats.webview_private_bytes_last_mb) / $($stats.webview_private_bytes_peak_mb)",
        "- WebView2 browser private bytes first/last/peak (MB): $($stats.webview_browser_private_first_mb) / $($stats.webview_browser_private_last_mb) / $($stats.webview_browser_private_peak_mb)",
        "- WebView2 renderer private bytes first/last/peak (MB): $($stats.webview_renderer_private_first_mb) / $($stats.webview_renderer_private_last_mb) / $($stats.webview_renderer_private_peak_mb)",
        "- WebView2 GPU private bytes first/last/peak (MB): $($stats.webview_gpu_private_first_mb) / $($stats.webview_gpu_private_last_mb) / $($stats.webview_gpu_private_peak_mb)",
        "- WebView2 utility private bytes first/last/peak (MB): $($stats.webview_utility_private_first_mb) / $($stats.webview_utility_private_last_mb) / $($stats.webview_utility_private_peak_mb)",
        "- WebView2 other private bytes first/last/peak (MB): $($stats.webview_other_private_first_mb) / $($stats.webview_other_private_last_mb) / $($stats.webview_other_private_peak_mb)",
        "- Host RAM / low-RAM candidate / threshold (MB): $($hostHardware.total_physical_memory_mb) / $($hostHardware.low_ram_candidate) / $LowRamMaxPhysicalMemoryMB",
        "- Host available RAM first/last/min (MB): $($stats.host_memory_available_first_mb) / $($stats.host_memory_available_last_mb) / $($stats.host_memory_available_min_mb)",
        "- Host available RAM minimum (%): $($stats.host_memory_available_percent_min)",
        "- Host commit peak (% of commit limit): $($stats.host_commit_percent_peak)",
        "- Host PagesInputPersec / PageReadsPersec peak: $($stats.host_pages_input_per_sec_peak) / $($stats.host_page_reads_per_sec_peak)",
        "- Host memory counter source: $($hostInitialMemory.source)",
        "- Max observed ACK p95 (ms): $($stats.ack_p95_ms_max)",
        "- Max observed frame-gap p95 (ms): $($stats.frame_gap_p95_ms_max)",
        "- Temporary profile cleaned: $dataCleaned",
        "- Process-tree resource CSV: $resourcesCsv",
        "- Per-process resource CSV: $processDetailsCsv",
        "- Backend preview event log: $diagnosticsPath",
        "- Summary JSON: $reportJson",
        "- Failure: $failure",
        '',
        'This is a short single-host soak, not a full-day, multi-camera, printer, Linux or production certification.'
    )
    $lines | Set-Content -LiteralPath $reportMd -Encoding UTF8
    Write-Host "TEMP_PROFILE_CLEANED=$dataCleaned"
    Write-Host "REPORT_JSON=$reportJson"
    Write-Host "REPORT_MARKDOWN=$reportMd"
    Write-Host "RESOURCE_CSV=$resourcesCsv"
    if ($status -eq 'PASS') {
        Write-Host ("PREVIEW_SOAK_GATE=PASS actual_min={0:N2} samples={1} settings_cycles={2} display_cycles={3} published={4} acked={5} cancelled={6} audit_gaps={7}" -f
            $report.actual_duration_minutes,$sampleCount,$settingsCycles,$fullscreenCycles,$publishedEventCount,$totalAcknowledgedFrames,$totalCancelledFrames,$unmatchedPublishCount)
    } else {
        Write-Host "PREVIEW_SOAK_GATE=FAIL reason=$failure"
    }
}

if ($status -ne 'PASS') { exit 1 }
exit 0
