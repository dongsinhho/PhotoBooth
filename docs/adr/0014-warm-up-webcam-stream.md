# ADR 0014: Warm up webcam stream before the first photo

**Status:** Accepted; current-scene A/B diagnostic completed 2026-10-31, representative booth-lighting validation remains open
**Date:** 2026-10-28

## Context

The Wails player opens a Windows Media Foundation MJPEG stream and then starts the guest-facing countdown. Previously, no frames were read between stream startup and the first capture. A B525 user test observed that the first photo could be slightly darker than the subsequent photos, consistent with UVC auto-exposure or white balance needing time to settle. The provider captures a frame from a video stream rather than triggering a dedicated still-camera shutter.

## Decision

After Media Foundation starts the stream, consume and discard a bounded warm-up period before reporting the provider as open. Default to one second, calculate the frame count from the negotiated stream FPS, cap the duration at three seconds and frame count at 120, and never save warm-up frames as guest photos. Repeat this behavior whenever the recovery wrapper opens a fresh provider. If a warm-up read fails, fail that open attempt with the frame index in the diagnostic instead of silently starting from an unusable stream.

Do not brighten only the first captured photo in the renderer. Preserve the camera's original pixels and treat lighting/exposure comparison as a hardware validation task.

## Consequences

- Camera startup takes approximately one additional second at the default 30 FPS.
- The first saved photo is captured after discarded frames have given UVC automatic controls a chance to settle.
- A stream failure during warm-up may consume an initial-open retry; cancellation during synchronous native reads remains subject to the existing Media Foundation limitation.
- The mitigation does not guarantee identical exposure across different scenes or lighting and does not establish production image quality.

## Validation

- Unit tests cover frame-count calculation, bounds, warm-up duration defaults/cap, exact frame discard count, early failure and original error preservation.
- On 2026-10-31, three real Wails B525 sessions (3 shots each, 3s countdown/1s interval) completed 9/9 capture-render-save. First-shot drop vs. median of later shots was 0.000%, 0.000% and 0.660%; all nine frames were 1280x720 and passed the gross-darkness/relative-consistency diagnostic gate. This is one current-scene diagnostic, not representative portrait certification.
- A diagnostic-only A/B command option and `tools/camera-poc/warmup-ab-test.ps1` compared three sessions with warm-up disabled against three with the default one-second warm-up (alternating run order; 3 shots/session). All 6 sessions and 18 captures passed capture/render/local-save and the current diagnostic quality gate. Mean signed first-shot drop was 0.047% with warm-up disabled and 0.010% with warm-up enabled, a measured reduction of 0.037 percentage points. Mean first-shot luminance was 131.278 vs. 130.537 and mean later-shot luminance was 131.340 vs. 130.550. The difference is very small and ambient/scene drift is a confounder; this does not justify increasing warm-up or changing gain/exposure. Keep the one-second default.
- A fixed-lighting, fixed-placement, representative portrait/booth-scene before/after visual assessment remains required before claiming a verified guest-facing image-quality improvement.
