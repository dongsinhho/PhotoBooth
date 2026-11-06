package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type mountInfoEntry struct {
	MountPoint string
	FSType     string
	Source     string
}

// parseMountInfo parses Linux /proc/self/mountinfo. The optional fields are
// variable length, so the separator ("-") rather than a fixed index identifies
// filesystem type and source.
func parseMountInfo(reader io.Reader) ([]mountInfoEntry, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var entries []mountInfoEntry
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := strings.Fields(scanner.Text())
		separator := -1
		for i, field := range fields {
			if field == "-" {
				separator = i
				break
			}
		}
		if len(fields) < 6 || separator < 6 || len(fields) < separator+3 {
			return nil, fmt.Errorf("invalid mountinfo record on line %d", lineNumber)
		}
		entries = append(entries, mountInfoEntry{
			MountPoint: unescapeMountInfo(fields[4]),
			FSType:     fields[separator+1],
			Source:     unescapeMountInfo(fields[separator+2]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read mountinfo: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("mountinfo contains no mount entries")
	}
	return entries, nil
}

// Linux mountinfo escapes space, tab, newline and backslash as octal sequences.
func unescapeMountInfo(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '\\' && i+3 < len(value) {
			if code, err := strconv.ParseUint(value[i+1:i+4], 8, 8); err == nil {
				builder.WriteByte(byte(code))
				i += 4
				continue
			}
		}
		builder.WriteByte(value[i])
		i++
	}
	return builder.String()
}

func mountForPath(path string, entries []mountInfoEntry) (mountInfoEntry, error) {
	bestLength := -1
	var best mountInfoEntry
	for _, entry := range entries {
		mountPoint := entry.MountPoint
		if mountPoint == "" {
			continue
		}
		matches := path == mountPoint || mountPoint == "/" || strings.HasPrefix(path, strings.TrimRight(mountPoint, "/")+"/")
		if matches && len(mountPoint) > bestLength {
			best, bestLength = entry, len(mountPoint)
		}
	}
	if bestLength < 0 {
		return mountInfoEntry{}, fmt.Errorf("no mountinfo entry covers storage path %q", path)
	}
	return best, nil
}

func isNetworkFilesystem(fsType, source string) bool {
	fsType = strings.ToLower(strings.TrimSpace(fsType))
	switch fsType {
	case "nfs", "nfs4", "cifs", "smb3", "smbfs", "afpfs", "9p",
		"ceph", "glusterfs", "lustre", "afs", "coda", "davfs", "davfs2",
		"sshfs", "fuse.sshfs", "fuse.rclone", "fuse.s3fs", "fuse.gcsfuse",
		"fuse.goofys", "fuse.davfs", "fuse.ceph":
		return true
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if strings.HasPrefix(source, "//") || strings.HasPrefix(source, "\\\\") {
		return true
	}
	return false
}
