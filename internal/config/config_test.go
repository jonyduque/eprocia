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

	if cfg.OcrEnabled != true {
		t.Errorf("expected OcrEnabled to be true, got %t", cfg.OcrEnabled)
	}

	if cfg.CleaningLevel != "basic" {
		t.Errorf("expected CleaningLevel to be 'basic', got %s", cfg.CleaningLevel)
	}

	if cfg.AutoStart != false {
		t.Errorf("expected AutoStart to be false, got %t", cfg.AutoStart)
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
	if cfg.OcrEnabled != def.OcrEnabled {
		t.Errorf("expected default OcrEnabled, got %t", cfg.OcrEnabled)
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
		MonitoredPath: "/custom/path",
		OcrEnabled:    false,
		CleaningLevel: "aggressive",
		AutoStart:     true,
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
	if loadedCfg.OcrEnabled != customCfg.OcrEnabled {
		t.Errorf("expected OcrEnabled %t, got %t", customCfg.OcrEnabled, loadedCfg.OcrEnabled)
	}
	if loadedCfg.CleaningLevel != customCfg.CleaningLevel {
		t.Errorf("expected CleaningLevel %s, got %s", customCfg.CleaningLevel, loadedCfg.CleaningLevel)
	}
	if loadedCfg.AutoStart != customCfg.AutoStart {
		t.Errorf("expected AutoStart %t, got %t", customCfg.AutoStart, loadedCfg.AutoStart)
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
	if cfg.OcrEnabled != true {
		t.Errorf("expected OcrEnabled to fall back to true, got %t", cfg.OcrEnabled)
	}
	if cfg.CleaningLevel != "basic" {
		t.Errorf("expected CleaningLevel to fall back to 'basic', got %s", cfg.CleaningLevel)
	}
	if cfg.AutoStart != false {
		t.Errorf("expected AutoStart to fall back to false, got %t", cfg.AutoStart)
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
