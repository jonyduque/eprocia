package models

import "archive/zip"

// Config represents the application configuration settings.
type Config struct {
	MonitoredPath string `json:"monitored_path"`
	OcrEnabled    bool   `json:"ocr_enabled"`
	CleaningLevel string `json:"cleaning_level"`
	AutoStart     bool   `json:"auto_start"`
}

// ProcessFile represents a single judicial process file within a ZIP archive.
type ProcessFile struct {
	Name         string    `json:"name"`
	EventNum     int       `json:"event_num"`
	DocName      string    `json:"doc_name"`
	Extension    string    `json:"extension"`
	ZipFileRef   *zip.File `json:"-"`
}

