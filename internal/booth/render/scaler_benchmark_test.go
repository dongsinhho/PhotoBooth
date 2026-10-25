package render

import (
	"image"
	"image/color"
	imagedraw "image/draw"
	"testing"

	xdraw "golang.org/x/image/draw"
)

func BenchmarkFitImageScalers(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for y := 0; y < 720; y++ {
		for x := 0; x < 1280; x++ {
			src.SetRGBA(x, y, colorPattern(x, y))
		}
	}
	bounds := image.Rect(0, 0, 800, 450)
	scalers := []struct {
		name   string
		scaler xdraw.Scaler
	}{
		{name: "CatmullRom", scaler: xdraw.CatmullRom},
		{name: "BiLinear", scaler: xdraw.BiLinear},
		{name: "ApproxBiLinear", scaler: xdraw.ApproxBiLinear},
	}
	for _, item := range scalers {
		b.Run(item.name, func(b *testing.B) {
			dst := image.NewRGBA(bounds)
			b.ReportAllocs()
			b.SetBytes(int64(bounds.Dx() * bounds.Dy() * 4))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				item.scaler.Scale(dst, dst.Bounds(), src, src.Bounds(), imagedraw.Src, nil)
			}
		})
	}
}

func colorPattern(x, y int) color.RGBA {
	return color.RGBA{
		R: uint8((x * 17) % 256),
		G: uint8((y * 23) % 256),
		B: uint8(((x + y) * 13) % 256),
		A: 255,
	}
}
