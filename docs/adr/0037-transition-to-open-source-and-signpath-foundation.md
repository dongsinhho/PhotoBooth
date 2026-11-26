# ADR 0037: Transition PhotoBooth to Open Source for SignPath Foundation

**Status:** MIT selected for transition; license and SignPath eligibility audit still in progress.

The product owner explicitly requested an open-source transition and free SignPath Foundation signing on 2026-11-26. MIT is selected as the permissive license for the implementation; the root LICENSE was added. This does not imply SignPath acceptance or completion of the asset/dependency audit.
**Date:** 2026-11-26
**Decision owner:** Product owner

## Context

The product owner explicitly reversed the prior controlled-core licensing decision and requested that PhotoBooth become open source and use the free SignPath Foundation GitHub signing service.

The prior product intent reserved core modification, derivative builds, rebranding and redistribution of modified core for separate permission. That is incompatible with open source and SignPath Foundation's current eligibility terms.

At transition start, the repository was public but had no root LICENSE/COPYING file. Public visibility alone did not grant open-source reuse rights. The root MIT LICENSE and draft signing policy are now in the working tree. The repository currently has no GitHub Actions workflow and no public GitHub Release/download page.

## Decision

1. Replace the controlled-core licensing model with a genuine OSI-approved open-source license for all PhotoBooth-owned code and build scripts.
2. Do not use commercial dual licensing or retain proprietary core components if applying for SignPath Foundation's free certificate.
3. Preserve the existing product/business direction: no hardware sales, free commercial photobooth use, optional paid support/setup/training/consulting/custom engineering. Optional paid services must not restrict rights granted by the open-source license.
4. **MIT is selected** as the permissive license for PhotoBooth-owned code, subject to completing ownership and third-party asset review. The root LICENSE file has been added.
5. MIT permits commercial use, modification and redistribution, including derivative works, while requiring the copyright and license notice to be retained. The project will not use commercial dual licensing for the SignPath Foundation application.
6. SignPath Foundation remains an application/approval gate, not a guaranteed entitlement. Its current terms require an OSI-approved license for all components without commercial dual licensing, no proprietary code, active maintenance, an already released binary, a download page describing the product, source-built artifacts, a public code-signing policy, MFA, author/reviewer/approver roles, GitHub trusted build provenance and manual release approval.
7. If accepted, the Windows certificate publisher will be **SignPath Foundation**, not the individual maintainer. The signing private key is held by SignPath's HSM. Signed artifacts attest to verified source/build origin according to its policy; this does not mean the maintainer receives a personal certificate.
8. Use the current GitHub integration `signpath/github-action-submit-signing-request@v3`, upload the unsigned artifact with `actions/upload-artifact@v7` (current official SignPath GitHub integration example), use GitHub-hosted runners for all jobs leading to signing, and pin third-party actions to reviewed full commit SHAs in the production workflow.

## License choice

MIT has been selected and a root `LICENSE` file added. The copyright notice currently uses `PhotoBooth contributors`; confirm that the repository owner has the authority to license all project-owned contributions and replace/augment the notice if needed. MIT allows commercial use, modification and redistribution, including derivative works, provided the copyright and license notice are retained. It does not require downstream forks to publish their changes.

The MIT choice is recorded in this ADR and the root LICENSE. Do not use an improvised custom license or commercial dual licensing for the SignPath Foundation application.

## Repository audit findings (initial, not complete; license file added)

- The root MIT LICENSE file was added as part of this transition; it covers PhotoBooth-owned code only and does not override third-party licenses.
- The only tracked vendored code license identified in the initial inventory is the MIT license for `third_party/windows-camera-go`.
- The Go module includes `golang.org/x/*`, `gopkg.in/yaml.v3`, and the Windows camera dependency; the Wails UI spike has additional Go and npm dependencies.
- The repository also contains scripts, documentation, examples, frame/theme data, UI assets and third-party files. These require a complete source/asset and dependency license review before public licensing.
- Local Python virtual environments and generated `node_modules` are present in the working tree but are not treated as project source unless tracked or included in release artifacts. The release/package ignore and artifact rules must be verified.
- The root .github directory currently contains issue templates only; no workflow has been implemented.

## SignPath Foundation integration plan

### Gate A - Legal/source ownership
- Confirm the selected OSI license and exact scope.
- Ensure all PhotoBooth-owned source, scripts and build configuration are covered.
- Remove or replace any proprietary/non-OSI component that SignPath terms prohibit.
- Preserve required third-party copyright/license notices and generate a third-party license inventory.
- Confirm frame/theme sample assets, fonts, icons, images and documentation illustrations have compatible redistribution rights; exclude non-compatible assets from signed distributions or replace them.

### Gate B - Release readiness
- Publish an initial reproducible release from source.
- Create an official download/release page with functionality and installation instructions.
- Add a public **Code signing policy** containing the required SignPath attribution, maintainer/committer and reviewer/approver roles, and privacy statement.
- Establish MFA and release approval roles.
- Apply for SignPath Foundation and wait for explicit project acceptance.

### Gate C - Trusted GitHub build
- Add a GitHub-hosted Windows workflow for build/test/package; no self-hosted jobs before the OSS signing request.
- Link the SignPath GitHub App/Trusted Build System and configure repository/branch/commit origin verification.
- Upload the unsigned installer as a GitHub Actions artifact, submit it through action v3, wait for approval/signing, and download the signed artifact.
- Require manual approval for every release signing request.
- Verify signed output and timestamp; publish only the signed installer and checksum/provenance.
- Keep the existing release manifest/payload verification, but update any publisher-thumbprint assumptions that currently expect the actual maintainer's own certificate. SignPath Foundation is the signer subject. Do not disable verification merely to make the new signer pass; update trust policy intentionally and test it.

## SmartScreen / user trust

SignPath Foundation describes its certificate as providing a clear link between the repository and the binary. It does not guarantee that every new download will immediately be free of every Windows SmartScreen warning. Do not promise "no installation warnings" without validating the current Windows behavior and reputation.

## Next steps

1. Complete a tracked source/dependency/asset license audit and resolve incompatible files/components.
2. Confirm the MIT copyright holder/ownership and update notices as required.
3. Assign an independent reviewer and release approver; configure GitHub MFA/branch protection.
4. Publish an initial source-built release and official download page.
5. Complete the Code signing policy page and apply to SignPath Foundation; wait for explicit acceptance.
6. Refactor release packaging/signature verification so SignPath's certificate subject and signed artifact flow are supported.
7. Add the trusted GitHub Actions workflow using action v3, manual approval and origin verification; enable production signing only after SignPath project configuration is ready.

## References

- SignPath Foundation terms: https://signpath.org/terms
- SignPath Foundation overview: https://signpath.org/
- SignPath GitHub trusted build: https://docs.signpath.io/trusted-build-systems/github
- SignPath GitHub action v3: https://github.com/SignPath/github-action-submit-signing-request
- SignPath project and approval setup: https://docs.signpath.io/projects
- Open Source Definition: https://opensource.org/osd
