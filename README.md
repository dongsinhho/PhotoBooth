# PhotoBooth

PhotoBooth is planned cross-platform photobooth software that combines reliable on-site photo capture with configurable themes/frames and optional online content services.

Status: A Windows MVP UI candidate is runnable for local evaluation; production release and broad hardware support remain unvalidated. Start with [the Windows Quickstart](docs/QUICKSTART_WINDOWS.md); issue/compatibility reports and read-only environment inspection are documented below. The experimental Go Windows Media Foundation provider passed 100 consecutive B525 sessions in one process after native resource-lifecycle cleanup (300/300 captures, 500/500 JPEGs verified); sampled working-set/private bytes did not rise across the run. The bounded recovery wrapper passed a physical USB unplug/replug test, recovering the same in-flight capture in 23.906 seconds without restarting the process. The Wails prototype has passed real capture-render-save and a two-trial UI automation suite covering same-session Continue and fresh-session restart after forced camera contention; four sessions completed in one app process and test artifacts were removed. The night-time image-quality batch measured mean luminance 2.52-2.54/255 and failed the gross-darkness gate. A 2026-10-29 morning retest passed the coarse darkness/first-shot-consistency gate across 9/9 captures, with first-shot drops <=0.122%; the inspected scene still lacked representative booth/portrait lighting, so final photo quality is not certified. A 2026-10-29 persistent same-process endurance run passed 1,000 sessions (3,000 captures, 1,000 renders, 1,000 local outputs; 5,000/5,000 JPEGs verified), with zero failed sessions and one Go goroutine after GC throughout. Working Set/Private Bytes peaked at 41.93/42.52 MB; first-vs-last decile averages rose by 2.90/3.20 MB, so repeat on target booth hardware before making a broad no-leak claim. Representative lighting/framing, physical USB unplug during capture, actual driver-native-read cancellation, printer adapter, backend and marketplace remain open. The long run was headless; it does not certify the Wails kiosk. Windows worker-level cancellation/deadline simulation and bounded Close-retry regression tests now pass.

## Current engineering handoff

The latest implementation state, verified test results, environment constraints and next acceptance gates are tracked in [HANDOFF_2026-11-02.md](HANDOFF_2026-11-02.md). It supersedes the 2026-11-01 handoff for current status.

## Self-service setup and support

- [Windows development quickstart](docs/QUICKSTART_WINDOWS.md) — verified synthetic path first, optional experimental B525 capture second.
- [Windows troubleshooting](docs/troubleshooting/WINDOWS_TROUBLESHOOTING.md) — camera busy/disconnect, image quality, frame mismatch, output and Wails profile issues.
- [Read-only environment inspection](tools/self-service/README.md) — checks Go, config/frame, local storage paths and optional Windows PnP camera names without opening the camera or modifying files.
- [Compatibility report template](docs/COMPATIBILITY_REPORT_TEMPLATE.md) and [GitHub issue forms](.github/ISSUE_TEMPLATE/) — report exact hardware/software evidence without uploading guest images or secrets.
- [Local output recovery](docs/knowledge-base/LOCAL_STORAGE_RECOVERY.md) — disk-full behavior and save-only recovery with `go run ./apps/output-retry --config <config> --session-id <id>`.

## Runnable local development workflow

### Windows desktop UI candidate

From the repository root, run:

```powershell
.\run-photobooth.ps1
```

This builds the Vite frontend and isolated Wails Windows app, then starts `tools/ui-spike/wails-v2/build/bin/PhotoBooth.exe`. The app opens on the Idle screen; with the reference B525 connected, the local live preview appears there. Preview intentionally stops while a capture session or Settings is active, and resumes when the player returns to Idle.

Requirements: Windows 10/11 with WebView2 Runtime, Node.js/npm, Go with automatic toolchain selection enabled, and Wails CLI v2.17.0. The nested module selects Go 1.25.0 without changing the root core's Go 1.24.8 baseline.

Useful variants:

```powershell
.\run-photobooth.ps1 -NoLaunch # build only
.\run-photobooth.ps1 -SkipBuild # launch the latest already-built candidate
.\run-photobooth.ps1 -Dev # Wails development mode
.\run-photobooth.ps1 -Kiosk # launch fullscreen kiosk mode
```

Kiosk mode is opt-in. It starts fullscreen, **Esc** exits fullscreen without closing PhotoBooth, and **F11** toggles fullscreen at any time. The on-screen hint remains visible while kiosk mode is active. The default launch remains a normal resizable window.

The launcher refuses to start a second instance and never terminates an existing window. Close an older `PhotoBooth Kiosk UI Spike` window, then run `.\run-photobooth.ps1 -SkipBuild` to open the newly built `PhotoBooth · Windows Preview` window. This is the current Windows MVP UI candidate for evaluation, not a production-certified release or a Linux compatibility claim. See [ADR 0021](docs/adr/0021-windows-mvp-ui-candidate.md).

Synthetic-camera CLI demo (does not open a physical camera):

```powershell
go run ./apps/booth-player --camera=fake --countdown=0s --interval=0s
```

Experimental Windows Media Foundation run with the reference Logitech B525 (this opens the physical camera and captures real images):

```powershell
go run ./apps/booth-player --camera=windowsmf --device="B525 HD Webcam" --width=1280 --height=720 --fps=30 --countdown=0s --interval=0s
```

Both modes run preflight, capture the configured number of photos, render the versioned JSON frame, and save a session-scoped output. The B525 adapter captures frames from an MJPEG video stream; it does not issue a dedicated DSLR-style still-shutter command. Default working captures/rendered files are under `data/sessions/`, and the finalized local copy is under `data/output/`; these paths are git-ignored. Real-camera test images may contain people or private surroundings: inspect locally and delete them after verification. The number of shots must match the frame's photo-slot count.

## Versioned local station configuration

The root Go player can load a versioned JSON station profile. The sample intentionally uses the synthetic camera:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json
```

For a real B525 on Windows, explicitly select the physical camera:

```powershell
go run ./apps/booth-player --config ./examples/store-config-v1.json --camera=windowsmf --device="B525 HD Webcam" --countdown=0s --interval=0s
```

CLI flags only override settings when explicitly supplied; omitted flags retain the config values. Schema v1 supports the camera/capture/frame/local-storage settings, local media delivery and disabled/free payment mode. Unknown fields, unsupported schema versions, invalid values and unimplemented cloud/payment modes fail validation. See [STORE_CONFIG.md](docs/knowledge-base/STORE_CONFIG.md). The same schema-v1 package also backs the Wails prototype's **Settings & frame** screen, which persists shot count, countdown, interval and frame path locally after validating frame slots. The Wails module remains isolated from the core toolchain; launch the Windows UI candidate from the repository root with `./run-photobooth.ps1`. It is not production-certified and does not expose cloud/payment options.

## Local frame packages (schema v1)

The Go core now supports a small, offline-only frame package format, .pbframe. Create a package from a validated schema-v1 frame:

~~~powershell
go run ./apps/frame-package create --frame ./examples/frames/photo-strip-v1.json --out ./photo-strip-1.0.0.pbframe --id photo-strip --version 1.0.0 --name "Sample photo strip" --author "Your name" --license "CC0-1.0"
go run ./apps/frame-package validate --package ./photo-strip-1.0.0.pbframe
go run ./apps/frame-package install --package ./photo-strip-1.0.0.pbframe
go run ./apps/frame-package list
~~~

Install validates, verifies payload hashes and stores an immutable package version locally; it does not activate it. The Wails prototype also has **Settings & frame → Frame packages** to validate, install, activate, roll back and remove a local frame. Activation aligns the capture count with the verified frame-slot count and saves a rollback point; rollback restores a validated prior frame and keeps the package installed. Remove is available only for inactive versions that are no longer referenced by pending activation or rollback state.

Schema v1 only supports a manifest, one frame JSON definition, one optional preview PNG/JPEG and optional LICENSE.txt. It does not execute code, fetch remote content, verify creator identity or authenticate license claims. Full themes/catalog/marketplace/signatures and payment remain out of scope. See [FRAME_PACKAGE_FORMAT.md](docs/knowledge-base/FRAME_PACKAGE_FORMAT.md) and [ADR 0017](docs/adr/0017-local-frame-packages-and-rollback.md).

## Local data retention (opt-in)

The separate retention CLI can list old local session/output directories. It does not run when the booth player starts. Both `storage.session_retention_days` and `storage.output_retention_days` default to `0` (disabled); the CLI can override them without changing the profile.

First run a read-only plan against the intended booth profile:

~~~powershell
$config = Join-Path $env:LOCALAPPDATA 'PhotoBooth\store-config-v1.json'
go run ./apps/retention-cleanup --config $config --session-days 30 --output-days 30
~~~

Review every printed path, then copy `PLAN_DIGEST` from that run and explicitly confirm the same policy/root/candidate set:

~~~powershell
$digest = 'PASTE_THE_REVIEWED_PLAN_DIGEST_HERE'
go run ./apps/retention-cleanup --config $config --session-days 30 --output-days 30 --apply --confirm-plan $digest
~~~

The placeholder is not a real token. Apply refuses a missing or mismatched digest. If eligible data changes between review and apply, run a new dry-run and review it again. Only complete recognized PhotoBooth sessions and finalized `photo.jpg` output directories are candidates; symlinks, incomplete sessions, temporary files and unknown layouts are left untouched. This is not secure erasure and is not a substitute for backups. See [LOCAL_DATA_RETENTION.md](docs/knowledge-base/LOCAL_DATA_RETENTION.md) and [ADR 0018](docs/adr/0018-opt-in-local-data-retention.md).

On 2026-10-25, three consecutive runs of the built executable initially established the capture-render-save path. A later 100-session reliability batch completed 300/300 captures, 100 renders and 100 local copies; all 500 JPEGs decoded with expected dimensions. Mean session wall time was 0.919 s, median 0.910 s, p95 0.960 s. The earlier pre-adaptive-render phase timings are historical. This is a repeated fresh-process reliability test, not a long-lived kiosk or production certification.

The output stage uses an independent-target coordinator. The local-storage target is implemented; a printer target can be added independently, and tests verify that one target failure does not suppress another. No physical printer adapter is included yet. For the repeatable Windows B525 reliability harness, see docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md.

## Performance and UI experiments

The schema-v1 renderer now uses adaptive downscaling: on the same textured three-photo fixture, Catmull-Rom-only measured 171.7 ms/render and 73.1 MB allocated/op, while the adaptive path measured 102.3 ms/render and 17.3 MB allocated/op. See docs/knowledge-base/RENDERING_PERFORMANCE.md for commands and limitations. The Wails v2 Windows UI candidate lives under `tools/ui-spike/wails-v2` in its own Go 1.25.0 module; the root/core toolchain remains Go 1.24.8. Build and launch it from the repository root with `./run-photobooth.ps1`. It connects the home/idle screen to a local live B525 preview, stops and closes the camera before capture or Settings, and restarts preview on return to idle. A physical B525 preview → three-shot capture/render/local-save → preview-return UI smoke passed once on Windows 11; it is not a full-day kiosk soak or production UI approval. The camera workflow also uses a one-second stream warm-up, bounded recovery and local output. Current image-quality checks pass only a coarse darkness/first-shot-consistency gate; the inspected scene was not a representative booth portrait. Use `tools/camera-poc/controlled-image-quality.ps1` with intended booth lighting/framing before claiming photo quality. Physical printer support, Linux runtime/hardware, full-screen kiosk behavior and production installer/update remain unimplemented.

The upstream Windows camera wrapper is vendored as a narrowly patched copy under third_party/windows-camera-go. See README.PHOTOBOOTH-PATCH.md there for the pinned upstream version, MIT notice, CGO compatibility and native resource-lifecycle fixes. The upstream project warns that it is under development; treat this adapter as experimental. Provider-level physical USB hot-unplug/replug has passed in the same process. On 2026-10-28, an automated smoke test clicked Start with webcam in the built Wails app and completed all three real captures, rendering and local save. The captured scene during the night test was near-black, and a same-camera CLI control was similarly dark; that three-session batch measured mean luminance 2.52-2.54/255 with 99.97%-100% of pixels at grayscale <=15. A follow-up morning batch on 2026-10-29 passed the gross-darkness/first-shot-consistency gate across 9/9 captures, with session means 49.108, 50.051 and 52.515 and first-shot drops no greater than 0.122%. A visual sample still showed a dim room with no representative guest/booth lighting, so this is not a final photo-quality certification. A repeatable UI Automation harness now passed two trials in one Wails process: each trial resumed the same failed session after releasing the camera holder, then exercised Start a fresh session after a second failure; all four completed sessions captured 3/3 frames, rendered and saved locally. The harness removed all six test session/output directories afterward. Representative booth lighting/framing, further physical USB hot-unplug during active capture, full kiosk soak and actual driver-native-read cancellation remain open. Worker-level blocked-read cancellation/deadline and Close-retry tests pass; see docs/knowledge-base/CAMERA_RELIABILITY_RESULTS.md.

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
- One temporary frame was visually inspected: readable but soft/noisy under current indoor lighting and framed too close/off-center. Controlled booth lighting/mounting, physical printing, additional hot-unplug/recovery cycles, the built-in laptop camera, Linux, and long-duration kiosk reliability remain untested.
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
- Repeatable UI recovery/fresh-session test: tools/camera-poc/ui-recovery-smoke.ps1
- Read-only UI settings smoke test: tools/ui-spike/wails-v2/ui-settings-smoke.ps1
- Repeatable image-quality harness: tools/camera-poc/controlled-image-quality.ps1
- AI coding-agent instructions: AGENTS.md
- Knowledge-base index: docs/knowledge-base/README.md

The feedback/interview guide is optional and not a project gate. Next validate image quality with intended booth lighting and a representative subject, then run a longer UI/camera soak and evaluate physical printer output, laptop camera, and Linux behavior. Maintain the distinction between tested, planned, and untested features.
