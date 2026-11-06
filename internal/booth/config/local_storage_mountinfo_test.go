package config

import (
	"strings"
	"testing"
)

func TestParseMountInfoHandlesOptionalFieldsAndEscapedMountpoints(t *testing.T) {
	input := strings.Join([]string{
		"25 1 0:22 / / rw,relatime shared:1 - ext4 /dev/root rw",
		"26 25 0:42 / /mnt/photo\\040archive rw,relatime master:1 - nfs4 server:/exports/photos rw",
	}, "\n")
	entries, err := parseMountInfo(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d mount entries, want 2", len(entries))
	}
	if entries[1].MountPoint != "/mnt/photo archive" || entries[1].FSType != "nfs4" || entries[1].Source != "server:/exports/photos" {
		t.Fatalf("unexpected parsed network mount: %+v", entries[1])
	}
}

func TestMountForPathUsesLongestMountpointBoundary(t *testing.T) {
	entries := []mountInfoEntry{
		{MountPoint: "/", FSType: "ext4"},
		{MountPoint: "/mnt", FSType: "ext4"},
		{MountPoint: "/mnt/photos", FSType: "nfs4"},
	}
	got, err := mountForPath("/mnt/photos/session-1", entries)
	if err != nil {
		t.Fatal(err)
	}
	if got.FSType != "nfs4" {
		t.Fatalf("selected filesystem %q, want nfs4", got.FSType)
	}
	got, err = mountForPath("/mnt/photos-old/session-1", entries)
	if err != nil {
		t.Fatal(err)
	}
	if got.FSType != "ext4" {
		t.Fatalf("prefix collision selected %q, want ext4", got.FSType)
	}
}

func TestIsNetworkFilesystem(t *testing.T) {
	tests := []struct {
		fsType string
		source string
		want   bool
	}{
		{"ext4", "/dev/sda1", false},
		{"xfs", "/dev/mapper/data", false},
		{"nfs4", "nas:/photos", true},
		{"cifs", "//nas/photos", true},
		{"fuse.sshfs", "user@nas:/photos", true},
		{"fuse.rclone", "remote:bucket", true},
		{"ext4", "//nas/photos", true},
	}
	for _, tt := range tests {
		if got := isNetworkFilesystem(tt.fsType, tt.source); got != tt.want {
			t.Errorf("isNetworkFilesystem(%q, %q)=%v, want %v", tt.fsType, tt.source, got, tt.want)
		}
	}
}

func TestParseMountInfoRejectsMalformedRecord(t *testing.T) {
	if _, err := parseMountInfo(strings.NewReader("not a mountinfo record")); err == nil {
		t.Fatal("expected malformed mountinfo record to fail")
	}
}
