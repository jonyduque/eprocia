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
		MonitoredPath:   monitoredPath,
		OcrMode:         "normal",
		CleaningLevel:   "basic",
		AutoStart:       false,
		WatchdogEnabled: true,
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

// configOverlay mirrors models.Config with pointer fields so LoadFromFile can
// tell "key absent from JSON" apart from "key present with zero value" —
// json.Unmarshal into a non-pointer bool can't make that distinction, which
// would otherwise silently zero out any bool field (e.g. WatchdogEnabled)
// that defaults to true but is missing from an older config.json.
type configOverlay struct {
	MonitoredPath   *string `json:"monitored_path"`
	OcrMode         *string `json:"ocr_mode"`
	CleaningLevel   *string `json:"cleaning_level"`
	AutoStart       *bool   `json:"auto_start"`
	WatchdogEnabled *bool   `json:"watchdog_enabled"`
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

	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	var overlay configOverlay
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, err
	}

	// Only override defaults for keys actually present (and non-empty, for
	// strings) in the file.
	if overlay.MonitoredPath != nil && *overlay.MonitoredPath != "" {
		cfg.MonitoredPath = *overlay.MonitoredPath
	}
	if overlay.OcrMode != nil {
		cfg.OcrMode = *overlay.OcrMode
	}
	if overlay.CleaningLevel != nil && *overlay.CleaningLevel != "" {
		cfg.CleaningLevel = *overlay.CleaningLevel
	}
	if overlay.AutoStart != nil {
		cfg.AutoStart = *overlay.AutoStart
	}
	if overlay.WatchdogEnabled != nil {
		cfg.WatchdogEnabled = *overlay.WatchdogEnabled
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
