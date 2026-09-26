package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	_ "whatsbot/gui/devroot"
)

// Window bounds in pixels. The window opens at its minimum size and may grow to the maximum.
const (
	minWindowWidth  = 1024
	minWindowHeight = 768
	maxWindowWidth  = 1280
	maxWindowHeight = 800
)

// maxChannel is the full value of an 8-bit colour channel.
const maxChannel = 255

//go:embed all:frontend/build
var assets embed.FS

//go:embed frontend/build/appicon.png
var icon []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "gui",
		Width:             minWindowWidth,
		Height:            minWindowHeight,
		MinWidth:          minWindowWidth,
		MinHeight:         minWindowHeight,
		MaxWidth:          maxWindowWidth,
		MaxHeight:         maxWindowHeight,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: maxChannel, G: maxChannel, B: maxChannel, A: maxChannel},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:             nil,
		Logger:           nil,
		LogLevel:         logger.DEBUG,
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		WindowStartState: options.Normal,
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: "",
			ZoomFactor:          1.0,
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "gui",
				Message: "",
				Icon:    icon,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
