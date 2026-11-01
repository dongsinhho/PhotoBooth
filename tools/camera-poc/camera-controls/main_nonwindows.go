//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "camera-controls is a Windows Media Foundation diagnostic and only runs on Windows.")
	os.Exit(2)
}
