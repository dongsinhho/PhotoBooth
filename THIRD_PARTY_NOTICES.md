# Third-party notices and license inventory

**Status: runtime dependency notices collected for the current Windows Wails build.** This inventory is generated from the non-standard-library module closure returned by `go list -deps .` inside `tools/ui-spike/wails-v2` on Windows amd64. The collector fails the release build if a runtime module or version is missing from `third_party/licenses/manifest.json`, or if a referenced notice file is missing.

This is a technical redistribution inventory, not legal advice or a representation that SignPath Foundation has accepted the project.

## Project license

PhotoBooth-owned source code and build scripts are licensed under the MIT License in the repository root. The nested Wails app and diagnostics modules carry matching MIT LICENSE files for module-level tooling. The root copyright line currently says `PhotoBooth contributors`; the maintainer still needs to confirm the legally correct rights-holder/attribution and authority for all historical contributions before public release.

Third-party code is not relicensed by PhotoBooth's root MIT file. Preserve the upstream license and notices below.

## Windows application runtime dependency closure

**Target:** Windows amd64 Wails application built from `tools/ui-spike/wails-v2` using Go 1.25.0 and Wails v2.17.0.

The collector observed 13 third-party Go modules (excluding the two local PhotoBooth modules and the Go standard library):

| Module | Version | License |
|---|---:|---|
| `github.com/Kirizu-Official/windows-camera-go` | v0.2.0 | MIT |
| `github.com/leaanthony/go-ansi-parser` | v1.6.1 | MIT |
| `github.com/leaanthony/slicer` | v1.6.0 | MIT |
| `github.com/leaanthony/u` | v1.1.1 | MIT |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause |
| `github.com/rivo/uniseg` | v0.4.7 | MIT |
| `github.com/wailsapp/go-webview2` | v1.0.22 | MIT |
| `github.com/wailsapp/wails/v2` | v2.17.0 | MIT |
| `golang.org/x/crypto` | v0.53.0 | BSD-3-Clause |
| `golang.org/x/image` | v0.41.0 | BSD-3-Clause |
| `golang.org/x/sys` | v0.46.0 | BSD-3-Clause |
| `golang.org/x/text` | v0.39.0 | BSD-3-Clause |
| `gopkg.in/yaml.v3` | v3.0.1 | MIT and Apache-2.0, file-specific scope |

The corresponding original license/notice files are preserved in `third_party/licenses/`; the original `windows-camera-go` MIT file remains at `third_party/windows-camera-go/LICENSE`. `golang.org/x/*` PATENTS files are also preserved. The complete concatenated notices are in `third_party/licenses/THIRD_PARTY_LICENSES.txt`.

### YAML license scope

The upstream `gopkg.in/yaml.v3` LICENSE says the files ported from libyaml retain the original MIT license, while the remaining project files are covered by Apache-2.0. Keep the full LICENSE and NOTICE together; do not collapse this dependency into an inaccurate single-license statement.

## Windows camera library and SignPath eligibility

`windows-camera-go v0.2.0` is MIT-licensed and its original copyright/license notice is preserved. PhotoBooth keeps a locally patched vendored copy under `third_party/windows-camera-go`; the patch rationale is recorded in `README.PHOTOBOOTH-PATCH.md`.

Official upstream repository: https://github.com/Kirizu-Official/windows-camera-go

The upstream GitHub releases `v0.1.0` and `v0.2.0` contain no release binary assets. The current upstream `main` commit is GitHub-verified as signed, while the `dev` branch tip inspected during this audit is unsigned. A signed source commit is not evidence of a published signed build artifact.

SignPath Foundation's published policy allows a project to sign its own modified version of upstream only if the upstream project publishes signed builds, the project visibly uses a fork, release branches are based on upstream branches that are usually signed, and upstream changes receive code review. PhotoBooth currently does not satisfy the visible-fork condition, and no upstream signed build artifact was found in the inspected releases. **Treat this as an unresolved SignPath eligibility blocker.** Before applying, either ask SignPath for a written determination for this statically included library, replace/reimplement the dependency without modified upstream source, or establish a valid upstream/fork workflow if the signed-build condition can also be satisfied. Do not create a cosmetic fork and claim eligibility.

## Frontend build tooling

The Wails frontend's `package-lock.json` contains 64 package entries at this checkpoint. The package metadata reports:

- 62 MIT
- 1 ISC
- 1 BSD-3-Clause
- 0 packages with unknown license metadata

These are Vite 7.3.7 and its frontend build-tool dependency tree. `package.json` lists Vite only under `devDependencies`; no third-party npm package is declared as an application runtime dependency. The current build output is the project's own frontend code bundled by Vite/Rollup/esbuild. The lockfile remains the source of truth for build-tool versions; re-run the license metadata audit whenever it changes.

## Bundled build assets

The only tracked image assets in the Windows Wails build folder are Wails' default application icon pair:

- `tools/ui-spike/wails-v2/build/appicon.png`
- `tools/ui-spike/wails-v2/build/windows/icon.ico`

Both files were byte-for-byte matched with the Wails v2.17.0 source tree:
- `v2/pkg/buildassets/build/appicon.png`, Git blob SHA-1 `63617fe4f746b8a878bd5f44725f4f317b9d9850`
- `v2/pkg/buildassets/build/windows/icon.ico`, Git blob SHA-1 `f33479841c61728c8b5ee076ed13771c68e48b40`

The upstream Wails repository is MIT-licensed; its LICENSE is included in the runtime notice bundle. These are Wails branding assets, not a PhotoBooth identity. Replace them with original PhotoBooth branding before production distribution if the maintainer does not intend to ship the Wails default icon.

## Package/release behavior

- `third_party/licenses/manifest.json` pins the runtime dependency paths, versions, SPDX/license descriptions and notice file paths.
- `tools/release/windows/Collect-PhotoBoothThirdPartyNotices.ps1` enumerates the Windows application dependency closure, requires an exact manifest match, then creates a deterministic consolidated notice file.
- The Windows release ZIP must include the root MIT LICENSE, this inventory, and the consolidated third-party license notices.
- The installer must preserve these files in the immutable version-specific release directory.
- On each release build, the collector must pass before an artifact can be packaged. New/updated runtime dependencies require the manifest and notice bundle to be updated first.

## Remaining legal/SignPath gates

1. Confirm the correct copyright holder for PhotoBooth-owned code and authority to license all historical contributions; establish DCO/CLA or an equivalent contribution-rights process before accepting outside code.
2. Obtain a written SignPath determination for the modified `windows-camera-go` library or remove that modified-upstream blocker.
3. Confirm whether the maintainer wants to replace the default Wails icon with original PhotoBooth branding before production.
4. Publish the code-signing policy and product download/release page, assign the author/reviewer/approver roles, enable MFA and apply for SignPath only after the above prerequisites are resolved.
5. Re-run the dependency collector on every release target/platform and ensure licenses for any platform-specific runtime modules are added before making cross-platform support claims.

No SignPath project has been accepted; no GitHub signing workflow or secrets are active; no production release has been signed.
