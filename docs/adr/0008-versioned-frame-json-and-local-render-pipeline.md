# ADR 0008: Versioned frame JSON and local render pipeline

**Status:** Accepted for the first runnable development slice
**Date:** 2026-10-25

## Context

The first end-to-end session needs a deterministic way to arrange captured photos without a UI framework, cloud service, or executable theme code. The repository already has a disposable Python/OpenCV renderer POC, but the player workflow should be testable in Go and should not depend on that Python environment.

## Decision

1. Implement the first supported renderer input as a declarative JSON frame definition with `schema_version: 1`. The current format includes canvas dimensions/background/border, ordered photo slots, cover/contain fitting, optional text, and JPEG quality.
2. Keep the frame definition non-executable. Do not run scripts, load native plugins, or fetch remote URLs from frame content.
3. Reject unknown JSON properties and validate colors, geometry, dimensions, slot count, text settings, quality, and intermediate-image resource limits before output allocation.
4. Accept local JPEG and PNG source photos in the Go renderer and emit a local JPEG via a temporary file and rename.
5. Require the session capture count to match frame slot count before opening the camera.
6. Use a synthetic camera and a session-scoped local-output provider to prove the offline path without physical hardware or guest photos.
7. Treat this as the renderer schema implemented by the current development slice, not as a complete marketplace Theme/Frame package specification. The broader content package specification remains proposed.

## Consequences

- Rendering and the core capture-to-save sequence can be unit- and integration-tested without a physical camera or Internet.
- A frame change does not require a core-code change as long as the frame remains within the supported schema.
- The Go workflow still needs an actual camera provider, UI/preview, printer/export validation, retention policy, and OS-specific hardware validation.
- Future format changes must be versioned and covered by compatibility tests; unknown future schema versions must fail clearly.
- Animation, GIF/video output, arbitrary layers/assets, font packaging, and full theme packages are not part of schema v1.

## Verification

On 2026-10-25, tests passed for JSON validation, geometry, cover/contain rendering, output dimensions, extreme aspect ratios, capture failure, local-output failure, preflight shot/slot matching, and synthetic offline runs. The later Windows Media Foundation provider has also passed three consecutive real B525 capture-render-local-save sessions. This does not establish controlled image quality, sustained reliability, or printer compatibility.
