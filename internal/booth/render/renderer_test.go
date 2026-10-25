package render

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	all := append([]string{root}, parts...)
	return filepath.Join(all...)
}

func writeTestConfig(t *testing.T, cfg Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "frame.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validTestConfig() Config {
	return Config{
		SchemaVersion: 1,
		Canvas:        CanvasConfig{Width: 320, Height: 480, Background: "#FFFFFF"},
		Slots: []SlotConfig{
			{ID: "one", X: 20, Y: 30, Width: 280, Height: 120, Fit: "cover"},
			{ID: "two", X: 20, Y: 170, Width: 280, Height: 120, Fit: "contain", Background: "#F0F0F0"},
			{ID: "three", X: 20, Y: 310, Width: 280, Height: 120, Fit: "cover"},
		},
		Text: []TextConfig{{Value: "PHOTO", X: 20, Y: 22, Scale: 0.5, Color: "#222222", Thickness: 1}},
	}
}

func createTestImage(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSampleFrameConfig(t *testing.T) {
	path := repoPath(t, "examples", "frames", "photo-strip-v1.json")
	if err := New().Validate(context.Background(), path); err != nil {
		t.Fatalf("sample frame rejected: %v", err)
	}
}

func TestRejectUnknownConfigField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unknown.json")
	data := []byte(`{"schema_version":1,"unknown":true,"canvas":{"width":320,"height":480,"background":"#FFFFFF"},"slots":[{"id":"a","x":0,"y":0,"width":320,"height":480}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(context.Background(), path); err == nil {
		t.Fatal("expected unknown property to be rejected")
	}
}

func TestRejectCanvasAndSlotBounds(t *testing.T) {
	cfg := validTestConfig()
	cfg.Canvas.Width = 5000
	path := writeTestConfig(t, cfg)
	if _, err := LoadConfig(context.Background(), path); err == nil {
		t.Fatal("expected oversize canvas to be rejected")
	}

	cfg = validTestConfig()
	cfg.Slots[0].X = 300
	cfg.Slots[0].Width = 280
	path = writeTestConfig(t, cfg)
	if _, err := LoadConfig(context.Background(), path); err == nil {
		t.Fatal("expected out-of-canvas slot to be rejected")
	}
}

func TestRenderThreeImageFrameProducesExpectedJPEG(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, validTestConfig())
	inputs := []string{
		filepath.Join(dir, "one.jpg"), filepath.Join(dir, "two.jpg"), filepath.Join(dir, "three.jpg"),
	}
	createTestImage(t, inputs[0], 640, 480, color.RGBA{R: 220, A: 255})
	createTestImage(t, inputs[1], 400, 800, color.RGBA{G: 200, A: 255})
	createTestImage(t, inputs[2], 800, 400, color.RGBA{B: 210, A: 255})
	out := filepath.Join(dir, "nested", "rendered.jpg")
	if err := New().Render(context.Background(), cfgPath, inputs, out); err != nil {
		t.Fatalf("Render: %v", err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if cfg.Width != 320 || cfg.Height != 480 {
		t.Fatalf("got %dx%d, want 320x480", cfg.Width, cfg.Height)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output is empty")
	}
}

func TestRenderRejectsWrongImageCount(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, validTestConfig())
	input := filepath.Join(dir, "one.jpg")
	createTestImage(t, input, 320, 240, color.RGBA{R: 120, A: 255})
	err := New().Render(context.Background(), cfgPath, []string{input}, filepath.Join(dir, "out.jpg"))
	if err == nil {
		t.Fatal("expected image/slot count mismatch")
	}
}

func TestRenderRejectsExtremeAspectRatioBeforeHugeAllocation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, validTestConfig())
	inputs := []string{
		filepath.Join(dir, "extreme.jpg"),
		filepath.Join(dir, "normal-2.jpg"),
		filepath.Join(dir, "normal-3.jpg"),
	}
	createTestImage(t, inputs[0], 8000, 1, color.RGBA{R: 180, A: 255})
	createTestImage(t, inputs[1], 640, 480, color.RGBA{G: 180, A: 255})
	createTestImage(t, inputs[2], 640, 480, color.RGBA{B: 180, A: 255})
	if err := New().Render(context.Background(), cfgPath, inputs, filepath.Join(dir, "out.jpg")); err == nil {
		t.Fatal("expected pathological resized dimensions to be rejected")
	}
}
