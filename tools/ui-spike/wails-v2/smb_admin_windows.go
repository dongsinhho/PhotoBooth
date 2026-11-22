//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var smbAdminCommandRunner = func(ctx context.Context, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	return cmd.CombinedOutput()
}

func locateSMBAdminHelper() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PHOTOBOOTH_SMB_HELPER_PATH")); override != "" {
		info, err := os.Stat(override)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("configured SMB administration helper is unavailable: %s", override)
		}
		return filepath.Abs(override)
	}

	candidates := make([]string, 0, 16)
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}

	seen := map[string]bool{}
	for _, start := range candidates {
		current := filepath.Clean(start)
		for depth := 0; depth < 10; depth++ {
			if seen[current] {
				break
			}
			seen[current] = true
			candidate := filepath.Join(current, "manage-smb-remote-admin.ps1")
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				return filepath.Abs(candidate)
			}
			candidate = filepath.Join(current, "tools", "self-service", "manage-smb-remote-admin.ps1")
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				return filepath.Abs(candidate)
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return "", errors.New("SMB administration helper was not found beside PhotoBooth or in the source checkout; reinstall the Windows package")
}

func powershellExecutable() string {
	if root := strings.TrimSpace(os.Getenv("SystemRoot")); root != "" {
		candidate := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "powershell.exe"
}

func runSMBAdminHelper(ctx context.Context, action string, extraArgs ...string) ([]byte, error) {
	helper, err := locateSMBAdminHelper()
	if err != nil {
		return nil, err
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", helper, "-Action", action}
	args = append(args, extraArgs...)
	output, err := smbAdminCommandRunner(ctx, powershellExecutable(), args...)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return output, fmt.Errorf("Windows SMB administration %s failed: %s", action, message)
	}
	return output, nil
}

// GetSmbAdminStatus reads the real Windows machine state; no status is inferred
// from application preferences or station configuration.
func (a *App) GetSmbAdminStatus() (SMBAdminStatus, error) {
	if err := a.requireOperator(); err != nil {
		return SMBAdminStatus{}, err
	}
	output, err := runSMBAdminHelper(a.smbAdminContext(), "Status", "-JsonOutput")
	if err != nil {
		return SMBAdminStatus{}, err
	}
	var status SMBAdminStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return SMBAdminStatus{}, fmt.Errorf("parse SMB administration status: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	if status.ServiceState == "" {
		return SMBAdminStatus{}, errors.New("SMB administration helper returned an empty service state")
	}
	return status, nil
}

// GetSmbAdminInterfaces returns only active physical adapters and their IPv4 addresses.
func (a *App) GetSmbAdminInterfaces() ([]SMBAdminInterface, error) {
	if err := a.requireOperator(); err != nil {
		return nil, err
	}
	output, err := runSMBAdminHelper(a.smbAdminContext(), "ListInterfaces", "-JsonOutput")
	if err != nil {
		return nil, err
	}
	var items []SMBAdminInterface
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, fmt.Errorf("parse physical network interfaces: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return items, nil
}

// SetSmbAdminEnabled delegates all machine-wide changes to the UAC-aware helper.
// The Go process never edits Windows services, registry keys or firewall rules.
func (a *App) SetSmbAdminEnabled(enabled bool, interfaceAlias, localAddress, remoteAddressText string) error {
	if err := a.requireOperator(); err != nil {
		return err
	}
	ctx := a.smbAdminContext()
	if !enabled {
		_, err := runSMBAdminHelper(ctx, "Disable", "-WaitForElevation")
		return err
	}

	args, err := buildSMBAdminEnableArgs(interfaceAlias, localAddress, remoteAddressText)
	if err != nil {
		return err
	}
	args = append(args, "-WaitForElevation")
	_, err = runSMBAdminHelper(ctx, "Enable", args...)
	return err
}

func buildSMBAdminEnableArgs(interfaceAlias, localAddress, remoteAddressText string) ([]string, error) {
	interfaceAlias = strings.TrimSpace(interfaceAlias)
	localAddress = strings.TrimSpace(localAddress)
	if interfaceAlias == "" {
		return nil, errors.New("select a physical network adapter")
	}
	if err := validatePrivateIPv4(localAddress); err != nil {
		return nil, fmt.Errorf("local address: %w", err)
	}
	remoteAddresses := splitRemoteAddresses(remoteAddressText)
	if len(remoteAddresses) == 0 {
		return nil, errors.New("enter at least one trusted remote IPv4 address")
	}
	for _, address := range remoteAddresses {
		if err := validatePrivateIPv4(address); err != nil {
			return nil, fmt.Errorf("remote address %q: %w", address, err)
		}
		if address == localAddress {
			return nil, errors.New("a remote source address cannot equal the booth's local address")
		}
	}

	args := []string{
		"-InterfaceAlias", interfaceAlias,
		"-LocalAddress", localAddress,
		"-RemoteAddress",
	}
	args = append(args, remoteAddresses...)
	return args, nil
}

func (a *App) smbAdminContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func splitRemoteAddresses(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	seen := make(map[string]bool, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		address := strings.TrimSpace(part)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		result = append(result, address)
	}
	return result
}

func validatePrivateIPv4(value string) error {
	value = strings.TrimSpace(value)
	// Do not accept IPv4-mapped IPv6 spellings; the PowerShell contract and
	// firewall UI require an explicit dotted-quad IPv4 literal.
	if strings.Contains(value, ":") || strings.Count(value, ".") != 3 {
		return errors.New("must be a literal IPv4 address")
	}
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		return errors.New("must be a literal IPv4 address")
	}
	if !ip.IsPrivate() {
		return errors.New("must be an RFC1918 private IPv4 address")
	}
	return nil
}
