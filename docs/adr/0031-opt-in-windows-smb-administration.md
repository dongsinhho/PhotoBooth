# ADR 0031: Disable Windows SMB administration by default

- **Status:** Accepted policy; integrated in the Windows Preview operator Settings panel and first-install release provisioning. The Enable/Disable cycle was accepted on the tested ShinPc/Unix path; broader Windows VM/network qualification is not claimed.
- **Date:** 2026-11-16
- **Decision owner:** Product owner

## Context

The 2026-11-16 Stage 2 audit found that the station's local media/payment policy and storage ACLs pass, the effective inbound firewall has no matching SMB Allow rule, and an independent Unix host could not reach TCP 445/139 over either physical ShinPc LAN interface. However, the Windows Server service was running, C$ covered both PhotoBooth storage roots, and SMB listeners remained present.

The product's normal booth workflow is local-only. It has no requirement to publish its storage directories over SMB. SMB administrative shares are therefore a remote-maintenance capability, not a guest-media-delivery feature.

The Windows Server service (LanmanServer) supports file, print and named-pipe sharing. Disabling it can affect remote management and applications that depend on those functions; Microsoft recommends checking whether applications/processes need it before disabling it. See [Microsoft: Secure SMB Traffic](https://learn.microsoft.com/en-us/windows-server/storage/file-server/smb-secure-traffic).

## Decision

### 1. Default booth state: SMB Server off

For a Windows workstation booth image:

1. Set LanmanServer to **Stopped** with startup type **Disabled**. Do not disable the SMB client service (LanmanWorkstation) and do not add outbound SMB blocks as part of this decision.
2. Set HKLM\SYSTEM\CurrentControlSet\Services\LanmanServer\Parameters\AutoShareWks to DWORD 0, preventing Windows workstation administrative shares such as drive-root shares from being automatically recreated if the Server service is later started manually.
3. Keep SMB1 disabled.
4. Do not create or enable inbound SMB firewall Allow rules. No router port-forwarding or UPnP mapping is permitted.
5. Keep the application media-delivery mode local and payment mode disabled. This operating-system setting is independent of StoreConfig; it is not a guest LAN delivery mode.

The two controls are intentional defense in depth. Disabling automatic administrative shares alone does not remove IPC$ or manually created shares; stopping/disabling LanmanServer prevents SMB Server shares from being served while the service is off. For workstation editions, the AutoShareWks DWORD is set to 0; Windows Server uses AutoShareServer. AutoShareWks=0 does not remove IPC$ or manually created shares, so the stopped Server service remains the primary default-off control. See [Microsoft: Remove administrative shares](https://learn.microsoft.com/en-us/troubleshoot/windows-server/networking/remove-administrative-shares) for the IPC$/manual-share limitation and [Google Cloud Threat Intelligence: 2026 ransomware hardening guidance](https://cloud.google.com/blog/topics/threat-intelligence/preparation-hardening-destructive-attacks) for the workstation AutoShareWks setting.

Before applying the default to an existing booth, preflight active SMB sessions, non-administrative shares, service dependents and known remote-management/backup/printer workflows. Abort rather than force-stop if the service is in use or a custom share is present. Validate the policy on a clean deployment image before broad rollout.

### 2. Explicit opt-in remote administration

SMB administration is enabled only after a local operator explicitly chooses **Enable remote administration**.

1. The app invokes a dedicated, package-owned PowerShell helper with **UAC elevation**. No silent privilege escalation, background elevation, or app process directly edits machine-wide security settings.
2. The helper displays the target interface, current Windows network profile, local address, permitted remote source addresses and exact changes before applying them. Both Enable and Disable operations require an elevated token.
3. Enable is refused unless the selected network is classified **Private** by Windows and the operator supplies one or more explicit trusted remote IP addresses. The helper must not change a Public network to Private automatically. Any, LocalSubnet, wildcard and internet-wide source scopes are not accepted.
4. Enable restores automatic administrative-share creation for the workstation, sets LanmanServer startup to **Automatic**, starts the service, and creates one PhotoBooth-owned inbound firewall Allow rule:
   - Direction: Inbound
   - Protocol/port: TCP 445 only
   - Profile: Private only
   - Local address: selected booth interface address
   - Remote address: explicit operator-supplied IP allowlist
   - Edge traversal: Block
   - Keep the PhotoBooth-owned TCP 445 Block rule enabled on Public and Domain profiles; do not remove the default block globally while opt-in is active.
5. Do not allow TCP 139 or UDP/TCP 137/138. The helper keeps explicit inbound Block rules for legacy NetBIOS ports on all profiles, including while opt-in SMB is enabled. SMB 2 or later uses direct-hosted TCP 445 and does not require NetBIOS ports 137–139. See [Microsoft: Secure SMB Traffic, updated firewall rules](https://learn.microsoft.com/en-us/windows-server/storage/file-server/smb-secure-traffic).
6. Do not enable SMB1, guest/anonymous SMB, weaken UAC remote restrictions, change share/NTFS ACLs, create new guest shares, enable router port forwarding, or enable access on Public networks.
7. If preflight discovers custom non-administrative shares, active SMB sessions, conflicting broad inbound rules, or service dependencies that have not been approved, Enable must stop and explain the blocker. It must not expose those shares as a side effect.
8. The opt-in state persists across restart until the operator chooses Disable. The UI must display that remote administration is enabled and show the configured source allowlist.
9. Disable establishes PhotoBooth-owned inbound Block rules for TCP 445 on all profiles and legacy NetBIOS ports, removes only the PhotoBooth Allow rule, stops LanmanServer, sets its startup type to Disabled, and sets AutoShareWks=0. While enabled, the TCP 445 Block rule remains active on Public and Domain, and only the selected Private local address is allowlisted. It must not delete or rewrite unrelated firewall rules or manually configured shares. If active sessions, dependent services or custom shares are detected, the helper aborts before changing state and reports the blocker; it does not force-stop the service.
10. Provide a read-only Status mode. Treat the actual Windows service, registry and firewall state as authoritative; do not infer state from a UI preference or StoreConfig field.

### 3. Scope and availability

- This is for Windows workstation booth deployments, not domain controllers or Windows file servers.
- The current ShinPc networks are both classified Public. The opt-in helper must refuse to create an effective SMB allowance until the operator deliberately changes the trusted network's profile using Windows network settings.
- The audit's C$/INTERACTIVE warning is retained as historical share-ACL evidence. On 2026-11-16 the operator explicitly ran the default-off Disable action on ShinPc; LanmanServer is now Stopped/Disabled, so SMB shares are not being served. Share definitions and ACLs were not deleted or rewritten.
- SMB remote administration is strictly for maintenance by an authorized administrator. It is never a guest-photo transport or substitute for the future session-scoped LAN delivery design.

## Consequences

- A default booth no longer serves SMB shares or administrative shares, reducing lateral-access surface when remote file/print/named-pipe sharing is not required.
- Local capture, render and local output must work with LanmanServer disabled. This is an explicit regression gate.
- Windows Remote Administration, remote backup, remote printer/file sharing and other software relying on LanmanServer may stop working; those dependencies must be inventoried and tested before adoption.
- Operators who deliberately enable remote administration receive a narrow, auditable firewall exception for TCP 445 from explicitly trusted source IPs on a Private network only. The Windows SMB authentication and share permissions still apply; opening the port is not authentication or authorization.
- The setting is machine-wide and must be handled by a privileged helper, not by the guest-facing workflow or ordinary station configuration.

## Implementation and validation status

A standalone helper now exists at tools/self-service/manage-smb-remote-admin.ps1:

- Status is read-only and was run on ShinPc; it accurately reported the current LanmanServer state, C$/ADMIN$/IPC$, and local listeners.
- Enable/Disable both request UAC elevation when launched from a non-elevated process. Enable is limited to one physical adapter with a Windows Private profile, one local IPv4, and explicit RFC1918 remote IPv4 source addresses. It adds one inbound TCP 445 Allow rule, starts LanmanServer, and verifies C$/ADMIN$ exist. It keeps NetBIOS 137–139 blocked.
- Disable adds managed Block rules for TCP 445 and NetBIOS TCP/UDP ports, removes only the PhotoBooth Allow rule, sets AutoShareWks=0, and stops/disables LanmanServer. It refuses active SMB sessions, running dependents and custom shares while the Server service is running. If already stopped, it does not query Get-SmbShare (which fails when LanmanServer is stopped); it leaves any persistent definitions untouched.
- The helper passed PowerShell AST parsing, RFC1918 allowlist positive/negative checks, and read-only Status checks. On 2026-11-16 the operator ran Disable on ShinPc. It stopped LanmanServer, set startup Disabled and AutoShareWks=0, removed the helper Allow rule, and created the helper Block rules. The first run returned a false failure because kernel listener registrations remained; read-only follow-up verified the service/registry state and all three ActiveStore Block rules. The helper was subsequently corrected to treat PID 4 kernel listener registrations as a warning after checking service/registry state and effective firewall Block filters, and to avoid Get-SmbShare while the Server service is stopped. The Srv2 and NetBT system drivers remain loaded and Get-NetTCPConnection reports TCP 445/139 endpoints owned by PID 4 (System); listener presence is not treated as proof of an active share server. No further state-changing action was run during the correction.

Remaining implementation and validation:

1. Test Enable/Disable and rollback on a disposable Windows 11 Pro image, including UAC, rule ownership, firewall ActiveStore behavior, service state, AutoShareWks, shares and reboot persistence. Record kernel listener registrations (which may remain under PID 4 after LanmanServer stops) but do not use listener absence as the service-stopped criterion; verify no remote TCP reachability from a second device instead.
2. Verify an allowlisted source can reach TCP 445 only on the selected Private interface, while an unlisted source and Public interfaces cannot; verify 137–139 remain blocked.
3. Run local PhotoBooth camera/capture/render/save regression with LanmanServer stopped and disabled. **Passed on ShinPc 2026-11-16:** real B525 Wails smoke verified preview before capture, exclusive handoff, 3/3 captures, render/local save, and preview resume; exit 0 and disposable test data cleaned. On the same host, LanmanServer is Stopped/Disabled, AutoShareWks=0, the PhotoBooth Allow rule is absent, and PhotoBooth-owned ActiveStore Block rules for TCP 445 and NetBIOS TCP/UDP ports were verified. A post-disable Unix client test timed out on TCP 445/139 over both Wi-Fi and Ethernet. PID 4 listener registrations remain as a local observation, not a remote reachability result.
4. Integrate the helper into operator UI/deployment tooling. Provisioning must run Disable to establish default-off state; merely shipping the helper does not change a Windows installation.
5. Review the existing C$/INTERACTIVE share-ACL behavior on the disposable target; do not rewrite built-in share or NTFS ACLs automatically.

Until the default is applied by deployment and these checks pass, the decision is accepted but Stage 2 remains open.

## References

- [Microsoft Learn — Secure SMB Traffic in Windows Server (applies to Windows 11)](https://learn.microsoft.com/en-us/windows-server/storage/file-server/smb-secure-traffic)
- [Microsoft Learn — Remove administrative shares](https://learn.microsoft.com/en-us/troubleshoot/windows-server/networking/remove-administrative-shares)
- [Google Cloud Threat Intelligence — Proactive Preparation and Hardening Against Destructive Attacks: 2026 Edition](https://cloud.google.com/blog/topics/threat-intelligence/preparation-hardening-destructive-attacks)


## Implementation integration and acceptance update — 2026-11-17

The original implementation gates above are updated by this dated record.

### Operator UI

The Windows Preview **Settings & frame → Remote administration** panel is now connected to the helper:

- It reads the actual LanmanServer state/start mode, AutoShareWks value, PhotoBooth-owned effective SMB/NetBIOS Block rules, Allow-rule scope, and listener count. It does not store or infer machine security state through StoreConfig.
- It lists active physical network adapters and their IPv4 addresses, including the Windows network profile.
- Enable requires a selected adapter whose current profile is already Private, one local IPv4 address, and explicit RFC1918 remote IPv4 addresses. The UI explains the machine-wide scope and asks for confirmation.
- Enable and Disable delegate all machine-wide changes to the PowerShell helper through UAC. The Go app does not directly edit Windows services, registry values, shares or firewall rules.
- Status is read again after an action; a helper error or failed refresh is presented as unverified rather than shown as success.
- Non-Windows builds expose an explicit Windows-only unavailable result.

### Fresh-install provisioning and release packaging

- New Windows Preview ZIPs contain four root-level files: PhotoBooth.exe, the helper, release-manifest.json and README.txt.
- The helper file size and SHA-256 are recorded in the manifest and checked by the installer before a release is activated.
- A first-time install invokes helper Disable through UAC before switching active.json. It fails closed when the helper is absent or when the helper refuses to change state due to active SMB sessions, custom shares, service dependencies or another blocker.
- An upgrade with an existing active pointer preserves the operator's existing SMB maintenance choice. It does not automatically disable a deliberately enabled configuration.
- `-SkipMachineProvisioning` exists for synthetic/isolated tests only; it is not for a real first booth installation.
- Source-checkout Preview builds find the helper under tools/self-service; installed releases find the colocated copy beside PhotoBooth.exe.

### Verification performed

- The operator's real Enable → authenticated allowlisted SMB share-list → Disable cycle on ShinPc passed as recorded in HANDOFF_2026-11-17.md. The final host state remains default-off with Wi-Fi Public.
- PowerShell helper AST parse and JSON read-only Status/ListInterfaces commands passed on ShinPc.
- Wails Go package `go test ./...` and `go vet ./...` passed, including RFC1918 validation, allowlist parsing/deduplication, status/interface JSON parsing, and helper command routing.
- Wails bindings regenerated, frontend production bundle built, and the Windows Preview executable built successfully.
- The extended Settings UI Automation smoke passed: status reported LanmanServer Stopped/Disabled, AutoShareWks=0 and the managed TCP 445 Block; adapter/IP/allowlist controls and Enable/Disable/Refresh actions were present. The test did not press a state-changing SMB button, save station settings or create a photo session.
- The synthetic release lifecycle smoke passed, including rejection of a helper-less legacy package for a fresh install, first-install test bypass, upgrade without reapplying machine provisioning, rollback, integrity rejection and cleanup.
- A real Windows Preview ZIP build/isolated installation smoke verified all four ZIP entries, colocated helper, manifest hash and the installed helper's JSON Status output. It used the explicit test-only provisioning bypass and did not mutate host SMB state.

### Accepted scope and limitations

The product owner accepted the tested ShinPc Enable/Disable scope as sufficient to proceed with this integration. The live Enable test used only the allowlisted Unix source. A separate unlisted source, reboot persistence, IPv6, other network profiles, and disposable-VM behavior were not tested in this continuation. Do not claim those as verified or silently expand the allowlist. The helper still fails closed on the checks it implements; additional platform qualification can be reopened if a target deployment requires it.

Stage 2 is not a blanket Windows security certification. This ADR's SMB default-off policy, helper, Windows Preview operator panel and first-install provisioning path are implemented for the current internal Preview flow; signed production packaging and broader OS/hardware/deployment qualification remain separate product gates.


## Product-owner scope decision — 2026-11-18

The product owner considers the current Remote administration feature sufficient and is pausing further feature qualification here.

### Intended use case

Remote administration is an **operator / maintainer convenience for authorized remote maintenance of a Windows Booth**, not a guest-facing feature and not the PhotoBooth photo-delivery mechanism. When explicitly enabled, a trusted maintenance computer can use authenticated Windows SMB administrative shares (for example `C$`, `ADMIN$`, and `IPC$`) to perform permitted maintenance tasks such as retrieving diagnostic logs, inspecting or updating application/configuration files, or using compatible Windows administration tools. Actual access remains subject to Windows credentials, share permissions and NTFS permissions.

The feature does not create a guest share, publish session photos, grant anonymous access, or replace a future session-scoped LAN delivery design. It is optional; if no remote maintenance is needed, leave it disabled.

### Accepted validation scope

- Happy path only: Enable on the selected already-Private physical adapter with explicit trusted RFC1918 source address(es); authenticate from the allowlisted maintenance device and confirm SMB administrative shares are listed; Disable and confirm default-off state is restored.
- The operator tested this lifecycle on ShinPc with the Unix maintenance host and accepted it as sufficient for the current product scope.
- Do not require additional non-allowlisted-source, reboot-persistence, IPv6, failure/rollback, NetBIOS UDP, or disposable-VM qualification to consider this feature complete for the current Preview scope. These are unverified, not passed; do not make claims that they were tested.
- Keep SMB Server default-off on fresh installs. Do not automatically enable it, broaden the allowlist, or change network profiles.
- Reopen additional qualification only if a concrete deployment requirement or reproducible issue makes it necessary.
