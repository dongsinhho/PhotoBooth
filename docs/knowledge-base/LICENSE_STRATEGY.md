# Licensing Strategy

**Status:** Open-source transition accepted by product owner; exact OSI license and full source/asset audit pending
**Supersedes:** Controlled-core licensing direction recorded 2026-10-25
**Last updated:** 2026-11-26
**Decision record:** [ADR 0037](../adr/0037-transition-to-open-source-and-signpath-foundation.md)

## Product and business model

PhotoBooth is software-only. The project does not sell cameras, computers, printers, cabinets, or bundled booth hardware.

The accepted direction is to release PhotoBooth as genuine Open Source under an OSI-approved license, and to apply for free code signing through SignPath Foundation.

The intended offer remains:
- The official application is free to use for commercial photobooth operations.
- Operators may self-install and self-operate without purchasing support.
- Optional paid services include setup, troubleshooting, training, camera/printer integration, migration, consulting, and custom engineering.
- Paid services must not restrict rights already granted by the project's open-source license.
- Optional paid frame/theme content or a future community store may have separate creator/content terms, provided they do not re-license or restrict the open-source core.

This file records current product intent and transition status; it is not final legal advice.

## Open-source license

The exact license has not yet been selected. **MIT is recommended** because it is permissive, straightforward, supports commercial use and redistribution, and aligns with broad adoption and optional paid support. The owner must confirm the license before the root LICENSE file is created.

Other candidates:
- GPL-3.0-only: permits commercial use and modification, but imposes copyleft/source obligations when distributing covered binaries.
- Apache-2.0: permissive with an explicit patent grant and additional notice requirements.

Do not retain a proprietary core, prohibit modifications/derivative works, or use commercial dual licensing if applying for SignPath Foundation's free OSS certificate. Do not describe the project as MIT/GPL/Apache until that exact license is selected and published.

## Rights and consequences of the open-source transition

Under the selected OSI license, users will receive the rights specified by that license, including the ability to inspect, modify, and redistribute the covered source and derivative works. Commercial use is allowed by OSI-compliant open-source licenses.

The previous controlled-core restriction—requiring separate permission to modify core, distribute modified builds, or create derivative works—is superseded and must be removed from current product documentation and terms.

The project may continue to:
- Provide official supported builds and installation documentation.
- Sell optional support, training, setup, integration, and consulting.
- Maintain official branding/trademarks separately from the copyright license, subject to a clearly documented trademark policy.
- Offer separately licensed content only where the content is not part of the open-source codebase and rights are clearly described.

## Third-party code and asset audit

Before publishing the license:
- Audit every tracked source file, build script, workflow, vendored dependency, submodule, generated source and packaged binary.
- Verify all dependencies and included components are compatible with the selected OSI license and SignPath Foundation's no-proprietary-component condition.
- Preserve third-party license and copyright notices.
- Audit fonts, icons, images, screenshots, sample frame/theme assets, documentation illustrations and release payloads.
- Remove or replace non-compatible assets and binaries; do not assume a local development virtual environment or untracked node_modules is part of the project release.
- Generate a third-party notices/license inventory and ensure release packaging includes required notices.

## SignPath Foundation conditions

SignPath Foundation's current public terms require:
- An OSI-approved license for all components without commercial dual licensing.
- No proprietary/non-open-source code (except permitted system libraries).
- An actively maintained project, already released in the form to be signed, with functionality described on a download page.
- The signing team to own/develop/maintain the repository and sign its own source-built binaries.
- MFA, defined author/reviewer/approver roles, a public Code signing policy and privacy statement.
- Verifiable GitHub-hosted build provenance, trusted build configuration and manual approval for every release.

The SignPath Foundation certificate is issued to SignPath Foundation; the Windows publisher identity will be SignPath Foundation, not the maintainer's personal name. SignPath holds the private key in its HSM. Project acceptance is not guaranteed.

## Migration gates

1. Owner selects the exact OSI license (MIT recommended).
2. Complete source/dependency/asset license audit and resolve incompatible items.
3. Add the root license and required third-party notices.
4. Update README, PRD, product brief, ADRs and public project pages to remove controlled-core restrictions.
5. Publish a real release/download page and public Code signing policy.
6. Establish MFA and author/reviewer/approver roles; configure GitHub rules/review.
7. Apply to SignPath Foundation and wait for acceptance.
8. Implement the GitHub Trusted Build System workflow using the supported SignPath action, with manual release approval.
9. Verify signature, origin metadata, installer behavior and Windows warnings before public release.

## References

- Open Source Definition: https://opensource.org/osd
- SignPath Foundation terms: https://signpath.org/terms
- SignPath GitHub trusted build: https://docs.signpath.io/trusted-build-systems/github
