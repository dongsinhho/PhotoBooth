# ADR 0026 — Idle-preview physical USB recovery validation

**Status:** Accepted as scoped hardware evidence for the experimental Windows Wails player; not a production compatibility guarantee.
**Date:** 2026-11-01

## Context

ADR 0020 defines idle-only preview using the same Windows Media Foundation worker as capture and a bounded recovery decorator for invalidated preview streams. Unit tests covered recovery after stream invalidation, but the hardware matrix did not yet contain a physical B525 unplug/replug performed while the Wails player was idle and actively displaying live preview.

The existing active-capture USB smoke is a different lifecycle and cannot substitute for this case.

## Decision

1. Add `tools/ui-spike/wails-v2/ui-idle-preview-hotplug-smoke.ps1` as a dedicated, interactive physical test. It opens a disposable local profile, starts idle preview, and never starts a photo session.
2. Confirm baseline preview frame/ACK progress before asking the operator to unplug the camera.
3. Observe removal and return of the original B525 PnP instance. Do not infer a physical unplug from a stream error alone.
4. Confirm the backend emits a `reconnecting` preview status, then a later `ready` status after reconnect. The harness reads the append-only preview JSONL because UI Automation did not reliably expose the transient reconnect label in the first attempt.
5. Require new frame/ACK progress after reconnect. On graceful shutdown, reconcile every published sequence to exactly one ACK, cancellation or timeout terminal; reject invalid JSON, duplicate/missing terminals, ACK timeouts or decode failures.
6. Confirm no photo session was created, close the app gracefully, restore process environment variables and remove the disposable profile.
7. Keep this evidence scoped to one B525, one USB port, ShinPc and Windows 11. Repeat cycles, ports and camera models remain open.

## Validation on 2026-11-01

Host and device:
- ASRock X600-ITX / Ryzen 5 8600G / Radeon 760M.
- Windows 11 Pro x64, build 26200.
- Logitech B525 HD Webcam / Windows Media Foundation.
- Experimental Wails Windows player, `PhotoBooth.exe`.

Physical run: **PASS**
- Idle preview reached ready: 5 frames / 5 ACKs; baseline before unplug reached 11 / 11.
- Windows PnP observed the original B525 instance disappear after physical unplug.
- Wails backend emitted 7 `reconnecting` status events, followed by a `ready` status after reconnection.
- Windows PnP observed a healthy B525 after reconnect; the same instance ID was present before and after.
- Preview resumed and advanced to 48 frames / 48 ACKs (37 additional frame/ACK pairs after the baseline snapshot); sampled resumed FPS was 2.32 during the short recovery window.
- Exact post-shutdown event audit: 48 published / 48 ACK; zero cancellations, ACK timeouts, decode failures, invalid JSON lines, duplicate sequences, unmatched publishes or orphan terminal events.
- No photo session was started. The app exited gracefully, the temporary profile was removed, and the camera was present/healthy after the run.

Report: `%TEMP%\photobooth-idle-preview-hotplug-c151fb720c924e19857e3013a1a4bb36.json`.

### Harness correction during validation

The first attempt observed physical PnP removal and the JSONL contained reconnecting/ready events, but the harness timed out because Windows UI Automation did not expose the transient reconnect label. That attempt was recorded as FAIL, not as a hardware recovery failure. The harness was updated to wait on backend `preview_status` events from the shared-read JSONL; the second physical run then completed PASS. The backend event log is the assertion source for state transitions; this test does not claim that a human visually inspected the reconnect label.

## Consequences and remaining limits

- There is now one real physical idle-preview recovery pass on the B525 reference host.
- This proves one tested removal/reconnect lifecycle for that exact camera/host/port and experimental build only.
- It does not establish repeated reliability, recovery across a different USB port or webcam, capture behavior during preview (capture is intentionally not active), production UI readiness, or Linux support.
- Resumed preview cadence in this short window was below the normal 5 FPS cap; longer performance/soak evaluation remains separate.
