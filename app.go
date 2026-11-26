package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"eprocia/internal/atualizacao"
	"eprocia/internal/autostart"
	"eprocia/internal/config"
	"eprocia/internal/etl"
	"eprocia/internal/historico"
	"eprocia/internal/janela"
	"eprocia/internal/models"
	"eprocia/internal/registro"
	"eprocia/internal/saida"
	"eprocia/internal/watchdog"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	watcher  *watchdog.Watcher
	pipeline *etl.Pipeline
	queue    *etl.Queue
	// cfg é lido por goroutines (arranque da atualização) enquanto o
	// frontend grava: só por configAtual e guardarConfig. O objeto guardado
	// nunca é alterado; quem muda grava uma cópia.
	cfg atomic.Pointer[models.Config]
	// gravarMu serializa SaveConfig e SetTheme (ler, mudar, guardar).
	gravarMu sync.Mutex
	// monitorMu serializa ligar e desligar o watcher pela config;
	// monitorRetido fica verdadeiro no processo novo da atualização até o
	// antigo sair e a trava de instância ser obtida (arranqueApos).
	monitorMu     sync.Mutex
	monitorRetido atomic.Bool
	// autostart é trocado nos testes para não mexer no registro real.
	autostart func(ativo bool) error
	// clipboard é trocado nos testes; o padrão é runtime.ClipboardSetText.
	clipboard func(ctx context.Context, texto string) error
	// explorer é trocado nos testes; nil abre o explorer.exe de verdade.
	explorer func(args ...string) error
	// notificar fica nil quando o sistema não oferece notificações.
	notificar func(titulo, corpo string) error
	// falhas guarda os ZIPs com erro para a varredura inicial não repeti-los.
	falhas *watchdog.Falhas
	// animador desvanece a janela ao ir para a bandeja; nil antes do startup.
	animador atomic.Pointer[janela.Animador]
	// saindo é marcado por Quit: daí em diante OnBeforeClose deixa sair e
	// nada espera animação.
	saindo atomic.Bool
	// emitir é trocado nos testes; o padrão é runtime.EventsEmit.
	emitir func(evento string, dados ...any)
	// oculta acompanha a janela na bandeja para JanelaVisivel.
	oculta atomic.Bool
	// fecharTelemetria descarrega a telemetria no shutdown; nil com o log
	// desligado.
	fecharTelemetria func()

	// Atualização pelo aplicativo (app_atualizacao.go).
	atualizador *atualizacao.Atualizador
	// abrirURL é trocado nos testes; o padrão é runtime.BrowserOpenURL.
	abrirURL func(url string)
	// apos é o PID do processo antigo (--apos-atualizar); 0 = abertura normal.
	apos int
	// limparAnterior: --limpar-anterior, apagar eprocia.anterior.exe depois
	// de o antigo sair.
	limparAnterior  bool
	modo            atualizacao.Modo
	pastaInstalacao string
	// pastaDados é %AppData%\eprocIA (main); os testes passam t.TempDir().
	pastaDados string
	// travarDepois bloqueia até obter a trava de instância adiada; nil = já
	// obtida.
	travarDepois func() bool
	// esperarSair, gravarVersao e apagarAnterior são trocados nos testes;
	// apagarAnterior nil = atualizacao.LimparAnterior.
	esperarSair    func(pid int, prazo time.Duration) bool
	gravarVersao   func(string) error
	apagarAnterior func(pasta string) error
	sinalUmaVez    sync.Once
	limpezaUmaVez  sync.Once

	// Histórico da fila (app_historico.go). historicoMu protege historico,
	// historicoAdiado e historicoProtegido. historicoAdiado: processo novo da
	// atualização ainda à espera do antigo (o gravador só nasce depois);
	// historicoProtegido: o Protegido do gravador adiado.
	historicoMu        sync.Mutex
	historico          *historico.Gravador
	historicoAdiado    bool
	historicoProtegido bool
	// historicoFechadoTroca: o Fechar da troca de versão já rodou e o Reabrir
	// ainda não; ligar o interruptor nesse meio-tempo não cria gravador.
	historicoFechadoTroca bool
	// historicoFalhou (sob falhaHistoricoMu): uma gravação do histórico já
	// falhou nesta sessão. Limita o evento a um e fica para HistoricoFalhou,
	// porque o evento pode sair antes de o frontend registrar o EventsOn.
	falhaHistoricoMu sync.Mutex
	historicoFalhou  bool
}

// eventoVisibilidade avisa o frontend que a janela sumiu na bandeja (false)
// ou voltou (true). Oculta, a WebView2 segue renderizando: o frontend para as
// animações e segura a fila até reabrir.
const eventoVisibilidade = "janela:visivel"

// filaMonitorada é a parte da fila que o callback do watcher usa.
type filaMonitorada interface {
	ReprocessarMonitorado(string) bool
	AddFilesComOrigem([]string, string) int
}

// encaminharMonitorado leva um arquivo da pasta monitorada à fila: um PDF
// editado depois de convertido reaproveita o item concluído (q1); senão vira
// item novo.
func encaminharMonitorado(f filaMonitorada, caminho string) {
	if f.ReprocessarMonitorado(caminho) {
		return
	}
	f.AddFilesComOrigem([]string{caminho}, "monitorada")
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		autostart:    autostart.Aplicar,
		clipboard:    runtime.ClipboardSetText,
		esperarSair:  atualizacao.EsperarSair,
		gravarVersao: autostart.GravarVersaoDesinstalacao,
	}
	app.watcher = watchdog.NewWatcher(func(filePath string) {
		if app.queue != nil {
			encaminharMonitorado(app.queue, filePath)
		}
	})
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.animador.Store(janela.NovoAnimador(janela.Nativa(a.ocultarJanela)))
	a.iniciarNotificacoes(ctx)
	a.iniciarFila(ctx)
	a.iniciarAtualizacao(a.opcoesAtualizacao())
	a.sinalNoStartup()
}

// sincronizarAutostart alinha o registro à config no boot: a entrada pode ter
// sido apagada por fora ou apontar para um executável que mudou de lugar.
func (a *App) sincronizarAutostart(cfg *models.Config) {
	if err := a.autostart(cfg.AutoStart); err != nil {
		slog.Warn("falha ao sincronizar a inicialização com o Windows", "erro", err)
	}
}

func (a *App) iniciarNotificacoes(ctx context.Context) {
	if err := runtime.InitializeNotifications(ctx); err != nil {
		slog.Warn("notificações indisponíveis", "erro", err)
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
	if item.Status == "error" {
		a.falhas.Registrar(item.Path)
	} else {
		a.falhas.Esquecer(item.Path)
	}
	if a.notificar == nil {
		return
	}
	titulo, corpo := mensagemFim(item)
	if err := a.notificar(titulo, corpo); err != nil {
		slog.Warn("falha ao enviar a notificação", "item_id", item.ID, "zip", item.Name, "erro", err)
	}
}

func mensagemFim(item *etl.QueueItem) (titulo, corpo string) {
	if item.Status == "success" {
		nome := filepath.Base(item.OutputPath)
		switch item.UnreadDocs {
		case 0:
			return "eprocIA", fmt.Sprintf("%s pronto (%d docs, %d págs)", nome, item.DocsCount, item.PagesCount)
		case 1:
			return "eprocIA", fmt.Sprintf("%s pronto, com 1 documento não lido (%d docs, %d págs)", nome, item.DocsCount, item.PagesCount)
		default:
			return "eprocIA", fmt.Sprintf("%s pronto, com %d documentos não lidos (%d docs, %d págs)", nome, item.UnreadDocs, item.DocsCount, item.PagesCount)
		}
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
	slog.Warn("falha ao carregar a configuração; usando a configuração padrão", "erro", err)
	padrao, _ := config.DefaultConfig()
	return padrao
}

// shutdown is called when the application closes
func (a *App) shutdown(ctx context.Context) {
	// Antes de parar watcher e fila: cancela um download e impede que uma
	// troca comece durante a saída.
	if a.atualizador != nil {
		a.atualizador.Encerrar()
	}
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.queue != nil {
		a.queue.Shutdown()
	}
	if a.fecharTelemetria != nil {
		a.fecharTelemetria()
	}
}

// Show mostra a janela no estado em que foi ocultada (maximizada continua
// maximizada) e a traz para a frente; interrompe uma animação de ocultar.
func (a *App) Show() {
	if an := a.animador.Load(); an != nil {
		an.Cancelar()
	}
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
	a.avisarVisibilidade(true)
}

// ocultarJanela é o ocultar de fato, no fim da animação ou sem ela.
func (a *App) ocultarJanela() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
	a.avisarVisibilidade(false)
}

// iniciarOculta registra que a janela abre já na bandeja (--tray).
func (a *App) iniciarOculta(oculta bool) { a.oculta.Store(oculta) }

// JanelaVisivel diz ao frontend, ao montar, se a janela está à vista.
func (a *App) JanelaVisivel() bool { return !a.oculta.Load() }

func (a *App) avisarVisibilidade(visivel bool) {
	a.oculta.Store(!visivel)
	if a.emitir != nil {
		a.emitir(eventoVisibilidade, visivel)
	} else if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventoVisibilidade, visivel)
	}
}

// Hide manda a janela para a bandeja com a animação de saída (botão X,
// Alt+F4 e "Ocultar" da bandeja passam por aqui).
func (a *App) Hide() {
	if a.saindo.Load() {
		return
	}
	if an := a.animador.Load(); an != nil {
		an.Ocultar()
	} else {
		a.ocultarJanela()
	}
}

// Quit encerra o app na hora, sem esperar animação.
func (a *App) Quit() {
	a.saindo.Store(true)
	if an := a.animador.Load(); an != nil {
		an.Cancelar()
	}
	// O que já foi registrado chega ao arquivo antes de a saída começar.
	registro.Descarregar()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// antesDeFechar é o OnBeforeClose do Wails. Fechar a janela (Alt+F4, barra
// de tarefas) chega aqui e vira ocultar para a bandeja; só a saída marcada
// por Quit passa. Roda na thread da janela: a animação vai numa goroutine.
func (a *App) antesDeFechar(_ context.Context) (impedir bool) {
	if a.saindo.Load() {
		return false
	}
	go a.Hide()
	return true
}

// configAtual devolve a config em uso (nil antes de carregada). Não altere
// o objeto devolvido: grave uma cópia com guardarConfig.
func (a *App) configAtual() *models.Config { return a.cfg.Load() }

func (a *App) guardarConfig(cfg *models.Config) { a.cfg.Store(cfg) }

// GetConfig returns the current configuration settings.
func (a *App) GetConfig() (*models.Config, error) {
	if cfg := a.configAtual(); cfg != nil {
		return cfg, nil
	}
	return config.Load()
}

// SaveConfig grava a configuração e reaplica o watchdog e a inicialização com
// o Windows. O tema não vem do formulário: só SetTheme o altera.
func (a *App) SaveConfig(cfg *models.Config) error {
	a.gravarMu.Lock()
	defer a.gravarMu.Unlock()
	atual := a.configAtual()
	autoAnterior := atual != nil && atual.AutoStart
	historicoAnterior := atual != nil && atual.GuardarHistorico
	if atual != nil {
		cfg.Theme = atual.Theme
	}

	// O registro vem antes da gravação: se ele recusar, o config.json guarda o
	// estado real, e a próxima gravação com AutoStart ligado tenta de novo.
	var errAuto error
	if cfg.AutoStart != autoAnterior {
		if err := a.autostart(cfg.AutoStart); err != nil {
			slog.Warn("falha ao alterar a inicialização com o Windows", "erro", err)
			cfg.AutoStart = autoAnterior
			errAuto = fmt.Errorf("configurações salvas, mas não foi possível alterar a inicialização com o Windows: %w", err)
		}
	}

	if err := config.Save(cfg); err != nil {
		if errAuto == nil && cfg.AutoStart != autoAnterior {
			if errVolta := a.autostart(autoAnterior); errVolta != nil {
				slog.Warn("falha ao desfazer a inicialização com o Windows", "erro", errVolta)
			}
		}
		return err
	}

	a.guardarConfig(cfg)
	if a.queue != nil {
		a.queue.SetConfig(cfg)
	}
	if cfg.GuardarHistorico != historicoAnterior {
		a.aplicarHistorico(cfg.GuardarHistorico)
	}
	if err := a.aplicarMonitor(cfg); err != nil {
		return err
	}
	return errAuto
}

// aplicarMonitor reaplica o watcher à config salva. Com o monitor retido
// (processo novo da atualização à espera do antigo e da trava), só
// desliga: quem liga é o arranqueApos, que lê a config guardada.
func (a *App) aplicarMonitor(cfg *models.Config) error {
	a.monitorMu.Lock()
	defer a.monitorMu.Unlock()
	// Start é no-op se a pasta já está sendo monitorada; chamá-lo sempre permite
	// tentar de novo depois de uma falha mesmo sem mudar o caminho.
	if cfg.WatchdogEnabled && cfg.MonitoredPath != "" {
		if a.monitorRetido.Load() {
			return nil
		}
		if err := a.watcher.Start(cfg.MonitoredPath, watchdog.Extensoes(cfg.MonitorarPDF)); err != nil {
			slog.Warn("falha ao iniciar o monitoramento da pasta", "pasta", cfg.MonitoredPath, "erro", err)
			return fmt.Errorf("configurações salvas, mas não foi possível monitorar a pasta %q: %w", cfg.MonitoredPath, err)
		}
	} else {
		a.watcher.Stop()
	}
	return nil
}

// SetTheme grava o tema escolhido ("system", "light" ou "dark"), que main.go lê
// na próxima abertura para pintar o fundo da janela antes do WebView.
func (a *App) SetTheme(tema string) error {
	if !config.TemaValido(tema) {
		return fmt.Errorf("tema inválido: %q", tema)
	}
	a.gravarMu.Lock()
	defer a.gravarMu.Unlock()
	cfg := a.configAtual()
	if cfg == nil {
		cfg = carregarConfig(config.Load)
	}
	// Cópia: o objeto guardado é lido por outras goroutines e pela fila.
	novo := *cfg
	novo.Theme = tema
	if err := config.Save(&novo); err != nil {
		return fmt.Errorf("não foi possível salvar o tema: %w", err)
	}
	a.guardarConfig(&novo)
	if a.queue != nil {
		a.queue.SetConfig(&novo)
	}
	return nil
}

// ProcessFile adds a file to the processing queue for backward compatibility.
func (a *App) ProcessFile(filePath string) (string, error) {
	_, err := a.AddFilesToQueue([]string{filePath})
	return "", err
}

// OpenDirectory abre o Explorer selecionando o arquivo ou abrindo a pasta. Se
// o arquivo sumiu, abre a pasta dele; se a pasta também sumiu, devolve erro.
func (a *App) OpenDirectory(path string) error {
	cleanPath := filepath.Clean(path)
	abrir := a.explorer
	if abrir == nil {
		abrir = func(args ...string) error {
			return exec.Command("explorer.exe", args...).Start()
		}
	}
	info, err := os.Stat(cleanPath)
	if err == nil {
		if info.IsDir() {
			return abrir(cleanPath)
		}
		return abrir("/select,", cleanPath)
	}
	pasta := filepath.Dir(cleanPath)
	if di, derr := os.Stat(pasta); derr == nil && di.IsDir() {
		return abrir(pasta)
	}
	slog.Warn("a pasta do arquivo não existe mais", "pasta", pasta)
	return errors.New("a pasta do arquivo não existe mais")
}

// CopyOutput copia para a área de transferência o TXT gerado por um item concluído.
func (a *App) CopyOutput(path string) error {
	if a.ctx == nil {
		return fmt.Errorf("contexto não inicializado")
	}
	return a.copiarSaida(path, a.GetQueue())
}

// copiarSaida lê a saída conferida contra os itens e a manda para a área de
// transferência.
func (a *App) copiarSaida(path string, itens []*etl.QueueItem) error {
	texto, err := lerSaidaParaCopiar(path, itens)
	var ausente *erroSaidaAusente
	if errors.As(err, &ausente) {
		if a.queue != nil {
			a.queue.MarcarSaidaAusente(ausente.caminho)
		}
		return fmt.Errorf("O TXT não está mais em %s; ele pode ter sido movido ou apagado", filepath.Dir(ausente.caminho))
	}
	if err != nil {
		return err
	}
	if err := a.clipboard(a.ctx, texto); err != nil {
		slog.Warn("falha ao copiar a saída para a área de transferência", "erro", err)
		return errors.New("não foi possível copiar para a área de transferência")
	}
	return nil
}

// lerSaidaParaCopiar só lê arquivos que a própria fila gerou, para que o
// binding exposto ao WebView não sirva para ler qualquer arquivo do disco: o
// item precisa estar concluído, ter gravado esse caminho e o caminho precisa
// passar em saida.SaidaDoItem. O erro de leitura vai à interface sem caminho
// nem erro cru; o detalhe fica no log.
func lerSaidaParaCopiar(path string, itens []*etl.QueueItem) (string, error) {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".txt") {
		slog.Warn("cópia recusada: o arquivo pedido não é um TXT", "caminho", path)
		return "", errors.New("o arquivo pedido não é um TXT de saída")
	}
	limpo := filepath.Clean(path)
	for _, it := range itens {
		if it.Status == "success" && it.OutputPath != "" && strings.EqualFold(filepath.Clean(it.OutputPath), limpo) &&
			saida.SaidaDoItem(it.Kind, it.Path, it.Paths, it.OutputPath) {
			dados, err := os.ReadFile(limpo)
			if errors.Is(err, fs.ErrNotExist) {
				slog.Warn("o TXT a copiar não existe mais", "caminho", limpo)
				return "", &erroSaidaAusente{caminho: limpo}
			}
			if err != nil {
				slog.Warn("falha ao ler a saída para copiar", "caminho", limpo, "erro", err)
				return "", errors.New("não foi possível ler o TXT gerado")
			}
			return string(dados), nil
		}
	}
	slog.Warn("cópia recusada: o TXT não é saída de item concluído", "caminho", path)
	return "", errors.New("este TXT não corresponde a nenhum processamento concluído")
}

// erroSaidaAusente é a leitura do TXT que falhou porque ele não existe mais.
// O texto é fixo e sem caminho; copiarSaida o troca pela frase com a pasta e
// marca o item.
type erroSaidaAusente struct{ caminho string }

func (e *erroSaidaAusente) Error() string { return "não foi possível ler o TXT gerado" }

func (e *erroSaidaAusente) Unwrap() error { return fs.ErrNotExist }

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

// filtrosSelecao são os filtros do diálogo de seleção de arquivos.
func filtrosSelecao() []runtime.FileFilter {
	return []runtime.FileFilter{
		{DisplayName: "ZIP do Eproc e PDF (*.zip;*.pdf)", Pattern: "*.zip;*.pdf"},
		{DisplayName: "Somente ZIP (*.zip)", Pattern: "*.zip"},
		{DisplayName: "Somente PDF (*.pdf)", Pattern: "*.pdf"},
	}
}

// SelectFiles abre o diálogo de vários arquivos para ZIP do Eproc e PDF e
// devolve os caminhos escolhidos.
func (a *App) SelectFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("contexto não inicializado")
	}
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Selecionar arquivos ZIP do Eproc ou PDF",
		Filters: filtrosSelecao(),
	})
}

// AddFilesToQueue adds files manually to the processing queue and returns how
// many actually entered (a path already pending or processing is dropped).
// Durante o reinício da atualização nada entra (recusaEntrada).
func (a *App) AddFilesToQueue(filePaths []string) (int, error) {
	if err := a.recusaEntrada(); err != nil {
		return 0, err
	}
	if a.queue != nil {
		return a.queue.AddFiles(filePaths), nil
	}
	return 0, nil
}

// AddDroppedFiles põe na fila os ZIPs arrastados para a janela (origem
// "arrastada") e devolve quantos entraram; durante o reinício da
// atualização, nenhum (recusaEntrada).
func (a *App) AddDroppedFiles(filePaths []string) (int, error) {
	if err := a.recusaEntrada(); err != nil {
		return 0, err
	}
	if a.queue != nil {
		return a.queue.AddFilesComOrigem(filePaths, "arrastada"), nil
	}
	return 0, nil
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

// ReprocessQueueItem devolve à fila um item concluído, com erro ou cancelado,
// usando o OCR, a higienização e o Forçar OCR informados só para ele. O
// Forçar OCR é sempre explícito: a tela decide o valor (a caixa nasce da
// config global; "Ler com OCR" manda false).
func (a *App) ReprocessQueueItem(id, ocrMode, cleaningLevel string, forcarOCR bool) error {
	if a.queue == nil {
		return fmt.Errorf("fila não inicializada")
	}
	if err := a.recusaEntrada(); err != nil {
		return err
	}
	return a.queue.Reprocessar(id, ocrMode, cleaningLevel, &forcarOCR)
}

// GetQueueActivity devolve o estado de cada documento do item (o que está em
// paralelo, em OCR, concluído ou com erro), ou nil se ele ainda não rodou.
func (a *App) GetQueueActivity(id string) *etl.InstantaneoAtividade {
	if a.queue == nil {
		return nil
	}
	return a.queue.GetAtividade(id)
}

// ClearQueue cancels all processes and clears the queue list.
func (a *App) ClearQueue() {
	if a.queue != nil {
		a.queue.Clear()
	}
}

// ClearFinishedQueue remove da lista os itens concluídos, com erro ou
// cancelados, sem tocar no que está em andamento ou aguardando.
func (a *App) ClearFinishedQueue() {
	if a.queue != nil {
		a.queue.LimparConcluidos()
	}
}

// Greet returns a greeting for the given name (default Wails method)
func (a *App) Greet(name string) string {
	return "Olá " + name + "! Bem-vindo ao eprocIA."
}
