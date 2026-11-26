# PhotoBooth Handoff - 2026-11-26 - Open Source and SignPath Foundation transition

## Product-owner decision

The product owner explicitly reversed the prior controlled-core licensing model and requested PhotoBooth to become Open Source and use free SignPath Foundation GitHub signing.

## License decision and files changed

- MIT selected for PhotoBooth-owned code (permissive; allows commercial use, modification and redistribution with notice retention).
- Root `LICENSE` added with `Copyright (c) 2026 PhotoBooth contributors`. Confirm legal ownership/authority and adjust attribution if needed before release.
- `docs/knowledge-base/LICENSE_STRATEGY.md` rewritten to current MIT/Open Source model.
- ADR 0037 created/updated as the new authoritative transition decision.
- ADR 0004 marked superseded (historical controlled-core decision).
- README, PRD, Product Brief, Product Plan and Windows Quickstart updated in part to reflect MIT and open-source rights.
- `docs/CODE_SIGNING_POLICY.md` created as a draft. It contains the required SignPath attribution, repository URL, privacy statement, source/build requirements and honest role status.
- ADR 0036 and Roadmap updated to mark the old individual OV/IV provider research as superseded.

## SignPath Foundation current conditions checked against official terms (2026-11-26)

- OSI-approved license for all components, no commercial dual-licensing.
- No proprietary/non-open-source code (except permitted system libraries).
- Project must be maintained, already released in the form to be signed, and functionality described on download/release page.
- Project team must own/develop/maintain source and build scripts, and sign own source-built binaries.
- MFA on SignPath and GitHub/source-control access.
- Author, reviewer and approver roles.
- Public Code signing policy on homepage/download page with required attribution, roles and privacy statement.
- GitHub trusted build: workflow artifact uploaded before signing; GitHub provides origin metadata; all jobs leading to signing must run on GitHub-hosted runners for OSS.
- Manual approval required for every release signing request.
- SignPath Foundation is the certificate publisher; private key is in SignPath HSM.
- Current GitHub action: `signpath/github-action-submit-signing-request@v3`; use `actions/upload-artifact@v7` per current official GitHub integration docs, and pin actions to full reviewed SHAs before production.

Official references:
- https://signpath.org/terms.html
- https://docs.signpath.io/trusted-build-systems/github
- https://docs.signpath.io/projects
- https://opensource.org/osd

## Important unfinished gates

1. Full tracked code, dependency, asset, font, image, sample frame and release payload license audit. Root MIT does not relicense third-party components.
2. Confirm PhotoBooth contributors/maintainer has authority to license all project-owned code and contributions; establish contribution terms/CLA or DCO if needed.
3. Add third-party notices and remove/replace any non-compatible/proprietary components/assets.
4. Assign an independent reviewer and release approver; currently single-owner role assignments are not complete. Configure MFA and GitHub branch protection.
5. Publish initial source-built release and official download page; current public repo had no release/download page.
6. SignPath Foundation application and project acceptance. This cannot be submitted/approved by ChatGPT without the owner's account action.
7. Refactor release pipeline: existing package scripts sign from a local certificate store and enforce signer thumbprint; SignPath uses its own certificate subject. Do not simply append SignPath action to current installer builder. Need stage inner file signing, hash/manifest creation, NSIS assembly, final installer signing, and signature verification in correct order.
8. Implement the trusted GitHub workflow after the release chain is compatible and SignPath project IDs/policy are available. Require manual approval and origin verification; no PR/fork signing.
9. Test signed installer install/upgrade/rollback and verify Authenticode signature, timestamp, provenance and expected publisher.

## Current state

- Branch: `main`; open-source transition commit `82000fd` is committed locally; branch was ahead of `origin/main` by 7 commits at the last check.
- Current audit checkpoint: outdated controlled-core language was found in active `AGENTS.md` and README; both are being corrected. Wails and diagnostics nested Go modules are receiving explicit MIT LICENSE files because automated license tools otherwise treat those local modules as unlicensed.
- No SignPath project created or accepted; no secrets configured; no signing workflow active.
- MIT is selected for PhotoBooth-owned code. Copyright ownership/attribution and third-party source/assets remain release gates.
- Next immediate task: finish dependency/source/assets license inventory and notice preservation; then commit this audit checkpoint. SignPath application/release pipeline comes after the audit.
