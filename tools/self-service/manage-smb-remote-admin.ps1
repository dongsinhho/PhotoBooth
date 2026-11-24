#requires -Version 5.1
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [ValidateSet('Status', 'ListInterfaces', 'Enable', 'Disable')]
    [string]$Action = 'Status',
    [string]$InterfaceAlias,
    [string]$LocalAddress,
    [string[]]$RemoteAddress = @(),
    [switch]$JsonOutput,
    [switch]$WaitForElevation,
    [switch]$AllowStoppingCustomShares,
    [switch]$SuppressConfirmation,
    [string]$ResultLogPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ($SuppressConfirmation) { $ConfirmPreference = 'None' }
if ($JsonOutput) { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false) }

$script:RuleGroup = 'PhotoBooth Remote Administration'
$script:AllowRuleName = 'PhotoBooth-SMB-RemoteAdmin-Allow'
$script:BlockRuleName = 'PhotoBooth-SMB-RemoteAdmin-DefaultBlock'
$script:TcpNetbiosBlockRuleName = 'PhotoBooth-SMB-NetBIOS-TCP-Block'
$script:UdpNetbiosBlockRuleName = 'PhotoBooth-SMB-NetBIOS-UDP-Block'
$script:RegistryPath = 'HKLM:\SYSTEM\CurrentControlSet\Services\LanmanServer\Parameters'
$script:RegistryValueName = 'AutoShareWks'

function Write-PhotoBoothSmbResultLog {
    param([Parameter(Mandatory)][string]$Message)
    if ([string]::IsNullOrWhiteSpace($ResultLogPath)) { return }
    try {
        $line = ('{0} {1}{2}' -f [DateTime]::UtcNow.ToString('o'), $Message, [Environment]::NewLine)
        [IO.File]::AppendAllText($ResultLogPath, $line, [Text.UTF8Encoding]::new($false))
    } catch {
        Write-Warning "Could not append SMB provisioning result log: $($_.Exception.Message)"
    }
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function ConvertTo-QuotedArgument {
    param([Parameter(Mandatory)][string]$Value)
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Start-ElevatedCopy {
    $powerShellExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $tokens = New-Object 'System.Collections.Generic.List[string]'
    $tokens.Add('-NoProfile')
    $tokens.Add('-ExecutionPolicy')
    $tokens.Add('Bypass')
    if (!$WaitForElevation) { $tokens.Add('-NoExit') }
    $tokens.Add('-File')
    $tokens.Add((ConvertTo-QuotedArgument $PSCommandPath))
    $tokens.Add('-Action')
    $tokens.Add((ConvertTo-QuotedArgument $Action))

    if ($InterfaceAlias) {
        $tokens.Add('-InterfaceAlias')
        $tokens.Add((ConvertTo-QuotedArgument $InterfaceAlias))
    }
    if ($LocalAddress) {
        $tokens.Add('-LocalAddress')
        $tokens.Add((ConvertTo-QuotedArgument $LocalAddress))
    }
    if ($RemoteAddress.Count -gt 0) {
        $tokens.Add('-RemoteAddress')
        foreach ($address in $RemoteAddress) { $tokens.Add((ConvertTo-QuotedArgument $address)) }
    }
    if ($JsonOutput) { $tokens.Add('-JsonOutput') }
    if ($WaitForElevation) { $tokens.Add('-WaitForElevation') }
    if ($AllowStoppingCustomShares) { $tokens.Add('-AllowStoppingCustomShares') }
    if ($SuppressConfirmation) { $tokens.Add('-SuppressConfirmation') }
    if (![string]::IsNullOrWhiteSpace($ResultLogPath)) {
        $tokens.Add('-ResultLogPath')
        $tokens.Add((ConvertTo-QuotedArgument $ResultLogPath))
    }
    if ($WhatIfPreference) { $tokens.Add('-WhatIf') }

    if ($WaitForElevation) {
        $process = Start-Process -FilePath $powerShellExe -Verb RunAs -ArgumentList ($tokens -join ' ') -Wait -PassThru
        exit [int]$process.ExitCode
    }

    Start-Process -FilePath $powerShellExe -Verb RunAs -ArgumentList ($tokens -join ' ')
}

function Get-PrivateIPv4 {
    param([Parameter(Mandatory)][string]$Address)

    if ($Address -notmatch '^\d{1,3}(\.\d{1,3}){3}$') {
        throw "Expected a literal IPv4 address, received: $Address"
    }

    $parsed = [System.Net.IPAddress]::Parse($Address)
    if ($parsed.AddressFamily -ne [System.Net.Sockets.AddressFamily]::InterNetwork) {
        throw "Only IPv4 addresses are supported: $Address"
    }

    $bytes = $parsed.GetAddressBytes()
    $isPrivate = ($bytes[0] -eq 10) -or ($bytes[0] -eq 172 -and $bytes[1] -ge 16 -and $bytes[1] -le 31) -or ($bytes[0] -eq 192 -and $bytes[1] -eq 168)
    if (!$isPrivate) {
        throw "Only RFC1918 private IPv4 addresses are accepted for SMB administration: $Address"
    }

    return $parsed.IPAddressToString
}

function Get-SelectedPrivateInterface {
    if ([string]::IsNullOrWhiteSpace($InterfaceAlias)) {
        throw 'Enable requires -InterfaceAlias for the trusted physical network adapter.'
    }

    $adapter = Get-NetAdapter -Physical -Name $InterfaceAlias -ErrorAction Stop
    if ($adapter.Status -ne 'Up') {
        throw "Selected physical adapter is not Up: $InterfaceAlias ($($adapter.Status))"
    }

    $profiles = @(Get-NetConnectionProfile -InterfaceIndex $adapter.ifIndex -ErrorAction Stop)
    if ($profiles.Count -ne 1 -or $profiles[0].NetworkCategory -ne 'Private') {
        $categories = ($profiles | ForEach-Object { [string]$_.NetworkCategory }) -join ','
        throw "SMB remote administration requires one Windows Private profile. Adapter=$InterfaceAlias; profiles=$categories. Change the trusted network profile manually in Windows Settings; this helper will not change a Public profile."
    }

    $addresses = @(Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 -AddressState Preferred -ErrorAction Stop | Where-Object {
        $_.IPAddress -notlike '169.254.*' -and $_.IPAddress -ne '0.0.0.0'
    } | Select-Object -ExpandProperty IPAddress -Unique)

    if ($addresses.Count -eq 0) {
        throw "No preferred IPv4 address found on $InterfaceAlias."
    }

    if ([string]::IsNullOrWhiteSpace($LocalAddress)) {
        if ($addresses.Count -ne 1) {
            throw "Adapter $InterfaceAlias has multiple IPv4 addresses. Specify one with -LocalAddress: $($addresses -join ', ')"
        }
        $selectedAddress = [string]$addresses[0]
    } else {
        $selectedAddress = Get-PrivateIPv4 -Address $LocalAddress
        if ($addresses -notcontains $selectedAddress) {
            throw "LocalAddress $selectedAddress is not assigned to adapter $InterfaceAlias. Assigned addresses: $($addresses -join ', ')"
        }
    }

    [pscustomobject]@{
        Adapter = $adapter
        Profile = 'Private'
        LocalAddress = $selectedAddress
    }
}

function Get-ValidatedRemoteAddresses {
    if ($RemoteAddress.Count -eq 0) {
        throw 'Enable requires explicit -RemoteAddress values. Any, LocalSubnet, wildcards, ranges and CIDR are not accepted.'
    }

    $validated = @($RemoteAddress | ForEach-Object { Get-PrivateIPv4 -Address $_ } | Select-Object -Unique)
    if ($validated.Count -eq 0) {
        throw 'No valid trusted remote IPv4 source address was supplied.'
    }
    return $validated
}

function Get-AutoShareWksValue {
    $item = Get-ItemProperty -LiteralPath $script:RegistryPath -Name $script:RegistryValueName -ErrorAction SilentlyContinue
    if ($null -eq $item) {
        return [pscustomobject]@{ Exists = $false; Value = $null }
    }
    return [pscustomobject]@{ Exists = $true; Value = [int]$item.($script:RegistryValueName) }
}

function Set-AutoShareWksValue {
    param([Parameter(Mandatory)][ValidateSet(0, 1)][int]$Value)
    if (!(Test-Path -LiteralPath $script:RegistryPath)) {
        New-Item -Path $script:RegistryPath -Force | Out-Null
    }
    New-ItemProperty -LiteralPath $script:RegistryPath -Name $script:RegistryValueName -PropertyType DWord -Value $Value -Force | Out-Null
}

function Assert-NoActiveSmbSessions {
    if ((Get-Service -Name LanmanServer -ErrorAction Stop).Status -ne 'Running') { return }
    $sessions = @(Get-SmbSession -ErrorAction Stop)
    if ($sessions.Count -gt 0) {
        $clients = ($sessions | ForEach-Object { '{0}\{1}' -f $_.ClientComputerName, $_.ClientUserName }) -join '; '
        throw "Active SMB session(s) exist: $clients. Let remote operations finish, then retry. No service/share state was changed."
    }
}

function Assert-NoRunningDependents {
    $dependents = @(Get-Service -Name LanmanServer -DependentServices -ErrorAction Stop | Where-Object { $_.Status -eq 'Running' })
    if ($dependents.Count -gt 0) {
        $names = ($dependents | ForEach-Object { $_.Name }) -join ', '
        throw "Cannot safely change LanmanServer while dependent service(s) are running: $names."
    }
}

function Assert-NoCustomShares {
    $allowedSystemShares = '^(?:[A-Z]\$|ADMIN\$|IPC\$|PRINT\$|FAX\$)$'
    $server = Get-Service -Name LanmanServer -ErrorAction Stop

    if ($server.Status -eq 'Running') {
        $customShares = @(Get-SmbShare -ErrorAction Stop | Where-Object { $_.Name -notmatch $allowedSystemShares })
        if ($customShares.Count -gt 0) {
            $details = ($customShares | ForEach-Object { '{0} [{1}]' -f $_.Name, $_.Path }) -join '; '
            if ($Action -eq 'Disable' -and $AllowStoppingCustomShares) {
                Write-Warning "Explicit opt-in accepted: these custom shares will stop being served when LanmanServer is disabled: $details. Share definitions, paths and permissions are preserved; no share is deleted."
                Write-PhotoBoothSmbResultLog -Message "CUSTOM_SHARES_WILL_BE_UNSERVED=$details"
            } else {
                throw "Custom SMB share(s) exist: $details. The helper will not expose or disrupt them implicitly. For an explicitly approved Disable operation only, pass -AllowStoppingCustomShares; active sessions and service dependencies remain hard blockers."
            }
        }
        return
    }

    # Get-SmbShare fails with ERROR_SERVICE_NOT_ACTIVE when LanmanServer is stopped.
    # For Enable only, inspect persistent share definitions before starting the service.
    if ($Action -eq 'Enable') {
        $shareKey = 'HKLM:\SYSTEM\CurrentControlSet\Services\LanmanServer\Shares'
        if (Test-Path -LiteralPath $shareKey) {
            $shareProperties = @((Get-ItemProperty -LiteralPath $shareKey -ErrorAction Stop).PSObject.Properties |
                Where-Object { $_.Name -notmatch '^PS' -and $_.Name -notmatch $allowedSystemShares })
            if ($shareProperties.Count -gt 0) {
                $names = ($shareProperties | Select-Object -ExpandProperty Name) -join ', '
                throw "Persistent custom SMB share definition(s) exist: $names. The helper will not start LanmanServer or expose them implicitly."
            }
        }
    } else {
        Write-Host 'LanmanServer is already stopped; active SMB shares are not being served. Persistent share definitions, if any, are left untouched.'
    }
}

function Assert-Smb2ServerConfiguration {
    $smbConfig = Get-SmbServerConfiguration -ErrorAction Stop
    if ([bool]$smbConfig.EnableSMB1Protocol) {
        throw 'SMB1 is enabled. Disable and verify SMB1 through the operator security baseline before enabling remote administration.'
    }
    if (![bool]$smbConfig.EnableSMB2Protocol) {
        throw 'SMB2 is disabled. This helper will not change SMB protocol configuration.'
    }
}

function Test-ProfileApplies {
    param([string]$Profile)
    return ($Profile -eq 'Any') -or ($Profile -match '(^|,\s*)Private($|,\s*)')
}

function Get-ExistingSmbPortRules {
    $portFilters = @(Get-NetFirewallPortFilter -PolicyStore ActiveStore -ErrorAction Stop)
    $registryPath = 'HKLM:\SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\FirewallRules'
    $found = @()

    foreach ($filter in $portFilters) {
        $ports = @($filter.LocalPort | ForEach-Object { [string]$_ })
        $protocol = [string]$filter.Protocol
        $isTcp445 = ($ports -contains '445') -and ($protocol -in @('TCP', '6', 'Any'))
        $isAnyPort = $ports -contains 'Any'
        $isSmbRelevantAnyPortProtocol = $protocol -in @('TCP', '6', 'Any')
        if (!$isTcp445 -and !($isAnyPort -and $isSmbRelevantAnyPortProtocol)) { continue }

        $rules = @(Get-NetFirewallRule -AssociatedNetFirewallPortFilter $filter -ErrorAction Stop | Where-Object {
            $_.Direction -eq 'Inbound' -and $_.Enabled -eq 'True' -and (Test-ProfileApplies -Profile ([string]$_.Profile))
        })

        foreach ($rule in $rules) {
            # Some Windows rules carry package identity or a WFD trust-tuple keyword
            # in their raw registry definition. NetSecurity's application filter may
            # show Program=Any / Package empty for these, so do not treat them as
            # host-wide SMB allows merely because their port filter says Any.
            $rawRule = $null
            $rawRuleItem = Get-ItemProperty -LiteralPath $registryPath -Name ([string]$rule.Name) -ErrorAction SilentlyContinue
            if ($null -ne $rawRuleItem) {
                $rawRuleProperty = $rawRuleItem.PSObject.Properties[[string]$rule.Name]
                if ($null -ne $rawRuleProperty) { $rawRule = [string]$rawRuleProperty.Value }
            }
            $isPackageScopedRule = $rawRule -match '(?i)(?:^|\|)(?:PFN|AppPkgId)=[^|]+'
            $isWifiDirectKernelRule = $rawRule -match '(?i)(?:^|\|)TTK2_27=WFDKmDriver(?:\||$)'
            if ($isAnyPort -and ($isPackageScopedRule -or $isWifiDirectKernelRule)) { continue }

            $serviceFilter = Get-NetFirewallServiceFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue | Select-Object -First 1
            $appFilter = Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue | Select-Object -First 1
            $serviceName = if ($serviceFilter) { [string]$serviceFilter.Service } else { 'unknown' }
            $program = if ($appFilter) { [string]$appFilter.Program } else { 'unknown' }
            $isBroadAnyPortServer = $isAnyPort -and $isSmbRelevantAnyPortProtocol -and (
                $serviceName -match '(?i)LanmanServer' -or $program -in @('Any', 'System', '*')
            )
            if ($isTcp445 -or $isBroadAnyPortServer) {
                $found += [pscustomobject]@{
                    Name = [string]$rule.Name
                    DisplayName = [string]$rule.DisplayName
                    Action = [string]$rule.Action
                    Profile = [string]$rule.Profile
                    Protocol = $protocol
                    LocalPort = ($ports -join ',')
                    Service = $serviceName
                    Program = $program
                }
            }
        }
    }
    return $found
}

function Assert-NoConflictingSmbRules {
    $rules = @(Get-ExistingSmbPortRules)
    $blockers = @($rules | Where-Object { $_.Action -eq 'Block' -and $_.Name -ne $script:BlockRuleName })
    if ($blockers.Count -gt 0) {
        $details = ($blockers | ForEach-Object { "$($_.DisplayName) [$($_.Profile); $($_.Protocol); $($_.LocalPort)]" }) -join '; '
        throw "A pre-existing effective SMB Block rule applies to the Private profile: $details"
    }

    $otherAllows = @($rules | Where-Object { $_.Action -eq 'Allow' -and $_.Name -ne $script:AllowRuleName })
    if ($otherAllows.Count -gt 0) {
        $details = ($otherAllows | ForEach-Object { "$($_.DisplayName) [$($_.Profile); $($_.Protocol); $($_.LocalPort)]" }) -join '; '
        throw "A pre-existing SMB Allow rule could broaden access beyond the PhotoBooth allowlist: $details"
    }
}

function Get-ManagedRule {
    param([Parameter(Mandatory)][string]$Name)
    $matches = @(Get-NetFirewallRule -PolicyStore PersistentStore -Name $Name -ErrorAction SilentlyContinue)
    if ($matches.Count -gt 1) { throw "Multiple persistent firewall rules use the managed name $Name." }
    if ($matches.Count -eq 1 -and $matches[0].Group -ne $script:RuleGroup) {
        throw "Firewall rule name collision for $Name; the existing rule is not owned by PhotoBooth."
    }
    return $matches | Select-Object -First 1
}

function Ensure-ManagedBlockRule {
    param([string[]]$Profiles = @('Any'))

    $rule = Get-ManagedRule -Name $script:BlockRuleName
    if ($null -eq $rule) {
        New-NetFirewallRule -Name $script:BlockRuleName -DisplayName 'PhotoBooth - Block inbound SMB by default' -Group $script:RuleGroup -Description 'PhotoBooth default-off policy: block inbound direct-hosted SMB except the explicitly allowlisted Private interface during opt-in administration.' -Direction Inbound -Action Block -Enabled True -Profile $Profiles -Protocol TCP -LocalPort 445 -LocalAddress Any -RemoteAddress Any -EdgeTraversalPolicy Block | Out-Null
    } else {
        Set-NetFirewallRule -Name $script:BlockRuleName -Enabled True -Direction Inbound -Action Block -Profile $Profiles -EdgeTraversalPolicy Block | Out-Null
        $portFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        Set-NetFirewallPortFilter -InputObject $portFilter -Protocol TCP -LocalPort 445 | Out-Null
        $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        Set-NetFirewallAddressFilter -InputObject $addressFilter -LocalAddress Any -RemoteAddress Any | Out-Null
    }
}

function Ensure-ManagedNetbiosBlockRules {
    $tcpRule = Get-ManagedRule -Name $script:TcpNetbiosBlockRuleName
    if ($null -eq $tcpRule) {
        New-NetFirewallRule -Name $script:TcpNetbiosBlockRuleName -DisplayName 'PhotoBooth - Block legacy NetBIOS SMB over TCP' -Group $script:RuleGroup -Description 'Block legacy NetBIOS name/session ports; opt-in administration uses direct-hosted SMB TCP 445 only.' -Direction Inbound -Action Block -Enabled True -Profile Any -Protocol TCP -LocalPort @('137', '139') -LocalAddress Any -RemoteAddress Any -EdgeTraversalPolicy Block | Out-Null
    } else {
        Set-NetFirewallRule -Name $script:TcpNetbiosBlockRuleName -Enabled True -Direction Inbound -Action Block -Profile Any -EdgeTraversalPolicy Block | Out-Null
        $tcpPortFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $tcpRule -ErrorAction Stop
        Set-NetFirewallPortFilter -InputObject $tcpPortFilter -Protocol TCP -LocalPort @('137', '139') | Out-Null
        $tcpAddressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $tcpRule -ErrorAction Stop
        Set-NetFirewallAddressFilter -InputObject $tcpAddressFilter -LocalAddress Any -RemoteAddress Any | Out-Null
    }

    $udpRule = Get-ManagedRule -Name $script:UdpNetbiosBlockRuleName
    if ($null -eq $udpRule) {
        New-NetFirewallRule -Name $script:UdpNetbiosBlockRuleName -DisplayName 'PhotoBooth - Block legacy NetBIOS datagram ports' -Group $script:RuleGroup -Description 'Block legacy NetBIOS name/datagram ports; opt-in administration uses direct-hosted SMB TCP 445 only.' -Direction Inbound -Action Block -Enabled True -Profile Any -Protocol UDP -LocalPort @('137', '138') -LocalAddress Any -RemoteAddress Any -EdgeTraversalPolicy Block | Out-Null
    } else {
        Set-NetFirewallRule -Name $script:UdpNetbiosBlockRuleName -Enabled True -Direction Inbound -Action Block -Profile Any -EdgeTraversalPolicy Block | Out-Null
        $udpPortFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $udpRule -ErrorAction Stop
        Set-NetFirewallPortFilter -InputObject $udpPortFilter -Protocol UDP -LocalPort @('137', '138') | Out-Null
        $udpAddressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $udpRule -ErrorAction Stop
        Set-NetFirewallAddressFilter -InputObject $udpAddressFilter -LocalAddress Any -RemoteAddress Any | Out-Null
    }
}

function Assert-ManagedBlockRulesEffective {
    param(
        [switch]$IncludeSmb445,
        [string[]]$Smb445Profiles = @('Any')
    )

    $specs = @(
        [pscustomobject]@{ Name=$script:TcpNetbiosBlockRuleName; Protocol='TCP'; Ports=@('137','139'); Profiles=@('Any') },
        [pscustomobject]@{ Name=$script:UdpNetbiosBlockRuleName; Protocol='UDP'; Ports=@('137','138'); Profiles=@('Any') }
    )
    if ($IncludeSmb445) {
        $specs += [pscustomobject]@{ Name=$script:BlockRuleName; Protocol='TCP'; Ports=@('445'); Profiles=$Smb445Profiles }
    }

    foreach ($spec in $specs) {
        $effectiveRules = @(Get-NetFirewallRule -PolicyStore ActiveStore -Name $spec.Name -ErrorAction SilentlyContinue |
            Where-Object { $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Block' })
        if ($effectiveRules.Count -eq 0) {
            throw "Managed firewall Block rule is not effective in ActiveStore: $($spec.Name)"
        }

        $rule = $effectiveRules | Select-Object -First 1
        $actualProfiles = @($rule.Profile | ForEach-Object { ([string]$_ -split ',') } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        $expectedProfiles = @($spec.Profiles)
        if ($expectedProfiles -contains 'Any') {
            if ($actualProfiles -notcontains 'Any') {
                throw "Managed firewall Block rule must apply to all profiles: $($spec.Name); profiles=$($actualProfiles -join ',')"
            }
        } elseif ($actualProfiles.Count -ne $expectedProfiles.Count -or
            @($expectedProfiles | Where-Object { $actualProfiles -notcontains $_ }).Count -gt 0) {
            throw "Managed firewall Block rule has unexpected profile scope: $($spec.Name); profiles=$($actualProfiles -join ','); expected=$($expectedProfiles -join ',')"
        }

        $portFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        $actualPorts = @($portFilter.LocalPort | ForEach-Object { [string]$_ })
        if ([string]$portFilter.Protocol -ne $spec.Protocol -or
            $actualPorts.Count -ne $spec.Ports.Count -or
            @($spec.Ports | Where-Object { $actualPorts -notcontains $_ }).Count -gt 0) {
            throw "Managed firewall Block rule has an unexpected protocol/port filter: $($spec.Name); protocol=$($portFilter.Protocol); ports=$($actualPorts -join ',')"
        }

        $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        if (@($addressFilter.LocalAddress | ForEach-Object { [string]$_ }) -notcontains 'Any' -or
            @($addressFilter.RemoteAddress | ForEach-Object { [string]$_ }) -notcontains 'Any') {
            throw "Managed firewall Block rule does not cover all local and remote addresses: $($spec.Name)"
        }
    }
}

function Ensure-ManagedAllowRule {
    param([Parameter(Mandatory)][string]$SelectedLocalAddress, [Parameter(Mandatory)][string[]]$SelectedRemoteAddresses)
    $rule = Get-ManagedRule -Name $script:AllowRuleName
    if ($null -eq $rule) {
        New-NetFirewallRule -Name $script:AllowRuleName -DisplayName 'PhotoBooth - Allow restricted SMB remote administration' -Group $script:RuleGroup -Description 'PhotoBooth opt-in rule: TCP 445 from explicit trusted RFC1918 IPv4 addresses, on one Private interface only.' -Direction Inbound -Action Allow -Enabled True -Profile Private -Protocol TCP -LocalPort 445 -LocalAddress $SelectedLocalAddress -RemoteAddress $SelectedRemoteAddresses -EdgeTraversalPolicy Block | Out-Null
    } else {
        Set-NetFirewallRule -Name $script:AllowRuleName -Enabled True -Direction Inbound -Action Allow -Profile Private -EdgeTraversalPolicy Block | Out-Null
        $portFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        Set-NetFirewallPortFilter -InputObject $portFilter -Protocol TCP -LocalPort 445 | Out-Null
        $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $rule -ErrorAction Stop
        Set-NetFirewallAddressFilter -InputObject $addressFilter -LocalAddress $SelectedLocalAddress -RemoteAddress $SelectedRemoteAddresses | Out-Null
    }

    $effective = @(Get-NetFirewallRule -PolicyStore ActiveStore -Name $script:AllowRuleName -ErrorAction Stop | Where-Object {
        $_.Enabled -eq 'True' -and $_.Action -eq 'Allow' -and $_.Direction -eq 'Inbound'
    })
    if ($effective.Count -eq 0) {
        throw 'The managed allow rule is not present in the effective ActiveStore. Local firewall rules may be blocked by policy; refusing to start SMB Server.'
    }

    foreach ($effectiveRule in $effective) {
        if ([string]$effectiveRule.Profile -ne 'Private' -or [string]$effectiveRule.EdgeTraversalPolicy -ne 'Block') {
            throw "Effective PhotoBooth SMB Allow rule has an unexpected profile/edge scope: profile=$($effectiveRule.Profile); edge=$($effectiveRule.EdgeTraversalPolicy)"
        }

        $portFilter = Get-NetFirewallPortFilter -AssociatedNetFirewallRule $effectiveRule -ErrorAction Stop
        $actualPorts = @($portFilter.LocalPort | ForEach-Object { [string]$_ })
        if ([string]$portFilter.Protocol -ne 'TCP' -or $actualPorts.Count -ne 1 -or $actualPorts[0] -ne '445') {
            throw "Effective PhotoBooth SMB Allow rule must be TCP 445 only; protocol=$($portFilter.Protocol); ports=$($actualPorts -join ',')"
        }

        $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $effectiveRule -ErrorAction Stop
        $actualLocal = @($addressFilter.LocalAddress | ForEach-Object { [string]$_ } | Sort-Object -Unique)
        $actualRemote = @($addressFilter.RemoteAddress | ForEach-Object { [string]$_ } | Sort-Object -Unique)
        $expectedLocal = @($SelectedLocalAddress | Sort-Object -Unique)
        $expectedRemote = @($SelectedRemoteAddresses | Sort-Object -Unique)
        if ($actualLocal.Count -ne 1 -or $actualLocal[0] -ne $expectedLocal[0] -or
            $actualRemote.Count -ne $expectedRemote.Count -or
            @($expectedRemote | Where-Object { $actualRemote -notcontains $_ }).Count -gt 0) {
            throw "Effective PhotoBooth SMB Allow rule address scope does not match the selected local IP and explicit remote allowlist. Local=$($actualLocal -join ','); Remote=$($actualRemote -join ',')"
        }
    }
}

function Remove-ManagedRule {
    param([Parameter(Mandatory)][string]$Name)
    $rule = Get-ManagedRule -Name $Name
    if ($null -ne $rule) {
        Remove-NetFirewallRule -Name $Name -PolicyStore PersistentStore -ErrorAction Stop
    }
}

function Get-SmbRemoteAdminStatus {
    $service = Get-CimInstance Win32_Service -Filter "Name='LanmanServer'" -ErrorAction Stop
    $autoShare = Get-AutoShareWksValue
    $allowRule = Get-ManagedRule -Name $script:AllowRuleName
    $blockRule = Get-ManagedRule -Name $script:BlockRuleName
    $tcpNetbiosBlock = Get-ManagedRule -Name $script:TcpNetbiosBlockRuleName
    $udpNetbiosBlock = Get-ManagedRule -Name $script:UdpNetbiosBlockRuleName

    Write-Host 'PhotoBooth Windows SMB remote-administration status' -ForegroundColor Cyan
    [pscustomobject]@{
        Service = $service.Name
        State = $service.State
        StartMode = $service.StartMode
        AutoShareWks = if ($autoShare.Exists) { $autoShare.Value } else { '(not configured; Windows default)' }
        ManagedAllowRule = if ($allowRule) { 'Present' } else { 'Absent' }
        ManagedDefaultBlockRule = if ($blockRule) { 'Present' } else { 'Absent' }
        ManagedTcpNetbiosBlockRule = if ($tcpNetbiosBlock) { 'Present' } else { 'Absent' }
        ManagedUdpNetbiosBlockRule = if ($udpNetbiosBlock) { 'Present' } else { 'Absent' }
    } | Format-List

    if ($service.State -eq 'Running') {
        try {
            Write-Host 'SMB shares currently reported:'
            Get-SmbShare -ErrorAction Stop | Select-Object Name, Path, Special | Sort-Object Name | Format-Table -AutoSize
        } catch {
            Write-Warning "Could not enumerate SMB shares: $($_.Exception.Message)"
        }
    } else {
        Write-Host 'SMB shares: not being served because LanmanServer is stopped.'
        $shareKey = 'HKLM:\SYSTEM\CurrentControlSet\Services\LanmanServer\Shares'
        if (Test-Path -LiteralPath $shareKey) {
            $persisted = (Get-ItemProperty -LiteralPath $shareKey -ErrorAction SilentlyContinue).PSObject.Properties |
                Where-Object { $_.Name -notmatch '^PS' } |
                Select-Object -ExpandProperty Name
            if ($persisted) {
                Write-Host "Persistent share definitions (not currently served): $($persisted -join ', ')"
            }
        }
    }

    try {
        Write-Host 'TCP listeners on SMB ports:'
        Get-NetTCPConnection -State Listen -ErrorAction Stop | Where-Object {
            $_.LocalPort -in @(445, 139)
        } | Select-Object LocalAddress, LocalPort, OwningProcess | Sort-Object LocalPort, LocalAddress -Unique | Format-Table -AutoSize
    } catch {
        Write-Warning "Could not inspect SMB TCP listeners: $($_.Exception.Message)"
    }

    if ($allowRule) {
        Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $allowRule | Select-Object LocalAddress, RemoteAddress | Format-List
    }
}

function Get-PhotoBoothSmbStatusObject {
    $service = Get-CimInstance Win32_Service -Filter "Name='LanmanServer'" -ErrorAction Stop
    $autoShare = Get-AutoShareWksValue
    $allowRule = Get-ManagedRule -Name $script:AllowRuleName
    $allowLocal = @()
    $allowRemote = @()
    if ($allowRule) {
        $addressFilter = Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $allowRule -ErrorAction SilentlyContinue
        if ($addressFilter) {
            $allowLocal = @($addressFilter.LocalAddress | ForEach-Object { [string]$_ })
            $allowRemote = @($addressFilter.RemoteAddress | ForEach-Object { [string]$_ })
        }
    }

    $activeManaged = @(Get-NetFirewallRule -PolicyStore ActiveStore -ErrorAction SilentlyContinue | Where-Object {
        $_.Group -eq $script:RuleGroup -and $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Block'
    })
    $activeNames = @($activeManaged | Select-Object -ExpandProperty Name -Unique)
    $listeners = @()
    try {
        $listeners = @(Get-NetTCPConnection -State Listen -ErrorAction Stop | Where-Object {
            $_.LocalPort -in @(445, 139)
        })
    } catch { }

    [pscustomobject]@{
        service_state = [string]$service.State
        service_start_mode = [string]$service.StartMode
        auto_share_wks_configured = [bool]$autoShare.Exists
        auto_share_wks = if ($autoShare.Exists) { [int]$autoShare.Value } else { 1 }
        allow_rule_present = ($null -ne $allowRule)
        allow_local_addresses = @($allowLocal)
        allowed_remote_addresses = @($allowRemote)
        smb_block_rule_active = ($activeNames -contains $script:BlockRuleName)
        netbios_tcp_block_rule_active = ($activeNames -contains $script:TcpNetbiosBlockRuleName)
        netbios_udp_block_rule_active = ($activeNames -contains $script:UdpNetbiosBlockRuleName)
        listener_count = $listeners.Count
    }
}

function Get-PhotoBoothPhysicalInterfaces {
    $items = @(foreach ($adapter in @(Get-NetAdapter -Physical -ErrorAction Stop | Where-Object { $_.Status -eq 'Up' })) {
        $profiles = @(Get-NetConnectionProfile -InterfaceIndex $adapter.ifIndex -ErrorAction SilentlyContinue)
        $profile = if ($profiles.Count -eq 1) { [string]$profiles[0].NetworkCategory } else { 'Unknown' }
        $addresses = @(Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 -AddressState Preferred -ErrorAction SilentlyContinue |
            Where-Object { $_.IPAddress -notlike '169.254.*' -and $_.IPAddress -ne '0.0.0.0' } |
            Select-Object -ExpandProperty IPAddress -Unique)
        if ($addresses.Count -eq 0) { continue }
        [pscustomobject]@{
            interface_alias = [string]$adapter.Name
            description = [string]$adapter.InterfaceDescription
            network_profile = $profile
            local_addresses = @($addresses)
        }
    })
    return $items
}

try {
    if ($AllowStoppingCustomShares -and $Action -ne 'Disable') {
        throw '-AllowStoppingCustomShares is supported only with -Action Disable.'
    }
    if ($SuppressConfirmation -and $Action -ne 'Disable') {
        throw '-SuppressConfirmation is supported only with -Action Disable.'
    }
    Write-PhotoBoothSmbResultLog -Message ("ACTION_START={0}; IS_ADMIN={1}; ALLOW_STOPPING_CUSTOM_SHARES={2}" -f $Action, (Test-IsAdministrator), [bool]$AllowStoppingCustomShares)

    if ($Action -eq 'ListInterfaces') {
        $interfaces = @(Get-PhotoBoothPhysicalInterfaces)
        if ($JsonOutput) {
            Write-Output (ConvertTo-Json -InputObject $interfaces -Depth 5 -Compress)
        } else {
            $interfaces | Format-Table -AutoSize
        }
        return
    }

    if ($Action -eq 'Status' -and $JsonOutput) {
        $status = Get-PhotoBoothSmbStatusObject
        Write-Output (ConvertTo-Json -InputObject $status -Depth 5 -Compress)
        return
    }

    if ($Action -in @('Enable', 'Disable') -and !(Test-IsAdministrator)) {
        Write-Host 'Requesting Administrator approval through Windows UAC...'
        Start-ElevatedCopy
        return
    }

    if ($Action -eq 'Status') {
        Get-SmbRemoteAdminStatus
        return
    }

    $service = Get-Service -Name LanmanServer -ErrorAction Stop
    $autoShareBefore = Get-AutoShareWksValue
    Assert-NoActiveSmbSessions
    Assert-NoRunningDependents
    Assert-NoCustomShares

    if ($Action -eq 'Enable') {
        $network = Get-SelectedPrivateInterface
        $remoteAddresses = @(Get-ValidatedRemoteAddresses)
        if ($remoteAddresses -contains $network.LocalAddress) {
            throw 'A remote source address cannot be the same as the selected booth local address.'
        }

        # The SMB CIM provider may fail with ERROR_FILE_NAME_TOO_LONG while
        # LanmanServer is stopped. Validate before changes only when already running;
        # otherwise validate immediately after the allowlisted service start below.
        if ($service.Status -eq 'Running') {
            Assert-Smb2ServerConfiguration
        }

        Assert-NoConflictingSmbRules
        $summary = "Enable LanmanServer on $($network.Adapter.Name) [$($network.LocalAddress)] / Private; allow TCP 445 only from $($remoteAddresses -join ', '); restore workstation administrative shares; keep legacy NetBIOS ports blocked."
        if (!$PSCmdlet.ShouldProcess($summary, 'Enable PhotoBooth SMB remote administration')) { return }

        try {
            Set-AutoShareWksValue -Value 1
            Ensure-ManagedNetbiosBlockRules
            # Keep TCP 445 blocked on Public/Domain while allowing only the selected Private interface.
            Ensure-ManagedBlockRule -Profiles @('Public', 'Domain')
            Assert-ManagedBlockRulesEffective -IncludeSmb445 -Smb445Profiles @('Public', 'Domain')
            Ensure-ManagedAllowRule -SelectedLocalAddress $network.LocalAddress -SelectedRemoteAddresses $remoteAddresses
            Set-Service -Name LanmanServer -StartupType Automatic

            if ($service.Status -eq 'Running' -and $autoShareBefore.Exists -and $autoShareBefore.Value -eq 0) {
                Restart-Service -Name LanmanServer -ErrorAction Stop
            } elseif ($service.Status -ne 'Running') {
                Start-Service -Name LanmanServer -ErrorAction Stop
            }

            $after = Get-Service -Name LanmanServer
            if ($after.Status -ne 'Running') { throw "LanmanServer did not reach Running state (current: $($after.Status))." }
            Assert-Smb2ServerConfiguration
            $shareNames = @(Get-SmbShare -ErrorAction Stop | Select-Object -ExpandProperty Name)
            if ($shareNames -notcontains 'C$' -or $shareNames -notcontains 'ADMIN$') {
                throw "Expected workstation administrative shares were not created after enabling Server. Current shares: $($shareNames -join ', ')"
            }

            Write-Host 'SMB remote administration is ENABLED.' -ForegroundColor Yellow
            Write-Host "Local interface: $($network.Adapter.Name) / $($network.LocalAddress) / Private"
            Write-Host "Allowed remote IPv4 sources: $($remoteAddresses -join ', ')"
            Write-Host 'Inbound rule: TCP 445 only, on the selected Private interface and remote allowlist. Public/Domain TCP 445 remain blocked; legacy NetBIOS TCP/UDP ports 137-139 remain blocked.'
            Write-Host 'Use authenticated administrator credentials on the remote client; this firewall rule is not an authentication grant.'
        } catch {
            $failure = $_
            Write-Warning 'Enable failed. Applying fail-closed rollback.'
            try { Ensure-ManagedBlockRule -Profiles @('Any'); Ensure-ManagedNetbiosBlockRules } catch { Write-Warning "Could not ensure default block rules: $($_.Exception.Message)" }
            try { Remove-ManagedRule -Name $script:AllowRuleName } catch { Write-Warning "Could not remove managed allow rule: $($_.Exception.Message)" }
            try {
                Set-AutoShareWksValue -Value 0
                Set-Service -Name LanmanServer -StartupType Disabled
                if ((Get-Service -Name LanmanServer).Status -eq 'Running') {
                    Stop-Service -Name LanmanServer -ErrorAction Stop
                }
            } catch {
                Write-Warning "Rollback service/registry operation needs administrator review: $($_.Exception.Message)"
            }
            throw $failure
        }
        return
    }

    if ($Action -eq 'Disable') {
        $summary = 'Block inbound TCP 445 on all profiles, remove only the PhotoBooth allow rule, set AutoShareWks=0, stop LanmanServer and disable its startup. Existing custom shares and active sessions were checked; no unrelated rules/shares will be deleted.'
        if (!$PSCmdlet.ShouldProcess($summary, 'Disable PhotoBooth SMB remote administration')) {
            Write-PhotoBoothSmbResultLog -Message 'WHATIF_PREFLIGHT_COMPLETE=True; NO_SYSTEM_CHANGES_APPLIED=True'
            return
        }

        Ensure-ManagedBlockRule
        Ensure-ManagedNetbiosBlockRules
        Remove-ManagedRule -Name $script:AllowRuleName
        Set-AutoShareWksValue -Value 0
        Set-Service -Name LanmanServer -StartupType Disabled
        if ((Get-Service -Name LanmanServer).Status -eq 'Running') {
            Stop-Service -Name LanmanServer -ErrorAction Stop
        }

        $after = Get-Service -Name LanmanServer
        if ($after.Status -ne 'Stopped') {
            throw "LanmanServer is not stopped (current: $($after.Status)). The managed inbound Block rules remain in place; review the service manually."
        }

        $serviceDetails = Get-CimInstance Win32_Service -Filter "Name='LanmanServer'" -ErrorAction Stop
        if ($serviceDetails.StartMode -ne 'Disabled') {
            throw "LanmanServer startup is not Disabled (current: $($serviceDetails.StartMode))."
        }
        $autoShareAfter = Get-AutoShareWksValue
        if (!$autoShareAfter.Exists -or $autoShareAfter.Value -ne 0) {
            throw 'AutoShareWks is not explicitly set to DWORD 0.'
        }
        if ($null -ne (Get-ManagedRule -Name $script:AllowRuleName)) {
            throw 'The PhotoBooth inbound Allow rule still exists after Disable.'
        }
        Assert-ManagedBlockRulesEffective -IncludeSmb445

        $listeners = @(Get-NetTCPConnection -State Listen -ErrorAction Stop | Where-Object { $_.LocalPort -in @(445, 139) })
        if ($listeners.Count -gt 0) {
            $endpoints = ($listeners | ForEach-Object { "$($_.LocalAddress):$($_.LocalPort)/PID$($_.OwningProcess)" }) -join ', '
            Write-Warning "Kernel-level SMB/NetBIOS listener registrations remain after LanmanServer stopped: $endpoints. Service, registry and effective inbound Block rules are verified; a listener alone does not prove remote reachability. Confirm from a trusted second device."
        }

        Write-Host 'SMB remote administration is DISABLED.' -ForegroundColor Green
        Write-PhotoBoothSmbResultLog -Message 'SMB_DISABLE_COMPLETED=True; LANMANSERVER=Stopped/Disabled; AUTOSHAREWKS=0; SHARE_DEFINITIONS_PRESERVED=True'
        Write-Host 'LanmanServer is Stopped/Disabled; AutoShareWks=0; effective inbound Block rules verified for TCP 445 and NetBIOS TCP/UDP ports.'
        Write-Host 'IPC$ and custom share definitions are not deleted. They are not served while LanmanServer is stopped.'
        return
    }
}
catch {
    Write-PhotoBoothSmbResultLog -Message ("ERROR_EXCEPTION=" + $_.Exception.ToString())
    Write-PhotoBoothSmbResultLog -Message ("ERROR_SCRIPT_STACK=" + [string]$_.ScriptStackTrace)
    Write-Host ("ERROR_POSITION: " + $_.InvocationInfo.PositionMessage) -ForegroundColor Red
    if ($_.ScriptStackTrace) {
        Write-Host ("ERROR_SCRIPT_STACK: " + $_.ScriptStackTrace) -ForegroundColor Red
    }
    Write-Host ("ERROR_EXCEPTION: " + $_.Exception.ToString()) -ForegroundColor Red
    Write-Error $_
    exit 1
}
