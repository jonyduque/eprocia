package etl

import (
	"context"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dslipak/pdf"
)

type ocrTask struct {
	pageNum  int
	fallback string
}

// PDFParser wraps the PDF file processing.
type PDFParser struct {
	filePath string

	// Fonte já aberta (PDF em memória ou temporário com handle aberto).
	// ParsePages só pede um caminho em disco a caminhoOCR se alguma página for
	// ao OCR.
	fonte      io.ReaderAt
	tamanho    int64
	leitor     *pdf.Reader
	numPaginas int
	caminhoOCR func() (string, error)
	onceOCR    sync.Once
	pathOCR    string
	errOCR     error

	lote      ocrLote
	blocosOCR int
}

// ocrLote faz o OCR de várias páginas do mesmo PDF e devolve os textos na
// ordem de paginas, chamando aoConcluir com cada página assim que o texto dela
// fica pronto.
type ocrLote func(ctx context.Context, caminho string, paginas []int, aoConcluir func(pagina int)) ([]string, error)

// comOCREmLote faz ParsePages dividir as páginas que vão ao OCR em até blocos
// chamadas de lote, em vez de uma chamada por página: cada processo de OCR
// custa a partida e a carga do modelo, e blocos igual ao número de workers
// mantém o paralelismo.
func (p *PDFParser) comOCREmLote(lote ocrLote, blocos int) *PDFParser {
	p.lote, p.blocosOCR = lote, max(blocos, 1)
	return p
}

// NewPDFParserDeLeitor lê o PDF de r. nome só aparece nos logs; materializar
// devolve um caminho em disco para o OCR e é chamado no máximo uma vez.
func NewPDFParserDeLeitor(r io.ReaderAt, size int64, nome string, materializar func() (string, error)) *PDFParser {
	return &PDFParser{filePath: nome, fonte: r, tamanho: size, caminhoOCR: materializar}
}

// comLeitor reaproveita o leitor já aberto na pré-contagem, evitando um
// segundo parse da xref.
func (p *PDFParser) comLeitor(reader *pdf.Reader, numPaginas int) *PDFParser {
	p.leitor, p.numPaginas = reader, numPaginas
	return p
}

// comPaginas informa o número de páginas já contado na pré-contagem, para que
// ParsePages não resolva a árvore de páginas de novo só para contá-las.
func (p *PDFParser) comPaginas(numPaginas int) *PDFParser {
	p.numPaginas = numPaginas
	return p
}

func (p *PDFParser) caminhoParaOCR() (string, error) {
	if p.caminhoOCR == nil {
		return "", fmt.Errorf("o PDF %s não tem caminho em disco para o OCR", p.filePath)
	}
	p.onceOCR.Do(func() {
		p.pathOCR, p.errOCR = p.caminhoOCR()
		if p.errOCR != nil {
			p.errOCR = fmt.Errorf("falha ao preparar o PDF para OCR: %w", p.errOCR)
		}
	})
	return p.pathOCR, p.errOCR
}

// abrir devolve o leitor do PDF e o número de páginas, reaproveitando o
// leitor da pré-contagem quando houver.
func (p *PDFParser) abrir() (*pdf.Reader, int, func(), error) {
	if p.leitor != nil {
		return p.leitor, p.numPaginas, func() {}, nil
	}
	log.Printf("[PDFParser] Instanciando leitor de PDF (tamanho: %d bytes)", p.tamanho)
	reader, n, err := abrirLeitorPDF(p.fonte, p.tamanho, p.numPaginas)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("falha ao abrir o PDF: %w", err)
	}
	return reader, n, func() {}, nil
}

// ParsePages parses the PDF page-by-page.
// If native text is insufficient and OCR is enabled, it uses the OCR runner callback.
func (p *PDFParser) ParsePages(
	ctx context.Context,
	ocrEnabled bool,
	ocrRunner func(context.Context, string, int) (string, error),
	onPageComplete func(pageNum int),
) (map[string]any, error) {
	reader, numPages, fechar, err := p.abrir()
	if err != nil {
		log.Printf("[PDFParser] Falha ao abrir PDF %s: %v", p.filePath, err)
		return nil, err
	}
	defer fechar()

	log.Printf("[PDFParser] PDF %s possui %d páginas", p.filePath, numPages)

	paginas, errResto, err := listarPaginas(reader, numPages)
	if err != nil {
		log.Printf("[PDFParser] Árvore de páginas inválida em %s: %v", p.filePath, err)
		return nil, fmt.Errorf("árvore de páginas inválida: %w", err)
	}

	paginasMap := make(map[string]any)
	var mapMu sync.Mutex
	var wg sync.WaitGroup
	var ocrFailures atomic.Int32

	// O OCR em lote avisa cada página assim que o texto dela sai do tesseract;
	// se o bloco cair depois e for refeito página a página, a página não pode
	// contar duas vezes no progresso.
	reportadas := make([]atomic.Bool, numPages+1)
	concluir := func(pageNum int) {
		if onPageComplete == nil || pageNum < 1 || pageNum > numPages {
			return
		}
		if !reportadas[pageNum].Swap(true) {
			onPageComplete(pageNum)
		}
	}

	// Concurrency is bounded by the shared OCR semaphore passed in via
	// ocrRunner (see Pipeline.ProcessZip), so each OCR task just blocks on
	// that instead of going through a second, redundant worker pool here.
	registrarOCR := func(task ocrTask, ocrText string, err error) {
		var pageData map[string]any
		if err == nil {
			log.Printf("[PDFParser] OCR concluído para página %d (tamanho: %d)", task.pageNum, len(ocrText))
			pageData = map[string]any{
				"index":   task.pageNum,
				"ocr":     true,
				"content": ocrText,
			}
		} else {
			content := task.fallback
			if content == "" {
				content = fmt.Sprintf("[ERRO de OCR: %s]", err.Error())
			}
			log.Printf("[PDFParser] Falha no OCR para página %d: %v. Usando texto nativo como fallback (%d chars).", task.pageNum, err, len(task.fallback))
			ocrFailures.Add(1)
			pageData = map[string]any{
				"index":    task.pageNum,
				"ocr":      false,
				"content":  content,
				"ocr_erro": err.Error(),
			}
		}

		mapMu.Lock()
		paginasMap[fmt.Sprintf("%d", task.pageNum)] = pageData
		mapMu.Unlock()
		concluir(task.pageNum)
	}

	runOCRTask := func(task ocrTask) {
		if ctx.Err() != nil {
			return
		}
		log.Printf("[PDFParser] Iniciando OCR para página %d...", task.pageNum)
		caminho, err := p.caminhoParaOCR()
		var ocrText string
		if err == nil {
			ocrText, err = ocrRunner(ctx, caminho, task.pageNum)
		}
		registrarOCR(task, ocrText, err)
	}

	runOCRBloco := func(tasks []ocrTask) {
		if len(tasks) == 1 {
			runOCRTask(tasks[0])
			return
		}
		paginas := make([]int, len(tasks))
		for i, task := range tasks {
			paginas[i] = task.pageNum
		}
		log.Printf("[PDFParser] Iniciando OCR em bloco das páginas %v...", paginas)
		caminho, err := p.caminhoParaOCR()
		var textos []string
		if err == nil {
			textos, err = p.lote(ctx, caminho, paginas, concluir)
		}
		if err == nil && len(textos) != len(tasks) {
			err = fmt.Errorf("OCR em bloco devolveu %d textos para %d páginas", len(textos), len(tasks))
		}
		if err != nil && ctx.Err() != nil {
			log.Printf("[PDFParser] OCR em bloco das páginas %v interrompido por cancelamento: %v", paginas, ctx.Err())
			return
		}
		if err != nil {
			// Uma página ruim derruba o tesseract do bloco inteiro; sozinhas,
			// as outras ainda saem.
			log.Printf("[PDFParser] OCR em bloco das páginas %v falhou: %v. Refazendo página a página...", paginas, err)
			for _, task := range tasks {
				runOCRTask(task)
			}
			return
		}
		for i, task := range tasks {
			registrarOCR(task, textos[i], nil)
		}
	}

	// Um bloco vai ao OCR assim que junta tamBloco páginas, enquanto o texto
	// nativo das seguintes ainda é lido. tamBloco é o bloco que sairia se todas
	// as páginas fossem ao OCR, então um documento todo escaneado ainda dá um
	// bloco por worker; o que sobrar no fim se divide pelos blocos restantes.
	var pendentes []ocrTask
	tamBloco := (numPages + p.blocosOCR - 1) / max(p.blocosOCR, 1)
	despachados := 0
	despachar := func(bloco []ocrTask) {
		despachados++
		wg.Go(func() { runOCRBloco(bloco) })
	}

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("[PDFParser] Contexto cancelado na leitura da página %d: %v", pageNum, err)
			wg.Wait()
			return nil, err
		}

		var an analisePagina
		var errPag error
		switch {
		case pageNum <= len(paginas):
			an, errPag = analisarPagina(paginas[pageNum-1], pageNum, ocrEnabled)
		case errResto != nil:
			errPag = errResto
		default:
			an.nula = true
		}
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
				"content": content,
			}
			mapMu.Unlock()
			concluir(pageNum)
			continue
		}
		cleanText, needsOcr := an.texto, an.precisaOcr

		if needsOcr && ocrRunner != nil {
			task := ocrTask{pageNum: pageNum, fallback: cleanText}
			if p.lote != nil {
				pendentes = append(pendentes, task)
				if len(pendentes) == tamBloco {
					despachar(pendentes)
					pendentes = nil
				}
			} else {
				wg.Go(func() { runOCRBloco([]ocrTask{task}) })
			}
		} else {
			log.Printf("[PDFParser] Usando texto nativo para página %d (OCR habilitado: %t, precisa de OCR: %t)", pageNum, ocrEnabled, needsOcr)
			pageData := map[string]any{
				"index":   pageNum,
				"ocr":     false,
				"content": cleanText,
			}
			mapMu.Lock()
			paginasMap[fmt.Sprintf("%d", pageNum)] = pageData
			mapMu.Unlock()
			concluir(pageNum)
		}
	}

	if len(pendentes) > 0 {
		restantes := max(p.blocosOCR-despachados, 1)
		k := (len(pendentes) + restantes - 1) / restantes
		for bloco := range slices.Chunk(pendentes, k) {
			despachar(bloco)
		}
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		log.Printf("[PDFParser] Análise abortada devido a cancelamento do contexto: %v", err)
		return nil, err
	}

	if n := ocrFailures.Load(); n > 0 {
		log.Printf("[PDFParser] ATENÇÃO: %d página(s) de %s com falha de OCR.", n, p.filePath)
	}
	log.Printf("[PDFParser] Análise concluída. %d páginas retornadas.", len(paginasMap))
	return paginasMap, nil
}

// abrirLeitorPDF opens the reader and counts pages, unless paginasConhecidas is
// positive. The PDF library panics on an inconsistent xref (e.g. "loading 1 0
// R: found 9 0"), so that becomes an error.
func abrirLeitorPDF(r io.ReaderAt, size int64, paginasConhecidas int) (reader *pdf.Reader, numPages int, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			reader, numPages, err = nil, 0, fmt.Errorf("pânico ao abrir o PDF: %v", rec)
		}
	}()
	reader, err = pdf.NewReader(r, size)
	if err != nil {
		return nil, 0, err
	}
	if paginasConhecidas > 0 {
		return reader, paginasConhecidas, nil
	}
	return reader, reader.NumPage(), nil
}

// maxProfundidadePaginas bounds the /Pages tree depth. Real trees are a few
// levels deep; a cycle (e.g. /Kids [2 0 R] inside object 2) only shows up as
// unbounded depth, since the library does not expose object identity.
const maxProfundidadePaginas = 64

// listarPaginas walks the /Pages tree once, in document order, and returns the
// leaf pages. It replaces reader.Page, which descends /Kids with no visited set
// and spins forever on a cyclic tree. A cycle, excessive depth or too many nodes
// is a fatal error for the document. A panic while resolving a node (malformed
// xref) stops the walk: the pages found so far are kept and errResto applies to
// the remaining ones, as reader.Page would also panic on them.
func listarPaginas(reader *pdf.Reader, numPages int) (paginas []pdf.Page, errResto, err error) {
	orcamento := 4*numPages + 1024

	var visitar func(no pdf.Value, prof int) error
	visitar = func(no pdf.Value, prof int) error {
		if len(paginas) >= numPages {
			return nil
		}
		orcamento--
		if orcamento < 0 {
			return fmt.Errorf("árvore de páginas com nós demais para %d páginas", numPages)
		}
		switch no.Key("Type").Name() {
		case "Page":
			paginas = append(paginas, pdf.Page{V: no})
		case "Pages":
			if prof >= maxProfundidadePaginas {
				return fmt.Errorf("árvore de páginas cíclica ou com mais de %d níveis", maxProfundidadePaginas)
			}
			kids := no.Key("Kids")
			for i := 0; i < kids.Len() && len(paginas) < numPages; i++ {
				if err := visitar(kids.Index(i), prof+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				errResto = fmt.Errorf("pânico ao percorrer a árvore de páginas: %v", rec)
			}
		}()
		err = visitar(reader.Trailer().Key("Root").Key("Pages"), 0)
	}()
	if err != nil {
		return nil, nil, err
	}
	return paginas, errResto, nil
}

type analisePagina struct {
	nula       bool
	texto      string
	precisaOcr bool
}

// analisarPagina extracts the native text of one page and decides on OCR. The
// library resolves objects lazily and panics on a malformed xref; the panic is
// confined to this page so the rest of the document is still read.
func analisarPagina(page pdf.Page, pageNum int, ocrEnabled bool) (an analisePagina, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			an, err = analisePagina{}, fmt.Errorf("pânico na leitura da página %d: %v", pageNum, rec)
		}
	}()

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
	// Não há fallback para GetPlainText: na v0.0.2 o recover de GetTextByRow
	// não chega ao retorno (resultados sem nome), então err é sempre nil e um
	// conteúdo malformado devolve zero linhas e o fallback nunca rodava.
	rows, err := page.GetTextByRow()
	if err != nil {
		log.Printf("[PDFParser] GetTextByRow() retornou erro: %v", err)
		return "", err
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
