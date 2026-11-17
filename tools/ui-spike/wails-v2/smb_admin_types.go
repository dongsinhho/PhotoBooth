package main

// SMBAdminStatus is read from the Windows service, registry and effective
// PhotoBooth firewall rules. It is deliberately not stored in StoreConfig.
type SMBAdminStatus struct {
	ServiceState           string   `json:"service_state"`
	ServiceStartMode       string   `json:"service_start_mode"`
	AutoShareWksConfigured bool     `json:"auto_share_wks_configured"`
	AutoShareWks           int      `json:"auto_share_wks"`
	AllowRulePresent       bool     `json:"allow_rule_present"`
	AllowLocalAddresses    []string `json:"allow_local_addresses"`
	AllowedRemoteAddresses []string `json:"allowed_remote_addresses"`
	SMBBlockRuleActive     bool     `json:"smb_block_rule_active"`
	NetBIOSBlockTCPActive  bool     `json:"netbios_tcp_block_rule_active"`
	NetBIOSBlockUDPActive  bool     `json:"netbios_udp_block_rule_active"`
	ListenerCount          int      `json:"listener_count"`
}

// SMBAdminInterface describes an up physical adapter and its IPv4 addresses.
type SMBAdminInterface struct {
	InterfaceAlias string   `json:"interface_alias"`
	Description    string   `json:"description"`
	NetworkProfile string   `json:"network_profile"`
	LocalAddresses []string `json:"local_addresses"`
}
