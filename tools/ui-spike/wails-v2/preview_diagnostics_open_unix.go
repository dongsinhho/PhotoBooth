//go:build !windows

package main

import "os"

// openPreviewDiagnostics opens the append-only diagnostic file on Unix-like
// hosts. The desktop soak currently targets Windows; this keeps diagnostics
// available to future Wails/Linux test builds without changing file semantics.
func openPreviewDiagnostics(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
