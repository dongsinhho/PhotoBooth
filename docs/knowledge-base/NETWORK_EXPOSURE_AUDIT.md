# Local network exposure audit

**Current status (2026-11-16):** the elevated Windows host audit passes with 0 FAIL / 2 WARN. An independent Unix host on the same trusted LAN resolved both physical ShinPc interface MAC addresses, but TCP 445 and 139 timed out on both Wi-Fi and Ethernet paths. This is strong, path-specific evidence that inbound SMB is not reachable from the tested client at the time of the test. The administrative-share overlap and local SMB listeners remain documented warnings; no firewall, SMB, share, ACL, service, account, photo, or session contents were changed.

## Purpose and scope

Stage 2 requires evidence that local media delivery does not expose PhotoBooth photos on a network accidentally. Media delivery remains local and payment remains disabled; this audit does not add LAN delivery, a guest download endpoint, QR, or payment processing.

The host audit is read-only. It checks:

- Station media/payment policy, storage-root existence, fixed local drive, and root separation.
- NTFS allow entries for broad principals (Everyone, Authenticated Users, BUILTIN\Users, and Domain Users).
- SMB shares whose paths contain or are contained by either storage root, including share ACL summaries.
- SMB1/SMB2 configuration, the Server service, local TCP listeners on 445/139, active Windows network categories, and effective firewall profile defaults.
- Effective inbound firewall Allow rules for TCP/UDP 445/139/137/138, plus Any-port rules explicitly associated with the LanmanServer service.
- Current inbound SMB sessions when the caller has sufficient rights.

The script does not open photo/session contents or change host configuration. Runbook: [NETWORK_EXPOSURE_AUDIT_RUNBOOK.md](NETWORK_EXPOSURE_AUDIT_RUNBOOK.md).

## Elevated host audit — ShinPc — 2026-11-16

Executed from an elevated PowerShell session against:

~~~text
C:\Users\sinhv\AppData\Local\PhotoBooth\store-config-v1.json
~~~

Result: **failed=0; warnings=2; checks=15; exit code 0**.

| Check | Result |
|---|---|
| Media delivery | PASS — local |
| Payment | PASS — disabled |
| Session/photo storage | PASS — separate directories on fixed local C: |
| Storage NTFS ACLs | PASS — no broad Everyone/Authenticated Users/BUILTIN Users/Domain Users allow ACE |
| Root separation | PASS |
| SMB1 / SMB2 | PASS — SMB1 disabled; SMB2 enabled |
| LanmanServer | PASS — running |
| Active network profiles | PASS — Ethernet and Wi-Fi both Public; Public firewall enabled, inbound default Block |
| Effective inbound SMB firewall allow rules | PASS — no matching enabled inbound Allow rule for TCP/UDP 445/139/137/138, nor Any-port rule explicitly associated with LanmanServer |
| Current SMB sessions | PASS — none reported |
| SMB administrative-share overlap | WARN — C$ maps to C:\ and contains both PhotoBooth storage roots |
| SMB TCP listeners | WARN — TCP 445 on :::445; TCP 139 on 10.10.10.10, 172.29.128.1, 192.168.1.11, 192.168.226.1, and 192.168.74.1 |

The storage ACL entries include SYSTEM and BUILTIN\Administrators FullControl, ShinPc\sinhv FullControl, and ShinPc\CodexSandboxUsers ReadAndExecute/Synchronize. The share ACL reported for C$ is:

- BUILTIN\Administrators: Full
- BUILTIN\Backup Operators: Full
- NT AUTHORITY\INTERACTIVE: Full

C$ is a Windows administrative share, not a separately created guest-facing PhotoBooth share. Its path nevertheless covers the storage roots. The remote applicability of the INTERACTIVE share ACE was not independently established. Do not infer that TCP reachability or file access follows from the share ACL alone.

### Audit-script correction

The previous broad firewall classification treated arbitrary Ports=Any; Service=Any application rules as SMB-related, causing false positives. The script now treats an Any-port rule as a broad SMB rule only when its service filter explicitly matches LanmanServer; explicit SMB port matches remain included. PowerShell AST parsing, four classification cases, and git diff --check passed after the change. No firewall rules were altered.

## Independent trusted-LAN reachability test — Unix — 2026-11-16

The source device is Unix (hndsinh@unix). Both physical paths were tested using only TCP 445 and 139.

| Path | Source | ShinPc destination | Neighbor MAC observed | TCP 445 | TCP 139 |
|---|---|---|---|---|---|
| Wi-Fi | 192.168.1.20/24 (wlan0) | 192.168.1.11/24 (Intel AX200) | 8c:e9:ee:86:0a:29 | Timeout | Timeout |
| Ethernet | 10.10.10.11/24 (eth0) | 10.10.10.10/24 (Realtek 2.5GbE) | 9c:6b:00:5c:b9:1e | Timeout | Timeout |

Both MACs match the corresponding physical adapters on ShinPc. ip route get selected the directly connected interfaces and the expected Unix source IPs. Ping failed on both paths (100% packet loss); ICMP failure alone is not proof of host unreachability because ICMP may be filtered. ARP neighbor resolution succeeded on both paths, confirming the Unix host resolved the destination MAC on each local segment.

**Interpretation:** TCP SMB was not reachable from this Unix client over either tested physical LAN path at the time of testing. Combined with the elevated audit's Public firewall profiles (inbound default Block) and absence of matching SMB Allow rules, the evidence is strongly consistent with Windows Firewall blocking inbound SMB. This is not proof about every client, VLAN, VPN, IPv6 route, interface, or future firewall configuration.

No anonymous or authenticated SMB command was run: the TCP connection did not establish, so an SMB authentication/access test could not proceed. No credentials were entered. No shares were mounted, no directories were listed, and no photo/session files were opened.

## Remaining decision and gate status

- **Verified sub-gate:** no inbound TCP SMB reachability from this Unix client on the tested Wi-Fi and Ethernet paths, under the current Windows configuration.
- **Operator decision still required:** decide whether the deployed booth should retain the Windows Server service and administrative shares (including C$) while relying on the inbound firewall block, or whether the deployment image should disable SMB/admin shares through its normal, separately reviewed hardening process. This audit does not make that change.
- **Scope boundary:** this evidence does not replace a broader deployment firewall/network policy review or native Ubuntu testing. WSL2 local-storage mount/runtime evidence is recorded in the handoff and remains scoped to WSL2.
- Do not add inbound SMB rules to make the test pass. Do not disable Windows Firewall, change network profiles, alter share/NTFS ACLs, or expose LAN photo access as part of this audit.
- Keep Stage 3 LAN photo access and payment deferred until the operator makes the explicit network policy decision and the separate predecessor gates are reviewed.


## Accepted deployment policy — 2026-11-16

The product owner selected **SMB Server and administrative shares disabled by default**, with explicit UAC-authorized opt-in for remote maintenance. The implementation decision is documented in [ADR 0031](../adr/0031-opt-in-windows-smb-administration.md).

A standalone helper prototype, [manage-smb-remote-admin.ps1](../../tools/self-service/manage-smb-remote-admin.ps1), now provides read-only Status and guarded Enable/Disable actions. Enable is constrained to an already Private physical interface and explicit RFC1918 remote IPv4 sources; it opens inbound TCP 445 only on the selected local address/profile. It retains explicit blocks for legacy NetBIOS TCP 137/139 and UDP 137/138. Disable installs app-owned Block rules, removes only the app-owned Allow rule, sets AutoShareWks=0 and stops/disables LanmanServer. Active sessions, running dependents, custom shares, conflicting rules or a Public profile prevent state changes.

**Important:** this helper has only passed parser, IPv4 allowlist checks and a read-only Status run on ShinPc. No state-changing action has been run on ShinPc or a disposable VM. The present ShinPc state remains LanmanServer Running/Automatic, with C$, ADMIN$ and IPC$ present. Policy acceptance does not retroactively harden this development host. Stage 2 remains OPEN until the default-off operation is validated on a disposable Windows image, local PhotoBooth regression passes with LanmanServer off, enable/disable/reboot and scoped network access tests pass, and provisioning/UI integration applies the default to new booth installations.


## 2026-11-16 default-off application on ShinPc and helper follow-up

The operator ran the helper's Disable action on ShinPc. The command initially returned an error after changing state because its post-check treated any TCP 445/139 listener as failure. Subsequent read-only inspection confirmed the intended default-off state:

- LanmanServer: Stopped; startup mode Disabled.
- AutoShareWks: DWORD 0.
- PhotoBooth-managed Allow rule: absent.
- PhotoBooth-managed inbound Block rules: present and verified effective in ActiveStore for TCP 445, TCP 137/139 and UDP 137/138.
- LanmanWorkstation remains Running. Srv2 and NetBT kernel drivers remain loaded.
- TCP 445 and 139 listener endpoints are still reported by Get-NetTCPConnection as OwningProcess 4 (System). This does not establish that LanmanServer is running or that a remote SMB connection can complete. The helper now records these as a warning after verifying service/registry/firewall state.

A second Disable attempt before the correction failed because Get-SmbShare returns ERROR_SERVICE_NOT_ACTIVE when LanmanServer is stopped. The helper is corrected so Status does not invoke Get-SmbShare while the service is stopped, and repeated Disable leaves persistent share definitions untouched instead of requiring live share enumeration. Enable checks persistent share definitions from the registry before starting a stopped Server service and checks SMB1/SMB2 after the service is started.

Read-only tests after the correction passed: PowerShell AST parse, stopped-service repeated-disable preflight, and actual ActiveStore filter verification for all three managed Block rules. The read-only Status action also completed without the previous Get-SmbShare error.

**Important remaining check:** repeat TCP 445/139 probes from Unix to ShinPc over Wi-Fi and Ethernet after this service/firewall state change. Do not infer external reachability solely from PID 4 listeners. No second state-changing Disable/Enable was run during the correction. Local PhotoBooth capture/render/save regression with LanmanServer stopped has not yet been run. Stage 2 remains OPEN pending these checks and disposable-VM validation.


## Post-disable trusted-client check and local workflow regression — 2026-11-16

After the operator applied the default-off state, Unix repeated the same four TCP probes:

| Path | Source → ShinPc | TCP 445 | TCP 139 |
|---|---|---|---|
| Wi-Fi | 192.168.1.20 → 192.168.1.11 | Timeout | Timeout |
| Ethernet | 10.10.10.11 → 10.10.10.10 | Timeout | Timeout |

The Ethernet neighbor entry still resolved to the previously matched Realtek adapter MAC. Earlier Wi-Fi neighbor resolution also matched the Intel AX200 adapter. Ping had 100% loss on Ethernet; ICMP is not used as the SMB acceptance criterion. This confirms that the tested Unix client cannot establish inbound SMB TCP connections over either tested physical path after the Server service was disabled and the PhotoBooth Block rules were installed.

The actual local Wails regression then passed with LanmanServer Stopped/Disabled:
- B525 present and PnP Status=OK.
- No other PhotoBooth/Wails process was running before the smoke.
- ui-camera-preview-smoke.ps1 used a unique PHOTOBOOTH_DATA_DIR under TEMP and the real Wails/Windows Media Foundation path.
- Idle preview before capture: PASS.
- Preview stop / exclusive camera handoff: PASS.
- 3/3 real capture JPEGs: PASS.
- Rendered session image: PASS.
- Local output photo: PASS.
- Return to Idle and preview restart: PASS.
- Smoke exit code 0; JSON report status PASS; temporary data root removal confirmed (temp_data_cleaned=true, Test-Path=False). Only the JSON metrics/report file was retained; no test image/session files were kept.

After the smoke, read-only Status again confirmed LanmanServer Stopped/Disabled, AutoShareWks=0, managed Allow absent, and all three managed Block rules present. PID 4 kernel listener registrations (TCP 445/139) remain visible locally, but the post-disable remote probes timed out from the trusted Unix test host. Do not disable Srv2/NetBT or infer remote availability from listener enumeration alone.

**Stage 2 interpretation:** the Windows default-off state, current Unix two-interface inbound SMB isolation, and local Wails camera→render→local-output regression are now verified on ShinPc. Broader Stage 2 still remains OPEN until the helper is validated on a disposable Windows 11 Pro image (including enable/disable/UAC/reboot and allowlist boundary tests) and the default is integrated into deployment/provisioning and operator UI. Native Ubuntu direct-USB validation and printer acceptance remain separate gates.


## Read-only audit after default-off application (non-elevated) — 2026-11-16

Re-ran the updated audit script against the station profile after the service was disabled. Result: **0 FAIL / 2 WARN / 14 checks**. The stopped-service paths now complete without attempting live share/session enumeration or SMB server CIM queries:
- SMB share overlap: PASS — LanmanServer is Stopped; shares are not being served.
- SMB server protocol availability: PASS — Server endpoint is inactive; opt-in helper checks SMB1/SMB2 before starting.
- SMB server service: PASS — Stopped.
- Current SMB sessions: PASS — no Server sessions can be served.

The two warnings were: (1) PID 4 TCP 445/139 listener registrations remain visible, and (2) the non-elevated audit identity received Access is denied when enumerating the full ActiveStore port-filter inventory. This non-elevated rerun does **not** replace the earlier elevated audit. Run the audit from an elevated PowerShell window to obtain the authoritative effective-firewall inventory. The separately verified PhotoBooth ActiveStore Block filters and Unix post-disable TCP probes are documented above.
