package etl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"eprocia/internal/models"
	"eprocia/internal/ocr"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Pipeline orchestrates the entire extraction process.
type Pipeline struct {
	ctx context.Context
}

// NewPipeline creates a new Pipeline instance.
func NewPipeline(ctx context.Context) *Pipeline {
	return &Pipeline{
		ctx: ctx,
	}
}

// SetContext updates the Wails context for events.
func (p *Pipeline) SetContext(ctx context.Context) {
	p.ctx = ctx
}

// ProcessZip extracts, parses, OCRs (if needed), sorts, and formats a ZIP file.
func (p *Pipeline) ProcessZip(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(progress float64, message string)) (string, int, int, error) {
	if p.ctx != nil {
		runtime.EventsEmit(p.ctx, "process:start", zipPath)
	}

	// 1. Read ZIP files
	files, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		if p.ctx != nil {
			runtime.EventsEmit(p.ctx, "process:error", map[string]string{
				"file":  zipPath,
				"error": err.Error(),
			})
		}
		return "", 0, 0, err
	}
	defer rc.Close()

	if len(files) == 0 {
		errNoFiles := fmt.Errorf("no PDF or HTML documents found in ZIP")
		if p.ctx != nil {
			runtime.EventsEmit(p.ctx, "process:error", map[string]string{
				"file":  zipPath,
				"error": errNoFiles.Error(),
			})
		}
		return "", 0, 0, errNoFiles
	}

	// 2. Sort documents chronologically and naturally
	SortProcessFiles(files)

	fileContents := make(map[string][]PageData)
	totalFiles := len(files)

	// OCR Runner wrapper using the internal ocr module
	ocrRunner := func(ctx context.Context, pdfPath string, pageNum int) (string, error) {
		return ocr.RunOCR(ctx, pdfPath, pageNum, cfg.OcrMode)
	}

	// 3. Process each document
	for i, f := range files {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return "", 0, 0, err
		}

		progressMsg := fmt.Sprintf("Processando %d/%d: %s", i+1, totalFiles, f.DocName)
		log.Printf("[Pipeline] %s", progressMsg)
		if onProgress != nil {
			onProgress(float64(i)/float64(totalFiles), progressMsg)
		}
		if p.ctx != nil {
			runtime.EventsEmit(p.ctx, "process:progress", map[string]interface{}{
				"file":     zipPath,
				"progress": float64(i) / float64(totalFiles),
				"message":  progressMsg,
			})
		}

		zipReader, err := f.ZipFileRef.Open()
		if err != nil {
			fileContents[f.Name] = []PageData{{PageNum: 1, Text: fmt.Sprintf("[ERRO ao abrir arquivo no ZIP: %s]", err.Error())}}
			continue
		}

		var pages []PageData

		if f.Extension == ".pdf" {
			// Extract the PDF to a temporary file since the PDF library needs io.ReaderAt and the size
			tempPdfFile, err := os.CreateTemp("", "eprocia_pdf_*.pdf")
			if err != nil {
				zipReader.Close()
				fileContents[f.Name] = []PageData{{PageNum: 1, Text: fmt.Sprintf("[ERRO ao criar arquivo temporário: %s]", err.Error())}}
				continue
			}
			tempPath := tempPdfFile.Name()

			_, errCopy := io.Copy(tempPdfFile, zipReader)
			tempPdfFile.Close()
			zipReader.Close()

			if errCopy != nil {
				os.Remove(tempPath)
				fileContents[f.Name] = []PageData{{PageNum: 1, Text: fmt.Sprintf("[ERRO ao extrair PDF temporário: %s]", errCopy.Error())}}
				continue
			}

			parser := NewPDFParser(tempPath)
			pdfPages, errParse := parser.ParsePages(ctx, cfg.OcrMode != "none", ocrRunner)
			os.Remove(tempPath)

			if errParse != nil {
				fileContents[f.Name] = []PageData{{PageNum: 1, Text: fmt.Sprintf("[ERRO ao analisar PDF: %s]", errParse.Error())}}
				continue
			}
			pages = pdfPages
		} else if f.Extension == ".html" || f.Extension == ".htm" {
			htmlText, errParse := ExtractTextFromHTML(zipReader)
			zipReader.Close()
			if errParse != nil {
				fileContents[f.Name] = []PageData{{PageNum: 1, Text: fmt.Sprintf("[ERRO ao analisar HTML: %s]", errParse.Error())}}
				continue
			}
			pages = []PageData{{PageNum: 1, Text: htmlText}}
		}

		fileContents[f.Name] = pages
		log.Printf("[Pipeline] Documento %s finalizado com %d páginas", f.DocName, len(pages))
	}

	// 4. Format consolidated output into XML
	outputText := GenerateConsolidatedOutput(files, fileContents, cfg.CleaningLevel)

	// 5. Save consolidated output to the same directory
	outputPath := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".txt"
	err = os.WriteFile(outputPath, []byte(outputText), 0644)
	if err != nil {
		if p.ctx != nil {
			runtime.EventsEmit(p.ctx, "process:error", map[string]string{
				"file":  zipPath,
				"error": fmt.Sprintf("failed to save output file: %s", err.Error()),
			})
		}
		return "", 0, 0, err
	}

	// Count total pages
	totalPages := 0
	for _, pages := range fileContents {
		totalPages += len(pages)
	}

	if p.ctx != nil {
		runtime.EventsEmit(p.ctx, "process:success", map[string]interface{}{
			"file":        zipPath,
			"output_path": outputPath,
			"docs_count":  totalFiles,
			"pages_count": totalPages,
		})
	}

	return outputPath, totalFiles, totalPages, nil
}

