package main

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Show shows the main window
func (a *App) Show() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
}

// Hide hides the main window
func (a *App) Hide() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// Quit terminates the application
func (a *App) Quit() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// Greet returns a greeting for the given name (default Wails method)
func (a *App) Greet(name string) string {
	return "Olá " + name + "! Bem-vindo ao eprocIA."
}
