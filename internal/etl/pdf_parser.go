package etl

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/dslipak/pdf"
)

type ocrTask struct {
	pageNum  int
	fallback string
}

// PageData represents the extracted data of a single page.
type PageData struct {
	PageNum int    `json:"page_num"`
	Text    string `json:"text"`
	IsOcr   bool   `json:"is_ocr"`
}

// PDFParser wraps the PDF file processing.
type PDFParser struct {
	filePath string
}

// NewPDFParser creates a new PDFParser instance.
func NewPDFParser(filePath string) *PDFParser {
	return &PDFParser{filePath: filePath}
}

// ParsePages parses the PDF page-by-page.
// If native text is insufficient and OCR is enabled, it uses the OCR runner callback.
func (p *PDFParser) ParsePages(
	ctx context.Context,
	ocrEnabled bool,
	ocrRunner func(context.Context, string, int) (string, string, error),
	onPageComplete func(pageNum int),
) (map[string]any, error) {
	log.Printf("[PDFParser] Abrindo arquivo PDF: %s", p.filePath)
	file, err := os.Open(p.filePath)
	if err != nil {
		log.Printf("[PDFParser] Falha ao abrir PDF %s: %v", p.filePath, err)
		return nil, fmt.Errorf("failed to open pdf: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		log.Printf("[PDFParser] Falha ao ler stat de %s: %v", p.filePath, err)
		return nil, fmt.Errorf("failed to stat pdf: %w", err)
	}

	log.Printf("[PDFParser] Instanciando leitor de PDF (tamanho: %d bytes)", stat.Size())
	reader, err := pdf.NewReader(file, stat.Size())
	if err != nil {
		log.Printf("[PDFParser] Falha ao criar leitor de PDF para %s: %v", p.filePath, err)
		return nil, fmt.Errorf("failed to create pdf reader: %w", err)
	}

	numPages := reader.NumPage()
	log.Printf("[PDFParser] PDF %s possui %d páginas", p.filePath, numPages)
	
	paginasMap := make(map[string]any)
	var mapMu sync.Mutex
	var wg sync.WaitGroup

	// Concurrency is bounded by the shared OCR semaphore passed in via
	// ocrRunner (see Pipeline.ProcessZip), so each OCR task just blocks on
	// that instead of going through a second, redundant worker pool here.
	runOCRTask := func(task ocrTask) {
		defer wg.Done()
		log.Printf("[PDFParser] Iniciando OCR para página %d...", task.pageNum)
		ocrText, imgPath, err := ocrRunner(ctx, p.filePath, task.pageNum)

		var pageData map[string]any
		if err == nil {
			log.Printf("[PDFParser] OCR concluído para página %d (tamanho: %d)", task.pageNum, len(ocrText))
			pageData = map[string]any{
				"index":   task.pageNum,
				"ocr":     true,
				"image":   imgPath,
				"content": ocrText,
			}
		} else {
			log.Printf("[PDFParser] Falha no OCR para página %d: %v. Usando texto nativo como fallback.", task.pageNum, err)
			pageData = map[string]any{
				"index":   task.pageNum,
				"ocr":     false,
				"image":   "",
				"content": task.fallback,
			}
		}

		mapMu.Lock()
		paginasMap[fmt.Sprintf("%d", task.pageNum)] = pageData
		mapMu.Unlock()
		if onPageComplete != nil {
			onPageComplete(task.pageNum)
		}
	}

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("[PDFParser] Contexto cancelado na leitura da página %d: %v", pageNum, err)
			wg.Wait()
			return nil, err
		}

		page := reader.Page(pageNum)
		if page.V.IsNull() {
			log.Printf("[PDFParser] Página %d está nula/vazia", pageNum)
			pageData := map[string]any{
				"index":   pageNum,
				"ocr":     false,
				"image":   "",
				"content": "",
			}
			mapMu.Lock()
			paginasMap[fmt.Sprintf("%d", pageNum)] = pageData
			mapMu.Unlock()
			if onPageComplete != nil {
				onPageComplete(pageNum)
			}
			continue
		}

		// Extract native text
		log.Printf("[PDFParser] Extraindo texto nativo da página %d...", pageNum)
		text, err := ExtractNativeText(page)
		if err != nil {
			log.Printf("[PDFParser] Falha ao extrair texto nativo na página %d: %v", pageNum, err)
			text = ""
		}

		cleanText := strings.TrimSpace(text)
		log.Printf("[PDFParser] Texto nativo da página %d extraído. Tamanho bruto: %d, Limpo: %d", pageNum, len(text), len(cleanText))
		needsOcr := false

		if ocrEnabled {
			if len(cleanText) < 100 {
				log.Printf("[PDFParser] Texto nativo da página %d é muito curto (%d chars). Verificando se há imagens...", pageNum, len(cleanText))
				hasImages := HasImages(page)
				log.Printf("[PDFParser] Página %d possui imagens: %t", pageNum, hasImages)
				if hasImages {
					needsOcr = true
				}
			}
		}

		if needsOcr && ocrRunner != nil {
			wg.Add(1)
			go runOCRTask(ocrTask{
				pageNum:  pageNum,
				fallback: cleanText,
			})
		} else {
			log.Printf("[PDFParser] Usando texto nativo para página %d (OCR habilitado: %t, precisa de OCR: %t)", pageNum, ocrEnabled, needsOcr)
			pageData := map[string]any{
				"index":   pageNum,
				"ocr":     false,
				"image":   "",
				"content": cleanText,
			}
			mapMu.Lock()
			paginasMap[fmt.Sprintf("%d", pageNum)] = pageData
			mapMu.Unlock()
			if onPageComplete != nil {
				onPageComplete(pageNum)
			}
		}
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		log.Printf("[PDFParser] Análise abortada devido a cancelamento do contexto: %v", err)
		return nil, err
	}

	log.Printf("[PDFParser] Análise concluída. %d páginas retornadas.", len(paginasMap))
	return paginasMap, nil
}

// ExtractNativeText extracts plain text from a pdf.Page using GetTextByRow as preferred layout option.
func ExtractNativeText(page pdf.Page) (string, error) {
	log.Printf("[PDFParser] Chamando page.GetTextByRow()...")
	rows, err := page.GetTextByRow()
	if err != nil {
		log.Printf("[PDFParser] GetTextByRow() retornou erro: %v. Tentando GetPlainText()...", err)
		// Fallback to plain text if row parsing fails
		b, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("[PDFParser] GetPlainText() falhou: %v", err)
			return "", err
		}
		log.Printf("[PDFParser] GetPlainText() funcionou. Chars obtidos: %d", len(b))
		return b, nil
	}

	log.Printf("[PDFParser] GetTextByRow() encontrou %d linhas de texto.", len(rows))
	var sb strings.Builder
	for _, row := range rows {
		var words []string
		for _, word := range row.Content {
			words = append(words, word.S)
		}
		if len(words) > 0 {
			sb.WriteString(strings.Join(words, " "))
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// HasImages checks if a page contains any image XObject of significant resolution.
func HasImages(page pdf.Page) bool {
	resources := page.Resources()
	if resources.IsNull() {
		return false
	}
	xobject := resources.Key("XObject")
	if xobject.IsNull() {
		return false
	}
	for _, k := range xobject.Keys() {
		obj := xobject.Key(k)
		subtype := obj.Key("Subtype")
		if !subtype.IsNull() && subtype.Name() == "Image" {
			width := obj.Key("Width").Int64()
			height := obj.Key("Height").Int64()
			// Filter out small decoration images (e.g. logos, icons)
			if width > 100 && height > 100 {
				return true
			}
		}
	}
	return false
}
