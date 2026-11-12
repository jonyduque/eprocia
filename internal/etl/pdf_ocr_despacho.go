package etl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"sync/atomic"

	"eprocia/internal/ocr"
)

// despachoOCR carrega o que ParsePages precisa para mandar páginas ao OCR e
// juntar os resultados: o mapa de páginas (sob mutex), o WaitGroup dos blocos em
// andamento, o aviso de progresso sem repetição e o contador de falhas. Quem o
// usa é só ParsePages, que o cria com novoDespachoOCR, entrega cada página por
// paginaPronta (não vai ao OCR) ou agendar (vai ao OCR) e fecha com esperar.
//
// A concorrência é limitada pelo semáforo de OCR que vem em ocrRunner (ver
// Pipeline.ProcessZip), então cada tarefa só bloqueia nele em vez de passar por
// um segundo pool aqui.
type despachoOCR struct {
	p              *PDFParser
	ctx            context.Context
	numPages       int
	ocrRunner      func(context.Context, string, int) (string, error)
	onPageComplete func(pageNum int)

	paginasMap map[string]any
	mapMu      sync.Mutex
	wg         sync.WaitGroup
	falhas     atomic.Int32

	// O OCR em lote avisa cada página assim que o texto dela sai do tesseract;
	// se o bloco cair depois e for refeito página a página, a página não pode
	// contar duas vezes no progresso.
	reportadas []atomic.Bool

	// Um bloco vai ao OCR assim que junta tamBloco páginas, enquanto o texto
	// nativo das seguintes ainda é lido. tamBloco é o bloco que sairia se todas
	// as páginas fossem ao OCR, então um documento todo escaneado ainda dá um
	// bloco por worker; o que sobrar no fim se divide pelos blocos restantes.
	pendentes   []ocrTask
	tamBloco    int
	despachados int
}

func (p *PDFParser) novoDespachoOCR(ctx context.Context, numPages int,
	ocrRunner func(context.Context, string, int) (string, error),
	onPageComplete func(pageNum int)) *despachoOCR {
	return &despachoOCR{
		p:              p,
		ctx:            ctx,
		numPages:       numPages,
		ocrRunner:      ocrRunner,
		onPageComplete: onPageComplete,
		paginasMap:     make(map[string]any),
		reportadas:     make([]atomic.Bool, numPages+1),
		tamBloco:       min((numPages+p.blocosOCR-1)/max(p.blocosOCR, 1), maxPaginasBlocoOCR),
	}
}

func (d *despachoOCR) concluir(pageNum int) {
	if d.onPageComplete == nil || pageNum < 1 || pageNum > d.numPages {
		return
	}
	if !d.reportadas[pageNum].Swap(true) {
		d.onPageComplete(pageNum)
	}
}

// paginaPronta grava uma página que não vai ao OCR e a dá como concluída.
func (d *despachoOCR) paginaPronta(pageNum int, pagina map[string]any) {
	d.mapMu.Lock()
	d.paginasMap[fmt.Sprintf("%d", pageNum)] = pagina
	d.mapMu.Unlock()
	d.concluir(pageNum)
}

// agendar põe a página na fila do OCR: em lote, junta até tamBloco páginas
// antes de despachar; sem lote, despacha uma a uma.
func (d *despachoOCR) agendar(task ocrTask) {
	if d.p.lote == nil {
		d.wg.Go(func() { d.runOCRBloco([]ocrTask{task}) })
		return
	}
	d.pendentes = append(d.pendentes, task)
	if len(d.pendentes) == d.tamBloco {
		d.despachar(d.pendentes)
		d.pendentes = nil
	}
}

func (d *despachoOCR) despachar(bloco []ocrTask) {
	d.despachados++
	d.wg.Go(func() { d.runOCRBloco(bloco) })
}

// aguardar espera os blocos já despachados, sem despachar o que sobrou.
func (d *despachoOCR) aguardar() { d.wg.Wait() }

// esperar despacha o que sobrou, espera todos os blocos e devolve o mapa de
// páginas.
func (d *despachoOCR) esperar() map[string]any {
	if len(d.pendentes) > 0 {
		restantes := max(d.p.blocosOCR-d.despachados, 1)
		k := min((len(d.pendentes)+restantes-1)/restantes, maxPaginasBlocoOCR)
		for bloco := range slices.Chunk(d.pendentes, k) {
			d.despachar(bloco)
		}
	}
	d.wg.Wait()
	return d.paginasMap
}

func (d *despachoOCR) registrarOCR(task ocrTask, ocrText string, err error) {
	var pageData map[string]any
	if err == nil {
		log.Printf("[PDFParser] OCR concluído para página %d (tamanho: %d)", task.pageNum, len(ocrText))
		pageData = map[string]any{
			"index":   task.pageNum,
			"ocr":     true,
			"content": ocrText,
		}
	} else {
		// O erro do OCR pode trazer o caminho de %TEMP% (com o nome de usuário)
		// e o stderr: a página e ocr_erro recebem só a frase fixa da classe, e
		// o erro inteiro fica no log.
		frase := fraseOCR(err)
		content := task.fallback
		if content == "" {
			content = "[ERRO de OCR: " + frase + "]"
		}
		log.Printf("[PDFParser] Falha no OCR para página %d: %v. Usando o texto de reserva (%d chars).", task.pageNum, err, len(task.fallback))
		d.falhas.Add(1)
		pageData = map[string]any{
			"index":    task.pageNum,
			"ocr":      false,
			"content":  content,
			"ocr_erro": frase,
		}
		if errors.Is(err, ocr.ErrPrazo) {
			pageData["ocr_prazo"] = true
		}
	}

	d.mapMu.Lock()
	d.paginasMap[fmt.Sprintf("%d", task.pageNum)] = pageData
	d.mapMu.Unlock()
	d.concluir(task.pageNum)
}

func (d *despachoOCR) runOCRTask(task ocrTask) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	log.Printf("[PDFParser] Iniciando OCR para página %d...", task.pageNum)
	caminho, err := d.p.caminhoParaOCR()
	var ocrText string
	if err == nil {
		ocrText, err = d.ocrRunner(d.ctx, caminho, task.pageNum)
	}
	d.registrarOCR(task, ocrText, err)
	return err
}

func (d *despachoOCR) runOCRBloco(tasks []ocrTask) {
	if len(tasks) == 1 {
		_ = d.runOCRTask(tasks[0])
		return
	}
	paginas := make([]int, len(tasks))
	for i, task := range tasks {
		paginas[i] = task.pageNum
	}
	log.Printf("[PDFParser] Iniciando OCR em bloco das páginas %v...", paginas)
	caminho, err := d.p.caminhoParaOCR()
	var textos []string
	if err == nil {
		textos, err = d.p.lote(d.ctx, caminho, paginas, d.concluir)
	}
	if err == nil && len(textos) != len(tasks) {
		err = fmt.Errorf("OCR em bloco devolveu %d textos para %d páginas", len(textos), len(tasks))
	}
	if err != nil && d.ctx.Err() != nil {
		log.Printf("[PDFParser] OCR em bloco das páginas %v interrompido por cancelamento: %v", paginas, d.ctx.Err())
		return
	}
	if err != nil {
		// Uma página ruim derruba o tesseract do bloco inteiro; sozinhas,
		// as outras ainda saem.
		log.Printf("[PDFParser] OCR em bloco das páginas %v falhou: %v. Refazendo página a página...", paginas, err)
		// Cada página que estoura o prazo custa o prazo inteiro: depois de
		// maxPrazosSeguidos seguidas, as outras do bloco nem rodam.
		seguidas := 0
		for _, task := range tasks {
			if seguidas >= maxPrazosSeguidos {
				d.registrarOCR(task, "", fmt.Errorf("%w: página não tentada depois de %d páginas seguidas sem resposta", ocr.ErrPrazo, seguidas))
				continue
			}
			if errors.Is(d.runOCRTask(task), ocr.ErrPrazo) {
				seguidas++
			} else {
				seguidas = 0
			}
		}
		return
	}
	for i, task := range tasks {
		d.registrarOCR(task, textos[i], nil)
	}
}
