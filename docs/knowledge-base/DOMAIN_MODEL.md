# Initial Domain Model

**Status:** Proposed conceptual model

This vocabulary is a starting point, not a finalized database schema.

## Core entities

### Store
Business/account context owning booths and controlling store settings, content activation, branding, and team access.

### BoothDevice
Player installation registered to a store. Records device identity, player version, capabilities, last sync, and configuration. Credentials must not be stored in plain text.

### PhotoSession
One guest photo workflow. May include timestamps, state, exact content version, output status, and technical diagnostics. Minimize guest data and define retention.

### Capture
One source image from a session, with sequence index, local file reference, processing metadata, and capture status.

### Theme
Declarative package that changes player presentation: screen layouts, typography, colors, approved assets, and transitions. Never arbitrary executable code.

### Frame
Declarative package defining photo composition: canvas, photo slots, overlays/backgrounds, layer order, output variants, and preview.

### ContentPackageVersion
Immutable published theme/frame version. Carries schema version, compatibility constraints, file manifest, hashes/signature, author, license, and release metadata.

### Installation
Record of a package version installed on a booth, with activation state and rollback information. Downloaded and active are distinct states.

### Creator
Person or organization publishing content; later profile, verification, listing ownership, statistics, and payout features.

### Listing
Marketplace entry with free/paid status, price, categories, previews, availability, moderation state, and package-version references.

### Entitlement
Grant allowing a store/account to use a paid package under specified terms. Use a signed or integrity-protected cached entitlement and a defined offline policy rather than a network call for each session.

### Order
Commercial transaction with payment/refund state and purchased listing/entitlements. Payment state must come from a trusted server/provider callback, not a client claim.

## Relationship sketch

~~~mermaid
erDiagram
  STORE ||--o{ BOOTH_DEVICE : operates
  BOOTH_DEVICE ||--o{ PHOTO_SESSION : runs
  PHOTO_SESSION ||--o{ CAPTURE : contains
  STORE ||--o{ INSTALLATION : installs
  CONTENT_PACKAGE_VERSION ||--o{ INSTALLATION : delivered_as
  CREATOR ||--o{ LISTING : publishes
  LISTING ||--o{ CONTENT_PACKAGE_VERSION : versions
  LISTING ||--o{ ORDER : purchased_through
  ORDER ||--o{ ENTITLEMENT : grants
~~~

Relationships are conceptual. Decide before database implementation whether PhotoSession references a theme version, frame version, both, or a full configuration snapshot.

## Modeling principles
- Use stable IDs and immutable content-version identifiers.
- Snapshot exact theme/frame versions and rendering settings used by a session; later updates must not alter historical outputs.
- Distinguish package ownership, installation, activation, and license entitlement.
- Model asynchronous device/output operations with explicit status and error information.
- Store image bytes in controlled file/object storage, not relational rows; reference them from metadata.
- Do not persist more guest data than required.
- Keep payments server-authoritative and make webhooks idempotent when commerce exists.

