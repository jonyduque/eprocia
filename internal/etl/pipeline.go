package etl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	stdRuntime "runtime"
	"strings"
	"sync"
	"sync/atomic"

	"eprocia/internal/models"
	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// contarPaginasPDF é uma variável para que os testes simulem um panic da biblioteca de PDF.
var contarPaginasPDF = func(r io.ReaderAt, size int64) (int, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return 0, err
	}
	return reader.NumPage(), nil
}

// Pipeline orchestrates the entire extraction process.
type Pipeline struct {
	mu  sync.RWMutex
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
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctx = ctx
}

func (p *Pipeline) getContext() context.Context {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ctx
}

// ProcessZip extracts, parses, OCRs (if needed), sorts, and formats a ZIP file.
func (p *Pipeline) ProcessZip(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int)) (string, int, int, error) {
	if wailsCtx := p.getContext(); wailsCtx != nil {
		runtime.EventsEmit(wailsCtx, "process:start", zipPath)
	}

	// 1. Read ZIP files
	files, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		if wailsCtx := p.getContext(); wailsCtx != nil {
			runtime.EventsEmit(wailsCtx, "process:error", map[string]string{
				"file":  zipPath,
				"error": err.Error(),
			})
		}
		return "", 0, 0, err
	}
	defer rc.Close()

	if len(files) == 0 {
		errNoFiles := fmt.Errorf("no PDF or HTML documents found in ZIP")
		if wailsCtx := p.getContext(); wailsCtx != nil {
			runtime.EventsEmit(wailsCtx, "process:error", map[string]string{
				"file":  zipPath,
				"error": errNoFiles.Error(),
			})
		}
		return "", 0, 0, errNoFiles
	}

	// 2. Sort documents chronologically and naturally
	SortProcessFiles(files)

	// Pre-calculate total pages across all files to show accurate progress.
	// PDFs are extracted to a temp file here (once) and reused by the
	// per-document goroutines below, instead of re-reading them from the zip
	// a second time.
	totalFiles := len(files)
	totalPagesAll := 0
	docPageCounts := make(map[string]int) // keep track of pages per file
	docTempFiles := make(map[string]string)
	var tempFilesMu sync.Mutex
	// Remove os PDFs pré-extraídos em qualquer saída, inclusive por panic.
	defer func() {
		tempFilesMu.Lock()
		for _, tp := range docTempFiles {
			os.Remove(tp)
		}
		tempFilesMu.Unlock()
	}()

	for _, f := range files {
		if err := ctx.Err(); err != nil {
			log.Printf("[Pipeline] Processamento cancelado durante o pré-cálculo de páginas")
			return "", 0, 0, err
		}
		if f.Extension == ".pdf" {
			pagesCount, tempPath, err := func() (n int, tPath string, err error) {
				// A biblioteca de PDF pode entrar em panic com arquivo malformado;
				// nesse caso o temporário é removido e o documento cai no fallback.
				var pf *os.File
				defer func() {
					if r := recover(); r != nil {
						if pf != nil {
							pf.Close()
						}
						if tPath != "" {
							os.Remove(tPath)
						}
						n, tPath, err = 0, "", fmt.Errorf("panic ao ler PDF: %v", r)
					}
				}()

				zr, err := f.ZipFileRef.Open()
				if err != nil {
					return 0, "", err
				}
				defer zr.Close()

				tempPdfFile, err := os.CreateTemp("", "eprocia_pdf_*.pdf")
				if err != nil {
					return 0, "", err
				}
				tPath = tempPdfFile.Name()

				if _, err := io.Copy(tempPdfFile, zr); err != nil {
					tempPdfFile.Close()
					os.Remove(tPath)
					return 0, "", err
				}
				tempPdfFile.Close()

				pf, err = os.Open(tPath)
				if err != nil {
					os.Remove(tPath)
					return 0, "", err
				}

				stat, err := pf.Stat()
				if err != nil {
					pf.Close()
					os.Remove(tPath)
					return 0, "", err
				}

				// No Windows, os.Remove falha com o arquivo ainda aberto: fechar antes de remover.
				n, err = contarPaginasPDF(pf, stat.Size())
				if err != nil {
					pf.Close()
					os.Remove(tPath)
					return 0, "", err
				}
				pf.Close()
				return n, tPath, nil
			}()
			if err != nil {
				log.Printf("[Pipeline] Erro ao pré-calcular páginas para %s: %v", f.DocName, err)
				docPageCounts[f.Name] = 1 // fallback
				totalPagesAll += 1
			} else {
				docPageCounts[f.Name] = pagesCount
				totalPagesAll += pagesCount
				tempFilesMu.Lock()
				docTempFiles[f.Name] = tempPath
				tempFilesMu.Unlock()
			}
		} else {
			docPageCounts[f.Name] = 1
			totalPagesAll += 1
		}
	}
	log.Printf("[Pipeline] Pré-cálculo concluído: %d documentos, %d páginas no total", totalFiles, totalPagesAll)

	eventos := make(map[string]any)

	// Group files by EventNum to pre-populate map
	eventsMap := make(map[int][]models.ProcessFile)
	var eventNums []int
	for _, f := range files {
		if _, exists := eventsMap[f.EventNum]; !exists {
			eventNums = append(eventNums, f.EventNum)
		}
		eventsMap[f.EventNum] = append(eventsMap[f.EventNum], f)
	}

	// Initialize the events map structure
	for _, eventNum := range eventNums {
		eventKey := fmt.Sprintf("%d", eventNum)
		documentosMap := make(map[string]any)
		eventos[eventKey] = map[string]any{
			"index":      eventNum,
			"documentos": documentosMap,
		}

		eventFiles := eventsMap[eventNum]
		for index, f := range eventFiles {
			docKey := f.Name
			paginasMap := make(map[string]any)
			documentosMap[docKey] = map[string]any{
				"index":   index + 1,
				"nome":    f.DocName,
				"paginas": paginasMap,
			}
		}
	}

	// Define active worker limits based on CPU count
	numWorkers := stdRuntime.NumCPU() - 1
	if numWorkers < 1 {
		numWorkers = 1
	}

	// OCR Runner wrapper using the internal ocr module with global semaphore to limit concurrent execs
	sem := make(chan struct{}, numWorkers)
	ocrRunner := func(ctx context.Context, pdfPath string, pageNum int) (string, string, error) {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
		defer func() { <-sem }()
		return ocr.RunOCR(ctx, pdfPath, pageNum, cfg.OcrMode)
	}

	var processedDocsVal int32
	var processedPagesVal int32

	// Progress reporting helper
	reportProgress := func(msg string) {
		pDocs := int(atomic.LoadInt32(&processedDocsVal))
		pPages := int(atomic.LoadInt32(&processedPagesVal))

		progress := 0.0
		if totalPagesAll > 0 {
			progress = float64(pPages) / float64(totalPagesAll)
		}

		if onProgress != nil {
			onProgress(progress, msg, pDocs, totalFiles, pPages, totalPagesAll)
		}

		if wailsCtx := p.getContext(); wailsCtx != nil {
			runtime.EventsEmit(wailsCtx, "process:progress", map[string]interface{}{
				"file":            zipPath,
				"progress":        progress,
				"message":         msg,
				"processed_docs":  pDocs,
				"total_docs":      totalFiles,
				"processed_pages": pPages,
				"total_pages":     totalPagesAll,
			})
		}
	}

	// 3. Process documents concurrently, bounded by docSem to prevent file descriptor/memory exhaustion
	var wg sync.WaitGroup
	var errParseFirst error
	var errMu sync.Mutex

	docSem := make(chan struct{}, numWorkers)

	for i, f := range files {
		wg.Add(1)
		go func(i int, f models.ProcessFile) {
			defer wg.Done()

			// Acquire slot for document processing
			select {
			case docSem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-docSem }()

			// Check for context cancellation before starting
			if err := ctx.Err(); err != nil {
				return
			}

			// Retrieve docData and its paginas map
			eventKey := fmt.Sprintf("%d", f.EventNum)
			eventData := eventos[eventKey].(map[string]any)
			documentosMap := eventData["documentos"].(map[string]any)
			docData := documentosMap[f.Name].(map[string]any)
			paginasMap := docData["paginas"].(map[string]any)

			if f.Extension == ".pdf" {
				tempFilesMu.Lock()
				tempPath, hasPreExtracted := docTempFiles[f.Name]
				tempFilesMu.Unlock()

				if !hasPreExtracted {
					// Fallback: the pre-scan pass couldn't extract this file (rare); extract it now.
					zr, err := f.ZipFileRef.Open()
					if err != nil {
						paginasMap["1"] = map[string]any{
							"index":   1,
							"ocr":     false,
							"image":   "",
							"content": fmt.Sprintf("[ERRO ao abrir arquivo no ZIP: %s]", err.Error()),
						}
						atomic.AddInt32(&processedPagesVal, 1)
						reportProgress(fmt.Sprintf("Erro ao abrir %s", f.DocName))
						return
					}

					tempPdfFile, err := os.CreateTemp("", "eprocia_pdf_*.pdf")
					if err != nil {
						zr.Close()
						paginasMap["1"] = map[string]any{
							"index":   1,
							"ocr":     false,
							"image":   "",
							"content": fmt.Sprintf("[ERRO ao criar arquivo temporário: %s]", err.Error()),
						}
						atomic.AddInt32(&processedPagesVal, 1)
						reportProgress(fmt.Sprintf("Erro de PDF temporário em %s", f.DocName))
						return
					}
					tempPath = tempPdfFile.Name()

					_, errCopy := io.Copy(tempPdfFile, zr)
					tempPdfFile.Close()
					zr.Close()

					if errCopy != nil {
						os.Remove(tempPath)
						paginasMap["1"] = map[string]any{
							"index":   1,
							"ocr":     false,
							"image":   "",
							"content": fmt.Sprintf("[ERRO ao extrair PDF temporário: %s]", errCopy.Error()),
						}
						atomic.AddInt32(&processedPagesVal, 1)
						reportProgress(fmt.Sprintf("Erro de extração em %s", f.DocName))
						return
					}
				}

				parser := NewPDFParser(tempPath)
				pdfPages, errParse := parser.ParsePages(ctx, cfg.OcrMode != "none", ocrRunner, func(pageNum int) {
					atomic.AddInt32(&processedPagesVal, 1)
					pDocs := int(atomic.LoadInt32(&processedDocsVal))
					reportProgress(fmt.Sprintf("Processando %s (pág. %d/%d) [Docs: %d/%d]", f.DocName, pageNum, docPageCounts[f.Name], pDocs, totalFiles))
				})
				os.Remove(tempPath)

				if errParse != nil {
					if ctx.Err() != nil {
						log.Printf("[Pipeline] Processamento cancelado durante a análise do PDF: %s", f.DocName)
						errMu.Lock()
						if errParseFirst == nil {
							errParseFirst = ctx.Err()
						}
						errMu.Unlock()
						return
					}
					paginasMap["1"] = map[string]any{
						"index":   1,
						"ocr":     false,
						"image":   "",
						"content": fmt.Sprintf("[ERRO ao analisar PDF: %s]", errParse.Error()),
					}
					return
				}

				// Copy the pages map returned by ParsePages into the document's paginas map
				for k, v := range pdfPages {
					paginasMap[k] = v
				}
			} else if f.Extension == ".html" || f.Extension == ".htm" {
				zipReader, err := f.ZipFileRef.Open()
				if err != nil {
					paginasMap["1"] = map[string]any{
						"index":   1,
						"ocr":     false,
						"image":   "",
						"content": fmt.Sprintf("[ERRO ao abrir arquivo no ZIP: %s]", err.Error()),
					}
					atomic.AddInt32(&processedPagesVal, 1)
					reportProgress(fmt.Sprintf("Erro ao abrir %s", f.DocName))
					return
				}

				htmlText, errParse := ExtractTextFromHTML(zipReader)
				zipReader.Close()
				if errParse != nil {
					paginasMap["1"] = map[string]any{
						"index":   1,
						"ocr":     false,
						"image":   "",
						"content": fmt.Sprintf("[ERRO ao analisar HTML: %s]", errParse.Error()),
					}
					atomic.AddInt32(&processedPagesVal, 1)
					reportProgress(fmt.Sprintf("Erro ao analisar HTML %s", f.DocName))
					return
				}
				paginasMap["1"] = map[string]any{
					"index":   1,
					"ocr":     false,
					"image":   "",
					"content": htmlText,
				}
				atomic.AddInt32(&processedPagesVal, 1)
				pDocs := int(atomic.LoadInt32(&processedDocsVal))
				reportProgress(fmt.Sprintf("Processando %s (HTML concluído) [Docs: %d/%d]", f.DocName, pDocs, totalFiles))
			}

			log.Printf("[Pipeline] Documento %s finalizado com %d páginas", f.DocName, len(paginasMap))
			pDocs := int(atomic.AddInt32(&processedDocsVal, 1))
			reportProgress(fmt.Sprintf("Concluído %d/%d: %s", pDocs, totalFiles, f.DocName))
		}(i, f)
	}

	wg.Wait()

	if errParseFirst != nil {
		return "", 0, 0, errParseFirst
	}

	// Double check for context cancellation before generating/writing the file
	if err := ctx.Err(); err != nil {
		log.Printf("[Pipeline] Processamento cancelado antes de salvar o arquivo de saída")
		return "", 0, 0, err
	}

	// 4. Format consolidated output into XML
	outputText := GenerateConsolidatedOutput(eventos, cfg.CleaningLevel)

	// 5. Save consolidated output to the same directory
	outputPath := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".txt"
	err = os.WriteFile(outputPath, []byte(outputText), 0644)
	if err != nil {
		if wailsCtx := p.getContext(); wailsCtx != nil {
			runtime.EventsEmit(wailsCtx, "process:error", map[string]string{
				"file":  zipPath,
				"error": fmt.Sprintf("failed to save output file: %s", err.Error()),
			})
		}
		return "", 0, 0, err
	}

	// Count total pages
	totalPages := 0
	for _, val := range eventos {
		evData := val.(map[string]any)
		documentosMap := evData["documentos"].(map[string]any)
		for _, dVal := range documentosMap {
			dData := dVal.(map[string]any)
			paginasMap := dData["paginas"].(map[string]any)
			totalPages += len(paginasMap)
		}
	}

	if wailsCtx := p.getContext(); wailsCtx != nil {
		runtime.EventsEmit(wailsCtx, "process:success", map[string]interface{}{
			"file":        zipPath,
			"output_path": outputPath,
			"docs_count":  totalFiles,
			"pages_count": totalPages,
		})
	}

	return outputPath, totalFiles, totalPages, nil
}
