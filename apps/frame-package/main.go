package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dongsinhho/PhotoBooth/internal/booth/framepackage"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "frame-package:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("command required: create, validate, install, or list")
	}
	ctx := context.Background()
	switch args[0] {
	case "create":
		flags := flag.NewFlagSet("create", flag.ContinueOnError)
		flags.SetOutput(stderr)
		frame := flags.String("frame", "", "input schema-v1 frame JSON")
		output := flags.String("out", "", "new .pbframe output path (must not exist)")
		id := flags.String("id", "", "stable package ID: lowercase letters, digits, hyphens")
		version := flags.String("version", "", "immutable numeric version, for example 1.0.0")
		name := flags.String("name", "", "display name")
		description := flags.String("description", "", "short package description")
		author := flags.String("author", "", "creator attribution")
		license := flags.String("license", "", "creator-provided license identifier/text")
		preview := flags.String("preview", "", "optional local PNG/JPEG preview")
		licenseFile := flags.String("license-file", "", "optional local UTF-8 LICENSE.txt")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
		}
		info, err := framepackage.CreatePackage(ctx, framepackage.CreateOptions{
			FramePath: *frame, OutputPath: *output, PackageID: *id, PackageVersion: *version,
			Name: *name, Description: *description, Author: *author, License: *license,
			PreviewPath: *preview, LicensePath: *licenseFile,
		})
		if err != nil {
			return err
		}
		return writeJSON(stdout, info)
	case "validate":
		flags := flag.NewFlagSet("validate", flag.ContinueOnError)
		flags.SetOutput(stderr)
		path := flags.String("package", "", "local .pbframe file to validate")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
		}
		info, err := framepackage.NewManager("").Validate(ctx, *path)
		if err != nil {
			return err
		}
		return writeJSON(stdout, info)
	case "install":
		flags := flag.NewFlagSet("install", flag.ContinueOnError)
		flags.SetOutput(stderr)
		path := flags.String("package", "", "local .pbframe file to install")
		framesDir := flags.String("frames-dir", defaultFramesDir(), "local managed frames root")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
		}
		info, err := framepackage.NewManager(*framesDir).Install(ctx, *path)
		if err != nil {
			return err
		}
		return writeJSON(stdout, info)
	case "list":
		flags := flag.NewFlagSet("list", flag.ContinueOnError)
		flags.SetOutput(stderr)
		framesDir := flags.String("frames-dir", defaultFramesDir(), "local managed frames root")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
		}
		items, err := framepackage.NewManager(*framesDir).List(ctx)
		if err != nil {
			return err
		}
		return writeJSON(stdout, items)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func defaultFramesDir() string {
	base := strings.TrimSpace(os.Getenv("PHOTOBOOTH_DATA_DIR"))
	if base == "" {
		cacheDir, err := os.UserCacheDir()
		if err == nil {
			base = filepath.Join(cacheDir, "PhotoBooth")
		}
	}
	if base == "" {
		return filepath.Join("data", "PhotoBooth", "frames")
	}
	return filepath.Join(base, "frames")
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON result: %w", err)
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `PhotoBooth local frame-package tool (no network access)

Commands:
  create   Build a .pbframe from a validated frame JSON.
  validate Validate a .pbframe without installing it.
  install  Validate and install an immutable package version locally.
  list     List and re-verify installed package versions.

Example:
  go run ./apps/frame-package create --frame ./examples/frames/photo-strip-v1.json --out ./photo-strip-1.0.0.pbframe --id photo-strip --version 1.0.0 --name "Sample photo strip" --author "Your name" --license "CC0-1.0"
  go run ./apps/frame-package validate --package ./photo-strip-1.0.0.pbframe
  go run ./apps/frame-package install --package ./photo-strip-1.0.0.pbframe
  go run ./apps/frame-package list

Install destination defaults to the user's local PhotoBooth frames folder. Use --frames-dir to override it. The tool only packages declarative frame JSON; it does not execute scripts or fetch content.`)
}
