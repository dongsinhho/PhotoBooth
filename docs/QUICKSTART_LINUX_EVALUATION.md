# PhotoBooth Linux UI evaluation (Ubuntu 26.04.1 LTS)

**Status:** WSL2 Ubuntu 26.04.1 Linux/amd64 build, native Go tests, Wails GUI startup, Settings/Cancel and local mount checks verified on 2026-11-08. Camera, printer and production support remain unvalidated.
**Target:** Ubuntu 26.04.1 LTS, amd64 (x86_64), Wails v2.17.0, Go 1.25.0, GTK3 + WebKit2GTK 4.1.
**Scope:** Build and launch the isolated Wails Windows Preview candidate on Linux. This is not a camera, printer, kiosk, installer or production-support claim.

## Verified WSL evaluation (2026-11-08)

The following gates passed on ShinPc using WSL2 + WSLg:

- Ubuntu 26.04.1 LTS, x86_64; WSL 2 / WSLg; kernel 6.18.54.1-microsoft-standard-WSL2.
- Root core Go 1.24.8: `go test -count=1 ./...`, `go vet ./...` and `go build ./...` passed. The full root test suite passed both from the distro ext4 working copy and from the original checkout at `/mnt/c`.
- Isolated Wails module Go 1.25.0, Node.js 22.23.4, npm 10.9.9 and Wails CLI v2.17.0. Wails module tests and vet passed with `-tags webkit2_41`.
- GTK3 `3.24.52`; WebKit2GTK 4.1 `2.52.6`. `npm ci` completed with zero reported vulnerabilities.
- `wails build -clean -platform linux/amd64 -tags webkit2_41 -o PhotoBooth` passed. The result is an ELF64 x86-64 executable (10,520,120 bytes); `ldd` resolved WebKit2GTK 4.1, GTK3 and all linked libraries with no missing dependency.
- WSLg rendered the home screen and Settings. Keyboard navigation selected Cancel; Cancel returned to home, the isolated config SHA-256 remained unchanged, and the app exited through X11 WM_DELETE_WINDOW. Temporary test data was removed. No photo session or camera capture was started.
- Mount probe: `/tmp` (tmpfs) passed local-storage validation; `/mnt/c` (9p) was correctly rejected for local-only media storage; a normal file child passed identity validation and a redirected file symlink was rejected.

**Known caveats:** Wails Doctor reports a generic `libwebkit` dependency as missing on this target, although `pkg-config webkit2gtk-4.1` resolves and the actual Wails build succeeds. WSLg smoke used X11 with software GL. The app still carries its existing “Windows Preview” title/branding. None of these checks validates a physical Ubuntu desktop, Linux camera provider, printer, kiosk deployment or production support.

The Linux working checkout used for native build/tests is under the distro ext4 home at `/home/hndsinh/PhotoBooth-linux-eval`. The Windows checkout is mounted at `/mnt/c` using 9p; keep guest sessions/photos under the distro ext4 filesystem rather than the Windows mount.

## Why this target

Ubuntu 26.04.1 LTS is the first selected Linux evaluation target. Ubuntu's release table lists standard support through May 2031. The Ubuntu Resolute package index contains \`libwebkit2gtk-4.1-dev\` for amd64.

Wails v2.17.0 supports Linux AMD64 and documents the \`webkit2_41\` build tag for current Linux distributions using WebKit2GTK 4.1. The pinned local Wails v2.17.0 source uses \`pkg-config webkit2gtk-4.1\` when this tag is enabled and \`gtk+-3.0\` for GTK.

## 1. Host preflight

Run on a graphical Ubuntu 26.04.1 LTS amd64 desktop session. Record the exact image, kernel and architecture before installing dependencies:

~~~bash
cat /etc/os-release
uname -a
uname -m
~~~

Expected architecture: \`x86_64\`.

The Wails GUI needs a working desktop/display session (X11 or Wayland). A headless shell alone is not a UI launch test.

## 2. Native build dependencies

Install the GTK3/WebKit2GTK 4.1 development stack and common compiler tools:

~~~bash
sudo apt update
sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
~~~

Verify pkg-config can resolve the exact ABIs selected by this evaluation:

~~~bash
pkg-config --modversion gtk+-3.0
pkg-config --modversion webkit2gtk-4.1
~~~

Do not substitute WebKit2GTK 4.0 or omit the \`webkit2_41\` build tag. The pinned Wails v2.17.0 source selects the 4.1 pkg-config module only when that tag is supplied.

## 3. Go, Node.js and Wails CLI

The Wails module is intentionally isolated from the root Go 1.24.8 core. Use Go 1.25.0 for this module and keep the root \`go.mod\` unchanged.

Install/activate Go 1.25.0 and a supported Node.js/npm toolchain, then verify:

~~~bash
go version
node --version
npm --version
~~~

Install the pinned Wails CLI (not \`@latest\`) and verify it:

~~~bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
export PATH="$(go env GOPATH)/bin:$PATH"
wails version
wails doctor
~~~

On Ubuntu 26.04.1 with Wails v2.17.0, \`wails doctor\` currently reports fatal \`Required dependencies missing: libwebkit\` (\`Package Name Unknown\`, \`Not Found\`) even though \`pkg-config --modversion webkit2gtk-4.1\` returns \`2.52.6\`. The tagged Linux build succeeds and \`ldd\` links \`libwebkit2gtk-4.1.so.0\`. Treat this as a Doctor detection mismatch for the 4.1 module; do not substitute WebKit2GTK 4.0 or drop \`webkit2_41\`. Retain the Doctor output, and verify the exact pkg-config module, tagged build and linked libraries directly.

## 4. Build the isolated Wails module

From the repository checkout:

~~~bash
cd tools/ui-spike/wails-v2
npm ci
wails build -clean -platform linux/amd64 -tags webkit2_41 -o PhotoBooth
~~~

Expected output: \`build/bin/PhotoBooth\` (Linux executable). Confirm its architecture and linked libraries:

~~~bash
file build/bin/PhotoBooth
ldd build/bin/PhotoBooth
~~~

Investigate any \`not found\` library reported by \`ldd\`; do not bundle or copy arbitrary libraries to hide missing system dependencies.

The build must remain in this nested module. Do not add Wails dependencies to the root \`go.mod\` or change the root Go toolchain.

## 5. Basic UI launch smoke (no camera capture)

Run from a graphical desktop session with a disposable data directory:

~~~bash
test_data="$(mktemp -d)"
PHOTOBOOTH_DATA_DIR="$test_data" ./build/bin/PhotoBooth
~~~

Verify manually:
- The application window opens and renders the home/idle screen.
- The operator Settings & frame view opens and can be cancelled.
- The application exits normally.
- The disposable data directory contains only test settings/frame files and can be removed after review.

After the app closes:

~~~bash
rm -rf "$test_data"
~~~

Do not start a photo session in this first Linux UI smoke.

## 6. V4L2 device preflight (read-only)

The Linux camera target is the kernel V4L2 userspace API. Before writing or enabling a Linux camera provider, confirm that the target OS actually exposes a video character device and that the process can access it.

Run from the repository root:

~~~bash
go run ./tools/camera-poc/v4l2-preflight
~~~

The command prints a JSON inventory containing:
- Kernel, architecture and process UID/GID/groups.
- Whether the `uvcvideo` module is loaded or its module file is present.
- Whether `/sys/class/video4linux` exists.
- Each `/dev/videoN` character node, sysfs name, owner, group and mode.
- A best-effort read/write estimate from POSIX owner/group/other mode bits. This estimate does not evaluate ACLs or Linux capabilities.

Exit codes: 0 means a visible device has estimated read/write access; 2 means no `/dev/videoN` character device exists; 3 means devices exist but the mode-bit estimate denies read/write; 1 means the report could not be produced. The tool does not open device nodes, call V4L2 ioctls, change groups/permissions, load kernel modules or start a stream.

### WSL2 USB pass-through prerequisite

Windows PnP detection alone does not expose a USB webcam inside WSL2. The 2026-11-08 ShinPc preflight found the B525 present in Windows, but WSL had no USB bus sysfs, no `/sys/class/video4linux`, and no `/dev/videoN`; the UVC module file existed but was not loaded. `usbipd-win` was not installed.

To make the device available to WSL, follow Microsoft's [Connect USB devices](https://learn.microsoft.com/en-us/windows/wsl/connect-usb) guide. The guide offers this Windows Package Manager installation command (use the interactive option so any required restart is not triggered silently):

~~~powershell
winget install --interactive --exact dorssel.usbipd-win
~~~

The typical workflow is:

1. Install usbipd-win on Windows (for example, from an elevated PowerShell using the official installation instructions).
2. In an Administrator PowerShell, list devices and bind the selected bus ID:
   ~~~powershell
   usbipd list
   usbipd bind --busid <BUSID>
   ~~~
3. In a normal PowerShell, attach that bus ID to WSL:
   ~~~powershell
   usbipd attach --wsl --busid <BUSID>
   ~~~
4. In Ubuntu, verify the USB device and V4L2 node, then rerun the read-only preflight:
   ~~~bash
   lsusb
   ls -l /dev/video*
   go run ./tools/camera-poc/v4l2-preflight
   ~~~
5. When finished, detach the device from WSL:
   ~~~powershell
   usbipd detach --busid <BUSID>
   ~~~

Binding requires Windows Administrator permission. While the camera is attached to WSL it is unavailable to Windows applications, including the Windows PhotoBooth build. Do not bind/attach during an active Windows camera test. The current preflight found no device node, so no Linux stream/capture test was attempted. Keep the provider unselected until a real V4L2 node is visible and exact format negotiation, streaming, staging, capture-to-render-local-output and recovery are tested.

See [the V4L2 preflight README](../tools/camera-poc/v4l2-preflight/README.md) and [ADR 0030](adr/0030-linux-v4l2-backend-evaluation.md).

## 7. Important current limitations

- The real webcam preview and capture provider in the Wails candidate are Windows Media Foundation only. On Linux, the UI may report camera preview/capture as unavailable; this is expected and must not be recorded as a Linux camera failure or success.
- The root Go core may compile on Linux, but that does not prove a Linux camera provider exists.
- Physical printer output, integrated laptop cameras, USB recovery, full-screen kiosk behavior, auto-start, package installation/update, image quality and sustained operation are not validated by this guide.
- The Windows ZIP installer/active-pointer scripts are Windows-specific and are not a Linux deployment solution.

## 8. Evidence to record

Record:
- Ubuntu release, kernel, architecture and desktop/display session.
- Go, Node.js, npm and Wails CLI versions.
- GTK3 and WebKit2GTK 4.1 pkg-config versions.
- \`wails doctor\` output.
- Exact build command and exit code; \`file\` and \`ldd\` output.
- Whether the window opened, Settings rendered, and the app exited normally.
- Any errors with secrets and personal paths redacted.

A successful build is compile evidence only. A successful UI launch is a basic shell smoke only. Do not mark Linux camera, printer or production compatibility as passed without separate hardware tests.

## References

- Ubuntu release table: https://ubuntu.com/project/docs/release-team/list-of-releases/
- Wails v2.17.0 installation guide: https://v2.wails.io/docs/gettingstarted/installation/
- Ubuntu 26.04 \`libwebkit2gtk-4.1-dev\`: https://packages.ubuntu.com/resolute-updates/libwebkit2gtk-4.1-dev
