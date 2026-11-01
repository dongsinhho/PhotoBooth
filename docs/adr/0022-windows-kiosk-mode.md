# ADR 0022: Opt-in Windows fullscreen kiosk mode with keyboard escape

**Status:** Accepted for Windows MVP candidate evaluation only; Windows shell/Assigned Access and production kiosk hardening remain out of scope.
**Date:** 2026-10-31

## Context

The Windows UI candidate currently opens as a normal resizable desktop window. The roadmap requires validating full-screen/kiosk behavior before deciding whether the candidate is suitable for a booth operator. Forcing fullscreen for every launch would make development and operator configuration less convenient and could leave a user feeling trapped in the application.

Wails v2.17 provides native fullscreen and unfullscreen runtime methods. The application can therefore switch modes without changing its Go core or simulating fullscreen by resizing the window.

## Decision

1. Keep windowed mode as the default.
2. Add an explicit root launcher switch `-Kiosk`. The launcher sets `PHOTOBOOTH_KIOSK=1` only in the launched process environment; normal launches explicitly set it to `0`, and the calling PowerShell environment is restored after process creation.
3. When requested, the Wails host starts fullscreen using its native `Fullscreen` option. Benchmark mode always overrides kiosk mode and remains hidden/windowed.
4. F11 toggles native fullscreen from either mode. Esc exits fullscreen without quitting PhotoBooth, including when a text/select control has focus. Esc has priority as the kiosk escape path.
5. Display a persistent, non-interactive hint while fullscreen is active: `KIOSK MODE | ESC TO EXIT | F11 TO TOGGLE`.
6. Do not attempt to disable Windows system shortcuts, task switching, secure attention, taskbar policy, account access, or shell behavior. This is application fullscreen, not a secured Windows kiosk account. OS-level Assigned Access/Shell Launcher and login auto-start are separate deployment tasks.
7. Validate startup bounds and both keyboard transitions with a disposable Windows UI Automation smoke test. Do not create test photo sessions or touch real guest outputs.

## Consequences

- Operators can launch fullscreen for a booth while retaining a normal windowed development/maintenance mode.
- There is an explicit keyboard route out of fullscreen, and Escape never terminates the app.
- Application fullscreen can be demonstrated on the reference Windows machine, but it does not prove Windows shell lockdown, automatic startup, multi-monitor placement, touch usability, or production security.
- The feature remains isolated in the Wails Windows candidate; Go core contracts and toolchain versions do not change.

## Validation

Run `tools/ui-spike/wails-v2/ui-kiosk-mode-smoke.ps1` after a fresh root-level build. The script verifies fullscreen at startup, Escape to windowed mode, F11 to fullscreen, and Escape again to windowed mode. It uses a temporary profile, performs no captures, and retains a JSON report under `%TEMP%`.


## Validation evidence (2026-10-31)

- Wails `go test -count=1 ./...` and `go vet ./...` passed after adding the kiosk methods and their tests.
- The PowerShell launcher and kiosk smoke script parsed successfully.
- Wails bindings, frontend/Vite production bundle and the Windows/amd64 application compiled through the root `run-photobooth.ps1 -NoLaunch` path.
- `ui-kiosk-mode-smoke.ps1` passed all four checks on the physical Windows 11 primary display: fullscreen on startup, Escape to windowed, F11 to fullscreen, Escape to windowed again. Temporary data was cleaned; no photo session was started.
- The root launcher was separately exercised with `-SkipBuild -Kiosk`: the actual child process opened fullscreen, showed the kiosk hint, then Escape restored windowed mode without closing the app.
- Smoke report: `%TEMP%\photobooth-kiosk-smoke-61b1b02bca074449b0c3cee0c1fbecc9.json`.

This does not validate Windows Assigned Access/Shell Launcher, OS shortcut lockdown, login auto-start, multiple monitors, touch-only maintenance, long-duration kiosk operation, installer/update behavior, or production release readiness.
