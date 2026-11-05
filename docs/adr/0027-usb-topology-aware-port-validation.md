# ADR 0027 — USB topology-aware port validation

**Status:** Accepted for test tooling; three physical XHC1 idle-preview cycles observed same-port unplug/replug on 2026-11-02. Automatic recovery failed in all three; the third confirmed 30 native zero-source enumeration results. A later user-initiated Retry Connect restored preview, but automatic recovery remains unvalidated/failing. Root cause and device-availability timing remain open.
**Date:** 2026-11-01

## Context

ADR 0026 recorded one successful physical idle-preview B525 unplug/replug on the original reference port. The next coverage gate is to test the same camera on a different USB port/controller. A device name or USB instance ID alone is not enough to prove the port changed; Windows may keep the same device instance ID when a device is moved.

## Decision

1. Record DEVPKEY_Device_LocationPaths, DEVPKEY_Device_LocationInfo, the USB parent and the resolved host-controller identity before and after the physical cycle.
2. Support -BaselineLocationPath to require the initial attached camera path to differ from a previously validated reference path.
3. Support -RequireSamePortOnReconnect to require the camera to return to the exact same location-path set used for the current test. This isolates the unplug/replug recovery result to the selected alternate port.
4. Retain -RequireDifferentPort for the separate case where a test intentionally requires a new physical path after unplug.
5. Do not count a test as a recovery failure if Windows never reports PnP removal. Mark it as not executed / physical unplug not observed; no recovery code path was exercised.
6. Do not infer which chassis socket is which from controller names alone. Use Windows location paths and record the observed topology. The operator must physically choose the desired socket.

Example: test the current alternate path against the original ADR 0026 reference path and reconnect on the same alternate socket:

~~~powershell
.\ui-idle-preview-hotplug-smoke.ps1 -PreflightOnly -BaselineLocationPath 'PCIROOT(0)#PCI(0801)#PCI(0003)#USBROOT(0)#USB(1)#USB(2)#USBMI(2)' -RequireSamePortOnReconnect

.\ui-idle-preview-hotplug-smoke.ps1 -BaselineLocationPath 'PCIROOT(0)#PCI(0801)#PCI(0003)#USBROOT(0)#USB(1)#USB(2)#USBMI(2)' -RequireSamePortOnReconnect
~~~

## Observed topology on 2026-11-01

Original ADR 0026 passing port:
- Location path: PCIROOT(0)#PCI(0801)#PCI(0003)#USBROOT(0)#USB(1)#USB(2)#USBMI(2).
- Host controller: AMD USB 3.10 eXtensible Host Controller (XHC0).
- Parent path includes a Generic USB Hub and USB 3.0 Root Hub.

Alternate attached path observed before the later retry:
- Location path: PCIROOT(0)#PCI(0801)#PCI(0004)#USBROOT(0)#USB(1)#USB(4)#USB(3)#USBMI(2).
- Host controller: AMD USB 2.0 eXtensible Host Controller (XHC1).
- Parent path includes two Generic USB Hubs and a USB Root Hub.
- The alternate path differs from the previously validated XHC0 path and was the starting path for the XHC1 preview attempt.

## Validation status

- PowerShell parser: PASS.
- Preflight with -BaselineLocationPath and -RequireSamePortOnReconnect: PASS; baseline_port_different=true.
- XHC1 idle preview started and reached 5 frames / 5 ACKs in one run.
- One XHC0 attempt did not observe Windows PnP removal within 45 seconds. After the B525 was attached on XHC1, two further XHC1 attempts (90-second unplug windows each) also did not observe PnP removal. All three attempts were recorded as FAIL at the unplug precondition; none reached reconnect, so none exercised recovery on XHC1.
- All three disposable profiles were removed, no photo session was created in any run, the app exited, and the camera remained healthy on XHC1.
- The location-path change is confirmed. Three XHC1 physical idle-preview cycles were exercised on 2026-11-02; all observed PnP removal/return and same-path reconnection. Automatic preview recovery exhausted 30 attempts and ended unavailable in every cycle. The third cycle, using the improved enumeration classification, recorded 30/30 attempts returning `Media Foundation MFEnumDeviceSources returned zero video capture sources`. After automatic failure, the operator clicked Retry Connect; preview restarted and returned to ready with 16 frames / 16 ACKs. This demonstrates manual recovery after the failed retry window, not automatic recovery.
- Separate controls after the failures found one B525 through `MF.Init`/`MF.EnumDevice` in a standalone process, including while Wails held normal preview. Twenty-four repeated runtime init/enumerate/shutdown cycles (12 on one locked OS thread and 12 on new locked worker threads) also found one B525 every time. These controls show that ordinary enumeration and sequential MF reinitialization work while the camera remains connected; they do not reproduce enumeration immediately after a physical unplug/replug in the same app process.
- A further 90-second observer/control run did not observe PnP removal; the independent observer returned one B525 in all 151 samples and Wails preview remained ready (444 frames / 444 ACKs). This run is a non-hotplug control, not a recovery attempt.
- The improved enumeration classification was physically exercised in the third cycle. It confirmed the genuine zero-native-source branch for all 30 automatic reopen attempts; the all-activation-metadata-failed branch was not observed. A later user-initiated Retry Connect restored preview. Root cause/time-to-availability remains unknown.

Reports:
- First port-switch precondition timeout: %TEMP%\photobooth-idle-preview-hotplug-5c75e6ecadc44920af1b0c6d0ca96433.json.
- XHC1 attempt with 90-second unplug window: %TEMP%\photobooth-idle-preview-hotplug-9010cd33183642a1852ca0815925894b.json.
- XHC1 same-port reconnect attempt without PnP unplug observation: %TEMP%\photobooth-idle-preview-hotplug-cf6df3a56cf7406f906840c028e7f64c.json.
- XHC1 same-port physical unplug/replug with failed preview recovery, first run (2026-11-02): %TEMP%\photobooth-idle-preview-hotplug-35f48692d9e549f38eca89b68d54219d.json.
- XHC1 same-port physical unplug/replug with improved enumeration classification (2026-11-02): %TEMP%\photobooth-idle-preview-hotplug-02a775bf7a90498798f83990dd6292c6.json (PARTIAL: automatic recovery failed; operator Retry Connect restored preview).
- XHC1 non-hotplug observer/control run (PnP removal not observed): %TEMP%\photobooth-idle-preview-hotplug-f31d4b6a61ca4ab7a0b22cb8ee8437fc.json.

## Physical XHC1 idle-preview recovery attempts (2026-11-02)

Machine: ShinPc / Windows 11 Pro build 26200 / Logitech B525 HD Webcam / Wails `PhotoBooth.exe`.

- Harness: `ui-idle-preview-hotplug-smoke.ps1` with the ADR 0026 XHC0 `-BaselineLocationPath`, `-RequireSamePortOnReconnect`, 90-second unplug/reconnect observation windows, 25-second backend recovery-state timeout and 60-second preview-resume timeout.
- Initial camera path was XHC1 (`PCIROOT(0)#PCI(0801)#PCI(0004)#USBROOT(0)#USB(1)#USB(4)#USB(3)#USBMI(2)`), distinct from the XHC0 baseline. Initial preview became ready with 6 frames / 5 ACKs; immediately before unplug it had 11 frames / 11 ACKs.
- Physical unplug was observed by Windows PnP. Wails emitted the `reconnecting` preview status. Physical replug was observed for the same B525 instance, and Windows reported the exact same XHC1 location-path set before and after reconnect (`USB_SAME_PORT_RECONNECT=True`).
- **Recovery result: FAIL.** The Wails preview never returned to `ready`. Windows Media Foundation camera reopen exhausted 30 attempts with `Windows Media Foundation found no camera devices`; the backend transitioned to `unavailable`. The JSONL contains 31 reconnecting status events and no ready-after-reconnecting event. `preview_after_reconnect` is null.
- Event audit: 374 JSONL events, 0 invalid JSON lines; 169 published frames and 169 ACKs; 0 cancellations, ACK timeouts, decode failures, duplicate publishes, duplicate terminal sequences, unmatched published sequences or orphan terminal sequences. This confirms frame ACK bookkeeping, not stream recovery.
- No photo session was started, the app was not force-stopped, and the isolated TEMP profile was cleaned. Harness exit code: 1.
- Report: `%TEMP%\photobooth-idle-preview-hotplug-35f48692d9e549f38eca89b68d54219d.json`.
- Exact diagnostic event log: `%TEMP%\photobooth-idle-preview-hotplug-35f48692d9e549f38eca89b68d54219d.jsonl`.

This is a confirmed physical XHC1 recovery failure after successful PnP re-enumeration. It does not identify whether the cause is Media Foundation timing, provider lifecycle, driver behavior or the specific port topology. Keep root-cause investigation separate from the successful XHC0 reference-port result.

## Second instrumented XHC1 cycle and non-hotplug controls (2026-11-02)

The instrumented Wails executable (`PhotoBooth.exe`) was rebuilt from the Wails submodule after adding opt-in per-attempt recovery telemetry. A second physical XHC1 same-port unplug/replug cycle was executed:

- PnP unplug and replug were observed; the same B525 instance returned on the exact same XHC1 Location Path. The initial idle preview reached 6 frames / 5 ACKs; immediately before unplug it reached 14 frames / 14 ACKs.
- Recovery failed again; preview did not return to `ready`. Exact JSONL now contains 30 `recovery_open_attempt_failed` records, attempts 1 through 30, all with `total=30` and the identical message `Windows Media Foundation found no camera devices`; followed by `recovery_failed` and `unavailable`.
- Event audit: 278 records, 0 invalid JSON lines; 105 published frames = 105 ACKs, with no cancellations, ACK timeouts, decode failures, duplicate publishes, duplicate terminal sequences, unmatched publishes or orphan terminals. No photo session, no forced app stop, TEMP profile cleaned. Harness exit code 1.
- Report: `%TEMP%\photobooth-idle-preview-hotplug-79e498bb735c40209b34726ff3793f84.json`.
- Diagnostics: `%TEMP%\photobooth-idle-preview-hotplug-79e498bb735c40209b34726ff3793f84.jsonl`.

Non-hotplug Media Foundation controls after the failure:
- A standalone Go process called `MF.Init` → `MF.EnumDevice` → `MF.Shutdown` and found exactly one `B525 HD Webcam` while Windows PnP reported `Status=OK`.
- The same independent enumeration found one B525 while a normally running `PhotoBooth.exe` was holding the idle preview.
- Twelve sequential MF init/enumerate/shutdown cycles on one locked OS thread all found one B525. A further 12 cycles, each on a newly locked worker OS thread, also all found one B525.
- A separate 90-second observer/control run did not observe PnP removal; its independent observer found one B525 in all 151 half-second samples, and Wails remained `ready` with 444 frames / 444 ACKs. Treat this as a stationary-camera control, not a hotplug recovery attempt. Report: `%TEMP%\photobooth-idle-preview-hotplug-f31d4b6a61ca4ab7a0b22cb8ee8437fc.json`.

Enumeration error classification was hardened in the vendored compatibility copy. `EnumDevice` now returns a contextual error if `MFEnumDeviceSources` provides one or more activation sources but every source fails friendly-name/symbolic-link metadata extraction; the previous code silently skipped such failures and could return an empty list. The Wails Media Foundation provider explicitly reports a zero-source `MFEnumDeviceSources` result. Unit tests cover the distinction, and the third physical XHC1 cycle confirmed that native enumeration genuinely returned zero sources on all 30 automatic reopen attempts.

## Third instrumented XHC1 cycle with enumeration classification (2026-11-02)

A third physical XHC1 same-port unplug/replug used the freshly built executable containing per-attempt telemetry and the vendored enumeration-classification change.

- PnP observed physical removal and return of the same B525 instance. Both XHC1 location paths before and after were identical, and differed from the XHC0 baseline (USB_SAME_PORT_RECONNECT=True).
- Initial preview was ready with 5 frames / 5 ACKs; immediately before unplug it reached 10/10.
- The first read after unplug reported MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED (0xC00D3EA2), triggering automatic recovery and reconnecting.
- All 30 automatic reopen attempts (attempts 1–30) returned the explicit error: Media Foundation MFEnumDeviceSources returned zero video capture sources. The backend then emitted recovery_failed and preview state unavailable. This physically confirms the genuine native zero-source branch; no all-metadata-failed error was observed.
- The operator then clicked Retry Connect. A new preview start event appeared at 16:26:34.102 UTC; ready followed at 16:26:36.305 UTC (about 2.2 seconds later). Preview resumed to 16 received / 16 decoded frames and 16 ACKs at 5.15 FPS. This proves the manual Retry Connect path worked after the automatic retry window; it does not make automatic recovery a pass.
- Report audit: status=PARTIAL, automatic_recovery_status=FAIL, 30 attempt failures, 1 recovery_failed, 0 recovery_succeeded, preview_restarted_after_recovery_failure=true, ready_after_recovery_failure=true. The harness originally classified this run as PASS based only on a later ready status after reconnecting; the saved report was corrected and the harness audit was updated to distinguish these sequences.
- Full event audit: 228 JSONL records, 0 invalid lines; 79 published frames = 79 ACKs; zero cancellations, ACK timeouts, decode failures, duplicate publishes, duplicate terminal sequences, unmatched published sequences or orphan terminal sequences. No photo session was started, no forced app stop occurred, and the temporary profile was cleaned.
- Report: %TEMP%\photobooth-idle-preview-hotplug-02a775bf7a90498798f83990dd6292c6.json.
- Diagnostics: %TEMP%\photobooth-idle-preview-hotplug-02a775bf7a90498798f83990dd6292c6.jsonl.

The automatic retry budget ended at 30 attempts around 16:26:01 UTC; manual Retry Connect was invoked about 33 seconds later and succeeded. This timing motivates a concurrent external Media Foundation enumeration observer during the next controlled hotplug cycle. It is not yet proof that extending the retry window is the correct fix.

## Consequences and limits

- The harness can now demonstrate that the test starts on a path/controller different from the previous passing port and can verify reconnection to that same alternate path.
- XHC1 is a different PnP/controller path from the original XHC0 reference. Three physical XHC1 unplug/replug cycles are confirmed; automatic preview recovery failed after the camera returned on the same path in all three. In the third cycle, a later user-initiated Retry Connect successfully reopened preview, but this remains a manual recovery result.
- The third instrumented run confirms that all 30 automatic reopen attempts returned a genuine zero-source `MFEnumDeviceSources` result. Standalone and repeated non-hotplug Media Foundation enumeration controls found the B525 while it remained connected. The manual retry worked about 33 seconds after the last automatic attempt. A concurrent observer is still needed to distinguish delayed OS enumeration from an in-process provider/lifecycle issue. This does not establish an XHC1 hardware defect, driver incompatibility, or a general B525 limitation.

## Long-lived observer state after an XHC1 removal and return (2026-11-02)

A further physical unplug/replug was attempted with the standalone Media Foundation observer running. The Wails idle-preview harness started on XHC1 and reached 6 frames / 5 ACKs, but PnP removal was not observed within its 90-second unplug window. It exited at the unplug precondition, cleaned the isolated profile and did not exercise PhotoBooth recovery. This is not a fourth recovery failure.

The independent observer continued across the physical removal and return:
- 253 samples reported one B525 before sample 254 changed to zero_sources at 2026-11-02T16:48:11.256Z (elapsed 126.5 seconds).
- It then reported zero_sources for another 348 samples until the 5-minute observer run ended at 2026-11-02T16:51:04.756Z. Each enumeration completed in milliseconds; no blocking timeout was seen.
- After the user reconnected the same cable, Windows PnP reported the exact MI_02 B525 instance present and healthy: Present=True, Status=OK, Problem=CM_PROB_NONE. Its XHC1 LocationPaths matched the same alternate port.
- The long-lived observer process continued to report zero sources after PnP had returned.
- A new observer process launched after reconnect found one B525 immediately in 21/21 samples over 10 seconds.

This is strong evidence that a stale state is retained by the long-lived Media Foundation process/runtime or its original OS thread, rather than the device simply remaining absent at the PnP layer. It does not yet distinguish process-scoped from thread-scoped state. The third XHC1 recovery cycle's manual Retry Connect also restored preview within the same PhotoBooth process, so a permanent process-wide failure is not established.

The observer now supports an optional -thread-probe-trigger-file. Its primary goroutine stays pinned to its original OS thread. When the trigger file appears, the observer serially performs one Init/EnumDevice/Shutdown on a distinct second locked OS thread, then resumes sampling on the primary thread. This avoids concurrent access to the vendored package's process-global MF/COM handles. The JSONL records primary_locked_thread vs. secondary_locked_thread. A connected-camera control passed with 17 primary samples and one secondary sample, all finding the B525; observer Go tests, vet and Windows build passed.

Reports:
- Wails unplug-precondition timeout (no PnP removal observed within 90 seconds): %TEMP%\photobooth-idle-preview-hotplug-11b02dc4676044c9b933682d7e1f8e06.json.
- Long-lived observer across removal/return: %TEMP%\photobooth-mf-enumeration-xhc1-20261102-234603.jsonl.
- Fresh process after reconnect, 21/21 samples found B525: %TEMP%\photobooth-mf-enumeration-fresh-process-20261102-235210.jsonl.

Next physical comparison: start the two-thread observer before another XHC1 unplug/replug. After Windows PnP reports the same B525 present, wait for a primary-thread zero_sources sample and create the configured trigger file. A secondary-thread camera_found result while primary remains zero would support a thread-scoped COM/MF state issue. If both threads remain zero while a fresh process succeeds, investigate state retained at the long-lived process/runtime level. Do not change recovery timing or provider lifecycle before this comparison.


## Checkpoint bổ sung - 2026-11-03: thread-scope evidence, recovery lifecycle hardening và build hợp lệ

### Kết quả chẩn đoán thread-scope

- Enhanced observer trên cùng process cho thấy primary locked OS thread tiếp tục trả về `zero_sources` sau khi Windows PnP đã nhận lại chính xác B525 MI_02.
- Khi trigger một enumeration tuần tự trên secondary locked OS thread trong cùng process, sample tại `2026-11-03T14:10:21.6721024Z` trả về `camera_found`, `device_count=1`, tên `B525 HD Webcam`, cùng symbolic link của instance MI_02 đang kiểm thử.
- Kết quả này phân biệt được thread scope với process scope tốt hơn bằng chứng trước đó: cùng process nhưng thread khác enumerate được camera trong khi primary thread vẫn ở zero-source. Đây là bằng chứng mạnh cho trạng thái COM/Media Foundation gắn với OS thread; không phải bằng chứng rằng PhotoBooth Wails đã tự recovery thành công.
- Wails harness cycle dùng kèm lần so sánh này đã dừng ở điều kiện xác nhận USB port mismatch trước khi recovery test bắt đầu, nên không có Wails recovery telemetry hợp lệ từ cycle đó.

### Production implementation hiện tại

- Thêm `camera.RetryableOpenProvider`; Windows Media Foundation provider đánh dấu failed Open có thể thử lại trên cùng provider instance.
- Khi một lần MF Open thất bại, worker cleanup native resource trên OS thread của nó nhưng giữ goroutine khóa thread đó. Retry tiếp theo tạo worker mới, buộc Go runtime dùng OS thread khác; thread cũ chỉ được release sau khi worker thay thế đã hoàn tất startup, hoặc khi retries bị hủy/hết budget/Close cuối cùng.
- Recovery decorator reuse đúng provider đánh dấu retryable thay vì tạo provider mới ở mỗi attempt. Retry budget và retry delay không đổi; vẫn là 30 attempts ở đường recovery hiện hữu. Không thay đổi topology validation hay XHC0 behavior.
- `retainOnExit` dùng `atomic.Bool` để tránh race khi worker đọc cờ ở defer trong khi caller đang Close/cancel.
- `closeWorkerAfterCancelledStartup` đợi native worker dừng và idempotently release thread ngay cả khi worker đã chọn nhánh retain trước khi nhận cancellation.
- Thêm `closeRetryableCandidate`: khi Close của retryable provider lỗi ở context cancel, retry wait cancel hoặc retry exhaustion, recovery decorator giữ lại provider reference trong `active` để caller có thể thử Close lại; không làm mất tham chiếu tới worker/thread có thể còn giữ native resource.
- Test mới bao phủ provider reuse/exhaustion, cleanup failure có thể retry, và cancellation sau khi failed worker đã retain thread. Test fixture cũ được cập nhật để khởi tạo release channel.

### Build và kiểm thử đã xác nhận

- Wails build không được thực hiện bằng `go build` trực tiếp. Build chuẩn dùng root launcher:
  `./run-photobooth.ps1 -NoLaunch`
  trong PowerShell từ repository root. Wails CLI v2.17.0 và nested module Go 1.25.0 được dùng; root Go baseline không bị thay đổi.
- Binary cuối: `tools/ui-spike/wails-v2/build/bin/PhotoBooth.exe`, 13,024,768 bytes, Windows PE header `MZ`, SHA-256 `7FE36E14AE2E54D7CCCEE5F38C43B4C6F4741F84B4CBD8A7AC8FC70A2077059F`.
- Latest post-change validation: root `go test -count=1 ./...`, vendored `go test -count=1 ./camera/v1 ./windows/mf`, Wails `go test -count=1 ./...`, root `go vet ./...`, Wails `go vet ./...`: all exit 0 (`CHECKS_FAILED=0`).
- Focused Windows MF/recovery tests and vet also pass. `git diff --check` exit 0 after normalizing whitespace-only blank lines from generated `frontend/wailsjs/go/models.ts`.
- All work remains uncommitted/unpushed. No existing user changes were reset or discarded.

### Physical validation still pending

- Latest exact B525 MI_02 PnP query reports `Status=Unknown`, `Problem=CM_PROB_PHANTOM`; the camera is still disconnected.
- Latest Wails harness startup failure was caused by launching an invalid binary built without Wails tags. It failed before initial preview and before any unplug action; it is not a recovery failure. Correctly built `PhotoBooth.exe` now exists.
- Keep the three previously confirmed XHC1 automatic-recovery failures. Do not count the Wails startup failure or earlier unplug-precondition/port-mismatch timeouts as another recovery failure.
- Do not mark the new lifecycle fix as physically validated yet. Next, reconnect B525 to the same alternate XHC1 socket; verify exact MI_02 Present=True / Status=OK and same XHC1 LocationPath, run `-PreflightOnly` against the exact `PhotoBooth.exe`, then start the physical idle-preview hotplug harness with XHC0 baseline and `-RequireSamePortOnReconnect -UnplugObservationTimeoutSeconds 180`. First prove initial frames/ACKs are flowing before asking for another unplug. During the cycle, do not click Retry Connect; collect the automatic recovery audit and distinguish PASS/FAIL from any later manual restart.

## Fifth confirmed XHC1 physical failure and correction to thread-retention lifecycle (2026-11-03)

A fifth physical idle-preview unplug/replug was performed on the same alternate XHC1 path using a production Wails binary built from the implementation that retained failed Open() worker threads. The initial preview reached 8 frames / 7 ACKs. Windows PnP observed the same B525 MI_02 disappear and return; Status=OK, and both XHC1 LocationPaths before and after were identical (USB_SAME_PORT_RECONNECT=True). Automatic recovery still failed all 30 attempts with Media Foundation MFEnumDeviceSources returned zero video capture sources; the backend emitted recovery_failed and entered unavailable. Final preview audit recorded 125 frames received / 125 decoded / 125 ACKs, with no ACK/decode failures. No photo session started; app shutdown was graceful and the temporary profile was cleaned. JSONL: %TEMP%\photobooth-idle-preview-hotplug-86629c5778514b7e8b0985f726a2dcab.jsonl. The harness exited with code 1. The report JSON was not present when subsequently read; metrics above are taken from JSONL and the final harness output.

Code inspection found that the prior thread-retention change did not cover the active preview worker. recovery.Provider.Preview called old.Close() before retrying; ordinary Windows MF captureWorker.close() explicitly disabled thread retention, so the original preview worker's OS thread was returned to the Go scheduler before recovery's first Open(). The previous implementation retained only workers whose Open() attempts themselves failed. This is a credible missing lifecycle path, not proof that it is the only cause of native zero-source enumeration.

A new optional camera.RecoveryCloseProvider contract has been introduced. The recovery decorator calls CloseForRecovery() for providers that implement it, falling back to ordinary Close() otherwise. Windows MF recovery close tears down the active native camera/MF resources on the original locked OS thread, but retains that thread in the backend's managed list while replacement workers are attempted. Retained threads are released only after replacement MF startup succeeds, or on recovery exhaustion, cancellation, or final ordinary Close. The recovery budget/delay, XHC0 behavior, topology gates and UI messaging are unchanged.

Tests added:
- Recovery decorator selects CloseForRecovery() for Preview recovery.
- Windows backend retains the original active-preview thread until a replacement can start on another OS thread.
- Existing retry-thread tests continue to verify failed Open workers remain pinned across multiple attempts.

Focused camera/recovery tests, full root tests, vendored camera tests, Wails submodule tests, root/Wails vet and git diff --check passed. A fresh production Wails binary was built with Wails CLI v2.17.0 / Go 1.25.0 (PhotoBooth.exe, 13,028,864 bytes, SHA-256 58EEA311A7297B5371C2ED490BD2D6FAD90F1FCD026187B36F193816D9CF3B4B). Physical validation of this newest CloseForRecovery change is still in progress and must not be marked as a pass until the backend emits recovery_succeeded/ready and preview frames/ACKs resume on the same XHC1 port.

### Addendum - 2026-11-03: preserve provider instance across the first recovery Open

- Reviewing the new recovery-close implementation exposed an ownership gap: after CloseForRecovery, the decorator cleared p.active and called openFresh with no candidate, so its first recovery attempt could create a fresh provider instance. That would strand the original provider's retained OS-thread list and prevent that same backend from releasing the original preview thread after a replacement succeeded.
- Added openFreshWithCandidate(ctx, recovering, initialCandidate). Capture and Preview recovery seed it with the provider whose active stream was closed. When that provider implements RetryableOpenProvider, it is reused for the first replacement Open and for subsequent failed-open retries. Context cancellation or exhaustion invokes ordinary Close on that same candidate, releasing retained threads; successful startup releases the retained threads from inside the Windows backend.
- The recovery test now verifies the same provider is reopened (factory called once, provider Open called twice) after CloseForRecovery and returns a fresh preview frame. Windows thread-retention and focused tests pass.
- A pending harness using the previous binary was deliberately stopped before any USB unplug was observed; PhotoBooth was closed with WM_CLOSE and its disposable temp profile was manually cleaned. This is not a physical recovery attempt and does not change the five confirmed failure count.
- Latest full root/vendored/Wails tests, root and Wails vet all pass (CHECKS_FAILED=0). Latest Wails build is 13,029,888 bytes, PE header MZ, SHA-256 BF6B34306D10D70D9C8C414CBB79A994EAA558CF1E8FF6B670110F4E3DB744BB. Preflight passes with B525 MI_02 Status=OK on XHC1, distinct from the XHC0 baseline. No new physical cycle has started yet; camera remains connected.


## Checkpoint - 2026-11-04: close activated Media Foundation source correctly; XHC1 reconnect window inconclusive

### Implementation: explicit IMFMediaSource shutdown

- Investigation after the in-process post-failure probe found that the vendored `Device.CloseDevice()` released `IMFSourceReader`, media type, presentation descriptor and `IMFMediaSource`, but never invoked `IMFMediaSource::Shutdown()` on the activated source.
- Added `Device.CloseDeviceWithError()`: release source reader/media type/presentation descriptor first, call `IMFMediaSource.Shutdown()`, then release the source COM reference and activation reference. Cleanup continues to Release even if Shutdown reports an error; all cleanup errors are preserved with `errors.Join`. Existing `CloseDevice()` remains source-compatible and logs cleanup errors. Partially initialized `OpenDevice()` failures now use the same cleanup path.
- The Windows MF capture worker calls `CloseDeviceWithError()` on its owning locked OS thread before `mf.Shutdown()`; source cleanup errors are logged. Added unit tests proving Shutdown precedes Release and Release still runs when Shutdown fails.
- Important: this is a native lifecycle correction supported by API contract/code inspection, but its physical hotplug benefit is **not yet validated**.

### Production build and test status

- Root `go test -count=1 ./...`: PASS.
- Vendored camera module `go test -count=1 ./camera/v1 ./windows/mf`: PASS.
- Wails submodule `go test -count=1 ./...`: PASS.
- Root and Wails `go vet ./...`: PASS. Direct vendored-module vet still emits five existing `unsafe.Pointer` warnings in `windows/mf/IMFMediaBuffer.go`, `windows/mf/mf.go`, and `camera/v1/IMFSourceReaderCallback.go`; these are outside the lifecycle change.
- `git diff --check`: PASS (Git only warns about its configured LF-to-CRLF conversion).
- Rebuilt using `run-photobooth.ps1 -NoLaunch`, Wails CLI 2.17.0 / Go 1.25.0. Binary: `tools/ui-spike/wails-v2/build/bin/PhotoBooth.exe`, 13,046,784 bytes, PE header `4D-5A-90-00`, SHA-256 `213FD95900E8BF83A974E399BD3EE399C659394953A99574026029DB17E00E02`. Build-only; app not started by build.
- Added an early harness guard that rejects the mutually exclusive flags `-RequireDifferentPort` and `-RequireSamePortOnReconnect`. The guard's explicit negative test passed. A prior mistaken invocation with both flags was stopped before any unplug; PhotoBooth closed gracefully and its temporary profile/log/report were cleaned. It is not a physical test.

### Physical XHC1 cycle after source-shutdown change

- Correct harness invocation used the baseline XHC0 LocationPath plus `-RequireSamePortOnReconnect` only (not `-RequireDifferentPort`). Preflight passed: exact B525 MI_02 was healthy on the alternate XHC1 path, distinct from the XHC0 baseline; preflight did not start the app or open the camera.
- Wails idle preview reached 5 frames / 5 ACKs at 5.95 FPS; before unplug it reached 10 / 10 ACKs at 5.33 FPS. No ACK or decode failures occurred.
- Windows PnP removal was observed, and the backend emitted `reconnecting`. All 30 automatic reopen attempts returned `MFEnumDeviceSources returned zero video capture sources`. The post-failure probe ran on OS thread 5736, different from all 31 retained preview/retry worker threads, and returned `zero_sources devices=0`. Backend entered `unavailable`.
- Harness then waited 60 seconds for PnP to report a healthy B525 but timed out: report fields `physical_replug_observed=false`, `camera_instance_id_after=null`, `camera_topology_after=null`, `same_port_reconnect_verified=false`. Thus the test did **not** establish when the camera reappeared relative to the 30 retries or within that 60-second replug-observation window. Treat this as an inconclusive replug gate, not as a confirmed post-reconnect automatic-recovery failure. No photo session started, app was not force-stopped, and temporary profile was cleaned. The frame audit remained consistent: 114 published = 114 ACKs, zero ACK timeout/decode failures/duplicates/orphans.
- After the harness ended, a fresh PnP query found the exact B525 MI_02 instance `Status=OK`; its current LocationPaths still matched the same XHC1 socket recorded before unplug (`PCI(0004)...USB(4)#USB(3)...` and the ACPI XHC1 path). A fresh standalone Media Foundation observer then found `B525 HD Webcam` in 21/21 samples over 10 seconds (first sample 7.4 ms; subsequent samples roughly 1.6–3.0 ms). This proves the camera was available to PnP and a new MF process after the harness window, but not that it was available during the automatic retry window.
- Report: `%TEMP%\\photobooth-idle-preview-hotplug-224f91e3673c4e7f886c61a19ffd2a0f.json`; JSONL: `%TEMP%\\photobooth-idle-preview-hotplug-224f91e3673c4e7f886c61a19ffd2a0f.jsonl`. Independent observer: `%TEMP%\\photobooth-mf-fresh-after-shutdown-fix-20261104.jsonl`.

### Next diagnostic gate

- Do not claim the source-shutdown fix passed physical hotplug, and do not count this cycle as a confirmed same-port post-reconnect failure: PnP did not return within the harness's 60-second window.
- Camera is currently connected and healthy on the same XHC1 path. Before another unplug, inspect the recorded cleanup warning output if available and verify a fresh Wails process can open the now-present camera. For the next controlled cycle, run an independent MF observer concurrently before unplug and extend the harness reconnect observation to `-ReconnectTimeoutSeconds 120`; correlate the observer's first `camera_found` timestamp with all 30 in-process retries. Keep recovery budget/timing unchanged until this timing is measured.
- All changes remain uncommitted/unpushed. No user changes were reset or discarded.


### Follow-up smoke after the delayed PnP return (2026-11-04)

- After the preceding harness had timed out waiting for reconnect, Windows PnP reported the same exact B525 MI_02 healthy on the XHC1 LocationPaths. A new standalone MF observer found the camera in 21/21 samples.
- A separate fresh PhotoBooth Wails process was then launched against an isolated disposable profile while that same B525 remained on XHC1. It reached `FRESH_PROCESS_PREVIEW_READY=True` with 30 frames / 30 decoded / 30 ACK, 4.99 FPS, zero ACK failures and zero decode failures. No photo session directory was created. The app closed gracefully and the temporary profile was cleaned; smoke process exit code 0.
- This confirms a fresh PhotoBooth process can open a capture stream after the delayed device return. It does not prove the original long-lived process could have recovered automatically, and it does not establish whether the camera returned just after or later than the harness's 60-second replug observation limit.
- The remaining discriminating experiment is one controlled physical cycle with a standalone MF observer started before unplug, recording the first post-return `camera_found` timestamp, and a longer 120-second harness reconnect-observation window. Keep the 30-attempt policy unchanged until the correlated timeline is available.
