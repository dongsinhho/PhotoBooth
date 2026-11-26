# ADR 0036: Windows Code Signing, Publisher Identity, and Trusted GitHub Builds

**Status:** Proposed; production signer/provider not selected. SignPath Foundation is not compatible with the currently accepted controlled-core licensing model.
**Date:** 2026-11-26
**Decision owner:** Product owner

## Context

The Windows NSIS Preview lifecycle has passed on ShinPc, but all Preview installers used for runtime acceptance were unsigned and are not distributable. ADR 0034 requires a genuine publisher certificate, timestamp authority and pinned publisher trust before production installer distribution.

The existing release implementation already has a strong local verification boundary:

- `New-PhotoBoothWindowsPackage.ps1` stages copies of `PhotoBooth.exe` and `manage-smb-remote-admin.ps1`, Authenticode-signs them when a certificate is supplied, records signer thumbprint and hashes, and creates a signed `release-manifest.ps1` envelope.
- `New-PhotoBoothWindowsInstaller.ps1` embeds the exact package, validates its checksum and payload, builds the NSIS installer, then signs the installer when a certificate thumbprint and timestamp service are supplied.
- `Release.Common.ps1` looks for the certificate in the Windows certificate store, checks Code Signing EKU/private-key availability/validity, requires a trusted timestamp and verifies the signer thumbprint. The production publisher allowlist is intentionally empty.
- The installer verifies signed release metadata and pins the accepted publisher thumbprint. Unsigned Preview requires an explicit opt-in.

The current implementation is certificate-store oriented; it has not been connected to a cloud HSM or a remote CI signing provider.

## Current project facts (verified 2026-11-26)

- The GitHub repository is public: `dongsinhho/PhotoBooth`.
- GitHub repository metadata reports `license: null`; there is no root `LICENSE`/ `COPYING` file. The only tracked license file found is the MIT license for the vendored Windows camera dependency.
- There are no GitHub Releases, no root homepage/download page, and no `.github/workflows` directory in the repository's current default branch.
- `README.md` describes a planned cross-platform MVP candidate and links to a Windows quickstart; it does not yet provide an official downloadable release page.
- `docs/knowledge-base/LICENSE_STRATEGY.md` explicitly says not to advertise the core as Open Source and intends to reserve source modification, derivative builds, rebranding and redistribution of modified core for separate permission/license.
- Legal terms and third-party redistribution review remain open.

A public GitHub repository is not, by itself, an Open Source licensed project. A repository without an explicit license does not grant general permission to reuse or redistribute its code.

## SignPath Foundation eligibility assessment

**Result: not eligible under the current approved product/licensing intent. Do not submit an application or create a SignPath Foundation signing workflow unless the product owner explicitly changes the licensing decision.**

SignPath Foundation's published terms require, among other things:

1. An OSI-approved Open Source license for all components, with no commercial dual-licensing.
2. No proprietary/non-open-source code or components (apart from permitted system libraries).
3. An actively maintained project that has already been released in the form to be signed and has functionality described on a download page/app-store entry.
4. The signing team must own, develop and maintain the repository and sign its own source-built binaries.
5. MFA for SignPath and source-control access; explicit author/reviewer/approver roles.
6. A public **Code signing policy** on the project homepage and release/download pages, including SignPath attribution, team roles and a privacy statement.
7. Verifiable CI provenance; every release requires manual signing approval.

This project currently fails at least the license/proprietary-code criteria. It also lacks the released/download-page prerequisites. Changing only the GitHub repository visibility or adding an action does not fix these blockers.

The Foundation certificate is issued to **SignPath Foundation**, so Windows displays SignPath Foundation as the certificate publisher, not the PhotoBooth rights holder. SignPath Foundation says it performs an initial project/reputation check and verifies each release's origin against the configured source repository. It stores the signing private key in its HSM; the key is not placed in GitHub or the repository.

The current supported GitHub integration is `SignPath/github-action-submit-signing-request@v3`. Do not use the older `SignPath/_archived-github-actions` repository; it is archived. The supported action submits a GitHub workflow artifact, waits for the signing request and downloads the signed artifact. For OSS verification, SignPath requires all jobs preceding signing to run on GitHub-hosted agents and uses origin metadata supplied by GitHub. A GitHub App/Trusted Build System link is also required.

### What would have to change to use SignPath Foundation

Only revisit this path if the product owner deliberately changes the licensing model so that the entire signed project (including source, build scripts and components) is released under an OSI-approved license, with no commercial dual-licensing or proprietary core. This grants modification and derivative-work rights and therefore conflicts with the current controlled-core model. It also requires a real public release/download page, Code signing policy, role groups, MFA, reviewer/approver process and acceptance of manual approval on every release.

Do not add an OSI license file merely to qualify for a certificate. That would change users' legal rights and contradict the accepted product strategy.

## Alternatives that preserve the controlled-core model

### A. Traditional public OV Code Signing certificate (preferred candidate)

Obtain a genuine OV Code Signing certificate issued by a public CA to the actual rights-holder/publisher identity (the verified individual or registered legal entity that owns and publishes PhotoBooth). Use a CA-supported HSM, hardware token or cloud/remote signing service. Do not export a private key into the repository or store an unrestricted PFX in GitHub secrets.

Advantages:
- Preserves the current controlled-core source/licensing model.
- The Windows publisher identity can match the real rights holder/legal entity.
- Can be integrated with GitHub Actions through the CA's approved remote-signing client/API or HSM/KSP integration, subject to provider support.

Constraints:
- The CA must validate the real publisher identity. Do not use a nominee, false address or entity that does not actually own/control the software.
- Obtain a supported RFC 3161 timestamp service and retain the certificate chain.
- OV signing does not guarantee a zero-warning first download. SmartScreen reputation accumulates over time; use a consistent signer and sign all production releases.

### B. Microsoft Azure Artifact Signing Public Trust

Microsoft currently lists Public Trust organizational availability in the USA, Canada, EU, UK, Australia, New Zealand, Japan, South Korea, Singapore, Switzerland, Norway and Israel; individual developers must be located in the USA or Canada. Vietnam is not on the current Public Trust list. Therefore this is not presently available to an individual developer or Vietnam-registered publisher based in Vietnam. Re-evaluate only if the real publisher identity is an organization in a supported jurisdiction or Microsoft changes eligibility. Do not misrepresent the publisher's location.

### C. SignPath.io using a project-owned certificate

A separately contracted SignPath.io service with a certificate owned by the project may be investigated if it supports the needed plan and signer configuration. This is distinct from the free SignPath Foundation certificate and does not remove the need to obtain/own a public CA certificate. No paid plan, availability or pricing is assumed by this ADR.

### D. Self-signed certificate

Allowed only for local engineering tests and controlled machines where the test certificate is explicitly installed as trusted. Never use a self-signed certificate for customer/public production distribution. The disposable test certificate used by `release-signing-smoke.ps1` remains a mechanics-only test.

## Target GitHub Actions signing architecture

Once a real publisher certificate/provider and remote-signing integration are selected:

1. Run release build/tests on GitHub-hosted Windows runners from a protected release tag based on reviewed `main`. No signing secrets are available to fork PRs or ordinary pull-request builds.
2. Generate unsigned build payload and provenance (source commit, version, toolchain, file hashes). Upload the unsigned artifact to GitHub Actions artifact storage.
3. Request signing using the selected provider's supported GitHub Action/CLI/remote HSM. For SignPath (only after eligibility/acceptance), use `actions/upload-artifact` followed by `signpath/github-action-submit-signing-request@v3`; store only the API token in GitHub Actions Secrets and organization/project/policy IDs in Variables. Pin the action to a reviewed full commit SHA when creating the production workflow.
4. Keep a human approval gate for production signing/release. Do not allow PRs, arbitrary branches, workflow re-runs or local artifacts to submit production signatures.
5. Preserve the existing signed-file chain: sign `PhotoBooth.exe`, the shipped PowerShell SMB helper, signed release-manifest envelope and final NSIS Setup EXE. Rebuild hashes/package metadata after any inner file is signed; never mutate a signed file afterward.
6. Verify every signature and timestamp on the Windows runner using the configured signer identity. The package manifest must record the actual signer thumbprint; the installer must enforce the same non-empty pinned thumbprint. Keep the pin independent of unsigned metadata.
7. Publish only the verified signed installer, release manifest, checksums and source revision/provenance. Retain the unsigned artifact as CI evidence but never present it as a production download.

The existing builder currently signs from a Windows certificate store and constructs the package/envelope in one local pass. A remote signer integration will require a deliberate build-stage refactor so signing is performed before final hashes/envelope/NSIS packaging, or a provider artifact-configuration design that preserves the same verification contract. Do not bolt the action onto the end of the current local builder without resolving this ordering.

## Decision and next dependency

- Do not use self-signed for production.
- Do not apply for SignPath Foundation under the current controlled-core licensing intent.
- Preferred candidate: public OV code-signing certificate in the real publisher identity, with CA-backed HSM/remote signing and GitHub Actions integration.
- Before provider selection, the product owner must identify the intended publisher identity: individual legal identity or an existing registered legal entity that actually owns the project. No certificate purchase or signing workflow can be completed correctly without this.
- After identity is confirmed, compare CA remote-signing options and integrate the selected provider into the existing release chain. Keep the production publisher thumbprint unset until the real certificate is independently verified.

## References

- SignPath Foundation eligibility/terms: https://signpath.org/terms
- SignPath Foundation overview: https://signpath.org/
- SignPath GitHub trusted build integration: https://docs.signpath.io/trusted-build-systems/github
- SignPath GitHub Action v3: https://github.com/SignPath/github-action-submit-signing-request
- Microsoft code signing options and availability: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
- Microsoft Artifact Signing geographic prerequisites: https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart
- Microsoft SmartScreen reputation: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation
