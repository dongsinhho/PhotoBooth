# Windows Troubleshooting

**Applies to:** current development alpha and the documented Logitech B525 / Windows 11 reference setup\
**Last reviewed:** 2026-10-30

This guide is symptom-oriented. Try the low-risk checks first. Do not repeatedly start camera processes while another application owns the device. Do not send guest photos, QR/payment details, credentials, or unredacted personal paths in an issue.

## Before diagnosing

1. Run the synthetic workflow first:
   `go run ./apps/booth-player --config ./examples/store-config-v1.json`
2. Confirm the synthetic final JPEG opens.
3. Close other camera applications before testing the real B525.
4. Record the exact OS build, camera model, USB connection, provider, requested resolution/FPS and the exact command.
5. Keep guest images and raw camera samples local. Share text logs only after redacting usernames, personal paths and device serials.

## The app says no camera devices were found

**Likely causes:** camera unplugged, Windows has not completed device enumeration, device privacy/permission issue, another application owns the camera, or the selected friendly-name/device selector does not match.

Try:
- Check Windows Device Manager and confirm the exact camera is present without an error icon.
- Close Camera, Teams, Zoom, OBS, browser tabs and any other camera test tool.
- Unplug/reconnect the USB camera and wait for Windows to enumerate it.
- Check the friendly name. The reference command uses `--device="B525 HD Webcam"`; another camera may expose a different name.
- Run `go run ./apps/booth-player --camera=windowsmf --device="B525 HD Webcam" --countdown=0s --interval=0s` only for the B525 reference device.

The Media Foundation provider has a bounded reconnect window; it does not retry forever. If the recovery window expires, record the error and use the UI's Continue or fresh-session action as appropriate. Do not assume another camera or driver behaves identically.

## Camera is busy / HRESULT 0xC00D3704

The recorded B525 contention test observed this when another process held the camera.

- Close the process that is using the camera.
- Wait briefly for Windows to release it.
- Retry a new session.
- Do not run several PhotoBooth or OpenCV camera tests concurrently.

The error is not proof that the camera is defective. The existing diagnostic maps the observed HRESULT to a camera streaming/resource issue and preserves the original error.

## Camera disconnects during a capture

The B525 recovery harness passed a physical unplug/replug test in one process: the provider reopened after the device returned and the same in-flight capture completed. This is evidence for that host/device/provider combination only.

- Reconnect the same camera and USB cable.
- Let Windows finish device enumeration.
- Use Continue to resume the current session when the UI offers it; completed shots should be retained.
- Use Start a fresh session only when you deliberately want a new session ID and a new set of shots.
- If recovery fails, preserve the text error and session ID. Do not delete the session folder before reviewing whether completed captures are needed.

A real USB unplug during every possible point in an active UI capture, all USB ports/hubs and all cameras have not been certified.

## First image appears darker than later images

The Windows provider performs a bounded one-second stream warm-up and discards initial frames before the guest countdown. It does not selectively brighten the first photo.

For a useful quality report:
- Keep camera position, exposure controls, subject position and lighting fixed.
- Run at least three sessions with the same settings.
- Compare all shots, not only the rendered strip.
- Use the controlled checker documented in `tools/camera-poc/README.md`.
- Include whether the scene is representative booth/portrait lighting.

A 9/9 coarse darkness/first-shot consistency result was recorded in a morning B525 test, but the scene was still not representative booth lighting. Do not treat this as final image-quality certification.

## Session fails because the frame slot count does not match

The number of configured shots must exactly match the number of slots in the selected schema-v1 frame.

- Check `capture.shots` in the station config.
- Check `slots` in the frame JSON.
- Either select a compatible frame or change the shot count.
- The Wails settings screen validates the frame before saving. Use its Browse action to select a local frame JSON.

Do not remove slots from a shared sample file as a quick workaround without preserving a copy.

## No local output / photo.jpg missing

- Read the per-target output status in the CLI log.
- Confirm the configured output directory exists or can be created.
- Check free disk space and write permissions.
- Confirm the session completed rendering before checking the output directory.
- Expected local copy: `<output_dir>/<session-id>/photo.jpg`.

The output target writes a temporary file and renames it to `photo.jpg` after copy/sync. If local output failed, keep the session directory and logs for diagnosis. A physical printer adapter does not currently exist.

## Existing Wails settings profile is invalid

The profile is normally at:

`%LOCALAPPDATA%\PhotoBooth\store-config-v1.json`

The prototype intentionally does not silently overwrite a corrupt/unsupported profile.

- Back up the file before attempting repair.
- Open Settings & frame and use the UI's repair/save path with a valid frame and matching shot count.
- Do not delete the profile unless you understand which local settings it contains.
- The first Wails launch can initialize defaults if no profile exists.

The UI profile and root CLI config may be different files. Confirm which command/app is reading the config before editing it.

## The Wails app does not build or start

Wails v2.17.0 is in a separate nested Go module and requires Go 1.25.0 plus its Windows/WebView2 prerequisites. The root Go core stays on its own declared toolchain.

From `tools/ui-spike/wails-v2`:
- Run `go test -count=1 ./...`.
- Run `npm ci` only if frontend dependencies are missing or the lockfile needs a clean install.
- Run `npm run build` in `frontend`.
- Run `wails build -clean -nopackage`.

A successful Windows build does not validate Linux. This prototype is not the production UI selection.

## Local output fails because the disk is full

The output provider recognizes Windows disk-full signals and removes partial temporary output. It preserves the original captures and `rendered.jpg`; the session reports an insufficient-storage error with recovery guidance.

1. Free space on the configured output volume or resolve the storage quota.
2. In the Wails result screen, choose **Retry local save**. This reuses the current session's preserved rendered image and does not capture or render again.
3. For CLI-based operator recovery, keep the failed session ID from the CLI/UI result and rerun only the local save from the repository root:

~~~powershell
go run ./apps/output-retry --config .\examples\store-config-v1.json --session-id 0123456789abcdef0123456789abcdef
~~~

Use the exact failed session ID and the same station config. The command verifies all configured captures plus the rendered JPEG, does not open the camera, and refuses to overwrite an existing `photo.jpg`. It does not recapture or rerender. See [LOCAL_STORAGE_RECOVERY.md](../knowledge-base/LOCAL_STORAGE_RECOVERY.md).

## Data retention / cleanup

Do not use `--apply` until you have inspected the dry-run paths. The retention tool is disabled by default and is not run on player startup.

- Run a dry-run.
- Review every candidate directory.
- Copy the reported PLAN_DIGEST.
- Use the same roots and policy with `--apply --confirm-plan <digest>`.
- If the candidate set changes, rerun and review a new dry-run.

See [LOCAL_DATA_RETENTION.md](../knowledge-base/LOCAL_DATA_RETENTION.md). Cleanup is not secure erasure and is not a substitute for backups.

## What to attach to a support issue

- Exact reproduction steps and command.
- Windows build, Go version, app build/commit (if known).
- Camera model, USB connection, provider and stream settings.
- Which step failed: open, capture, render or local output.
- Redacted text logs and stable error/HRESULT, if present.
- Whether synthetic mode succeeds.
- Whether the issue reproduces after closing other camera applications.

Do **not** attach photos, webcam frames, rendered strips, payment screenshots, credentials, access tokens, Windows account names, or unredacted device serials. A maintainer may request a local-only visual inspection separately.
