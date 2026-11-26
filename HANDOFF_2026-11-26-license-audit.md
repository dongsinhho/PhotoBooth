# PhotoBooth Handoff - 2026-11-26 - Windows third-party notice integration

## Current repository checkpoint

- Repository: `C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth`
- Branch: `main`
- Previous committed baseline: `867f9e8` (MIT license audit checkpoint); at baseline local main was ahead of `origin/main` by 8 commits.
- Current changes in this handoff are not yet committed at time of writing; check `git status`/HEAD before continuing.
- No push performed.
- User's selected direction remains MIT Open Source for PhotoBooth-owned code and free SignPath Foundation signing; no SignPath project accepted, no workflow/secrets, no signed production release.

## Completed this step

### Active runtime dependency audit (Windows Wails application)

The actual non-standard-library runtime closure was enumerated using `go list -deps` from `tools/ui-spike/wails-v2` on Windows. It contains 13 third-party modules; the pinned paths, versions and notice file references are recorded in `third_party/licenses/manifest.json`.

- 8 MIT modules: modified/vendored windows-camera-go v0.2.0, leaanthony/go-ansi-parser v1.6.1, leaanthony/slicer v1.6.0, leaanthony/u v1.1.1, rivo/uniseg v0.4.7, wailsapp/go-webview2 v1.0.22, wailsapp/wails/v2 v2.17.0, and yaml.v3 file-specific MIT scope.
- 1 BSD-2-Clause: github.com/pkg/errors v0.9.1.
- 4 BSD-3-Clause: golang.org/x/crypto v0.53.0, x/image v0.41.0, x/sys v0.46.0, x/text v0.39.0.
- yaml.v3 v3.0.1 has file-specific MIT and Apache-2.0 obligations; its complete LICENSE and NOTICE are retained.
- Google Go PATENTS files are retained for x/crypto, x/image, x/sys and x/text.
- The original windows-camera-go MIT LICENSE remains under third_party/windows-camera-go.

The npm lockfile contains 64 package entries: 62 MIT, 1 ISC and 1 BSD-3-Clause; no unknown license metadata was found. It is a Vite 7.3.7 build-tool dependency tree (devDependency), not an npm app runtime dependency.

The tracked appicon.png and icon.ico were byte-for-byte matched to the Wails v2.17.0 source tree assets (blob SHA-1s recorded in THIRD_PARTY_NOTICES.md). They are upstream Wails branding; replace with original PhotoBooth branding before production if desired.

### Notice collection and release packaging

Added:
- `third_party/licenses/manifest.json`
- Original dependency notice copies under `third_party/licenses/`
- `third_party/licenses/THIRD_PARTY_LICENSES.txt` (27,583 bytes; SHA-256 `f12b011502ce8aa5e487872bc492f436291028c99ddd7aced55ee298da8c8249`)
- `tools/release/windows/Collect-PhotoBoothThirdPartyNotices.ps1`
- `tools/release/windows/release-third-party-notices-smoke.ps1`

Updated:
- `THIRD_PARTY_NOTICES.md` now documents the 13 runtime modules, npm/asset provenance, upstream camera dependency condition and remaining legal gates.
- `New-PhotoBoothWindowsPackage.ps1` collects three notice files, records each file's size/SHA in the release manifest, and includes them in the ZIP.
- `Release.Common.ps1` verifies exact notice names, sizes and SHA-256 digests in the release directory; it still permits legacy manifest formats only when no unlisted notice files are present.
- `Install-PhotoBoothWindowsPackage.ps1` permits the three flat notice files, requires all-or-none, bounds their sizes, copies them to each immutable release folder, and rejects existing-version collisions when notice metadata differs.
- `tools/release/windows/README.md` explains notice packaging and smoke test.

### SignPath modified-upstream eligibility blocker

Official SignPath terms require an upstream project whose modified source is included to publish signed builds, require a visible GitHub fork, release branches based on upstream branches usually signed, and code review for upstream changes.

The upstream `Kirizu-Official/windows-camera-go` repo is MIT and has two releases (v0.1.0 and v0.2.0), but both have zero release binary assets. GitHub's commit verification API showed current `main` commit signed/verified, but `dev` tip unsigned. A signed commit is not evidence of a signed upstream build/release binary.

PhotoBooth statically includes a locally modified vendored copy, not a visible fork. This remains an unresolved and potentially decisive SignPath eligibility blocker. A cosmetic fork does not solve the missing signed-upstream-build condition. Before applying, obtain a written SignPath ruling for this dependency or replace/reimplement it without modified upstream source. Do not claim eligibility yet.

## Checks run

- Root Go tests: `go test -count=1 ./...` — PASS.
- Wails Go tests: `cd tools/ui-spike/wails-v2; go test -count=1 ./...` — PASS.
- Existing release lifecycle smoke — PASS (legacy unsigned denial, first install fail-closed, install/update/rollback, hash/sidecar rejection, operator-data path protection).
- Signing mechanics smoke — PASS for temporary test-only certificate, signed EXE/helper/manifest verification, tamper detection, empty production publisher allowlist rejection, unsigned default-deny. Its temp test certificate thumbprint `D8EAE8F1F4D73F84440C7294510F4563376A19DA` was verified absent from CurrentUser My/Root/TrustedPublisher afterward; its temp smoke directory was removed. This is not a production signature.
- New `release-third-party-notices-smoke.ps1` — PASS: builds temporary ZIP, verifies 7 entries, installs into isolated TEMP root with `-SkipMachineProvisioning -SkipAutoStart`, confirms all three notices installed, then tampers a notice and confirms integrity rejection; temp data cleaned.
- All five changed/new PowerShell scripts passed AST parse.
- `git diff --check` — PASS (only Git LF-to-CRLF warnings for files without normalized CRLF; no whitespace errors).
- No physical hardware or normal installed PhotoBooth profile was touched during notice package acceptance.

## Remaining work / next dependency

1. Verify copyright holder and authority for root MIT `PhotoBooth contributors` notice; establish a DCO/CLA or equivalent contributor process before accepting outside contributions.
2. Resolve the SignPath modified-upstream condition for windows-camera-go (written SignPath answer or replace/reimplement).
3. Replace Wails default icon assets with original PhotoBooth identity before production release if maintaining brand distinction.
4. Extend dependency/notice manifests for Linux runtime and any future targets before making those packages.
5. Continue release-chain refactor from local certificate-store thumbprint signing to SignPath Foundation's artifact/origin model; retain embedded payload and notice hash verification.
6. Prepare official release/download page, public code signing policy, author/reviewer/approver roles, MFA, branch protection, then apply to SignPath and wait for acceptance.

## Do not do yet

- Do not create SignPath GitHub Action workflow or secrets before application/acceptance and before the modified-upstream issue is resolved.
- Do not claim PhotoBooth is signed, SignPath-approved or release-ready.
- Do not push unless the product owner requests it.


## Latest execution details

- End-to-end Windows third-party notices smoke: `PHOTOBOOTH_THIRD_PARTY_NOTICES_SMOKE=PASS`. The generated temporary ZIP had exactly 7 flat entries, all three notice files were installed under `releases/<version>`, manifest size/SHA checks passed, and tampered `THIRD_PARTY_LICENSES.txt` was rejected. Its temp package/install tree was removed by the smoke script.
- Signing mechanics smoke: all cryptographic checks passed. Its ephemeral test-only code-signing certificate was removed from CurrentUser My, Root and TrustedPublisher after the Desktop Commander session paused in cleanup; the dedicated temp smoke folder was also manually removed. Do not treat this as production signing.
- Root `go test -count=1 ./...`, Wails `go test -count=1 ./...`, release lifecycle smoke and AST parse of all changed PowerShell scripts passed.
- Latest files awaiting commit include the collector, license manifest/original notices, package builder/installer verification changes, docs and notice smoke script.


## SignPath eligibility investigation continuation - 2026-11-26

- Official SignPath Foundation terms were re-opened. Modified upstream software may be signed only when the upstream publishes signed builds, a visible fork is used, signing release branches are based on upstream branches that are usually signed, and changes to upstream code receive code review. The official support page lists `support@signpath.io` for first interactions/single requests.
- Current upstream GitHub observations: repo `Kirizu-Official/windows-camera-go`; latest main commit `dc19bbfa3ff0353df8fd1c6372534cdc5e69b97e` and last push 2025-08-20; releases v0.1.0/v0.2.0 both have `assets: []`; tree has no `.github` workflow directory; only one closed PR (v0.2.0 release merge); main/dev branch protection is false. Main commit signature is GitHub verified. No upstream signed build was found.
- PhotoBooth's dependency is Go source statically compiled into the application; no separate upstream DLL/EXE is packaged. The dependency is used only by `internal/booth/camera/windowsmf/provider_windows.go`, and only a narrow API surface is referenced: Init/Shutdown, EnumDevice/OpenDevice, EnumerateCaptureFormats, StartCapture, GetFrame, ParseSampleToBuffer, CloseDeviceWithError, and associated data/HRESULT types.
- Prepared `docs/licensing/SIGNPATH_ELIGIBILITY_INQUIRY.md` with facts and copy-ready email addressed to support@signpath.io. **Not sent**; no external email action taken.
- Added `docs/adr/0038-resolve-signpath-camera-dependency-eligibility.md`. Strategy: obtain written policy interpretation before making an irreversible fork/rewrite. If SignPath requires upstream signed build artefacts unconditionally, implement a PhotoBooth-maintained Media Foundation backend behind the existing Provider boundary, or choose a compatible alternative signing provider. A cosmetic fork alone is not a solution.
- The native adapter replacement is bounded but not trivial: COM interface/vtable correctness, MF startup/shutdown, activation/device enumeration, capture-format selection, source-reader sample handling, COM ownership and locked-OS-thread lifecycle all need implementation and Windows physical/hotplug validation. Leave current backend untouched pending the SignPath answer.
- No changes to code, camera backend, Go dependencies, release signing config, workflow, secrets, or GitHub fork. No email sent.
