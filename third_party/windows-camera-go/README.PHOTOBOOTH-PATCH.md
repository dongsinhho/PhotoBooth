# PhotoBooth compatibility patch

Upstream: https://github.com/Kirizu-Official/windows-camera-go
Pinned upstream module: v0.2.0
Upstream license: MIT; see LICENSE in this directory.

## Why this copy exists

The upstream v0.2.0 source contains `import "C"` in `windows/mf/IMFMediaBuffer.go` and `windows/mf/mf.go`. The first uses `C.GoBytes`; as a result, that source does not build with `CGO_ENABLED=0` and fails when its CGo-only file is excluded, despite the upstream README stating that CGO is not required.

## Local compatibility change

- Removed the unused `import "C"` in `windows/mf/mf.go`.
- Replaced `C.GoBytes` in `windows/mf/IMFMediaBuffer.go` with an immediate Go-owned copy from an `unsafe.Slice` while the native Media Foundation buffer remains locked.
- No other upstream behavior is intentionally changed.

This is a narrow compatibility patch, not a claim that the upstream library is production-grade. Upstream itself warns that it is under development and may be unstable. Keep the adapter isolated behind `camera.Provider`, preserve this license/notice, run the Windows hardware tests before updating the pinned copy, and do not rely on undocumented behavior without regression coverage.
