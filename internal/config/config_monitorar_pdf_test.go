package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFile_MonitorarPDFAusenteCarregaFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	antigo := `{"monitored_path":"C:\\Users\\fulano\\Downloads","watchdog_enabled":true,"ocr_mode":"fast"}`
	if err := os.WriteFile(path, []byte(antigo), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MonitorarPDF {
		t.Fatal("MonitorarPDF ausente no JSON deveria carregar false")
	}
}

func TestDefaultConfig_MonitorarPDFDesligado(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MonitorarPDF {
		t.Fatal("MonitorarPDF deveria nascer false")
	}
}

func TestSaveLoad_MonitorarPDFLigado(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MonitorarPDF = true
	if err := SaveToFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	dados, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dados), `"monitorar_pdf": true`) && !strings.Contains(string(dados), `"monitorar_pdf":true`) {
		t.Fatalf("config.json sem monitorar_pdf true: %s", dados)
	}
	lido, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !lido.MonitorarPDF {
		t.Fatal("MonitorarPDF true não sobreviveu à ida e volta")
	}
}
