# ADR 0030: Gate Linux V4L2 provider implementation on a real device path

- **Status:** V4L2 selected as the Linux API. The experimental Linux/amd64 provider passes B525 capture→render→local-output and 100 same-instance lifecycle cycles, three consecutive in-process USB detach/reattach recoveries in one workflow, and a 15-minute single-stream soak (1,800 previews + 1,800 captures) on WSL2 USB/IP at 640x480 MJPEG / 15 FPS requested. A later WSL2 kernel 6.18.58.1 run also passed 1280x720 MJPEG / 5 FPS provider/workflow acceptance, 10 lifecycle cycles, a 15-minute single-stream soak (1,800 previews + 1,800 captures fully decoded) and three consecutive physical in-process USB detach/reattach recoveries in one six-shot diagnostic workflow. 1280x720 at 10/15 FPS and 640x480 at 30 FPS still failed with error-flagged buffers. Native Ubuntu direct-USB, repeated hotplug campaigns at 1280x720, broader endurance, UI and printer acceptance remain open; production config selection remains deferred.
- **Date:** 2026-11-08
- **Decision owner:** Product owner

## Context

The product targets Windows and Linux while keeping camera-specific APIs behind `camera.Provider`. The Linux kernel exposes UVC capture devices through V4L2 character devices such as `/dev/video0`. A working provider must query device capabilities, negotiate an actual pixel format/resolution/frame interval, stream frames, validate the returned JPEG/raw frame, and support bounded cancellation/recovery. A device appearing in Windows PnP or USB enumeration is not evidence that Linux can open it.

The first Linux target is Ubuntu 26.04.1 LTS x86_64 running in WSL2 on ShinPc. Windows PnP sees the reference Logitech B525 HD Webcam (USB VID_046D / PID_0836), but WSL2 currently exposes no `/sys/bus/usb/devices`, `/sys/class/video4linux`, `/dev/video*` or `/dev/media*`. The `uvcvideo.ko` module file exists in the WSL kernel modules directory but is not loaded. The current Windows host has no `usbipd.exe` command on PATH. The current Linux user is not a member of the `video` group.

## Decision

1. Use the Linux kernel V4L2 userspace API as the standard UVC backend target. Do not use Windows Media Foundation abstractions on Linux.
2. Keep the root Go module on Go 1.24.8. The inspected `github.com/vladimirvivien/go4vl@v0.5.0` module declares `go 1.25`, so it cannot be added to the root module without violating the current toolchain boundary.
3. Do not add `github.com/blackjack/webcam@v0.6.2` to the root implementation yet. It is MIT-licensed and declares Go 1.18, but the inspected API has important adapter considerations: `WaitForFrame` accepts whole-second timeouts; `ReadFrame` releases the MMAP buffer before returning its byte slice (a caller must use `GetFrame`, copy the bytes, then `ReleaseFrame`); `SetImageFormat` returns the originally requested dimensions instead of the negotiated width/height; and partial failures in `StartStreaming` need cleanup scrutiny. A library wrapper would need explicit regression tests around those behaviours before adoption.
4. Add a read-only V4L2 inventory preflight. It reports character-device nodes, sysfs names, owner/group/mode, a best-effort POSIX mode-bit read/write estimate, kernel version and UVC module state. The estimate does not evaluate ACLs or Linux capabilities. It must not open camera nodes, modify permissions, load modules or start streaming.
5. Keep the V4L2 compatibility status **Untested / blocked by WSL USB passthrough**. Do not add `v4l2` as a selectable production `camera.provider`, and do not change Wails' Windows-only real-camera guard until a real Linux device passes capture-to-render-local-output and recovery tests.
6. The next environment gate is to install usbipd-win on Windows, bind and attach the B525 to WSL2, then confirm `lsusb`, `/sys/class/video4linux`, `/dev/videoN`, UVC capabilities and non-root access. Binding requires Administrator permission; while attached to WSL, the B525 is unavailable to Windows. Detach it after the Linux test.

## Validation evidence

- WSL2 Ubuntu 26.04.1 LTS x86_64; kernel `6.18.54.1-microsoft-standard-WSL2`.
- Windows PnP: B525 HD Webcam present and `Status=OK`.
- WSL: no USB bus sysfs directory, no V4L2 sysfs directory, no `/dev/video*` or `/dev/media*`.
- WSL kernel provides the `uvcvideo.ko` module file (`uvc_driver_module_available=true`); `/sys/module/uvcvideo` is absent (`uvc_driver_loaded=false`). `/sys/class/video4linux` is absent. The current account is not in the `video` group.
- Windows: `usbipd.exe` not found on PATH.
- Added `internal/booth/camera/v4l2` read-only inventory and `tools/camera-poc/v4l2-preflight`, plus unit tests. Package tests, vet and CLI build pass with root Go 1.24.8. Actual output reports zero devices; no camera node is opened.
- Read/write metadata is a best-effort POSIX mode-bit estimate; ACLs and Linux capabilities are outside its scope.
- V4L2 UAPI reference: https://docs.kernel.org/userspace-api/media/v4l/dev-capture.html
- WSL USB pass-through instructions: https://learn.microsoft.com/en-us/windows/wsl/connect-usb

## Consequences

- No camera provider is claimed or enabled on Linux yet.
- The core remains Go 1.24.8; Wails remains isolated at Go 1.25.0.
- Once a device node is available, the next implementation slice is an experimental `camera.Provider` that starts with the validated B525 formats and exact negotiated resolution, reads/copies frames safely from MMAP buffers, normalizes abbreviated MJPEG, and observes the existing workflow's staging/no-overwrite contract.
- Real Linux camera tests, physical USB recovery, image quality, preview, printer and production support remain separate acceptance gates.


## Follow-up evidence — WSL2 B525 USB/IP evaluation (2026-11-08)

The original Context and Validation evidence above describe the state before usbipd-win was installed. The following results supersede the prior "no device exposed" blocker for the WSL2 evaluation host:

- usbipd-win 5.3.0 bound/attached the B525 (046d:0836, BUSID 6-4) to WSL2 Ubuntu 26.04.1.
- Kernel 6.18.54.1-microsoft-standard-WSL2 loaded `uvcvideo`; `/sys/class/video4linux` and `/dev/video0` became available. The account has video-group read/write access (mode 0660, root:video).
- `/dev/video0` is Video Capture + Streaming. `/dev/video1` is a metadata-only node and must be excluded from candidate capture selection.
- 640x480 MJPEG @15 configured FPS passed 3/3 single-frame captures, a 30-frame burst, a 100-frame burst with 100/100 successful Pillow decodes, and ten independent process open/capture/close cycles with ten successful JPEG decodes.
- User detached/re-attached the B525 through usbipd-win and confirmed a post-reattach JPEG capture. This is a manual reconnect smoke test only.
- Effective sustained throughput observed through WSL USB/IP is about 12.3–12.4 FPS despite the 15 FPS negotiated setting. 640x480 @30, 1280x720 @15 and 1280x720 @30 produced V4L2 error-flagged buffers/timeouts in this environment. Do not generalize these results to native Linux or a direct USB host controller.
- Root Go 1.24.8 `go test -count=1 ./...` and `go vet ./...` pass on WSL. The read-only preflight reports /dev/video0 and /dev/video1; it only inventories device nodes and does not query capture capabilities.
- The remaining implementation gate is an experimental Go `camera.Provider` and in-process lifecycle/recovery validation, followed by capture→render→local-output workflow acceptance. Keep the provider out of station configuration and leave the Wails platform guard unchanged until those gates pass.


## Follow-up evidence — experimental Go provider and workflow smoke (2026-11-08)

The WSL2 reference B525 has now passed the provider and workflow smoke at 640x480 MJPEG @15 configured FPS:

- An experimental Go Linux/amd64 `camera.Provider` was added in `internal/booth/camera/v4l2`. It owns a single-planar V4L2 MMAP stream, verifies exact negotiated MJPEG dimensions, bounds polling/cancellation, rejects metadata-only nodes and error-flagged/corrupt frames, normalizes MJPEG via the shared helper, and writes only to the caller's exclusive staging path.
- ABI struct sizes, ioctl request values and field offsets were cross-checked against the WSL Linux headers. Provider unit tests cover ABI constants, basic buffer layout, JPEG validation, staging no-overwrite, cancellation and option boundaries.
- The Go provider smoke passed with warm-up, Preview and 10 captures in one process (10/10 fully decoded JPEGs).
- The workflow smoke uses the existing recovery decorator and passed 3/3 capture→render→local-output with fully decoded captures, render and final local JPEG. The workflow completed successfully on WSL2.
- The provider intentionally builds only on Linux/amd64. A non-Linux/non-amd64 stub compiles elsewhere; Linux/arm64 camera streaming is not implemented.
- A live detach/attach test was attempted through the optional hotplug workflow CLI, but usbipd remained in `Attached` state throughout the countdown. No live disconnect occurred. In-process recovery is therefore still OPEN.
- WSL tests/vet pass; Windows/amd64 root build and Linux/arm64 unsupported-stub build pass.

**Production decision remains deferred:** this provider is experimental only. Three consecutive in-process USB detach/attach recovery cycles in one workflow, the 100-cycle lifecycle smoke and 15-minute continuous-stream soak now pass on the WSL2 B525 path. Do not add `v4l2` to station config, enable Linux camera UI or relax Wails platform guards until longer-duration soak, broader hotplug campaigns, native Ubuntu direct-USB, UI and printer acceptance pass. General Linux support is not claimed.


## Superseding lifecycle result (2026-11-08)

The Go-owned provider passed 100 repeated `Open → warm-up → Preview → Capture → full JPEG decode → Close` cycles on the same provider instance with the WSL2 B525 at 640x480 MJPEG / 15 FPS requested. All 100 previews and captures decoded at the negotiated dimensions and all Close/Open transitions completed; the CLI emitted `SMOKE_PASS open_close_cycles=100 captures=100 total_elapsed=2m21.767121175s`.

A separate operator-assisted in-process hotplug test physically detached BUSID 6-4 during the first workflow countdown and reattached it after recovery-open failures appeared. V4L2 poll reported `revents 0x18`; the recovery decorator saw `/dev/video0` absent during attempts 1–7, reopened the provider on attempt 8 and emitted `recovery_succeeded`. The same workflow then captured all three shots, rendered and fully decoded the local output, and reached `completed` in 1m14.055s.

These results close the 100-cycle repeated lifecycle smoke and establish repeated in-process detach/reattach recovery for this specific WSL2 USB/IP + B525 path. A three-cycle operator-assisted campaign ran in one workflow process: poll reported `revents 0x18` on each detach, the recovery decorator reopened at attempts 9, 7 and 10 respectively, and CLI emitted `HOTPLUG_RECOVERY_PASS requested_cycles=3 observed_successes=3`. All three captures, render and local output then passed and workflow exited 0 in 2m58.011s. Separately, a 3-minute soak passed first, followed by a 15-minute continuous-stream soak on one open provider stream: 1,800/1,800 Preview frames and 1,800/1,800 captured JPEGs fully decoded at 640x480; one Open and one Close; elapsed 15m0.0106s; CLI emitted `STREAM_SOAK_PASS` and exited 0. This does not establish full-shift endurance, repeated-day hotplug reliability, native Ubuntu direct-USB support or general Linux compatibility. Production config/UI enablement remains deferred.


## Follow-up evidence — WSL2 kernel 6.18.58 mode acceptance and smoke cleanup (2026-11-11)

The B525 was attached to Ubuntu WSL2 through usbipd-win 5.3.0 (BUSID 6-4). The test host was Ubuntu 26.04.1 / WSL2 kernel 6.18.58.1-microsoft-standard-WSL2, x86_64. Read-only preflight confirmed `uvcvideo` loaded, `/dev/video0` as Video Capture + Streaming, `/dev/video1` as metadata-only, and mode 0660 root:video with the operator in the `video` group. `v4l2-ctl --list-formats-ext` enumerated YUYV and MJPEG modes, but enumeration did not imply successful streaming.

- 640x480 MJPEG @15: provider smoke passed 3/3 captures; three-shot workflow passed capture→render→local-output.
- 1280x720 MJPEG @5: provider smoke passed 3/3 captures; three-shot workflow passed capture→render→local-output; same provider instance passed 10/10 Open→warm-up→Preview→Capture/full-JPEG-decode→Close cycles in 16.49 seconds.
- 1280x720 MJPEG @10 and @15, and 640x480 MJPEG @30: each failed during warm-up after five consecutive buffers marked `V4L2_BUF_FLAG_ERROR`.
- 1920x1080 MJPEG @5 appeared in format enumeration but was not streamed/tested.
- The 1280x720 @5 result is short-run WSL2 USB/IP evidence only; it has not passed a longer soak or hotplug campaign at that mode.

The provider smoke CLI previously used `os.Exit(1)` in its failure helper, which bypassed deferred cleanup and left empty temp directories after early failures. It now returns errors from `run()`; `main()` exits only after `run()` has returned and deferred provider Close / temporary-directory removal have executed. A deliberate missing-device invocation returned the expected failure and left no provider-smoke temp directory. Three empty temp directories from earlier failed mode attempts were inspected (directory entries only) and removed with `rmdir`. Final checks found no process holding `/dev/video0` or `/dev/video1`, and no provider/workflow smoke temp directories.

Validation after the cleanup change:
- Root Go 1.24.8 `go test -count=1 ./...`: PASS.
- Root Go 1.24.8 `go vet ./...`: PASS.
- Linux/amd64 CGO-disabled root build: PASS.
- Windows/amd64 CGO-disabled cross-build: PASS.
- Provider smoke package compile/test: PASS.
- `gofmt -d` for the changed smoke source: no output.
- `git diff --check`: PASS.

These are WSL2 USB/IP + B525 results only. Native Ubuntu direct-USB validation, 1280x720@5 long soak/hotplug, repeated hotplug across separate sessions/days, printer and production preview/UI acceptance remain OPEN. The provider remains experimental and unselectable in StoreConfig; Wails camera platform guards remain unchanged. B525 is left attached to WSL, so Windows applications cannot use it until the operator detaches it. No commit or push was performed.


## Follow-up evidence — 1280x720@5 15-minute soak and in-process hotplug (2026-11-11)

Environment: ShinPc, Ubuntu 26.04.1 WSL2 kernel 6.18.58.1-microsoft-standard-WSL2, usbipd-win 5.3.0, Logitech B525 BUSID 6-4 attached to WSL. Mode: MJPEG 1280x720, 5 FPS requested, four MMAP buffers, one-second warm-up, five-second frame timeout. This is WSL2 USB/IP evidence, not native Ubuntu direct-USB.

**Continuous stream soak:**
- Command used `v4l2-provider-smoke -device /dev/video0 -width 1280 -height 720 -fps 5 -stream-soak=15m -stream-soak-interval=500ms`.
- CLI emitted `STREAM_SOAK_PASS target_duration=15m0s actual_elapsed=15m0.009052375s open_close_cycles=1 captures=1800 previews=1800`.
- All 1,800 preview JPEGs and 1,800 capture JPEGs were fully decoded and verified at 1280x720. One provider Open and one Close. Process exit code 0.

**Physical in-process hotplug recovery:**
- Ran the 3-shot workflow with `-test-hotplug -hotplug-countdown=20s -hotplug-cycles=1` at 1280x720@5.
- During shot 1 countdown, usbipd detached BUSID 6-4. V4L2 poll reported `revents 0x18`; recovery open attempt 1 saw `/dev/video0` absent. After usbipd reattached BUSID 6-4, recovery attempts 2–6 still found no node; attempt 7 opened the provider and emitted `recovery_succeeded`.
- The same workflow then completed shots 1–3, rendered the output and completed local output. CLI emitted `HOTPLUG_RECOVERY_PASS requested_cycles=1 observed_successes=1` and `WORKFLOW_PASS`; workflow exit code 0, elapsed 1m12.720s.
- Post-run inspection found no process holding `/dev/video0` or `/dev/video1`, and no provider/workflow smoke temp directories. The B525 remained Attached to WSL.

The 1280x720@5 mode now has a 15-minute soak and one confirmed in-process physical recovery cycle. Repeated hotplug cycles/campaigns at this mode and full-shift endurance remain open. Native Ubuntu direct-USB, printer and Linux preview/UI acceptance remain separate gates. The V4L2 provider remains experimental and is not selectable in StoreConfig; Wails platform guards remain unchanged.


## Follow-up evidence — 1280x720@5 three-cycle hotplug campaign (2026-11-12)

Environment: ShinPc, Ubuntu 26.04.1 WSL2 kernel 6.18.58.1-microsoft-standard-WSL2, usbipd-win 5.3.0, Logitech B525 BUSID 6-4 attached to WSL. Mode: MJPEG 1280x720, 5 FPS requested. This remains WSL2 USB/IP evidence, not native Ubuntu direct-USB.

A six-shot diagnostic workflow with a temporary six-slot frame was used to provide enough capture boundaries for three consecutive recovery events. A temporary Windows PowerShell orchestrator watched the workflow event stream, detached BUSID 6-4 on each designated countdown, waited for the first failed recovery-open event, reattached the B525 and waited for `recovery_succeeded` before the next detach. Both the frame and orchestrator lived under Windows TEMP and were removed after the test.

- Workflow command: `v4l2-workflow-smoke -device /dev/video0 -width 1280 -height 720 -fps 5 -shots 6 -frame <temporary-six-slot-frame> -test-hotplug -hotplug-countdown=10s -hotplug-cycles=3`.
- All three detaches produced V4L2 poll `revents 0x18`; `/dev/video0` disappeared from WSL. Each recovery then opened the provider at attempt 4/60 after usbipd reattach and emitted `recovery_succeeded`.
- Orchestrator counted 1/3, 2/3 and 3/3 actual `recovery_succeeded` events. CLI emitted `HOTPLUG_RECOVERY_PASS requested_cycles=3 observed_successes=3`.
- Workflow reached `completed`, decoded and validated all 6 captures, rendered the six-slot frame, validated local output and emitted `WORKFLOW_PASS`; workflow elapsed 1m15.91152531s and process exit code 0. Orchestrator exit code 0, observed_recoveries=3.

The first attempted three-shot run yielded only one actual recovery because recovery is triggered on Capture and a frame can still be dequeued from buffers after disconnect; a detach during the final shot has no subsequent capture to trigger another recovery. The six-shot diagnostic workflow provided enough capture boundaries to verify three recoveries within the same workflow. This is a test-harness detail, not a change to the production workflow.

The workflow-smoke timeout was also corrected. Its prior fixed 45-second recovery allowance was insufficient for multiple events. The CLI now computes the bounded retry budget from the configured retry schedule: 60 attempts, 500ms linear delay capped at 2s, totaling 115 seconds per recovery. Total hotplug timeout is `cycles * (countdown + retry budget + 15s per-cycle overhead) + 60s`; for this run it printed 8m0s. Added unit tests cover the budget calculation and edge cases.

After the run, B525 remained Attached to WSL; `/dev/video0` and `/dev/video1` were present, neither had a process owner, and no provider/workflow smoke temp directories remained. The temporary six-slot frame, temporary orchestrator and accidental root build binaries were removed. Native Ubuntu direct-USB, repeated campaigns across sessions/days, full-shift endurance, printer and Linux preview/UI acceptance remain OPEN. V4L2 remains experimental and unselectable in StoreConfig; Wails platform guards remain unchanged.
