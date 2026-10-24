# Instructions for AI Coding Agents

These instructions apply to the entire repository.

## Before changes

1. Read README.md and docs/knowledge-base/README.md.
2. Read the relevant product, architecture, security, domain, licensing, and package documents.
3. Inspect the current implementation and dependencies. Do not assume planned modules already exist.
4. Distinguish accepted requirements, proposals, and implemented behavior.
5. If documents conflict, follow an accepted ADR and the latest user-approved requirement. Surface unresolved conflicts instead of guessing.

## Product invariants

- The official app is intended to be free for commercial photobooth operation; do not add mandatory per-booth/per-session fees without explicit product approval.
- The product does not sell hardware or bundled booth kits.
- Operators may self-install and operate the app without buying support.
- User customization is at the supported application layer: settings, configuration, frame/theme content, and documented content formats.
- Do not treat application-level customization as permission to modify core source code.
- Core source modification, modified builds, and redistribution of modified core require explicit rights under a separate written authorization/license, subject to final legal terms.
- Do not describe the controlled-core product as OSI open source.
- Media delivery is configurable: local-only, controlled LAN access, or optional platform cloud. Do not upload photos by default.
- LAN delivery must authorize only the requested session/output, not expose the whole photo directory.
- Payment is optional and store-configurable: free/disabled, payment during the guest flow, or verified prepayment.
- If payment is required, only trusted server/provider confirmation can authorize a paid session.
- Payment and media delivery are separate modules from camera providers and capture orchestration.
- Keep camera and printer integrations behind adapters; support claims must match tested model/OS/driver combinations.
- Windows and Linux are target platforms, but compatibility is validated per actual hardware/software combination.
- Go is the preferred backend language; start with a modular monolith unless evidence supports splitting services.
- Capture/render/local output remain usable offline after configuration and asset installation except where a configured payment policy explicitly requires online verification.
- Downloaded themes/frames must not execute arbitrary code.
- Do not upload guest photos by default.
- Unsupported hardware and package versions must fail clearly.

## Engineering practices

- Prefer the simplest architecture that meets current needs. Do not introduce microservices, brokers, Redis, or other infrastructure without demonstrated need.
- Isolate vendor SDKs and OS APIs behind narrow adapters.
- Validate external input and packages; bound resource sizes, retries, and media-processing time.
- Make failures observable and recoverable; avoid losing sessions due to transient camera, printer, disk, or network problems.
- Add/update tests for changed behavior. Document tests requiring physical hardware.
- Avoid unrelated refactors and unrequested dependency upgrades.
- Never commit credentials, signing keys, guest photos, or sensitive production configuration.
- Never claim a build, test, hardware check, or deployment passed unless actually run and observed.

## Licensing and contribution rules

- Read docs/knowledge-base/LICENSE_STRATEGY.md and ADR 0004 before adding a LICENSE file, publishing source, accepting outside core-code contributions, or designing redistributable plugins.
- Do not infer copying, modification, or redistribution rights from a public repository.
- Do not add or promise a source-code license without explicit user approval and legal review.
- Make supported configuration/content formats flexible enough for normal booth customization without core edits.
- Define frame/theme creator rights separately from rights to the core application.
- Paid support is optional; do not deliberately cripple normal supported features to pressure users into buying it.

## Documentation maintenance

- Update relevant knowledge-base docs when product behavior, architecture, schema, privacy, security, licensing, or operations change.
- Add an ADR for consequential decisions.
- Keep examples aligned with implementation and mark conceptual examples.
- Preserve Proposed, Accepted, Implemented, and Deprecated status distinctions.
- Record important open questions instead of inventing decisions.

## Change workflow

1. Summarize intended behavior and constraints.
2. Inspect affected files and test approach.
3. Implement the smallest coherent change.
4. Run available tests, formatting, static checks, and builds.
5. Report changed files, checks actually run, observed results, and remaining risks.
