# Camera Integration

**Status:** Required architectural constraint; providers and support matrix remain open

## Goal
Allow new camera providers/models to be added through versioned application releases without rewriting session orchestration or UI.

## Core design
- Define a platform-neutral camera capability contract.
- Put vendor SDKs and native interop behind provider-specific implementations.
- Advertise capabilities such as Connect, CaptureStill, TransferImage, LiveView, SetCameraSettings, and FocusControl only where actually supported.
- Expose camera identity, firmware, provider/SDK version and diagnostics for supportability.
- Handle disconnect, busy state, capture timeout, transfer failure and cancellation explicitly.
- Never assume all camera models support the same controls or Linux/Windows targets.

## Provider responsibilities
- Discover/select supported devices.
- Open and release the SDK/device session safely.
- Capture and retrieve original files.
- Translate vendor errors into stable internal errors while retaining diagnostic details.
- Report capabilities and SDK compatibility.
- Respect vendor SDK redistribution, license, and support requirements.

## Version and update model
- Initially ship provider binaries as part of a signed/released player build; avoid arbitrary downloaded native plugins.
- Add model/SDK support through a tested application update.
- Keep provider version and supported model/OS matrix in release notes.
- Do not update a provider or player during a photo session; use staged update and rollback.
- If providers later become separately updateable, require signature verification, capability contracts, compatibility checks, rollback, and a strict trust boundary.

## Validation matrix
For every supported combination, record:
- Camera brand/model, firmware and connection mode.
- OS distribution/version/architecture.
- SDK/provider version and required drivers/permissions.
- Trigger capture, original transfer, live view if required, reconnection behavior and long-run stability.
- Known unsupported capabilities and recovery procedure.

## Evaluation strategy
Start with one camera model commonly used by pilot customers. Compare official vendor SDKs and open-source options such as libgphoto2 against exact required operations and models. Do not equate appearing on a supported-camera list with full remote-control compatibility. Confirm license, OS support, and feature limitations against current vendor documentation before making a release claim.


## Reference links for compatibility research

- [libgphoto2 supported cameras](https://gphoto.sourceforge.io/proj/libgphoto2/support.php) — distinguish image download support from remote-trigger/live-view capabilities.
- [Sony Camera Remote SDK](https://support.d-imaging.sony.co.jp/app/sdk/en/) — verify supported models, firmware, OS architecture and interfaces.
- Verify any Canon/vendor SDK against its current official documentation, including supported models, OS, redistribution and license requirements.
