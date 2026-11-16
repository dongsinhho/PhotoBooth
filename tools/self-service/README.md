# Self-service diagnostics

The environment inspector and local network audit are read-only helpers for a local checkout. They do not open the camera, start the player, write the station profile, create sample photos, delete files, or upload diagnostics. The SMB remote-administration helper documented below is intentionally state-changing and requires UAC Administrator approval for Enable/Disable; do not treat it as a read-only diagnostic.

## Inspect the reference/sample environment

From the repository root:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1
~~~

Check Windows Plug-and-Play camera names without opening the camera:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1 -CheckCamera
~~~

Inspect a different profile without changing it:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\inspect-environment.ps1 -ConfigPath "$env:LOCALAPPDATA\PhotoBooth\store-config-v1.json" -CheckCamera
~~~

The checker validates:
- Go availability/version.
- Station config JSON schema version, supported local/free modes and shot range.
- Frame JSON existence, schema version and equality between configured shot count and slot count.
- Whether configured session/output directories already exist (it does not create them).
- Optional Windows PnP camera names. PnP presence is not proof that Media Foundation can stream or capture.

Exit code is 1 when a required config/tool check fails. Missing storage directories and an unchecked camera are warnings, not automatic failures.

## Other self-service resources

- [Windows Quickstart](../../docs/QUICKSTART_WINDOWS.md)
- [Windows Troubleshooting](../../docs/troubleshooting/WINDOWS_TROUBLESHOOTING.md)
- [Compatibility report template](../../docs/COMPATIBILITY_REPORT_TEMPLATE.md)
- [Camera compatibility matrix](../../docs/knowledge-base/CAMERA_COMPATIBILITY_MATRIX.md)
- [Camera POC instructions](../camera-poc/README.md)

Do not attach guest photos, webcam frames, rendered strips, credentials, payment screenshots, tokens, serial numbers, or unredacted personal paths to issue reports.


## Audit local network exposure

Use the dedicated read-only audit against the actual booth profile:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\audit-local-network-exposure.ps1
~~~

Or specify a profile explicitly:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\audit-local-network-exposure.ps1 -ConfigPath "$env:LOCALAPPDATA\PhotoBooth\store-config-v1.json"
~~~

It checks local/fixed storage roots, broad NTFS allow entries, overlapping SMB shares and share ACLs, SMB protocol/service/listeners, active network categories, effective firewall defaults and named SMB-related inbound rules. It attempts to enumerate current SMB sessions; that check may require an elevated PowerShell session. The script never changes firewall/share/service state and never opens photo/session contents. WARN means the host-local gate is incomplete; run the elevated and second-device checks documented in [NETWORK_EXPOSURE_AUDIT.md](../../docs/knowledge-base/NETWORK_EXPOSURE_AUDIT.md).


## Optional Windows SMB remote administration

**Policy:** SMB Server and workstation administrative shares are disabled by default for deployed booth images. Remote administration is an explicit capability and is not guest-photo delivery.

Helper: tools/self-service/manage-smb-remote-admin.ps1

Read the current state (no elevation required for the status path):

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\manage-smb-remote-admin.ps1 -Action Status
~~~

Apply the default-off state (requests UAC Administrator approval when necessary):

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\manage-smb-remote-admin.ps1 -Action Disable
~~~

Explicitly enable SMB remote administration from one trusted physical adapter, allowing only named remote private IPv4 source addresses:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\manage-smb-remote-admin.ps1 -Action Enable -InterfaceAlias Ethernet -RemoteAddress 10.10.10.11
~~~

Use the actual adapter name/address. Add further -RemoteAddress values only for individually trusted maintenance devices. If the adapter has multiple IPv4 addresses, supply -LocalAddress as well. The selected Windows network profile must already be **Private**; the helper refuses Public profiles and never changes network classification itself.

Behavior:
- Enable/Disable re-launch through Windows UAC when the current process is not elevated. The elevated PowerShell window remains open to show the result.
- Enable starts LanmanServer, enables workstation administrative-share creation, and creates a PhotoBooth-owned inbound Allow rule for **TCP 445 only**, on the selected local IPv4 address and Private profile, from the explicit remote IPv4 allowlist. The helper keeps its TCP 445 Block rule on Public and Domain profiles, so other interfaces remain blocked even when one trusted Private interface is enabled.
- Legacy NetBIOS ports remain blocked: TCP 137/139 and UDP 137/138. The helper does not enable SMB1, guest/anonymous SMB, router port-forwarding, or modify share/NTFS ACLs.
- Disable first establishes PhotoBooth-owned inbound Block rules for TCP 445 on all profiles and legacy NetBIOS ports, removes only the PhotoBooth Allow rule, sets AutoShareWks=0, sets LanmanServer startup to Disabled, and stops the service. It does not delete IPC$ or manually configured share definitions.
- Both state-changing actions abort if active SMB sessions, running dependent services, or custom shares are found. Enable additionally requires a Private selected network and aborts on conflicting effective SMB firewall rules. Disable is allowed on Public profiles so the operator can close SMB regardless of network classification. Review blockers instead of forcing changes.
- The allowlist restricts network sources; it does not authenticate a user or grant a share permission. Windows SMB authentication and the existing share/NTFS ACLs still apply.

The PowerShell -WhatIf switch previews the requested state transition through ShouldProcess. It does not replace validation on a disposable Windows image.

**Validation status (2026-11-16):** PowerShell parser check, RFC1918 allowlist tests, repeated-disable preflight and read-only ActiveStore Block-rule verification passed on ShinPc. The operator executed Disable on ShinPc; LanmanServer is now Stopped/Disabled, AutoShareWks=0, the managed Allow rule is absent and all three managed Block rules are effective. The first execution reported a false failure because PID 4 kernel listeners remained; the helper now reports these as warnings after verifying service/registry/firewall state. No corrected state-changing rerun was performed. Enable and the UAC/UI integration remain unvalidated; repeat remote TCP reachability tests and disposable-VM/reboot checks are pending. Production provisioning must still apply -Action Disable to establish the default on new booths. See [ADR 0031](../../docs/adr/0031-opt-in-windows-smb-administration.md).


## 2026-11-16 Disable follow-up: service stopped, kernel listeners remain

The first operator-run Disable attempt changed the intended state but returned an error at the listener post-check. A read-only inspection then confirmed:

- LanmanServer: Stopped / Disabled.
- AutoShareWks: DWORD 0.
- PhotoBooth allow rule: absent.
- PhotoBooth inbound Block rules: present and effective in ActiveStore for TCP 445, TCP 137/139, and UDP 137/138.
- LanmanWorkstation remains Running; kernel drivers Srv2 and NetBT remain loaded.
- Get-NetTCPConnection still reports ports 445/139 with OwningProcess 4 (System). This is not by itself proof that the Server service is running or that a remote client can complete an SMB connection. The helper must report these kernel listener registrations and rely on the verified service/registry/effective firewall state; a second-device TCP probe remains the external acceptance check.

The initial post-check incorrectly treated any listener as Disable failure. The follow-up execution also showed that Get-SmbShare returns ERROR_SERVICE_NOT_ACTIVE when LanmanServer is stopped. The helper was corrected:
- When the service is stopped, Status no longer calls Get-SmbShare; it reports shares are not being served and may display persistent share definitions from the registry.
- Repeated Disable skips live-share enumeration when the service is already stopped; it leaves any saved definitions untouched.
- Enable checks persistent share definitions while stopped and queries Get-SmbServerConfiguration only after the service starts (or beforehand if it was already running).
- Disable now verifies LanmanServer Stopped/Disabled, AutoShareWks=0, absence of the app Allow rule, and exact effective ActiveStore block filters before reporting success. Remaining PID 4 kernel listeners are warnings, not an automatic failure.

Read-only validation after the correction passed: PowerShell AST parse, repeated-disable share preflight, and actual ActiveStore filter verification for all three managed block rules. No further state-changing Disable/Enable was run during this follow-up. Current ShinPc is already in the service/registry/firewall default-off state; repeat remote reachability from Unix after this state change is still required.
