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

