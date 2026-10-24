# Media Delivery and Optional Payment

**Status:** Accepted product flexibility; provider details and final UX remain open

## 1. Media delivery modes

Each store configures a default mode. Per-session overrides can be considered later if explicitly allowed by the operator.

### Local-only
- Output is stored on the booth local disk.
- No automatic cloud upload.
- Operator retrieves files locally.
- Define storage quotas, cleanup, permissions and retention.

### LAN download
- Photos remain on the booth or a store-controlled local node.
- Guest devices access only an explicitly shared output, not a directory listing or the whole image library.
- Prefer a short-lived, unguessable session URL/QR token scoped to one output. Token expiry, rate limits and revocation must be supported.
- Provide a trusted-LAN setup guide. A LAN is not automatically safe: isolate guest access, use TLS where feasible, and avoid exposing the service on public interfaces by default.
- Define guest discovery, link expiry, and protection against one guest accessing another guest's photos.

### Platform cloud
- Upload is optional and explicitly enabled/configured by the store.
- Use authenticated, short-lived upload/download authorization and object access policies.
- Define region, encryption, retention, deletion, quotas, transfer/storage billing and guest consent UX.
- The local copy should remain until output/upload state meets store policy; cloud upload failure must not silently destroy the only copy.
- Basic capture must not depend on cloud availability.

## Media-delivery state
Keep capture/output success separate from delivery status. Example statuses: NotRequested, Pending, Uploading, Available, FailedRetryable, Expired, Deleted. Define retries and cleanup carefully; a QR must not promise an image is available before the file exists.

## 2. Payment modes

### Disabled/free
No payment provider is required. Sessions start according to store policy.

### QR payment during guest flow
The QR is generated from a trusted payment request or merchant-configured method. Show amount, purpose/session reference and expiry clearly. Decide how payment confirmation is verified; a static QR or guest screenshot alone is not proof of payment.

### Prepayment before session
Create a payment intent/order, wait for authoritative success confirmation, then issue a session-scoped permit. The permit should be non-reusable or idempotently bound to the intended session/booth and have an expiry/reconciliation policy.

## Payment safety invariants
- Store payment mode is configuration; payment code does not belong inside camera adapters.
- The client must not mark a transaction paid based solely on client callback, redirect, screenshot or a manual claim.
- Verify provider notifications/signatures server-side where applicable.
- Make callbacks idempotent and reconcile pending/unknown transactions.
- Define timeout, duplicate notification, late payment, refund, cancellation, network loss, and payment success after booth abandonment.
- If payment is required but cannot be authoritatively verified, do not silently grant the paid session.
- Never store customer banking credentials or QR payment secrets in plaintext.

## Configuration model
Store-level policy should make these settings explicit:
- payment mode: disabled / during-flow / prepay-before-session.
- provider and merchant reference; store secrets securely.
- amount/currency/product or pricing rule.
- confirmation mechanism and timeout/reconciliation behavior.
- media delivery mode: local-only / LAN / platform-cloud.
- retention, expiry, size limits and upload behavior.
- whether operator overrides or session-level choices are allowed.

These field names are illustrative until a configuration API is implemented.

## MVP recommendation
Start with payment disabled and local-only output to prove capture reliability. Design interfaces and state transitions so QR payment and LAN/cloud delivery can be added as independently testable modules. If a pilot specifically requires payment, choose one supported payment confirmation flow and test reconciliation before launch.

