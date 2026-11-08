# Linux V4L2 preflight

This read-only diagnostic checks whether the current Linux environment exposes V4L2 camera device nodes to PhotoBooth.

From the repository root:

~~~bash
go run ./tools/camera-poc/v4l2-preflight
~~~

Exit codes:
- `0`: at least one V4L2 character device is visible and the POSIX mode-bit estimate indicates read/write.
- `2`: no `/dev/videoN` character devices were found.
- `3`: one or more visible video devices have a mode-bit estimate that denies read/write.
- `1`: the report could not be encoded.

The JSON report includes Linux kernel/architecture, `uvcvideo` module loaded/available state, `/sys/class/video4linux` presence, each `/dev/videoN` node's sysfs name/owner/group/mode and a best-effort read/write estimate from POSIX owner/group/other mode bits. This estimate does not account for ACLs or Linux capabilities.

**Safety:** the diagnostic does not open camera device nodes, call V4L2 ioctls, change permissions/groups, load kernel modules, or start a stream. It is an inventory/preflight only, not proof that a camera can capture.

For WSL2, USB devices are not passed through automatically. Follow Microsoft's [WSL USB connection guide](https://learn.microsoft.com/en-us/windows/wsl/connect-usb) to install usbipd-win, bind a USB device (Administrator required), and attach it to WSL. The attached device is unavailable to Windows until detached. Re-run this preflight after attachment, then validate the exact device/format before claiming support.
