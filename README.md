# PhotoBooth

**PhotoBooth** is planned cross-platform photobooth software that combines reliable on-site photo capture with configurable themes/frames and optional online content services.

> **Status:** Product definition and knowledge base only. The player, backend, and marketplace have not yet been implemented.

## Product vision

Help booth operators reuse cameras, computers and printers they already own. The official app is intended to be free for commercial photobooth operations. Owners may configure supported application behavior and add supported frames/themes without editing code. Optional paid support and authorized custom engineering are the intended business model.

## Product surfaces

1. **PhotoBooth Player** — desktop kiosk app for camera control, guest sessions, rendering, printing/export, optional payment and media delivery.
2. **Application-level customization** — settings, configuration, frame/theme packages and documented content formats; not core-source editing.
3. **Optional Photo Delivery** — local-only storage, controlled LAN downloads, or a configured cloud service.
4. **Optional Payments** — free mode, payment in the guest flow, or verified prepayment.
5. **Future Content Catalog** — browse, preview and install content; paid/community distribution may be added later.
6. **Store Configuration** — device settings, content, media policy, payment policy and retention.

## Core principles

- Free-to-use commercial operation is the intended default for the official app; support is optional.
- Normal customization happens through supported app settings and frame/theme content, not source-code editing.
- Core modification or distribution of a modified core build requires separate permission/licensing under the intended controlled-core model; final legal terms are pending.
- Do not label the core Open Source unless the final license permits modifications and derived works under the Open Source Definition.
- Capture/render/local output work offline once configured and assets are installed.
- Camera and printer providers are extensible, with explicit tested compatibility.
- Photo delivery and payment are independently configurable per store.
- Guest photos stay local by default.

## Proposed architecture

~~~mermaid
flowchart TD
  subgraph Player["Cross-platform PhotoBooth Player"]
    UI["Kiosk UI + Supported Settings"]
    Session["Session Orchestrator"]
    Camera["Camera Provider"]
    Render["Image Render Pipeline"]
    Output["Print / Export"]
    Payment["Payment Adapter (optional)"]
    Media["Media Delivery Adapter"]
    Local["Local DB + Asset/Photo Store"]
    UI --> Session
    Session --> Camera
    Session --> Render
    Render --> Output
    Session --> Payment
    Session --> Media
    Session --> Local
  end
  subgraph Cloud["Optional Cloud Platform"]
    API["Go API (preferred candidate)"]
    Catalog["Catalog / Orders / Entitlements"]
    Storage["Object Storage + CDN"]
    API --> Catalog
    API --> Storage
  end
  Local <-->|"Sync when enabled/online"| API
~~~

This is a target proposal, not a description of implemented code.

## Initial technical direction

- Backend: Go is preferred; start with a modular monolith.
- Player UI: evaluate cross-platform choices such as Avalonia/.NET and Qt with a hardware/kiosk proof of concept.
- Camera/printing: provider/adapter boundaries and a tested compatibility matrix.
- Local data: SQLite and local asset/photo storage are candidates.
- Cloud data/assets: PostgreSQL and S3-compatible storage/CDN are candidates, not locked decisions.
- Media delivery: local-only, LAN, or optional platform cloud.
- Payment: disabled/free, QR during flow, or verified prepayment.

## Distribution and business model

The product does not sell hardware. The official application is intended to be free for commercial booth use. Support, setup, training, hardware integration, and authorized core modifications may be paid services.

Supported settings and theme/frame formats are the normal customization path. The standard user grant does not permit core-source modification or redistribution of modified core builds. See [License Strategy](docs/knowledge-base/LICENSE_STRATEGY.md); final legal text is not yet published.

## Knowledge base for AI-assisted development

Read [AGENTS.md](AGENTS.md), then [docs/knowledge-base/README.md](docs/knowledge-base/README.md), and then the relevant product/architecture/license documents before changing code. Preserve the distinction between accepted requirements, proposals, and implementation that has actually been verified.
