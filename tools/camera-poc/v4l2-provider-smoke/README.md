# Experimental Linux V4L2 provider smoke

This is an engineering spike for the Linux/amd64 V4L2 camera provider. It is not a production configuration option and is not wired into the station Settings UI or Wails player.

## Current validated reference modes

The reference device is the Logitech B525 HD Webcam passed into WSL2 Ubuntu 26.04.1 through usbipd-win. The latest continuation used WSL2 kernel `6.18.58.1-microsoft-standard-WSL2`.

Two MJPEG modes have passed provider and end-to-end workflow acceptance on this WSL2 USB/IP path, with different depths of endurance/recovery evidence:

| Mode | Provider smoke | Workflow smoke | Additional lifecycle evidence |
|---|---|---|---|
| 640x480 @ 15 FPS | 3 captures decoded | 3 captures → render → local output | Earlier 100-cycle same-instance lifecycle, 15-minute single-stream soak and 3-cycle physical recovery campaign on WSL2 + B525 |
| 1280x720 @ 5 FPS | 3 captures decoded | 3 captures → render → local output | 10 same-instance lifecycle cycles; 15-minute soak (1,800 previews + 1,800 captures); 3 consecutive physical in-process hotplug recoveries in one workflow |

Both modes use `/dev/video0` (single-planar Video Capture + Streaming), four MMAP buffers, one-second default warm-up, five-second frame timeout, and Linux/amd64. `1280x720@5` passed a 15-minute one-stream soak on 2026-11-11 (1,800/1,800 previews and 1,800/1,800 captures fully decoded; one Open/Close; exit 0). On 2026-11-12, a six-shot diagnostic workflow passed three consecutive real USB detach/reattach recoveries in the same workflow process: each detach produced `revents 0x18`, each reattach reopened the provider at attempt 4/60, then all 6/6 captures were decoded, rendered and saved to local output; `HOTPLUG_RECOVERY_PASS requested_cycles=3 observed_successes=3`, exit 0. The six-slot frame was a temporary diagnostic input and was removed after the test.

The B525's `/dev/video1` node is metadata capture only and is rejected by the provider. Do not treat it as a second camera. `v4l2-ctl --list-formats-ext` enumerates many YUYV/MJPEG formats, but enumeration alone does not prove a stream works. On the current WSL2 USB/IP + B525 path, 1280x720 MJPEG at 10 or 15 FPS and 640x480 MJPEG at 30 FPS failed during warm-up after five consecutive `V4L2_BUF_FLAG_ERROR` buffers. 1280x720 at 5 FPS passed. 1920x1080@5 is enumerated but remains untested. These results are specific to this WSL2/usbipd-win setup and must not be generalized to direct USB on native Linux.

## Run provider smoke

From the repository root on Linux/amd64:

~~~bash
go run ./tools/camera-poc/v4l2-provider-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 -frames 3
~~~

The command opens one V4L2 stream, performs the default warm-up, validates one preview frame, captures the requested JPEGs, fully decodes the saved captures, closes the camera and removes its temporary artifacts.

For diagnostics only, `-skip-warmup` skips the default warm-up. It must not be used as a production setting.

## Run workflow smoke

~~~bash
go run ./tools/camera-poc/v4l2-workflow-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 -shots 3
~~~

This runs the existing workflow runner with the experimental provider, the 3-slot sample frame, render pipeline and local output dispatcher. It verifies all captures, rendered JPEG and local output by fully decoding the images, then removes the temporary test workspace.

## Implementation safeguards

- Opens only nodes exposing single-planar `V4L2_CAP_VIDEO_CAPTURE` and `V4L2_CAP_STREAMING`; metadata-only nodes are rejected.
- Requests and verifies exact MJPEG dimensions, negotiates and reads back the integer frame rate.
- Uses MMAP buffers, copies frame bytes before requeue, honors `V4L2_BUF_FLAG_ERROR`, and polls with bounded context cancellation.
- Discards at most five consecutive malformed JPEG buffers within a bounded per-frame deadline. Persistent errors return `camera.ErrStreamUnavailable`.
- Uses the shared MJPEG normalization helper and checks the JPEG header/dimensions before preview/capture.
- Capture creates the caller-owned staging path with exclusive no-replace semantics, syncs and closes it. The workflow then performs its normal full JPEG validation and atomic publication.
- Preview and Capture serialize access to the same stream.
- The provider-smoke CLI returns errors from its `run` function before `main` exits, so deferred provider Close and temporary-directory removal execute on early Open/Preview/Capture failures as well as on success. A deliberate missing-device regression smoke returned the expected error and left no provider-smoke temp directory.
- The workflow-smoke hotplug timeout is computed from the exact capped retry schedule (60 attempts, 500ms linear backoff capped at 2s: 115 seconds maximum per recovery), plus each countdown and per-cycle overhead. Unit tests cover the timeout-budget calculation; the previous fixed 45-second allowance was insufficient for repeated recovery cycles.
- Provider is not selectable in `StoreConfig`. Current supported config remains `fake` or `windowsmf`. No Wails Linux camera UI/provider path or production claim is implied.

## Remaining gates

- Three consecutive operator-assisted in-process USB detach/attach recovery cycles passed in one workflow process at both 640x480@15 and 1280x720@5. At 1280x720@5 the campaign used a temporary six-slot diagnostic frame; all 6 captures, render and local output passed. Repeated hotplug campaigns across separate sessions/days and longer detach intervals remain open.
- 100 same-instance lifecycle cycles and a 15-minute single-stream soak (1,800 previews + 1,800 decoded capture JPEGs) passed at 640x480@15. At 1280x720@5, 10 lifecycle cycles and a separate 15-minute single-stream soak passed (1,800/1,800 previews + 1,800/1,800 captures decoded; one Open/Close). Full-shift endurance remains open.
- Native Ubuntu/direct-USB runtime validation remains open; WSL2 USB/IP results must not be generalized to a direct Linux USB host controller.
- Production config/UI selection remains deferred pending native Linux runtime validation and complete operational acceptance.
- Linux/arm64 is currently excluded; the Linux provider implementation is built only for amd64.


To exercise repeated cleanup/reopen on the **same Go provider instance**, use:

~~~bash
go run ./tools/camera-poc/v4l2-provider-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 \
  -frames 1 -iterations 100
~~~

Each iteration performs Open (including the default warm-up), Preview, Capture + full JPEG decode, and Close before the next Open. On the reference WSL2 + B525 setup, 100/100 such cycles passed in 2m21.767s and the CLI emitted `SMOKE_PASS`. The harness uses a six-minute context for this test.

## Run a continuous single-stream soak

To keep one provider instance and one V4L2 stream open while repeatedly exercising Preview and Capture, run:

~~~bash
go run ./tools/camera-poc/v4l2-provider-smoke \\
  -device /dev/video0 -width 640 -height 480 -fps 15 \\
  -stream-soak=15m -stream-soak-interval=500ms
~~~

The duration is measured after the provider opens and completes warm-up. Each loop decodes the preview JPEG and captured JPEG and verifies 640x480 dimensions. The tool logs progress every 20 pairs, closes the provider once at the end and removes all temporary captures. Duration is bounded to 30 minutes; the overall context includes one additional minute. The reference WSL2 + B525 run passed first at 3m0.005s (360/360 preview + capture pairs), then at 15m0.0106s with 1,800/1,800 previews and 1,800/1,800 captures (`STREAM_SOAK_PASS`). Both runs used one Open/Close cycle and every JPEG decoded at 640x480. This is stronger continuous-stream evidence, not a full-shift endurance certification.
