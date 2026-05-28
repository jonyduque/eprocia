package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"eprocia/internal/config"
	"eprocia/internal/etl"
	"eprocia/internal/models"
	"eprocia/internal/watchdog"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	watcher  *watchdog.Watcher
	pipeline *etl.Pipeline
	cfg      *models.Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{}
	app.watcher = watchdog.NewWatcher(func(filePath string) {
		// Run processing in background goroutine to not block the file watcher
		go func() {
			if app.pipeline != nil && app.cfg != nil {
				_, _ = app.pipeline.ProcessZip(filePath, app.cfg)
			}
		}()
	})
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.pipeline = etl.NewPipeline(ctx)

	// Load configuration
	cfg, err := config.Load()
	if err == nil {
		a.cfg = cfg
		// Start watchdog if path is set
		if cfg.MonitoredPath != "" {
			_ = a.watcher.Start(cfg.MonitoredPath)
		}
	}
}

// shutdown is called when the application closes
func (a *App) shutdown(ctx context.Context) {
	if a.watcher != nil {
		a.watcher.Stop()
	}
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

// GetConfig returns the current configuration settings.
func (a *App) GetConfig() (*models.Config, error) {
	if a.cfg != nil {
		return a.cfg, nil
	}
	return config.Load()
}

// SaveConfig saves the configuration settings and updates the watchdog path if it has changed.
func (a *App) SaveConfig(cfg *models.Config) error {
	err := config.Save(cfg)
	if err != nil {
		return err
	}

	oldPath := ""
	if a.cfg != nil {
		oldPath = a.cfg.MonitoredPath
	}
	a.cfg = cfg

	if cfg.MonitoredPath != oldPath {
		if cfg.MonitoredPath != "" {
			_ = a.watcher.Start(cfg.MonitoredPath)
		} else {
			a.watcher.Stop()
		}
	}

	return nil
}

// ProcessFile manually triggers the consolidation process for a ZIP file (e.g. from drag & drop)
func (a *App) ProcessFile(filePath string) (string, error) {
	if a.cfg == nil {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		a.cfg = cfg
	}

	return a.pipeline.ProcessZip(filePath, a.cfg)
}

// OpenDirectory opens the native Windows Explorer selecting the file, or opens the directory.
func (a *App) OpenDirectory(path string) error {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err == nil && !info.IsDir() {
		// If it's a file, select it in Windows Explorer
		return exec.Command("explorer.exe", "/select,", cleanPath).Start()
	}
	return exec.Command("explorer.exe", cleanPath).Start()
}

// Greet returns a greeting for the given name (default Wails method)
func (a *App) Greet(name string) string {
	return "Olá " + name + "! Bem-vindo ao eprocIA."
}
