# PhotoBooth Handoff - 2026-11-26 - MIT license audit

## Current checkpoint

- User requested the next step after choosing MIT and SignPath Foundation.
- Previous transition commit: 82000fd (license: transition PhotoBooth to MIT open source); branch was ahead of origin/main by 7 commits before this audit checkpoint.
- Active AGENTS.md and README controlled-core/licensing contradictions have been corrected.
- Added tools/ui-spike/wails-v2/LICENSE and tools/diagnostics/mf-enumeration-observer/LICENSE (MIT scope for these project-owned nested modules).
- Added THIRD_PARTY_NOTICES.md as an explicit audit draft; it is not release-ready.

## Verified preliminary dependency information

- Root go-licenses report ./... exited 0 and reported root Go dependencies: x/crypto, x/image, x/sys, x/term, x/text BSD-3-Clause; gopkg.in/yaml.v3 MIT; local windows-camera-go MIT (license verified in the vendored directory; scanner could not resolve the local replacement URL).
- Wails nested Go module has 112 module graph records. Preliminary license-file classification: 65 MIT, 11 Apache-2.0, 20 BSD-3-Clause, 8 BSD-2-Clause, 2 ISC, 1 MPL-2.0, 3 license texts still unclassified, and 2 local module records not resolved by the scanner. This is a module graph, not a finalized release dependency closure.
- go-licenses v1.6.0 report failed for Wails on Go 1.25.0 standard-library package metadata; do not claim a complete Wails license report.
- Frontend is Vite 7.3.7; npm license metadata preliminarily shows MIT/BSD-3-Clause/ISC. Lockfile-level notice verification remains.
- Tracked images are tools/ui-spike/wails-v2/build/appicon.png and tools/ui-spike/wails-v2/build/windows/icon.ico; creator/source/license is not documented.
- Upstream windows-camera-go v0.2.0 is MIT and its original license/copyright is preserved. PhotoBooth's local patch is documented in third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md.

## Potential SignPath eligibility blocker

Official SignPath Foundation terms say modified upstream software may be signed only under additional conditions, including upstream signed builds, a visible fork, release branches based on upstream signed branches, and code review of upstream changes. PhotoBooth currently vendors a modified copy locally, not a visible GitHub fork, and upstream signed-build status has not been verified. Resolve this before application (visible fork, replace dependency, or ask SignPath).

## Next work

1. Confirm copyright holder/authority and contribution terms before release.
2. Classify the three Wails license texts and determine exact release dependency closure.
3. Verify npm licenses and notices from the lockfile.
4. Confirm/replace icons and audit generated/package assets.
5. Implement deterministic third-party license collection in the release ZIP and installed package.
6. Resolve the modified-upstream SignPath condition.
7. Only then continue to release/download page, roles/MFA, SignPath application and trusted GitHub signing workflow.

## Signing status

No SignPath project accepted/created, no secrets/workflow, no signed release. Do not claim signing is active.
