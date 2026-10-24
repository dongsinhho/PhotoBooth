# PhotoBooth

**PhotoBooth** is a planned cross-platform photobooth platform combining reliable on-site capture with an online ecosystem for themes, photo frames, and community creators.

> **Status:** Early product definition. This repository currently contains product/engineering documentation; the player, backend, and marketplace have not yet been implemented.

## Vision

Let booth operators run dependable photo sessions, configure media delivery and payment behavior, and refresh the booth's visual experience without rebuilding the software. Creators should eventually be able to publish free or paid theme/frame packages.

## Product surfaces

1. **PhotoBooth Player** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â desktop kiosk app for camera control, guest sessions, rendering, printing/export, optional payment, and media delivery.
2. **Content Marketplace** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â discover, preview, download, and purchase themes and frames.
3. **Store Dashboard** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â store/device configuration, policies, installed content, and eventually multi-booth management.
4. **Creator Studio** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â future package validation, publishing, versioning, and creator analytics.
5. **Optional Photo Delivery** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â local-only storage, downloads over the store's LAN, or a cloud service supplied by the platform.
6. **Optional Payments** ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â free mode, QR payment during the guest flow, or verified prepayment before a session.

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
8. Define and test one media-delivery mode and one payment modeÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Âor the disabled/free pathsÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Âwithout making them hard dependencies of capture.

## Repository map

- `AGENTS.md` - repository instructions for AI coding agents.
- `docs/knowledge-base/README.md` - documentation index and reading order.
- `docs/knowledge-base/PRODUCT_PLAN.md` - end-to-end plan from discovery to launch.
- `docs/knowledge-base/LICENSE_STRATEGY.md` - free commercial use, modification rights, and paid support.
- `docs/knowledge-base/PRODUCT_BRIEF.md` - vision and target users.
- `docs/knowledge-base/MVP_SCOPE.md` - initial product scope and acceptance criteria.
- `docs/knowledge-base/SYSTEM_ARCHITECTURE.md` - proposed system and module boundaries.
- `docs/knowledge-base/DOMAIN_MODEL.md` - domain concepts.
- `docs/knowledge-base/CAMERA_INTEGRATION.md` - camera provider strategy.
- `docs/knowledge-base/MEDIA_DELIVERY_AND_PAYMENT.md` - local/LAN/cloud delivery and payment.
- `docs/knowledge-base/CONTENT_PACKAGE_SPEC.md` - theme/frame package format proposal.
- `docs/knowledge-base/SECURITY_PRIVACY.md` - security and photo privacy.
- `docs/knowledge-base/ENGINEERING_PLAYBOOK.md` - workflow and test strategy.
- `docs/knowledge-base/ROADMAP.md` - staged rollout and validation gates.
- `docs/adr/` - architecture decision records, including licensing and support model.
## Distribution and business model

PhotoBooth is intended to be software-only: the project does not sell camera, computer, printer, or booth hardware.

- The official app is intended to be free to use for commercial photobooth sessions.
- Operators can self-install and use it without buying support.
- Supported settings and theme/frame packages are the primary customization route.
- Optional paid services may include setup, troubleshooting, training, hardware integration, migration, and authorized custom core work.
- The exact code license is not finalized. See [License and Support Strategy](docs/knowledge-base/LICENSE_STRATEGY.md). Do not assume the project is open source solely because the repository is public.
## For AI-assisted development

Before planning or changing code, read [AGENTS.md](AGENTS.md), then [docs/knowledge-base/README.md](docs/knowledge-base/README.md), then the relevant topic documents and ADRs.

The knowledge base distinguishes accepted requirements from proposals and open questions. Update it as decisions and implementation evolve.
