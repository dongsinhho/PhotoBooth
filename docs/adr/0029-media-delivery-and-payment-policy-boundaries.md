# ADR 0029: Keep media delivery and payment policy outside capture

- **Status:** Accepted; policy boundaries implemented in the root workflow, CLI and Windows Preview Wails prototype. Local delivery and disabled/free payment remain the only supported modes.
- **Date:** 2026-11-07

## Context

The station profile already declares media_delivery.mode and payment.mode, but the schema-v1 implementation supports only local delivery and disabled payment. The workflow already depends on an output.Dispatcher interface, while CLI and Wails entrypoints constructed the local provider directly. The payment setting was validated but did not participate in the workflow lifecycle.

Future LAN/cloud delivery or provider-backed payment must not require capture/provider changes. A selected policy must be explicit and must never silently fall back to local/free behavior.

## Decision

1. Add a delivery factory that resolves the configured media-delivery policy to output.Dispatcher. The only current implementation is local storage. Unknown modes and a missing local output root fail closed.
2. Add a payment.Gate interface invoked by the workflow after frame/config validation and before creating a session workspace or opening the camera. The disabled/free gate performs no network work and respects context cancellation. A future provider-backed gate receives the stable session ID and a Resume flag; its contract requires idempotent authorization/reuse of an existing permit on resume rather than issuing a second charge.
3. Resolve both components from StoreConfig in the root CLI and Wails player. The capture/render workflow receives only the dispatcher and gate interfaces, not provider transports.
4. Add media_delivery_mode and payment_mode to schema-v2 session manifests and the run-config fingerprint. New sessions persist explicit policy values. Continue recomputes the fingerprint and refuses to resume if either policy changes. Legacy schema-v1 manifests remain readable under their existing compatibility rules.
5. Keep schema-v1 StoreConfig unchanged. local and disabled remain defaults. LAN/cloud/QR or any other unsupported values are rejected during config validation/factory resolution; no UI option, network exposure, QR display, payment intent, webhook or provider integration is introduced by this ADR.
6. Do not make capture success depend on optional delivery. The existing output coordinator continues to report each target independently and preserves local behavior.

## Consequences

- Capture and camera providers remain independent of payment transports and delivery protocols.
- The disabled payment path remains free and offline; no payment network call is made.
- The Local dispatcher factory is the only delivery resolver today. Network exposure remains impossible through this setting because LAN/cloud are not implemented.
- A policy change between an interrupted session and Continue is treated as configuration drift, not silently accepted.
- A future paid mode needs a durable permit/idempotency model, provider-side verification, expiry and reconciliation before it may be enabled. A future LAN mode needs session-scoped authorization, explicit interface binding, short-lived links and cross-session isolation tests.
- Stage 2 remains OPEN: broader settings-mode acceptance and local network-exposure proof have not yet been completed.

## Validation

- Unit tests cover Local dispatcher resolution, missing local root and unsupported LAN rejection.
- Unit tests cover the disabled gate, cancellation/invalid session identity and unsupported QR mode rejection.
- Workflow regression verifies payment-gate denial leaves the session workspace unclaimed and does not open the fake camera.
- Session/workflow tests verify schema-v2 manifests require explicit local/disabled policy fields and reject unimplemented modes.
- Root CLI and Wails Go suites pass. Full repository/vet/build/UI smoke results are recorded in HANDOFF_2026-11-07.md.

### End-to-end local/offline policy acceptance follow-up (2026-11-08)

- Added a CLI entrypoint integration test that loads a real schema-v1 profile with fake camera + local delivery + disabled payment, runs all three captures, writes the session-local output, and verifies the schema-v2 manifest records both policy modes and reaches output_finalized.
- Added CLI fail-closed tests for LAN delivery, cloud delivery and QR payment using externally supplied invalid profiles. The full CLI entrypoint returns the policy validation error before writing to stdout, creating the configured sessions root, constructing a camera run or creating a session workspace. Config Save also refuses to persist these unsupported values.
- Added workflow payment-gate request tests for both new sessions and resumed sessions. The request retains the same session ID, sets Resume=false for a new session and Resume=true for Continue. Payment denial remains before session workspace claim and camera open.
- Added a recovery regression proving a changed unsupported delivery/payment policy is rejected before opening the camera and leaves the interrupted manifest byte-for-byte unchanged.
- Added Wails tests proving Settings saves preserve policy fields that are not exposed as editable controls and each running session receives an immutable snapshot of the delivery/payment policy values. Factory assertions verify they still resolve to the concrete local dispatcher and DisabledGate.
- Focused root CLI/config/delivery/payment/workflow tests and the complete Wails Go suite passed on the Windows host.

This strengthens the Stage 2 policy-boundary acceptance, but it does not close Stage 2. The only supported modes remain local and disabled/free; no LAN/cloud listener, QR flow, payment provider or credential handling was added. The Linux path guards are still compile-tested only in this environment, and an explicit runtime audit demonstrating no unintended network exposure on a deployed booth profile remains a separate acceptance item.
