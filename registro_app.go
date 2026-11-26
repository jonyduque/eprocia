package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"os/exec"
	stdRuntime "runtime"
	"time"

	"eprocia/internal/etl"
	"eprocia/internal/models"
	"eprocia/internal/registro"
	"eprocia/internal/telemetria"
)

//go:embed wails.json
var wailsJSON []byte

// iniciarRegistro liga o log em arquivo conforme o valor de EPROCIA_LOG, lido
// uma vez na partida. Desligado (ou sem pasta de dados), não toca em
// slog.Default() nem no pacote log. Ligado, aponta o slog e o log para o
// arquivo e grava a linha de início. Uma falha ao abrir o log vai ao stderr e
// o app segue como log desligado.
func iniciarRegistro(valor, pastaDados string, cfg *models.Config) bool {
	if pastaDados == "" {
		return false
	}
	ativo, err := registro.Iniciar(registro.Opcoes{
		Valor:      valor,
		PastaDados: pastaDados,
		Saneador:   registro.SaneadorDoSistema(),
	})
	if err != nil {
		slog.Warn("não foi possível abrir o log", "erro", err)
		return false
	}
	if !ativo {
		return false
	}
	h := registro.Manipulador()
	if h == nil {
		return false
	}
	slog.SetDefault(slog.New(h))

	modo := "1"
	if registro.Detalhado() {
		modo = "detalhado"
	}
	workers := max(stdRuntime.NumCPU()-1, 1)
	slog.Info("eprocIA iniciado",
		"versao", versaoApp(),
		"eprocia_log", modo,
		"windows", versaoWindows(),
		"cpus", stdRuntime.NumCPU(),
		"semaforo_docs", workers,
		"semaforo_ocr", workers,
		"pasta_logs", registro.Pasta(),
		configComoGrupo(cfg),
	)
	if registro.ValorNaoReconhecido() {
		slog.Warn("valor de EPROCIA_LOG não reconhecido", "tratado_como", "1")
	}
	return true
}

// configComoGrupo põe a config efetiva na linha de início; o manipulador
// saneia o caminho monitorado.
func configComoGrupo(cfg *models.Config) slog.Attr {
	if cfg == nil {
		return slog.Group("config")
	}
	return slog.Group("config",
		slog.String("monitored_path", cfg.MonitoredPath),
		slog.String("ocr_mode", cfg.OcrMode),
		slog.String("cleaning_level", cfg.CleaningLevel),
		slog.Bool("auto_start", cfg.AutoStart),
		slog.Bool("watchdog_enabled", cfg.WatchdogEnabled),
		slog.String("theme", cfg.Theme),
		slog.Any("omitir_tipos", cfg.OmitirTipos),
	)
}

// prepararInstancia aplica a trava de uma instância por executável. Com
// outra instância aberta devolve sair=true sem chamar aoTravar. Com a trava
// obtida, ou indisponível (o app abre assim mesmo), chama aoTravar, que
// inicia o registro: a segunda cópia nunca mexe no log da primeira. liberar
// nunca é nil.
func prepararInstancia(travar func() (liberar func(), ok bool, err error), aoTravar func()) (sair bool, liberar func()) {
	liberar, ok, err := travar()
	switch {
	case err != nil:
		aoTravar()
		slog.Warn("trava de instância indisponível", "erro", err)
		return false, func() {}
	case !ok:
		return true, func() {}
	}
	aoTravar()
	if liberar == nil {
		liberar = func() {}
	}
	return false, liberar
}

// versaoApp é a productVersion do wails.json embutido.
func versaoApp() string {
	var w struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsJSON, &w); err != nil {
		return ""
	}
	return w.Info.ProductVersion
}

// LogAtivo diz se o log em arquivo está ligado: EPROCIA_LOG definida na
// partida e o arquivo aberto. Sem ele a interface não mostra nada de log.
func (a *App) LogAtivo() bool {
	return registro.Ativo()
}

// AbrirPastaLogs abre a pasta dos logs no Explorer; com o log inativo,
// devolve erro.
func (a *App) AbrirPastaLogs() error {
	pasta := registro.Pasta()
	if !registro.Ativo() || pasta == "" {
		return errors.New("o log não está ativo")
	}
	return exec.Command("explorer.exe", pasta).Start()
}

// ligarTelemetria abre <pastaDados>\telemetria\telemetria.jsonl, coleta a
// máquina uma vez e grava um registro sem nomes por ZIP terminado. Só é
// chamada com o registro ativo (App.startup); fechar descarrega e fecha o
// arquivo e nunca é nil. Uma falha ao abrir vira um aviso no log e a
// telemetria fica desligada.
func ligarTelemetria(q *etl.Queue, pastaDados string) (fechar func()) {
	g, err := telemetria.NovoGravador(pastaDados)
	if err != nil {
		slog.Warn("não foi possível abrir a telemetria", "erro", err)
		return func() {}
	}
	maquina := telemetria.ColetarMaquina()
	versao := versaoApp()
	q.SetAoTerminarZip(func(e models.ExecucaoZip) {
		g.Gravar(telemetria.Montar(e, maquina, versao, time.Now()))
	})
	return func() {
		q.SetAoTerminarZip(nil)
		g.Fechar()
	}
}

// ligarTelemetriaSeAtivo liga a telemetria da fila do App só com o registro
// ativo (EPROCIA_LOG lida na partida); desligado, nada dela executa e
// fecharTelemetria fica nil. Chamada pelo startup, depois de criar a fila.
func (a *App) ligarTelemetriaSeAtivo(pastaDados string) {
	if !registro.Ativo() || a.queue == nil {
		return
	}
	a.fecharTelemetria = ligarTelemetria(a.queue, pastaDados)
}
