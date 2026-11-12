package etl

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	stdRuntime "runtime"
	"slices"
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
// It returns the output path, document and page counts and the estimated tokens.
func (p *Pipeline) ProcessZip(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int)) (string, int, int, int, error) {
	return p.ProcessZipComAtividade(ctx, zipPath, cfg, onProgress, nil)
}

// ProcessZipComAtividade é ProcessZip registrando em atv o estado de cada
// documento e dos processos de OCR. atv pode ser nil.
func (p *Pipeline) ProcessZipComAtividade(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int), atv *Atividade) (string, int, int, int, error) {
	// 1. Read ZIP files
	files, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		return "", 0, 0, 0, err
	}
	defer rc.Close()

	if len(files) == 0 {
		errNoFiles := fmt.Errorf("no PDF or HTML documents found in ZIP")
		return "", 0, 0, 0, errNoFiles
	}

	// 2. Sort documents chronologically and naturally
	SortProcessFiles(files)

	// Define active worker limits based on CPU count
	numWorkers := max(stdRuntime.NumCPU()-1, 1)

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
		return "", 0, 0, 0, err
	}

	totalPagesAll := 0
	for i := range files {
		totalPagesAll += contagens[i]
	}
	log.Printf("[Pipeline] Pré-cálculo concluído: %d documentos, %d páginas no total", totalFiles, totalPagesAll)

	eventos := make(map[string]any)

	// Group files by EventNum to pre-populate map
	eventsMap := make(map[int][]int)
	var eventNums []int
	for i, f := range files {
		if _, exists := eventsMap[f.EventNum]; !exists {
			eventNums = append(eventNums, f.EventNum)
		}
		eventsMap[f.EventNum] = append(eventsMap[f.EventNum], i)
	}

	// Initialize the events map structure. Cada goroutine recebe o mapa de
	// páginas do seu documento por índice: a chave do documento leva o índice
	// porque o ZIP aceita duas entradas com o mesmo caminho, e elas não podem
	// dividir o mesmo mapa.
	paginasDoc := make([]map[string]any, totalFiles)
	docsMap := make([]map[string]any, totalFiles)
	omitidos := make([]bool, totalFiles)
	for _, eventNum := range eventNums {
		eventKey := fmt.Sprintf("%d", eventNum)
		documentosMap := make(map[string]any)
		eventos[eventKey] = map[string]any{
			"index":      eventNum,
			"documentos": documentosMap,
		}

		for index, i := range eventsMap[eventNum] {
			paginasDoc[i] = make(map[string]any)
			doc := map[string]any{
				"index":   index + 1,
				"nome":    files[i].DocName,
				"paginas": paginasDoc[i],
			}
			// As chaves novas entram só aqui, antes das goroutines.
			if tipo := tipoDocumento(files[i].DocName); tipo != "" && slices.Contains(cfg.OmitirTipos, tipo) {
				doc["omitido"] = tipo
				doc["total_paginas"] = contagens[i]
				omitidos[i] = true
			}
			docsMap[i] = doc
			documentosMap[fmt.Sprintf("%d:%s", i, files[i].Name)] = doc
		}
	}

	docsAtv := make([]AtividadeDoc, totalFiles)
	for i, f := range files {
		docsAtv[i] = AtividadeDoc{Nome: f.DocName, Evento: f.EventNum, Estado: DocAguardando, Paginas: contagens[i]}
	}
	atv.iniciar(docsAtv, numWorkers, numWorkers)

	// O OCR de todo o ZIP divide um semáforo de numWorkers vagas. Cada documento
	// recebe os seus executores, que marcam na atividade as páginas em OCR só
	// depois de conseguir a vaga.
	sem := make(chan struct{}, numWorkers)
	ocrDoDoc := func(i int) (func(context.Context, string, int) (string, error), ocrLote) {
		runner := func(ctx context.Context, pdfPath string, pageNum int) (string, error) {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			defer func() { <-sem }()
			atv.ocrInicio(i, 1)
			defer atv.ocrFim(i, 1)
			return ocr.RunOCR(ctx, pdfPath, pageNum, cfg.OcrMode)
		}
		lote := func(ctx context.Context, pdfPath string, paginas []int, aoConcluir func(int)) ([]string, error) {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			defer func() { <-sem }()
			atv.ocrInicio(i, len(paginas))
			var prontas atomic.Int32
			defer func() { atv.ocrFim(i, len(paginas)-int(prontas.Load())) }()
			return ocr.RunOCRPaginas(ctx, pdfPath, paginas, cfg.OcrMode, func(pagina int) {
				// A página sai da contagem de OCR do documento assim que fica pronta;
				// o processo segue contado em OCRAtivos até o lote terminar.
				if int(prontas.Add(1)) <= len(paginas) {
					atv.mudar(i, func(d *AtividadeDoc) { d.OCR = max(d.OCR-1, 0) })
				}
				if aoConcluir != nil {
					aoConcluir(pagina)
				}
			})
		}
		return runner, lote
	}

	var processedDocsVal atomic.Int32
	var processedPagesVal atomic.Int32

	// Progress reporting helper. Os contadores são lidos fora de qualquer trava,
	// então dois documentos podem entregar valores fora de ordem: a fila só
	// aceita avanço (ver Queue.processItem).
	reportProgress := func(msg string) {
		pDocs := int(processedDocsVal.Load())
		pPages := int(processedPagesVal.Load())

		progress := 0.0
		if totalPagesAll > 0 {
			progress = min(float64(pPages)/float64(totalPagesAll), 1)
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
		wg.Go(func() {
			paginasMap, docMap := paginasDoc[i], docsMap[i]
			// Páginas deste documento já contadas no progresso. O OCR em lote
			// avisa páginas de outras goroutines, por isso o contador é atômico.
			var feitas atomic.Int32
			contarPagina := func() {
				feitas.Add(1)
				processedPagesVal.Add(1)
				atv.docPagina(i)
			}
			// encerrar fecha o documento em qualquer saída que não seja
			// cancelamento: conta o documento e completa as páginas que a
			// pré-contagem previa e não foram avisadas (um PDF que falhou ao
			// abrir), para os contadores chegarem ao total.
			var encerrado bool
			encerrar := func(msgErro, msgProgresso string) {
				if encerrado {
					return
				}
				encerrado = true
				if falta := contagens[i] - int(feitas.Load()); falta > 0 {
					processedPagesVal.Add(int32(falta))
				}
				processedDocsVal.Add(1)
				atv.docFim(i, msgErro)
				reportProgress(msgProgresso)
			}
			marcarErro := func(conteudo string) {
				for k := range paginasMap {
					delete(paginasMap, k)
				}
				paginasMap["1"] = map[string]any{
					"index":   1,
					"ocr":     false,
					"content": conteudo,
				}
			}
			// naoLido encerra um documento sem texto: o marcador (se houver)
			// continua no .txt, o painel recebe a frase e o texto técnico fica
			// no detalhe. Sem marcador, as páginas já lidas ficam como estão.
			naoLido := func(l Leitura, marcador, detalhe, msgProgresso string) {
				if marcador != "" {
					marcarErro(marcador)
				}
				docMap["nao_lido"] = l.Frase()
				atv.docLeitura(i, l, detalhe)
				encerrar(l.Frase(), msgProgresso)
			}

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
				naoLido(LeituraIlegivel, fmt.Sprintf("[ERRO ao processar documento: %v]", r),
					fmt.Sprintf("%v", r), fmt.Sprintf("Erro ao processar %s", f.DocName))
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
			// Tipo omitido: o arquivo não é aberto nem passa por OCR; encerrar
			// completa as páginas pré-contadas.
			if omitidos[i] {
				log.Printf("[Pipeline] Documento %s omitido por tipo", f.DocName)
				atv.mudar(i, func(d *AtividadeDoc) { d.Feitas = d.Paginas })
				encerrar("", fmt.Sprintf("Omitido %d/%d: %s", int(processedDocsVal.Load())+1, totalFiles, f.DocName))
				return
			}
			atv.docLendo(i)

			if f.Extension == ".pdf" {
				fonte := fontes[i]
				if fonte == nil {
					var err error
					fonte, err = extrairPDF(f, mem)
					if err != nil {
						naoLido(classificarErro(err), fmt.Sprintf("[ERRO ao extrair PDF do ZIP: %s]", err.Error()),
							err.Error(), fmt.Sprintf("Erro ao abrir %s", f.DocName))
						return
					}
					fontes[i] = fonte
				}
				fonteDoc = fonte

				ocrRunner, ocrEmLote := ocrDoDoc(i)
				parser := NewPDFParserDeLeitor(fonte.readerAt(), fonte.tamanho, f.DocName, fonte.materializar).
					comOCREmLote(ocrEmLote, numWorkers)
				if fonte.leitor != nil {
					parser.comLeitor(fonte.leitor, fonte.paginas)
				} else if contados[i] {
					parser.comPaginas(contagens[i])
				}
				pdfPages, errParse := parser.ParsePages(ctx, cfg.OcrMode != "none", ocrRunner, func(pageNum int) {
					contarPagina()
					pDocs := int(processedDocsVal.Load())
					reportProgress(fmt.Sprintf("Processando %s (pág. %d/%d) [Docs: %d/%d]", f.DocName, pageNum, contagens[i], pDocs, totalFiles))
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
					naoLido(classificarErro(errParse), fmt.Sprintf("[ERRO ao analisar PDF: %s]", errParse.Error()),
						errParse.Error(), fmt.Sprintf("Erro ao analisar %s", f.DocName))
					return
				}

				// Copy the pages map returned by ParsePages into the document's paginas map
				maps.Copy(paginasMap, pdfPages)
			} else if f.Extension == ".html" || f.Extension == ".htm" {
				zipReader, err := f.ZipFileRef.Open()
				if err != nil {
					naoLido(classificarErro(err), fmt.Sprintf("[ERRO ao abrir arquivo no ZIP: %s]", err.Error()),
						err.Error(), fmt.Sprintf("Erro ao abrir %s", f.DocName))
					return
				}

				htmlText, errParse := ExtractTextFromHTML(zipReader)
				zipReader.Close()
				if errParse != nil {
					naoLido(classificarErro(errParse), fmt.Sprintf("[ERRO ao analisar HTML: %s]", errParse.Error()),
						errParse.Error(), fmt.Sprintf("Erro ao analisar HTML %s", f.DocName))
					return
				}
				paginasMap["1"] = map[string]any{
					"index":   1,
					"ocr":     false,
					"content": htmlText,
				}
				contarPagina()
			}

			log.Printf("[Pipeline] Documento %s finalizado com %d páginas", f.DocName, len(paginasMap))
			l := classificarPaginas(paginasMap, false)
			if l.NaoLido() {
				naoLido(l, "", "", fmt.Sprintf("Concluído %d/%d: %s", int(processedDocsVal.Load())+1, totalFiles, f.DocName))
				return
			}
			atv.docLeitura(i, l, "")
			encerrar("", fmt.Sprintf("Concluído %d/%d: %s", int(processedDocsVal.Load())+1, totalFiles, f.DocName))
		})
	}

	wg.Wait()

	if errParseFirst != nil {
		return "", 0, 0, 0, errParseFirst
	}

	// Double check for context cancellation before generating/writing the file
	if err := ctx.Err(); err != nil {
		log.Printf("[Pipeline] Processamento cancelado antes de salvar o arquivo de saída")
		return "", 0, 0, 0, err
	}

	// 4. Format consolidated output into XML
	outputText, tokens := GenerateConsolidatedOutput(eventos, cfg.CleaningLevel)

	// 5. Save consolidated output to the same directory
	outputPath := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".txt"
	// WriteString não copia o texto para um []byte, como os.WriteFile exigiria.
	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", 0, 0, 0, err
	}
	_, err = out.WriteString(outputText)
	if errClose := out.Close(); err == nil {
		err = errClose
	}
	if err != nil {
		return "", 0, 0, 0, err
	}

	// Count total pages
	totalPages := 0
	for _, val := range eventos {
		evData := val.(map[string]any)
		documentosMap := evData["documentos"].(map[string]any)
		for _, dVal := range documentosMap {
			dData := dVal.(map[string]any)
			paginasMap := dData["paginas"].(map[string]any)
			if _, omitido := dData["omitido"].(string); omitido {
				// Igual ao resumo do <contexto>: conta as páginas da origem.
				totalPages += dData["total_paginas"].(int)
				continue
			}
			totalPages += len(paginasMap)
		}
	}

	return outputPath, totalFiles, totalPages, tokens, nil
}
