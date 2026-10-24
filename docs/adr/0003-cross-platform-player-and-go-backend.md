# ADR 0003: Cross-Platform Player and Go Backend Direction

- **Status:** Direction accepted; technology and hardware choices still require validation.
- **Date:** 2026-10-24
- **Decision owner:** Product owner

## Context
The product should reduce booth hardware cost by supporting Linux in addition to Windows and should keep cloud services lightweight to operate. Camera and printer SDK support can differ by OS and model.

## Decision
- Windows and Linux are target desktop platforms, but support claims must be expressed as a tested compatibility matrix.
- Keep domain logic and session orchestration independent from OS APIs and vendor SDKs.
- Put camera, printer, secure-storage, kiosk/autostart, and packaging differences behind platform/provider adapters as needed.
- Go is the preferred backend language.
- Start with a modular monolith; do not split into microservices without evidence requiring separate deployment or scaling.
- UI framework, Linux distribution, first camera provider, and exact backend libraries remain open pending a technical spike.

## Trade-offs
- Cross-platform UI can reduce duplicate presentation logic but does not eliminate native hardware integration.
- More OS combinations increase build, packaging, diagnostics and regression-test burden.
- An abstract camera interface improves extensibility but does not make unsupported camera features possible.
- Linux may reduce some licensing/OS costs, but total cost must include support effort, hardware compatibility, deployment, remote maintenance, and printers.

## Alternatives considered
1. **Windows-only:** simpler initial device validation but does not meet the Linux target.
2. **One native UI codebase per OS:** can maximize platform-native behavior but increases duplicated presentation and maintenance.
3. **Shared cross-platform UI plus platform adapters:** preferred direction, subject to prototype validation.
4. **Immediate microservices:** not selected; unnecessary for initial scope.

## Validation gate
Build a small prototype that launches as a kiosk on the selected Windows and Linux targets, exercises touch/input, connects to the first target camera through an actual provider, and completes one output path. Choose the UI framework and release strategy based on that evidence.

