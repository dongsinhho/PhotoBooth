# Stage 2 network-exposure audit — operator runbook

**Purpose:** complete the two outstanding Windows SMB/network checks on ShinPc without changing firewall, shares, services, ACLs, station config, or guest data.

**Safety boundary:** the PowerShell audit is read-only. The second-device tests below target only ShinPc and TCP 445/139. Do not scan the subnet, use admin credentials for the access-boundary test, browse/list session folders, open photos, or change Windows networking policy during this exercise.

## Part A — elevated host-local audit on ShinPc

### 1. Prepare the elevated session

On ShinPc, open **Windows PowerShell → Run as administrator**. Prefer elevating the same signed-in Windows account (`SHINPC\sinhv`) so the OneDrive checkout and station profile remain accessible.

Paste:

```powershell
$repo = 'C:\Users\sinhv\OneDrive\Máy tính\Work\Go\PhotoBooth'
$config = 'C:\Users\sinhv\AppData\Local\PhotoBooth\store-config-v1.json'
$script = Join-Path $repo 'tools\self-service\audit-local-network-exposure.ps1'

$principal = [Security.Principal.WindowsPrincipal]::new(
    [Security.Principal.WindowsIdentity]::GetCurrent()
)
$isAdmin = $principal.IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator
)
"RUNNING_AS=$([Security.Principal.WindowsIdentity]::GetCurrent().Name)"
"IS_ELEVATED=$isAdmin"
"CONFIG_EXISTS=$(Test-Path -LiteralPath $config -PathType Leaf)"
"SCRIPT_EXISTS=$(Test-Path -LiteralPath $script -PathType Leaf)"
if (-not $isAdmin) { throw 'This PowerShell window is not elevated. Stop and reopen it as Administrator.' }
if (-not (Test-Path -LiteralPath $config -PathType Leaf)) { throw "Station profile not found: $config" }
if (-not (Test-Path -LiteralPath $script -PathType Leaf)) { throw "Audit script not found: $script" }

Set-Location -LiteralPath $repo
```

If the UAC credential prompt switched to a different Windows account, keep the explicit config path above; do not silently substitute that account's `%LOCALAPPDATA%`.

### 2. Run the audit

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File $script -ConfigPath $config
"AUDIT_EXIT_CODE=$LASTEXITCODE"
```

The child PowerShell inherits the elevated token. `-ExecutionPolicy Bypass` applies only to this process; it does not change the machine execution policy.

The script reads the station profile, storage-root path/ACL metadata, SMB share/ACL metadata, SMB server configuration, active Windows network categories, effective firewall profiles/rules, TCP listeners and current SMB sessions. It does not read photo/session contents or alter host configuration.

### 3. Interpret the important rows

- `Effective inbound SMB firewall allow rules = PASS`: no enabled inbound Allow rule matching TCP/UDP 445/139/137/138, and no broad Any-port Allow rule explicitly bound to the LanmanServer service, was found for active profiles.
- The same row = `WARN`: Windows denied the effective port-filter inventory. Record it as unresolved, not closed.
- The same row = `FAIL`: matching inbound Allow rules were found. Record each rule's profile, protocol, ports, service and remote addresses; do not disable it during this audit.
- `Current SMB sessions = PASS`: no active SMB client sessions were returned at that instant.
- `Current SMB sessions = WARN`: sessions exist or could not be enumerated. A session alone does not prove PhotoBooth files were opened; record client names and identity only.
- `SMB administrative-share overlap = WARN`: expected while C$ covers a storage root. Record the share ACL exactly, especially `INTERACTIVE`, but do not change it.
- `SMB TCP listeners = WARN`: a local listener is not proof that a remote machine can reach it; Part B tests this.
- A summary of `failed=0` is not sufficient if warnings remain. Stage 2 is not closed until the unresolved evidence is reviewed.

Keep the complete audit output, including the summary and the rows above. Do not post credentials or passwords.

## Part B — probe ShinPc from one separate trusted LAN device

### 1. Select the correct ShinPc address

On ShinPc, in PowerShell (elevated or normal), run:

```powershell
Get-NetConnectionProfile |
    Format-Table InterfaceAlias, NetworkCategory, IPv4Connectivity, IPv6Connectivity -AutoSize

Get-NetAdapter -Physical |
    Where-Object Status -eq 'Up' |
    ForEach-Object {
        $adapter = $_
        $cfg = Get-NetIPConfiguration -InterfaceIndex $adapter.ifIndex
        [pscustomobject]@{
            Interface = $adapter.Name
            Description = $adapter.InterfaceDescription
            IPv4 = ($cfg.IPv4Address.IPAddress -join ',')
            PrefixLength = ($cfg.IPv4Address.PrefixLength -join ',')
            Gateway = ($cfg.IPv4DefaultGateway.NextHop -join ',')
        }
    } | Format-Table -AutoSize
```

Choose the IPv4 address of the physical Wi-Fi/Ethernet interface that the second device will use on the same trusted LAN/VLAN. Do not choose a VirtualBox, WSL, Hyper-V, VPN, loopback or other virtual-adapter address. Record the interface name, network category, IPv4 address and prefix length.

The second device must be a device you control or are authorized to test, connected to that same trusted LAN. Do not run these checks from a public Wi-Fi, guest network, office network without authorization, or across the internet. Do not turn off either device's firewall.

### 2. Test only TCP 445 and 139

On a **Windows** second device, open PowerShell and replace the example address:

```powershell
$target = '192.168.1.11'   # replace with ShinPc's physical-LAN IPv4 address
$results = foreach ($port in 445,139) {
    $result = Test-NetConnection -ComputerName $target -Port $port -WarningAction SilentlyContinue
    [pscustomobject]@{
        Target = $result.RemoteAddress
        Port = $result.RemotePort
        SourceAddress = $result.SourceAddress
        TcpTestSucceeded = $result.TcpTestSucceeded
    }
}
$results | Format-Table -AutoSize
```

On a **Linux** second device, use the following if `nc` is already installed:

```bash
TARGET=192.168.1.11  # replace with ShinPc's physical-LAN IPv4 address
nc -vz -w 3 "$TARGET" 445
nc -vz -w 3 "$TARGET" 139
```

Do not install tools or scan other ports just for this test.

Interpretation:
- `TcpTestSucceeded=True` / `succeeded`: a TCP handshake reached a listener from this client/network path. It does **not** prove SMB authentication or file access.
- `False` / `Connection refused`: the target was reachable but the port did not accept the connection at that time.
- Timeout / no route: the path may be filtered, isolated, or routing may be wrong. Reconfirm both devices are on the same intended LAN before drawing a conclusion.
- Test both ports separately and report the source address shown on the second device.

### 3. Optional anonymous SMB check (Linux client only)

If `smbclient` is already installed, this makes a no-password SMB attempt and does not list folder contents:

```bash
TARGET=192.168.1.11  # replace with ShinPc's physical-LAN IPv4 address
smbclient -L "//$TARGET" -N -m SMB3
smbclient "//$TARGET/C$" -N -m SMB3 -c 'cd Users/sinhv/AppData/Local/PhotoBooth; pwd'
```

The second command only attempts to enter the known PhotoBooth profile directory and print its path; it does not run `ls`, download, open or modify files. If the real storage roots are elsewhere, use their exact relative path from the elevated audit instead.

An access-denied result means this anonymous/guest attempt could not enter C$ through this path. It does not prove that every authenticated identity is denied.

### 4. Optional least-privilege authenticated path check

Do this only if ShinPc already has a **standard, non-administrator account** that is also not in **Backup Operators**. Do not create an account or change group membership for this exercise. Never use `SHINPC\sinhv`, another administrator, or a Backup Operators credential for this test.

On a Windows second device:
1. Confirm there is no existing SMB connection to ShinPc:
   ```powershell
   Get-SmbConnection | Format-Table ServerName, ShareName, UserName -AutoSize
   ```
   If any existing SMB connection is shown, stop this optional test; do not disconnect it or replace its credentials while other work may depend on it. This conservative check avoids missing a connection made by hostname when the next step uses an IP address.
2. Use the existing standard account on ShinPc. Replace the account name and target address:
   ```powershell
   $target = '192.168.1.11'  # replace
   $credential = Get-Credential -UserName 'SHINPC\<standard-user>'
   New-PSDrive -Name PBProbe -PSProvider FileSystem -Root "\\$target\C$" -Credential $credential -ErrorAction Stop | Out-Null
   try {
       # Use the exact root paths printed by the audit. These are the expected
       # defaults; adjust only if the audit printed different storage roots.
       foreach ($relative in @(
           'Users\sinhv\AppData\Local\PhotoBooth\sessions',
           'Users\sinhv\AppData\Local\PhotoBooth\photos'
       )) {
           try {
               Get-Item -LiteralPath ("PBProbe:\" + $relative) -ErrorAction Stop |
                   Select-Object FullName, Attributes
               "PATH_ACCESS=ALLOWED ($relative)"
           } catch {
               "PATH_ACCESS=DENIED_OR_UNAVAILABLE ($relative): $($_.Exception.Message)"
           }
       }
   } finally {
       Remove-PSDrive -Name PBProbe -Force -ErrorAction SilentlyContinue
       $credential = $null
   }
   ```
   This checks metadata for the two directory paths only; it does not enumerate their contents. Use only an existing least-privilege target account and the exact paths shown by the audit. If PowerShell reports an existing SMB-credential conflict, stop and report it; do not run `net use * /delete`.

If no suitable standard account exists, skip this authenticated check and report that fact. The anonymous check plus TCP reachability are still useful evidence, but the authenticated access-boundary gate remains incomplete until a least-privilege test identity is available and explicitly approved.

## What to send back

Send the following (screenshots are optional; pasted text is easier to inspect):

1. **Part A:** the `IS_ELEVATED` line, complete `SUMMARY`, and rows for `SMB administrative-share overlap`, `Effective inbound SMB firewall allow rules`, `Current SMB sessions`, `Firewall <profile>` and `SMB TCP listeners`.
2. **ShinPc interface:** physical interface name, network category, IPv4 address and prefix length. You may mask the last IPv4 octet in a public write-up, but retain the full address in this private troubleshooting conversation if needed to correlate the probe.
3. **Part B:** the two per-port results, including the second device's source address and the target address.
4. **SMB access check:** anonymous/guest result, or least-privilege result and whether the test identity is standard/non-admin. Never send passwords, tokens, photos or directory listings.

## Stop conditions

Stop without changing anything if:
- The PowerShell session is not elevated or the station config path cannot be found.
- The second device is not on the intended trusted LAN/VLAN.
- An SMB connection to ShinPc already exists on the client before the authenticated test.
- The only available credentials are Administrator/Backup Operators.
- A command proposes changing firewall rules, SMB settings, shares, ACLs, accounts or services.

**No commit or host hardening change is part of this runbook.** The outcome is evidence for a later explicit policy decision, not authorization to disable SMB or alter the firewall.
