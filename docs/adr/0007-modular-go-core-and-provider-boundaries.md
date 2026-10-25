# ADR 0007: Modular Go core and provider boundaries

**Status:** Accepted
**Date:** 2026-10-25

## Context

The booth application must support multiple camera families, optional payment, multiple media-delivery modes, Windows and Linux, and a configurable content layer. The critical capture path should remain usable offline. Vendor SDKs and OS APIs are expected to differ substantially between devices.

The project also wants a lightweight implementation that can be maintained by AI-assisted development without prematurely introducing distributed infrastructure.

## Decision

1. Use Go for the booth application's domain/orchestration core and for the backend service.
2. Start as a modular monolith. Do not introduce microservices, brokers, or mandatory cloud dependencies for the local capture MVP.
3. Keep camera integration behind a narrow `camera.Provider` contract. Provider implementations must expose capabilities rather than forcing every camera to support every operation.
4. Keep the session lifecycle independent from camera, payment, rendering, output, and media-delivery implementations.
5. Treat the UI as an adapter/client of the session core. The UI technology is intentionally not locked by this ADR.
6. Use explicit state transitions and tests for the session lifecycle. Payment and media delivery may be skipped according to configuration.
7. Keep platform-specific behavior behind adapters so Windows and Linux can share domain/session code.

## Consequences

### Positive

- Camera vendor/OS changes are isolated.
- The core can be tested without physical hardware.
- The same session model can support payment-disabled and payment-required flows.
- A future Linux player can reuse domain logic.
- Go keeps the first implementation small and operationally simple.

### Trade-offs

- The provider contracts must be designed carefully to avoid leaking assumptions from the first webcam implementation.
- UI integration and camera preview may still require platform-specific code.
- Go is not itself a cross-platform UI toolkit, so the final UI choice remains a separate engineering spike.

## Rejected for now

- Microservices for the booth player.
- A cloud-only capture workflow.
- Direct camera SDK calls from UI code.
- Arbitrary executable theme plugins.
- Choosing a UI framework before the first end-to-end session flow and camera/output constraints are better understood.
