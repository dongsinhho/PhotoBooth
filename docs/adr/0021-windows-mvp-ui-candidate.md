# ADR 0021: Make Wails the runnable Windows MVP UI candidate

**Status:** Accepted for Windows-first MVP evaluation only. Production UI approval and cross-platform support remain open.
**Date:** 2026-10-31

## Context

The Go core and CLI workflow are runnable, but the CLI is not a guest-facing kiosk interface. The Wails shell already exercises the real Go session runner, Windows Media Foundation camera, frame renderer, local output, settings/frame-package lifecycle, recovery flows, output retry, and idle-only live camera preview. However, the shell was hidden under `tools/ui-spike/wails-v2`, visibly labeled as a spike, and had no root-level launch command. That made it easy to run the CLI or an older window and incorrectly conclude that the latest UI work had not changed the application.

ADR 0012 intentionally isolated Wails v2.17.0 because it requires Go 1.25.0, while the root/core module remains on Go 1.24.8. That toolchain boundary remains valid.

## Decision

1. Use the existing Wails shell as the **runnable Windows MVP UI candidate** for the next product-validation steps. This is not a final production-framework selection.
2. Keep the Wails module and its Go 1.25.0 toolchain isolated under `tools/ui-spike/wails-v2`. Do not add Wails dependencies to the root module or raise the core Go version.
3. Add a root-level `run-photobooth.ps1` entry point. By default it builds a distinct `PhotoBooth.exe` and launches it. It supports build-only and development modes, checks the nested module's selected toolchain, and refuses to start a second instance against the same local profile/camera. It never kills an existing UI process.
4. Identify the visible window as **PhotoBooth · Windows Preview**, and label the shell **Windows Preview** rather than presenting it as a generic production release. The Idle screen owns live preview; Settings and capture stop preview and release the camera.
5. Continue to treat Linux UI, physical printer, kiosk auto-start/full-screen, long-running preview, representative portrait quality, and additional cameras as unvalidated. Do not advertise those capabilities based on a Windows build.
6. Keep ADR 0012's comparison/exit requirements active. Final production adoption requires target-hardware measurements, install/update behavior, touch/full-screen/kiosk testing, Linux target evaluation if still required, and review of support/runtime costs.

## Consequences

- There is one documented root command for launching the current Windows UI candidate; users do not need to navigate into the nested module or accidentally run the headless CLI when looking for a GUI.
- The Wails candidate remains independently buildable and does not change the core toolchain contract.
- A successful build or single-device UI smoke remains feasibility evidence, not production certification.
- The candidate can now be evaluated as an actual visible player before committing to a long-term UI framework.
