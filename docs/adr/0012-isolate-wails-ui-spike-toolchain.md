# ADR 0012: Isolate the Wails UI spike from the core module

**Status:** Accepted for evaluation only; production UI decision remains open
**Date:** 2026-10-25

## Context

The current product has a Go 1.24.8 modular core, a real Windows Media Foundation camera adapter, schema-v1 frame renderer, session orchestrator, and independent output targets. The next step needs evidence about a touch-first desktop shell, the UI-to-Go bridge, and resource consumption. Committing the main module to a toolkit before a runnable comparison would be premature.

The Wails v2.17.0 CLI reports that its module requires Go 1.25.0 or newer. The development host has WebView2 and Node.js/npm available and its Wails doctor check passes.

## Decision

1. Create an isolated Wails v2.17.0 shell under `tools/ui-spike/wails-v2` with its own Go module, frontend dependencies and local build instructions.
2. Do not add Wails dependencies to the root `go.mod`, do not change the core Go 1.24.8 baseline, and do not make this shell the production player yet.
3. Keep the prototype intentionally narrow: idle, simulated three-shot session, and result screen. The physical camera, production renderer, output coordinator and printer are not wired to it.
4. Add a hidden benchmark mode that verifies 100 sequential JavaScript-to-Go round trips and reports p50/p95 plus backend-startup-to-frontend-ready time. Use an external process sampler to observe working set/private bytes separately.
5. Require explicit Linux validation on a selected distribution before making a cross-platform support claim.

## Evidence

- Wails CLI v2.17.0 installed; `wails doctor` reports the Windows environment ready.
- Frontend build, Wails Windows build, and isolated Go module tests pass.
- Three hidden benchmark runs each completed 100 sequential round trips and wrote a report.
- Mean of the per-run medians: 0.267 ms; mean of the per-run p95: 0.367 ms.
- Average backend-startup-to-frontend-ready: 92.1 ms.
- Highest observed sample in the three process runs: 24.92 MB working set and 18.53 MB private bytes. Sampling is approximate and does not establish peak RSS/heap or behavior during real camera/render workloads.
- The resulting Windows executable is approximately 11 MB.

## Consequences

- The project has early measured evidence for a WebView-based desktop shell without binding the core module to it.
- The core remains buildable with Go 1.24.8, but the UI experiment currently needs Go 1.25.0 because of Wails v2.17.0.
- The measurement does not include visible input latency, actual photo preview, a physical camera, renderer or printer.
- Wails adoption requires a follow-up comparison on low-cost booth hardware and an actual Linux target, including GTK3/WebKit2GTK dependencies, installation, full-screen behavior, input handling and resource use.
- If those constraints are poor for the product, discard the spike rather than forcing the core toolchain or architecture around it.
