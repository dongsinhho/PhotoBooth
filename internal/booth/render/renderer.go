package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	maxConfigBytes     = 1 << 20
	maxImageBytes      = 40 << 20
	maxInputPixels     = 40_000_000
	maxCanvasSize      = 4000
	maxCanvasPixels    = 16_000_000
	maxFrameSlots      = 12
	maxTextItems       = 40
	maxTextLength      = 500
	defaultJPEGQuality = 92
)

// Config is the version-1 declarative frame format. It cannot contain code or
// remote resources: rendering uses only this geometry and local image inputs.
type Config struct {
	SchemaVersion int          `json:"schema_version"`
	Name          string       `json:"name,omitempty"`
	Canvas        CanvasConfig `json:"canvas"`
	Slots         []SlotConfig `json:"slots"`
	Text          []TextConfig `json:"text,omitempty"`
	JPEGQuality   int          `json:"jpeg_quality,omitempty"`
}

type CanvasConfig struct {
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	Background string        `json:"background"`
	Border     *BorderConfig `json:"border,omitempty"`
}

type SlotConfig struct {
	ID         string        `json:"id"`
	X          int           `json:"x"`
	Y          int           `json:"y"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	Fit        string        `json:"fit,omitempty"`
	Background string        `json:"background,omitempty"`
	Border     *BorderConfig `json:"border,omitempty"`
}

type BorderConfig struct {
	Color     string `json:"color"`
	Thickness int    `json:"thickness"`
	Inset     int    `json:"inset,omitempty"`
}

type TextConfig struct {
	Value     string  `json:"value"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Scale     float64 `json:"scale"`
	Color     string  `json:"color"`
	Thickness int     `json:"thickness,omitempty"`
}

// Renderer provides deterministic composition into a local JPEG file.
type Renderer struct{}

func New() *Renderer { return &Renderer{} }

// Validate performs frame preflight without opening a camera or capturing.
func (r *Renderer) Validate(ctx context.Context, configPath string) error {
	_, err := LoadConfig(ctx, configPath)
	return err
}

// SlotCount validates a frame and reports how many captures it expects.
func (r *Renderer) SlotCount(ctx context.Context, configPath string) (int, error) {
	cfg, err := LoadConfig(ctx, configPath)
	if err != nil {
		return 0, err
	}
	return len(cfg.Slots), nil
}

// LoadConfig rejects unknown JSON properties and resource-heavy frame layouts.
func LoadConfig(ctx context.Context, configPath string) (Config, error) {
	var cfg Config
	if err := ctx.Err(); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(configPath) == "" {
		return cfg, errors.New("frame config path is required")
	}
	info, err := os.Stat(configPath)
	if err != nil {
		return cfg, fmt.Errorf("stat frame config: %w", err)
	}
	if !info.Mode().IsRegular() {
		return cfg, errors.New("frame config must be a regular file")
	}
	if info.Size() > maxConfigBytes {
		return cfg, fmt.Errorf("frame config exceeds %d bytes", maxConfigBytes)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("read frame config: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode frame config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return cfg, errors.New("frame config must contain a single JSON value")
		}
		return cfg, fmt.Errorf("invalid trailing data in frame config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1; got %d", cfg.SchemaVersion)
	}
	w, h := cfg.Canvas.Width, cfg.Canvas.Height
	if w < 64 || w > maxCanvasSize || h < 64 || h > maxCanvasSize {
		return fmt.Errorf("canvas dimensions must each be between 64 and %d pixels", maxCanvasSize)
	}
	if int64(w)*int64(h) > maxCanvasPixels {
		return fmt.Errorf("canvas exceeds %d pixels", maxCanvasPixels)
	}
	if _, err := parseColor(cfg.Canvas.Background, "canvas.background"); err != nil {
		return err
	}
	if err := validateBorder(cfg.Canvas.Border, w, h, "canvas.border", true); err != nil {
		return err
	}
	if len(cfg.Slots) < 1 || len(cfg.Slots) > maxFrameSlots {
		return fmt.Errorf("slots must contain 1 to %d items", maxFrameSlots)
	}
	for i := range cfg.Slots {
		slot := &cfg.Slots[i]
		label := fmt.Sprintf("slots[%d]", i)
		if strings.TrimSpace(slot.ID) == "" {
			return fmt.Errorf("%s.id must be a non-empty string", label)
		}
		if slot.X < 0 || slot.X >= w || slot.Y < 0 || slot.Y >= h {
			return fmt.Errorf("%s origin must be inside the canvas", label)
		}
		if slot.Width < 1 || slot.Width > w || slot.Height < 1 || slot.Height > h {
			return fmt.Errorf("%s width and height must fit the canvas", label)
		}
		if slot.X+slot.Width > w || slot.Y+slot.Height > h {
			return fmt.Errorf("%s extends outside the canvas", label)
		}
		if slot.Fit == "" {
			slot.Fit = "cover"
		}
		if slot.Fit != "cover" && slot.Fit != "contain" {
			return fmt.Errorf("%s.fit must be 'cover' or 'contain'", label)
		}
		if slot.Background != "" {
			if _, err := parseColor(slot.Background, label+".background"); err != nil {
				return err
			}
		}
		if err := validateBorder(slot.Border, slot.Width, slot.Height, label+".border", false); err != nil {
			return err
		}
	}
	if len(cfg.Text) > maxTextItems {
		return fmt.Errorf("text must contain at most %d items", maxTextItems)
	}
	for i, item := range cfg.Text {
		label := fmt.Sprintf("text[%d]", i)
		if len(item.Value) > maxTextLength {
			return fmt.Errorf("%s.value exceeds %d bytes", label, maxTextLength)
		}
		if item.X < 0 || item.X >= w || item.Y < 0 || item.Y >= h {
			return fmt.Errorf("%s position must be inside the canvas", label)
		}
		if item.Scale < 0.1 || item.Scale > 5 || math.IsNaN(item.Scale) || math.IsInf(item.Scale, 0) {
			return fmt.Errorf("%s.scale must be between 0.1 and 5", label)
		}
		if _, err := parseColor(item.Color, label+".color"); err != nil {
			return err
		}
		if item.Thickness == 0 {
			cfg.Text[i].Thickness = 1
		} else if item.Thickness < 1 || item.Thickness > 20 {
			return fmt.Errorf("%s.thickness must be between 1 and 20", label)
		}
	}
	if cfg.JPEGQuality == 0 {
		cfg.JPEGQuality = defaultJPEGQuality
	}
	if cfg.JPEGQuality < 1 || cfg.JPEGQuality > 100 {
		return errors.New("jpeg_quality must be between 1 and 100")
	}
	return nil
}

func validateBorder(border *BorderConfig, width, height int, label string, frameBorder bool) error {
	if border == nil {
		return nil
	}
	if _, err := parseColor(border.Color, label+".color"); err != nil {
		return err
	}
	if border.Thickness < 1 || border.Thickness > 100 {
		return fmt.Errorf("%s.thickness must be between 1 and 100", label)
	}
	if border.Inset < 0 || border.Inset > min(width, height)/2 {
		return fmt.Errorf("%s.inset is outside the allowed range", label)
	}
	if frameBorder && 2*border.Inset+border.Thickness >= min(width, height) {
		return fmt.Errorf("%s does not fit the canvas", label)
	}
	if !frameBorder && border.Thickness*2 >= min(width, height) {
		return fmt.Errorf("%s thickness does not fit the slot", label)
	}
	return nil
}

func parseColor(value, label string) (color.RGBA, error) {
	if len(value) != 7 || value[0] != '#' {
		return color.RGBA{}, fmt.Errorf("%s must be a hex color such as #F5F5F5", label)
	}
	parse := func(s string) (uint8, error) {
		n, err := strconv.ParseUint(s, 16, 8)
		return uint8(n), err
	}
	r, err := parse(value[1:3])
	if err != nil {
		return color.RGBA{}, fmt.Errorf("%s contains invalid hex digits", label)
	}
	g, err := parse(value[3:5])
	if err != nil {
		return color.RGBA{}, fmt.Errorf("%s contains invalid hex digits", label)
	}
	b, err := parse(value[5:7])
	if err != nil {
		return color.RGBA{}, fmt.Errorf("%s contains invalid hex digits", label)
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}, nil
}

// Render composes one local image per slot into a deterministic JPEG.
// Input paths must match slot order; JPEG and PNG inputs are supported.
func (r *Renderer) Render(ctx context.Context, configPath string, inputPaths []string, outputPath string) error {
	cfg, err := LoadConfig(ctx, configPath)
	if err != nil {
		return err
	}
	if len(inputPaths) != len(cfg.Slots) {
		return fmt.Errorf("received %d image(s), but frame has %d slot(s)", len(inputPaths), len(cfg.Slots))
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("render output path is required")
	}
	sources := make([]image.Image, len(inputPaths))
	for i, path := range inputPaths {
		if err := ctx.Err(); err != nil {
			return err
		}
		img, err := decodeImage(path)
		if err != nil {
			return fmt.Errorf("decode input image %d: %w", i+1, err)
		}
		sources[i] = img
	}
	background, _ := parseColor(cfg.Canvas.Background, "canvas.background")
	canvas := image.NewRGBA(image.Rect(0, 0, cfg.Canvas.Width, cfg.Canvas.Height))
	imagedraw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: background}, image.Point{}, imagedraw.Src)
	for i, slot := range cfg.Slots {
		if err := ctx.Err(); err != nil {
			return err
		}
		slotBackground := background
		if slot.Background != "" {
			slotBackground, _ = parseColor(slot.Background, fmt.Sprintf("slots[%d].background", i))
		}
		fitted, err := fitImage(sources[i], slot.Width, slot.Height, slot.Fit, slotBackground)
		if err != nil {
			return fmt.Errorf("fit input image %d: %w", i+1, err)
		}
		rect := image.Rect(slot.X, slot.Y, slot.X+slot.Width, slot.Y+slot.Height)
		imagedraw.Draw(canvas, rect, fitted, fitted.Bounds().Min, imagedraw.Src)
		if slot.Border != nil {
			c, _ := parseColor(slot.Border.Color, fmt.Sprintf("slots[%d].border.color", i))
			drawBorder(canvas, rect, c, slot.Border.Thickness, 0)
		}
	}
	if cfg.Canvas.Border != nil {
		c, _ := parseColor(cfg.Canvas.Border.Color, "canvas.border.color")
		drawBorder(canvas, canvas.Bounds(), c, cfg.Canvas.Border.Thickness, cfg.Canvas.Border.Inset)
	}
	for _, item := range cfg.Text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := drawText(canvas, item); err != nil {
			return fmt.Errorf("draw text: %w", err)
		}
	}
	return writeJPEGAtomic(ctx, outputPath, canvas, cfg.JPEGQuality)
}

func decodeImage(path string) (image.Image, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("input image must be a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxImageBytes {
		return nil, fmt.Errorf("input image must be between 1 byte and %d bytes", maxImageBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	header, _, err := image.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("read image header: %w", err)
	}
	if header.Width < 1 || header.Height < 1 || header.Width > 8000 || header.Height > 8000 || int64(header.Width)*int64(header.Height) > maxInputPixels {
		return nil, fmt.Errorf("input image dimensions %dx%d exceed limits", header.Width, header.Height)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

func fitImage(src image.Image, width, height int, mode string, background color.Color) (*image.RGBA, error) {
	if src == nil || src.Bounds().Empty() {
		return nil, errors.New("source image is empty")
	}
	if width < 1 || height < 1 {
		return nil, errors.New("target slot dimensions must be positive")
	}
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	outBounds := image.Rect(0, 0, width, height)
	out := image.NewRGBA(outBounds)

	switch mode {
	case "cover":
		// Crop the source to the target aspect ratio before scaling. Scaling a
		// potentially larger intermediate image and cropping it afterwards
		// multiplies peak allocations with no visual benefit.
		sourceRect := bounds
		sourceAspect := float64(srcW) / float64(srcH)
		targetAspect := float64(width) / float64(height)
		if sourceAspect > targetAspect {
			cropWidth := max(1, min(srcW, int(math.Round(float64(srcH)*targetAspect))))
			left := bounds.Min.X + (srcW-cropWidth)/2
			sourceRect = image.Rect(left, bounds.Min.Y, left+cropWidth, bounds.Max.Y)
		} else if sourceAspect < targetAspect {
			cropHeight := max(1, min(srcH, int(math.Round(float64(srcW)/targetAspect))))
			top := bounds.Min.Y + (srcH-cropHeight)/2
			sourceRect = image.Rect(bounds.Min.X, top, bounds.Max.X, top+cropHeight)
		}
		scaleInto(out, outBounds, src, sourceRect)
		return out, nil

	case "contain":
		imagedraw.Draw(out, outBounds, &image.Uniform{C: background}, image.Point{}, imagedraw.Src)
		scale := math.Min(float64(width)/float64(srcW), float64(height)/float64(srcH))
		resizedW := max(1, int(math.Round(float64(srcW)*scale)))
		resizedH := max(1, int(math.Round(float64(srcH)*scale)))
		rect := image.Rect((width-resizedW)/2, (height-resizedH)/2,
			(width-resizedW)/2+resizedW, (height-resizedH)/2+resizedH)
		scaleInto(out, rect, src, bounds)
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported fit mode %q", mode)
	}
}

// scaleInto chooses a low-allocation interpolator for downscaling and the
// higher-quality Catmull-Rom kernel for upscaling. PhotoBooth's common UVC
// path reduces 1280x720 captures into smaller frame slots, where ApproxBiLinear
// offers reasonable quality without allocating a large kernel scratch buffer.
func scaleInto(dst *image.RGBA, dstRect image.Rectangle, src image.Image, srcRect image.Rectangle) {
	if dstRect.Dx() <= srcRect.Dx() && dstRect.Dy() <= srcRect.Dy() {
		xdraw.ApproxBiLinear.Scale(dst, dstRect, src, srcRect, imagedraw.Src, nil)
		return
	}
	xdraw.CatmullRom.Scale(dst, dstRect, src, srcRect, imagedraw.Src, nil)
}

func drawBorder(dst *image.RGBA, bounds image.Rectangle, c color.Color, thickness, inset int) {
	rect := bounds.Inset(inset)
	if rect.Empty() {
		return
	}
	strips := []image.Rectangle{
		image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, min(rect.Min.Y+thickness, rect.Max.Y)),
		image.Rect(rect.Min.X, max(rect.Min.Y, rect.Max.Y-thickness), rect.Max.X, rect.Max.Y),
		image.Rect(rect.Min.X, rect.Min.Y, min(rect.Min.X+thickness, rect.Max.X), rect.Max.Y),
		image.Rect(max(rect.Min.X, rect.Max.X-thickness), rect.Min.Y, rect.Max.X, rect.Max.Y),
	}
	for _, strip := range strips {
		imagedraw.Draw(dst, strip, &image.Uniform{C: c}, image.Point{}, imagedraw.Src)
	}
}

func drawText(dst *image.RGBA, item TextConfig) error {
	ttf, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{Size: 14 * item.Scale, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	c, err := parseColor(item.Color, "text.color")
	if err != nil {
		return err
	}
	strokeRadius := min(2, max(0, (item.Thickness-1)/2))
	for dy := -strokeRadius; dy <= strokeRadius; dy++ {
		for dx := -strokeRadius; dx <= strokeRadius; dx++ {
			if dx*dx+dy*dy > strokeRadius*strokeRadius {
				continue
			}
			drawer := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face, Dot: fixed.P(item.X+dx, item.Y+dy)}
			drawer.DrawString(item.Value)
		}
	}
	return nil
}

func writeJPEGAtomic(ctx context.Context, outputPath string, img image.Image, quality int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create render output directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".photobooth-render-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary render: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := jpeg.Encode(temp, img, &jpeg.Options{Quality: quality}); err != nil {
		_ = temp.Close()
		return fmt.Errorf("encode rendered JPEG: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync rendered JPEG: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close rendered JPEG: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("activate rendered JPEG: %w", err)
	}
	return nil
}
