package etl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

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
	reader, numPages, err := abrirLeitorPDF(file, stat.Size())
	if err != nil {
		log.Printf("[PDFParser] Falha ao criar leitor de PDF para %s: %v", p.filePath, err)
		return nil, fmt.Errorf("failed to create pdf reader: %w", err)
	}

	log.Printf("[PDFParser] PDF %s possui %d páginas", p.filePath, numPages)

	paginasMap := make(map[string]any)
	var mapMu sync.Mutex
	var wg sync.WaitGroup
	var ocrFailures int32

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
			content := task.fallback
			if content == "" {
				content = fmt.Sprintf("[ERRO de OCR: %s]", err.Error())
			}
			log.Printf("[PDFParser] Falha no OCR para página %d: %v. Usando texto nativo como fallback (%d chars).", task.pageNum, err, len(task.fallback))
			atomic.AddInt32(&ocrFailures, 1)
			pageData = map[string]any{
				"index":    task.pageNum,
				"ocr":      false,
				"image":    "",
				"content":  content,
				"ocr_erro": err.Error(),
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

		an, errPag := analisarPagina(reader, pageNum, ocrEnabled)
		if errPag != nil || an.nula {
			content := ""
			if errPag != nil {
				log.Printf("[PDFParser] Falha ao ler a página %d de %s: %v", pageNum, p.filePath, errPag)
				content = fmt.Sprintf("[ERRO ao ler página: %s]", errPag.Error())
			} else {
				log.Printf("[PDFParser] Página %d está nula/vazia", pageNum)
			}
			mapMu.Lock()
			paginasMap[fmt.Sprintf("%d", pageNum)] = map[string]any{
				"index":   pageNum,
				"ocr":     false,
				"image":   "",
				"content": content,
			}
			mapMu.Unlock()
			if onPageComplete != nil {
				onPageComplete(pageNum)
			}
			continue
		}
		cleanText, needsOcr := an.texto, an.precisaOcr

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

	if n := atomic.LoadInt32(&ocrFailures); n > 0 {
		log.Printf("[PDFParser] ATENÇÃO: %d página(s) de %s com falha de OCR.", n, p.filePath)
	}
	log.Printf("[PDFParser] Análise concluída. %d páginas retornadas.", len(paginasMap))
	return paginasMap, nil
}

// abrirLeitorPDF opens the reader and counts pages. The PDF library panics on
// an inconsistent xref (e.g. "loading 1 0 R: found 9 0"), so that becomes an error.
func abrirLeitorPDF(r io.ReaderAt, size int64) (reader *pdf.Reader, numPages int, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			reader, numPages, err = nil, 0, fmt.Errorf("pânico ao abrir o PDF: %v", rec)
		}
	}()
	reader, err = pdf.NewReader(r, size)
	if err != nil {
		return nil, 0, err
	}
	return reader, reader.NumPage(), nil
}

type analisePagina struct {
	nula       bool
	texto      string
	precisaOcr bool
}

// analisarPagina resolves one page, extracts its native text and decides on
// OCR. Page and HasImages resolve objects lazily and panic on a malformed xref;
// the panic is confined to this page so the rest of the document is still read.
func analisarPagina(reader *pdf.Reader, pageNum int, ocrEnabled bool) (an analisePagina, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			an, err = analisePagina{}, fmt.Errorf("pânico na leitura da página %d: %v", pageNum, rec)
		}
	}()

	page := reader.Page(pageNum)
	if page.V.IsNull() {
		return analisePagina{nula: true}, nil
	}

	log.Printf("[PDFParser] Extraindo texto nativo da página %d...", pageNum)
	text, errTexto := ExtractNativeText(page)
	if errTexto != nil {
		log.Printf("[PDFParser] Falha ao extrair texto nativo na página %d: %v", pageNum, errTexto)
		text = ""
	}

	an.texto = strings.TrimSpace(text)
	log.Printf("[PDFParser] Texto nativo da página %d extraído. Tamanho bruto: %d, Limpo: %d", pageNum, len(text), len(an.texto))

	if ocrEnabled && len(an.texto) < 100 {
		log.Printf("[PDFParser] Texto nativo da página %d é muito curto (%d chars). Verificando se há imagens...", pageNum, len(an.texto))
		hasImages := HasImages(page)
		hasDrawn := !hasImages && HasDrawnContent(page)
		log.Printf("[PDFParser] Página %d possui imagens: %t, conteúdo vetorial: %t", pageNum, hasImages, hasDrawn)
		an.precisaOcr = hasImages || hasDrawn
	}
	return an, nil
}

// drawnContentMinBytes is the decoded content-stream size above which a page
// with no native text is assumed to draw text some other way. Blank pages and
// pages with a few rules or logos stay well below it.
const drawnContentMinBytes = 4096

// HasDrawnContent reports whether the page paints a substantial amount of
// content. Some generators (e.g. iLovePDF) draw text as vector outlines with no
// fonts and no images; such pages have no native text but are readable by OCR.
func HasDrawnContent(page pdf.Page) (drawn bool) {
	// The PDF library panics on unsupported filters; treat that as "unknown".
	defer func() {
		if recover() != nil {
			drawn = false
		}
	}()

	contents := page.V.Key("Contents")
	streams := []pdf.Value{contents}
	if contents.Kind() == pdf.Array {
		streams = streams[:0]
		for i := 0; i < contents.Len(); i++ {
			streams = append(streams, contents.Index(i))
		}
	}

	var total int64
	for _, s := range streams {
		if s.Kind() != pdf.Stream {
			continue
		}
		rc := s.Reader()
		n, _ := io.Copy(io.Discard, rc)
		rc.Close()
		total += n
		if total >= drawnContentMinBytes {
			return true
		}
	}
	return false
}

// checkContentStream verifies that the page content is a single, fully readable
// stream. The PDF library only interprets that shape: for a /Contents array
// (valid PDF, emitted e.g. by "Microsoft: Print To PDF"), a missing /Contents or
// a corrupt stream, its lexer swallows the read error and spins forever.
func checkContentStream(page pdf.Page) error {
	contents := page.V.Key("Contents")
	if contents.Kind() != pdf.Stream {
		return fmt.Errorf("conteúdo da página não é um stream único (kind %v)", contents.Kind())
	}
	rc := contents.Reader()
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("stream de conteúdo ilegível: %w", err)
	}
	return nil
}

// ExtractNativeText extracts plain text from a pdf.Page using GetTextByRow as preferred layout option.
// Pages the PDF library cannot interpret yield an error (never a hang or a panic),
// so the caller treats them as having no native text and may fall back to OCR.
func ExtractNativeText(page pdf.Page) (text string, err error) {
	// The library panics on unsupported filters and malformed objects; this runs
	// in a per-document goroutine, where an unrecovered panic kills the app.
	defer func() {
		if r := recover(); r != nil {
			text, err = "", fmt.Errorf("pânico na extração de texto nativo: %v", r)
		}
	}()

	if err := checkContentStream(page); err != nil {
		log.Printf("[PDFParser] Extração nativa ignorada: %v", err)
		return "", err
	}

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
