# SignPath Foundation eligibility inquiry - windows-camera-go

**Status:** Draft only. Not sent to SignPath.
**Prepared:** 2026-11-26
**Project:** PhotoBooth - https://github.com/dongsinhho/PhotoBooth
**Contact:** support@signpath.io (SignPath says first interactions and single requests can be sent by email)

## Why we are asking

PhotoBooth is transitioning its own source code to the MIT License and is evaluating the SignPath Foundation free code-signing certificate. The Windows application is a Go/Wails desktop application. It uses the Go source library `github.com/Kirizu-Official/windows-camera-go` for Windows Media Foundation camera access.

PhotoBooth pins upstream v0.2.0 but currently uses a local replacement:

```go
require github.com/Kirizu-Official/windows-camera-go v0.2.0
replace github.com/Kirizu-Official/windows-camera-go => ./third_party/windows-camera-go
```

This is not a separately distributed DLL or executable. The Go package is imported by PhotoBooth's Windows Media Foundation adapter and compiled into the PhotoBooth Windows executable. The package ZIP contains the PhotoBooth executable and project-owned helper, not a separately built windows-camera-go binary.

## Exact changes in the vendored copy

The local compatibility patch is documented in `third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md`. It preserves upstream MIT attribution and includes these changes:

- Removed a CGo dependency so the Windows code can build with `CGO_ENABLED=0`; replaced `C.GoBytes` with a bounded-lifetime copy from an `unsafe.Slice` while the Media Foundation buffer is locked.
- Corrected COM/native resource ownership: releases activation objects, temporary attributes and media type handlers; frees the `MFEnumDeviceSources` pointer array with `CoTaskMemFree`; shuts down the Media Foundation source before releasing COM references; releases DLL handles.
- Corrected handling of non-`S_OK` camera-control results so a rejected hardware control write cannot look successful.
- Returns contextual errors when Media Foundation enumerates sources but all metadata reads fail, instead of presenting that as zero cameras.
- Removed one unconditional diagnostic print.

These are not only build-system edits: several changes alter native resource-lifetime and error-handling behavior. Replacing the library is therefore a camera-backend engineering change that needs Windows regression coverage.

## How much of the upstream API PhotoBooth uses

The application references the dependency only from `internal/booth/camera/windowsmf/provider_windows.go`. The adapter calls the following dependency API surface:

- `Init` / `Shutdown`
- `EnumDevice` / `OpenDevice`
- `Device.EnumerateCaptureFormats`
- `Device.StartCapture`
- `CaptureSync.GetFrame`
- `Device.ParseSampleToBuffer`
- `Device.CloseDeviceWithError`
- The `DeviceInfo`, `CaptureFormats`, `CaptureSync` and HRESULT error types used by those calls.

The PhotoBooth-owned `camera.Provider` abstraction, capture workflow, retry/recovery policy, warm-up, JPEG validation and atomic file output are outside the upstream package and remain in PhotoBooth.

## Public upstream repository observations (checked 2026-11-26)

Repository: https://github.com/Kirizu-Official/windows-camera-go

- Default branch: `main`.
- The current `main` tip is `dc19bbfa3ff0353df8fd1c6372534cdc5e69b97e`, dated 2025-08-20. GitHub reports its commit signature as verified.
- Last push reported by GitHub: 2025-08-20.
- Releases: `v0.1.0` and `v0.2.0`; each release has an empty `assets` array.
- The repository tree has no `.github` directory or public workflow files.
- The only public pull request found is the closed `release: v0.2.0 code merge` PR.
- Branches `main` and `dev` are reported as unprotected. The inspected `dev` tip is `f16ccdb718342601f5899bfa35364395c59b730e`; the inspected `main` tip is signed/verified.
- Upstream describes itself as still under development and potentially unstable.

A verified Git commit signature is not the same thing as an upstream-signed PE/DLL build. No signed upstream build artifact was found in the inspected releases. The upstream library is a source-only Go dependency in the PhotoBooth build, rather than an upstream binary that PhotoBooth packages separately.

## Questions for SignPath

1. Does the policy condition "the upstream project publishes signed builds" apply to a Go source-only library that has no upstream binary release and is statically compiled into the PhotoBooth executable, with no upstream DLL/EXE shipped separately?
2. Does PhotoBooth's locally patched vendored source count as "building our own modified version of upstream software" under that condition, even though the product distributes only the PhotoBooth executable?
3. If the answer is yes, is a visible GitHub fork, branches based on upstream branches, and code review of every local change sufficient when the upstream project itself has no signed build artifacts? Or is the signed-upstream-build condition mandatory with no exception?
4. Would the condition be avoided only by replacing the dependency with a PhotoBooth-maintained Media Foundation implementation written in the PhotoBooth repository (while retaining third-party OSI-licensed APIs/system libraries only)?
5. Is it acceptable to sign only the PhotoBooth executable when all Go dependency source is statically linked into that executable and no separately built upstream binaries are included in the package?

We are not asking for an exemption from the policy. We want to choose an implementation/release path that complies before creating a fork or rewriting the camera backend.

## Copy-ready email draft

**To:** support@signpath.io
**Subject:** Eligibility question: statically linked, patched Go source dependency in PhotoBooth

Hello SignPath Foundation team,

I am preparing PhotoBooth, an MIT-licensed open-source Go/Wails desktop application, for a possible application to the SignPath Foundation free code-signing certificate.

The Windows application imports `github.com/Kirizu-Official/windows-camera-go` (MIT, v0.2.0) as source and compiles it into the PhotoBooth executable. We do not package a separate DLL or EXE from this dependency. We currently use a local vendored replacement with a small set of Windows Media Foundation compatibility and native resource-lifecycle fixes. The original MIT license and copyright notice are preserved.

The upstream repository is https://github.com/Kirizu-Official/windows-camera-go. Its two GitHub releases (`v0.1.0` and `v0.2.0`) have no release assets, and we found no public CI workflow or signed binary build. The latest `main` commit is GitHub-verified as signed, but we understand that a signed commit is not the same as a signed upstream build.

Your modified-upstream condition says that an upstream project must publish signed builds, that the project must visibly use a fork, that signing release branches must be based on upstream branches that are usually signed, and that changes to upstream code must receive code review.

Could you please clarify how this condition applies to our case:

1. Does a Go source-only dependency statically linked into our own executable count as a modified upstream project for this rule, even though no separate upstream DLL/EXE is distributed?
2. Since the upstream project does not publish build artifacts, is this an absolute blocker even if we create a public fork, base our release branch on upstream, and have all changes to the upstream code reviewed?
3. Would replacing the dependency with a PhotoBooth-maintained Windows Media Foundation implementation remove this modified-upstream condition, provided all code in that implementation is maintained in our repository and third-party/system-library notices are preserved?

We are not asking to bypass the policy. We would like to confirm the compliant path before either creating a fork or undertaking a camera-backend rewrite.

Project repository: https://github.com/dongsinhho/PhotoBooth

Thank you for your guidance.

Regards,
PhotoBooth maintainer

## Decision rule

- **If SignPath confirms the source-only/static-link case is acceptable:** document the response, then build a fork/review process exactly as SignPath requires and submit the project for formal acceptance. Do not infer acceptance from an informal answer.
- **If SignPath says signed upstream builds are mandatory:** do not create a cosmetic fork. Replace the dependency with a PhotoBooth-maintained Media Foundation implementation, or use a different signing provider with compatible requirements.
- **If the answer is ambiguous:** request a short written clarification against the exact source-only/static-link facts above. Keep the SignPath application and signing workflow blocked.

## References

- SignPath Foundation eligibility terms: https://signpath.org/terms.html
- SignPath first-contact support: https://signpath.io/support
- Upstream repository: https://github.com/Kirizu-Official/windows-camera-go
- PhotoBooth camera adapter: `internal/booth/camera/windowsmf/provider_windows.go`
- Local upstream patch description: `third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md`
