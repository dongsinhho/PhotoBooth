# ADR 0006: Start the Camera Proof of Concept with UVC Webcams

- **Status:** Accepted; Windows webcam capture and disposable sample-render POC partially passed; production compatibility not yet proven
- **Date:** 2026-10-25
- **Decision owner:** Product owner

## Context

The project currently has a Logitech B525 HD Webcam available on a Windows 11 Pro 64-bit development machine. A built-in laptop camera is also an important low-barrier use case, but no laptop camera was present in the development machine's current device listing. DSLR/mirrorless hardware is not currently available for a hands-on integration test.

## Initial evidence (2026-10-25)

The Logitech B525 was detected by Windows 11 Pro 64-bit. A disposable OpenCV 5.0.0.93 test read 50/50 frames at 1280x720 through both DirectShow and Media Foundation and wrote a temporary JPEG through each. Three additional open/read/save/release cycles of 10 frames each passed with both backends. A JSON-configured renderer composed three captured images into a 1200x1800 sample strip; ten renderer tests passed. While one process held the camera, a second reader failed on its first frame and emitted a diagnostic; after the holder released the device, a new capture succeeded. All temporary image files were deleted. Image quality, production preview/renderer, printing, hot-unplug/reconnect, sustained reliability, laptop camera, and Linux remain untested.

## Decision

1. Start the first camera technical spike with the available Logitech B525 UVC webcam.
2. Include built-in laptop cameras as an intended camera use case and test on a laptop when one is available; do not claim it has passed because the external webcam works.
3. Treat webcam/laptop video-frame capture and DSLR/mirrorless remote-still capture as distinct provider capability paths under a shared session/render contract.
4. Keep the provider API platform-neutral and capability-driven.
5. Investigate Windows Media Foundation and Linux V4L2 for the generic UVC path. A cross-platform library may be evaluated for the proof of concept but is not selected for production solely for convenience.
6. Do not claim any capture/output support until the exact device/OS/backend workflow is tested.

## Consequences

### Positive
- The project can begin a real camera proof of concept using hardware already available.
- A no-DSLR mode can reduce entry cost and validate the frame-render workflow early.
- Camera abstraction can accommodate both webcam frame capture and future DSLR still capture without pretending their capabilities are identical.

### Trade-offs
- Webcam/laptop image quality, stream resolution, exposure, and capture latency may differ materially from DSLR stills.
- The first spike validates one USB webcam on Windows, not Linux or internal laptop cameras.
- A separate laptop test and later DSLR/mirrorless integration remain necessary for broader compatibility claims.

## Revisit when

Revisit provider priorities if a DSLR/mirrorless reference device becomes available or real usage reports show that webcam-first is not a useful entry path.
