# System Architecture

**Status:** Tested local Go vertical slice, experimental Windows Media Foundation provider, independent output coordinator and adaptive renderer. One hundred sequential B525 sessions and a competing-camera/recovery probe passed their expected outcomes. Production kiosk UI, physical printer adapter, USB hot-unplug, persistent-process memory soak, cloud modules and Linux hardware remain incomplete.

## Architecture approach
Start with a modular player and a Go modular-monolith backend. Do not introduce microservices or message brokers before operational needs justify them. Preserve boundaries that permit future extraction.

The critical capture path is local. Catalog, asset sync, optional cloud photo delivery, and payment verification may use the network, but network loss must not block a session that does not require payment.

## UI framework evaluation

The production UI framework remains unselected. An isolated Wails v2.17.0 shell spike under tools/ui-spike/wails-v2 validates the desktop WebView-to-Go binding and a simulated three-shot flow; it is not connected to the camera, renderer, output coordinator, or printer. Wails v2.17.0 requires Go 1.25.0, so the spike lives in a separate module and does not change the Go 1.24.8 core. Three hidden-mode runs measured a mean p50 round trip of 0.233 ms and mean p95 of 0.400 ms, with sampled working-set maximum 25.93 MB. These do not establish visible input latency, low-cost hardware performance, or Linux support. See UI_SPIKE_RESULTS.md and ADR 0012.

## Player modules

### Presentation / Kiosk UI
Renders guest and operator experiences using a theme schema and supported components. It does not own device or payment control.

### Session Orchestrator
Owns session state, cancellation, deadlines, recovery, and sequencing. Coordinates capture, render, output, payment policy, and selected media-delivery behavior through interfaces.

Suggested state model: Idle -> Preflight -> AwaitingPayment (optional) -> Capturing -> Rendering -> Output -> Delivery -> Completed. Include PartialSuccess, Failed, Cancelled, TimedOut, and recovery paths. Payment and delivery states may be skipped according to configuration.

### Camera Provider Layer
A stable internal contract with provider-specific implementations. Each provider advertises capabilities and supported camera/OS combinations. Do not assume every camera exposes trigger capture, live view, focus, or settings equally.

### Render Pipeline
Produces deterministic static output from captured photos, frame definition, and versioned settings. Keep layout, crop, color/output dimensions, and error handling separate from UI.

### Output Coordinator and Targets
The output coordinator receives the finalized photo and invokes configured output targets independently. The coordinator bounds concurrency, preserves a result for each target, and reports per-target status/duration/error. A failed target does not cancel or roll back a sibling target. The current implementation supports the local-storage target and a tested target contract; the physical printer adapter remains unimplemented.

### Local Storage Target
Persists the finalized photo under a session-scoped path. A successful local save is reported independently from the printer outcome.

### Printer Target
A future adapter will submit a print job through the selected OS/driver integration. It should return a spool/job identifier and queued status when accepted, rather than block the capture/session thread until paper physically emerges. Retry and duplicate-print behavior must be provider-aware; do not blindly retry an uncertain spool operation.

### Payment Module
A policy/orchestrator plus provider adapters. It may be disabled. When payment is required, authoritative server/provider verification must happen before a session permit is granted. Do not treat a client redirect, QR display, screenshot, or guest claim as proof of payment.

### Media Delivery Module
Policy-based router with three modes:
- **Local-only:** save photos at the booth; operator-managed access.
- **LAN:** expose only the selected output to devices on the trusted store network via a short-lived session link/QR or controlled download endpoint.
- **Platform cloud:** upload through a configured cloud service with explicit retention and access controls.

Capture/render must complete even if optional delivery later fails; delivery can become pending/retryable where policy permits.

### Local Persistence
SQLite is a candidate for session metadata, settings, package versions, and sync state. Photos/assets live on local storage. Define disk-full handling and retention. Do not store payment secrets in plaintext configuration.

### Update / Sync Agent
Runs outside active session critical sections. Downloads metadata/content to staging, validates integrity and compatibility, then activates when safe. Preserve rollback capability.

## Cloud backend modules (Go preferred)
- Identity/store/device configuration.
- Catalog/content versioning and signed package delivery.
- Payment intent/order/verification and webhook handling.
- Media upload/download authorization, retention and deletion.
- Optional cloud album/session access.
- Creator publishing, moderation, marketplace orders and entitlements.
- Device update metadata and non-sensitive diagnostics.

Prefer a modular monolith with explicit packages/domain boundaries. PostgreSQL and S3-compatible object storage/CDN are candidates, not final decisions.

## Cross-platform direction
Keep domain/session contracts independent of OS APIs. Implement camera, printing, secret storage, kiosk integration, and packaging behind platform/provider adapters when behavior differs. Cross-platform UI does not make vendor SDKs cross-platform automatically.

Candidate UI technologies:
- Avalonia (.NET): potential C# reuse and Windows/Linux desktop target.
- Qt: mature cross-platform UI ecosystem; licensing and tooling need evaluation.

Choose only after an engineering spike covering touch/kiosk UX, camera preview if needed, rendering performance, printer integration, deployment/update, and maintenance.

## Non-functional boundaries
- No network calls may block capture/render/output when payment is not required.
- Payment-required sessions fail closed if authoritative confirmation is unavailable; do not silently grant unpaid sessions.
- Optional upload failure must not erase local output.
- Failure of one configured output target must not suppress another target; aggregate session state must distinguish completed, partial_success, and total output failure.
- UI remains responsive during device, render, and network I/O.
- Package/app updates are staged, integrity-checked, and rollback-capable.
- Images and credentials are excluded from routine logs.


## External references to re-check before implementation

- [Avalonia supported platforms](https://docs.avaloniaui.net/docs/supported-platforms) and [Avalonia Desktop Linux guide](https://docs.avaloniaui.net/docs/platform-specific-guides/linux): useful when evaluating a .NET cross-platform kiosk UI. Support tiers and requirements can change between releases.
- [Sony Camera Remote SDK](https://support.d-imaging.sony.co.jp/app/sdk/en/) and [libgphoto2 supported cameras](https://gphoto.sourceforge.io/proj/libgphoto2/support.php): investigate exact operations and supported OS/model combinations; these are not blanket guarantees for every camera.
