# PhotoBooth Linux UI evaluation (Ubuntu 26.04.1 LTS)

**Status:** Evaluation instructions only; Linux UI build/runtime is not yet validated.
**Target:** Ubuntu 26.04.1 LTS, amd64 (x86_64), Wails v2.17.0, Go 1.25.0, GTK3 + WebKit2GTK 4.1.
**Scope:** Build and launch the isolated Wails Windows Preview candidate on Linux. This is not a camera, printer, kiosk, installer or production-support claim.

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

\`wails doctor\` should report the Linux dependencies as available. Resolve any missing dependencies before building; retain its output with the evaluation notes.

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

## 6. Important current limitations

- The real webcam preview and capture provider in the Wails candidate are Windows Media Foundation only. On Linux, the UI may report camera preview/capture as unavailable; this is expected and must not be recorded as a Linux camera failure or success.
- The root Go core may compile on Linux, but that does not prove a Linux camera provider exists.
- Physical printer output, integrated laptop cameras, USB recovery, full-screen kiosk behavior, auto-start, package installation/update, image quality and sustained operation are not validated by this guide.
- The Windows ZIP installer/active-pointer scripts are Windows-specific and are not a Linux deployment solution.

## 7. Evidence to record

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
