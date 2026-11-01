# ADR 0024 — WebView2 preview memory profiling and diagnostic GPU toggle

**Status:** Accepted for diagnostics only; hardware acceleration remains enabled by default.
**Date:** 2026-11-01

## Context

The 10.24-minute B525/Windows 11 preview run recorded a WebView2 private-byte peak of 1,254.63 MB while the PhotoBooth process itself peaked at 31.86 MB. The existing report aggregated all WebView2 subprocesses, so it did not show whether memory was concentrated in the browser, renderer, GPU or utility processes.

The current preview renderer already uses one persistent canvas, one decoded `ImageBitmap` at a time, explicit bitmap close before ACK, one in-flight frame and a 5 FPS cap. The observed process-tree footprint therefore should not be addressed by assuming an unbounded JavaScript frame queue.

## Decision

1. Extend the Windows preview profiler to record each WebView2 subprocess role, PID and parent PID, and report per-role private bytes separately from the whole process tree.
2. Add an opt-in diagnostic environment flag, `PHOTOBOOTH_DISABLE_WEBVIEW_GPU=1`, mapped to Wails v2.17.0's native `windows.Options.WebviewGpuIsDisabled` setting. The soak harness exposes it as `-DisableWebViewGPU`.
3. Keep GPU acceleration enabled for normal launches. The diagnostic flag is not a user-facing setting or a production default.
4. Store role first/last/peak values in CSV, JSON and Markdown reports. Identify these as process private bytes, not dedicated GPU memory or VRAM.
5. Do not label the renderer growth as a memory leak based on short samples. Do not claim low-RAM suitability from this single host.

## Validation

ShinPc / Windows 11 / Logitech B525, 1280x720, fullscreen kiosk. Both runs used the same 3-minute harness configuration and passed the exact post-shutdown JSONL event audit. Disposable profiles were removed and no photo session was started.

| Metric | GPU acceleration disabled | Default GPU acceleration |
|---|---:|---:|
| Actual duration | 3.15 min | 3.06 min |
| Resource samples | 70 | 69 |
| Settings / display cycles | 3 / 3 | 2 / 2 |
| Preview FPS mean / min | 4.75 / 4.71 | 4.75 / 4.71 |
| CPU mean / peak (% of one logical core) | 13.96 / 27 | 13.78 / 22 |
| Process-tree private bytes peak | 612.53 MB | 1,003.94 MB |
| WebView2 private bytes peak | 582.54 MB | 977.52 MB |
| WebView2 GPU-process private bytes peak | 33.20 MB | 576.38 MB |
| WebView2 renderer private bytes peak | 493.21 MB | 479.01 MB |
| Published / ACK / cancelled | 865 / 863 / 2 | 848 / 848 / 0 |
| Unmatched publishes / audit gaps | 0 | 0 |

The two runs are a useful same-host diagnostic comparison, not a controlled benchmark: process startup baselines differed, the durations and lifecycle counts were not identical, and only one camera/host was tested. The disabled-GPU run reduced the observed WebView2/process-tree peak substantially, while renderer private bytes remained high. CPU mean and FPS were similar in these runs.

A subsequent 2.03-minute disabled-GPU verification confirmed the updated JSON/Markdown role summaries: 48 samples, 569 publications = 569 ACKs, one Settings cycle and one display cycle, zero audit gaps, and temporary profile cleanup.

Reports:
- GPU disabled: `%TEMP%\photobooth-preview-soak-8014c237e26d49c197ecac1e3bcbbac9\summary.json`
- GPU enabled: `%TEMP%\photobooth-preview-soak-5318ffaca8aa40a395cfb9d639ae7180\summary.json`
- Report-schema verification: `%TEMP%\photobooth-preview-soak-858681b1d1f5428a8d0a6bf79923a6be\summary.json`

## Consequences and remaining limits

- The soak report can now attribute WebView2 private bytes by subprocess role.
- Developers can compare the same short preview path with and without GPU acceleration without changing the normal app default.
- The renderer remains a material memory consumer in both modes. A real low-RAM reference PC, longer target-hardware validation, broader camera/display coverage, visual responsiveness and capture quality still need evaluation.
- This ADR does not approve Wails as the production UI, set a minimum RAM specification, prove or disprove a memory leak, or complete the Stage 1 exit gate.
