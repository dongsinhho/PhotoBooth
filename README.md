# PhotoBooth

PhotoBooth is planned cross-platform photobooth software that combines reliable on-site photo capture with configurable themes/frames and optional online content services.

Status: Product definition and technical proof-of-concept stage. An experimental Go Windows Media Foundation provider has passed an end-to-end Logitech B525 capture-render-local-save run and three consecutive compiled-executable sessions. The player UI, printer path, reliable recovery/soak validation, backend, and marketplace are not yet implemented as a product.

## Runnable local development workflow

Synthetic-camera demo (does not open a physical camera):

```powershell
go run ./apps/booth-player --camera=fake --countdown=0s --interval=0s
```

Experimental Windows Media Foundation run with the reference Logitech B525 (this opens the physical camera and captures real images):

```powershell
go run ./apps/booth-player --camera=windowsmf --device="B525 HD Webcam" --width=1280 --height=720 --fps=30 --countdown=0s --interval=0s
```

Both modes run preflight, capture the configured number of photos, render the versioned JSON frame, and save a session-scoped output. The B525 adapter captures frames from an MJPEG video stream; it does not issue a dedicated DSLR-style still-shutter command. Default working captures/rendered files are under `data/sessions/`, and the finalized local copy is under `data/output/`; these paths are git-ignored. Real-camera test images may contain people or private surroundings: inspect locally and delete them after verification. The number of shots must match the frame's photo-slot count.

On 2026-10-25, three consecutive runs of the built executable each completed three real B525 captures, rendering and local save; a separate go run pass verified all capture and output dimensions. In the built-binary runs, first-frame capture was 321-329 ms, later captures were 39-64 ms, pre-adaptive render was 143-155 ms, and local save was 2-10 ms. A separate go run took about 2.02 s including process startup; these are early smoke measurements, not a production benchmark or soak result. The old Python/OpenCV webcam POC remains a separate experiment and reference.

The output stage uses an independent-target coordinator. The local-storage target is implemented; a printer target can be added independently, and tests verify that one target failure does not suppress another. No physical printer adapter is included yet.

## Performance and UI experiments

The schema-v1 renderer now uses adaptive downscaling: on the same textured three-photo fixture, Catmull-Rom-only measured 171.7 ms/render and 73.1 MB allocated/op, while the adaptive path measured 102.3 ms/render and 17.3 MB allocated/op. See docs/knowledge-base/RENDERING_PERFORMANCE.md for commands and limitations. A separate Wails v2 kiosk-shell spike lives under tools/ui-spike/wails-v2; it requires Go 1.25.0 in its own module and is not the production UI decision.

The upstream Windows camera wrapper is vendored as a narrowly patched copy under `third_party/windows-camera-go`. See `README.PHOTOBOOTH-PATCH.md` there for the upstream version, MIT notice, and CGO compatibility change. The upstream project warns that it is under development; treat this adapter as experimental until repeated-session, hot-unplug, cancellation, and image-quality validation passes.

## Product vision

Help booth operators reuse cameras, computers, and printers they already own. The official app is intended to be free for commercial photobooth operations. Owners may configure supported application behavior and add supported frames/themes without editing core code. Optional paid support and authorized custom engineering are the intended business model.

## Product surfaces

1. PhotoBooth Player: desktop kiosk app for camera control, guest sessions, rendering, printing/export, optional payment, and media delivery.
2. Application-level customization: settings, configuration, frame/theme packages, and documented content formats; not core-source editing.
3. Optional photo delivery: local-only storage, controlled LAN downloads, or a configured cloud service.
4. Optional payments: free mode, payment during the guest flow, or verified prepayment.
5. Future content catalog: browse, preview, and install content; paid/community distribution may be added later.
6. Store configuration: device settings, content, media policy, payment policy, and retention.

## Core principles

- Free-to-use commercial operation is the intended default for the official app; support is optional.
- Normal customization happens through supported app settings and frame/theme content, not source-code editing.
- Core modification or distribution of a modified core build requires separate permission/licensing under the intended controlled-core model; final legal terms are pending.
- Do not label the core Open Source unless the final license permits modifications and derived works under the Open Source Definition.
- Capture, rendering, and local output should work offline once configured and assets are installed, except where a selected payment flow requires verification.
- Camera and printer providers are extensible, with explicit tested compatibility.
- Photo delivery and payment are independently configurable per store.
- Guest photos stay local by default.

## Proposed architecture

The player contains the kiosk UI and supported settings, session orchestrator, camera provider, render pipeline, print/export adapter, optional payment adapter, optional media-delivery adapter, and local database/asset/photo store. The optional cloud platform may provide the Go API, content catalog, orders/entitlements, and object storage/CDN.

This is a target proposal, not a description of implemented code.

## Initial technical direction

- Backend: Go is preferred; start with a modular monolith.
- Player UI: evaluate cross-platform choices such as Avalonia/.NET and Qt with a hardware/kiosk proof of concept.
- Camera: begin with the available Logitech B525 UVC webcam; laptop integrated cameras are a required separate test case. DSLR/mirrorless support is a distinct future provider capability.
- Windows 11 Pro 64-bit detected the B525. A disposable POC read 50 of 50 frames at 1280x720 using both DirectShow and Media Foundation. A JSON-configured three-photo strip rendered to a 1200x1800 JPEG and decoded successfully; 10 renderer tests passed. Concurrent camera access failed clearly, then capture worked after the holder released the device. Temporary images were deleted.
- One temporary frame was visually inspected: readable but soft/noisy under current indoor lighting and framed too close/off-center. Controlled booth lighting/mounting, production frame rendering/printing, hot-unplug recovery, built-in laptop camera, Linux, and sustained reliability remain untested.
- Local data: SQLite and local asset/photo storage are candidates.
- Cloud data/assets: PostgreSQL and S3-compatible storage/CDN are candidates, not locked decisions.
- Media delivery: local-only, LAN, or optional platform cloud.
- Payment: disabled/free, QR during flow, or verified prepayment.

## Distribution and business model

The product does not sell hardware. The official application is intended to be free for commercial booth use. Support, setup, training, hardware integration, and authorized core modifications may be paid services.

Supported settings and theme/frame formats are the normal customization path. The standard user grant does not permit core-source modification or redistribution of modified core builds. See docs/knowledge-base/LICENSE_STRATEGY.md; final legal text is not yet published.

## Discovery and technical documents

- Product plan: docs/knowledge-base/PRODUCT_PLAN.md
- Product requirements: docs/knowledge-base/PRD.md
- Competitor context: docs/knowledge-base/COMPETITOR_RESEARCH.md
- Self-service adoption hypotheses: docs/knowledge-base/DISCOVERY_LOG.md
- Camera integration: docs/knowledge-base/CAMERA_INTEGRATION.md
- Camera compatibility evidence: docs/knowledge-base/CAMERA_COMPATIBILITY_MATRIX.md
- Webcam POC and test instructions: tools/camera-poc/README.md
- AI coding-agent instructions: AGENTS.md
- Knowledge-base index: docs/knowledge-base/README.md

The feedback/interview guide is optional and not a project gate. The next work is to validate image quality, recovery, production rendering/output, laptop camera, and Linux behavior. Maintain the distinction between tested, planned, and untested features.
