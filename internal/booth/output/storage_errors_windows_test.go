//go:build windows

package output

import "syscall"

func diskFullCauseForTest() error { return syscall.Errno(112) }
