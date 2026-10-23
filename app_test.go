package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"eprocia/internal/config"
	"eprocia/internal/models"
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
	if *cfg != *padrao {
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
