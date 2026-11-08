package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dongsinhho/PhotoBooth/internal/booth/camera/v4l2"
)

func main() {
	report := v4l2.Inventory()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(report.Devices) == 0 {
		os.Exit(2)
	}
	for _, device := range report.Devices {
		if !device.ReadWrite {
			os.Exit(3)
		}
	}
}
