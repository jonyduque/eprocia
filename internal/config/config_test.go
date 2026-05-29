package config

import (
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
