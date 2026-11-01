# ADR 0023: Bounded canvas preview rendering and auditable kiosk soak diagnostics

**Status:** Accepted for the experimental Windows MVP UI candidate.
**Date:** 2026-11-01

## Context

The first 10-minute B525 idle-preview/kiosk soak passed the functional gates but exposed two concerns:

1. WebView2 private bytes peaked at 1,418 MB and ended at 1,315 MB on the reference Windows 11 host. The footprint is significant for low-cost booth PCs.
2. The append-only preview JSONL log contained missing ACK audit records even though the serial preview pump continued. The old harness inferred earlier ACKs from later frame publications, hiding an observability defect and producing counters that were not an exact backend event count.

The guest-facing preview previously replaced an `img.src` data URL on every frame. Each frame was capped at 5 FPS and ACK-gated, but image decode buffers and replaced DOM image resources were left to WebView2's lifetime/GC behavior.

## Decision

1. Render idle preview into one persistent HTML canvas. Decode one JPEG into an `ImageBitmap`, verify the frame is still current and the player is still Idle, draw it into the canvas, then explicitly call `ImageBitmap.close()` before ACK.
2. Keep the canvas backing store unchanged when resolution is unchanged; resize only when camera dimensions change. Clear its backing store when preview leaves the ready state.
3. Preserve the one-frame-in-flight and 5 FPS maximum backpressure contract. A stale frame decoded after Settings/capture begins is closed and never ACKed.
4. Replace per-event open/write/close diagnostics with one mutex-serialized append handle per opt-in soak run. Close it on path change, write failure, or Wails shutdown; count writer failures and retry opening on a later event.
5. On Windows, open the diagnostics file with read/write/delete sharing so the host-side sampler can read snapshots while the app appends. Keep a Unix append implementation for future platform builds.
6. Record `frame_cancelled` when context cancellation wins before the current frame's ACK. At a cancellation boundary, consume an already-queued ACK first; do not label that frame cancelled if the backend actually received the ACK.
7. Never infer an ACK solely because a later frame was published. After graceful app shutdown, the soak harness reconciles the flushed JSONL by sequence. Unmatched published frames, orphan ACKs, duplicate publish/ACK/cancel events, overlapping ACK/cancel terminals, and invalid JSON lines fail the run.

## Validation

On ShinPc / Windows 11 / Logitech B525, the updated executable completed a 10.24-minute fullscreen-kiosk soak with 195 resource samples, five Settings stop/resume cycles and five fullscreen/windowed cycles:

- 2,828 frame-published events and 2,828 ACK events; 0 cancellations, timeouts, decode failures, unmatched publishes, orphan ACKs, duplicate events or invalid lines.
- Average preview FPS 4.73; minimum sampled FPS 4.70; final FPS 4.78.
- CPU process-tree mean/peak 14.50% / 22% of one logical core.
- Process-tree private bytes first/last/peak: 344.54 / 1,130.56 / 1,285.69 MB.
- WebView2 private bytes first/last/peak: 315.20 / 1,098.87 / 1,254.63 MB.
- PhotoBooth.exe private bytes first/last/peak: 29.33 / 31.69 / 31.86 MB.
- Temporary profile was removed; no capture session or guest photos were created.

The previous same-host 10.26-minute run recorded a WebView2 private-byte peak of 1,418.22 MB and process-tree peak of 1,447.70 MB. The new run's corresponding peaks were about 11.5% and 11.2% lower. These are two independent runs, not a controlled A/B test; startup state, allocator timing and other process variation remain confounders. The new peak of 1,254.63 MB is still high for low-RAM PCs.

Latest report: `%TEMP%\photobooth-preview-soak-d9ce78c26007467fac0cee06e3f4ebb1\summary.json`, with CSV/JSONL/Markdown evidence alongside it.

## Consequences and remaining limits

- Preview uses an explicitly bounded decode/draw/close lifecycle, and the host soak now audits exact backend events rather than inferred ACKs.
- The observed memory peak improved in this repeat run, but no universal memory requirement is established and the WebView2 footprint remains substantial.
- This is a 10-minute single-host/single-camera soak, not a full-day or overnight soak. Low-RAM target hardware, multiple monitors, other cameras/ports, Linux, printer output, representative booth lighting and production UI certification remain open.
