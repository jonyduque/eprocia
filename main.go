package main

import (
	"context"
	"embed"
	"log"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Run systray in a goroutine
	go func() {
		systray.Run(onReady(app), onExit)
	}()

	// Run the application
	err := wails.Run(&options.App{
		Title:  "eprocIA",
		Width:  1024,
		Height: 768,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 248, G: 249, B: 250, A: 1},
		OnStartup:        app.startup,
		OnShutdown: func(ctx context.Context) {
			app.shutdown(ctx)
			systray.Quit()
		},
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func onReady(app *App) func() {
	return func() {
		systray.SetIcon(DefaultIcon)
		systray.SetTitle("eprocIA")
		systray.SetTooltip("eprocIA - ETL Judicial")

		mShow := systray.AddMenuItem("Abrir", "Abrir o eprocIA")
		mHide := systray.AddMenuItem("Ocultar", "Ocultar a janela")
		mQuit := systray.AddMenuItem("Sair", "Sair do aplicativo")

		go func() {
			for {
				select {
				case _, ok := <-mShow.ClickedCh:
					if !ok {
						return
					}
					app.Show()
				case _, ok := <-mHide.ClickedCh:
					if !ok {
						return
					}
					app.Hide()
				case _, ok := <-mQuit.ClickedCh:
					if !ok {
						return
					}
					systray.Quit()
					app.Quit()
					return
				}
			}
		}()
	}
}

func onExit() {
	// Cleanup on exit
}
