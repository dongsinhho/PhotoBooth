# ADR 0025 — Low-RAM host readiness and system memory-pressure telemetry

**Status:** Accepted for validation tooling; the low-RAM target is a test profile, not a published minimum system requirement.
**Date:** 2026-11-01

## Context

ADR 0024 identified substantial WebView2 private bytes on the B525/Windows 11 reference machine and showed that a diagnostic GPU-off run reduced the observed process-tree peak. That machine has approximately 15 GiB of visible physical memory and cannot establish whether the Wails candidate is suitable for a low-cost 8 GiB booth PC.

A process-only memory report is insufficient for this decision. The same process-tree allocation can behave differently depending on host-wide available physical memory, system commit headroom and paging activity.

## Decision

1. Use a Windows 11 x64 machine with **8 GiB installed RAM** as the next low-RAM validation candidate. The harness classifies a host as eligible when Windows reports no more than **8,192 MiB visible physical memory**. This is a test target only; it is not yet an official hardware minimum.
2. Do not establish a CPU/GPU minimum from the development host. The first reference low-RAM unit should record its exact CPU, graphics adapter, Windows build and camera model.
3. Add a read-only preflight mode to `ui-preview-kiosk-soak.ps1`:
   - `-PreflightOnly` reports the host, installed/visible memory, initial available memory, commit state and whether it matches the test profile.
   - `-RequireLowRamHost` refuses to start the webcam or application unless the host is within the configured physical-memory ceiling.
   - `-LowRamMaxPhysicalMemoryMB` can explicitly override the default 8,192 MiB ceiling for a separately documented test profile.
4. Sample system memory counters alongside process-tree metrics at each resource interval:
   - Available physical bytes and percentage of total visible RAM.
   - Committed bytes, commit limit and commit percentage.
   - `PagesInputPersec` and `PageReadsPersec`.
5. Write these fields to the resource CSV and aggregate first/last/minimum/peak values into JSON and Markdown reports. Prefer `Win32_PerfFormattedData_PerfOS_Memory`; fall back to `Win32_OperatingSystem` for available physical memory when performance counters are unavailable.
6. Treat paging/commit counters as host-wide Windows indicators, not PhotoBooth-specific attribution and not dedicated GPU memory. Do not create artificial memory pressure, change pagefile settings or disable services to qualify a machine.
7. Keep GPU acceleration enabled by default. GPU-off remains a diagnostic comparison only.

## Validation on 2026-11-01

Host: ASRock X600-ITX / Ryzen 5 8600G / Radeon 760M / Windows 11 Pro build 26200.

- Windows reports 15,484 MiB visible physical RAM. Preflight classified it as `above-low-ram-target`.
- `-PreflightOnly` reported the host without opening the camera or starting PhotoBooth.
- `-PreflightOnly -RequireLowRamHost` correctly rejected the host before camera/application startup.
- A 1.02-minute telemetry smoke produced 23 resource samples and a valid host-memory CSV/JSON/Markdown report. Preview mean FPS was 4.72 with zero ACK/decode failures in the observed UI metrics.
- The normal soak gate reported FAIL because the one-minute window did not complete the required Settings and fullscreen/windowed lifecycle cycles. This was an instrumentation smoke, not a passing soak.
- During that short run, available physical memory fell from 1,699.20 MB at the first sample to a minimum of 1,361.88 MB (8.80% of visible RAM); commit peaked at 96.67%. System-wide `PagesInputPersec` and `PageReadsPersec` peaks were 4,585 and 47 respectively. These are host-wide counters; the run does not prove that PhotoBooth alone caused the observed pressure.
- The disposable profile was cleaned.

Report: `%TEMP%\photobooth-preview-soak-db22bee67dbb4b3480b4289eb859213f\summary.json`.

## Consequences and remaining limits

- Future soak runs can be filtered to the intended 8 GiB class and will include host memory pressure evidence rather than just process private bytes.
- ShinPc is explicitly not a low-RAM validation machine. The only connected Desktop Commander device during this session was ShinPc.
- No actual 8 GiB machine was tested. Low-RAM UI responsiveness, capture/render/local-output, integrated-GPU behavior and sustained memory headroom remain unvalidated.
- This ADR does not publish an official minimum specification, certify Windows 11 on 8 GiB, or approve Wails as the production UI.
