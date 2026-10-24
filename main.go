package main

import (
	"context"
	"embed"
	"log"
	"os"
	"runtime"

	"eprocia/internal/autostart"
	"eprocia/internal/config"

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
	app.cfg = carregarConfig(config.Load)

	go executarTray(func() {
		systray.Run(onReady(app), onExit)
	})

	// Run the application
	err := wails.Run(&options.App{
		Title:     "eprocIA",
		Width:     1024,
		Height:    768,
		MinWidth:  800,
		MinHeight: 600,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: corDeFundo(temaClaro(app.cfg.Theme, windowsUsaTemaClaro)),
		OnStartup:        app.startup,
		OnShutdown: func(ctx context.Context) {
			app.shutdown(ctx)
			systray.Quit()
		},
		HideWindowOnClose: true,
		StartHidden:       autostart.IniciarOculto(os.Args),
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

// executarTray roda o laço do tray. A janela oculta do systray recebe as
// mensagens Win32 só na thread que a criou, e GetMessage é bombeado nessa
// mesma thread; sem prender a goroutine, o runtime pode migrá-la e o tray
// deixa de responder.
func executarTray(run func()) {
	runtime.LockOSThread()
	run()
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

// corDeFundo espelha o --bg-main de style.css para a janela nativa não
// piscar outra cor antes do WebView pintar.
func corDeFundo(claro bool) *options.RGBA {
	if claro {
		return &options.RGBA{R: 0xF3, G: 0xF4, B: 0xF6, A: 255}
	}
	return &options.RGBA{R: 0x0B, G: 0x0F, B: 0x19, A: 255}
}

// temaClaro segue o tema escolhido no app; o do Windows só vale para
// "system" ou config antiga sem tema.
func temaClaro(tema string, windowsClaro func() bool) bool {
	switch tema {
	case "light":
		return true
	case "dark":
		return false
	}
	return windowsClaro()
}

func onExit() {
	// Cleanup on exit
}
