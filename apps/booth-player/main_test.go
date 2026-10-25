package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFrameConfigExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveFrameConfig(path)
	if err != nil {
		t.Fatalf("resolveFrameConfig: %v", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q, want %q", got, abs)
	}
}

func TestResolveFrameConfigRejectsMissingExplicitPath(t *testing.T) {
	_, err := resolveFrameConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected missing frame path to fail")
	}
}
