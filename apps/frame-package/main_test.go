package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testFrameJSON = `{"schema_version":1,"name":"CLI fixture","canvas":{"width":320,"height":640,"background":"#FFFFFF"},"slots":[{"id":"photo-1","x":0,"y":0,"width":320,"height":640,"fit":"cover"}],"jpeg_quality":90}`

func TestRunCreateValidateInstallAndList(t *testing.T) {
	dir := t.TempDir()
	framePath := filepath.Join(dir, "frame.json")
	if err := os.WriteFile(framePath, []byte(testFrameJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "fixture-1.0.0.pbframe")
	var out, stderr bytes.Buffer

	err := run([]string{"create", "--frame", framePath, "--out", archive, "--id", "cli-fixture", "--version", "1.0.0", "--name", "CLI Fixture", "--author", "Test", "--license", "CC0-1.0"}, &out, &stderr)
	if err != nil {
		t.Fatalf("create: %v (stderr %s)", err, stderr.String())
	}
	if !strings.Contains(out.String(), `"package_id": "cli-fixture"`) {
		t.Fatalf("unexpected create output: %s", out.String())
	}

	out.Reset()
	if err := run([]string{"validate", "--package", archive}, &out, &stderr); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(out.String(), `"slot_count": 1`) {
		t.Fatalf("unexpected validate output: %s", out.String())
	}

	managedRoot := filepath.Join(dir, "local-frames")
	out.Reset()
	if err := run([]string{"install", "--package", archive, "--frames-dir", managedRoot}, &out, &stderr); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(out.String(), `"frame_path":`) {
		t.Fatalf("unexpected install output: %s", out.String())
	}

	out.Reset()
	if err := run([]string{"list", "--frames-dir", managedRoot}, &out, &stderr); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), `"package_version": "1.0.0"`) {
		t.Fatalf("unexpected list output: %s", out.String())
	}
}

func TestRunRejectsUnknownCommandAndMissingSubcommand(t *testing.T) {
	var out, stderr bytes.Buffer
	if err := run(nil, &out, &stderr); err == nil || !strings.Contains(err.Error(), "command required") {
		t.Fatalf("run empty error=%v", err)
	}
	if err := run([]string{"delete", "--everything"}, &out, &stderr); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unknown command error=%v", err)
	}
}
