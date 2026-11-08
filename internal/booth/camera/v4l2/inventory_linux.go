//go:build linux

package v4l2

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Inventory reports /dev/video* nodes and access metadata without opening or
// streaming from any camera device.
func Inventory() InventoryReport {
	groups, _ := syscall.Getgroups()
	groupIDs := make(map[uint32]struct{}, len(groups)+2)
	groupIDs[uint32(os.Getgid())] = struct{}{}
	groupIDs[uint32(os.Getegid())] = struct{}{}
	for _, gid := range groups {
		groupIDs[uint32(gid)] = struct{}{}
	}

	report := inspectPaths("/dev", "/sys/class/video4linux", "/proc/sys/kernel/osrelease", os.Geteuid(), os.Getegid(), groupIDs)
	if current, err := user.Current(); err == nil {
		report.EffectiveUID, _ = strconv.Atoi(current.Uid)
		for _, gid := range groups {
			group, lookupErr := user.LookupGroupId(strconv.Itoa(gid))
			if lookupErr == nil {
				report.Groups = append(report.Groups, group.Name)
			}
		}
	}
	report.OS = runtime.GOOS
	report.Architecture = runtime.GOARCH
	_, driverErr := os.Stat("/sys/module/uvcvideo")
	report.UVCDriverLoaded = driverErr == nil
	if report.Kernel != "" {
		modulePath := filepath.Join("/lib/modules", report.Kernel, "kernel/drivers/media/usb/uvc/uvcvideo.ko")
		_, moduleErr := os.Stat(modulePath)
		report.UVCDriverModuleAvailable = moduleErr == nil
	}
	if videoGroup, err := user.LookupGroup("video"); err == nil {
		if gid, convErr := strconv.ParseUint(videoGroup.Gid, 10, 32); convErr == nil {
			_, report.InVideoGroup = groupIDs[uint32(gid)]
		}
	}
	return report
}

func inspectPaths(devRoot, classRoot, kernelFile string, euid, egid int, groups map[uint32]struct{}) InventoryReport {
	report := InventoryReport{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		EffectiveUID: euid,
		EffectiveGID: egid,
		Devices:      make([]DeviceInventory, 0),
	}
	if kernel, err := os.ReadFile(kernelFile); err == nil {
		report.Kernel = strings.TrimSpace(string(kernel))
	}
	if info, err := os.Stat(classRoot); err == nil && info.IsDir() {
		report.Video4LinuxSysfs = true
	}

	entries, err := os.ReadDir(devRoot)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("cannot read %s: %v", devRoot, err))
		return report
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "video") || !isDecimal(name[len("video"):]) {
			continue
		}
		path := filepath.Join(devRoot, name)
		info, statErr := os.Stat(path)
		if statErr != nil || info.Mode()&os.ModeDevice == 0 || info.Mode()&os.ModeCharDevice == 0 {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		item := DeviceInventory{
			Path:      path,
			OwnerID:   stat.Uid,
			GroupID:   stat.Gid,
			Mode:      fmt.Sprintf("%04o", info.Mode().Perm()),
			ReadWrite: hasReadWriteAccess(info.Mode(), uint32(euid), stat.Uid, groups, stat.Gid),
		}
		if group, lookupErr := user.LookupGroupId(strconv.FormatUint(uint64(stat.Gid), 10)); lookupErr == nil {
			item.GroupName = group.Name
		}
		if label, readErr := os.ReadFile(filepath.Join(classRoot, name, "name")); readErr == nil {
			item.Name = strings.TrimSpace(string(label))
		}
		report.Devices = append(report.Devices, item)
	}
	if len(report.Devices) == 0 {
		report.Warnings = append(report.Warnings, "no V4L2 character devices found under "+filepath.Join(devRoot, "videoN"))
	} else {
		for _, device := range report.Devices {
			if !device.ReadWrite {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s is not readable and writable by the current process; inspect its udev rule and video-group membership", device.Path))
			}
		}
	}
	return report
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// hasReadWriteAccess estimates POSIX DAC access from owner/group/other mode
// bits without opening the device. Filesystem ACLs and Linux capabilities may
// still affect the kernel's final decision.
func hasReadWriteAccess(mode os.FileMode, euid, owner uint32, groups map[uint32]struct{}, group uint32) bool {
	if euid == 0 {
		return true
	}
	permissions := mode.Perm()
	if euid == owner {
		return permissions&0o600 == 0o600
	}
	if _, ok := groups[group]; ok {
		return permissions&0o060 == 0o060
	}
	return permissions&0o006 == 0o006
}
