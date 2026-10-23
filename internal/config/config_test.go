package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"eprocia/internal/models"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	if cfg.OcrMode != "normal" {
		t.Errorf("expected OcrMode to be 'normal', got %s", cfg.OcrMode)
	}

	if cfg.CleaningLevel != "basic" {
		t.Errorf("expected CleaningLevel to be 'basic', got %s", cfg.CleaningLevel)
	}

	if cfg.AutoStart != false {
		t.Errorf("expected AutoStart to be false, got %t", cfg.AutoStart)
	}

	if cfg.WatchdogEnabled != true {
		t.Errorf("expected WatchdogEnabled to be true, got %t", cfg.WatchdogEnabled)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		expectedPath := filepath.Join(home, "Downloads")
		if cfg.MonitoredPath != expectedPath {
			t.Errorf("expected MonitoredPath to be %s, got %s", expectedPath, cfg.MonitoredPath)
		}
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nonExistentPath := filepath.Join(tmpDir, "config.json")
	cfg, err := LoadFromFile(nonExistentPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed on non-existent file: %v", err)
	}

	def, _ := DefaultConfig()
	if cfg.MonitoredPath != def.MonitoredPath {
		t.Errorf("expected default MonitoredPath, got %s", cfg.MonitoredPath)
	}
	if cfg.OcrMode != def.OcrMode {
		t.Errorf("expected default OcrMode, got %s", cfg.OcrMode)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Nesting path to test parent directory creation
	configPath := filepath.Join(tmpDir, "nested", "dirs", "config.json")

	customCfg := &models.Config{
		MonitoredPath:   "/custom/path",
		OcrMode:         "fast",
		CleaningLevel:   "aggressive",
		AutoStart:       true,
		WatchdogEnabled: false,
	}

	err = SaveToFile(configPath, customCfg)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("SaveToFile did not create the file: %v", err)
	}

	loadedCfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if loadedCfg.MonitoredPath != customCfg.MonitoredPath {
		t.Errorf("expected MonitoredPath %s, got %s", customCfg.MonitoredPath, loadedCfg.MonitoredPath)
	}
	if loadedCfg.OcrMode != customCfg.OcrMode {
		t.Errorf("expected OcrMode %s, got %s", customCfg.OcrMode, loadedCfg.OcrMode)
	}
	if loadedCfg.CleaningLevel != customCfg.CleaningLevel {
		t.Errorf("expected CleaningLevel %s, got %s", customCfg.CleaningLevel, loadedCfg.CleaningLevel)
	}
	if loadedCfg.AutoStart != customCfg.AutoStart {
		t.Errorf("expected AutoStart %t, got %t", customCfg.AutoStart, loadedCfg.AutoStart)
	}
	if loadedCfg.WatchdogEnabled != customCfg.WatchdogEnabled {
		t.Errorf("expected WatchdogEnabled %t, got %t", customCfg.WatchdogEnabled, loadedCfg.WatchdogEnabled)
	}
}

func TestLoad_PartialJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	partialJSON := `{"monitored_path": "/some/path"}`
	err = os.WriteFile(configPath, []byte(partialJSON), 0644)
	if err != nil {
		t.Fatalf("failed to write partial json: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.MonitoredPath != "/some/path" {
		t.Errorf("expected MonitoredPath to be /some/path, got %s", cfg.MonitoredPath)
	}
	if cfg.OcrMode != "normal" {
		t.Errorf("expected OcrMode to fall back to normal, got %s", cfg.OcrMode)
	}
	if cfg.CleaningLevel != "basic" {
		t.Errorf("expected CleaningLevel to fall back to 'basic', got %s", cfg.CleaningLevel)
	}
	if cfg.AutoStart != false {
		t.Errorf("expected AutoStart to fall back to false, got %t", cfg.AutoStart)
	}
	if cfg.WatchdogEnabled != true {
		t.Errorf("expected WatchdogEnabled to fall back to true, got %t", cfg.WatchdogEnabled)
	}
}

func TestLoad_EmptyFieldsFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	partialJSON := `{"monitored_path": "", "cleaning_level": ""}`
	err = os.WriteFile(configPath, []byte(partialJSON), 0644)
	if err != nil {
		t.Fatalf("failed to write json: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	def, _ := DefaultConfig()
	if cfg.MonitoredPath != def.MonitoredPath {
		t.Errorf("expected MonitoredPath to fall back to default, got %s", cfg.MonitoredPath)
	}
	if cfg.CleaningLevel != "basic" {
		t.Errorf("expected CleaningLevel to fall back to 'basic', got %s", cfg.CleaningLevel)
	}
}

func TestSaveToFile_FalhaNaEscritaPreservaArquivoAnterior(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "config.json")

	original := &models.Config{MonitoredPath: "/original", OcrMode: "fast", CleaningLevel: "basic", WatchdogEnabled: true, Theme: "system"}
	if err := SaveToFile(caminho, original); err != nil {
		t.Fatalf("SaveToFile inicial: %v", err)
	}

	antes := escreverDados
	defer func() { escreverDados = antes }()
	escreverDados = func(f *os.File, data []byte) error {
		_, _ = f.Write(data[:len(data)/2])
		return errors.New("queda simulada no meio da escrita")
	}

	novo := &models.Config{MonitoredPath: "/novo", OcrMode: "best", CleaningLevel: "aggressive"}
	if err := SaveToFile(caminho, novo); err == nil {
		t.Fatal("SaveToFile devolveu nil com falha na escrita")
	}

	cfg, err := LoadFromFile(caminho)
	if err != nil {
		t.Fatalf("config.json corrompido após falha na escrita: %v", err)
	}
	if *cfg != *original {
		t.Fatalf("config = %+v, esperado o original %+v", *cfg, *original)
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 1 {
		nomes := make([]string, 0, len(entradas))
		for _, e := range entradas {
			nomes = append(nomes, e.Name())
		}
		t.Fatalf("temporário deixado no diretório: %v", nomes)
	}
}

// Contrato: LoadFromFile é estrito com JSON inválido e devolve o erro; quem cai
// para a configuração padrão com aviso é o app (carregarConfig em app.go).
func TestLoad_CorruptJSON(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(caminho, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(caminho)
	if err == nil {
		t.Fatalf("LoadFromFile aceitou JSON inválido e devolveu %+v", cfg)
	}
	if cfg != nil {
		t.Fatalf("LoadFromFile devolveu config %+v junto com o erro", *cfg)
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	// Um diretório no lugar do arquivo dá erro de leitura diferente de NotExist,
	// que não pode ser confundido com "primeira execução".
	caminho := filepath.Join(t.TempDir(), "config.json")
	if err := os.Mkdir(caminho, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(caminho)
	if err == nil {
		t.Fatalf("LoadFromFile leu um diretório e devolveu %+v", cfg)
	}
	if os.IsNotExist(err) {
		t.Fatalf("erro tratado como inexistente: %v", err)
	}
	if cfg != nil {
		t.Fatalf("LoadFromFile devolveu config %+v junto com o erro", *cfg)
	}
}

func TestLoadESave_UsamCaminhoPadrao(t *testing.T) {
	// os.UserConfigDir lê APPDATA no Windows e XDG_CONFIG_HOME no Linux.
	base := t.TempDir()
	t.Setenv("APPDATA", base)
	t.Setenv("XDG_CONFIG_HOME", base)

	caminho, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath: %v", err)
	}
	if esperado := filepath.Join(base, "eprocIA", "config.json"); caminho != esperado {
		t.Fatalf("GetConfigPath = %q, esperado %q", caminho, esperado)
	}

	padrao, _ := DefaultConfig()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load sem arquivo: %v", err)
	}
	if *cfg != *padrao {
		t.Fatalf("Load sem arquivo = %+v, esperado padrão %+v", *cfg, *padrao)
	}

	salvo := &models.Config{MonitoredPath: "/salvo", OcrMode: "best", CleaningLevel: "none", AutoStart: true, Theme: "dark"}
	if err := Save(salvo); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load depois do Save: %v", err)
	}
	if *cfg != *salvo {
		t.Fatalf("Load = %+v, esperado %+v", *cfg, *salvo)
	}

	if err := os.WriteFile(caminho, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err := Load(); err == nil {
		t.Fatalf("Load aceitou JSON inválido e devolveu %+v", cfg)
	}
}

func TestLoad_TemaPadraoEValidacao(t *testing.T) {
	def, _ := DefaultConfig()
	if def.Theme != "system" {
		t.Fatalf("tema padrão %q, esperado system", def.Theme)
	}
	casos := map[string]string{
		`{}`:                 "system",
		`{"theme": "dark"}`:  "dark",
		`{"theme": "light"}`: "light",
		`{"theme": "roxo"}`:  "system",
		`{"theme": ""}`:      "system",
	}
	for conteudo, quer := range casos {
		caminho := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFromFile(caminho)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Theme != quer {
			t.Errorf("%s: tema %q, esperado %q", conteudo, cfg.Theme, quer)
		}
	}
}
