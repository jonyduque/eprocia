package main

import (
	"os"
	"path/filepath"
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
