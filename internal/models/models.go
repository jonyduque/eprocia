package models

// Config represents the application configuration settings.
type Config struct {
	MonitoredPath string `json:"monitored_path"`
	OcrEnabled    bool   `json:"ocr_enabled"`
	CleaningLevel string `json:"cleaning_level"`
	AutoStart     bool   `json:"auto_start"`
}
