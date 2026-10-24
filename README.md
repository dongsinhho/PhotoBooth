# PhotoBooth

**PhotoBooth** is a planned Windows-first photobooth platform combining reliable on-site photo capture with an online ecosystem for themes, photo frames, and community creators.

> **Status:** Early product definition. This repository contains product/engineering documentation; the player, backend, and marketplace have not yet been implemented.

## Vision

Let a photobooth operator run a dependable booth and refresh its look without rebuilding the software. Operators should be able to discover, preview, download, purchase, install, and update theme/frame packages. Creators should eventually publish free or paid content to a community marketplace.

## Product surfaces

1. **PhotoBooth Player** — Windows desktop app for camera control, guest sessions, rendering, printing, and export.
2. **Content Marketplace** — discover, preview, download, and purchase themes and frames.
3. **Store Dashboard** — store/device settings and installed content; later, multi-booth management.
4. **Creator Studio** — future package upload, validation, publishing, and creator analytics.

## Core principles

- **Offline-first capture:** previously configured booths must complete sessions without Internet access.
- **Reliability first:** capture, image output, and recovery are critical paths.
- **Content is data, not arbitrary code:** downloadable themes/frames use a validated, versioned package format.
- **Separate presentation from capture:** theme changes must not destabilize camera or printing workflows.
- **Privacy by default:** guest photos stay local unless an operator explicitly enables a cloud-delivery feature.
- **Explicit compatibility:** camera models, printer paths, player versions, and package schemas are versioned and tested.
- **Living documentation:** durable changes update the knowledge base and, when appropriate, an ADR.

## Proposed high-level architecture

~~~mermaid
flowchart TD
  subgraph Local["Windows PhotoBooth Player"]
    UI["Kiosk UI / Theme Host"]
    Session["Session Orchestrator"]
    Camera["Camera Adapter"]
    Render["Image Render Pipeline"]
    Output["Print / Export"]
    Cache["SQLite + Asset Cache"]
    UI --> Session
    Session --> Camera
    Session --> Render
    Render --> Output
    Session --> Cache
  end
  subgraph Cloud["Cloud Platform (later phase)"]
    API["Marketplace API"]
    Catalog["Catalog / Entitlements"]
    Assets["Object Storage + CDN"]
    Creator["Creator Publishing"]
    API --> Catalog
    API --> Assets
    Creator --> API
  end
  Cache <-->|"Sync when online"| API
~~~

This is a **target proposal**, not a description of implemented code.

## Initial technical direction (proposed)

- Windows desktop: C#/.NET + WPF, with WebView2 as a candidate for configurable presentation.
- Camera integration: vendor-specific providers behind an adapter boundary.
- Local data: SQLite and local asset/photo storage.
- Cloud backend: start as a modular monolith if/when needed; ASP.NET Core or Go, PostgreSQL, and S3-compatible storage/CDN are candidates, not locked decisions.
- Content: design a versioned manifest and constrained theme/frame schemas before building the marketplace.

## MVP direction

1. Start a session and show a countdown.
2. Capture a configured sequence with one explicitly supported camera model.
3. Render the photos into a static frame.
4. Preview and print/export the result through a tested output path.
5. Save session state and recover from common failures.
6. Complete the workflow without a network connection.

Marketplace payments, creator payouts, arbitrary theme scripting, analytics, and broad multi-camera support are not MVP prerequisites.

## Repository map

~~~text
.
├── AGENTS.md
├── README.md
└── docs/
    ├── adr/
    └── knowledge-base/
        ├── README.md
        ├── PRODUCT_BRIEF.md
        ├── MVP_SCOPE.md
        ├── SYSTEM_ARCHITECTURE.md
        ├── DOMAIN_MODEL.md
        ├── CONTENT_PACKAGE_SPEC.md
        ├── SECURITY_PRIVACY.md
        ├── ENGINEERING_PLAYBOOK.md
        └── ROADMAP.md
~~~

## For AI-assisted development

Before planning or changing code, read [AGENTS.md](AGENTS.md), then [docs/knowledge-base/README.md](docs/knowledge-base/README.md), then the relevant topic documents and ADRs.

The knowledge base distinguishes confirmed product constraints from proposals and open questions. Update it as decisions and implementation evolve.

