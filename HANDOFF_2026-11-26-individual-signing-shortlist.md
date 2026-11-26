# PhotoBooth Handoff - 2026-11-26 - Individual publisher and CA shortlist

## Current state

- Repo: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- GitHub: `dongsinhho/PhotoBooth`
- Branch `main`; last known baseline before this checkpoint `8a4a39e`; working tree clean; local main was ahead of origin by 5 commits.
- Publisher identity decision from product owner: **Individual**.
- Existing ADR 0036 is the signing/provider decision record. Continue from its latest Individual publisher follow-up section.
- No certificate/private key/HSM account/signing secret purchased or created. Production thumbprint allowlist remains empty.

## Current research decision

Primary candidate: **SSL.com Individual Validation (IV) Code Signing + eSigner for Code cloud signing**, pending written confirmation that a Vietnam-resident individual can enroll and complete identity validation.

Why:
- SSL.com offers IV Code Signing with personal verified name, no business registration requirement.
- Current public list price: $129/year for IV certificate.
- eSigner for Code Tier 1: $15/month ($180/year if annual plan), 240 signing operations/year, one credential; 30-day trial advertised.
- Estimated annual list cost: approx. $309 USD before taxes/fees, subject to provider confirmation.
- SSL.com says IV certificate can be enrolled in eSigner and cloud HSM holds private key; eSigner CKA integrates with Windows SignTool and GitHub Actions.
- SSL.com publishes individual identity validation requirements (government photo ID, address validation, face/ID comparison) and code signing GitHub Actions/CI integration.
- Current public Code Signing certificate lifetime is capped at roughly 459 days; longer subscriptions require certificate reissue/renewal, and each release must be timestamped.

References:
- https://www.ssl.com/products/software-integrity/code-signing/iv/
- https://www.ssl.com/products/software-integrity/signing-service/
- https://www.ssl.com/how-to/validation-process-for-document-signing-code-signing-and-ev-code-signing-certificates/
- https://www.ssl.com/how-to/integrating-esigner-with-ci-cd-pipelines-a-complete-setup-and-configuration-guide/
- https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options

Important unknown: the reviewed SSL.com pages do not explicitly confirm acceptance of an individual resident in Vietnam. Do not buy before written confirmation of country eligibility, name format, price, remote signing and GitHub Actions authentication.

## Supplier questions before purchase

Ask SSL.com:
1. Do you issue IV Code Signing to an individual resident in Vietnam, using Vietnamese government ID/passport and Vietnam address?
2. What exact certificate Subject/Publisher name appears in Windows UAC and Authenticode verification?
3. Can this IV certificate use eSigner for Code Tier 1 from GitHub-hosted Windows Actions through eSigner CKA + SignTool?
4. Confirm total first-year and renewal price, including certificate, eSigner subscription, taxes/fees, and whether 30-day trial/cancellation apply.
5. Confirm MFA/TOTP requirements for unattended GitHub Actions, whether TOTP seed must be stored as GitHub Environment Secret, signing authorization controls, rotation and revocation process.
6. Confirm support for Authenticode .exe/.dll/.ps1 and final NSIS Setup.exe, RFC 3161 timestamping and timestamp URL.
7. Confirm issuance validity and multi-year reissue policy.
8. Provide issuer/CA chain and confirm Microsoft Trusted Root Program compatibility.

## GitHub Actions design after confirmation

- Build on GitHub-hosted Windows runner.
- Use protected release tag and GitHub Environment required reviewer for signing.
- No signing credentials in PR/fork jobs.
- Store provider auth only in GitHub Environment Secrets; no private key/PFX.
- Install/configure eSigner CKA on runner, load certificate in Windows cert store, verify thumbprint, sign using existing Authenticode release code.
- Refactor pipeline order if necessary: inner EXE/helper signatures -> hashes -> signed manifest -> NSIS package -> final Setup.exe signature -> verify all signatures/timestamps and publisher pin.
- Clean ephemeral eSigner config/material from runner and retain audit evidence.
- Pin third-party GitHub Actions by reviewed full commit SHA in production.

## Alternatives

- MuaSSL.com IV CodeSign: local reseller listing advertises IV personal certificate and hardware token/HSM; pricing appears lower but issuer/chain and remote GitHub-hosted signing are unclear. Ask for issuing CA and cloud signing integration in writing.
- Sectigo Individual Code Signing: individual identity validation is documented; local product listing exists. Hardware-protected key means a USB token cannot be used directly by GitHub-hosted runner; only proceed if a supported cloud signer is confirmed.
- Microsoft Azure Artifact Signing Public Trust: not available to an individual developer located in Vietnam under current Microsoft geographic guidance.
- SignPath Foundation: not eligible under controlled-core license.
- Self-signed: local test only.

## Next step

Send a vendor inquiry to SSL.com (draft in the current conversation) and wait for written response. Do not purchase or create the GitHub workflow before country eligibility, actual publisher subject, cloud signer authentication and final pricing are confirmed.
