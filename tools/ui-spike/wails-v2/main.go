package main

import (
	"embed"
	"io/fs"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var embeddedAssets embed.FS

func main() {
	app := NewApp()
	benchmarkMode := slices.Contains(os.Args, "--benchmark")

	assets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		panic(err)
	}

	err = wails.Run(&options.App{
		Title:            "PhotoBooth Kiosk UI Spike",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		DisableResize:    false,
		StartHidden:      benchmarkMode,
		BackgroundColour: &options.RGBA{R: 14, G: 17, B: 27, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("PhotoBooth Kiosk UI spike:", err.Error())
		os.Exit(1)
	}
}
