package etl

import (
	"archive/zip"
	"bytes"
	"compress/flate"
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
)

// contarPaginasPDF é uma variável para que os testes simulem um panic da biblioteca de PDF.
var contarPaginasPDF = func(r io.ReaderAt, size int64) (*pdf.Reader, int, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return nil, 0, err
	}
	return reader, reader.NumPage(), nil
}

// aoFimDaPreContagem deixa os testes inspecionarem a memória de PDFs retida
// entre a pré-contagem e o processamento.
var aoFimDaPreContagem func(mem *memoriaPDF)

// Tetos para manter PDFs extraídos do ZIP em memória. Acima deles o PDF vai
// para um temporário em disco.
const (
	maxPDFEmMemoria      = 32 << 20
	maxTotalPDFEmMemoria = 256 << 20
)

// fontePDF é um PDF extraído do ZIP uma única vez: em memória até o teto, ou
// num temporário cujo handle fica aberto. Reabrir um arquivo recém-gravado
// custa dezenas de ms no Windows (antivírus), e PDFs sem OCR nem precisam de
// disco: o temporário só é gravado quando alguma página vai ao OCR.
type fontePDF struct {
	mu        sync.Mutex
	dados     []byte
	arq       *os.File
	caminho   string
	tamanho   int64
	reservado int64
	mem       *memoriaPDF

	leitor  *pdf.Reader
	paginas int
}

func (f *fontePDF) readerAt() io.ReaderAt {
	if f.arq != nil {
		return f.arq
	}
	return bytes.NewReader(f.dados)
}

// materializar devolve um caminho em disco para o OCR, gravando o temporário
// na primeira chamada.
func (f *fontePDF) materializar() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.caminho != "" {
		return f.caminho, nil
	}
	tmp, err := os.CreateTemp("", "eprocia_pdf_*.pdf")
	if err != nil {
		return "", err
	}
	_, err = tmp.Write(f.dados)
	if errClose := tmp.Close(); err == nil {
		err = errClose
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	f.caminho = tmp.Name()
	return f.caminho, nil
}

// memoriaPDF limita os bytes de PDF em memória num ZIP e reaproveita os
// buffers entre a pré-contagem e o processamento, que extraem o mesmo PDF duas
// vezes. Vive só durante um ProcessZip: um pool global retinha os buffers entre
// ZIPs e elevava o pico de memória. Canal nil (valor zero) desliga o reuso.
type memoriaPDF struct {
	disponivel atomic.Int64
	bufs       chan []byte
}

func (m *memoriaPDF) buffer(tamanho int64) []byte {
	select {
	case b := <-m.bufs:
		if int64(cap(b)) >= tamanho {
			return b[:tamanho]
		}
	default:
	}
	return make([]byte, tamanho)
}

func (m *memoriaPDF) devolver(b []byte) {
	select {
	case m.bufs <- b[:0]:
	default:
	}
}

// flateReciclavel devolve o descompressor a uma lista livre ao fechar. O pool
// do archive/zip é esvaziado a cada GC e, com cada PDF lido duas vezes, a
// janela de 32 KB realocada por entrada pesava no B/op.
type flateReciclavel struct {
	io.ReadCloser
	livres chan io.ReadCloser
}

func (f *flateReciclavel) Close() error {
	if f.ReadCloser == nil {
		return nil
	}
	err := f.ReadCloser.Close()
	select {
	case f.livres <- f.ReadCloser:
	default:
	}
	f.ReadCloser = nil
	return err
}

func descompressorReciclavel(livres chan io.ReadCloser) zip.Decompressor {
	return func(r io.Reader) io.ReadCloser {
		select {
		case fr := <-livres:
			if err := fr.(flate.Resetter).Reset(r, nil); err == nil {
				return &flateReciclavel{fr, livres}
			}
		default:
		}
		return &flateReciclavel{flate.NewReader(r), livres}
	}
}

// liberar fecha o handle antes de remover o temporário (no Windows, os.Remove
// falha com o arquivo aberto) e devolve a memória reservada ao orçamento.
func (f *fontePDF) liberar() {
	f.liberarBuffer(true)
}

// liberarBuffer com reusar=false não devolve o buffer ao pool: depois de um
// panic, goroutines de OCR do documento ainda podem lê-lo.
func (f *fontePDF) liberarBuffer(reusar bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.arq != nil {
		f.arq.Close()
		f.arq = nil
	}
	if f.caminho != "" {
		os.Remove(f.caminho)
		f.caminho = ""
	}
	if f.reservado > 0 {
		f.mem.disponivel.Add(f.reservado)
		f.reservado = 0
	}
	if reusar && f.dados != nil {
		f.mem.devolver(f.dados)
	}
	f.dados = nil
	f.leitor = nil
}

// extrairPDF lê a entrada do ZIP para memória, se couber no orçamento, ou para
// um temporário em disco mantido aberto.
func extrairPDF(f models.ProcessFile, mem *memoriaPDF) (*fontePDF, error) {
	zr, err := f.ZipFileRef.Open()
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tamanho := int64(f.ZipFileRef.UncompressedSize64)
	if tamanho <= maxPDFEmMemoria {
		if mem.disponivel.Add(-tamanho) >= 0 {
			dados := mem.buffer(tamanho)
			_, err := io.ReadFull(zr, dados)
			if err == nil {
				// Lê até o EOF para o archive/zip conferir o CRC e para não
				// truncar calado uma entrada maior que o tamanho declarado.
				var extra [1]byte
				if n, errEOF := zr.Read(extra[:]); n > 0 {
					err = fmt.Errorf("entrada do ZIP maior que o tamanho declarado")
				} else if errEOF != io.EOF {
					err = errEOF
				}
			}
			if err != nil {
				mem.disponivel.Add(tamanho)
				return nil, err
			}
			return &fontePDF{dados: dados, tamanho: tamanho, reservado: tamanho, mem: mem}, nil
		}
		mem.disponivel.Add(tamanho)
	}

	tmp, err := os.CreateTemp("", "eprocia_pdf_*.pdf")
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(tmp, zr)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	return &fontePDF{arq: tmp, caminho: tmp.Name(), tamanho: n, mem: mem}, nil
}

// contarPaginas abre o leitor do PDF uma vez; ele é reaproveitado por
// ParsePages. A biblioteca pode entrar em panic com arquivo malformado.
func (f *fontePDF) contarPaginas() (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			n, err = 0, fmt.Errorf("panic ao ler PDF: %v", r)
		}
	}()
	reader, n, err := contarPaginasPDF(f.readerAt(), f.tamanho)
	if err != nil {
		return 0, err
	}
	f.leitor, f.paginas = reader, n
	return n, nil
}

// Pipeline orchestrates the entire extraction process. It emits no Wails
// events: progress reaches the frontend through the Queue.
type Pipeline struct{}

// NewPipeline creates a new Pipeline instance.
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// ProcessZip extracts, parses, OCRs (if needed), sorts, and formats a ZIP file.
func (p *Pipeline) ProcessZip(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int)) (string, int, int, error) {
	// 1. Read ZIP files
	files, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		return "", 0, 0, err
	}
	defer rc.Close()

	if len(files) == 0 {
		errNoFiles := fmt.Errorf("no PDF or HTML documents found in ZIP")
		return "", 0, 0, errNoFiles
	}

	// 2. Sort documents chronologically and naturally
	SortProcessFiles(files)

	// Define active worker limits based on CPU count
	numWorkers := stdRuntime.NumCPU() - 1
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Pré-contagem das páginas para o progresso, sem tocar o disco para PDFs que
	// cabem em memória. É serial e não retém os PDFs para o processamento: com
	// cada um em memória, extrair de novo custa pouco, e reter todos (ou contar
	// em paralelo) elevava o pico de memória.
	totalFiles := len(files)
	fontes := make([]*fontePDF, totalFiles)
	contagens := make([]int, totalFiles)
	contados := make([]bool, totalFiles)
	// Listas livres pequenas: reaproveitam buffers e descompressores entre as
	// duas extrações de cada PDF sem segurar memória de todos os workers.
	mem := &memoriaPDF{bufs: make(chan []byte, 2)}
	mem.disponivel.Store(maxTotalPDFEmMemoria)
	rc.RegisterDecompressor(zip.Deflate, descompressorReciclavel(make(chan io.ReadCloser, 2)))
	// Libera os PDFs extraídos em qualquer saída, inclusive por panic.
	defer func() {
		for _, fonte := range fontes {
			if fonte != nil {
				fonte.liberar()
			}
		}
	}()

	for i, f := range files {
		contagens[i] = 1
		if f.Extension != ".pdf" {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		fonte, err := extrairPDF(f, mem)
		if err == nil {
			var n int
			if n, err = fonte.contarPaginas(); err == nil {
				contagens[i], contados[i] = n, true
			}
			// O temporário em disco (PDF acima do teto) fica aberto para não
			// ser gravado duas vezes.
			if err == nil && fonte.arq != nil {
				fontes[i] = fonte
			} else {
				fonte.liberar()
			}
		}
		if err != nil {
			log.Printf("[Pipeline] Erro ao pré-calcular páginas para %s: %v", f.DocName, err)
		}
	}
	if aoFimDaPreContagem != nil {
		aoFimDaPreContagem(mem)
	}
	if err := ctx.Err(); err != nil {
		log.Printf("[Pipeline] Processamento cancelado durante o pré-cálculo de páginas")
		return "", 0, 0, err
	}

	totalPagesAll := 0
	docPageCounts := make(map[string]int, totalFiles)
	for i, f := range files {
		docPageCounts[f.Name] = contagens[i]
		totalPagesAll += contagens[i]
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

			// Um panic não recuperado neste goroutine derruba o app inteiro; o
			// documento é marcado com erro e os demais seguem.
			var fonteDoc *fontePDF
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				log.Printf("[Pipeline] Panic ao processar %s: %v", f.DocName, r)
				if fonteDoc != nil {
					fonteDoc.liberarBuffer(false)
				}
				docData := eventos[fmt.Sprintf("%d", f.EventNum)].(map[string]any)["documentos"].(map[string]any)[f.Name].(map[string]any)
				paginasMap := docData["paginas"].(map[string]any)
				for k := range paginasMap {
					delete(paginasMap, k)
				}
				paginasMap["1"] = map[string]any{
					"index":   1,
					"ocr":     false,
					"image":   "",
					"content": fmt.Sprintf("[ERRO ao processar documento: %v]", r),
				}
				reportProgress(fmt.Sprintf("Erro ao processar %s", f.DocName))
			}()

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
				fonte := fontes[i]
				if fonte == nil {
					var err error
					fonte, err = extrairPDF(f, mem)
					if err != nil {
						paginasMap["1"] = map[string]any{
							"index":   1,
							"ocr":     false,
							"image":   "",
							"content": fmt.Sprintf("[ERRO ao extrair PDF do ZIP: %s]", err.Error()),
						}
						atomic.AddInt32(&processedPagesVal, 1)
						reportProgress(fmt.Sprintf("Erro ao abrir %s", f.DocName))
						return
					}
					fontes[i] = fonte
				}
				fonteDoc = fonte

				parser := NewPDFParserDeLeitor(fonte.readerAt(), fonte.tamanho, f.DocName, fonte.materializar)
				if fonte.leitor != nil {
					parser.comLeitor(fonte.leitor, fonte.paginas)
				} else if contados[i] {
					parser.comPaginas(contagens[i])
				}
				pdfPages, errParse := parser.ParsePages(ctx, cfg.OcrMode != "none", ocrRunner, func(pageNum int) {
					atomic.AddInt32(&processedPagesVal, 1)
					pDocs := int(atomic.LoadInt32(&processedDocsVal))
					reportProgress(fmt.Sprintf("Processando %s (pág. %d/%d) [Docs: %d/%d]", f.DocName, pageNum, docPageCounts[f.Name], pDocs, totalFiles))
				})
				fonte.liberar()

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
	// WriteString não copia o texto para um []byte, como os.WriteFile exigiria.
	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", 0, 0, err
	}
	_, err = out.WriteString(outputText)
	if errClose := out.Close(); err == nil {
		err = errClose
	}
	if err != nil {
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

	return outputPath, totalFiles, totalPages, nil
}
