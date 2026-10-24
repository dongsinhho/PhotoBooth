# PhotoBooth

**PhotoBooth** is a planned cross-platform photobooth platform combining reliable on-site capture with an online ecosystem for themes, photo frames, and community creators.

> **Status:** Early product definition. This repository currently contains product/engineering documentation; the player, backend, and marketplace have not yet been implemented.

## Vision

Let booth operators run dependable photo sessions, configure media delivery and payment behavior, and refresh the booth's visual experience without rebuilding the software. Creators should eventually be able to publish free or paid theme/frame packages.

## Product surfaces

1. **PhotoBooth Player** — desktop kiosk app for camera control, guest sessions, rendering, printing/export, optional payment, and media delivery.
2. **Content Marketplace** — discover, preview, download, and purchase themes and frames.
3. **Store Dashboard** — store/device configuration, policies, installed content, and eventually multi-booth management.
4. **Creator Studio** — future package validation, publishing, versioning, and creator analytics.
5. **Optional Photo Delivery** — local-only storage, downloads over the store's LAN, or a cloud service supplied by the platform.
6. **Optional Payments** — free mode, QR payment during the guest flow, or verified prepayment before a session.

## Core principles

- **Offline-first capture:** network loss must not interrupt a configured capture workflow.
- **Configurable per store:** media delivery and payment modes can be enabled, disabled, or selected independently.
- **Reliability first:** capture, rendering, print/export, and recovery are critical paths.
- **Extensible hardware:** camera and printer integrations sit behind provider/adapter contracts with explicit capability and compatibility metadata.
- **Content is data, not arbitrary code:** downloaded packages use validated, versioned declarative formats.
- **Privacy by default:** photos remain local unless the operator explicitly enables another delivery mode.
- **Cross-platform by design:** target Windows and Linux, but validate each hardware/OS combination instead of assuming parity.
- **Living documentation:** durable decisions update the knowledge base and ADRs.

## Proposed architecture

~~~mermaid
flowchart TD
  subgraph Player["Cross-platform PhotoBooth Player"]
    UI["Kiosk UI / Theme Host"]
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
  subgraph Cloud["Cloud Platform (optional/later phase)"]
    API["Go API (preferred candidate)"]
    Catalog["Catalog / Orders / Entitlements"]
    Storage["Object Storage + CDN"]
    API --> Catalog
    API --> Storage
  end
  Local <-->|"Sync when enabled/online"| API
~~~

This is a **target proposal**, not a description of implemented code.

## Initial technical direction

- **Backend:** Go is the preferred candidate; start with a modular monolith, not microservices by default.
- **Player UI:** cross-platform UI candidates include Avalonia (.NET) and Qt. A technical spike must validate kiosk behavior, deployment, touch input, and integration effort.
- **Camera/printing:** platform- and vendor-specific providers behind stable interfaces. Model/SDK capabilities must be validated on real hardware.
- **Local data:** SQLite and local asset/photo storage are candidate foundations.
- **Cloud data and assets:** PostgreSQL and S3-compatible object storage/CDN are candidates, not locked decisions.
- **Media delivery:** local-only, LAN download, or platform cloud, configured per store.
- **Payment:** disabled/free, QR/payment during the flow, or verified prepayment, configured per store.

## MVP direction

1. Start a session and show a countdown.
2. Capture a configured sequence using one explicitly supported camera configuration.
3. Render the photos into a static frame.
4. Preview and print/export through one tested output path.
5. Save session state and recover from common failures.
6. Complete the capture workflow without a network connection.
7. Prove the player can be built and run on the first selected target OS; keep the second OS as an explicit validation target.
8. Define and test one media-delivery mode and one payment mode—or the disabled/free paths—without making them hard dependencies of capture.

## Repository map

~~~text
.
├── AGENTS.md
├── README.md
└── docs/
    ├── adr/
    └── knowledge-base/
        ├── README.md
        ├── PRODUCT_PLAN.md
        ├── PRODUCT_BRIEF.md
        ├── MVP_SCOPE.md
        ├── SYSTEM_ARCHITECTURE.md
        ├── DOMAIN_MODEL.md
        ├── CAMERA_INTEGRATION.md
        ├── MEDIA_DELIVERY_AND_PAYMENT.md
        ├── CONTENT_PACKAGE_SPEC.md
        ├── SECURITY_PRIVACY.md
        ├── ENGINEERING_PLAYBOOK.md
        └── ROADMAP.md
~~~

## For AI-assisted development

Before planning or changing code, read [AGENTS.md](AGENTS.md), then [docs/knowledge-base/README.md](docs/knowledge-base/README.md), then the relevant topic documents and ADRs.

The knowledge base distinguishes accepted requirements from proposals and open questions. Update it as decisions and implementation evolve.
