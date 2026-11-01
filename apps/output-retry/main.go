package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "output-retry:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("output-retry", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "station schema-v1 config used by the failed session")
	sessionID := flags.String("session-id", "", "32-character lowercase hexadecimal session ID")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if strings.TrimSpace(*configPath) == "" {
		return errors.New("--config is required; refusing to guess a station's storage paths")
	}

	cfg, err := stationconfig.Load(*configPath)
	if err != nil {
		return err
	}
	savedPath, err := output.RetryLocalFromSession(
		context.Background(),
		cfg.Storage.SessionsDir,
		cfg.Storage.OutputDir,
		*sessionID,
		cfg.Capture.Shots,
	)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "OUTPUT_RETRY_SUCCEEDED\nSESSION_ID=%s\nOUTPUT=%s\n", *sessionID, savedPath)
	return nil
}
