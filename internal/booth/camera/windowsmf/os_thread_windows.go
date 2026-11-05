//go:build windows

package windowsmf

import "syscall"

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var getCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")

func currentOSThreadID() uint32 {
	r, _, _ := getCurrentThreadID.Call()
	return uint32(r)
}
