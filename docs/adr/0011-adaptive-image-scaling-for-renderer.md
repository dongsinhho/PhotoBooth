# ADR 0011: Adaptive image scaling for frame rendering

**Status:** Accepted for the schema-v1 renderer
**Date:** 2026-10-25

## Context

The first realistic local renderer benchmark found that applying Catmull-Rom scaling to every source-to-slot transform created high allocation volume. PhotoBooth commonly reduces 1280x720 UVC camera frames into smaller frame slots, so a full high-kernel downscaler costs both time and transient memory on the main capture-to-output path.

## Decision

1. For downscaling (destination rectangle is no larger than the source crop in both dimensions), use `ApproxBiLinear`, which has reasonable speed and quality and avoids large kernel scratch allocations.
2. Retain Catmull-Rom for upscaling, where sharpness and quality matter more and the approximation is less suitable.
3. In cover mode, calculate a centered source crop with the destination aspect ratio and scale that source rectangle directly into the target slot. Do not allocate a full resized image that will immediately be cropped.
4. Keep the frame schema and user-facing options unchanged. Add regression tests for centered crop, extreme aspect ratios and the pixel difference between adaptive downscale and a Catmull-Rom reference.

## Evidence

Windows 11 Pro x64, AMD Ryzen 5 8600G, Go 1.24.8. The benchmark rendered a schema-v1 1200x1800 three-photo strip from deterministic textured 1280x720 synthetic images; ten iterations were measured for each variant.

| Metric | Catmull-Rom for all scaling | Adaptive scaler |
|---|---:|---:|
| Render time | 171.75 ms/op | 102.26 ms/op |
| Bytes allocated/op | 73,139,144 B | 17,279,337 B |
| Allocations/op | 265 | 245 |

The adaptive variant was approximately 40% faster and allocated approximately 76% fewer bytes per render on this fixture. A pixel-difference regression test remains green. A follow-up run with a real Logitech B525 captured three 1280x720 stream frames, rendered the 1200x1800 strip, and saved local output; all five image artifacts decoded with the expected dimensions, and test files were deleted.

## Consequences

- Rendering is faster with substantially lower allocation volume for the tested downscaling workload.
- ApproxBiLinear is a quality/performance compromise compared to Catmull-Rom; a future frame-schema version may expose a supported quality preset if production evaluations warrant it.
- This benchmark does not measure peak memory, camera acquisition, live preview, printer output, or long-run stability.
- Representative real photos, controlled-lighting samples, additional aspect ratios, and target booth hardware must be measured before claiming consistent performance or image quality.
