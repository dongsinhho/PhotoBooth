# Local network exposure audit

**Status:** Read-only Windows host audit implemented and run on ShinPc on 2026-11-08. Result: 0 FAIL / 3 WARN. Stage 2 exit gate remains OPEN.

## Purpose

Stage 2 requires evidence that local media delivery does not expose PhotoBooth photos on a network accidentally. The delivery policy remains local and payment remains disabled; this audit does not add LAN delivery, a guest download endpoint, QR, or payment processing.

Run from an elevated or ordinary PowerShell session at the repository root:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\audit-local-network-exposure.ps1
~~~

To audit a specific station profile:

~~~powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\self-service\audit-local-network-exposure.ps1 -ConfigPath "$env:LOCALAPPDATA\PhotoBooth\store-config-v1.json"
~~~

The script is read-only. It does not open photo/session contents, create files, alter shares, change firewall rules, stop services, or probe another machine. It checks:

- Store policy values and that the session/output roots are on a fixed local drive and do not overlap.
- NTFS allow entries for broad principals (Everyone, Authenticated Users, BUILTIN\\Users, and Domain Users).
- SMB shares whose paths contain or are contained by either storage root, including share ACL summaries.
- SMB1/SMB2 configuration, the Server service, local TCP listeners on 445/139, active Windows network categories and effective firewall profile defaults.
- The effective firewall port-filter inventory for inbound Allow rules on TCP/UDP 445/139/137/138, plus broad Any-port rules associated with LanmanServer or Any service.
- Current SMB sessions when the caller has sufficient rights.

A FAIL produces exit code 1. WARN is deliberately non-fatal but means the gate is not proven. This is a host-local configuration review, not a remote penetration test. Reading the effective firewall port-filter inventory and SMB sessions may require an elevated PowerShell session; when Windows denies either query, the script reports WARN instead of treating the missing evidence as PASS.

## ShinPc result — 2026-11-08

The profile at %LOCALAPPDATA%\PhotoBooth\store-config-v1.json selected media_delivery.mode=local and payment.mode=disabled. The configured sessions and photos directories both existed on fixed local drive C: and were distinct.

The audit found no broad NTFS allow ACE for Everyone, Authenticated Users, BUILTIN\Users, or Domain Users. The returned directory ACL entries were ShinPc\CodexSandboxUsers:ReadAndExecute, NT AUTHORITY\SYSTEM:FullControl, BUILTIN\Administrators:FullControl, and ShinPc\sinhv:FullControl.

The only disk shares reported were ADMIN$, C$, and IPC$. C$ maps to C:\, so it contains both PhotoBooth storage roots. Its reported share ACL was:

- BUILTIN\Administrators: Full
- BUILTIN\Backup Operators: Full
- NT AUTHORITY\INTERACTIVE: Full

This is a Windows administrative share, not a separately created guest-facing PhotoBooth share. However, the storage trees are within its namespace, so privileged SMB access may reach them. The meaning/effective remote applicability of the INTERACTIVE ACE was not independently proven in this audit and should be reviewed with an elevated account.

SMB1 was disabled; SMB2 was enabled; LanmanServer was running. TCP listeners were observed on :::445 and NetBIOS port 139 on 10.10.10.10, 192.168.1.11, 192.168.226.1, and 192.168.74.1. Wi-Fi and Ethernet were both categorized Public. The effective Public firewall profile was enabled with default inbound action Block. An initial name-scoped query did not find enabled, named File and Printer Sharing/SMB/NetBIOS allow rules, but the follow-up comprehensive port-filter inventory query was denied by Windows and is therefore unverified.

Get-SmbSession returned Access Denied under the current identity, so active SMB client sessions could not be enumerated. No PhotoBooth process was running during this audit. No PhotoBooth session or photo bytes were opened.

**Initial run result:** 0 FAIL / 3 WARN.

**Follow-up run after the comprehensive firewall inventory change:** 0 FAIL / 4 WARN. The caller is `SHINPC\\sinhv` and is not elevated. `Get-NetFirewallPortFilter -PolicyStore ActiveStore` returned Access Denied, so the effective inbound port-filter inventory cannot be reviewed. `Get-SmbSession` also returned Access Denied. The updated script reports both as WARN instead of PASS. `C$` still overlaps both storage roots and SMB listeners remain active. Only ShinPc is registered online as a Desktop Commander device, so no independent second-device reachability check was possible.

The current evidence is consistent with local-only operation protected by the active Public firewall default and NTFS ACLs, and no ordinary SMB share directly exposes either storage root. It does not prove the storage roots are unreachable from every remote host.

## Remaining Stage 2 acceptance

1. Repeat the audit from an elevated PowerShell session on the intended deployed booth profile. Review the C$ share ACL and current SMB sessions, including the INTERACTIVE entry.
2. From a separate trusted LAN device, verify whether ports 445/139 are reachable on the booth's active interface and whether any SMB authentication path can access the PhotoBooth paths. Do not use guest-photo data for this check.
3. Decide explicitly whether the booth image should retain Windows administrative shares/SMB service or disable them through the operator's normal Windows hardening process. This tool does not make that change.
4. Run Linux storage/mount integration tests on the selected Ubuntu target. The Linux compile-only result is not runtime evidence.

Do not advance to Stage 3 LAN photo access or payment until the Stage 2 network-exposure and Linux runtime/mount gates are separately reviewed.