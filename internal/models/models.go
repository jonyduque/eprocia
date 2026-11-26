package models

import (
	"archive/zip"
	"io"
)

// Config represents the application configuration settings.
type Config struct {
	MonitoredPath   string `json:"monitored_path"`
	OcrMode         string `json:"ocr_mode"` // "none", "fast", "normal", "best"
	CleaningLevel   string `json:"cleaning_level"`
	AutoStart       bool   `json:"auto_start"`
	WatchdogEnabled bool   `json:"watchdog_enabled"`
	// Theme fica no config.json para a janela nativa abrir com o fundo do
	// tema escolhido: "system", "light" ou "dark".
	Theme string `json:"theme"`
	// OmitirTipos lista códigos de tipo de documento (EXTR, COMP...) que não são
	// lidos: viram um aviso no texto. Nunca nil depois de Load.
	OmitirTipos []string `json:"omitir_tipos"`
	// ForcarOCR leva ao OCR toda página de PDF de documento lido, inclusive as
	// de texto nativo legível. Sem efeito com OcrMode "none" (o valor salvo é
	// preservado).
	ForcarOCR bool `json:"forcar_ocr"`
	// MonitorarPDF faz a pasta monitorada converter também os PDFs novos,
	// cada um no seu item e no seu TXT. Desligado por padrão.
	MonitorarPDF bool `json:"monitorar_pdf"`
	// GuardarHistorico guarda a lista da fila da última sessão em
	// historico.json, para ela voltar ao reabrir. Desligado por padrão.
	GuardarHistorico bool `json:"guardar_historico"`
}

// ProcessFile represents a single judicial process file, inside a ZIP archive
// or loose on disk (Avulso).
type ProcessFile struct {
	Name       string    `json:"name"`
	EventNum   int       `json:"event_num"`
	DocNum     int       `json:"doc_num"` // document ID inside the event (2nd sort level)
	DocName    string    `json:"doc_name"`
	Extension  string    `json:"extension"`
	ZipFileRef *zip.File `json:"-"`
	// Abrir abre o conteúdo do documento (entrada do ZIP ou PDF solto).
	Abrir func() (io.ReadCloser, error) `json:"-"`
	// Tamanho é o tamanho descomprimido declarado; 0 num PDF avulso que não
	// passou nas conferências de LerEntrada.
	Tamanho int64 `json:"-"`
	// Avulso marca um PDF solto, fora de ZIP.
	Avulso bool `json:"-"`
}
