# Initial Domain Model

**Status:** Proposed conceptual model, not a database schema

## Entities and concepts
- **Store:** operator/business settings, user access, media delivery policy, payment policy, retention policy.
- **BoothDevice:** player installation, OS/player version, registered capabilities, last sync, store assignment.
- **PhotoSession:** lifecycle/status, timestamps, selected theme/frame versions, captures, payment authorization reference if required, output and delivery state.
- **Capture:** image sequence index, local path/reference, capture status and processing metadata.
- **Theme / Frame:** immutable content package types with version and compatibility metadata.
- **ContentPackageVersion:** schema version, package version, checksums/signature, player compatibility, author and license.
- **Installation:** package installed/active on a specific booth, with rollback version.
- **PaymentPolicy:** disabled, pay-during-flow, or prepay-before-session, plus provider/configuration reference.
- **PaymentAttempt / PaymentTransaction:** amount/currency, provider reference, idempotency key, state, timestamps and reconciliation metadata; no raw payment credentials.
- **SessionPermit:** server/provider-verifiable authorization for a paid session, scoped to a session and expiring/one-time where supported.
- **MediaDeliveryPolicy:** local-only, LAN, or platform cloud; retention and sharing settings.
- **MediaAsset / DeliveryJob:** output file metadata, storage location/mode, delivery state, expiry and deletion status.
- **Creator / Listing / Order / Entitlement:** later marketplace entities.

## Relationships (conceptual)
- Store configures multiple BoothDevices.
- BoothDevice runs PhotoSessions.
- PhotoSession contains Captures and one finalized output configuration.
- PhotoSession may reference PaymentAttempt/SessionPermit and MediaAsset/DeliveryJob.
- Store configuration selects PaymentPolicy and MediaDeliveryPolicy.
- A booth installs immutable ContentPackageVersions.
- Creator publishes Listings with immutable package versions.
- Paid Listing purchase grants Entitlement under explicit license terms.

## Modeling rules
- Persist which theme/frame version rendered an output; later package updates must not change past results.
- Separate payment state from session state, and delivery state from output-generation state.
- Use idempotency keys for payment requests and webhook handling.
- Represent policy changes explicitly; define whether changes apply only to new sessions or immediately.
- Store image bytes in controlled file/object storage and metadata separately.
- Do not store full card/bank credentials or payment secrets.
- Keep only minimal guest/session identifiers and define deletion/retention behavior.

