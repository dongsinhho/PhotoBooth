package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/windowsmf"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
	"github.com/dongsinhho/PhotoBooth/internal/booth/render"
	"github.com/dongsinhho/PhotoBooth/internal/booth/session"
	"github.com/dongsinhho/PhotoBooth/internal/booth/workflow"
)

type options struct {
	sessions, shots, width, height                int
	device, frame, workDir, outputDir, metricsCSV string
	testContention                                bool
	contentionAfter, holderSeconds                int
	python, holderScript, heapProfile             string
}

type holder struct {
	cmd      *exec.Cmd
	logPath  string
	done     chan error
	finished bool
	waitErr  error
}

func main() {
	var o options
	flag.IntVar(&o.sessions, "sessions", 100, "session attempts in this one Go process")
	flag.IntVar(&o.shots, "shots", 3, "captures per session")
	flag.IntVar(&o.width, "width", 1280, "capture width")
	flag.IntVar(&o.height, "height", 720, "capture height")
	flag.StringVar(&o.device, "device", "B525 HD Webcam", "camera friendly name or exact symbolic link")
	flag.StringVar(&o.frame, "frame", "examples/frames/photo-strip-v1.json", "frame JSON")
	flag.StringVar(&o.workDir, "work-dir", "", "directory for captures and rendered work images")
	flag.StringVar(&o.outputDir, "output-dir", "", "directory for final local outputs")
	flag.StringVar(&o.metricsCSV, "metrics-csv", "", "path to per-session CSV metrics")
	flag.BoolVar(&o.testContention, "test-contention", false, "temporarily occupy webcam and verify recovery in the same Go process")
	flag.IntVar(&o.contentionAfter, "contention-after", 3, "start competing camera holder after this session")
	flag.IntVar(&o.holderSeconds, "holder-seconds", 5, "duration the competing holder owns the camera")
	flag.StringVar(&o.python, "python", "", "Python executable for contention testing")
	flag.StringVar(&o.holderScript, "holder-script", "", "path to hold_camera.py")
	flag.StringVar(&o.heapProfile, "heap-profile", "", "optional path to write an in-use Go heap profile after the soak")
	flag.Parse()
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "persistent soak failed:", err)
		os.Exit(1)
	}
}

func run(o options) (runErr error) {
	if o.sessions < 1 || o.sessions > 1000 {
		return errors.New("sessions must be between 1 and 1000")
	}
	if o.shots < 1 || o.shots > 12 {
		return errors.New("shots must be between 1 and 12")
	}
	if o.width < 1 || o.height < 1 {
		return errors.New("capture dimensions must be positive")
	}
	if o.workDir == "" || o.outputDir == "" || o.metricsCSV == "" {
		return errors.New("work-dir, output-dir and metrics-csv are required; use temporary folders for real images")
	}
	if o.testContention {
		if o.contentionAfter < 1 || o.contentionAfter >= o.sessions {
			return errors.New("contention-after must be between 1 and sessions-1")
		}
		if o.holderSeconds < 2 || o.holderSeconds > 60 {
			return errors.New("holder-seconds must be between 2 and 60")
		}
		if o.python == "" || o.holderScript == "" {
			return errors.New("test-contention requires python and holder-script paths")
		}
	}
	framePath, err := filepath.Abs(o.frame)
	if err != nil {
		return fmt.Errorf("resolve frame path: %w", err)
	}
	if _, err := os.Stat(framePath); err != nil {
		return fmt.Errorf("frame config: %w", err)
	}
	workDir, err := filepath.Abs(o.workDir)
	if err != nil {
		return err
	}
	outputDir, err := filepath.Abs(o.outputDir)
	if err != nil {
		return err
	}
	metricsPath, err := filepath.Abs(o.metricsCSV)
	if err != nil {
		return err
	}
	for _, path := range []string{workDir, outputDir, filepath.Dir(metricsPath)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
	}
	frameWidth, frameHeight, err := readFrameSize(framePath)
	if err != nil {
		return err
	}
	metrics, err := os.Create(metricsPath)
	if err != nil {
		return fmt.Errorf("create metrics CSV: %w", err)
	}
	defer func() {
		if err := metrics.Close(); err != nil && runErr == nil {
			runErr = err
		}
	}()
	writer := csv.NewWriter(metrics)
	header := []string{"index", "session_id", "state", "wall_ms", "captures", "render_ms", "output_ms", "heap_alloc_before_gc", "heap_inuse_before_gc", "heap_sys_before_gc", "heap_alloc_after_gc", "heap_inuse_after_gc", "heap_sys_after_gc", "total_alloc_bytes", "num_gc", "goroutines_after_gc", "error"}
	if err := writer.Write(header); err != nil {
		return err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	runner := &workflow.Runner{
		Camera:   windowsmf.New(windowsmf.Options{DeviceID: o.device, Width: o.width, Height: o.height, FPS: 30}),
		Renderer: render.New(),
		Output:   output.NewLocal(outputDir),
		OnEvent: func(e workflow.Event) {
			if e.Type == "failed" {
				fmt.Printf("SESSION_ERROR id=%s state=%s error=%q\n", e.SessionID, e.State, e.Message)
			}
		},
	}
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	fmt.Printf("PERSISTENT_SOAK_START pid=%d sessions=%d shots=%d device=%q capture=%dx%d output=%dx%d baseline_heap_alloc_mb=%.2f baseline_heap_sys_mb=%.2f\n",
		os.Getpid(), o.sessions, o.shots, o.device, o.width, o.height, frameWidth, frameHeight,
		toMB(baseline.HeapAlloc), toMB(baseline.HeapSys))

	var h *holder
	failed, firstFailure, lastFailure := 0, 0, 0
	recovery := false
	contentionFailure := false
	successful := 0
	startedAll := time.Now()
	for i := 1; i <= o.sessions; i++ {
		released := false
		if h != nil {
			var pollErr error
			released, pollErr = pollHolder(h)
			if pollErr != nil {
				return pollErr
			}
		}

		started := time.Now()
		res, runErr := runner.Run(context.Background(), workflow.Config{
			FrameConfigPath: framePath, WorkDir: workDir, CaptureCount: o.shots,
			CaptureWidth: o.width, CaptureHeight: o.height,
		})
		wall := time.Since(started)
		ok := runErr == nil && (res.State == session.Completed || res.State == session.PartialSuccess)
		if ok {
			successful++
			if released && firstFailure > 0 {
				recovery = true
			}
		} else {
			failed++
			if firstFailure == 0 {
				firstFailure = i
			}
			lastFailure = i
			if h != nil && !released && runErr != nil && strings.Contains(runErr.Error(), "0xC00D3704") {
				contentionFailure = true
			}
		}
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		errText := ""
		if runErr != nil {
			errText = runErr.Error()
		}
		row := []string{
			strconv.Itoa(i), res.SessionID, string(res.State), fmt.Sprintf("%.3f", float64(wall)/float64(time.Millisecond)),
			strconv.Itoa(len(res.CapturePaths)), fmt.Sprintf("%.3f", float64(res.RenderDuration)/float64(time.Millisecond)),
			fmt.Sprintf("%.3f", float64(res.OutputDuration)/float64(time.Millisecond)),
			strconv.FormatUint(before.HeapAlloc, 10), strconv.FormatUint(before.HeapInuse, 10), strconv.FormatUint(before.HeapSys, 10),
			strconv.FormatUint(after.HeapAlloc, 10), strconv.FormatUint(after.HeapInuse, 10), strconv.FormatUint(after.HeapSys, 10),
			strconv.FormatUint(after.TotalAlloc, 10), strconv.FormatUint(uint64(after.NumGC), 10),
			strconv.Itoa(runtime.NumGoroutine()), errText,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		fmt.Printf("SESSION_END index=%03d id=%s state=%s wall_ms=%.1f captures=%d render_ms=%.1f output_ms=%.1f heap_before_gc_mb=%.2f heap_after_gc_mb=%.2f heap_sys_mb=%.2f goroutines=%d error=%q\n",
			i, res.SessionID, res.State, float64(wall)/float64(time.Millisecond), len(res.CapturePaths),
			float64(res.RenderDuration)/float64(time.Millisecond), float64(res.OutputDuration)/float64(time.Millisecond),
			toMB(before.HeapAlloc), toMB(after.HeapAlloc), toMB(after.HeapSys), runtime.NumGoroutine(), errText)

		if o.testContention && i == o.contentionAfter {
			holderLog := filepath.Join(workDir, "contention-holder.log")
			h, err = startHolder(o.python, o.holderScript, o.holderSeconds, holderLog)
			if err != nil {
				return err
			}
			fmt.Printf("CONTENTION_HOLDER_OPENED after_session=%d duration_s=%d\n", i, o.holderSeconds)
		}
	}

	if h != nil {
		if err := waitHolder(h, 20*time.Second); err != nil {
			return err
		}
		fmt.Printf("CONTENTION_HOLDER_RELEASED log=%q\n", h.logPath)
	}
	if o.testContention {
		// Run one explicit recovery session after the holder has been confirmed
		// released. The earlier fixed batch may finish before a short holder exits.
		index := o.sessions + 1
		started := time.Now()
		res, recoveryErr := runner.Run(context.Background(), workflow.Config{
			FrameConfigPath: framePath, WorkDir: workDir, CaptureCount: o.shots,
			CaptureWidth: o.width, CaptureHeight: o.height,
		})
		wall := time.Since(started)
		ok := recoveryErr == nil && (res.State == session.Completed || res.State == session.PartialSuccess)
		if ok {
			successful++
			recovery = true
		} else {
			failed++
			lastFailure = index
			if firstFailure == 0 {
				firstFailure = index
			}
		}
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		errText := ""
		if recoveryErr != nil {
			errText = recoveryErr.Error()
		}
		row := []string{
			strconv.Itoa(index), res.SessionID, string(res.State), fmt.Sprintf("%.3f", float64(wall)/float64(time.Millisecond)),
			strconv.Itoa(len(res.CapturePaths)), fmt.Sprintf("%.3f", float64(res.RenderDuration)/float64(time.Millisecond)),
			fmt.Sprintf("%.3f", float64(res.OutputDuration)/float64(time.Millisecond)),
			strconv.FormatUint(before.HeapAlloc, 10), strconv.FormatUint(before.HeapInuse, 10), strconv.FormatUint(before.HeapSys, 10),
			strconv.FormatUint(after.HeapAlloc, 10), strconv.FormatUint(after.HeapInuse, 10), strconv.FormatUint(after.HeapSys, 10),
			strconv.FormatUint(after.TotalAlloc, 10), strconv.FormatUint(uint64(after.NumGC), 10),
			strconv.Itoa(runtime.NumGoroutine()), errText,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		fmt.Printf("SESSION_END index=%03d phase=recovery id=%s state=%s wall_ms=%.1f captures=%d heap_after_gc_mb=%.2f goroutines=%d error=%q\n",
			index, res.SessionID, res.State, float64(wall)/float64(time.Millisecond), len(res.CapturePaths), toMB(after.HeapAlloc), runtime.NumGoroutine(), errText)
	}
	verified, err := verify(workDir, outputDir, successful, o.shots, o.width, o.height, frameWidth, frameHeight)
	if err != nil {
		return err
	}
	// JPEG verification allocates image buffers; collect them before profiling
	// or reporting retained heap so this value is comparable to per-session samples.
	runtime.GC()
	if o.heapProfile != "" {
		profilePath, err := filepath.Abs(o.heapProfile)
		if err != nil {
			return fmt.Errorf("resolve heap profile path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(profilePath), 0o700); err != nil {
			return fmt.Errorf("create heap profile directory: %w", err)
		}
		profile, err := os.Create(profilePath)
		if err != nil {
			return fmt.Errorf("create heap profile: %w", err)
		}
		profileErr := pprof.WriteHeapProfile(profile)
		closeErr := profile.Close()
		if profileErr != nil {
			return fmt.Errorf("write heap profile: %w", profileErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close heap profile: %w", closeErr)
		}
		fmt.Printf("PERSISTENT_SOAK_HEAP_PROFILE path=%q\n", profilePath)
		runtime.GC()
	}
	var final runtime.MemStats
	runtime.ReadMemStats(&final)
	fmt.Printf("PERSISTENT_SOAK_SUMMARY attempts=%d success=%d failed=%d contention_failure_observed=%t recovery_after_release=%t first_failure=%d last_failure=%d elapsed_s=%.2f heap_alloc_mb=%.2f heap_inuse_mb=%.2f heap_sys_mb=%.2f total_alloc_mb=%.2f num_gc=%d goroutines=%d verified_jpegs=%d metrics=%q\n",
		o.sessions+boolToInt(o.testContention), successful, failed, contentionFailure, recovery, firstFailure, lastFailure, time.Since(startedAll).Seconds(),
		toMB(final.HeapAlloc), toMB(final.HeapInuse), toMB(final.HeapSys), toMB(final.TotalAlloc), final.NumGC, runtime.NumGoroutine(), verified, metricsPath)
	if !o.testContention && failed > 0 {
		return fmt.Errorf("%d unexpected session failures; inspect %s", failed, metricsPath)
	}
	if o.testContention && (!contentionFailure || !recovery) {
		return errors.New("contention test must observe HRESULT 0xC00D3704 while the holder is active and later recover in the same Go process")
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func toMB(v uint64) float64 { return float64(v) / (1024 * 1024) }

func readFrameSize(path string) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var cfg render.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Canvas.Width < 1 || cfg.Canvas.Height < 1 {
		return 0, 0, errors.New("invalid frame canvas size")
	}
	return cfg.Canvas.Width, cfg.Canvas.Height, nil
}

func verify(workDir, outputDir string, sessions, shots, width, height, outputWidth, outputHeight int) (int, error) {
	verified := 0
	capturePaths, err := filepath.Glob(filepath.Join(workDir, "*", "captures", "capture-*.jpg"))
	if err != nil {
		return 0, err
	}
	if len(capturePaths) != sessions*shots {
		return 0, fmt.Errorf("found %d capture files; successful session count implies %d", len(capturePaths), sessions*shots)
	}
	for _, p := range capturePaths {
		if err := verifyJPEG(p, width, height); err != nil {
			return 0, fmt.Errorf("%s: %w", p, err)
		}
		verified++
	}
	rendered, err := filepath.Glob(filepath.Join(workDir, "*", "rendered.jpg"))
	if err != nil {
		return 0, err
	}
	if len(rendered) != sessions {
		return 0, fmt.Errorf("found %d rendered files for %d successful sessions", len(rendered), sessions)
	}
	for _, p := range rendered {
		if err := verifyJPEG(p, outputWidth, outputHeight); err != nil {
			return 0, fmt.Errorf("%s: %w", p, err)
		}
		verified++
	}
	local, err := filepath.Glob(filepath.Join(outputDir, "*", "photo.jpg"))
	if err != nil {
		return 0, err
	}
	if len(local) != sessions {
		return 0, fmt.Errorf("found %d local files for %d successful sessions", len(local), sessions)
	}
	for _, p := range local {
		if err := verifyJPEG(p, outputWidth, outputHeight); err != nil {
			return 0, fmt.Errorf("%s: %w", p, err)
		}
		verified++
	}
	fmt.Printf("PERSISTENT_SOAK_VERIFY successful_sessions=%d capture_jpegs=%d rendered_jpegs=%d local_jpegs=%d total_jpegs=%d capture=%dx%d output=%dx%d\n",
		sessions, len(capturePaths), len(rendered), len(local), verified, width, height, outputWidth, outputHeight)
	return verified, nil
}

func verifyJPEG(path string, w, h int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	img, decodeErr := jpeg.Decode(f)
	closeErr := f.Close()
	if decodeErr != nil {
		return fmt.Errorf("decode JPEG: %w", decodeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		return fmt.Errorf("dimensions %dx%d, expected %dx%d", b.Dx(), b.Dy(), w, h)
	}
	return nil
}

func startHolder(python, script string, seconds int, logPath string) (*holder, error) {
	f, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(python, script, "--index", "0", "--backend", "msmf", "--seconds", strconv.Itoa(seconds))
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	h := &holder{cmd: cmd, logPath: logPath, done: make(chan error, 1)}
	go func() { h.done <- cmd.Wait() }()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), "HOLDER_OPENED") {
			return h, nil
		}
		select {
		case e := <-h.done:
			h.finished = true
			h.waitErr = e
			return nil, fmt.Errorf("holder failed before acquiring camera: %v; %s", e, string(data))
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for holder to acquire camera: %s", readText(logPath))
}

func pollHolder(h *holder) (bool, error) {
	if h == nil {
		return false, nil
	}
	if !h.finished {
		select {
		case e := <-h.done:
			h.finished = true
			h.waitErr = e
		default:
		}
	}
	if !h.finished {
		return false, nil
	}
	data := readText(h.logPath)
	if h.waitErr != nil {
		return true, fmt.Errorf("holder failed: %w; %s", h.waitErr, data)
	}
	if !strings.Contains(data, "HOLDER_RELEASED") {
		return true, fmt.Errorf("holder did not confirm release: %s", data)
	}
	return true, nil
}

func waitHolder(h *holder, timeout time.Duration) error {
	if h == nil {
		return nil
	}
	if h.finished {
		_, e := pollHolder(h)
		return e
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case e := <-h.done:
		h.finished = true
		h.waitErr = e
	case <-t.C:
		return fmt.Errorf("timeout waiting for camera holder: %s", readText(h.logPath))
	}
	_, err := pollHolder(h)
	return err
}
func readText(p string) string {
	b, e := os.ReadFile(p)
	if e != nil {
		return e.Error()
	}
	return string(b)
}
