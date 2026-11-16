[CmdletBinding()]
param(
    [string]$ConfigPath = (Join-Path $env:LOCALAPPDATA 'PhotoBooth\store-config-v1.json')
)

$ErrorActionPreference = 'Stop'
$checks = [System.Collections.Generic.List[object]]::new()
$script:failed = 0
$script:warnings = 0

function Add-Check {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][ValidateSet('PASS','WARN','FAIL')][string]$Status,
        [Parameter(Mandatory)][string]$Details
    )
    $checks.Add([pscustomobject]@{ Check = $Name; Status = $Status; Details = $Details })
    switch ($Status) {
        'FAIL' { $script:failed++ }
        'WARN' { $script:warnings++ }
    }
}

function Resolve-LocalPath {
    param([Parameter(Mandatory)][string]$Value, [Parameter(Mandatory)][string]$Base)
    if ([string]::IsNullOrWhiteSpace($Value)) { throw 'Storage path is empty' }
    if ($Value.StartsWith('\\')) { throw "UNC/network path is not local: $Value" }
    if ([IO.Path]::IsPathRooted($Value)) {
        return [IO.Path]::GetFullPath($Value)
    }
    return [IO.Path]::GetFullPath((Join-Path $Base $Value))
}

function Normalize-DirectoryPath {
    param([Parameter(Mandatory)][string]$Path)
    $full = [IO.Path]::GetFullPath($Path)
    $root = [IO.Path]::GetPathRoot($full)
    if ($full.Equals($root, [StringComparison]::OrdinalIgnoreCase)) { return $root }
    return $full.TrimEnd([char[]]@('\','/'))
}

function Test-PathOverlap {
    param([Parameter(Mandatory)][string]$First, [Parameter(Mandatory)][string]$Second)
    $a = Normalize-DirectoryPath $First
    $b = Normalize-DirectoryPath $Second
    if ($a.Equals($b, [StringComparison]::OrdinalIgnoreCase)) { return $true }
    $aPrefix = $a
    $bPrefix = $b
    if (!$aPrefix.EndsWith('\')) { $aPrefix += '\' }
    if (!$bPrefix.EndsWith('\')) { $bPrefix += '\' }
    return $a.StartsWith($bPrefix, [StringComparison]::OrdinalIgnoreCase) -or
        $b.StartsWith($aPrefix, [StringComparison]::OrdinalIgnoreCase)
}

function Get-ResolvedShareAccess {
    param([Parameter(Mandatory)][string]$ShareName)
    try {
        $entries = @(Get-SmbShareAccess -Name $ShareName -ErrorAction Stop)
        if ($entries.Count -eq 0) { return 'No share ACL entries returned' }
        return (($entries | ForEach-Object {
            '{0} {1} {2}' -f $_.AccessControlType, $_.AccessRight, $_.AccountName
        }) -join '; ')
    } catch {
        return "Share ACL unavailable: $($_.Exception.Message)"
    }
}

Write-Host 'PhotoBooth local-network exposure audit (read-only)' -ForegroundColor Cyan
Write-Host 'No firewall, share, service, profile, or photo/session data will be changed or opened.'
Write-Host ''

try {
    $configFullPath = [IO.Path]::GetFullPath($ConfigPath)
    if (!(Test-Path -LiteralPath $configFullPath -PathType Leaf)) {
        throw "Station config not found: $configFullPath"
    }

    $config = Get-Content -LiteralPath $configFullPath -Raw | ConvertFrom-Json -ErrorAction Stop
    if ([int]$config.schema_version -ne 1) { throw 'Only station config schema_version 1 is supported' }
    if ($config.media_delivery.mode -ne 'local') {
        Add-Check 'Media-delivery policy' 'FAIL' "Configured mode is '$($config.media_delivery.mode)'; this audit expects local"
    } else {
        Add-Check 'Media-delivery policy' 'PASS' 'Configured mode is local'
    }
    if ($config.payment.mode -ne 'disabled') {
        Add-Check 'Payment policy' 'FAIL' "Configured mode is '$($config.payment.mode)'; this audit expects disabled/free"
    } else {
        Add-Check 'Payment policy' 'PASS' 'Configured mode is disabled/free'
    }

    $configBase = Split-Path -Parent $configFullPath
    $storageRoots = @(
        [pscustomobject]@{ Name='Session storage'; Path=(Resolve-LocalPath ([string]$config.storage.sessions_dir) $configBase) },
        [pscustomobject]@{ Name='Photo output'; Path=(Resolve-LocalPath ([string]$config.storage.output_dir) $configBase) }
    )

    $normalizedRoots = @()
    foreach ($entry in $storageRoots) {
        $path = Normalize-DirectoryPath $entry.Path
        $normalizedRoots += [pscustomobject]@{ Name=$entry.Name; Path=$path }
        if (Test-Path -LiteralPath $path -PathType Container) {
            $driveRoot = [IO.Path]::GetPathRoot($path)
            $logicalDrive = Get-CimInstance Win32_LogicalDisk -Filter ("DeviceID='" + $driveRoot.TrimEnd('\') + "'") -ErrorAction Stop
            if ($null -eq $logicalDrive -or $logicalDrive.DriveType -ne 3) {
                Add-Check $entry.Name 'FAIL' "Storage is not on a fixed local drive: $path"
            } else {
                Add-Check $entry.Name 'PASS' ("Existing directory on fixed local drive {0}: {1}" -f $logicalDrive.DeviceID, $path)
            }

            try {
                $acl = Get-Acl -LiteralPath $path -ErrorAction Stop
                $broad = @($acl.Access | Where-Object {
                    $_.AccessControlType -eq 'Allow' -and
                    $_.IdentityReference.Value -in @('Everyone','NT AUTHORITY\Authenticated Users','Authenticated Users','BUILTIN\Users','Users','Domain Users','S-1-1-0','S-1-5-11','S-1-5-32-545') -and
                    (($_.FileSystemRights.ToString() -match 'Read|Write|Modify|FullControl|CreateFiles|CreateDirectories'))
                })
                if ($broad.Count -gt 0) {
                    $summary = ($broad | ForEach-Object { "$($_.IdentityReference):$($_.FileSystemRights)" }) -join '; '
                    Add-Check "$($entry.Name) NTFS ACL" 'FAIL' "Broad principal has allow access: $summary"
                } else {
                    $principals = ($acl.Access | ForEach-Object {
                        "$($_.IdentityReference):$($_.FileSystemRights)"
                    }) -join '; '
                    Add-Check "$($entry.Name) NTFS ACL" 'PASS' "No Everyone/Authenticated Users/BUILTIN Users/Domain Users allow ACE found. Entries: $principals"
                }
            } catch {
                Add-Check "$($entry.Name) NTFS ACL" 'WARN' "Could not inspect ACL: $($_.Exception.Message)"
            }
        } else {
            Add-Check $entry.Name 'WARN' "Directory does not exist yet; no ACL or share target can be verified: $path"
        }
    }

    if (Test-PathOverlap $normalizedRoots[0].Path $normalizedRoots[1].Path) {
        Add-Check 'Storage-root separation' 'FAIL' 'Session and photo-output roots overlap'
    } else {
        Add-Check 'Storage-root separation' 'PASS' 'Session and photo-output roots are distinct and non-overlapping'
    }

    $serverServiceForShares = Get-Service -Name LanmanServer -ErrorAction Stop
    if ($serverServiceForShares.Status -eq 'Running') {
        try {
            $shares = @(Get-SmbShare -ErrorAction Stop | Where-Object { ![string]::IsNullOrWhiteSpace([string]$_.Path) })
            $overlappingShares = [System.Collections.Generic.List[object]]::new()
            foreach ($share in $shares) {
                try { $sharePath = Normalize-DirectoryPath ([string]$share.Path) } catch { continue }
                foreach ($root in $normalizedRoots) {
                    if (Test-PathOverlap $sharePath $root.Path) {
                        $access = Get-ResolvedShareAccess ([string]$share.Name)
                        $overlappingShares.Add([pscustomobject]@{
                            Name = [string]$share.Name
                            Path = $sharePath
                            Special = [bool]$share.Special
                            Root = $root.Name
                            Access = $access
                        })
                    }
                }
            }

            if ($overlappingShares.Count -eq 0) {
                Add-Check 'SMB share overlap' 'PASS' 'No configured SMB share path contains or is contained by either PhotoBooth storage root'
            } else {
                foreach ($group in ($overlappingShares | Group-Object Name,Path)) {
                    $share = $group.Group[0]
                    $rootNames = ($group.Group | Select-Object -ExpandProperty Root -Unique) -join ', '
                    $detail = "Share=$($share.Name); share_path=$($share.Path); storage_roots=$rootNames; special=$($share.Special); ACL=$($share.Access)"
                    if ($share.Special) {
                        Add-Check 'SMB administrative-share overlap' 'WARN' "$detail. Administrative shares are not ordinary guest shares, but privileged remote access may reach this storage tree."
                    } else {
                        Add-Check 'SMB non-administrative-share overlap' 'FAIL' "$detail. A normal SMB share overlaps PhotoBooth storage."
                    }
                }
            }
        } catch {
            Add-Check 'SMB share overlap' 'WARN' "Could not inspect active SMB shares: $($_.Exception.Message)"
        }
    } else {
        Add-Check 'SMB share overlap' 'PASS' "LanmanServer is $($serverServiceForShares.Status); SMB shares are not being served. Persistent share definitions, if any, are left untouched."
    }

    if ($serverServiceForShares.Status -eq 'Running') {
        try {
            $server = Get-SmbServerConfiguration -ErrorAction Stop
            if ($server.EnableSMB1Protocol) {
                Add-Check 'SMB1 protocol' 'WARN' 'SMB1 is enabled; review whether it is required'
            } else {
                Add-Check 'SMB1 protocol' 'PASS' 'SMB1 is disabled'
            }
            Add-Check 'SMB2 protocol' 'PASS' ("SMB2 enabled={0}; SMB1 disabled={1}" -f [bool]$server.EnableSMB2Protocol, -not [bool]$server.EnableSMB1Protocol)
        } catch {
            Add-Check 'SMB server configuration' 'WARN' "Could not read SMB server configuration: $($_.Exception.Message)"
        }
    } else {
        Add-Check 'SMB1 server protocol availability' 'PASS' "LanmanServer is $($serverServiceForShares.Status); no SMB Server endpoint is active. The opt-in helper checks SMB1/SMB2 before starting the service."
    }

    try {
        $serverService = Get-Service -Name LanmanServer -ErrorAction Stop
        if ($serverService.Status -eq 'Running') {
            Add-Check 'SMB server service' 'PASS' 'LanmanServer is running; the separate listener, firewall, share, and ACL checks determine exposure'
        } else {
            Add-Check 'SMB server service' 'PASS' "LanmanServer state is $($serverService.Status)"
        }
    } catch {
        Add-Check 'SMB server service' 'WARN' "Could not inspect LanmanServer: $($_.Exception.Message)"
    }

    $activeProfiles = @()
    try {
        $networkProfiles = @(Get-NetConnectionProfile -ErrorAction Stop)
        foreach ($network in $networkProfiles) {
            $firewallName = switch ([string]$network.NetworkCategory) {
                'DomainAuthenticated' { 'Domain'; break }
                'Private' { 'Private'; break }
                'Public' { 'Public'; break }
                default { $null }
            }
            if ($firewallName) {
                $activeProfiles += $firewallName
                Write-Host ("Network: {0}; category={1}; IPv4={2}; IPv6={3}" -f $network.InterfaceAlias,$network.NetworkCategory,$network.IPv4Connectivity,$network.IPv6Connectivity)
            }
        }

        $firewallProfiles = @(Get-NetFirewallProfile -PolicyStore ActiveStore -ErrorAction Stop)
        foreach ($name in @($activeProfiles | Select-Object -Unique)) {
            $profile = $firewallProfiles | Where-Object Name -eq $name | Select-Object -First 1
            if ($null -eq $profile) {
                Add-Check "Firewall $name profile" 'WARN' 'No matching effective firewall profile was returned'
            } elseif (!$profile.Enabled -or $profile.DefaultInboundAction -ne 'Block') {
                Add-Check "Firewall $name profile" 'FAIL' ("Enabled={0}; DefaultInboundAction={1}; AllowInboundRules={2}" -f $profile.Enabled,$profile.DefaultInboundAction,$profile.AllowInboundRules)
            } else {
                Add-Check "Firewall $name profile" 'PASS' ("Enabled with default inbound Block; AllowInboundRules={0}" -f $profile.AllowInboundRules)
            }
        }
        if ($activeProfiles.Count -eq 0) {
            Add-Check 'Active network profile' 'WARN' 'No active Windows network category could be mapped to a firewall profile'
        }
    } catch {
        Add-Check 'Windows Firewall / network profiles' 'WARN' "Could not fully inspect effective firewall profiles: $($_.Exception.Message)"
    }

    try {
        # Query the port-filter inventory directly rather than walking every
        # firewall rule and fetching its associated filters one by one. This
        # requires permission to read the effective ActiveStore inventory.
        $portFilters = @(Get-NetFirewallPortFilter -PolicyStore ActiveStore -ErrorAction Stop)
        $candidateFilters = @($portFilters | Where-Object {
            $localPorts = @($_.LocalPort | ForEach-Object { [string]$_ })
            ($localPorts | Where-Object { $_ -in @('445','139','137','138') }).Count -gt 0 -or
            ($localPorts -contains 'Any')
        })
        $allowedRules = [System.Collections.Generic.List[string]]::new()
        foreach ($portFilter in $candidateFilters) {
            $localPorts = @($portFilter.LocalPort | ForEach-Object { [string]$_ })
            $rules = @(Get-NetFirewallRule -AssociatedNetFirewallPortFilter $portFilter -ErrorAction Stop |
                Where-Object { $_.Direction -eq 'Inbound' -and $_.Enabled -eq 'True' -and $_.Action -eq 'Allow' })
            foreach ($rule in $rules) {
                $ruleProfiles = [string]$rule.Profile
                $profileApplies = $ruleProfiles -eq 'Any' -or $ruleProfiles -match 'Any' -or
                    ($activeProfiles | Where-Object { $ruleProfiles -match [regex]::Escape($_) }).Count -gt 0
                if (!$profileApplies) { continue }

                $serviceFilter = Get-NetFirewallServiceFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
                $service = if ($serviceFilter) { [string]$serviceFilter.Service } else { 'unknown' }
                $isKnownSMBPort = ($localPorts | Where-Object { $_ -in @('445','139','137','138') }).Count -gt 0
                # Do not classify a program-specific Any-port rule as SMB exposure.
                # Any-port rules bound to LanmanServer or all/system programs can also cover SMB.
                $appFilter = Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
                $program = if ($appFilter) { [string]$appFilter.Program } else { 'unknown' }
                $isBroadServerRule = ($localPorts -contains 'Any') -and (
                    $service -match '(?i)lanmanserver' -or $program -in @('Any', 'System', '*')
                )
                if ($isKnownSMBPort -or $isBroadServerRule) {
                    $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
                    $allowedRules.Add(
                        "$($rule.DisplayName) [Profile=$ruleProfiles; Protocol=$($portFilter.Protocol); Ports=$($localPorts -join ','); Service=$service; Program=$program; RemoteAddress=$($addressFilter.RemoteAddress -join ',')]"
                    )
                }
            }
        }
        if ($allowedRules.Count -gt 0) {
            Add-Check 'Effective inbound SMB firewall allow rules' 'FAIL' ($allowedRules -join '; ')
        } else {
            Add-Check 'Effective inbound SMB firewall allow rules' 'PASS' 'No enabled inbound Allow rule for TCP/UDP 445/139/137/138 or broad Any-port rule bound to LanmanServer/Any/System matched an active profile'
        }
    } catch {
        Add-Check 'Effective inbound SMB firewall allow rules' 'WARN' "Could not enumerate the effective firewall port-filter inventory (an elevated session may be required): $($_.Exception.Message)"
    }

    try {
        $listeners = @(Get-NetTCPConnection -State Listen -ErrorAction Stop |
            Where-Object { $_.LocalPort -in @(445,139) })
        if ($listeners.Count -gt 0) {
            $summary = ($listeners | ForEach-Object { "$($_.LocalAddress):$($_.LocalPort)" } | Sort-Object -Unique) -join ', '
            Add-Check 'SMB TCP listeners' 'WARN' "Listening endpoints: $summary. A listener alone does not prove firewall reachability."
        } else {
            Add-Check 'SMB TCP listeners' 'PASS' 'No TCP listener on ports 445 or 139 was found'
        }
    } catch {
        Add-Check 'SMB TCP listeners' 'WARN' "Could not inspect TCP listeners: $($_.Exception.Message)"
    }

    if ($serverServiceForShares.Status -ne 'Running') {
        Add-Check 'Current SMB sessions' 'PASS' "LanmanServer is $($serverServiceForShares.Status); no SMB Server sessions can be served."
    } else {
        try {
            $sessions = @(Get-SmbSession -ErrorAction Stop)
            if ($sessions.Count -gt 0) {
                $clients = ($sessions | ForEach-Object { "$($_.ClientComputerName)\$($_.ClientUserName)" }) -join '; '
                Add-Check 'Current SMB sessions' 'WARN' "$($sessions.Count) SMB session(s) currently visible: $clients. This does not establish that PhotoBooth files were opened."
            } else {
                Add-Check 'Current SMB sessions' 'PASS' 'No current SMB client sessions were reported'
            }
        } catch {
            Add-Check 'Current SMB sessions' 'WARN' "Cannot enumerate SMB sessions with this identity: $($_.Exception.Message)"
        }
    }
}
catch {
    Add-Check 'Audit execution' 'FAIL' $_.Exception.Message
}

Write-Host ''
Write-Host 'Audit results'
$checks | Format-Table -AutoSize -Wrap
Write-Host ''
Write-Host ("SUMMARY: failed={0}; warnings={1}; checks={2}" -f $script:failed,$script:warnings,$checks.Count)
Write-Host 'Interpretation: this is a host-local configuration review, not a remote penetration test or proof that every network path is unreachable.'
Write-Host 'No settings, shares, firewall rules, services, photos, or session contents were changed or opened.'

if ($script:failed -gt 0) { exit 1 }
