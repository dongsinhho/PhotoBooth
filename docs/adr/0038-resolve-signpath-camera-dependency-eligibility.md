# ADR 0038: Resolve SignPath eligibility for the Windows camera source dependency

**Status:** Proposed execution path - written SignPath clarification pending
**Date:** 2026-11-26
**Decision owner:** Product owner

## Context

PhotoBooth is transitioning to MIT open source and evaluating the SignPath Foundation free code-signing certificate. The Windows Wails application imports `github.com/Kirizu-Official/windows-camera-go v0.2.0` as Go source through a local `replace` directive:

```go
replace github.com/Kirizu-Official/windows-camera-go => ./third_party/windows-camera-go
```

The vendored source contains PhotoBooth compatibility and resource-lifecycle patches. The module is statically compiled into the PhotoBooth Windows executable; no separate upstream DLL/EXE is packaged.

SignPath's current Foundation terms say a project may sign its own modified version of upstream software if the upstream publishes signed builds, the project visibly uses a fork, its signing release branches are based on upstream branches that are usually signed, and the project's changes to upstream code receive code review. The policy also says unsigned upstream OSS binaries may be included in signed packages; that allowance is not obviously applicable to Go source compiled into the project's own executable.

## Verified upstream facts as at 2026-11-26

- Repository: https://github.com/Kirizu-Official/windows-camera-go
- Latest `main` commit: `dc19bbfa3ff0353df8fd1c6372534cdc5e69b97e` (2025-08-20), GitHub commit verification reports signed/verified.
- GitHub reports last push: 2025-08-20.
- Releases `v0.1.0` and `v0.2.0` exist, but both have zero release assets.
- Public repository tree has no `.github` directory/workflow.
- Only one public PR was found: the closed merge for `v0.2.0`.
- `main` and `dev` are unprotected. Inspected `dev` tip `f16ccdb718342601f5899bfa35364395c59b730e` was unsigned; `main` tip was signed/verified.

A signed source commit does not establish that the upstream project publishes signed build artifacts. There are no upstream binary artifacts to validate from the inspected releases.

## Technical scope of PhotoBooth's use

The upstream dependency is isolated behind the PhotoBooth-owned `camera.Provider` interface and is imported only from `internal/booth/camera/windowsmf/provider_windows.go`.

PhotoBooth calls only a narrow part of the upstream API:
- Media Foundation initialization/shutdown and device enumeration/open.
- Device capture-format enumeration and stream start.
- Synchronous frame read and sample-to-buffer conversion.
- Device close with cleanup errors.
- Basic device/format/HRESULT types.

The application owns camera worker-thread scheduling, serialization, retry/recovery policy, warm-up, JPEG validation, output writing, diagnostics and the public provider abstraction. Replacing the upstream module therefore does not require replacing the rest of the capture workflow, but a new provider implementation still must correctly implement the COM/Media Foundation native lifecycle on Windows.

The existing patch is not purely a compile fix. It contains changes to COM reference ownership, Media Foundation shutdown order, task-memory cleanup, camera-control HRESULT handling and device metadata error reporting. Replacing it must be treated as a native camera backend change with hardware regression tests, not as a dependency-name substitution.

## Decision / proposed execution

1. **Ask SignPath for a written interpretation before creating a public fork or rewriting native camera code.** Use `docs/licensing/SIGNPATH_ELIGIBILITY_INQUIRY.md`. The question is specifically whether a Go source-only dependency statically compiled into PhotoBooth's own PE falls under the modified-upstream signed-build condition.
2. Keep SignPath application, GitHub Actions signing workflow and signing secrets blocked while this is unresolved.
3. If SignPath confirms this source-only/static-link case is acceptable, record the written response and then establish the visible fork, branch ancestry, upstream code-review process and required project roles before applying. Formal SignPath acceptance remains a separate gate.
4. If SignPath confirms the missing upstream signed builds are a hard blocker, do not create a cosmetic fork. Implement a PhotoBooth-maintained Media Foundation backend inside the existing `internal/booth/camera/windowsmf` boundary or select a different signing provider with compatible terms.
5. If SignPath's response is ambiguous, ask one focused follow-up before changing implementation.

## Why not rewrite now

The current adapter has a narrow upstream API surface, making replacement technically bounded. However, the Media Foundation layer requires correct COM ABI/vtable calls, activation and source-reader setup, stream format selection, sample and buffer ownership, and thread-affine initialization/shutdown. The existing implementation has already required real USB hot-plug and repeated-session recovery work. Replacing the dependency before receiving SignPath's interpretation would introduce avoidable hardware risk and could still leave other eligibility gates unresolved.

## Alternatives considered

| Alternative | Assessment |
|---|---|
| Keep local vendored patches without clarification | Not suitable for SignPath application while terms are ambiguous; valid only for current engineering preview pending eligibility decision. |
| Create a public fork immediately | Not enough by itself: no upstream signed build artifacts were found, so the stated condition remains unresolved. |
| Upstream the patches to the original project | Desirable for maintenance if the upstream maintainer accepts them, but does not itself solve the no-signed-build condition. |
| Replace with PhotoBooth-maintained Media Foundation code | Strong fallback for removing the modified-upstream condition; requires dedicated Windows native API implementation and hardware regression. |
| Choose a different code-signing provider | Commercial/other provider evaluation if the project cannot reasonably meet SignPath Foundation rules. |

## Acceptance / exit criteria

- Written response from SignPath clarifies treatment of a statically linked Go source dependency and the missing upstream signed build requirement.
- The selected path is recorded in an ADR and handoff before any fork/reimplementation.
- If a backend replacement is selected: existing camera provider contract tests pass; Windows physical camera open/enumerate/capture/preview/warm-up, repeated same-process sessions and USB unplug/replug recovery pass before removing the old dependency.
- SignPath project eligibility and formal acceptance are still required before turning on a production signing workflow.

## References

- SignPath Foundation terms: https://signpath.org/terms.html
- SignPath support: https://signpath.io/support
- Upstream repository: https://github.com/Kirizu-Official/windows-camera-go
- PhotoBooth's local upstream patch: `third_party/windows-camera-go/README.PHOTOBOOTH-PATCH.md`
- Copy-ready inquiry: `docs/licensing/SIGNPATH_ELIGIBILITY_INQUIRY.md`
