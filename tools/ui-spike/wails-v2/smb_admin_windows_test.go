//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSMBAdminEnableArgs(t *testing.T) {
	args, err := buildSMBAdminEnableArgs(
		"Wi-Fi",
		"192.168.1.11",
		"192.168.1.20\n10.10.10.11;192.168.1.20",
	)
	if err != nil {
		t.Fatalf("buildSMBAdminEnableArgs: %v", err)
	}
	want := []string{
		"-InterfaceAlias", "Wi-Fi",
		"-LocalAddress", "192.168.1.11",
		"-RemoteAddress", "192.168.1.20", "10.10.10.11",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildSMBAdminEnableArgsRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name, iface, local, remote string
	}{
		{name: "missing adapter", iface: "", local: "192.168.1.11", remote: "192.168.1.20"},
		{name: "public local address", iface: "Wi-Fi", local: "8.8.8.8", remote: "192.168.1.20"},
		{name: "ipv6 local address", iface: "Wi-Fi", local: "fd00::1", remote: "192.168.1.20"},
		{name: "ipv4 mapped ipv6 local address", iface: "Wi-Fi", local: "::ffff:192.168.1.11", remote: "192.168.1.20"},
		{name: "ipv4 mapped ipv6 remote address", iface: "Wi-Fi", local: "192.168.1.11", remote: "::ffff:192.168.1.20"},
		{name: "missing allowlist", iface: "Wi-Fi", local: "192.168.1.11", remote: ""},
		{name: "public remote address", iface: "Wi-Fi", local: "192.168.1.11", remote: "8.8.8.8"},
		{name: "cidr remote address", iface: "Wi-Fi", local: "192.168.1.11", remote: "192.168.1.0/24"},
		{name: "remote equals local", iface: "Wi-Fi", local: "192.168.1.11", remote: "192.168.1.11"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildSMBAdminEnableArgs(test.iface, test.local, test.remote); err == nil {
				t.Fatal("expected unsafe SMB administration arguments to be rejected")
			}
		})
	}
}

func TestSMBAdminReadMethodsParseHelperJSONAndPassReadOnlyActions(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "manage-smb-remote-admin.ps1")
	if err := os.WriteFile(helper, []byte("# test helper path only"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTOBOOTH_SMB_HELPER_PATH", helper)

	oldRunner := smbAdminCommandRunner
	t.Cleanup(func() { smbAdminCommandRunner = oldRunner })

	app := NewApp()
	var calls [][]string
	smbAdminCommandRunner = func(_ context.Context, executable string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		if containsArg(args, "ListInterfaces") {
			return []byte(`[{"interface_alias":"Wi-Fi","description":"test adapter","network_profile":"Private","local_addresses":["192.168.1.11"]}]`), nil
		}
		return []byte(`{"service_state":"Stopped","service_start_mode":"Disabled","auto_share_wks_configured":true,"auto_share_wks":0,"allow_rule_present":false,"allow_local_addresses":[],"allowed_remote_addresses":[],"smb_block_rule_active":true,"netbios_tcp_block_rule_active":true,"netbios_udp_block_rule_active":true,"listener_count":1}`), nil
	}

	status, err := app.GetSmbAdminStatus()
	if err != nil {
		t.Fatalf("GetSmbAdminStatus: %v", err)
	}
	if status.ServiceState != "Stopped" || status.ServiceStartMode != "Disabled" ||
		status.AutoShareWks != 0 || status.AllowRulePresent || !status.SMBBlockRuleActive ||
		!status.NetBIOSBlockTCPActive || !status.NetBIOSBlockUDPActive || status.ListenerCount != 1 {
		t.Fatalf("unexpected SMB status: %+v", status)
	}
	if !containsSequence(calls[0], "-Action", "Status") || !containsArg(calls[0], "-JsonOutput") {
		t.Fatalf("status helper arguments = %#v", calls[0])
	}

	interfaces, err := app.GetSmbAdminInterfaces()
	if err != nil {
		t.Fatalf("GetSmbAdminInterfaces: %v", err)
	}
	if len(interfaces) != 1 || interfaces[0].InterfaceAlias != "Wi-Fi" ||
		interfaces[0].NetworkProfile != "Private" ||
		!reflect.DeepEqual(interfaces[0].LocalAddresses, []string{"192.168.1.11"}) {
		t.Fatalf("unexpected physical interfaces: %+v", interfaces)
	}
	if !containsSequence(calls[1], "-Action", "ListInterfaces") || !containsArg(calls[1], "-JsonOutput") {
		t.Fatalf("interface helper arguments = %#v", calls[1])
	}
}

func TestSetSMBAdminEnabledInvokesUACHelperWithValidatedScope(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "manage-smb-remote-admin.ps1")
	if err := os.WriteFile(helper, []byte("# test helper path only"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTOBOOTH_SMB_HELPER_PATH", helper)

	oldRunner := smbAdminCommandRunner
	t.Cleanup(func() { smbAdminCommandRunner = oldRunner })

	var calls [][]string
	smbAdminCommandRunner = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return []byte("ok"), nil
	}
	app := NewApp()
	if err := app.SetSmbAdminEnabled(true, "Wi-Fi", "192.168.1.11", "192.168.1.20,10.10.10.11"); err != nil {
		t.Fatalf("SetSmbAdminEnabled(true): %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("helper call count = %d, want 1", len(calls))
	}
	got := calls[0]
	for _, pair := range [][]string{
		{"-Action", "Enable"},
		{"-InterfaceAlias", "Wi-Fi"},
		{"-LocalAddress", "192.168.1.11"},
		{"-RemoteAddress", "192.168.1.20"},
	} {
		if !containsSequence(got, pair...) {
			t.Fatalf("helper args %#v do not contain %q", got, pair)
		}
	}
	if !containsArg(got, "10.10.10.11") || !containsArg(got, "-WaitForElevation") {
		t.Fatalf("helper args %#v omit second allowlist entry or UAC wait", got)
	}

	calls = nil
	if err := app.SetSmbAdminEnabled(false, "", "", ""); err != nil {
		t.Fatalf("SetSmbAdminEnabled(false): %v", err)
	}
	if len(calls) != 1 || !containsSequence(calls[0], "-Action", "Disable") || !containsArg(calls[0], "-WaitForElevation") {
		t.Fatalf("disable helper args = %#v", calls)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func containsSequence(args []string, sequence ...string) bool {
	for start := 0; start+len(sequence) <= len(args); start++ {
		if reflect.DeepEqual(args[start:start+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func TestSplitRemoteAddressesDeduplicatesAndAcceptsWhitespace(t *testing.T) {
	got := splitRemoteAddresses("192.168.1.20, 10.10.10.11\n192.168.1.20")
	want := []string{"192.168.1.20", "10.10.10.11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitRemoteAddresses = %#v, want %#v", got, want)
	}
	if strings.Contains(strings.Join(got, ","), " ") {
		t.Fatalf("splitRemoteAddresses retained whitespace: %#v", got)
	}
}
