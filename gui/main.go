package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed frontend/dist/*
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "elevate" {
		app := NewApp()
		if err := app.RunElevated(); err != nil {
			fmt.Fprintf(os.Stderr, "Elevate 模式错误: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "OpenHijack",
		Width:            1200,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        false,
		DisableResize:    false,
		StartHidden:      false,
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
			WebviewGpuPolicy:     linux.WebviewGpuPolicyAlways,
			ProgramName:          "OpenHijack",
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "OpenHijack",
				Message: "本地 HTTPS 代理服务器",
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}
