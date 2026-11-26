# Third-party notices and license audit

**Status: AUDIT DRAFT — NOT RELEASE-READY.** This file is a working inventory. Do not package or publish it as a completed third-party notice until every production dependency, bundled asset and notice has been verified.

## Project-owned code

PhotoBooth-owned code is licensed under the MIT License; see the repository root `LICENSE`. The nested Wails player and camera diagnostics modules also carry a copy of the MIT license to make their module license scope explicit.

The root copyright line currently says `PhotoBooth contributors`. Before public release, the maintainer must confirm the correct copyright holder(s) and authority to license all project-owned contributions. No CLA/DCO or contributor-rights process has yet been confirmed.

## Vendored upstream code

### github.com/Kirizu-Official/windows-camera-go v0.2.0

- License: MIT
- Upstream: https://github.com/Kirizu-Official/windows-camera-go
- Original license file: `third_party/windows-camera-go/LICENSE` (preserved)
- Original copyright line: Copyright (c) 2025 桐泉和谷
- PhotoBooth changes: see `third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md`; this is a locally vendored, modified copy and is not currently maintained as a visible GitHub fork.

The upstream MIT notice and full license must be retained. SignPath Foundation's terms specifically allow signing a modified upstream version only when the upstream publishes signed builds, the project visibly uses a fork (e.g. GitHub fork), release branches are based on upstream branches that are usually signed, and upstream changes receive code review. PhotoBooth currently has a locally vendored modified copy rather than a visible GitHub fork; upstream signed-release status has not been verified. Treat this as a potential SignPath eligibility blocker and resolve it before applying. See https://signpath.org/terms.html.

## Root Go module — preliminary production dependency report

A `go-licenses report ./...` run completed successfully for the root module and reported:

| Module | License |
|---|---|
| `golang.org/x/crypto v0.41.0` | BSD-3-Clause |
| `golang.org/x/image v0.32.0` | BSD-3-Clause |
| `golang.org/x/sys v0.36.0` | BSD-3-Clause |
| `golang.org/x/term v0.35.0` | BSD-3-Clause |
| `golang.org/x/text v0.30.0` | BSD-3-Clause |
| `gopkg.in/yaml.v3 v3.0.1` | MIT |
| `windows-camera-go v0.2.0` | MIT (local replacement; scanner could not resolve a remote license URL, verified against the preserved local LICENSE) |

The report identifies dependencies used by the root Go package graph. The Wails UI is a separate nested Go module and must be audited separately.

## Wails Windows player — preliminary module graph scan

The nested module's `go list -m all` graph contained 112 module records at the time of this scan. A local license-file classifier found these preliminary categories:

- 65 MIT
- 11 Apache-2.0
- 20 BSD-3-Clause
- 8 BSD-2-Clause
- 2 ISC
- 1 MPL-2.0
- 3 license files needing exact classification
- 2 local modules not resolved by the scanner (PhotoBooth main module and the local vendored camera module)

This is a **module graph inventory, not a finalized compiled/bundled dependency closure**. Some modules are build/test-only. The `go-licenses v1.6.0` report failed on the nested Go 1.25.0 module because it could not associate standard-library packages with module metadata. The three unclassified license texts and exact runtime/release dependency set remain open.

## Frontend build dependencies

The Wails frontend uses Vite 7 and its npm dependency tree (including esbuild, Rollup, PostCSS, nanoid, picomatch and related packages). A preliminary package metadata scan found MIT, BSD-3-Clause and ISC license fields; all package notices and transitive production/build-time obligations still need to be verified against the lockfile and actual release build.

## Tracked binary assets

The only tracked image assets found in the repository are:

- `tools/ui-spike/wails-v2/build/appicon.png`
- `tools/ui-spike/wails-v2/build/windows/icon.ico`

The Windows installer references `icon.ico`. The icon files' original creator/source and license are not currently documented. Confirm that the project owns them or replace them with an explicitly licensed/original icon before publishing a release.

No tracked font, photo, video or other image asset was found by the extension inventory. This does not cover files generated during packaging or assets embedded by third-party build tools.

## Release packaging gap

The current Windows release package/installer scripts do not yet generate and include a complete third-party license/notice bundle. Before the first public release:

1. Complete the license classification for every module used by the root and Wails release builds.
2. Resolve the Wails Go 1.25 scanner limitation or use an independently verified method.
3. Verify npm dependency licenses from the lockfile and package license files.
4. Confirm/replace the app icons and verify every asset included in the release ZIP/installer.
5. Preserve all required BSD, Apache-2.0, ISC, MIT and MPL-2.0 notices; include Apache NOTICE files where present and satisfy MPL file-level obligations if applicable.
6. Produce a deterministic notices bundle and verify it is present in both the release ZIP and installed package.
7. Obtain maintainer confirmation of copyright ownership and contributor licensing authority.

**Release gate:** this audit is not complete, and no SignPath application or signed release should be represented as ready until the remaining items are resolved.
