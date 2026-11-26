# PhotoBooth Handoff - 2026-11-26 - Signing provider evaluation

## Repository baseline

- Repository: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- GitHub: `https://github.com/dongsinhho/PhotoBooth` (public)
- Branch: `main`
- Local HEAD at start: `b4ab042 docs: verify SMB denial from Raspbian client`
- Remote `origin/main` at start: `757f65c docs: record NSIS lifecycle acceptance`
- Local main was ahead by 3 commits and working tree clean at start.
- ShinPc is the connected build host, but it is not used for production signing or release identity decisions.

## Product licensing constraints (source of truth)

`docs/knowledge-base/LICENSE_STRATEGY.md` says:
- Free commercial operation is intended.
- Users may customize only via supported settings/configuration/frame/theme content.
- Standard distribution does not grant core-source modification or modified-core redistribution rights.
- Core modification, derivative builds, rebranding or redistribution require prior written permission/separate license.
- Do not advertise core as Open Source under this controlled-core model.
- Final legal terms and legal review remain open; do not publish an improvised LICENSE file.

This is incompatible with the SignPath Foundation open-source certificate rules unless the product owner changes the licensing model.

## SignPath Foundation research (current official terms checked 2026-11-26)

The current GitHub repository being public is not enough to qualify. Current observed blockers:
- GitHub repo metadata returns `license=null`; there is no root LICENSE/COPYING file.
- No GitHub Releases or release tags at the time of inspection.
- No homepage/download page and no `.github/workflows` directory.
- The licensing strategy explicitly retains proprietary/controlled core rights.

SignPath Foundation requires an OSI-approved license for every component without commercial dual licensing, no proprietary code, active maintenance, a released binary and documented download page. For its certificate, it also requires the project team to own/maintain source and build scripts, MFA, author/reviewer/approver roles, public Code signing policy, privacy statement, verifiable origin, manual approval of every release and adherence to technical restrictions. The certificate subject/publisher is SignPath Foundation, not the PhotoBooth rights holder. SignPath holds the private key in its HSM and verifies the build origin per release.

Therefore: **do not apply for the SignPath Foundation certificate or wire its GitHub workflow under the current accepted controlled-core licensing model.** Simply adding an OSI LICENSE file would materially change users' legal rights and contradict `LICENSE_STRATEGY.md`.

Technical GitHub integration is available: current action is `SignPath/github-action-submit-signing-request@v3` (Node 24); the old `SignPath/_archived-github-actions` repository is archived and must not be used. SignPath's GitHub connector expects a GitHub-hosted workflow build and uploaded workflow artifact; it checks build origin metadata supplied by GitHub. The action requires API token, organization ID, project slug, signing-policy slug and artifact ID. OSS signing requires all jobs before the signing request to use GitHub-hosted runners. The public Foundation terms require manual approval for each release.

## Alternatives

1. Preferred candidate preserving product intent: a traditional public OV Code Signing certificate issued to the genuine rights-holder/publisher identity, paired with CA-supported HSM/hardware token/cloud remote-signing capability. This can integrate with GitHub Actions using the provider's supported signing client/API/KSP. Must use RFC 3161 timestamping and a consistent verified publisher identity.
2. Microsoft Azure Artifact Signing Public Trust is not currently eligible for a Vietnam-based individual or Vietnam-registered organization according to Microsoft's current country list. The list supports organizations in several regions including Singapore, but individual developers are restricted to US/Canada. Only a real qualifying legal entity should use an organization eligibility path; do not misstate location.
3. Paid SignPath.io with a project-owned public CA certificate may be evaluated separately if its plan and integration fit; no pricing or plan is assumed here.
4. Self-signed remains mechanics-only for local/controlled tests; never production/public release.

A trusted Authenticode signature does not guarantee that SmartScreen will never show a warning on first downloads. Microsoft's current guidance says OV/EV reputation builds over time; EV no longer bypasses SmartScreen since 2024. Keep signatures consistent, sign all produced releases and communicate the publisher identity.

## Existing release code

- `New-PhotoBoothWindowsPackage.ps1` locally stages and signs `PhotoBooth.exe`, the SMB helper and release-manifest envelope; embeds package metadata with hashes/signer thumbprint.
- `New-PhotoBoothWindowsInstaller.ps1` builds NSIS and optionally signs final Setup.exe.
- `Release.Common.ps1` reads a signing cert from CurrentUser/My or LocalMachine/My; validates Code Signing EKU, private key, validity and timestamp; verifies expected publisher thumbprint. Production allowlist remains empty.
- The current builder is certificate-store oriented. CI integration with a remote HSM/signing provider needs a deliberate refactor/order: sign inner binary/helper first, then calculate signed hashes and build/sign manifest envelope, package/NSIS Setup and sign the final installer. Verify all signatures after signing; never modify signed binaries afterward.

## Current decision

New ADR `docs/adr/0036-windows-code-signing-and-trusted-github-builds.md` records the evaluation and proposed route.

No GitHub Actions workflow was added yet because the project fails SignPath Foundation eligibility and the actual publisher/certificate provider is unknown. Do not add fake `SIGNPATH_*` placeholders or an inactive production workflow. The next necessary product-owner input is the intended real publisher identity: individual legal identity or an existing registered legal entity that actually owns the software. After that, compare OV CA/remote HSM providers and implement the appropriate GitHub Actions signer integration.

No signing certificate, private key, HSM account or GitHub secret was created, exported or changed in this work.
