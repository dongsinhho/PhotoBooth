package render

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func BenchmarkRenderPhotoStrip1200x1800(b *testing.B) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	configPath := filepath.Join(repoRoot, "examples", "frames", "photo-strip-v1.json")
	if _, err := os.Stat(configPath); err != nil {
		b.Fatalf("sample frame config: %v", err)
	}

	fixtureDir := b.TempDir()
	inputs := []string{
		filepath.Join(fixtureDir, "one.jpg"),
		filepath.Join(fixtureDir, "two.jpg"),
		filepath.Join(fixtureDir, "three.jpg"),
	}
	for i, path := range inputs {
		img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
		for y := 0; y < 720; y++ {
			for x := 0; x < 1280; x++ {
				r := uint8((x*200/1280 + (x*y+i*23)%31) % 256)
				g := uint8((y*200/720 + (x*3+y*5+i*17)%29) % 256)
				blue := uint8((90 + (x/16+y/12+i*11)%90))
				if x%160 < 3 || y%100 < 3 {
					r, g, blue = 245, 245, 245
				}
				img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: blue, A: 255})
			}
		}
		f, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}
		if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
			_ = f.Close()
			b.Fatal(err)
		}
		if err := f.Close(); err != nil {
			b.Fatal(err)
		}
	}
	var sourceBytes int64
	for _, path := range inputs {
		info, err := os.Stat(path)
		if err != nil {
			b.Fatal(err)
		}
		sourceBytes += info.Size()
	}
	b.SetBytes(sourceBytes)
	b.ReportAllocs()
	b.ResetTimer()

	renderer := New()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		out := filepath.Join(fixtureDir, "rendered.jpg")
		if err := renderer.Render(ctx, configPath, inputs, out); err != nil {
			b.Fatal(err)
		}
		if err := os.Remove(out); err != nil {
			b.Fatal(err)
		}
	}
}
