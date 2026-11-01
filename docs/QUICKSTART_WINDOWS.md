# PhotoBooth Windows Quickstart (Development Alpha)

**Status:** Self-build/development quickstart; not a signed end-user installer\
**Reference setup:** Windows 11 Pro 64-bit build 26200, AMD Ryzen 5 8600G / Radeon 760M, Logitech B525 HD Webcam, Windows Media Foundation, local JPEG output\
**Last reviewed:** 2026-10-30

This guide gets a developer or self-builder from a repository checkout to a synthetic demo and then, optionally, a real B525 capture. It describes the currently implemented local workflow only. It is not a claim that every Windows camera, laptop camera, printer, or Linux distribution is supported.

## 1. What works today

| Capability | Current status |
|---|---|
| Synthetic camera → 3 captures → frame render → local JPEG | Implemented in root Go core |
| Logitech B525 on the reference Windows machine | Experimental Windows Media Foundation provider; repeated tests passed on the stated host |
| Recovery after camera contention / USB reconnect | Bounded recovery implemented; test evidence is scoped to the recorded B525 runs |
| Local station config and schema-v1 frame | Implemented |
| Local .pbframe validate/install/activate/rollback/uninstall | Implemented |
| Wails touch-first kiosk shell | Windows prototype only; not selected as production UI |
| Physical printer | No printer adapter implemented |
| LAN photo sharing, cloud upload, QR payment | Not implemented |
| Laptop integrated camera and Linux hardware | Not validated |

The official-app product direction is free commercial use with optional support. This repository checkout and Wails shell are engineering/development artifacts, not a published installer. See knowledge-base/LICENSE_STRATEGY.md; final user-facing legal terms still require review.

## 2. Reference hardware and software

The evidence-backed reference configuration is:

- **OS:** Windows 11 Pro 64-bit, build 26200.
- **PC:** AMD Ryzen 5 8600G with Radeon 760M Graphics.
- **Camera:** Logitech B525 HD Webcam connected by USB.
- **Camera provider:** experimental Go 1.24.8 Windows Media Foundation (windowsmf).
- **Stream request:** 1280×720 at 30 FPS.
- **Frame:** schema v1, 1200×1800 canvas, three photo slots.
- **Output:** session-scoped local JPEG copy.
- **Root Go module:** go 1.24.8 in go.mod.

The Wails prototype is a separate module: Wails v2.17.0, Go 1.25.0, Node.js 22.12.0 / npm 10.9.0 and WebView2 Runtime. Keep its toolchain separate from the root module. Building Wails on Windows does not establish Linux UI support.

No physical printer is part of the reference configuration. The validated output target is local file storage.

## 3. Prerequisites

For the root Go workflow:

1. Windows 11 64-bit.
2. Go 1.24.8 or a compatible newer Go toolchain.
3. A repository checkout with the bundled sample config and frame.
4. For a real-camera attempt, the B525 connected and not being used by Camera, Teams, Zoom, a browser tab, OBS, or another capture process.

From PowerShell, open the repository root and verify:

~~~powershell
go version
go env GOOS GOARCH
Test-Path .\examples\store-config-v1.json
Test-Path .\examples\frames\photo-strip-v1.json
~~~

The root go.mod declares Go 1.24.8. Do not upgrade the root toolchain just to build the separate Wails spike.

## 4. First run: synthetic demo (does not access a webcam)

Run from the repository root:

~~~powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json
~~~

The sample intentionally selects camera.provider = fake. It is safe for a synthetic demo and should not open a physical camera. It captures synthetic images, renders the sample three-photo strip and writes the local output.

The sample config resolves its paths relative to examples/store-config-v1.json:

- Working session files: data/sessions/<session-id>/
- Final local copy: data/output/<session-id>/photo.jpg

The command prints the actual paths. Open the rendered/local JPEG locally to confirm the result. Do not commit generated images; they may contain private surroundings if you later use a real camera.

## 5. Optional: real Logitech B525 capture

Only continue after the synthetic run works. Close every other application that could own the camera.

~~~powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json --camera=windowsmf --device="B525 HD Webcam"
~~~

This uses the sample's three shots, 3-second countdown and 1-second interval, 1280×720 request and 30 FPS. For a controlled operator test with no countdown/delay, explicitly add --countdown=0s --interval=0s.

The provider performs a bounded one-second stream warm-up before the first guest photo. It discards initial frames; it does not brighten or otherwise post-process the first image.

**Privacy:** this command opens the physical camera and saves real photographs. Check the camera view and surroundings first. Keep the resulting JPEGs local and remove them after inspection if they are not needed. No automatic upload is implemented.

Do not claim image quality is production-ready based only on a successful capture. The recorded morning B525 check passed a coarse darkness/first-shot consistency gate (9/9 frames), but the inspected scene was not representative booth/portrait lighting. Repeat the controlled image-quality test with the intended booth lighting and camera mount.

## 6. Optional: Wails Windows UI prototype

The Wails UI is an experiment, not the selected production UI and not a packaged installer. It requires the separate Go 1.25.0 / Wails / Node toolchain described in tools/ui-spike/wails-v2/README.md.

~~~powershell
Set-Location .\tools\ui-spike\wails-v2
wails dev
~~~

Use **Try without a webcam** first. It exercises the same workflow with synthetic images. **Start with webcam** opens the physical Windows camera. The result screen shows the saved strip; the current prototype shows the latest captured still during capture, not a continuous live preview.

The Windows Wails profile is stored at %LOCALAPPDATA%\PhotoBooth\store-config-v1.json. If the existing profile is invalid, the UI reports a repair warning instead of silently replacing it. Back up the profile before manual edits.

When capture and rendering finish but the final local save fails, the result screen offers **Retry local save**. Free space or resolve the storage issue, then use that action; it reuses the preserved render and does not open the camera or repeat capture. The equivalent operator CLI is documented below.

## 7. Local settings, frames and storage

The canonical sample config is examples/store-config-v1.json. It configures fake camera, three shots, a three-second countdown, one-second interval, local storage, local media delivery and disabled payment.

A frame's slots count must exactly match capture.shots. Use Settings & frame in the Wails prototype or edit a local schema-v1 frame/config pair deliberately. The player does not silently resize or invent missing photo slots.

Local frame package operations are documented in knowledge-base/FRAME_PACKAGE_FORMAT.md. Package installation is offline and does not activate a frame until the operator explicitly chooses Activate.

Both retention settings default to 0 (disabled). Cleanup is a separate opt-in CLI and never runs on player startup. Before any deletion, run its dry-run, inspect every path and use the exact reviewed plan digest with --apply --confirm-plan. See knowledge-base/LOCAL_DATA_RETENTION.md.

When local output fails because the volume is full or a storage quota is exceeded, the player preserves captures and rendered.jpg and reports an insufficient-storage error. In the Wails result screen, use **Retry local save** after freeing space. The CLI below is an alternative for operator recovery (without reopening the camera):

~~~powershell
go run ./apps/output-retry --config ./examples/store-config-v1.json --session-id 0123456789abcdef0123456789abcdef
~~~

Replace the example ID with the failed session's actual ID. The command verifies all expected captures plus the rendered JPEG and refuses to overwrite an existing photo.jpg. See knowledge-base/LOCAL_STORAGE_RECOVERY.md.

## 8. Checks before a booth pilot

- [ ] Synthetic three-shot flow succeeds.
- [ ] The selected frame has the intended number of slots and visual layout.
- [ ] The exact camera is listed by Windows and is not in use by another application.
- [ ] Real-camera capture produces the expected number of JPEGs at the requested dimensions.
- [ ] Lighting, camera distance, framing and all shots have been visually reviewed.
- [ ] Final local photo.jpg exists and opens.
- [ ] The operator knows where session captures and final output are stored.
- [ ] Disk free space, profile backup and photo retention policy are understood.
- [ ] Printer, LAN, cloud and payment are not represented as available until separately implemented and tested.

For compatibility evidence, use knowledge-base/CAMERA_COMPATIBILITY_MATRIX.md and submit a privacy-safe COMPATIBILITY_REPORT_TEMPLATE.md.

## 9. Automated development checks

From the repository root:

~~~powershell
go test -count=1 ./...
go vet ./...
go build ./...
~~~

These checks validate the root Go core. The Python/OpenCV camera POC is separate from production code; its setup and hardware commands are in tools/camera-poc/README.md. The Wails module has its own tests/build instructions.

---

**Support boundary:** successful local capture/render/save on the B525 reference machine does not certify a physical printer, all UVC cameras, integrated laptop cameras, kiosk auto-start, Linux, LAN sharing, cloud delivery, or payment. Report the exact tested combination rather than a generic “works on Windows” claim.
