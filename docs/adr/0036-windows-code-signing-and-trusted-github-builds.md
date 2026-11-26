# ADR 0036: Windows Code Signing, Publisher Identity, and Trusted GitHub Builds

**Status:** Superseded by ADR 0037 (2026-11-26). SignPath Foundation is now the selected free signing route, subject to eligibility review and project acceptance.
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

**Historical result (2026-11-26):** SignPath Foundation was not compatible with the prior controlled-core license. The product owner has now reversed that decision and selected MIT. Eligibility must be re-evaluated against ADR 0037; application remains subject to full dependency/asset audit and SignPath acceptance.

SignPath Foundation's published terms require, among other things:

1. An OSI-approved Open Source license for all components, with no commercial dual-licensing.
2. No proprietary/non-open-source code or components (apart from permitted system libraries).
3. An actively maintained project that has already been released in the form to be signed and has functionality described on a download page/app-store entry.
4. The signing team must own, develop and maintain the repository and sign its own source-built binaries.
5. MFA for SignPath and source-control access; explicit author/reviewer/approver roles.
6. A public **Code signing policy** on the project homepage and release/download pages, including SignPath attribution, team roles and a privacy statement.
7. Verifiable CI provenance; every release requires manual signing approval.

At the time this ADR was originally written, the project failed the license/proprietary-code criteria and lacked the released/download-page prerequisites. The product owner later reversed the licensing decision; see ADR 0037. Changing only repository visibility or adding an action still does not satisfy the remaining release, policy, team and trusted-build requirements.

The Foundation certificate is issued to **SignPath Foundation**, so Windows displays SignPath Foundation as the certificate publisher, not the PhotoBooth rights holder. SignPath Foundation says it performs an initial project/reputation check and verifies each release's origin against the configured source repository. It stores the signing private key in its HSM; the key is not placed in GitHub or the repository.

The current supported GitHub integration is `SignPath/github-action-submit-signing-request@v3`. Do not use the older `SignPath/_archived-github-actions` repository; it is archived. The supported action submits a GitHub workflow artifact, waits for the signing request and downloads the signed artifact. For OSS verification, SignPath requires all jobs preceding signing to run on GitHub-hosted agents and uses origin metadata supplied by GitHub. A GitHub App/Trusted Build System link is also required.

### Historical: what would have to change to use SignPath Foundation (now superseded by ADR 0037)

Only revisit this path if the product owner deliberately changes the licensing model so that the entire signed project (including source, build scripts and components) is released under an OSI-approved license, with no commercial dual-licensing or proprietary core. This grants modification and derivative-work rights and therefore conflicts with the current controlled-core model. It also requires a real public release/download page, Code signing policy, role groups, MFA, reviewer/approver process and acceptance of manual approval on every release.

Historical guidance only: do not add an OSI license file merely to qualify for a certificate unless the product owner intentionally changes the product's legal rights. The owner has now intentionally selected MIT in ADR 0037.

## Historical alternatives considered before the open-source transition

The following options were evaluated before the product owner selected MIT and SignPath Foundation. They are retained for history; ADR 0037 is now authoritative.

### A. Traditional publicly trusted Authenticode Code Signing certificate (preferred candidate)

Select the certificate subject class only after the actual rights-holder/publisher is identified:
- If PhotoBooth is owned/published by a registered legal entity, obtain an organization-validated Code Signing certificate (commonly called OV) in that entity's exact legal name.
- If PhotoBooth is genuinely owned/published by an individual, ask CAs that issue individual Code Signing certificates about their identity/address validation and publisher-name behavior. Do not call an individual certificate OV or assume every CA offers this product.

Use a CA-supported HSM, hardware token or cloud/remote-signing service. Do not export a private key into the repository or store an unrestricted PFX in GitHub secrets.

Advantages:
- Preserves the current controlled-core source/licensing model.
- The Windows publisher identity can match the real rights holder, whether an individual or an organization.
- Can be integrated with GitHub Actions through the CA's approved remote-signing client/API or HSM/KSP integration, subject to provider support.

Constraints:
- The CA must validate the real publisher identity. Do not use a nominee, false address or entity that does not actually own/control the software.
- Obtain a supported RFC 3161 timestamp service and retain the certificate chain.
- A publicly trusted Code Signing certificate does not guarantee a zero-warning first download. SmartScreen reputation accumulates over time; use a consistent signer and sign all production releases.

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

## Historical decision and next dependency (superseded by ADR 0037)

- Do not use self-signed for production.
- Historical: do not apply for SignPath Foundation under the prior controlled-core licensing intent. This was superseded by the 2026-11-26 MIT/open-source decision in ADR 0037.
- Historical preferred candidate: a publicly trusted Authenticode Code Signing certificate issued in the real publisher identity. Superseded by the SignPath Foundation route selected in ADR 0037.
- Historical next dependency: publisher identity selection was completed as Individual, then superseded by the SignPath Foundation open-source signing decision in ADR 0037.
- After identity is confirmed, compare CA remote-signing options and integrate the selected provider into the existing release chain. Keep the production publisher thumbprint unset until the real certificate is independently verified.

## References

- SignPath Foundation eligibility/terms: https://signpath.org/terms
- SignPath Foundation overview: https://signpath.org/
- SignPath GitHub trusted build integration: https://docs.signpath.io/trusted-build-systems/github
- SignPath GitHub Action v3: https://github.com/SignPath/github-action-submit-signing-request
- Microsoft code signing options and availability: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
- Microsoft Artifact Signing geographic prerequisites: https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart
- Microsoft SmartScreen reputation: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation
- CA/Browser Forum Code Signing Baseline Requirements v3.11.0 (includes organization and individual identity validation): https://cabforum.org/working-groups/code-signing/requirements/


## Individual publisher follow-up - 2026-11-26

The product owner selected **Individual** as the intended PhotoBooth publisher identity. The next provider-selection task is therefore individual-validated Authenticode Code Signing (IV), not organization OV.

### Research outcome

**Primary candidate to contact: SSL.com IV Code Signing + eSigner for Code.** This is a candidate, not an approved purchase; country acceptance and account-level details must be confirmed in writing first.

Official SSL.com product information states:
- IV Code Signing verifies the individual's personal name and does not require a business entity.
- The published base certificate price is USD $129/year.
- eSigner for Code cloud signing is a separate subscription; published Tier 1 is $15/month with 240 signings/year and one credential (annual subscription shown as $180/year). SSL.com advertises a 30-day eSigner trial.
- IV Code Signing is compatible with eSigner cloud signing; the private key remains in SSL.com's cloud HSM rather than being exported to GitHub.
- SSL.com publishes GitHub Actions integration for eSigner CKA + `signtool.exe`; the Windows CKA registers the cloud certificate in the Windows certificate store so the existing Authenticode signing calls can potentially use it.
- SSL.com's individual validation asks for government-issued photo ID, address evidence and a face/ID comparison photo; the validation guide describes individual applicants and the IV process.
- Public Code Signing certificates are now short-lived (maximum approximately 459 days under current CA/B requirements); multi-year purchase is a subscription/reissue term, not one long-lived certificate. Timestamp each signature.

Reference pages:
- SSL.com IV Code Signing: https://www.ssl.com/products/software-integrity/code-signing/iv/
- SSL.com eSigner for Code / pricing: https://www.ssl.com/products/software-integrity/signing-service/
- SSL.com validation process: https://www.ssl.com/how-to/validation-process-for-document-signing-code-signing-and-ev-code-signing-certificates/
- SSL.com eSigner GitHub Actions/CI guide: https://www.ssl.com/how-to/integrating-esigner-with-ci-cd-pipelines-a-complete-setup-and-configuration-guide/
- Microsoft code-signing geographic guidance: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options

Estimated first-year list price if annual Tier 1 is selected: $129 certificate + $180 eSigner = **about $309 USD before taxes/fees**, subject to quote, eligible country, billing and any required add-ons. Tier 1 is 240 signing operations/year; expected PhotoBooth release volume should fit comfortably if each final release signs a small number of files. Confirm whether all internal Authenticode signatures count separately and how a package/retry consumes credits.

### Geographic/identity caveat

Microsoft confirms individual Azure Artifact Signing is currently limited to US/Canada, so Azure Public Trust is not the route for a Vietnam-based individual.

SSL.com publishes an IV product and worldwide/cloud-signing claims, but the reviewed public pages do **not explicitly confirm issuance to an individual resident in Vietnam**. Do not assume acceptance based only on the product page. Obtain written confirmation from SSL.com before purchase, including the exact Certificate Subject/Publisher name for a Vietnamese ID and address.

### Required pre-purchase confirmation

Contact SSL.com sales/support and obtain answers to:
1. Can SSL.com issue IV Code Signing to an individual resident in Vietnam, using a Vietnamese passport/ID and Vietnam residential address?
2. What exact Subject/Publisher name will appear in Windows UAC and `Get-AuthenticodeSignature`? Will it be the full legal name as transliterated/entered on the accepted identity documents?
3. Can this exact IV certificate be enrolled in eSigner for Code Tier 1 and used with GitHub-hosted Windows Actions via eSigner CKA + `signtool.exe`?
4. What are the full first-year and renewal costs, including certificate, eSigner subscription, tax, validation, and any credential/token fee? Confirm 30-day trial and cancellation/renewal behavior.
5. What is the supported workflow for MFA/TOTP on unattended GitHub Actions, and how are eSigner credentials/TOTP secrets protected, rotated and revoked? Confirm whether the service requires a long-lived TOTP seed in Actions Secrets and whether an approval/authorization policy can constrain signing to protected release workflows.
6. Does the subscription support Authenticode signing for `.exe`, `.dll`, `.ps1` and the final NSIS Setup EXE, with RFC 3161 timestamping? Confirm timestamp service URL and recommended digest.
7. What is the maximum certificate validity per issuance and how are reissues handled during the purchased term?
8. Provide the CA chain/root name and confirm it is in the Microsoft Trusted Root Program for standard Windows Authenticode verification.

### GitHub Actions architecture candidate

If SSL.com confirms eligibility and security controls:
- Use GitHub-hosted `windows-latest` for build and package stages; no physical USB token is needed.
- Protect the signing job with a GitHub Environment (required reviewer), restrict it to protected release tags/main, and never expose signing credentials to PR/fork workflows.
- Store only provider credentials/config in GitHub Environment Secrets; never store a private key/PFX. Use ephemeral runner files and clean them after signing.
- Configure eSigner CKA, load the cloud certificate into the runner's Windows certificate store, obtain the verified certificate thumbprint, then call the existing release scripts using that thumbprint and timestamp URL.
- Keep provider authentication separate from ordinary build permissions; audit signing requests and retain CI logs.
- The existing builder must be refactored into ordered stages if needed: sign inner EXE/helper, compute hashes, create signed release manifest, assemble NSIS package, sign final Setup.exe, then verify every signature/timestamp and enforce the pinned publisher identity.
- Do not commit a workflow until provider account, supported auth method, certificate subject and exact signing API behavior are confirmed.

### Alternatives / comparison

- **MuaSSL.com IV CodeSign**: local reseller advertises IV personal Code Signing and hardware token/HSM. Current public listing shows a lower multi-year price, but issuer/chain and cloud remote signing/GitHub-hosted runner support are not sufficiently clear. Consider only if they provide the exact issuing CA, chain, and a supported remote cloud-signing method in writing. A USB token alone cannot be attached to a GitHub-hosted runner.
- **Sectigo Individual Code Signing**: Sectigo's CP/CPS documents individual validation; local reseller lists a Sectigo IV product. It appears to require hardware-protected keys. A physical token would require a carefully isolated self-hosted signer or a separate supported cloud signing service; not the preferred GitHub-hosted CI path unless remote signing is confirmed.
- **SSL.com IV + eSigner**: currently the strongest match to the requirement for personal publisher identity plus hosted GitHub Actions signing, subject to Vietnam eligibility and security/pricing confirmation.

### Current gate

Status: **Provider shortlist complete; SSL.com written eligibility/technical confirmation pending.**
No certificate has been purchased. No GitHub secrets or production workflow have been created. Production publisher thumbprint remains unset.