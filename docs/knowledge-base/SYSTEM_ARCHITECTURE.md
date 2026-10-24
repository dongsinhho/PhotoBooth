# System Architecture

**Status:** Proposed target architecture; not yet implemented

## Architecture approach
Start with a modular desktop application and, when needed, a modular-monolith cloud API. Avoid distributing functionality across services until operational or scaling evidence requires it.

The local player owns all time-sensitive capture work. Cloud services distribute content and manage accounts, commerce, and publishing; they must not be required for a configured photo session.

## Windows player modules

### 1. Kiosk UI / Theme Host
Displays start screen, instructions, countdown, selection, preview, and completion screens. Theme data may change presentation but must not own or bypass capture workflow.

### 2. Session Orchestrator
Owns session state, cancellation, timeouts, retry rules, and transitions. Coordinates camera, rendering, and output through explicit interfaces.

Initial state proposal:
Idle -> Preparing -> Capturing -> Reviewing/Rendering -> Output -> Completed
with explicit error, cancellation, recovery, and cleanup transitions. Validate the final states against real UX before treating them as fixed.

### 3. Camera Adapter
Wraps one vendor SDK or camera integration. Normalizes connection, live view, capture, transfer, timeout, and disconnect behavior. Vendor SDK types should not leak into domain or UI code.

### 4. Render Pipeline
Converts source images and a frame definition into deterministic output. Define coordinate systems, slots, layer order, asset sizing, color handling, output resolution, and failure reporting.

### 5. Print / Export
Owns output sizing, printer interaction or file export, retries, and output status. It must not require an active Internet connection.

### 6. Local Persistence and Asset Cache
SQLite is a candidate for settings, session metadata, installed package versions, and sync state. Filesystem storage holds photos and content. Define retention, cleanup, and disk-full behavior.

### 7. Sync Agent
Runs outside the critical capture path. Downloads metadata/content, validates and stages updates, then activates them safely. Previously installed content remains usable when cloud sync fails.

## Cloud modules (later phase)
- Identity and store/device registration.
- Catalog, categories, tags, search, and metadata.
- Asset upload, validation, preview generation, versioning, publishing, and moderation.
- Orders, payments, entitlements, refunds, and licensing.
- Creator profiles and payout records.
- Signed content delivery and client update metadata.
- Operational telemetry that avoids collecting guest photos by default.

Potential stack: ASP.NET Core or Go API, PostgreSQL, S3-compatible object storage, and CDN. These are candidates, not accepted implementation decisions.

## Content delivery flow
1. Player requests catalog/update metadata when online.
2. Player downloads an immutable package version.
3. Player validates manifest, compatibility, file sizes, hashes/signature, and entitlement where relevant.
4. Player stages package in a temporary location.
5. Player activates only after full validation and outside an active session.
6. The prior known-good version remains available for rollback.

## Reliability boundaries
- No network request blocks camera capture or printing.
- UI remains responsive during device/file I/O.
- Hardware failures produce actionable errors and safe state transitions.
- Corrupt or incompatible packages cannot replace working content.
- Logs use session IDs and technical codes, not photos or unnecessary guest data.

## Candidate technology direction

| Area | Candidate | Status / validation |
|---|---|---|
| Windows desktop | C#/.NET + WPF | Proposed; validate kiosk, packaging, maintenance |
| Configurable UI | WebView2 + declarative theme schema | Proposed; test performance/offline/security |
| Camera | Vendor-specific adapter | First model and SDK undecided |
| Image processing | .NET or suitable native/library component | Select after quality, speed, license tests |
| Local database | SQLite | Proposed |
| Backend | ASP.NET Core or Go | Undecided |
| Cloud database | PostgreSQL | Candidate |
| Asset storage | S3-compatible storage + CDN | Candidate |
| Payments | Provider meeting marketplace needs | Undecided; verify local support/fees/payouts |

## Evolution triggers
Split services only for a demonstrated reason such as independent deployment/security boundaries, sustained load, or isolated team ownership. Keep contracts stable and versioned as boundaries evolve.

