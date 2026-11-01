package main

import (
	"embed"
	"io/fs"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var embeddedAssets embed.FS

func main() {
	app := NewApp()
	benchmarkMode := slices.Contains(os.Args, "--benchmark")
	kioskMode := kioskModeRequested() && !benchmarkMode
	app.setKioskMode(kioskMode)

	assets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		panic(err)
	}

	err = wails.Run(&options.App{
		Title:            "PhotoBooth · Windows Preview",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		DisableResize:    false,
		Fullscreen:       kioskMode,
		StartHidden:      benchmarkMode,
		BackgroundColour: &options.RGBA{R: 14, G: 17, B: 27, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		// Opt-in diagnostic only; normal launches retain hardware acceleration.
		Windows: &windows.Options{
			WebviewGpuIsDisabled: os.Getenv("PHOTOBOOTH_DISABLE_WEBVIEW_GPU") == "1",
		},
	})
	if err != nil {
		println("PhotoBooth Kiosk UI spike:", err.Error())
		os.Exit(1)
	}
}
