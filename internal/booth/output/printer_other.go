//go:build !windows

package output

import (
	"context"
	"errors"
)

func printJPEGToWindowsQueue(context.Context, string, string, string, []byte, int, int, bool, string) (string, error) {
	return "", errors.New("Windows printer queues are supported only on Windows")
}
