package etl

import (
	"archive/zip"
	"fmt"
	"log"
	"strings"

	"eprocia/internal/models"
)

// ReadZipFiles lists and parses all relevant files from a ZIP archive.
func ReadZipFiles(zipPath string) ([]models.ProcessFile, *zip.ReadCloser, error) {
	log.Printf("[ZipReader] Lendo arquivo ZIP: %s", zipPath)
	rc, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Printf("[ZipReader] Erro ao abrir arquivo ZIP: %v", err)
		return nil, nil, fmt.Errorf("failed to open zip file: %w", err)
	}

	var files []models.ProcessFile
	for _, f := range rc.File {
		// Ignore directories and system/metadata files
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.Contains(f.Name, "__MACOSX") || strings.HasPrefix(f.Name, ".") || strings.Contains(f.Name, "/.") {
			continue
		}

		// Only process PDF and HTML (or HTM) files
		ext := strings.ToLower(f.Name)
		if !strings.HasSuffix(ext, ".pdf") && !strings.HasSuffix(ext, ".html") && !strings.HasSuffix(ext, ".htm") {
			continue
		}

		files = append(files, ParseFileInfo(f))
	}

	log.Printf("[ZipReader] ZIP %s contém %d arquivos válidos para processamento", zipPath, len(files))
	return files, rc, nil
}
