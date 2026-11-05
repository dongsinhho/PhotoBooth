# Media Foundation Enumeration Observer

A disposable Windows diagnostic process for correlating native Media Foundation camera enumeration with PhotoBooth's in-process recovery attempts.

It does not open a capture stream, create a PhotoBooth session, or change camera/provider behavior. Each sample performs:

1. camera.Init() (COM + Media Foundation initialization)
2. camera.EnumDevice() (enumerate camera activation sources and read friendly name/symbolic link)
3. camera.Shutdown() (release MF/COM state)

The process locks its main goroutine to one OS thread so each initialization/enumeration/shutdown sequence runs on the same thread, as required by the vendored camera package.

## Run a connected-camera control

From this directory, with the B525 connected:

~~~powershell
go test ./...
go vet ./...
go run . -interval 500ms -duration 5s -output "$env:TEMP\photobooth-mf-enumeration-control.jsonl"
~~~

The output path must not already exist. The tool refuses to overwrite a previous diagnostic file.

## Observe a physical hotplug cycle

Start this process before the PhotoBooth unplug/replug test. Run long enough to cover the unplug, replug, the 30 automatic recovery attempts, and the manual retry decision:

~~~powershell
go run . -interval 500ms -duration 3m -output "$env:TEMP\photobooth-mf-enumeration-xhc1-$(Get-Date -Format yyyyMMdd-HHmmss).jsonl"
~~~

Keep the observer running while the camera is unplugged and after it is reconnected. Do not click PhotoBooth's Retry Connect until the observer has captured whether and when the B525 becomes visible to a separate process. Then correlate JSONL timestamps with PhotoBooth's recovery_open_attempt_failed records and PnP observations.

## Compare a second OS thread inside the same observer process

The observer can run one extra sample on a second OS thread while the primary observer goroutine remains locked to its original OS thread. This distinguishes a thread-local problem from a state that persists across threads within the same process.

Start it before the physical cycle:

~~~powershell
$trigger = Join-Path $env:TEMP 'photobooth-mf-secondary-thread.trigger'
Remove-Item -LiteralPath $trigger -Force -ErrorAction SilentlyContinue
go run . -interval 500ms -duration 5m -output "$env:TEMP\photobooth-mf-thread-compare.jsonl" -thread-probe-trigger-file $trigger
~~~

After reconnect, when Windows PnP reports the B525 present again and the primary thread still records zero_sources, create the trigger file in another PowerShell window:

~~~powershell
New-Item -ItemType File -Path $trigger
~~~

The observer consumes the trigger and records one sample with thread_role=secondary_locked_thread. The regular samples use thread_role=primary_locked_thread.

Interpretation:
- Primary thread returns zero_sources while secondary thread finds B525: evidence for thread-scoped COM/Media Foundation state.
- Both threads return zero_sources while a fresh process finds B525: evidence for state scoped to the long-lived process or its Media Foundation runtime, not proof of the exact internal cause.
- Both threads find B525: the primary thread may have recovered by the time the probe was triggered; correlate timestamps before drawing a conclusion.

The secondary sample runs sequentially, not concurrently with the primary enumeration call, because the vendored package uses process-global runtime handles. The primary thread remains locked while the secondary goroutine is pinned, so the secondary sample necessarily uses a different OS thread.

## JSONL fields

Every line records:
- UTC timestamp and sample index
- elapsed time since observer start
- thread_role: primary_locked_thread or secondary_locked_thread
- status: camera_found, zero_sources, init_error, or enumeration_error
- device count, friendly name and symbolic link
- Init, enumeration and total sample durations
- initialization/enumeration errors when present

A successful empty enumeration is recorded as zero_sources. Errors are kept distinct from an empty device list. Each line is flushed to disk immediately.
