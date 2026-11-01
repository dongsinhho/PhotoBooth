# PhotoBooth Hardware Compatibility Report

Use this template when GitHub issue forms are unavailable. Report a specific combination and test; do not generalize one result to all Windows/Linux cameras.

## Build and host

- PhotoBooth version / commit:
- OS edition, version, build, architecture:
- PC / CPU:
- Go version:
- UI path: root CLI / Wails prototype / other:

## Camera

- Manufacturer and exact model:
- Firmware (if known):
- Connection: direct USB / hub / integrated / other:
- Cable / port / hub details:
- Provider/backend:
- Driver / SDK version:
- Requested format, resolution and FPS:
- Detection result:
- Stream-open/read result:
- JPEG capture result:
- Disconnect / recovery result:

## Output

- Frame schema/version and slot count:
- Capture count:
- Local output target:
- Render/export result:
- Printer make/model and OS/driver (only if a real printer test was performed):
- Physical print result: not tested / pass / fail:

## Reproduction

- Exact redacted command:
- Steps:
- Expected:
- Actual:
- Stable error code/HRESULT:
- Repeat count and successful/failed attempts:
- Lighting / camera distance / framing notes:

## Privacy and artifacts

- Synthetic workflow succeeds? yes / no / not tested
- Guest photos or webcam images attached? **Must be no.**
- Logs redacted for usernames, private paths, serials, access tokens and credentials? yes / no
- Payment screenshots, banking details or QR payloads attached? **Must be no.**

Keep photos locally. If visual review is needed, arrange a local review without uploading identifiable guest images to the issue tracker.
