package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"eprocia/internal/models"
)

// DefaultConfig returns a Config struct initialized with default values.
func DefaultConfig() (*models.Config, error) {
	home, err := os.UserHomeDir()
	var monitoredPath string
	if err == nil {
		monitoredPath = filepath.Join(home, "Downloads")
	} else {
		monitoredPath = ""
	}

	return &models.Config{
		MonitoredPath: monitoredPath,
		OcrMode:       "normal",
		CleaningLevel: "basic",
		AutoStart:     false,
	}, nil
}

// GetConfigPath returns the default path to the config.json file.
func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "eprocIA", "config.json"), nil
}

// Load loads the configuration from the default path.
// If the file does not exist, it returns the default configuration.
func Load() (*models.Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFromFile(path)
}

// LoadFromFile loads the configuration from a specific file path.
// If the file does not exist, it returns the default configuration.
func LoadFromFile(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig()
		}
		return nil, err
	}

	// Initialize with default values first so missing JSON fields keep their defaults.
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Post-unmarshal checks for fields that shouldn't be empty
	if cfg.MonitoredPath == "" {
		def, err := DefaultConfig()
		if err == nil {
			cfg.MonitoredPath = def.MonitoredPath
		}
	}
	if cfg.CleaningLevel == "" {
		cfg.CleaningLevel = "basic"
	}

	return cfg, nil
}

// Save saves the configuration to the default path.
func Save(cfg *models.Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	return SaveToFile(path, cfg)
}

// SaveToFile saves the configuration to a specific file path.
// It ensures that the parent directory exists.
func SaveToFile(path string, cfg *models.Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
