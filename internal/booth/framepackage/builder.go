package framepackage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// CreateOptions builds a .pbframe from a schema-v1 definition and optional
// preview/license text. It never modifies the source frame or preview.
type CreateOptions struct {
	FramePath      string
	OutputPath     string
	PackageID      string
	PackageVersion string
	Name           string
	Description    string
	Author         string
	License        string
	PreviewPath    string
	LicensePath    string
}

// CreatePackage creates a validated, code-free package archive using a temporary
// file and a no-overwrite publish step. Package versions are immutable.
func CreatePackage(ctx context.Context, options CreateOptions) (PackageInfo, error) {
	if err := ctx.Err(); err != nil {
		return PackageInfo{}, err
	}
	framePath := strings.TrimSpace(options.FramePath)
	outputPath := strings.TrimSpace(options.OutputPath)
	if framePath == "" || outputPath == "" {
		return PackageInfo{}, errors.New("frame input and package output paths are required")
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ArchiveExtension) {
		return PackageInfo{}, fmt.Errorf("package output must use the %s extension", ArchiveExtension)
	}
	if !validPackageID(options.PackageID) {
		return PackageInfo{}, errors.New("package ID must use lowercase letters, digits and hyphens (1–63 characters)")
	}
	if !validVersion(options.PackageVersion) {
		return PackageInfo{}, errors.New("package version must use numeric major.minor.patch form, such as 1.0.0")
	}

	payloads := map[string][]byte{}
	frameBytes, err := readRegularFile(framePath, maxDefinitionBytes)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("read frame definition: %w", err)
	}
	slots, err := validateFrameBytes(ctx, frameBytes)
	if err != nil {
		return PackageInfo{}, fmt.Errorf("validate frame definition: %w", err)
	}
	payloads["definition.json"] = frameBytes

	if strings.TrimSpace(options.PreviewPath) != "" {
		previewPath := strings.TrimSpace(options.PreviewPath)
		ext := strings.ToLower(filepath.Ext(previewPath))
		name := ""
		switch ext {
		case ".png":
			name = "preview.png"
		case ".jpg", ".jpeg":
			name = "preview.jpg"
		default:
			return PackageInfo{}, errors.New("preview must be a PNG or JPEG file")
		}
		data, err := readRegularFile(previewPath, maxPreviewBytes)
		if err != nil {
			return PackageInfo{}, fmt.Errorf("read package preview: %w", err)
		}
		if err := validateOptionalPayloads(map[string][]byte{name: data}); err != nil {
			return PackageInfo{}, err
		}
		payloads[name] = data
	}
	if strings.TrimSpace(options.LicensePath) != "" {
		data, err := readRegularFile(strings.TrimSpace(options.LicensePath), maxLicenseBytes)
		if err != nil {
			return PackageInfo{}, fmt.Errorf("read package LICENSE.txt: %w", err)
		}
		if !utf8.Valid(data) {
			return PackageInfo{}, errors.New("LICENSE.txt must be UTF-8 text")
		}
		payloads["LICENSE.txt"] = data
	}

	manifest := Manifest{
		SchemaVersion:  PackageSchemaVersion,
		PackageType:    "frame",
		PackageID:      options.PackageID,
		PackageVersion: options.PackageVersion,
		Name:           options.Name,
		Description:    options.Description,
		Author:         options.Author,
		License:        options.License,
		EntryPoint:     "definition.json",
	}
	for _, name := range []string{"definition.json", "preview.png", "preview.jpg", "LICENSE.txt"} {
		data, ok := payloads[name]
		if !ok {
			continue
		}
		sum := sha256.Sum256(data)
		manifest.Files = append(manifest.Files, FileRecord{
			Path: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), MediaType: mediaTypeForPath(name),
		})
	}
	if err := validateManifest(manifest); err != nil {
		return PackageInfo{}, err
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return PackageInfo{}, fmt.Errorf("encode package manifest: %w", err)
	}
	if len(manifestBytes) > int(maxManifestBytes) {
		return PackageInfo{}, errors.New("encoded package manifest exceeds its size limit")
	}
	archive, err := createArchiveBytes(ctx, manifestBytes, payloads)
	if err != nil {
		return PackageInfo{}, err
	}
	if int64(len(archive)) > maxArchiveBytes {
		return PackageInfo{}, fmt.Errorf("created frame package exceeds %d bytes", maxArchiveBytes)
	}
	if err := writeArchiveNoReplace(outputPath, archive); err != nil {
		return PackageInfo{}, err
	}

	// Re-open the published result through the import validator. If this catches
	// a defect, remove only the archive created by this call.
	parsed, err := readArchive(ctx, outputPath)
	if err != nil {
		_ = os.Remove(outputPath)
		return PackageInfo{}, fmt.Errorf("verify newly created package: %w", err)
	}
	info := infoFromManifest(parsed.manifest, parsed.payloads, slots, framePath)
	if preview := previewName(parsed.payloads); preview != "" {
		info.PreviewPath = preview
	}
	return info, nil
}

func createArchiveBytes(ctx context.Context, manifest []byte, payloads map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	writeEntry := func(name string, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	}
	if err := writeEntry("manifest.json", manifest); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("write package manifest: %w", err)
	}
	for _, name := range []string{"definition.json", "preview.png", "preview.jpg", "LICENSE.txt"} {
		if err := ctx.Err(); err != nil {
			_ = writer.Close()
			return nil, err
		}
		if data, ok := payloads[name]; ok {
			if err := writeEntry(name, data); err != nil {
				_ = writer.Close()
				return nil, fmt.Errorf("write package payload %q: %w", name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish package archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func writeArchiveNoReplace(path string, data []byte) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve package output: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(absolute), ArchiveExtension) {
		return fmt.Errorf("package output must use the %s extension", ArchiveExtension)
	}
	if _, err := os.Lstat(absolute); err == nil {
		return errors.New("package output already exists; refusing to overwrite an immutable archive")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect package output: %w", err)
	}
	dir := filepath.Dir(absolute)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create package output directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".pbframe-build-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary package archive: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// A hard link publishes the fully synced temporary file atomically and
	// fails if another writer already created the destination. Both paths are in
	// the same directory, so they share a filesystem.
	if err := os.Link(tempPath, absolute); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("package output appeared during creation; refusing to overwrite it")
		}
		return fmt.Errorf("publish package archive without overwriting an existing file: %w", err)
	}
	return nil
}
