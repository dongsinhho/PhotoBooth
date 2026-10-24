# Instructions for AI Coding Agents

These instructions apply to the entire repository.

## Before changes

1. Read README.md and docs/knowledge-base/README.md.
2. Read relevant product, architecture, security, domain, and package documents.
3. Inspect current files and implementation. Do not assume planned modules already exist.
4. Distinguish implemented behavior from proposals; documentation is not proof that code exists.
5. If documents conflict, follow an accepted ADR and latest user-approved requirement. Surface unresolved conflicts rather than guessing.

## Product invariants

- Once configured and provisioned with required assets, the capture path must not require live Internet access.
- Keep camera control, session workflow, rendering, printing/export, UI presentation, and cloud sync clearly separated.
- Downloaded themes/assets must not execute arbitrary code.
- Theme changes must not silently alter capture, output, or privacy behavior.
- Do not upload guest photos by default. Cloud delivery requires explicit configuration and a retention policy.
- Unsupported hardware and package versions must fail clearly.
- Do not add network calls to the critical capture path without an explicitly approved requirement.

## Engineering practices

- Prefer the simplest architecture that meets current needs. No microservices, brokers, Redis, or other infrastructure without demonstrated need.
- Isolate device/vendor SDKs behind narrow adapters. Document model, OS, and licensing limitations.
- Validate external inputs and downloaded packages. Bound resource sizes, retries, and media-processing time.
- Make error paths observable and recoverable; avoid losing a session due to transient device, disk, or network failures.
- Add or update tests. Document tests requiring physical hardware.
- Avoid unrelated refactors and unrequested dependency upgrades.
- Never commit credentials, signing keys, guest photos, or sensitive production configuration.
- Never claim a build, test, hardware check, or deployment passed unless it actually ran and the result was observed.

## Documentation maintenance

- Update knowledge-base documents when product behavior, architecture, schemas, privacy, security, or operations change.
- Add an ADR under docs/adr/ for consequential, hard-to-reverse decisions.
- Keep examples aligned with implementation and label conceptual examples.
- Preserve the statuses Proposed, Accepted, Implemented, and Deprecated.
- Record unresolved questions instead of inventing decisions.

## Change workflow

1. Summarize intended behavior and constraints.
2. Inspect affected files and test approach.
3. Implement the smallest coherent change.
4. Run relevant tests/formatters/builds available.
5. Report changed files, actual checks/results, and remaining risks.


## Additional product invariants

- Media delivery is a store-level policy: local-only, controlled LAN download, or optional platform cloud. Do not upload by default.
- LAN download must expose only the authorized output/session, not the whole photo directory; use short-lived access tokens and expiry.
- Payment is optional and store-configurable: disabled/free, payment during the guest flow, or verified prepayment.
- When payment is required, only trusted server/provider confirmation can authorize a paid session. The player must not trust a screenshot, QR display, redirect, or client-side success claim.
- Payment and media delivery are independent of camera provider code and should use separate interfaces and state.
- Windows and Linux are product targets, but support must be claimed per tested OS/camera/driver/printer combination.
- Go is the preferred backend language. Keep the backend a modular monolith unless concrete evidence justifies splitting it.

## Distribution and licensing invariants

- The product does not sell hardware. The intended basic player offer is free commercial use, with optional paid support and authorized custom engineering.
- Keep supported store customization in documented settings and versioned theme/frame packages wherever possible.
- Never imply that a public repository grants permission to modify or redistribute the core.
- Read docs/knowledge-base/LICENSE_STRATEGY.md before adding a LICENSE file, publishing source code, accepting external code contributions, or designing plugin redistribution.
- Do not call the core OSI open source unless the actual selected license meets the Open Source Definition.
- Do not implement artificial feature restrictions merely to pressure users into purchasing support; paid support is optional under the intended product model.
