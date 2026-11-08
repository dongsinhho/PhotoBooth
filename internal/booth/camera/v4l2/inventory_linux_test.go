//go:build linux

package v4l2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectPathsReportsNoDevicesWithoutOpeningAnything(t *testing.T) {
	root := t.TempDir()
	dev := filepath.Join(root, "dev")
	sys := filepath.Join(root, "sys", "class", "video4linux")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sys, 0o755); err != nil {
		t.Fatal(err)
	}
	kernel := filepath.Join(root, "kernel")
	if err := os.WriteFile(kernel, []byte("test-kernel\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := inspectPaths(dev, sys, kernel, 1000, 1000, map[uint32]struct{}{1000: {}})
	if report.Kernel != "test-kernel" {
		t.Fatalf("kernel=%q", report.Kernel)
	}
	if !report.Video4LinuxSysfs {
		t.Fatal("expected sysfs directory to be reported")
	}
	if len(report.Devices) != 0 {
		t.Fatalf("devices=%+v, want none", report.Devices)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("expected missing-device warning")
	}
}

func TestInspectPathsIgnoresNonCharacterVideoFiles(t *testing.T) {
	root := t.TempDir()
	dev := filepath.Join(root, "dev")
	sys := filepath.Join(root, "sys", "class", "video4linux")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sys, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "video0"), []byte("not a device"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "video-not-a-number"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := inspectPaths(dev, sys, filepath.Join(root, "missing-kernel"), 1000, 1000, map[uint32]struct{}{})
	if len(report.Devices) != 0 {
		t.Fatalf("regular files must not be reported as V4L2 devices: %+v", report.Devices)
	}
}

func TestHasReadWriteAccessModeBits(t *testing.T) {
	groups := map[uint32]struct{}{44: {}}
	cases := []struct {
		name  string
		mode  os.FileMode
		euid  uint32
		owner uint32
		gid   uint32
		want  bool
	}{
		{name: "root", mode: 0o000, euid: 0, owner: 1000, gid: 44, want: true},
		{name: "owner", mode: 0o600, euid: 1000, owner: 1000, gid: 44, want: true},
		{name: "owner missing write", mode: 0o400, euid: 1000, owner: 1000, gid: 44, want: false},
		{name: "video group", mode: 0o660, euid: 1000, owner: 0, gid: 44, want: true},
		{name: "video group missing write", mode: 0o640, euid: 1000, owner: 0, gid: 44, want: false},
		{name: "other", mode: 0o006, euid: 1000, owner: 0, gid: 100, want: true},
		{name: "other denied", mode: 0o640, euid: 1000, owner: 0, gid: 100, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasReadWriteAccess(tc.mode, tc.euid, tc.owner, groups, tc.gid); got != tc.want {
				t.Fatalf("hasReadWriteAccess()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsDecimal(t *testing.T) {
	for _, value := range []string{"0", "12", "999"} {
		if !isDecimal(value) {
			t.Fatalf("%q should be decimal", value)
		}
	}
	for _, value := range []string{"", "x", "video0", "-1", "1a"} {
		if isDecimal(value) {
			t.Fatalf("%q should not be decimal", value)
		}
	}
}
