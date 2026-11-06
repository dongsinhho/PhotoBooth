// Package retention provides an opt-in, conservative planner for local PhotoBooth data.
package retention

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
)

const (
	KindSession      = "session"
	KindOutput       = "output"
	maxRetentionDays = 3650
)

type Policy struct {
	SessionRetentionDays int
	OutputRetentionDays  int
}

type Options struct {
	SessionsDir string
	OutputDir   string
	Policy      Policy
	Now         time.Time
}

type Candidate struct {
	Kind         string
	SessionID    string
	Path         string
	LastModified time.Time
	SizeBytes    int64
	FileCount    int
	fingerprint  string
	fileInfo     os.FileInfo
}

type rootSnapshot struct {
	exists bool
	info   os.FileInfo
}

type Plan struct {
	GeneratedAt   time.Time
	Candidates    []Candidate
	ScannedCount  int
	SkippedCount  int
	Disabled      bool
	options       Options
	rootSnapshots map[string]rootSnapshot
}

type ApplyResult struct {
	Removed []Candidate
}

// PlanDigest returns a stable confirmation token for the exact roots, policy and
// candidate metadata in a plan. GeneratedAt is deliberately excluded so an
// operator can rerun the same dry-run later; current candidates are rebuilt
// before any apply operation and the digest must still match.
func PlanDigest(plan Plan) (string, error) {
	if plan.GeneratedAt.IsZero() || strings.TrimSpace(plan.options.SessionsDir) == "" ||
		strings.TrimSpace(plan.options.OutputDir) == "" {
		return "", errors.New("invalid retention plan; call BuildPlan first")
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "retention-plan-v1\nroot-sessions=%s\nroot-output=%s\nsession-days=%d\noutput-days=%d\ndisabled=%t\n",
		filepath.Clean(plan.options.SessionsDir), filepath.Clean(plan.options.OutputDir),
		plan.options.Policy.SessionRetentionDays, plan.options.Policy.OutputRetentionDays, plan.Disabled)
	for _, candidate := range plan.Candidates {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s\n",
			candidate.Kind, candidate.SessionID, filepath.Clean(candidate.Path),
			candidate.LastModified.UTC().Format(time.RFC3339Nano), candidate.FileCount,
			candidate.SizeBytes, candidate.fingerprint)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (p Policy) Validate() error {
	if p.SessionRetentionDays < 0 || p.SessionRetentionDays > maxRetentionDays {
		return fmt.Errorf("session retention days must be 0 (disabled) or 1-%d", maxRetentionDays)
	}
	if p.OutputRetentionDays < 0 || p.OutputRetentionDays > maxRetentionDays {
		return fmt.Errorf("output retention days must be 0 (disabled) or 1-%d", maxRetentionDays)
	}
	return nil
}

// BuildPlan is read-only. Unknown directory names, symlinks and unexpected
// files are skipped rather than treated as cleanup candidates.
func BuildPlan(ctx context.Context, options Options) (Plan, error) {
	plan := Plan{Candidates: []Candidate{}, options: options}
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	if err := options.Policy.Validate(); err != nil {
		return plan, err
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	options.Now = options.Now.UTC()
	for key, value := range map[string]string{"sessions": options.SessionsDir, "output": options.OutputDir} {
		if strings.TrimSpace(value) == "" {
			return plan, fmt.Errorf("%s directory is required", key)
		}
		absolute, err := filepath.Abs(value)
		if err != nil {
			return plan, fmt.Errorf("resolve %s directory: %w", key, err)
		}
		if key == "sessions" {
			options.SessionsDir = filepath.Clean(absolute)
		} else {
			options.OutputDir = filepath.Clean(absolute)
		}
	}
	if err := stationconfig.ValidateLocalStoragePath(options.SessionsDir); err != nil {
		return plan, fmt.Errorf("validate retention sessions root: %w", err)
	}
	if err := stationconfig.ValidateLocalStoragePath(options.OutputDir); err != nil {
		return plan, fmt.Errorf("validate retention output root: %w", err)
	}
	if err := stationconfig.ValidateDistinctStorageRoots(options.SessionsDir, options.OutputDir); err != nil {
		return plan, fmt.Errorf("refuse retention plan: %w", err)
	}
	plan.options, plan.GeneratedAt = options, options.Now
	if options.Policy.SessionRetentionDays == 0 && options.Policy.OutputRetentionDays == 0 {
		plan.Disabled = true
		return plan, nil
	}
	plan.rootSnapshots = make(map[string]rootSnapshot, 2)
	if options.Policy.SessionRetentionDays > 0 {
		items, scanned, skipped, snapshot, err := scanRoot(ctx, options.SessionsDir, KindSession, options.Policy.SessionRetentionDays, options.Now)
		if err != nil {
			return plan, err
		}
		plan.Candidates = append(plan.Candidates, items...)
		plan.ScannedCount += scanned
		plan.SkippedCount += skipped
		plan.rootSnapshots[options.SessionsDir] = snapshot
	}
	if options.Policy.OutputRetentionDays > 0 {
		items, scanned, skipped, snapshot, err := scanRoot(ctx, options.OutputDir, KindOutput, options.Policy.OutputRetentionDays, options.Now)
		if err != nil {
			return plan, err
		}
		plan.Candidates = append(plan.Candidates, items...)
		plan.ScannedCount += scanned
		plan.SkippedCount += skipped
		plan.rootSnapshots[options.OutputDir] = snapshot
	}
	sort.Slice(plan.Candidates, func(i, j int) bool {
		if plan.Candidates[i].Kind != plan.Candidates[j].Kind {
			return plan.Candidates[i].Kind < plan.Candidates[j].Kind
		}
		return plan.Candidates[i].SessionID < plan.Candidates[j].SessionID
	})
	return plan, nil
}

func scanRoot(ctx context.Context, root, kind string, days int, now time.Time) ([]Candidate, int, int, rootSnapshot, error) {
	items := []Candidate{}
	snapshot, exists, err := inspectRoot(root)
	if err != nil {
		return items, 0, 0, snapshot, fmt.Errorf("inspect %s root: %w", kind, err)
	}
	if !exists {
		return items, 0, 0, snapshot, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return items, 0, 0, snapshot, fmt.Errorf("list %s root: %w", kind, err)
	}
	scanned, skipped := 0, 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, scanned, skipped, snapshot, err
		}
		if !validSessionID(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
			skipped++
			continue
		}
		candidate, safe, err := inspectCandidate(root, entry.Name(), kind)
		if err != nil {
			return nil, scanned, skipped, snapshot, err
		}
		if !safe {
			skipped++
			continue
		}
		scanned++
		if eligible(candidate.LastModified, days, now) {
			items = append(items, candidate)
		}
	}
	if err := verifyRootSnapshot(root, snapshot); err != nil {
		return nil, scanned, skipped, snapshot, err
	}
	return items, scanned, skipped, snapshot, nil
}

func rejectSymlinkedPathComponents(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absolute = filepath.Clean(absolute)
	volume := filepath.VolumeName(absolute)
	rest := strings.TrimPrefix(absolute, volume)
	root := volume + string(filepath.Separator)
	parts := strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == '\\' })
	current := root
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %q is a symlink", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("path component %q is not a directory", current)
		}
	}
	return nil
}

func inspectRoot(root string) (rootSnapshot, bool, error) {
	if err := rejectSymlinkedPathComponents(root); err != nil {
		return rootSnapshot{}, false, err
	}
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return rootSnapshot{}, false, nil
	}
	if err != nil {
		return rootSnapshot{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return rootSnapshot{}, false, errors.New("retention root must be a regular directory, not a symlink or file")
	}
	return rootSnapshot{exists: true, info: info}, true, nil
}

func verifyRootSnapshot(root string, expected rootSnapshot) error {
	current, exists, err := inspectRoot(root)
	if err != nil {
		return err
	}
	if exists != expected.exists {
		return errors.New("retention root existence changed since planning")
	}
	if exists && !os.SameFile(expected.info, current.info) {
		return errors.New("retention root identity changed since planning")
	}
	return nil
}

func inspectCandidate(root, id, kind string) (Candidate, bool, error) {
	path := filepath.Join(root, id)
	info, err := os.Lstat(path)
	if err != nil {
		return Candidate{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Candidate{}, false, nil
	}
	candidate := Candidate{Kind: kind, SessionID: id, Path: path, LastModified: info.ModTime(), fileInfo: info}
	h := sha256.New()
	addFingerprint(h, ".", info)
	safe, seenFiles := true, 0
	hasRendered, hasCaptures := false, false
	add := func(name string, fileInfo os.FileInfo) {
		addFingerprint(h, name, fileInfo)
		if fileInfo.ModTime().After(candidate.LastModified) {
			candidate.LastModified = fileInfo.ModTime()
		}
		if fileInfo.Mode().IsRegular() {
			candidate.SizeBytes += fileInfo.Size()
			candidate.FileCount++
			seenFiles++
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return Candidate{}, false, err
	}
	switch kind {
	case KindSession:
		for _, entry := range entries {
			itemPath := filepath.Join(path, entry.Name())
			item, statErr := os.Lstat(itemPath)
			if statErr != nil {
				return Candidate{}, false, statErr
			}
			switch entry.Name() {
			case "rendered.jpg":
				if item.Mode()&os.ModeSymlink != 0 || !item.Mode().IsRegular() || item.Size() == 0 {
					safe = false
					continue
				}
				hasRendered = true
				add(entry.Name(), item)
			case "captures":
				if item.Mode()&os.ModeSymlink != 0 || !item.IsDir() {
					safe = false
					continue
				}
				hasCaptures = true
				add(entry.Name(), item)
				captures, readErr := os.ReadDir(itemPath)
				if readErr != nil {
					return Candidate{}, false, readErr
				}
				for captureIndex, capture := range captures {
					capturePath := filepath.Join(itemPath, capture.Name())
					captureInfo, captureErr := os.Lstat(capturePath)
					if captureErr != nil {
						return Candidate{}, false, captureErr
					}
					expectedName := fmt.Sprintf("capture-%02d.jpg", captureIndex+1)
					if capture.Name() != expectedName || !validCaptureName(capture.Name()) ||
						captureInfo.Mode()&os.ModeSymlink != 0 || !captureInfo.Mode().IsRegular() || captureInfo.Size() == 0 {
						safe = false
						continue
					}
					add(filepath.Join("captures", capture.Name()), captureInfo)
				}
			default:
				safe = false
			}
		}
	case KindOutput:
		if len(entries) != 1 || entries[0].Name() != "photo.jpg" {
			return Candidate{}, false, nil
		}
		item, statErr := os.Lstat(filepath.Join(path, "photo.jpg"))
		if statErr != nil {
			return Candidate{}, false, statErr
		}
		if item.Mode()&os.ModeSymlink != 0 || !item.Mode().IsRegular() || item.Size() == 0 {
			return Candidate{}, false, nil
		}
		add("photo.jpg", item)
	default:
		return Candidate{}, false, fmt.Errorf("unsupported retention kind %q", kind)
	}
	if kind == KindSession && (!hasRendered || !hasCaptures) {
		// Incomplete sessions may still be diagnostics/recovery material or an
		// in-flight capture. Only finalized session layouts are retention candidates.
		safe = false
	}
	if !safe || seenFiles == 0 {
		return Candidate{}, false, nil
	}
	candidate.fingerprint = hex.EncodeToString(h.Sum(nil))
	return candidate, true, nil
}

func validSessionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validCaptureName(name string) bool {
	if len(name) != len("capture-01.jpg") || !strings.HasPrefix(name, "capture-") || !strings.HasSuffix(name, ".jpg") {
		return false
	}
	return name[8] >= '0' && name[8] <= '9' && name[9] >= '0' && name[9] <= '9' &&
		name[8:10] != "00" && name[8:10] <= "12"
}

func addFingerprint(h interface{ Write([]byte) (int, error) }, name string, info os.FileInfo) {
	_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\x00%o\n", name, info.Size(), info.ModTime().UnixNano(), info.Mode())
}

func eligible(modified time.Time, days int, now time.Time) bool {
	if days < 1 || modified.After(now) {
		return false
	}
	return !modified.After(now.Add(-time.Duration(days) * 24 * time.Hour))
}

func rootForKind(options Options, kind string) string {
	if kind == KindSession {
		return options.SessionsDir
	}
	return options.OutputDir
}

func retentionDays(policy Policy, kind string) int {
	if kind == KindSession {
		return policy.SessionRetentionDays
	}
	return policy.OutputRetentionDays
}

func sameCandidates(left, right []Candidate) bool {
	if len(left) != len(right) {
		return false
	}
	byKey := make(map[string]Candidate, len(left))
	for _, item := range left {
		byKey[item.Kind+"/"+item.SessionID] = item
	}
	for _, item := range right {
		old, ok := byKey[item.Kind+"/"+item.SessionID]
		if !ok || old.fingerprint != item.fingerprint || !os.SameFile(old.fileInfo, item.fileInfo) {
			return false
		}
	}
	return true
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// Apply removes only expired directories present in a reviewed BuildPlan. It
// re-scans the whole candidate set and each target immediately before removal;
// if any candidate changes, it aborts rather than recalculating a broader set.
func Apply(ctx context.Context, plan Plan, confirmationDigest string) (ApplyResult, error) {
	result := ApplyResult{Removed: []Candidate{}}
	if plan.options.Now.IsZero() || (plan.rootSnapshots == nil && !plan.Disabled) {
		return result, errors.New("invalid retention plan; call BuildPlan first")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if plan.Disabled || len(plan.Candidates) == 0 {
		return result, nil
	}
	plannedDigest, err := PlanDigest(plan)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(confirmationDigest) == "" ||
		!strings.EqualFold(strings.TrimSpace(confirmationDigest), plannedDigest) {
		return result, errors.New("confirmation digest does not match this retention plan; no data was removed")
	}
	current, err := BuildPlan(ctx, plan.options)
	if err != nil {
		return result, err
	}
	if !sameCandidates(plan.Candidates, current.Candidates) {
		return result, errors.New("retention candidates changed since planning; no data was removed; rebuild and review the plan")
	}
	for _, planned := range plan.Candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		root := rootForKind(plan.options, planned.Kind)
		if err := verifyRootSnapshot(root, plan.rootSnapshots[root]); err != nil {
			return result, fmt.Errorf("refuse cleanup for %s %s: %w", planned.Kind, planned.SessionID, err)
		}
		fresh, safe, err := inspectCandidate(root, planned.SessionID, planned.Kind)
		if err != nil {
			return result, err
		}
		if !safe || fresh.fingerprint != planned.fingerprint || !os.SameFile(fresh.fileInfo, planned.fileInfo) ||
			!eligible(fresh.LastModified, retentionDays(plan.options.Policy, planned.Kind), plan.options.Now) {
			return result, fmt.Errorf("candidate %s %s changed immediately before cleanup; remaining items were left untouched", planned.Kind, planned.SessionID)
		}
		// Root-scoped operations keep every subsequent path beneath the directory
		// handle even if an ancestor is concurrently renamed or replaced.
		rootHandle, err := os.OpenRoot(root)
		if err != nil {
			return result, fmt.Errorf("open retention root: %w", err)
		}
		openedRootInfo, statErr := rootHandle.Stat(".")
		if statErr != nil || !os.SameFile(plan.rootSnapshots[root].info, openedRootInfo) {
			_ = rootHandle.Close()
			return result, fmt.Errorf("retention root %q changed before deletion", root)
		}
		if err := removeCandidateUnderRoot(rootHandle, planned.SessionID, planned.Kind, planned.fileInfo,
			planned.fingerprint, retentionDays(plan.options.Policy, planned.Kind), plan.options.Now); err != nil {
			_ = rootHandle.Close()
			return result, fmt.Errorf("remove expired %s %s: %w", planned.Kind, planned.SessionID, err)
		}
		if err := rootHandle.Close(); err != nil {
			return result, fmt.Errorf("close retention root: %w", err)
		}
		result.Removed = append(result.Removed, planned)
	}
	return result, nil
}

type removalEntry struct {
	path string
	info os.FileInfo
}

// removeCandidateUnderRoot only removes a known PhotoBooth layout. os.Root keeps
// every relative path within the opened root, and every child is checked again
// before it is removed. Unexpected entries cause a fail-closed result.
func removeCandidateUnderRoot(root *os.Root, id, kind string, expectedDir os.FileInfo, expectedFingerprint string, retentionDays int, now time.Time) error {
	dirInfo, err := root.Lstat(id)
	if err != nil {
		return err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() || !os.SameFile(expectedDir, dirInfo) {
		return errors.New("candidate directory was replaced or is not a regular directory")
	}
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(id))
	if err != nil {
		return err
	}
	remove := make([]removalEntry, 0, 16)
	h := sha256.New()
	addFingerprint(h, ".", dirInfo)
	latest := dirInfo.ModTime()
	addObserved := func(name string, info os.FileInfo) {
		addFingerprint(h, name, info)
		latest = maxTime(latest, info.ModTime())
	}
	addFile := func(relative, fingerprintName string, requireNonEmpty bool) (os.FileInfo, error) {
		info, statErr := root.Lstat(relative)
		if statErr != nil {
			return nil, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || (requireNonEmpty && info.Size() == 0) {
			return nil, fmt.Errorf("unexpected or unsafe file %q", relative)
		}
		addObserved(fingerprintName, info)
		remove = append(remove, removalEntry{path: relative, info: info})
		return info, nil
	}
	addDir := func(relative, fingerprintName string) (os.FileInfo, error) {
		info, statErr := root.Lstat(relative)
		if statErr != nil {
			return nil, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("unexpected or unsafe directory %q", relative)
		}
		addObserved(fingerprintName, info)
		return info, nil
	}
	switch kind {
	case KindSession:
		hasRendered, hasCaptures := false, false
		captureFiles := []removalEntry{}
		var captureDir, renderedFile removalEntry
		for _, entry := range entries {
			relative := filepath.Join(id, entry.Name())
			switch entry.Name() {
			case "rendered.jpg":
				_, err := addFile(relative, entry.Name(), true)
				if err != nil {
					return err
				}
				renderedFile = remove[len(remove)-1]
				remove = remove[:len(remove)-1]
				hasRendered = true
			case "captures":
				info, err := addDir(relative, entry.Name())
				if err != nil {
					return err
				}
				captureDir = removalEntry{path: relative, info: info}
				captures, err := fs.ReadDir(root.FS(), filepath.ToSlash(relative))
				if err != nil {
					return err
				}
				if len(captures) == 0 || len(captures) > 12 {
					return errors.New("capture directory has an unexpected number of files")
				}
				for i, capture := range captures {
					if capture.Name() != fmt.Sprintf("capture-%02d.jpg", i+1) || !validCaptureName(capture.Name()) {
						return fmt.Errorf("unexpected capture filename %q", capture.Name())
					}
					capturePath := filepath.Join(relative, capture.Name())
					captureInfo, err := root.Lstat(capturePath)
					if err != nil {
						return err
					}
					if captureInfo.Mode()&os.ModeSymlink != 0 || !captureInfo.Mode().IsRegular() || captureInfo.Size() == 0 {
						return fmt.Errorf("unexpected or unsafe capture %q", capturePath)
					}
					addObserved(filepath.Join("captures", capture.Name()), captureInfo)
					captureFiles = append(captureFiles, removalEntry{path: capturePath, info: captureInfo})
				}
				hasCaptures = true
			default:
				return fmt.Errorf("unexpected entry %q in session; refusing cleanup", entry.Name())
			}
		}
		if !hasRendered || !hasCaptures {
			return errors.New("session is incomplete; refusing cleanup")
		}
		remove = append(remove, captureFiles...)
		remove = append(remove, captureDir, renderedFile, removalEntry{path: id, info: dirInfo})
	case KindOutput:
		if len(entries) != 1 || entries[0].Name() != "photo.jpg" {
			return errors.New("output directory is incomplete or contains temporary/unknown files; refusing cleanup")
		}
		_, err := addFile(filepath.Join(id, "photo.jpg"), "photo.jpg", true)
		if err != nil {
			return err
		}
		remove = append(remove, removalEntry{path: id, info: dirInfo})
	default:
		return fmt.Errorf("unsupported retention kind %q", kind)
	}
	if hex.EncodeToString(h.Sum(nil)) != expectedFingerprint {
		return errors.New("candidate contents changed after the last validation; refusing cleanup")
	}
	if !eligible(latest, retentionDays, now) {
		return errors.New("candidate contains data newer than the configured retention period; refusing cleanup")
	}
	return removeCheckedEntries(root, remove)
}

func removeCheckedEntries(root *os.Root, entries []removalEntry) error {
	for _, planned := range entries {
		current, err := root.Lstat(planned.path)
		if err != nil {
			return err
		}
		if current.Mode()&os.ModeSymlink != 0 || !os.SameFile(planned.info, current) ||
			current.Mode() != planned.info.Mode() {
			return fmt.Errorf("entry %q changed before removal", planned.path)
		}
		if !planned.info.IsDir() &&
			(current.Size() != planned.info.Size() || !current.ModTime().Equal(planned.info.ModTime())) {
			return fmt.Errorf("file %q metadata changed before removal", planned.path)
		}
		if planned.info.IsDir() {
			children, err := fs.ReadDir(root.FS(), filepath.ToSlash(planned.path))
			if err != nil {
				return err
			}
			if len(children) != 0 {
				return fmt.Errorf("directory %q gained unexpected entries; refusing removal", planned.path)
			}
		}
		if err := root.Remove(planned.path); err != nil {
			return fmt.Errorf("remove %q: %w", planned.path, err)
		}
	}
	return nil
}
