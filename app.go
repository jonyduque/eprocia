package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"eprocia/internal/autostart"
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
	// autostart é trocado nos testes para não mexer no registro real.
	autostart func(ativo bool) error
	// clipboard é trocado nos testes; o padrão é runtime.ClipboardSetText.
	clipboard func(ctx context.Context, texto string) error
	// notificar fica nil quando o sistema não oferece notificações.
	notificar func(titulo, corpo string) error
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{autostart: autostart.Aplicar, clipboard: runtime.ClipboardSetText}
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
	a.pipeline = etl.NewPipeline()
	a.queue = etl.NewQueue(a.pipeline)
	a.queue.SetContext(ctx)
	a.iniciarNotificacoes(ctx)
	a.queue.SetOnFinished(a.aoTerminar)

	cfg := carregarConfig(config.Load)
	a.cfg = cfg
	a.queue.SetConfig(cfg)
	a.sincronizarAutostart(cfg)
	if cfg.WatchdogEnabled && cfg.MonitoredPath != "" {
		if err := a.watcher.Start(cfg.MonitoredPath); err != nil {
			log.Printf("Erro ao iniciar o monitoramento da pasta %q: %v", cfg.MonitoredPath, err)
			runtime.EventsEmit(ctx, "watchdog:error", err.Error())
		}
	}
}

// sincronizarAutostart alinha o registro à config no boot: a entrada pode ter
// sido apagada por fora ou apontar para um executável que mudou de lugar.
func (a *App) sincronizarAutostart(cfg *models.Config) {
	if err := a.autostart(cfg.AutoStart); err != nil {
		log.Printf("Erro ao sincronizar a inicialização com o Windows: %v", err)
	}
}

func (a *App) iniciarNotificacoes(ctx context.Context) {
	if err := runtime.InitializeNotifications(ctx); err != nil {
		log.Printf("Aviso: notificações indisponíveis: %v", err)
		return
	}
	if !runtime.IsNotificationAvailable(ctx) {
		return
	}
	a.notificar = func(titulo, corpo string) error {
		return runtime.SendNotification(ctx, runtime.NotificationOptions{
			ID:    fmt.Sprintf("eprocia-%d", time.Now().UnixNano()),
			Title: titulo,
			Body:  corpo,
		})
	}
}

// aoTerminar avisa o fim de um item mesmo com a janela oculta na bandeja.
func (a *App) aoTerminar(item *etl.QueueItem) {
	if a.notificar == nil {
		return
	}
	titulo, corpo := mensagemFim(item)
	if err := a.notificar(titulo, corpo); err != nil {
		log.Printf("Erro ao enviar notificação de %s: %v", item.Name, err)
	}
}

func mensagemFim(item *etl.QueueItem) (titulo, corpo string) {
	if item.Status == "success" {
		return "eprocIA", fmt.Sprintf("%s pronto (%d docs, %d págs)", filepath.Base(item.OutputPath), item.DocsCount, item.PagesCount)
	}
	motivo := item.Error
	if motivo == "" {
		motivo = item.Message
	}
	return "eprocIA", fmt.Sprintf("Erro ao processar %s: %s", item.Name, motivo)
}

// carregarConfig nunca devolve nil: se a leitura falhar (ex.: JSON corrompido),
// registra o aviso e segue com a configuração padrão.
func carregarConfig(load func() (*models.Config, error)) *models.Config {
	cfg, err := load()
	if err == nil && cfg != nil {
		return cfg
	}
	log.Printf("Aviso: falha ao carregar a configuração (%v); usando a configuração padrão", err)
	padrao, _ := config.DefaultConfig()
	return padrao
}

// shutdown is called when the application closes
func (a *App) shutdown(ctx context.Context) {
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.queue != nil {
		a.queue.Shutdown()
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

	autoAnterior := a.cfg != nil && a.cfg.AutoStart
	a.cfg = cfg
	if a.queue != nil {
		a.queue.SetConfig(cfg)
	}

	var errAuto error
	if cfg.AutoStart != autoAnterior {
		if err := a.autostart(cfg.AutoStart); err != nil {
			log.Printf("Erro ao alterar a inicialização com o Windows: %v", err)
			errAuto = fmt.Errorf("configurações salvas, mas não foi possível alterar a inicialização com o Windows: %w", err)
		}
	}

	// Start é no-op se a pasta já está sendo monitorada; chamá-lo sempre permite
	// tentar de novo depois de uma falha mesmo sem mudar o caminho.
	if cfg.WatchdogEnabled && cfg.MonitoredPath != "" {
		if err := a.watcher.Start(cfg.MonitoredPath); err != nil {
			log.Printf("Erro ao iniciar o monitoramento da pasta %q: %v", cfg.MonitoredPath, err)
			return fmt.Errorf("configurações salvas, mas não foi possível monitorar a pasta %q: %w", cfg.MonitoredPath, err)
		}
	} else {
		a.watcher.Stop()
	}

	return errAuto
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

// CopyOutput copia para a área de transferência o TXT gerado por um item concluído.
func (a *App) CopyOutput(path string) error {
	if a.ctx == nil {
		return fmt.Errorf("contexto não inicializado")
	}
	texto, err := lerSaidaParaCopiar(path, a.GetQueue())
	if err != nil {
		return err
	}
	if err := a.clipboard(a.ctx, texto); err != nil {
		return fmt.Errorf("não foi possível copiar para a área de transferência: %w", err)
	}
	return nil
}

// lerSaidaParaCopiar só lê arquivos que a própria fila gerou, para que o
// binding exposto ao WebView não sirva para ler qualquer arquivo do disco.
func lerSaidaParaCopiar(path string, itens []*etl.QueueItem) (string, error) {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".txt") {
		return "", fmt.Errorf("arquivo %q não é uma saída .txt", path)
	}
	limpo := filepath.Clean(path)
	for _, it := range itens {
		if it.Status == "success" && it.OutputPath != "" && strings.EqualFold(filepath.Clean(it.OutputPath), limpo) {
			dados, err := os.ReadFile(limpo)
			if err != nil {
				return "", fmt.Errorf("não foi possível ler %q: %w", limpo, err)
			}
			return string(dados), nil
		}
	}
	return "", fmt.Errorf("arquivo %q não corresponde a nenhum processamento concluído", path)
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
