# ADR 0015: Versioned Local Station Configuration

- **Status:** Accepted
- **Date:** 2026-10-29
- **Decision owners:** PhotoBooth engineering

## Context

The first capture-to-output workflow is configurable through CLI flags, but the defaults and flags are not a durable station profile. As the app adds a settings UI, it needs one local contract for camera selection, capture timing, frame path, local storage, media delivery policy, and payment policy.

This next increment must remain offline-first and must not pretend unimplemented cloud or payment features work.

## Decision

Introduce a strict JSON station configuration in the root Go module at `internal/booth/config`, versioned with a required `schema_version`.

Schema version 1 includes:

- Camera provider, device selector, frame size and FPS.
- Capture count, countdown and inter-shot delay.
- Declarative frame JSON path.
- Temporary session directory and finalized local-output directory.
- Media-delivery mode.
- Payment mode.

The current default policy is synthetic camera, local media delivery and disabled/free payment. The example config uses the synthetic camera so it cannot open hardware accidentally. Real camera use must be explicitly selected with `--camera=windowsmf`.

Config loading is strict and bounded: maximum 64 KiB, unknown fields rejected, one JSON document required, schema version checked, numeric/duration boundaries validated, and unsupported payment/media modes rejected. Missing fields inherit documented defaults. Relative frame/storage paths resolve against the configuration file's directory.

The CLI accepts `--config <path>`. Only command-line flags the operator explicitly provided override the file; omitted flags preserve the file values. When `--config` is not used, existing CLI defaults and flag usage remain supported.

Frame JSON validation, file existence and capture-count/frame-slot matching remain delegated to the existing renderer/workflow preflight rather than duplicated in the settings loader.

## Consequences

- A station can now define repeatable capture and local-output behavior without hard-coding these values into a camera provider.
- The same contract can later back a UI settings page, installer defaults and migration checks.
- Schema versions provide a place to introduce compatible migrations rather than silently reinterpreting settings.
- Cloud/LAN delivery and payment execution are not implemented by this ADR. Their non-default modes currently fail validation.
- At the time of this ADR, the Wails prototype still used fixed settings. The scoped follow-up integration is recorded in [ADR 0016](0016-wails-local-settings-screen.md); the production UI framework remains undecided.
- Relative paths are convenient for portable station profiles, but they are not a sandbox. The operator must control the config and referenced local files.

## Validation

- Unit tests cover defaults, partial configs, relative paths, required/unsupported versions, unknown fields, trailing documents, size limits, durations, invalid ranges and unsupported modes.
- CLI tests cover config loading, explicit-flag precedence, legacy no-config behavior and rejection of unsupported modes.
- A synthetic-camera CLI smoke run using `examples/store-config-v1.json` completed capture, render and local save.
- Root Go tests/vet, Windows build and Linux/amd64 compile-only build passed for the implementation.
