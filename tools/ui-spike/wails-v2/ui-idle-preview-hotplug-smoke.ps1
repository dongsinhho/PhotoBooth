[CmdletBinding()]
param(
    [string]$Device = 'B525 HD Webcam',
    [ValidateRange(15,60)][int]$StartupTimeoutSeconds = 30,
    [ValidateRange(15,90)][int]$PreviewReadyTimeoutSeconds = 35,
    [ValidateRange(10,180)][int]$UnplugObservationTimeoutSeconds = 45,
    [ValidateRange(5,45)][int]$RecoveryStateTimeoutSeconds = 20,
    [ValidateRange(15,120)][int]$ReconnectTimeoutSeconds = 60,
    [ValidateRange(15,120)][int]$PreviewResumeTimeoutSeconds = 45,
    [ValidateRange(1,60)][int]$MinimumFramesAfterReconnect = 5,
    [string]$ExecutablePath = '',
    [switch]$PreflightOnly,
    [switch]$RequireDifferentPort,
    [string]$BaselineLocationPath = '',
    [switch]$RequireSamePortOnReconnect
)

$ErrorActionPreference = 'Stop'
if ($RequireDifferentPort -and $RequireSamePortOnReconnect) {
    throw '-RequireDifferentPort and -RequireSamePortOnReconnect are mutually exclusive.'
}
$moduleRoot = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ExecutablePath)) {
    $candidate = Join-Path $moduleRoot 'build\bin\PhotoBooth.exe'
    if (!(Test-Path -LiteralPath $candidate)) { $candidate = Join-Path $moduleRoot 'build\bin\wails-v2.exe' }
    $exe = $candidate
} else {
    $exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
}
if (!(Test-Path -LiteralPath $exe -PathType Leaf)) {
    throw "Wails executable is missing; build the current Windows UI candidate first: $exe"
}
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$runId = [guid]::NewGuid().ToString('N')
$dataRoot = Join-Path $env:TEMP "photobooth-idle-preview-hotplug-$runId"
$sessionsRoot = Join-Path $dataRoot 'sessions'
$configPath = Join-Path $dataRoot 'store-config-v1.json'
$reportPath = Join-Path $env:TEMP "photobooth-idle-preview-hotplug-$runId.json"
$diagnosticsPath = Join-Path $env:TEMP "photobooth-idle-preview-hotplug-$runId.jsonl"
$previousDataRoot = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', 'Process')
$previousDiagnosticsPath = [Environment]::GetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', 'Process')
$app = $null
$cameraInstanceId = $null
$reconnectedInstanceId = $null
$cameraTopologyBefore = $null
$cameraTopologyAfter = $null
$differentPortVerified = $false
$samePortReconnectVerified = $false
$baselinePortDifferentVerified = $false
$failure = $null
$status = 'FAIL'
$forcedStop = $false
$dataCleaned = $false
$unplugObserved = $false
$replugObserved = $false
$reconnectingStatusObserved = $false
$recoveryReadyObserved = $false
$initialMetrics = $null
$metricsBeforeUnplug = $null
$metricsAfterReconnect = $null
$checks = [System.Collections.Generic.List[string]]::new()
$startedAt = [DateTime]::UtcNow

function Get-B525 {
    try {
        return @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction Stop |
            Where-Object { $_.FriendlyName -like "*$Device*" -and $_.Status -eq 'OK' })
    } catch { return @() }
}

function Get-B525ByInstanceId([string]$InstanceId) {
    try {
        return @(Get-PnpDevice -PresentOnly -Class Camera -ErrorAction Stop |
            Where-Object { $_.InstanceId -eq $InstanceId -and $_.Status -eq 'OK' })
    } catch { return @() }
}

function Get-CameraTopology([string]$InstanceId) {
    $locationInfo = Get-PnpDeviceProperty -InstanceId $InstanceId -KeyName 'DEVPKEY_Device_LocationInfo' -ErrorAction Stop
    $locationPathsProperty = Get-PnpDeviceProperty -InstanceId $InstanceId -KeyName 'DEVPKEY_Device_LocationPaths' -ErrorAction Stop
    $locationPaths = @($locationPathsProperty.Data | ForEach-Object { [string]$_ } | Where-Object { $_ })
    $parentProperty = Get-PnpDeviceProperty -InstanceId $InstanceId -KeyName 'DEVPKEY_Device_Parent' -ErrorAction SilentlyContinue
    $parentId = if ($parentProperty) { [string]$parentProperty.Data } else { '' }
    $parentName = ''
    $controllerName = ''
    $controllerInstanceId = ''
    $chain = [System.Collections.Generic.List[string]]::new()
    $currentId = $InstanceId
    for ($level = 0; $level -lt 10 -and $currentId; $level++) {
        try { $node = Get-PnpDevice -InstanceId $currentId -ErrorAction Stop } catch { break }
        $chain.Add(('{0}|{1}|{2}' -f $node.Class,$node.FriendlyName,$currentId)) | Out-Null
        if ($level -eq 1) { $parentName = [string]$node.FriendlyName }
        if ([string]$node.FriendlyName -match 'Host Controller') {
            $controllerName = [string]$node.FriendlyName
            $controllerInstanceId = [string]$currentId
            break
        }
        $nextParent = Get-PnpDeviceProperty -InstanceId $currentId -KeyName 'DEVPKEY_Device_Parent' -ErrorAction SilentlyContinue
        if (!$nextParent -or !$nextParent.Data) { break }
        $currentId = [string]$nextParent.Data
    }
    [pscustomobject]@{
        location_info = [string]$locationInfo.Data
        location_paths = $locationPaths
        parent_instance_id = $parentId
        parent_friendly_name = $parentName
        host_controller_name = $controllerName
        host_controller_instance_id = $controllerInstanceId
        topology_chain = @($chain.ToArray())
    }
}

function Get-Window {
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
    $window = Get-Window
    if (!$window) { return @() }
    $values = [System.Collections.Generic.List[string]]::new()
    foreach ($element in $window.FindAll(
        [System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.Condition]::TrueCondition
    )) {
        if ($element.Current.Name) { $values.Add($element.Current.Name) | Out-Null }
    }
    return $values.ToArray()
}

function Get-PreviewMetrics([string[]]$Text) {
    $line = @($Text | Where-Object { $_ -like 'PREVIEW_METRICS*' } | Select-Object -Last 1)
    if ($line.Count -eq 0) { return $null }
    $value = [string]$line[0]
    $result = [ordered]@{}
    foreach ($key in @('state','frames_received','frames_decoded','acks','ack_failed','decode_failed','fps')) {
        $match = [regex]::Match($value, "(?:^|\s)$key=([^\s]+)")
        if (!$match.Success) { return $null }
        $result[$key] = $match.Groups[1].Value
    }
    return [pscustomobject]@{
        state = [string]$result.state
        frames_received = [int]$result.frames_received
        frames_decoded = [int]$result.frames_decoded
        acks = [int]$result.acks
        ack_failed = [int]$result.ack_failed
        decode_failed = [int]$result.decode_failed
        fps = [double]$result.fps
    }
}

function Get-PreviewStatusEvents {
    $records = [System.Collections.Generic.List[object]]::new()
    if (!(Test-Path -LiteralPath $diagnosticsPath -PathType Leaf)) { return @() }
    foreach ($line in Get-Content -LiteralPath $diagnosticsPath -ErrorAction SilentlyContinue) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        try {
            $record = $line | ConvertFrom-Json -ErrorAction Stop
            if ($record.event -eq 'preview_status') { $records.Add($record) | Out-Null }
        } catch {
            # The writer appends one JSON object per line; ignore an incomplete
            # trailing line while the app is still running and audit after close.
        }
    }
    return $records.ToArray()
}

function Wait-PreviewStatusEvent([string]$State, [int]$TimeoutSeconds, [int]$AfterIndex, [string]$Label) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $events = @(Get-PreviewStatusEvents)
        for ($i = [Math]::Max(0,$AfterIndex); $i -lt $events.Count; $i++) {
            if ($events[$i].state -eq $State) {
                return [pscustomobject]@{ index = $i; event = $events[$i] }
            }
        }
        if ($app -and !(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "PhotoBooth exited while waiting for backend preview state '$State'."
        }
        Start-Sleep -Milliseconds 100
    }
    $states = @((Get-PreviewStatusEvents) | ForEach-Object { $_.state }) -join ', '
    throw "Timed out waiting for backend preview state '$State' ($Label). Observed: $states"
}

function Wait-PreviewReady([int]$TimeoutSeconds, [int]$MinimumFrames, [int]$MinimumAcks, [string]$Label) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $text = @(Get-WindowText)
        $metrics = Get-PreviewMetrics $text
        $joined = $text -join ' '
        if ($metrics -and $metrics.state -eq 'ready' -and
            $metrics.frames_received -ge $MinimumFrames -and $metrics.acks -ge $MinimumAcks -and
            $joined -match 'LIVE CAMERA PREVIEW') { return $metrics }
        if ($app -and !(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw "PhotoBooth exited while waiting for $Label."
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for $Label. UI: $((Get-WindowText) -join ' | ')"
}

function Get-PreviewEventAudit([string]$Path) {
    $events = [System.Collections.Generic.List[object]]::new()
    $invalidLines = 0
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        foreach ($line in Get-Content -LiteralPath $Path) {
            if ([string]::IsNullOrWhiteSpace($line)) { continue }
            try { $events.Add(($line | ConvertFrom-Json -ErrorAction Stop)) | Out-Null }
            catch { $invalidLines++ }
        }
    }
    $published = @($events | Where-Object { $_.event -eq 'frame_published' })
    $acked = @($events | Where-Object { $_.event -eq 'frame_acknowledged' })
    $cancelled = @($events | Where-Object { $_.event -eq 'frame_cancelled' })
    $timeouts = @($events | Where-Object { $_.event -eq 'frame_ack_timeout' })
    $decodeFailures = @($events | Where-Object { $_.event -eq 'frame_decode_failed' })
    $terminals = @($acked) + @($cancelled) + @($timeouts)
    $publishedGroups = @($published | Group-Object { [string]$_.sequence })
    $terminalGroups = @($terminals | Group-Object { [string]$_.sequence })
    $terminalBySequence = @{}
    foreach ($group in $terminalGroups) { $terminalBySequence[$group.Name] = $group.Count }
    $publishedBySequence = @{}
    foreach ($group in $publishedGroups) { $publishedBySequence[$group.Name] = $group.Count }
    $unmatched = @($publishedBySequence.Keys | Where-Object { !$terminalBySequence.ContainsKey($_) -or $terminalBySequence[$_] -ne 1 })
    $orphanTerminals = @($terminalBySequence.Keys | Where-Object { !$publishedBySequence.ContainsKey($_) })
    $statusEvents = @($events | Where-Object { $_.event -eq 'preview_status' })
    $reconnectingIndexes = @()
    for ($i = 0; $i -lt $statusEvents.Count; $i++) {
        if ($statusEvents[$i].state -eq 'reconnecting') { $reconnectingIndexes += $i }
    }
    $readyAfterReconnect = $false
    if ($reconnectingIndexes.Count -gt 0) {
        $firstReconnect = $reconnectingIndexes[0]
        for ($i = $firstReconnect + 1; $i -lt $statusEvents.Count; $i++) {
            if ($statusEvents[$i].state -eq 'ready') { $readyAfterReconnect = $true; break }
        }
    }

    # A later "ready" is not proof of automatic recovery: the operator may have
    # used Retry Connect after the 30-attempt recovery loop had already failed.
    $firstRecoveryFailureIndex = -1
    $firstRecoverySuccessIndex = -1
    $recoveryAttemptFailures = @($events | Where-Object { $_.event -eq 'recovery_open_attempt_failed' })
    $recoveryFailures = @($events | Where-Object { $_.event -eq 'recovery_failed' })
    $recoverySuccesses = @($events | Where-Object { $_.event -eq 'recovery_succeeded' })
    $recoveryStarted = @($events | Where-Object { $_.event -eq 'recovery_started' })
    if ($reconnectingIndexes.Count -gt 0) {
        $firstReconnectEventIndex = -1
        for ($i = 0; $i -lt $events.Count; $i++) {
            if ($events[$i].event -eq 'preview_status' -and $events[$i].state -eq 'reconnecting') {
                $firstReconnectEventIndex = $i
                break
            }
        }
        for ($i = $firstReconnectEventIndex + 1; $i -lt $events.Count; $i++) {
            if ($events[$i].event -eq 'recovery_failed' -and $firstRecoveryFailureIndex -lt 0) {
                $firstRecoveryFailureIndex = $i
            }
            if ($events[$i].event -eq 'recovery_succeeded' -and $firstRecoverySuccessIndex -lt 0) {
                $firstRecoverySuccessIndex = $i
            }
        }
    }
    $automaticRecoverySucceeded = (
        $firstRecoverySuccessIndex -ge 0 -and
        ($firstRecoveryFailureIndex -lt 0 -or $firstRecoverySuccessIndex -lt $firstRecoveryFailureIndex)
    )
    $automaticRecoveryStatus = 'NOT_CONFIRMED'
    if ($automaticRecoverySucceeded) {
        $automaticRecoveryStatus = 'PASS'
    } elseif ($firstRecoveryFailureIndex -ge 0) {
        $automaticRecoveryStatus = 'FAIL'
    }

    $previewRestartedAfterRecoveryFailure = $false
    $readyAfterRecoveryFailure = $false
    if ($firstRecoveryFailureIndex -ge 0) {
        for ($i = $firstRecoveryFailureIndex + 1; $i -lt $events.Count; $i++) {
            if ($events[$i].event -eq 'preview_status' -and $events[$i].state -eq 'starting') {
                $previewRestartedAfterRecoveryFailure = $true
            }
            if ($previewRestartedAfterRecoveryFailure -and $events[$i].event -eq 'preview_status' -and $events[$i].state -eq 'ready') {
                $readyAfterRecoveryFailure = $true
                break
            }
        }
    }

    return [pscustomobject]@{
        event_count = $events.Count
        invalid_json_lines = $invalidLines
        published = $published.Count
        acknowledged = $acked.Count
        cancelled = $cancelled.Count
        ack_timeouts = $timeouts.Count
        decode_failures = $decodeFailures.Count
        duplicate_publishes = @($publishedGroups | Where-Object Count -gt 1).Count
        duplicate_terminal_sequences = @($terminalGroups | Where-Object Count -gt 1).Count
        unmatched_published_sequences = $unmatched.Count
        orphan_terminal_sequences = $orphanTerminals.Count
        reconnecting_status_events = $reconnectingIndexes.Count
        ready_after_reconnecting = $readyAfterReconnect
        recovery_started = $recoveryStarted.Count
        recovery_open_attempt_failures = $recoveryAttemptFailures.Count
        recovery_failures = $recoveryFailures.Count
        recovery_successes = $recoverySuccesses.Count
        automatic_recovery_status = $automaticRecoveryStatus
        preview_restarted_after_recovery_failure = $previewRestartedAfterRecoveryFailure
        ready_after_recovery_failure = $readyAfterRecoveryFailure
        all_published_have_one_terminal = ($unmatched.Count -eq 0 -and $orphanTerminals.Count -eq 0)
    }
}

function Invoke-GracefulClose {
    if (!$app) { return }
    $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
    if (!$process) { return }
    [void]$process.CloseMainWindow()
    if (!$process.WaitForExit(5000)) {
        $script:forcedStop = $true
        Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
    }
}

if ($PreflightOnly) {
    $preflightCamera = @(Get-B525)
    if ($preflightCamera.Count -ne 1) {
        throw "Expected exactly one healthy present B525 camera matching '$Device'; found $($preflightCamera.Count)."
    }
    $preflightApps = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('PhotoBooth.exe', 'wails-v2.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath -like '*PhotoBooth*tools\ui-spike\wails-v2\build\bin*'
    })
    if ($preflightApps.Count -gt 0) {
        throw 'Close the existing PhotoBooth UI before running the physical idle-preview test.'
    }
    [pscustomobject]@{
        preflight_only = $true
        executable = $exe
        camera_name = [string]$preflightCamera[0].FriendlyName
        camera_instance_id = [string]$preflightCamera[0].InstanceId
        camera_status = [string]$preflightCamera[0].Status
        camera_topology = Get-CameraTopology ([string]$preflightCamera[0].InstanceId)
        require_different_port = [bool]$RequireDifferentPort
        baseline_location_path = $BaselineLocationPath
        require_same_port_on_reconnect = [bool]$RequireSamePortOnReconnect
        baseline_port_different = if ($BaselineLocationPath) { @($preflightCameraTopology = Get-CameraTopology ([string]$preflightCamera[0].InstanceId); $preflightCameraTopology.location_paths | Where-Object { $_ -eq $BaselineLocationPath }).Count -eq 0 } else { $null }
        physical_unplug_replug_required = $true
        opens_camera = $false
        starts_application = $false
    } | ConvertTo-Json -Depth 4
    exit 0
}

try {
    $initialCamera = @(Get-B525)
    if ($initialCamera.Count -ne 1) {
        throw "Expected exactly one healthy present B525 camera matching '$Device'; found $($initialCamera.Count)."
    }
    $cameraInstanceId = [string]$initialCamera[0].InstanceId
    $cameraTopologyBefore = Get-CameraTopology $cameraInstanceId
    if (($RequireDifferentPort -or $BaselineLocationPath) -and $cameraTopologyBefore.location_paths.Count -eq 0) {
        throw 'Windows did not provide a USB location path; cannot safely verify the requested port constraint.'
    }
    if ($BaselineLocationPath) {
        if (@($cameraTopologyBefore.location_paths | Where-Object { $_ -eq $BaselineLocationPath }).Count -gt 0) {
            throw "The camera is still on the reference USB location path '$BaselineLocationPath'; move it to the alternate physical port before this run."
        }
        $baselinePortDifferentVerified = $true
        $differentPortVerified = $true
        $checks.Add('Initial B525 location path differs from the supplied reference port') | Out-Null
        Write-Host ("USB_PORT_DIFFERS_FROM_REFERENCE=True current={0} reference={1}" -f ($cameraTopologyBefore.location_paths -join ';'),$BaselineLocationPath)
    }

    $existingApps = @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @('PhotoBooth.exe', 'wails-v2.exe') -and
        $_.ExecutablePath -and
        $_.ExecutablePath -like '*PhotoBooth*tools\ui-spike\wails-v2\build\bin*'
    })
    if ($existingApps.Count -gt 0) {
        $details = ($existingApps | ForEach-Object { "PID=$($_.ProcessId) EXE=$($_.ExecutablePath)" }) -join '; '
        throw "Another PhotoBooth UI process is already running; close it before the exclusive-camera test. $details"
    }

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
    [IO.File]::WriteAllText($diagnosticsPath, '', [Text.UTF8Encoding]::new($false))
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $dataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', $diagnosticsPath, 'Process')

    Write-Host "WAILS_IDLE_PREVIEW_HOTPLUG=START device=$Device"
    Write-Host 'This test exercises idle live preview only. It will NOT start a photo session.'
    Write-Host "TEMP_PROFILE=$dataRoot"
    Write-Host "PREVIEW_EVENT_LOG=$diagnosticsPath"

    $app = Start-Process -FilePath $exe -WorkingDirectory $moduleRoot -PassThru
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $window = $null
    while ((Get-Date) -lt $deadline -and !$window) {
        Start-Sleep -Milliseconds 200
        $window = Get-Window
        if (!(Get-Process -Id $app.Id -ErrorAction SilentlyContinue)) {
            throw 'PhotoBooth exited before the main window appeared.'
        }
    }
    if (!$window) { throw 'PhotoBooth main window did not appear.' }
    $checks.Add('isolated PhotoBooth window opened') | Out-Null

    $initialMetrics = Wait-PreviewReady $PreviewReadyTimeoutSeconds 5 5 'initial idle preview frames and ACKs'
    $checks.Add('idle preview reached ready state with at least five acknowledged frames') | Out-Null
    Write-Host ("IDLE_PREVIEW_READY=True frames={0} acks={1} fps={2:N2}" -f
        $initialMetrics.frames_received,$initialMetrics.acks,$initialMetrics.fps)

    $baselineDeadline = (Get-Date).AddSeconds(8)
    do {
        Start-Sleep -Milliseconds 250
        $metricsBeforeUnplug = Get-PreviewMetrics @(Get-WindowText)
    } while ((Get-Date) -lt $baselineDeadline -and
        (!$metricsBeforeUnplug -or $metricsBeforeUnplug.frames_received -lt ($initialMetrics.frames_received + 5) -or
         $metricsBeforeUnplug.acks -lt ($initialMetrics.acks + 5)))
    if (!$metricsBeforeUnplug -or
        $metricsBeforeUnplug.frames_received -lt ($initialMetrics.frames_received + 5) -or
        $metricsBeforeUnplug.acks -lt ($initialMetrics.acks + 5)) {
        throw 'Preview did not continue delivering and acknowledging frames before the unplug action.'
    }
    $checks.Add('preview frame delivery and ACKs continued before unplug') | Out-Null
    $statusEventCountBeforeUnplug = @(Get-PreviewStatusEvents).Count

    Write-Host ''
    Write-Host '======================================================================'
    Write-Host 'PHYSICAL ACTION REQUIRED: UNPLUG THE B525 USB CABLE NOW.'
    Write-Host 'Keep it disconnected until this script observes Windows PnP removal'
    if ($RequireDifferentPort) {
        Write-Host 'Reconnect the B525 to a DIFFERENT physical USB port on the PC, not the same socket.'
    } elseif ($RequireSamePortOnReconnect) {
        Write-Host 'Reconnect the B525 to THIS SAME alternate USB port so recovery is isolated to this port.'
    } else {
        Write-Host 'Reconnect the B525 to the same or another USB port.'
    }
    Write-Host 'The script records the Windows USB location path.'
    Write-Host 'Do NOT start a photo session or close the app during this test.'
    Write-Host '======================================================================'
    $unplugDeadline = (Get-Date).AddSeconds($UnplugObservationTimeoutSeconds)
    while ((Get-Date) -lt $unplugDeadline) {
        if (@(Get-B525ByInstanceId $cameraInstanceId).Count -eq 0) {
            $unplugObserved = $true
            $checks.Add('Windows PnP observed the original B525 instance disappear') | Out-Null
            Write-Host 'USB_UNPLUG_OBSERVED=True'
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (!$unplugObserved) {
        throw "Windows did not observe B525 instance '$cameraInstanceId' disappear within $UnplugObservationTimeoutSeconds seconds."
    }

    $reconnectingStatus = Wait-PreviewStatusEvent 'reconnecting' $RecoveryStateTimeoutSeconds $statusEventCountBeforeUnplug 'idle preview reconnecting state'
    $reconnectingStatusObserved = $true
    $checks.Add('Wails backend emitted reconnecting state while idle preview owned the camera') | Out-Null
    Write-Host ("PREVIEW_RECONNECTING_STATUS=OBSERVED index={0} message={1}" -f $reconnectingStatus.index,$reconnectingStatus.event.message)

    Write-Host ''
    Write-Host '======================================================================'
    Write-Host 'RECONNECT_B525_NOW: plug the B525 USB cable back in now.'
    if ($RequireDifferentPort) {
        Write-Host 'IMPORTANT: use a DIFFERENT physical USB port on the PC from the one used before unplugging.'
    } elseif ($RequireSamePortOnReconnect) {
        Write-Host 'IMPORTANT: reconnect to the SAME alternate USB socket you used immediately before unplugging.'
    }
    Write-Host 'Keep it connected after Windows detects it.'
    Write-Host '======================================================================'
    $reconnectDeadline = (Get-Date).AddSeconds($ReconnectTimeoutSeconds)
    $returnedCamera = @()
    while ((Get-Date) -lt $reconnectDeadline) {
        $returnedCamera = @(Get-B525)
        if ($returnedCamera.Count -gt 0) { break }
        Start-Sleep -Milliseconds 100
    }
    if ($returnedCamera.Count -eq 0) {
        throw "Windows did not observe a healthy B525 within $ReconnectTimeoutSeconds seconds after reconnect."
    }
    $replugObserved = $true
    $reconnectedInstanceId = [string]$returnedCamera[0].InstanceId
    $cameraTopologyAfter = Get-CameraTopology $reconnectedInstanceId
    $checks.Add('Windows PnP observed a healthy B525 after reconnect') | Out-Null
    Write-Host "USB_REPLUG_OBSERVED=True instance_id=$reconnectedInstanceId"
    Write-Host ("USB_LOCATION_BEFORE={0}" -f ($cameraTopologyBefore.location_paths -join ';'))
    Write-Host ("USB_LOCATION_AFTER={0}" -f ($cameraTopologyAfter.location_paths -join ';'))
    $beforePaths = @($cameraTopologyBefore.location_paths)
    $afterPaths = @($cameraTopologyAfter.location_paths)
    $sharedPaths = @($beforePaths | Where-Object { $afterPaths -contains $_ })
    if ($RequireDifferentPort) {
        if ($afterPaths.Count -eq 0) {
            throw 'Windows did not provide a USB location path after reconnect; cannot verify a different physical port.'
        }
        if ($sharedPaths.Count -gt 0) {
            throw ("B525 was detected on the SAME USB location path. Original='{0}'. Re-run and reconnect to a different physical USB socket." -f ($sharedPaths -join ';'))
        }
        $differentPortVerified = $true
        $checks.Add('Windows USB location path changed; reconnect used a different physical port path') | Out-Null
        Write-Host 'USB_PORT_CHANGED=True'
    }
    if ($RequireSamePortOnReconnect) {
        $beforeKey = (@($beforePaths | Sort-Object) -join '||')
        $afterKey = (@($afterPaths | Sort-Object) -join '||')
        if (!$beforeKey -or $beforeKey -ne $afterKey) {
            throw ("B525 did not return to the same tested alternate port path. Before='{0}' After='{1}'." -f $beforeKey,$afterKey)
        }
        $samePortReconnectVerified = $true
        $checks.Add('B525 returned to the same alternate USB location path after recovery') | Out-Null
        Write-Host 'USB_SAME_PORT_RECONNECT=True'
    }

    $readyStatus = Wait-PreviewStatusEvent 'ready' $PreviewResumeTimeoutSeconds ($reconnectingStatus.index + 1) 'preview ready after B525 reconnect'
    $recoveryReadyObserved = $true
    $checks.Add('Wails backend emitted ready after the reconnecting state') | Out-Null
    Write-Host ("PREVIEW_READY_STATUS=OBSERVED index={0} message={1}" -f $readyStatus.index,$readyStatus.event.message)

    $minimumFrames = $metricsBeforeUnplug.frames_received + $MinimumFramesAfterReconnect
    $minimumAcks = $metricsBeforeUnplug.acks + $MinimumFramesAfterReconnect
    $metricsAfterReconnect = Wait-PreviewReady $PreviewResumeTimeoutSeconds $minimumFrames $minimumAcks 'idle preview frame/ACK progress after reconnect'
    $checks.Add('idle preview delivered new ACKed frames after reconnect') | Out-Null
    Write-Host ("PREVIEW_RESUMED=True frames={0} acks={1} fps={2:N2}" -f
        $metricsAfterReconnect.frames_received,$metricsAfterReconnect.acks,$metricsAfterReconnect.fps)

    if ($metricsAfterReconnect.ack_failed -ne 0 -or $metricsAfterReconnect.decode_failed -ne 0) {
        throw "Preview reported ACK/decode failures after reconnect: ack_failed=$($metricsAfterReconnect.ack_failed), decode_failed=$($metricsAfterReconnect.decode_failed)."
    }
    $sessionDirectories = @(Get-ChildItem -LiteralPath $sessionsRoot -Directory -ErrorAction SilentlyContinue)
    if ($sessionDirectories.Count -ne 0) {
        throw 'A photo session directory appeared during an idle-preview-only test; refusing to classify this as idle-only.'
    }
    $checks.Add('no photo session was started') | Out-Null

    $recoveryAudit = Get-PreviewEventAudit $diagnosticsPath
    Write-Host "AUTOMATIC_RECOVERY_STATUS=$($recoveryAudit.automatic_recovery_status)"
    Write-Host "PREVIEW_RESTARTED_AFTER_RECOVERY_FAILURE=$($recoveryAudit.preview_restarted_after_recovery_failure)"
    if ($recoveryAudit.automatic_recovery_status -eq 'PASS') {
        $status = 'PASS'
    } elseif ($recoveryAudit.automatic_recovery_status -eq 'FAIL' -and $recoveryAudit.ready_after_recovery_failure) {
        # Keep the diagnostic result useful while refusing to count a user-assisted
        # Retry Connect as automatic hotplug recovery.
        $status = 'PARTIAL'
        $failure = 'Automatic hotplug recovery failed; preview became ready only after a later restart/retry.'
        Write-Host 'AUTOMATIC_HOTPLUG_RECOVERY=FAIL'
        Write-Host 'PREVIEW_RESTART_AFTER_FAILURE=PASS'
    } else {
        $status = 'FAIL'
        $failure = 'Automatic hotplug recovery was not confirmed by recovery_succeeded telemetry.'
    }
} catch {
    $failure = $_.Exception.Message
    Write-Host 'WAILS_IDLE_PREVIEW_HOTPLUG=FAIL'
    Write-Host "FAILURE=$failure"
    Write-Host "UI_TEXT=$((Get-WindowText) -join ' | ')"
} finally {
    if ($app) {
        $process = Get-Process -Id $app.Id -ErrorAction SilentlyContinue
        if ($process) {
            [void]$process.CloseMainWindow()
            if (!$process.WaitForExit(5000)) {
                $forcedStop = $true
                Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
            }
        }
    }
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_DATA_DIR', $previousDataRoot, 'Process')
    [Environment]::SetEnvironmentVariable('PHOTOBOOTH_PREVIEW_METRICS_PATH', $previousDiagnosticsPath, 'Process')

    $sessionDirectories = @(Get-ChildItem -LiteralPath $sessionsRoot -Directory -ErrorAction SilentlyContinue)
    if ($sessionDirectories.Count -gt 0 -and $status -in @('PASS', 'PARTIAL')) {
        $status = 'FAIL'
        $failure = 'A photo session directory was created during an idle-preview-only test.'
    }
    if (Test-Path -LiteralPath $dataRoot) {
        Remove-Item -LiteralPath $dataRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    $dataCleaned = !(Test-Path -LiteralPath $dataRoot)
    $audit = Get-PreviewEventAudit $diagnosticsPath
    if ($status -in @('PASS', 'PARTIAL') -and (
        $forcedStop -or
        $audit.invalid_json_lines -gt 0 -or
        !$audit.all_published_have_one_terminal -or
        $audit.ack_timeouts -gt 0 -or
        $audit.decode_failures -gt 0 -or
        !$audit.ready_after_reconnecting
    )) {
        $status = 'FAIL'
        $failure = 'Post-shutdown preview JSONL audit failed or the app required a forced stop.'
    }
    $report = [ordered]@{
        started_utc = $startedAt.ToString('o')
        finished_utc = [DateTime]::UtcNow.ToString('o')
        status = $status
        failure = $failure
        camera = $Device
        camera_instance_id_before = $cameraInstanceId
        camera_instance_id_after = $reconnectedInstanceId
        camera_topology_before = $cameraTopologyBefore
        camera_topology_after = $cameraTopologyAfter
        require_different_port = [bool]$RequireDifferentPort
        baseline_location_path = $BaselineLocationPath
        baseline_port_different_verified = $baselinePortDifferentVerified
        require_same_port_on_reconnect = [bool]$RequireSamePortOnReconnect
        same_port_reconnect_verified = $samePortReconnectVerified
        different_port_verified = $differentPortVerified
        physical_unplug_observed = $unplugObserved
        physical_replug_observed = $replugObserved
        reconnecting_status_observed = $reconnectingStatusObserved
        ready_status_after_reconnecting_observed = $recoveryReadyObserved
        automatic_recovery_status = $audit.automatic_recovery_status
        preview_restarted_after_recovery_failure = $audit.preview_restarted_after_recovery_failure
        ready_after_recovery_failure = $audit.ready_after_recovery_failure
        preview_initial = $initialMetrics
        preview_before_unplug = $metricsBeforeUnplug
        preview_after_reconnect = $metricsAfterReconnect
        no_photo_session_started = ($sessionDirectories.Count -eq 0)
        forced_app_stop = $forcedStop
        temporary_profile_cleaned = $dataCleaned
        event_audit = $audit
        checks = @($checks)
        diagnostics_jsonl = $diagnosticsPath
        report_path = $reportPath
    }
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host "TEMP_PROFILE_CLEANED=$dataCleaned"
    Write-Host "PREVIEW_EVENT_LOG=$diagnosticsPath"
    Write-Host "TEST_REPORT=$reportPath"
}

if ($status -ne 'PASS') {
    Write-Host "WAILS_IDLE_PREVIEW_HOTPLUG=FAIL reason=$failure"
    exit 1
}
Write-Host 'WAILS_IDLE_PREVIEW_HOTPLUG=PASS'
exit 0
