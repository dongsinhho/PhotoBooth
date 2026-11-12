# Experimental Linux V4L2 workflow smoke

Run from the repository root on Linux/amd64:

~~~bash
go run ./tools/camera-poc/v4l2-workflow-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 -shots 3
~~~

The smoke creates isolated temporary session and output directories, runs three captures through the real `workflow.Runner`, renders `tools/camera-poc/sample-frame.json`, sends the result through `output.NewLocal`, fully decodes each capture/render/local-output JPEG, and removes the temporary workspace on exit.

This is an experimental integration acceptance tool. It does not enable `camera.provider=v4l2` in the station config and does not by itself validate USB hotplug recovery, printing, native Ubuntu direct-USB support, or production kiosk deployment. On both success and failure, the smoke removes its isolated temporary workspace before exiting.


## Optional in-process USB hotplug recovery test

The smoke wraps the experimental provider in the existing `camera/recovery` decorator. To run one physical detach/attach recovery cycle:

~~~bash
go run ./tools/camera-poc/v4l2-workflow-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 -shots 3 \
  -test-hotplug -hotplug-countdown=20s -hotplug-cycles=1
~~~

To require three physical cycles in the same workflow process, one during each shot countdown:

~~~bash
go run ./tools/camera-poc/v4l2-workflow-smoke \
  -device /dev/video0 -width 640 -height 480 -fps 15 -shots 3 \
  -test-hotplug -hotplug-countdown=20s -hotplug-cycles=3
~~~

When `HOTPLUG_TEST_ARMED` appears, repeat the following for each of the first `-hotplug-cycles` shot countdowns:

1. Wait for `WORKFLOW_COUNTDOWN shot=N seconds=<hotplug-countdown>` (20 seconds by default). In Windows PowerShell detach BUSID 6-4: `& "C:\Program Files\usbipd-win\usbipd.exe" detach --busid 6-4`.
2. Leave it detached until the CLI reports `CAMERA_RECOVERY type=recovery_open_attempt_failed` (the open retry loop has begun).
3. Attach it again: `& "C:\Program Files\usbipd-win\usbipd.exe" attach --wsl --busid 6-4`.
4. Wait for `CAMERA_RECOVERY type=recovery_succeeded` before detaching for the next shot.

PASS requires the requested number of recovery events, a final `HOTPLUG_RECOVERY_PASS` summary, then all three captures, render and local output to complete in the same process. The smoke's overall timeout scales with the requested cycle count (bounded by the shot count); the recovery decorator allows up to 60 bounded reopen attempts per failure.

An operator-assisted single-cycle test passed on 2026-11-08 (reopened at attempt 8). On 2026-11-09, a three-cycle campaign also passed in one process on WSL2 USB/IP + B525: recovery reopened at attempts 9, 7 and 10, emitted `HOTPLUG_RECOVERY_PASS requested_cycles=3 observed_successes=3`, then completed all three captures, render and local output (exit code 0). This validates three consecutive detach/reattach recoveries in one workflow; repeated-day endurance and native direct-USB recovery remain unvalidated. Do not detach a camera during a real guest session.
