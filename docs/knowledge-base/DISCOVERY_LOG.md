# Self-Service Adoption and Hypothesis Log

**Status:** Initial tracking document
**Last updated:** 2026-10-30

## Strategy

The primary audience is people setting up new booths and self-builders using equipment they own or source second-hand. Formal interviews and incumbent migrations are not required project gates. Grow gradually through a useful free product, reproducible installation, truthful compatibility claims, public documentation, and optional community feedback.

## Hypothesis register

| ID | Hypothesis | Observable evidence | Positive evidence | Counter-evidence | Confidence | Next action |
|---|---|---|---|---|---|---|
| H1 | New booth builders value free commercial-use software enough to try it before buying a bundle. | First-install reports and completed capture-to-output demos. | Not collected | Not collected | Low / untested | Publish a transparent alpha when usable. |
| H2 | Users can self-install with clear documentation. | Install success, setup time, issues needing direct intervention. | Documentation and maintainer-run synthetic smoke now exist; no independent first-install report yet. | Not collected | Low / untested | Collect an independent self-install report; do not infer adoption from maintainer tests. |
| H3 | Selected used camera/PC/printer setups can satisfy requirements. | Exact hardware details and repeatable tests. | Not collected | Not collected | Low / untested | Create tested compatibility matrix. |
| H4 | Supported application settings cover most desired changes. | Configuration usage, requests, and support issues. | Not collected | Not collected | Low / untested | Define config schema and sample file. |
| H5 | Optional paid support is viable without forcing it on users. | Real requests for specific tasks, time spent, repeat demand. | Not collected | Not collected | Low / untested | Track support requests when users appear. |
| H6 | Some builders prefer local/LAN delivery over mandatory cloud. | Mode selection and voluntary reports. | Not collected | Not collected | Low / untested | Keep local-first as initial baseline. |
| H7 | Linux can reduce total cost for some setups without excessive support burden. | Successful camera/printer/kiosk tests on a named distro. | Not collected | Not collected | Low / untested | Run a bounded technical spike. |
| H8 | Creators can make compatible frames/themes without changing core code. | Package creation time, validation errors, installs. | Not collected | Not collected | Low / untested | Provide sample package and schema. |

## Evidence to track

- Successful installation attempts and completed first sessions.
- Voluntary compatibility reports (model/OS/provider/output).
- Repeat usage by new builders.
- Issues resolved by documentation versus direct support.
- Organic referrals and contributions to docs/sample content.
- Types and effort of optional support requests.
- Frame/theme creation and successful install rate.

Do not treat stars, likes, downloads, or page views as proof of adoption. Never upload guest photos as telemetry.

## Optional feedback conversations

Formal interviews are optional. If a user offers feedback, use DISCOVERY_INTERVIEW_GUIDE.md. No interviews are claimed to have happened.

## Decision journal

- Date:
- Observed evidence/source:
- Assumption affected:
- Decision:
- Counter-evidence:
- Follow-up:

## Self-service documentation update (2026-10-30)

Published a Windows-first development quickstart, symptom-based troubleshooting guide, read-only environment inspector, compatibility report template, and GitHub compatibility/bug/feature issue forms. The quickstart distinguishes the recorded B525/Windows 11 reference from untested integrated cameras, printers, Linux and unimplemented LAN/cloud/payment. The inspector passed on the development PC for Go 1.24.8, sample schema-v1 config, matching 3-shot/3-slot frame, and B525 PnP name enumeration; it did not open the camera or modify/create storage. The synthetic capture-render-local-output path passed in a unique TEMP directory and the generated files were removed.

This is evidence that repository self-service material exists and its commands were exercised by the maintainer. It is **not** evidence that an independent new builder can install or operate the app without help. No independent self-install/adoption report has been collected; H2 remains unvalidated. The Windows quickstart is for a self-built development checkout, not a signed installer. Linux distro, physical printer target, production UI, update package and final user-facing license remain unresolved.
