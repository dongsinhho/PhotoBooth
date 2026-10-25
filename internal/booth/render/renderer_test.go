package render

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	xdraw "golang.org/x/image/draw"
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

func TestRenderHandlesExtremeAspectRatioWithoutHugeIntermediate(t *testing.T) {
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
	out := filepath.Join(dir, "out.jpg")
	if err := New().Render(context.Background(), cfgPath, inputs, out); err != nil {
		t.Fatalf("Render should crop directly into the target without a huge intermediate: %v", err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode output config: %v", err)
	}
	if cfg.Width != 320 || cfg.Height != 480 {
		t.Fatalf("got %dx%d, want 320x480", cfg.Width, cfg.Height)
	}
}

func TestFitImageCoverCropsCenterOfSource(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 100; x++ {
			switch {
			case x < 30:
				src.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			case x >= 70:
				src.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
			default:
				src.SetRGBA(x, y, color.RGBA{G: 255, A: 255})
			}
		}
	}
	fitted, err := fitImage(src, 20, 20, "cover", color.Black)
	if err != nil {
		t.Fatalf("fitImage: %v", err)
	}
	center := fitted.RGBAAt(10, 10)
	if center.G < 200 || center.R > 40 || center.B > 40 {
		t.Fatalf("center pixel=%+v, want center crop's green region", center)
	}
}

func TestDownscaleApproxBiLinearRemainsCloseToCatmullRom(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			r := uint8(x * 255 / src.Bounds().Dx())
			g := uint8(y * 255 / src.Bounds().Dy())
			b := uint8((x + y) * 255 / (src.Bounds().Dx() + src.Bounds().Dy()))
			if x%40 < 2 || y%30 < 2 {
				r, g, b = 250, 250, 250
			}
			src.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	got := image.NewRGBA(image.Rect(0, 0, 160, 90))
	reference := image.NewRGBA(got.Bounds())
	scaleInto(got, got.Bounds(), src, src.Bounds())
	xdraw.CatmullRom.Scale(reference, reference.Bounds(), src, src.Bounds(), imagedraw.Src, nil)

	var total uint64
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			a, b := got.RGBAAt(x, y), reference.RGBAAt(x, y)
			for _, pair := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
				diff := int(pair[0]) - int(pair[1])
				if diff < 0 {
					diff = -diff
				}
				total += uint64(diff)
			}
		}
	}
	meanAbsoluteDifference := float64(total) / float64(got.Bounds().Dx()*got.Bounds().Dy()*3)
	if meanAbsoluteDifference > 8 {
		t.Fatalf("downscale differs too much from Catmull-Rom reference: MAE=%.2f (limit 8)", meanAbsoluteDifference)
	}
}
