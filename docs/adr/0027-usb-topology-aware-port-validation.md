# ADR 0027 — USB topology-aware port validation

**Status:** Accepted for test tooling; recovery on the alternate USB controller remains unvalidated.
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
- The location-path change is confirmed; alternate-port recovery is not yet validated.

Reports:
- First port-switch precondition timeout: %TEMP%\photobooth-idle-preview-hotplug-5c75e6ecadc44920af1b0c6d0ca96433.json.
- XHC1 attempt with 90-second unplug window: %TEMP%\photobooth-idle-preview-hotplug-9010cd33183642a1852ca0815925894b.json.
- XHC1 same-port reconnect attempt: %TEMP%\photobooth-idle-preview-hotplug-cf6df3a56cf7406f906840c028e7f64c.json.

## Consequences and limits

- The harness can now demonstrate that the test starts on a path/controller different from the previous passing port and can verify reconnection to that same alternate path.
- XHC1 is currently a different PnP/controller path from the original XHC0 reference, but a successful unplug/replug cycle on XHC1 is still required.
- No conclusion is made about USB hardware reliability, driver compatibility or the reason the physical unplug was not observed. The B525 stayed present/healthy during the failed attempts.
