package output

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"
	"unicode/utf16"
)

const maxPrinterImageBytes = 256 << 20
const maxPrinterImagePixels = 50_000_000

// PrinterSubmissionError distinguishes a definite pre-spool rejection from an
// outcome that may have reached the Windows spooler. Unknown/ambiguous outcomes
// must never be retried automatically.
type PrinterSubmissionError struct {
	Uncertain bool
	JobID     string
	Err       error
}

func (e *PrinterSubmissionError) Error() string {
	if e == nil || e.Err == nil {
		return "printer submission failed"
	}
	if e.Uncertain {
		return fmt.Sprintf("printer submission outcome is uncertain (job %s): %v", e.JobID, e.Err)
	}
	return fmt.Sprintf("printer rejected submission before spooler acceptance: %v", e.Err)
}

func (e *PrinterSubmissionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func definitePrinterError(err error) error {
	if err == nil {
		return nil
	}
	return &PrinterSubmissionError{Err: err}
}

// WindowsPrinterOptions contains per-job settings. These settings are passed to
// the selected Windows queue for this job; they do not change the queue defaults.
type WindowsPrinterOptions struct {
	MediaForm    string
	Landscape    bool
	FlipVertical bool
}

// WindowsPrinter submits one rendered JPEG to a named Windows printer queue.
// It reports queued only after the Windows spooler accepts EndDoc successfully.
// A queued job is not evidence that paper physically emerged.
type WindowsPrinter struct {
	PrinterName string
	Options     WindowsPrinterOptions
}

func NewWindowsPrinter(printerName string, options WindowsPrinterOptions) (*WindowsPrinter, error) {
	printerName = strings.TrimSpace(printerName)
	if printerName == "" {
		return nil, errors.New("Windows printer queue name is required")
	}
	options.MediaForm = strings.TrimSpace(options.MediaForm)
	if strings.ContainsAny(options.MediaForm, "\r\n\x00") {
		return nil, errors.New("Windows printer media form contains invalid control characters")
	}
	return &WindowsPrinter{PrinterName: printerName, Options: options}, nil
}

func (p *WindowsPrinter) Name() string { return TargetPrinter }

func (p *WindowsPrinter) Execute(ctx context.Context, job Job) (TargetOutput, error) {
	if ctx == nil {
		return TargetOutput{}, definitePrinterError(errors.New("printer context is required"))
	}
	if err := ctx.Err(); err != nil {
		return TargetOutput{}, definitePrinterError(err)
	}
	if p == nil || strings.TrimSpace(p.PrinterName) == "" {
		return TargetOutput{}, definitePrinterError(errors.New("Windows printer queue name is required"))
	}
	if !validSessionID(job.SessionID) {
		return TargetOutput{}, definitePrinterError(errors.New("printer job session ID must be a 32-character lowercase hexadecimal identifier"))
	}
	if strings.TrimSpace(job.SourcePath) == "" {
		return TargetOutput{}, definitePrinterError(errors.New("printer source path is required"))
	}

	info, err := os.Lstat(job.SourcePath)
	if err != nil {
		return TargetOutput{}, definitePrinterError(fmt.Errorf("inspect rendered printer source: %w", err))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
		return TargetOutput{}, definitePrinterError(errors.New("printer source must be a non-empty regular file, not a symlink"))
	}
	if info.Size() > maxPrinterImageBytes {
		return TargetOutput{}, definitePrinterError(fmt.Errorf("printer source exceeds the %d-byte limit", maxPrinterImageBytes))
	}

	file, err := os.Open(job.SourcePath)
	if err != nil {
		return TargetOutput{}, definitePrinterError(fmt.Errorf("open rendered printer source: %w", err))
	}
	defer file.Close()

	src, err := jpeg.Decode(file)
	if err != nil {
		return TargetOutput{}, definitePrinterError(fmt.Errorf("decode rendered printer JPEG: %w", err))
	}
	pixels, width, height, err := preparePrinterPixels(src, p.Options.FlipVertical)
	if err != nil {
		return TargetOutput{}, definitePrinterError(err)
	}
	if err := ctx.Err(); err != nil {
		return TargetOutput{}, definitePrinterError(err)
	}

	documentName := strings.TrimSpace(job.DocumentName)
	if documentName == "" {
		documentName = "PhotoBooth session " + job.SessionID
	}
	if len(documentName) > 240 || strings.ContainsAny(documentName, "\r\n\x00") {
		return TargetOutput{}, definitePrinterError(errors.New("printer document name is invalid or too long"))
	}
	jobID, err := printJPEGToWindowsQueue(ctx, p.PrinterName, job.SessionID, documentName, pixels, width, height, p.Options.Landscape, p.Options.MediaForm)
	if err != nil {
		return TargetOutput{}, err
	}
	return TargetOutput{Status: TargetQueued, JobID: jobID}, nil
}

// preparePrinterPixels creates a top-down 32-bit BGRA DIB. Alpha is composited
// onto white so transparent sources cannot become black on drivers that ignore
// the alpha byte. FlipVertical is an explicit per-queue compatibility option;
// it must not be enabled globally based only on a virtual-port converter.
func preparePrinterPixels(src image.Image, flipVertical bool) ([]byte, int, int, error) {
	if src == nil {
		return nil, 0, 0, errors.New("printer source image is nil")
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, 0, errors.New("printer source image has empty dimensions")
	}
	if int64(width)*int64(height) > maxPrinterImagePixels {
		return nil, 0, 0, fmt.Errorf("printer source image exceeds the %d-pixel limit", maxPrinterImagePixels)
	}
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		sourceY := bounds.Min.Y + y
		if flipVertical {
			sourceY = bounds.Max.Y - 1 - y
		}
		for x := 0; x < width; x++ {
			nrgba := color.NRGBAModel.Convert(src.At(bounds.Min.X+x, sourceY)).(color.NRGBA)
			alpha := uint32(nrgba.A)
			red := (uint32(nrgba.R)*alpha + 255*(255-alpha) + 127) / 255
			green := (uint32(nrgba.G)*alpha + 255*(255-alpha) + 127) / 255
			blue := (uint32(nrgba.B)*alpha + 255*(255-alpha) + 127) / 255
			offset := (y*width + x) * 4
			pixels[offset+0] = byte(blue)
			pixels[offset+1] = byte(green)
			pixels[offset+2] = byte(red)
			pixels[offset+3] = 255
		}
	}
	return pixels, width, height, nil
}

// PrinterQueueJob is a read-only snapshot of one job still visible in the
// Windows spooler. Its status flags describe the spooler/driver report, not
// independently verified physical paper output.
type PrinterQueueJob struct {
	JobID        string   `json:"job_id"`
	DocumentName string   `json:"document_name"`
	DriverStatus string   `json:"driver_status,omitempty"`
	StatusFlags  uint32   `json:"status_flags"`
	Status       []string `json:"status,omitempty"`
	Position     uint32   `json:"position"`
	TotalPages   uint32   `json:"total_pages"`
	PagesPrinted uint32   `json:"pages_printed"`
}

// PrinterJobInspection reports whether the exact job/document identity is
// currently listed in a local Windows printer queue. Found=false means only
// that the spooler no longer lists it; it does not imply successful printing.
type PrinterJobInspection struct {
	QueueName string            `json:"queue_name"`
	Found     bool              `json:"found"`
	Jobs      []PrinterQueueJob `json:"jobs,omitempty"`
}

const (
	devmodePublicSizeOffset         = 68
	devmodeFieldsOffset             = 72
	devmodeOrientationOffset        = 76
	devmodeFormNameOffset           = 102
	devmodeFormNameChars            = 32
	devmodeFieldOrientation  uint32 = 0x00000001
	devmodeFieldFormName     uint32 = 0x00010000
)

// applyWindowsDevModeOptions updates only public per-job DEVMODEW fields.
// Vendor-private bytes remain untouched. DocumentPropertiesW is responsible
// for validating the requested form name against the selected driver.
func applyWindowsDevModeOptions(devMode []byte, landscape bool, mediaForm string) error {
	const orientationEnd = devmodeOrientationOffset + 2
	if len(devMode) < orientationEnd || len(devMode) < devmodeFieldsOffset+4 ||
		len(devMode) < devmodePublicSizeOffset+2 {
		return errors.New("printer driver returned a truncated DEVMODE")
	}
	publicSize := int(binary.LittleEndian.Uint16(devMode[devmodePublicSizeOffset : devmodePublicSizeOffset+2]))
	if publicSize < orientationEnd || publicSize > len(devMode) {
		return errors.New("printer driver returned an invalid DEVMODE size")
	}
	fields := binary.LittleEndian.Uint32(devMode[devmodeFieldsOffset : devmodeFieldsOffset+4])
	if fields&devmodeFieldOrientation == 0 {
		return errors.New("selected printer driver does not expose per-job orientation")
	}
	orientation := uint16(1)
	if landscape {
		orientation = 2
	}
	binary.LittleEndian.PutUint16(devMode[devmodeOrientationOffset:orientationEnd], orientation)
	fields |= devmodeFieldOrientation

	mediaForm = strings.TrimSpace(mediaForm)
	if mediaForm != "" {
		formUTF16 := utf16.Encode([]rune(mediaForm))
		if len(formUTF16) == 0 || len(formUTF16) >= devmodeFormNameChars {
			return fmt.Errorf("printer media form must contain between 1 and %d UTF-16 code units", devmodeFormNameChars-1)
		}
		const formEnd = devmodeFormNameOffset + devmodeFormNameChars*2
		if publicSize < formEnd || len(devMode) < formEnd {
			return errors.New("selected printer driver does not expose the DEVMODEW form-name field")
		}
		for index := 0; index < devmodeFormNameChars; index++ {
			binary.LittleEndian.PutUint16(
				devMode[devmodeFormNameOffset+index*2:devmodeFormNameOffset+index*2+2], 0,
			)
		}
		for index, value := range formUTF16 {
			offset := devmodeFormNameOffset + index*2
			binary.LittleEndian.PutUint16(devMode[offset:offset+2], value)
		}
		fields |= devmodeFieldFormName
	}
	binary.LittleEndian.PutUint32(devMode[devmodeFieldsOffset:devmodeFieldsOffset+4], fields)
	return nil
}
