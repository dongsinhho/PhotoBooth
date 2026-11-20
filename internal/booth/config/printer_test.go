package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigratesSchemaV1ToV2WithoutRewritingSource(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "legacy-v1.json", `{
		"schema_version": 1,
		"capture": {"shots": 4, "countdown": "2s"}
	}`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load legacy v1: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("loaded schema version=%d, want %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	if cfg.Capture.Shots != 4 || cfg.Capture.Countdown != "2s" {
		t.Fatalf("v1 fields were not preserved: %+v", cfg.Capture)
	}
	if cfg.Printer.Enabled || cfg.Printer.QueueName != "" || cfg.Printer.MediaForm != "" ||
		cfg.Printer.Orientation != PrinterOrientationPortrait || cfg.Printer.Transform.FlipVertical {
		t.Fatalf("legacy v1 migration did not disable printing safely: %+v", cfg.Printer)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("Load must not rewrite the legacy source file: before=%s after=%s", before, after)
	}

	// An explicit Save is the migration commit point.
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save migrated config: %v", err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(persisted), `"schema_version": 2`) ||
		!strings.Contains(string(persisted), `"printer"`) ||
		!strings.Contains(string(persisted), `"enabled": false`) {
		t.Fatalf("explicit save did not persist the v2 disabled printer policy: %s", persisted)
	}
}

func TestLoadSchemaV2UsesSafePrinterDefaultsWhenSectionIsOmitted(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "partial-v2.json", `{"schema_version":2}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion || cfg.Printer.Enabled ||
		cfg.Printer.Orientation != PrinterOrientationPortrait ||
		cfg.Printer.QueueName != "" || cfg.Printer.MediaForm != "" ||
		cfg.Printer.Transform.FlipVertical {
		t.Fatalf("schema-v2 omitted printer section did not inherit safe defaults: %+v", cfg.Printer)
	}
}

func TestPrinterSettingsValidateDisabledAndEnabledProfiles(t *testing.T) {
	// An unconfigured printer is a valid profile while disabled.
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled/unconfigured printer must remain valid: %v", err)
	}

	cfg.Printer = PrinterSettings{
		Enabled:     true,
		QueueName:   "DS-RX1",
		MediaForm:   "6x4",
		Orientation: PrinterOrientationLandscape,
		Transform:   PrinterTransformSettings{FlipVertical: true},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("complete enabled printer profile should validate: %v", err)
	}

	tests := []struct {
		name string
		edit func(*PrinterSettings)
		want string
	}{
		{"missing queue", func(p *PrinterSettings) { p.QueueName = "" }, "printer.queue_name is required"},
		{"missing media form", func(p *PrinterSettings) { p.MediaForm = "" }, "printer.media_form is required"},
		{"queue whitespace", func(p *PrinterSettings) { p.QueueName = " DS-RX1 " }, "printer.queue_name"},
		{"queue newline", func(p *PrinterSettings) { p.QueueName = "DS-RX1\nOther" }, "printer.queue_name"},
		{"queue tab", func(p *PrinterSettings) { p.QueueName = "DS-RX1\t" }, "printer.queue_name"},
		{"queue too long", func(p *PrinterSettings) { p.QueueName = strings.Repeat("Q", 257) }, "printer.queue_name"},
		{"form whitespace", func(p *PrinterSettings) { p.MediaForm = " 6x4" }, "printer.media_form"},
		{"form newline", func(p *PrinterSettings) { p.MediaForm = "6x4\r" }, "printer.media_form"},
		{"form too long", func(p *PrinterSettings) { p.MediaForm = strings.Repeat("F", 129) }, "printer.media_form"},
		{"form exceeds DEVMODE UTF-16 field", func(p *PrinterSettings) { p.MediaForm = strings.Repeat("F", 32) }, "31 UTF-16 code units"},
		{"invalid orientation", func(p *PrinterSettings) { p.Orientation = "auto" }, "printer.orientation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := cfg.Printer
			tt.edit(&candidate)
			copy := cfg
			copy.Printer = candidate
			if err := copy.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error=%v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestPrinterSettingsRoundTripAndRemainDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store-v2.json")
	cfg := Defaults()
	cfg.Storage.SessionsDir = filepath.Join(dir, "sessions")
	cfg.Storage.OutputDir = filepath.Join(dir, "output")
	cfg.Printer = PrinterSettings{
		Enabled:     true,
		QueueName:   "DS-RX1",
		MediaForm:   "6x4",
		Orientation: PrinterOrientationLandscape,
		Transform:   PrinterTransformSettings{FlipVertical: true},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save enabled printer config: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load enabled printer config: %v", err)
	}
	if got.Printer != cfg.Printer {
		t.Fatalf("printer policy did not round trip: got=%+v want=%+v", got.Printer, cfg.Printer)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema_version": 2`) {
		t.Fatalf("saved printer profile did not use config schema v2: %s", data)
	}

	disabled := Defaults()
	disabled.Storage.SessionsDir = filepath.Join(dir, "disabled-sessions")
	disabled.Storage.OutputDir = filepath.Join(dir, "disabled-output")
	if err := Save(filepath.Join(dir, "disabled.json"), disabled); err != nil {
		t.Fatal(err)
	}
	disabledData, err := os.ReadFile(filepath.Join(dir, "disabled.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(disabledData), `"enabled": false`) {
		t.Fatalf("new profile must persist printer.enabled=false: %s", disabledData)
	}
}

func TestLoadRejectsPrinterSectionInLegacySchemaV1(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "invalid-v1.json", `{
		"schema_version": 1,
		"printer": {"enabled": false}
	}`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "schema_version 1 cannot contain printer settings") {
		t.Fatalf("Load() error=%v, want schema-v1 printer section rejection", err)
	}
}

func TestCanonicalStoreConfigV2ExampleLoadsWithPrintingDisabled(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "store-config-v2.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load canonical schema-v2 example: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion || cfg.Camera.Provider != "fake" ||
		cfg.MediaDelivery.Mode != MediaDeliveryModeLocal || cfg.Payment.Mode != PaymentModeDisabled {
		t.Fatalf("canonical example policy mismatch: %+v", cfg)
	}
	if cfg.Printer.Enabled || cfg.Printer.QueueName != "" || cfg.Printer.MediaForm != "" ||
		cfg.Printer.Orientation != PrinterOrientationPortrait || cfg.Printer.Transform.FlipVertical {
		t.Fatalf("canonical example must keep printing disabled and unconfigured: %+v", cfg.Printer)
	}
}
