package main

import (
	"context"
	"fmt"
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
	queue    *etl.Queue
	cfg      *models.Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{}
	app.watcher = watchdog.NewWatcher(func(filePath string) {
		if app.queue != nil {
			app.queue.AddFiles([]string{filePath})
		}
	})
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.pipeline = etl.NewPipeline(ctx)
	a.queue = etl.NewQueue(a.pipeline)
	a.queue.SetContext(ctx)

	// Load configuration
	cfg, err := config.Load()
	if err == nil {
		a.cfg = cfg
		a.queue.SetConfig(cfg)
		// Start watchdog if path is set and enabled
		if cfg.WatchdogEnabled && cfg.MonitoredPath != "" {
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
	oldEnabled := false
	if a.cfg != nil {
		oldPath = a.cfg.MonitoredPath
		oldEnabled = a.cfg.WatchdogEnabled
	}
	a.cfg = cfg
	if a.queue != nil {
		a.queue.SetConfig(cfg)
	}

	if cfg.MonitoredPath != oldPath || cfg.WatchdogEnabled != oldEnabled {
		if cfg.WatchdogEnabled && cfg.MonitoredPath != "" {
			_ = a.watcher.Start(cfg.MonitoredPath)
		} else {
			a.watcher.Stop()
		}
	}

	return nil
}

// ProcessFile adds a file to the processing queue for backward compatibility.
func (a *App) ProcessFile(filePath string) (string, error) {
	a.AddFilesToQueue([]string{filePath})
	return "", nil
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

// SelectDirectory opens a native open directory dialog and returns the selected path.
func (a *App) SelectDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("context not initialized")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecionar Pasta para Monitorar",
	})
}

// SelectZipFile opens a native file dialog for selecting a ZIP file (backward compatibility).
func (a *App) SelectZipFile() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("context not initialized")
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecionar Arquivo ZIP do Eproc",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Arquivos ZIP (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
}

// SelectZipFiles opens a native multiple files dialog and returns the selected paths.
func (a *App) SelectZipFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("context not initialized")
	}
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecionar Arquivos ZIP do Eproc",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Arquivos ZIP (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
}

// AddFilesToQueue adds files manually to the processing queue.
func (a *App) AddFilesToQueue(filePaths []string) {
	if a.queue != nil {
		a.queue.AddFiles(filePaths)
	}
}

// GetQueue returns the current list of items in the queue.
func (a *App) GetQueue() []*etl.QueueItem {
	if a.queue != nil {
		return a.queue.GetItems()
	}
	return nil
}

// PauseQueue pauses the queue worker.
func (a *App) PauseQueue() {
	if a.queue != nil {
		a.queue.Pause()
	}
}

// ResumeQueue resumes the queue worker.
func (a *App) ResumeQueue() {
	if a.queue != nil {
		a.queue.Resume()
	}
}

// IsQueuePaused returns true if the queue is currently paused.
func (a *App) IsQueuePaused() bool {
	if a.queue != nil {
		return a.queue.IsPaused()
	}
	return false
}

// CancelQueueItem cancels an item in the queue by its ID.
func (a *App) CancelQueueItem(id string) {
	if a.queue != nil {
		a.queue.CancelItem(id)
	}
}

// RemoveQueueItem removes an item from the queue by its ID.
func (a *App) RemoveQueueItem(id string) {
	if a.queue != nil {
		a.queue.RemoveItem(id)
	}
}

// ClearQueue cancels all processes and clears the queue list.
func (a *App) ClearQueue() {
	if a.queue != nil {
		a.queue.Clear()
	}
}

// Greet returns a greeting for the given name (default Wails method)
func (a *App) Greet(name string) string {
	return "Olá " + name + "! Bem-vindo ao eprocIA."
}
