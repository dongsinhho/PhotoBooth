# Rendering Performance and Lifecycle Validation

**Status:** Renderer optimization and synthetic lifecycle checks implemented; peak process RAM, real-printer performance, long-run camera recovery and UI resources are still unmeasured.
**Last measured:** 2026-10-25

## Benchmark environment

- OS: Windows 11 Pro 64-bit, build 10.0.26200
- CPU: AMD Ryzen 5 8600G with Radeon 760M Graphics
- Go: 1.24.8, windows/amd64
- Frame: schema-v1 1200x1800 three-photo strip
- Benchmark inputs: deterministic textured synthetic 1280x720 JPEGs; no guest/user photos
- Iterations: ten per benchmark unless noted
- Commands:

```powershell
go test ./internal/booth/render -run '^$' -bench 'BenchmarkRenderPhotoStrip1200x1800' -benchtime=10x -benchmem
go test ./internal/booth/render -run '^$' -bench 'BenchmarkFitImageScalers' -benchtime=10x -benchmem
go test ./internal/booth/output -run '^$' -bench 'BenchmarkCoordinatorLocalSave' -benchtime=10x -benchmem
```

These are local smoke benchmarks, not a statistical performance study. Run multiple fresh processes on representative booth hardware before making an SLA claim.

## Renderer optimization

The original path used Catmull-Rom scaling for all source-to-slot transforms. For the common case where a camera image is being reduced to a smaller frame slot, x/image's Catmull-Rom scaler allocates a sizable intermediate scratch buffer. The updated implementation chooses ApproxBiLinear for downscaling and retains Catmull-Rom for upscaling. The renderer also crops a cover-fit source rectangle before scaling directly into the slot, rather than scaling a larger intermediate image and cropping/copying it afterward.

| 10-iteration render benchmark | Catmull-Rom for downscale | Adaptive downscale/upscale |
|---|---:|---:|
| Render time | 171.75 ms/op | 102.26 ms/op |
| Bytes allocated per op | 73,139,144 B | 17,279,337 B |
| Allocations per op | 265 | 245 |

On this fixture, the adaptive path reduced render time by about 40% and allocated bytes by about 76%. The results describe Go allocations during one render, not peak live heap or process working-set/RSS.

The adaptive scaler is guarded by a pixel regression test comparing its downscaled output with Catmull-Rom on a synthetic gradient/grid fixture (mean absolute per-channel difference must stay at or below 8 on an 8-bit scale). Cover-mode centering and extreme-aspect behavior also have tests. The existing frame JSON schema is unchanged.

## Scaling algorithm microbenchmark

At 1280x720 source to 800x450 destination, ten iterations produced:

| Algorithm | Time | Allocated bytes/op | Allocations/op |
|---|---:|---:|---:|
| Catmull-Rom | 21.90 ms | 18,620,192 B | 7 |
| BiLinear | 12.31 ms | 18,553,635 B | 6 |
| ApproxBiLinear | 6.07 ms | 0 B | 0 |

Treat this as a comparative measurement for one scaling ratio and source fixture, not a universal claim for arbitrary photo sizes or transformations.

## Local output benchmark

The coordinator/local-storage benchmark copies a deterministic 720,000-byte fixture into a session folder, fsyncs, then atomically renames the temp file to the final path.

| Metric | Result |
|---|---:|
| Time per local target | 4.70 ms |
| Throughput reported by Go benchmark | 153.10 MB/s |
| Bytes allocated per op | 37,950 B |
| Allocations per op | 34 |

This excludes real printer spool time, OS queue behavior, and slow/network storage.

## Lifecycle and cancellation checks

- A 25-session regression uses fresh fake-camera providers and unique session IDs, verifies three captures and one output per session, and checks that each camera is closed after completion.
- A context-deadline test interrupts a blocking fake camera, expects the session to become `timed_out`, and verifies the camera is closed.
- Renderer tests cover output dimensions, unknown JSON properties, frame slot mismatch, centered cover cropping, extreme aspect ratios without huge intermediates, and scaler pixel-difference bounds.
- Three consecutive real Logitech B525 sessions previously passed with the Windows Media Foundation adapter. After this renderer change, another B525 3-shot run passed; each capture decoded at 1280x720, and both rendered/local output decoded at 1200x1800. The output remained local and test photos were deleted.

## Important gaps

- `B/op` is cumulative allocation volume, not peak memory. Peak heap, garbage-collection behavior, working set/RSS and sustained-session memory growth have not yet been measured.
- The race detector was not run on this host: the available toolchain has CGO disabled and no C/C++ compiler in PATH. Re-run on a configured CI/Linux environment.
- Cancellation against a blocking fake provider does not prove cancellation can interrupt a synchronous Windows Media Foundation driver call. Physical unplug/reconnect and blocked-driver behavior remain untested.
- UI memory, UI-to-Go binding latency, preview frame rate, thermal behavior and input latency have not been measured.
- Camera quality under controlled booth lighting, printer latency/queue behavior and long-run production stability remain unvalidated.
