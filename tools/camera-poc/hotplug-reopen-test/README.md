# Physical USB hot-unplug/replug recovery test

This is a manual, hardware-dependent test of the same `camera/recovery` decorator used by the Windows Media Foundation booth-player path. It is intentionally kept separate from automated tests because it requires physically unplugging and reconnecting a real camera.

## Run

From the repository root in Windows PowerShell, with the Logitech B525 connected:

```powershell
Set-Location .\tools\camera-poc\hotplug-reopen-test
go run .
```

The harness opens the B525 and captures repeatedly in one long-lived Go process. When you unplug the USB cable, it waits for the stream read to fail, closes the invalid provider, and retries opening the device for up to 30 attempts with capped linear backoff (roughly 28 seconds of retry delays, plus device open/close time). Reconnect the webcam while those attempts are running. A passing run prints `AUTO_RECOVERY_SUCCEEDED` and `CAPTURE_OK_AFTER_RECOVERY` without restarting the process.

The harness continues to capture after recovery, up to 600 attempts. At the end it reports counts and the result of closing the provider. Stop the run with Ctrl+C only if needed; a normal complete run takes several minutes.

## What a pass proves

- The specific B525 + Windows 11 + Media Foundation combination detected the unplug event.
- The same process closed the invalid provider and opened a new provider after re-enumeration.
- A capture succeeded again without restarting the Go process or the computer.

## What a pass does not prove

- The full production GUI/kiosk can resume or safely restart a guest session.
- A capture blocked inside a native driver call can always be cancelled immediately.
- Every webcam, driver, USB hub, sleep/resume state or Linux backend recovers identically.
- Recovery succeeds if the device remains absent beyond the bounded retry window.

Test images are written only under the user's TEMP directory and are automatically removed when the harness exits normally. If the harness is forcibly interrupted, delete the temporary photobooth-hotplug-recovery directory after local inspection.
