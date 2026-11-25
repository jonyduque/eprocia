package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"eprocia/internal/config"
	"eprocia/internal/etl"
	"eprocia/internal/models"
	"eprocia/internal/watchdog"
)

func TestSaveConfig_DevolveErroQuandoPastaNaoPodeSerMonitorada(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()

	inexistente := filepath.Join(t.TempDir(), "nao-existe")
	cfg := &models.Config{MonitoredPath: inexistente, WatchdogEnabled: true, OcrMode: "normal", CleaningLevel: "basic"}
	if err := app.SaveConfig(cfg); err == nil {
		t.Fatal("SaveConfig devolveu nil com pasta monitorada inexistente")
	}

	// Salvar de novo a mesma config, depois de criar a pasta, deve ligar o watchdog.
	if err := os.MkdirAll(inexistente, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig com pasta existente: %v", err)
	}
	if got := app.watcher.GetCurrentPath(); got != inexistente {
		t.Fatalf("watchdog monitorando %q, esperado %q", got, inexistente)
	}
}

func TestCarregarConfig_JSONInvalidoCaiParaPadrao(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(caminho, []byte("{corrompido"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := carregarConfig(func() (*models.Config, error) { return config.LoadFromFile(caminho) })
	if cfg == nil {
		t.Fatal("carregarConfig devolveu nil com JSON inválido")
	}
	padrao, _ := config.DefaultConfig()
	if !reflect.DeepEqual(*cfg, *padrao) {
		t.Fatalf("config = %+v, esperado padrão %+v", *cfg, *padrao)
	}
}

func TestSaveConfig_AplicaAutostartSoQuandoMuda(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()
	var chamadas []bool
	app.autostart = func(ativo bool) error { chamadas = append(chamadas, ativo); return nil }

	cfg := &models.Config{OcrMode: "normal", CleaningLevel: "basic"}
	for _, auto := range []bool{true, true, false, false} {
		c := *cfg
		c.AutoStart = auto
		if err := app.SaveConfig(&c); err != nil {
			t.Fatalf("SaveConfig(AutoStart=%v): %v", auto, err)
		}
	}
	if want := []bool{true, false}; !slices.Equal(chamadas, want) {
		t.Fatalf("autostart chamado com %v, esperado %v", chamadas, want)
	}
}

func TestSaveConfig_DevolveErroQuandoAutostartFalha(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()
	app.autostart = func(bool) error { return errors.New("acesso negado") }

	if err := app.SaveConfig(&models.Config{AutoStart: true, OcrMode: "normal", CleaningLevel: "basic"}); err == nil {
		t.Fatal("SaveConfig devolveu nil com falha ao gravar a inicialização")
	}
}

func TestSincronizarAutostart_AplicaEstadoDaConfig(t *testing.T) {
	app := NewApp()
	var chamadas []bool
	app.autostart = func(ativo bool) error { chamadas = append(chamadas, ativo); return nil }

	app.sincronizarAutostart(&models.Config{AutoStart: true})
	app.sincronizarAutostart(&models.Config{AutoStart: false})
	if want := []bool{true, false}; !slices.Equal(chamadas, want) {
		t.Fatalf("autostart chamado com %v, esperado %v", chamadas, want)
	}
}

// Se o registro recusar a mudança, a config gravada precisa refletir o estado
// real; senão a próxima gravação acha que nada mudou e nunca tenta de novo.
func TestSaveConfig_AutostartFalhoNaoFicaGravado(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()
	falhar := true
	var chamadas []bool
	app.autostart = func(ativo bool) error {
		chamadas = append(chamadas, ativo)
		if falhar {
			return errors.New("acesso negado")
		}
		return nil
	}

	cfg := models.Config{AutoStart: true, OcrMode: "fast", CleaningLevel: "basic"}
	c := cfg
	if err := app.SaveConfig(&c); err == nil {
		t.Fatal("SaveConfig devolveu nil com falha ao gravar a inicialização")
	}
	gravada, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if gravada.AutoStart {
		t.Fatal("AutoStart=true gravado no disco apesar da falha no registro")
	}
	if gravada.OcrMode != "fast" {
		t.Fatalf("demais campos não foram gravados: ocr_mode=%q", gravada.OcrMode)
	}
	if app.cfg.AutoStart {
		t.Fatal("AutoStart=true mantido em memória apesar da falha no registro")
	}

	falhar = false
	// Objeto novo, como cada chamada vinda do frontend.
	c2 := cfg
	if err := app.SaveConfig(&c2); err != nil {
		t.Fatalf("segunda tentativa: %v", err)
	}
	if want := []bool{true, true}; !slices.Equal(chamadas, want) {
		t.Fatalf("autostart chamado com %v, esperado %v (nova tentativa)", chamadas, want)
	}
	if gravada, _ := config.Load(); !gravada.AutoStart {
		t.Fatal("AutoStart não gravado depois do registro aceitar")
	}
}

func TestSaveConfig_PreservaTemaGravadoPorSetTheme(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()
	app.autostart = func(bool) error { return nil }

	if err := app.SaveConfig(&models.Config{OcrMode: "normal", CleaningLevel: "basic"}); err != nil {
		t.Fatal(err)
	}
	if err := app.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	// O formulário manda o objeto que leu antes da troca de tema.
	if err := app.SaveConfig(&models.Config{OcrMode: "fast", CleaningLevel: "basic", Theme: "light"}); err != nil {
		t.Fatal(err)
	}
	gravada, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if gravada.Theme != "dark" {
		t.Fatalf("tema gravado %q, esperado dark", gravada.Theme)
	}
}

func TestSetTheme_ValidaEGrava(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	defer app.watcher.Stop()

	if err := app.SetTheme("roxo"); err == nil {
		t.Fatal("SetTheme aceitou tema inválido")
	}
	for _, tema := range []string{"light", "dark", "system"} {
		if err := app.SetTheme(tema); err != nil {
			t.Fatalf("SetTheme(%q): %v", tema, err)
		}
		gravada, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		if gravada.Theme != tema {
			t.Fatalf("tema gravado %q, esperado %q", gravada.Theme, tema)
		}
	}
}

func TestAoTerminar_RegistraFalhaEEsqueceSucesso(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "proc.zip")
	if err := os.WriteFile(zip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(zip)
	app := NewApp()
	app.falhas = watchdog.CarregarFalhas(filepath.Join(dir, "falhas.json"))

	app.aoTerminar(&etl.QueueItem{Path: zip, Status: "error"})
	if !app.falhas.Ignorar(zip, info.ModTime()) {
		t.Fatal("ZIP com erro não foi registrado")
	}
	app.aoTerminar(&etl.QueueItem{Path: zip, Status: "success"})
	if app.falhas.Ignorar(zip, info.ModTime()) {
		t.Fatal("ZIP processado com sucesso continua registrado como falha")
	}
}

func TestAoTerminar_NaoLidosNaoVaoParaFalhas(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "proc.zip")
	if err := os.WriteFile(zip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(zip)
	app := NewApp()
	app.falhas = watchdog.CarregarFalhas(filepath.Join(dir, "falhas.json"))
	app.falhas.Registrar(zip)

	app.aoTerminar(&etl.QueueItem{Path: zip, Status: "success", UnreadDocs: 3})
	if app.falhas.Ignorar(zip, info.ModTime()) {
		t.Fatal("item de sucesso com não lidos continua registrado como falha")
	}
}

// esperarItemFim espera o único item da fila sair de pending/processing.
func esperarItemFim(t *testing.T, q *etl.Queue) *etl.QueueItem {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		itens := q.GetItems()
		if len(itens) == 1 && itens[0].Status != "pending" && itens[0].Status != "processing" {
			return itens[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("o item não terminou")
	return nil
}

// O botão "Ler com OCR" manda forcarOCR=false: com a global true, o
// reprocessamento não força (o valor é sempre explícito, nunca "usa a global").
func TestReprocessQueueItem_PassaForcarExplicito(t *testing.T) {
	q := etl.NewQueue(etl.NewPipeline())
	t.Cleanup(q.Shutdown)
	q.SetConfig(&models.Config{OcrMode: "normal", CleaningLevel: "standard", OmitirTipos: []string{}, ForcarOCR: true})
	app := &App{queue: q}

	// O ZIP não existe: a execução termina em erro, mas registra o Forçar OCR efetivo.
	zip := filepath.Join(t.TempDir(), "5000000-00.2026.8.26.0001.zip")
	q.AddFiles([]string{zip})
	if it := esperarItemFim(t, q); !it.ForcarOCR {
		t.Fatalf("primeira execução com a global true não forçou: %+v", it)
	}
	id := q.GetItems()[0].ID

	if err := app.ReprocessQueueItem(id, "normal", "standard", false); err != nil {
		t.Fatalf("ReprocessQueueItem: %v", err)
	}
	if it := esperarItemFim(t, q); it.ForcarOCR {
		t.Errorf("ReprocessQueueItem(..., false) forçou o OCR com a global true: %+v", it)
	}
}
