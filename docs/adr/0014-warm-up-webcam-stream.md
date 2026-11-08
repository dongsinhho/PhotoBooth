# ADR 0014: Warm up webcam stream before the first photo

**Status:** Accepted; one-second default retained after a 6-pair current-scene A/B retest on 2026-11-07
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
- A newer 2026-11-07 A/B used six alternating pairs: six three-shot sessions with warm-up disabled versus six with the one-second default (36 real B525 captures total). All 12 sessions / 36 captures passed capture/render/local-save and both arms passed the existing quality gate. Mean signed first-shot drop was -0.155% without warm-up (the first shot was slightly brighter overall) and +0.025% with one-second warm-up; the difference (no-warm-up minus warm-up) was -0.180 percentage points, so this batch does not demonstrate a measurable brightness benefit from one second. The median paired warm-up-minus-no-warm-up effect was +0.028 percentage points; five of six pair differences were within -0.121 to +0.075 pp, while one pair had a +1.099 pp scene/auto-exposure outlier in the no-warm-up arm. Absolute mean luminance also drifted between arms (129.973 vs. 132.273), so relative within-session first-vs-later comparison is more meaningful than comparing absolute arm brightness. Report: `%TEMP%\\photobooth-warmup-ab-20261107-211532.json` (SHA-256 `2DC1203A478498203CBBF8A89131E27FF352E5C84BE7F762E983C8F107D566BC`).
- Decision after the retest: retain the existing one-second warm-up as a small, bounded stream-settling precaution with only one second of startup cost; do not increase it. The experiment found no material first-shot-darkness problem even with warm-up disabled, and no repeatable image-quality gain that justifies tuning camera controls. No exposure/gain/white-balance or renderer changes are indicated. This is scoped B525/current-scene evidence, not a universal guarantee across cameras/lighting.
