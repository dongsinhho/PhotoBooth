# ADR 0009: Experimental Windows Media Foundation camera adapter

**Status:** Accepted for experimental Windows validation; not production-certified
**Date:** 2026-10-25

## Context

The session runner, frame renderer and local output path were implemented in Go, while the first successful webcam check used a separate Python/OpenCV POC. Before selecting a UI framework, the project needed to determine whether a native Windows camera stream could feed the same Go pipeline without making Python a required runtime dependency.

The Logitech B525 is the available reference camera on Windows 11 Pro 64-bit.

## Decision

1. Implement the first native camera adapter behind the platform-independent camera.Provider contract in internal/booth/camera/windowsmf.
2. Use Windows Media Foundation for this experimental Windows provider. Keep its COM/native lifecycle calls on a dedicated goroutine locked to one OS thread.
3. Pin upstream github.com/Kirizu-Official/windows-camera-go at v0.2.0 in third_party/windows-camera-go, preserving the upstream MIT license and attribution. The upstream project warns that it is under development.
4. Carry a narrow local CGO-compatibility patch in the vendored copy: remove unused C imports and replace C.GoBytes with a Go-owned byte copy from the locked native buffer. The unpatched upstream source does not build with CGO_ENABLED=0 despite its README claim.
5. Select a negotiated MJPEG stream format by requested resolution and target FPS. This provider captures a frame from the video stream; it does not claim dedicated still-shutter capture or an original DSLR file.
6. If an MJPEG sample omits Huffman-table markers, insert the standard JPEG DHT tables before passing it to Go's JPEG decoder. Leave complete JPEGs unchanged and test both cases.
7. Keep native provider details out of the session orchestrator. On non-Windows builds, the same package exposes an explicit unsupported-platform implementation.
8. Instrument per-shot capture, render and local-save timing. Event handlers execute on the session goroutine and must return quickly; a future UI adapter should enqueue events rather than block capture.
9. Treat this implementation as experimental until repeated sessions, camera-busy handling, cancellation, physical unplug/reconnect, controlled-lighting image quality, and resource/soak testing are performed.

## Evidence observed on 2026-10-25

- A standalone probe initialized Media Foundation, enumerated the Logitech B525 and found 471 device formats, including 1280x720 MJPEG at 30 FPS.
- The first integrated run captured all three frames but Go's JPEG decoder rejected abbreviated MJPEG because DHT markers were absent. OpenCV decoded the same samples. The adapter was updated to normalize this format difference and unit tests were added.
- Three consecutive runs of the built executable each captured three real B525 frames at 1280x720, rendered a 1200x1800 JPEG, and saved session-scoped local output. A separate run verified all five expected artifacts by decoding them and checking dimensions.
- OpenCV verified all three capture dimensions and both rendered-output dimensions. Temporary real-camera photos were deleted after verification.
- A separate go run took about 2.02 seconds with countdown/interval disabled, including process startup. In the three built-executable runs, capture timings were 321-329ms for the first frame, 39-49ms for the second, 63-64ms for the third; render took 143-155ms and local save 2-10ms. These are smoke measurements, not a performance benchmark.
- Go unit tests passed after integration. Full Windows build/vet and a Linux cross-build must be rerun against the final revision before this commit is considered complete.

- In one concurrent probe, the Go Media Foundation provider completed a full session while a Python/OpenCV Media Foundation holder was also open on the B525. This is an observation for this device/driver combination, not a blanket multi-client guarantee.

## Consequences and limitations

- The main Go session pipeline can now capture from a real UVC webcam on Windows, render locally and save without Python at runtime.
- The first provider is specific to Windows Media Foundation. It does not make the camera path Linux-compatible; Linux needs a separately tested provider such as a V4L2-backed implementation.
- Synchronous native frame reads cannot be forcibly cancelled in the middle by context.Context. The provider uses bounded shutdown waiting, but a blocked driver call and hot-unplug behavior may still need additional process-level isolation or platform-specific recovery.
- The wrapper is experimental and must remain isolated behind camera.Provider. Re-evaluate its maintenance, native-resource behavior and license before updating the vendored version.
- This ADR does not select a UI framework, add preview, implement printer output, or establish production image quality or reliability.
