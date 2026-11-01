# ADR 0016: Connect Local Station Settings to the Wails Player Prototype

- **Status:** Accepted for the UI spike; not a production UI commitment
- **Date:** 2026-10-29
- **Decision owners:** PhotoBooth engineering

## Context

ADR 0015 introduced a versioned, validated station profile in the root Go core and CLI. The Wails player still used fixed shot count, countdown, interval and frame-path values, which risked diverging from the local configuration contract.

The next increment should let an operator change the supported capture behavior without editing source code, opening camera controls, or introducing network services. Wails remains an isolated evaluation spike because its toolchain/runtime costs are not yet accepted as the production UI decision.

## Decision

Add an operator-only **Settings & frame** view to `tools/ui-spike/wails-v2`, backed by the root `internal/booth/config` package.

The UI exposes only the current supported capture settings:

- Shot count, 1–12.
- Countdown and inter-shot interval as Go duration strings (for example `3s`, `250ms`, or `0s`).
- Local frame JSON path, with a native file picker.

The station profile is stored at `%LOCALAPPDATA%\PhotoBooth\store-config-v1.json` on Windows. The app's default frame and session/output directories are under the same local application data folder. The profile uses schema v1; paths inside the profile are relative to its directory when serialized, then resolved when loaded.

Before saving, the player validates the station config, parses both durations, loads the selected frame with the existing renderer and requires the frame slot count to equal the configured shot count. Saving uses the config package's atomic replacement path. Settings are not saved while a session is active. A corrupt or unsupported existing profile is not silently overwritten; the settings view surfaces a warning and capture remains blocked until an operator saves a valid replacement.

The Wails app always needs the real camera provider for its real-camera action; saving the UI settings selects `windowsmf`. The synthetic demo still uses the fake camera provider. Media delivery remains local and payment remains disabled; LAN/cloud/payment options are not exposed in this UI.

The app dynamically renders the capture-progress rows and session counts using the saved shot count. This means the setting reaches the real workflow, not just a frontend label.

## Consequences

- Capture count, countdown and interval survive application restarts in a local station profile.
- A custom frame must be a valid schema-v1 JSON file with exactly the configured number of slots. The UI does not modify arbitrary frame contents.
- A local JSON path is a trusted local-file reference, not a filesystem sandbox; only operators should be able to change the profile or selected frame files on a shared kiosk.
- The settings view does not edit camera device selectors, resolution/FPS, payment or media-delivery policy.
- The Wails prototype still does not provide continuous live preview, physical printer support, config hot reload, settings-profile migration, or Linux runtime certification.
- No camera is opened and no photo is captured just by viewing or saving settings.

## Validation

- Wails unit tests cover save/reload persistence, slot-count mismatch rejection, invalid-profile repair without silent overwrite, blocking setting changes during active sessions, and a two-shot synthetic workflow using a two-slot frame.
- The variable-shot workflow test passed 20 consecutive runs.
- UI Automation smoke opened the built Wails settings view, verified the shot selector, countdown, interval, frame-path field, local profile path and Save/Cancel/Browse controls, then cancelled without saving.
- Wails Go tests/vet, frontend production build and Windows production build passed.
- The root Go suite, vet, Windows build and Linux/amd64 compile-only check remain separate core validation gates.
