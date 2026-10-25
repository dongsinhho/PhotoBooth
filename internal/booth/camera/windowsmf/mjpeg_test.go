package windowsmf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func encodeFixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 96, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 96; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 3), B: uint8((x + y) % 255), A: 255})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func stripDHTSegments(t *testing.T, source []byte) []byte {
	t.Helper()
	if len(source) < 4 || source[0] != 0xff || source[1] != 0xd8 {
		t.Fatal("fixture is not a JPEG")
	}
	out := append([]byte(nil), source[:2]...)
	i := 2
	for i < len(source) {
		if source[i] != 0xff {
			t.Fatalf("invalid test JPEG marker prefix at %d", i)
		}
		start := i
		for i < len(source) && source[i] == 0xff {
			i++
		}
		if i >= len(source) {
			t.Fatal("truncated marker")
		}
		marker := source[i]
		i++
		if marker == 0xda {
			out = append(out, source[start:]...)
			return out
		}
		if marker == 0xd8 || marker == 0xd9 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			out = append(out, source[start:i]...)
			continue
		}
		if i+2 > len(source) {
			t.Fatal("truncated segment length")
		}
		length := int(source[i])<<8 | int(source[i+1])
		if length < 2 || i+length > len(source) {
			t.Fatalf("invalid segment length %d", length)
		}
		end := i + length
		if marker != 0xc4 {
			out = append(out, source[start:end]...)
		}
		i = end
	}
	t.Fatal("JPEG scan marker missing from fixture")
	return nil
}

func TestNormalizeMJPEGAddsMissingHuffmanTables(t *testing.T) {
	full := encodeFixture(t)
	abbreviated := stripDHTSegments(t, full)
	if bytes.Contains(abbreviated, []byte{0xff, 0xc4}) {
		t.Fatal("fixture should not contain Huffman table markers")
	}
	normalized, err := normalizeMJPEG(abbreviated)
	if err != nil {
		t.Fatalf("normalizeMJPEG: %v", err)
	}
	if !bytes.Contains(normalized, []byte{0xff, 0xc4}) {
		t.Fatal("normalized JPEG is missing Huffman table markers")
	}
	decoded, err := jpeg.Decode(bytes.NewReader(normalized))
	if err != nil {
		t.Fatalf("decoder rejected normalized JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 96 || decoded.Bounds().Dy() != 64 {
		t.Fatalf("got bounds %v; want 96x64", decoded.Bounds())
	}
}

func TestNormalizeMJPEGLeavesCompleteJPEGUnchanged(t *testing.T) {
	full := encodeFixture(t)
	normalized, err := normalizeMJPEG(full)
	if err != nil {
		t.Fatalf("normalizeMJPEG: %v", err)
	}
	if !bytes.Equal(normalized, full) {
		t.Fatal("complete JPEG should remain byte-for-byte unchanged")
	}
}

func TestNormalizeMJPEGRejectsMalformedInput(t *testing.T) {
	for _, input := range [][]byte{nil, {0x01, 0x02, 0x03}, {0xff, 0xd8, 0x00, 0x01}} {
		if _, err := normalizeMJPEG(input); err == nil {
			t.Fatalf("expected malformed JPEG %x to fail", input)
		}
	}
}
