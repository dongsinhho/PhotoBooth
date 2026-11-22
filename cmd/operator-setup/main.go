package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dongsinhho/PhotoBooth/internal/operatorauth"
	"golang.org/x/term"
)

func main() {
	resetMode := false
	if len(os.Args) == 2 && os.Args[1] == "--reset" {
		resetMode = true
	} else if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: operator-setup [--reset]")
		os.Exit(2)
	}
	if err := run(resetMode); err != nil {
		fmt.Fprintln(os.Stderr, "Operator setup failed:", err)
		os.Exit(1)
	}
}

func run(resetMode bool) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("run this command from an interactive terminal; PIN input must not be piped or echoed")
	}
	dataDir := strings.TrimSpace(os.Getenv("PHOTOBOOTH_DATA_DIR"))
	if dataDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("resolve current OS user's cache directory: %w", err)
		}
		dataDir = filepath.Join(cacheDir, "PhotoBooth")
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve PhotoBooth data directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create PhotoBooth data directory: %w", err)
	}

	storePath := operatorauth.StorePath(dataDir)
	controller := operatorauth.New(storePath)
	state, err := controller.Load()
	if err != nil {
		return err
	}
	if resetMode && !state.Configured {
		return operatorauth.ErrNotConfigured
	}
	if state.Configured && !resetMode {
		fmt.Println("An operator credential is already configured.")
		return errors.New("use Change station PIN in Operator Mode, or close PhotoBooth and run operator-setup --reset under the operator OS account")
	}

	reader := bufio.NewReader(os.Stdin)
	if resetMode {
		fmt.Println("PhotoBooth operator PIN recovery")
		fmt.Println("Close PhotoBooth before continuing. This recovery relies on control of the current OS account.")
		fmt.Println("Enter a new 8-digit PIN. The PIN will not be displayed.")
	} else {
		fmt.Println("PhotoBooth first-time operator setup")
		fmt.Println("Create an 8-digit PIN. The PIN will not be displayed.")
	}
	pin, err := readPIN(reader, "New PIN: ")
	if err != nil {
		return err
	}
	confirm, err := readPIN(reader, "Confirm PIN: ")
	if err != nil {
		return err
	}
	if pin != confirm {
		return errors.New("PIN entries do not match")
	}
	if strings.TrimSpace(pin) != pin {
		return operatorauth.ErrInvalidPIN
	}

	if resetMode {
		if err := controller.ResetFromOS(pin); err != nil {
			return err
		}
		fmt.Println("Operator PIN reset. Start PhotoBooth; it will open in Customer Mode.")
	} else {
		if err := controller.Setup(pin); err != nil {
			return err
		}
		fmt.Println("Operator credential created. Start PhotoBooth; it will open in Customer Mode.")
	}
	fmt.Println("Credential data is stored in the current OS user's PhotoBooth data directory.")
	return nil
}

func readPIN(_ *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return "", fmt.Errorf("read PIN securely: %w", err)
	}
	return string(value), nil
}
