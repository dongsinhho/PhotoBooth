# ADR 0030: Gate Linux V4L2 provider implementation on a real device path

- **Status:** V4L2 selected as the Linux API; provider implementation deferred until a V4L2 device is exposed to the evaluation host.
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
