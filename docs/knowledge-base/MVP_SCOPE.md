# MVP Scope

**Status:** Proposed baseline

## Goal
Prove that the Windows player can reliably complete a capture-to-output workflow using a specific, explicitly supported hardware configuration. Build the foundation so online theme/frame delivery can be added without rewriting capture.

## MVP vertical slice
1. Launch in a kiosk-style interface.
2. Load local configuration and a built-in/sample frame.
3. Start a session and show countdown instructions.
4. Capture a configured number of photos with one supported camera model.
5. Track session state and handle cancellation/timeouts.
6. Render selected photos into a static frame at a known output size.
7. Preview and print or export through one tested output path.
8. Store session metadata and required assets locally.
9. Complete the core workflow with network disconnected.
10. Log technical failures without logging guest photos or unnecessary personal information.

## Acceptance criteria
The pilot is not complete until the exact camera, driver/SDK, Windows version, and printer/output path have been tested.

- A configured session captures the expected number of photos.
- Failed capture attempts are clear and do not silently produce an incomplete result.
- Camera disconnect or timeout does not permanently freeze the kiosk UI.
- Rendering produces predictable dimensions and correct photo-slot placement.
- Operator can recover from print/export failure without restarting the machine.
- Required assets remain available after restart and without Internet.
- Automated tests cover session transitions, rendering geometry, package/config validation, and hardware-independent error handling.

Set numeric reliability targets after establishing a baseline on real hardware.

## Include in the first milestone
- One supported camera integration.
- Local photo session workflow.
- Static frame/render pipeline.
- One tested output path; add a second only if scope permits.
- Local settings and asset storage.
- Basic diagnostics and recoverable error states.
- A small set of bundled sample frames.
- Version-controlled documentation and tests.

## Defer until capture is stable
- Public marketplace and creator payouts.
- Paid checkout and subscription billing.
- Unrestricted theme scripting or third-party plugins.
- Broad support for all cameras and printers.
- Cloud photo storage or guest accounts.
- Complex video/GIF exports.
- Advanced analytics, promotions, and multi-store management.
- Microservices, event streaming, or distributed infrastructure without demonstrated need.

## Pilot validation
Interview booth operators about:
- Most-used camera models and connection methods.
- Print sizes, printers, QR, and digital delivery needs.
- Whether guests select frames or the operator chooses a default.
- Frequency and effort of frame/theme changes.
- Offline expectations and acceptable update behavior.
- Willingness to pay for individual content, bundles, or a subscription.

