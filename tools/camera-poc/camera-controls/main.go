//go:build windows

// Command camera-controls reports the current Windows Media Foundation camera
// controls without changing settings unless a bounded probe mode is requested.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	cameraSDK "github.com/Kirizu-Official/windows-camera-go/camera/v1"
	mfconsts "github.com/Kirizu-Official/windows-camera-go/windows/consts"
	winmf "github.com/Kirizu-Official/windows-camera-go/windows/mf"
)

const (
	mjpegSubtypeToken    = "47504a4d"
	maxProbeWarmupFrames = 120
)

type controlValue struct {
	Available bool   `json:"available"`
	Value     int32  `json:"value,omitempty"`
	Min       int32  `json:"min,omitempty"`
	Max       int32  `json:"max,omitempty"`
	Step      int32  `json:"step,omitempty"`
	Default   int32  `json:"default,omitempty"`
	RawFlags  int32  `json:"raw_flags,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Error     string `json:"error,omitempty"`
}

type gainProbeResult struct {
	Original        controlValue `json:"original_gain"`
	Temporary       controlValue `json:"gain_during_probe"`
	Restored        controlValue `json:"restored_gain"`
	TemporaryMode   string       `json:"temporary_mode"`
	TemporaryValue  int32        `json:"temporary_value"`
	WarmupFrames    int          `json:"discarded_warmup_frames"`
	CaptureFiles    []string     `json:"capture_files"`
	GainWasRestored bool         `json:"gain_was_restored"`
}

type report struct {
	Device     string                  `json:"device"`
	SymbolLink string                  `json:"symbol_link"`
	Stream     map[string]any          `json:"stream"`
	Controls   map[string]controlValue `json:"controls"`
	GainProbe  *gainProbeResult        `json:"gain_probe,omitempty"`
	Note       string                  `json:"note"`
}

func main() {
	deviceName := flag.String("device", "B525 HD Webcam", "camera friendly name or exact symbolic link")
	width := flag.Int("width", 1280, "stream width")
	height := flag.Int("height", 720, "stream height")
	fps := flag.Float64("fps", 30, "target stream FPS")
	probeAutoGain := flag.Bool("probe-auto-gain", false, "temporarily try AUTO gain, capture three diagnostic frames, then restore the prior gain state")
	probeMaxGain := flag.Bool("probe-max-gain", false, "temporarily set maximum manual gain, capture three diagnostic frames, then restore the prior gain state")
	outputDir := flag.String("output-dir", "", "required output directory when a gain probe mode is selected")
	flag.Parse()

	if *probeAutoGain && *probeMaxGain {
		fmt.Fprintln(os.Stderr, "choose only one of --probe-auto-gain or --probe-max-gain")
		os.Exit(2)
	}
	probeMode := ""
	if *probeAutoGain {
		probeMode = "auto"
	} else if *probeMaxGain {
		probeMode = "max-manual"
	}

	if err := run(strings.TrimSpace(*deviceName), *width, *height, *fps, probeMode, *outputDir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "camera-controls:", err)
		os.Exit(1)
	}
}

func run(selector string, width, height int, fps float64, probeMode, outputDir string, stdout *os.File) error {
	if runtime.GOOS != "windows" {
		return errors.New("camera control query is supported on Windows only")
	}
	if width < 1 || height < 1 || fps <= 0 {
		return errors.New("width, height and fps must be positive")
	}
	if probeMode != "" && strings.TrimSpace(outputDir) == "" {
		return errors.New("--output-dir is required for a gain-probe mode")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := cameraSDK.Init(); err != nil {
		return fmt.Errorf("initialize Media Foundation: %w", err)
	}
	defer cameraSDK.Shutdown()

	devices, err := cameraSDK.EnumDevice()
	if err != nil {
		return fmt.Errorf("enumerate camera devices: %w", err)
	}
	var selected *cameraSDK.DeviceInfo
	for _, candidate := range devices {
		if selector == "" || strings.EqualFold(selector, candidate.Name) || selector == candidate.SymbolLink {
			selected = candidate
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("camera %q was not found", selector)
	}

	device, err := cameraSDK.OpenDevice(selected.SymbolLink)
	if err != nil {
		return fmt.Errorf("open camera %q: %w", selected.Name, err)
	}
	defer device.CloseDevice()

	formats, err := device.EnumerateCaptureFormats()
	if err != nil {
		return fmt.Errorf("enumerate capture formats: %w", err)
	}
	format, err := selectMJPEGFormat(formats, width, height, fps)
	if err != nil {
		return err
	}
	capture, err := device.StartCapture(format)
	if err != nil {
		return fmt.Errorf("start camera stream: %w", err)
	}
	cameraControl, videoControl, err := device.GetControl()
	if err != nil {
		return fmt.Errorf("access camera controls: %w", err)
	}
	defer videoControl.Release()
	defer cameraControl.Release()

	result := report{
		Device:     selected.Name,
		SymbolLink: selected.SymbolLink,
		Stream: map[string]any{
			"width":   format.Width,
			"height":  format.Height,
			"fps":     format.Fps,
			"subtype": fmt.Sprint(format.SubType),
		},
		Controls: map[string]controlValue{
			"exposure":               queryCameraControl(cameraControl, mfconsts.CameraControl_Exposure),
			"focus":                  queryCameraControl(cameraControl, mfconsts.CameraControl_Focus),
			"brightness":             queryVideoControl(videoControl, mfconsts.VideoProcAmp_Brightness),
			"gamma":                  queryVideoControl(videoControl, mfconsts.VideoProcAmp_Gamma),
			"white_balance":          queryVideoControl(videoControl, mfconsts.VideoProcAmp_WhiteBalance),
			"backlight_compensation": queryVideoControl(videoControl, mfconsts.VideoProcAmp_BacklightCompensation),
			"gain":                   queryVideoControl(videoControl, mfconsts.VideoProcAmp_Gain),
		},
		Note: "Read-only query. Control values and flags are driver-specific; no controls are changed unless a gain-probe flag is explicitly supplied.",
	}

	if probeMode != "" {
		probe, err := runGainProbe(device, capture, videoControl, format, outputDir, probeMode)
		if err != nil {
			return fmt.Errorf("gain probe (restoration attempted and verified when possible): %w", err)
		}
		result.GainProbe = probe
		result.Note = "Gain A/B probe temporarily changed the gain mode/value for one second of warm-up plus three diagnostic frames, then restored and verified the original gain value and flags. Images are diagnostic only; inspect and delete them after analysis."
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}

func queryCameraControl(controls *winmf.IAMCameraControl, property mfconsts.CameraControlProperty) controlValue {
	rangeInfo, err := controls.GetRange(property)
	if err != nil {
		return controlValue{Error: "read range: " + err.Error()}
	}
	if !rangeInfo.Found {
		return controlValue{Available: false, Mode: "unsupported"}
	}
	value, flags, err := controls.Get(property)
	if err != nil {
		return controlValue{Available: true, Error: "read current value: " + err.Error()}
	}
	return controlValue{
		Available: true,
		Value:     value,
		Min:       rangeInfo.Min,
		Max:       rangeInfo.Max,
		Step:      rangeInfo.SteppingDelta,
		Default:   rangeInfo.DefaultValue,
		RawFlags:  int32(flags),
		Mode:      controlMode(int32(flags), int32(mfconsts.KSPROPERTY_CAMERACONTROL_FLAGS_AUTO), int32(mfconsts.KSPROPERTY_CAMERACONTROL_FLAGS_MANUAL)),
	}
}

func queryVideoControl(controls *winmf.IAMVideoProcAmp, property mfconsts.VideoProcAmpProperty) controlValue {
	rangeInfo, err := controls.GetRange(property)
	if err != nil {
		return controlValue{Error: "read range: " + err.Error()}
	}
	if !rangeInfo.Found {
		return controlValue{Available: false, Mode: "unsupported"}
	}
	value, flags, err := controls.Get(property)
	if err != nil {
		return controlValue{Available: true, Error: "read current value: " + err.Error()}
	}
	return controlValue{
		Available: true,
		Value:     value,
		Min:       rangeInfo.Min,
		Max:       rangeInfo.Max,
		Step:      rangeInfo.SteppingDelta,
		Default:   rangeInfo.DefaultValue,
		RawFlags:  int32(flags),
		Mode:      controlMode(int32(flags), int32(mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_AUTO), int32(mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL)),
	}
}

func controlMode(flags, autoFlag, manualFlag int32) string {
	switch {
	case flags&autoFlag != 0:
		return "auto"
	case flags&manualFlag != 0:
		return "manual"
	default:
		return "unknown"
	}
}

func selectMJPEGFormat(formats []*cameraSDK.CaptureFormats, width, height int, targetFPS float64) (*cameraSDK.CaptureFormats, error) {
	var below, above *cameraSDK.CaptureFormats
	for _, format := range formats {
		if format == nil || int(format.Width) != width || int(format.Height) != height {
			continue
		}
		if !strings.Contains(strings.ToLower(fmt.Sprint(format.SubType)), mjpegSubtypeToken) {
			continue
		}
		if format.Fps <= targetFPS && (below == nil || format.Fps > below.Fps) {
			below = format
		}
		if format.Fps > targetFPS && (above == nil || format.Fps < above.Fps) {
			above = format
		}
	}
	selected := below
	if selected == nil {
		selected = above
	}
	if selected == nil {
		return nil, fmt.Errorf("camera has no MJPEG stream at %dx%d near %.1f FPS", width, height, targetFPS)
	}
	return selected, nil
}

// runGainProbe performs a bounded, reversible gain experiment using either AUTO
// mode or the maximum supported manual gain, then verifies restoration of the
// original value and mode before returning.
func runGainProbe(
	device *cameraSDK.Device,
	capture *cameraSDK.CaptureSync,
	controls *winmf.IAMVideoProcAmp,
	format *cameraSDK.CaptureFormats,
	outputDir, targetMode string,
) (result *gainProbeResult, returnErr error) {
	original := queryVideoControl(controls, mfconsts.VideoProcAmp_Gain)
	if !original.Available || original.Error != "" {
		return nil, fmt.Errorf("could not read original gain value and range: %+v", original)
	}
	if original.Mode != "manual" {
		return nil, fmt.Errorf("probe expects manual gain so it can compare against a known original state; current mode is %q", original.Mode)
	}
	originalFlags := mfconsts.VideoProcAmpFlags(original.RawFlags)
	temporaryValue := original.Value
	temporaryFlags := originalFlags
	expectedMode := "manual"
	switch targetMode {
	case "auto":
		temporaryFlags = mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_AUTO
		expectedMode = "auto"
	case "max-manual":
		temporaryValue = original.Max
		temporaryFlags = mfconsts.KSPROPERTY_VIDEOPROCAMP_FLAGS_MANUAL
	default:
		return nil, fmt.Errorf("unsupported gain probe mode %q", targetMode)
	}
	result = &gainProbeResult{
		Original: original, TemporaryMode: targetMode, TemporaryValue: temporaryValue,
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, fmt.Errorf("create diagnostic image directory: %w", err)
	}

	restored := false
	restore := func() error {
		if restored {
			return nil
		}
		current := queryVideoControl(controls, mfconsts.VideoProcAmp_Gain)
		if gainStateMatches(current, original.Value, "manual", mfconsts.VideoProcAmpFlags(original.RawFlags)) {
			result.Restored = current
			restored = true
			result.GainWasRestored = true
			return nil
		}

		setErr := controls.Set(mfconsts.VideoProcAmp_Gain, original.Value, originalFlags)
		observed, _, waitErr := waitForGainState(
			device, capture, controls, original.Value, "manual",
			mfconsts.VideoProcAmpFlags(original.RawFlags), int(math.Ceil(format.Fps)),
		)
		result.Restored = observed
		if gainStateMatches(observed, original.Value, "manual", mfconsts.VideoProcAmpFlags(original.RawFlags)) {
			restored = true
			result.GainWasRestored = true
			return nil
		}
		if setErr != nil || waitErr != nil {
			return fmt.Errorf("restore gain to value=%d flags=%d failed (setErr=%v, verifyErr=%v); readback=%+v", original.Value, original.RawFlags, setErr, waitErr, observed)
		}
		return fmt.Errorf("gain restore did not match original state; original=%+v observed=%+v", original, observed)
	}
	defer func() {
		if restored {
			return
		}
		if err := restore(); err != nil {
			restoreErr := fmt.Errorf("gain probe could not verify original gain restoration: %w", err)
			if returnErr == nil {
				returnErr = restoreErr
			} else {
				returnErr = errors.Join(returnErr, restoreErr)
			}
		}
	}()

	if err := controls.Set(mfconsts.VideoProcAmp_Gain, temporaryValue, temporaryFlags); err != nil {
		return result, fmt.Errorf("temporarily set gain mode %q value %d: %w", targetMode, temporaryValue, err)
	}
	var waitedFrames int
	var waitErr error
	result.Temporary, waitedFrames, waitErr = waitForGainState(
		device, capture, controls, temporaryValue, expectedMode, temporaryFlags, int(math.Ceil(format.Fps)),
	)
	result.WarmupFrames += waitedFrames
	if waitErr != nil {
		return result, fmt.Errorf("driver did not report the requested gain mode/value within the bounded verification window (mode %q value %d): last readback=%+v: %w", expectedMode, temporaryValue, result.Temporary, waitErr)
	}

	warmupFrames := int(math.Ceil(format.Fps))
	if warmupFrames < 1 {
		warmupFrames = 1
	}
	if warmupFrames > maxProbeWarmupFrames {
		warmupFrames = maxProbeWarmupFrames
	}
	for frameIndex := 1; frameIndex <= warmupFrames; frameIndex++ {
		if _, err := readFrameBytes(device, capture); err != nil {
			return result, fmt.Errorf("discard gain-probe warm-up frame %d/%d: %w", frameIndex, warmupFrames, err)
		}
	}
	result.WarmupFrames += warmupFrames

	result.CaptureFiles = make([]string, 0, 3)
	for shot := 1; shot <= 3; shot++ {
		data, err := readFrameBytes(device, capture)
		if err != nil {
			return result, fmt.Errorf("read gain-probe sample %d/3: %w", shot, err)
		}
		path := filepath.Join(outputDir, fmt.Sprintf("gain-%s-capture-%02d.jpg", targetMode, shot))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return result, fmt.Errorf("write gain-probe sample %d/3: %w", shot, err)
		}
		result.CaptureFiles = append(result.CaptureFiles, path)
	}

	if err := restore(); err != nil {
		return result, fmt.Errorf("restore original gain after capture: %w", err)
	}
	return result, nil
}

func readFrameBytes(device *cameraSDK.Device, capture *cameraSDK.CaptureSync) ([]byte, error) {
	frame, err := capture.GetFrame()
	if err != nil {
		return nil, err
	}
	if frame == nil || frame.PpSample == nil {
		if frame != nil {
			_ = frame.Release()
		}
		return nil, errors.New("camera returned an empty frame sample")
	}
	buffer, err := device.ParseSampleToBuffer(frame.PpSample)
	if err != nil {
		_ = frame.Release()
		return nil, fmt.Errorf("parse camera sample: %w", err)
	}
	length := int(buffer.Length)
	if length < 1 || length > len(buffer.Buffer) {
		buffer.Release()
		_ = frame.Release()
		return nil, fmt.Errorf("invalid sample length %d (buffer has %d bytes)", length, len(buffer.Buffer))
	}
	data := append([]byte(nil), buffer.Buffer[:length]...)
	buffer.Release()
	if err := frame.Release(); err != nil {
		return nil, fmt.Errorf("release camera sample: %w", err)
	}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return nil, errors.New("camera stream did not return a JPEG frame")
	}
	return data, nil
}

func gainStateMatches(current controlValue, value int32, mode string, flags mfconsts.VideoProcAmpFlags) bool {
	if !current.Available || current.Error != "" || current.Mode != mode || current.RawFlags != int32(flags) {
		return false
	}
	if mode == "manual" {
		return current.Value == value
	}
	return mode == "auto"
}

// waitForGainState advances the real camera stream while checking driver
// readback. Some drivers apply controls on a frame boundary; immediate readback
// alone is not enough. The wait is bounded by a caller-selected frame count.
func waitForGainState(
	device *cameraSDK.Device,
	capture *cameraSDK.CaptureSync,
	controls *winmf.IAMVideoProcAmp,
	value int32,
	mode string,
	flags mfconsts.VideoProcAmpFlags,
	maxFrames int,
) (controlValue, int, error) {
	if maxFrames < 1 {
		maxFrames = 1
	}
	if maxFrames > maxProbeWarmupFrames {
		maxFrames = maxProbeWarmupFrames
	}
	last := queryVideoControl(controls, mfconsts.VideoProcAmp_Gain)
	for frame := 1; frame <= maxFrames; frame++ {
		if _, err := readFrameBytes(device, capture); err != nil {
			return queryVideoControl(controls, mfconsts.VideoProcAmp_Gain), frame - 1,
				fmt.Errorf("read verification frame %d/%d: %w", frame, maxFrames, err)
		}
		last = queryVideoControl(controls, mfconsts.VideoProcAmp_Gain)
		if gainStateMatches(last, value, mode, flags) {
			return last, frame, nil
		}
	}
	return last, maxFrames, fmt.Errorf("readback did not match requested mode=%q value=%d flags=%d after %d frames", mode, value, flags, maxFrames)
}
