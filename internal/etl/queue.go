package etl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"eprocia/internal/config"
	"eprocia/internal/historico"
	"eprocia/internal/models"
	"eprocia/internal/registro"
	"eprocia/internal/saida"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// QueueItem represents an item in the execution queue.
type QueueItem struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	Status          string  `json:"status"` // "pending", "processing", "paused", "cancelled", "interrupted", "success", "error"
	Progress        float64 `json:"progress"`
	Message         string  `json:"message"`
	OutputPath      string  `json:"output_path"`
	DocsCount       int     `json:"docs_count"`
	PagesCount      int     `json:"pages_count"`
	EstimatedTokens int     `json:"estimated_tokens"`
	ProcessedDocs   int     `json:"processed_docs"`
	TotalDocs       int     `json:"total_docs"`
	ProcessedPages  int     `json:"processed_pages"`
	TotalPages      int     `json:"total_pages"`
	// Kind é "zip" ou "pdf". Paths são os PDFs do item, na ordem por evento
	// (vazio para ZIP); Path do item de PDF é Paths[0].
	Kind  string   `json:"kind"`
	Paths []string `json:"paths,omitempty"`
	// UnreadDocs conta os documentos que ficaram sem texto aproveitável; o item
	// continua "success".
	UnreadDocs int `json:"unread_docs"`
	// Relatorio é o relatório da última execução com sucesso; nil enquanto o
	// item aguarda ou processa, e depois de erro ou cancelamento.
	Relatorio *models.RelatorioZip `json:"relatorio,omitempty"`
	Error     string               `json:"error,omitempty"`
	Timestamp int64                `json:"timestamp"`
	// FinishedAt é a hora (ms) em que a execução terminou, em qualquer fim
	// (sucesso, erro, cancelamento, interrupção); 0 enquanto aguarda ou processa.
	FinishedAt int64 `json:"finished_at"`
	// OutputMissing e SourceMissing marcam, num item restaurado do histórico,
	// o TXT e a origem que não estão mais no disco.
	OutputMissing bool `json:"output_missing"`
	SourceMissing bool `json:"source_missing"`
	// Restaurado marca o item que veio do histórico da sessão anterior.
	Restaurado bool `json:"restaurado"`
	// OcrMode e CleaningLevel são os da última execução do item (ou os que a
	// próxima vai usar, num item reprocessado que ainda aguarda).
	OcrMode       string `json:"ocr_mode,omitempty"`
	CleaningLevel string `json:"cleaning_level,omitempty"`
	// ForcarOCR é o Forçar OCR efetivo da última execução (false com OCR
	// desligado), ou o que a próxima vai usar se o Reprocessar o informou.
	ForcarOCR bool               `json:"forcar_ocr,omitempty"`
	cancel    context.CancelFunc `json:"-"`

	// ajuste troca OCR, higienização e Forçar OCR da config global só para
	// este item.
	ajuste *ajusteConfig
	// origem é por onde o ZIP entrou: "monitorada", "arrastada" ou "manual".
	origem string
	// atividade guarda o estado por documento da execução atual ou da última.
	atividade *Atividade
	// saidaAnterior é o TXT da última execução com sucesso, guardado ao
	// reabrir o item (Reprocessar, ReprocessarMonitorado; Restaurar no
	// histórico), para a próxima gravar por cima dele. Zerada só no sucesso.
	saidaAnterior string
	// reaberto marca o item que ReprocessarMonitorado devolveu à fila: sem
	// ajuste, mas a execução conta como reprocesso na telemetria.
	reaberto bool

	ultimoEnvio  time.Time   // último queue:item deste item
	envioPend    *time.Timer // queue:item agendado pelo limitador
	geracaoEnvio uint64      // identifica o agendamento vigente
}

type ajusteConfig struct {
	ocrMode, cleaningLevel string
	// forcarOCR nil usa o Forçar OCR da config global.
	forcarOCR *bool
}

// Queue handles file processing sequentially in a thread-safe manner.
type Queue struct {
	mu         sync.RWMutex
	items      []*QueueItem
	pipeline   *Pipeline
	wailsCtx   context.Context
	isPaused   bool
	isWorking  bool
	cfg        *models.Config
	baseCtx    context.Context
	baseCancel context.CancelFunc
	encerrada  bool
	wg         sync.WaitGroup // processItem em curso

	// emitMu serializa cópia+envio, para que um evento copiado antes de outro
	// nunca chegue ao frontend depois dele. Nunca é tomado com q.mu travado.
	emitMu          sync.Mutex
	statusEmitido   bool
	ultimoPausado   bool
	ultimoTrabalhou bool

	// emitir substitui runtime.EventsEmit nos testes.
	emitir func(ctx context.Context, nome string, dados ...any)

	// processar substitui pipeline.ProcessarEntrada nos testes.
	processar func(ctx context.Context, e Entrada, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error)

	// carregarConfig substitui config.Load nos testes.
	carregarConfig func() (*models.Config, error)

	// onFinished recebe uma cópia do item que terminou com sucesso ou erro
	// (cancelamentos não avisam), para notificar com a janela oculta.
	onFinished func(item *QueueItem)

	// aoTerminarZip recebe cada execução terminada (sucesso, erro ou
	// cancelamento), para a telemetria; sempre chamado fora de q.mu.
	aoTerminarZip func(models.ExecucaoZip)

	// retida impede o início de itens novos sem pausar a fila (a atualização
	// espera o item em curso antes de trocar o executável).
	retida bool

	// aoFicarOciosa é avisado, numa goroutine e fora de q.mu, quando um
	// caminho termina sem iniciar item e a fila está ociosa.
	aoFicarOciosa func()

	// existe diz se um caminho de origem está no disco (nil = os.Stat sem
	// erro); trocado nos testes.
	existe func(caminho string) bool

	// gravador recebe a lista gravada a cada broadcast (nil = histórico
	// desligado). Lido sob q.mu; chamado só sob q.emitMu.
	gravador GravadorHistorico
	// sessaoComEntrada liga na primeira entrada nova da sessão (AddFiles,
	// Reprocessar, ReprocessarMonitorado) e não volta a false: com ele, a
	// lista gravada é a dos itens sem Restaurado; sem ele, a dos restaurados.
	sessaoComEntrada bool
}

// prazoShutdown limita quanto o encerramento do app espera o item em curso.
const prazoShutdown = 5 * time.Second

// NewQueue creates a new Queue instance.
func NewQueue(pipeline *Pipeline) *Queue {
	baseCtx, baseCancel := context.WithCancel(context.Background())
	return &Queue{
		items:      make([]*QueueItem, 0),
		pipeline:   pipeline,
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
		emitir:     runtime.EventsEmit,
	}
}

// Shutdown cancels the base context, stopping all in-flight and future
// processing, and waits (up to prazoShutdown) for the current item to
// finish. Call on app shutdown so OCR/PDF subprocesses don't outlive
// the Wails runtime and their temp files get removed.
func (q *Queue) Shutdown() {
	q.shutdownComPrazo(prazoShutdown)
}

// shutdownComPrazo devolve false se o item em curso não terminou no prazo.
func (q *Queue) shutdownComPrazo(prazo time.Duration) bool {
	q.mu.Lock()
	q.encerrada = true
	q.mu.Unlock()
	q.baseCancel()

	terminou := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(terminou)
	}()
	terminouNoPrazo := true
	select {
	case <-terminou:
	case <-time.After(prazo):
		slog.Warn("encerramento: o item em processamento não terminou no prazo; saindo mesmo assim", "prazo_ms", prazo.Milliseconds())
		terminouNoPrazo = false
	}
	// A escrita final fecha o gravador: o broadcast tardio de um processItem
	// que passou do prazo não regrava por cima.
	if err := q.FecharHistorico(); err != nil {
		slog.Warn("encerramento: não foi possível gravar o histórico", "erro", err)
	}
	return terminouNoPrazo
}

// SetContext sets the Wails context for event emission.
func (q *Queue) SetContext(ctx context.Context) {
	q.mu.Lock()
	q.wailsCtx = ctx
	q.mu.Unlock()
}

// SetConfig sets/updates the configuration settings for the queue.
func (q *Queue) SetConfig(cfg *models.Config) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cfg = cfg
}

// SetOnFinished registra quem é avisado quando um item termina com sucesso ou erro.
func (q *Queue) SetOnFinished(fn func(item *QueueItem)) {
	q.mu.Lock()
	q.onFinished = fn
	q.mu.Unlock()
}

// avisarFimLocked exige q.mu travado; devolve a chamada a fazer depois de destravar.
func (q *Queue) avisarFimLocked(item *QueueItem) func() {
	fn := q.onFinished
	if fn == nil || (item.Status != "success" && item.Status != "error") {
		return func() {}
	}
	copia := copiarItem(item)
	return func() { fn(copia) }
}

// SetAoTerminarZip registra quem recebe, uma vez por execução, o resumo de
// cada ZIP que terminou com sucesso, erro ou cancelamento (a telemetria). A
// chamada é feita fora de q.mu.
func (q *Queue) SetAoTerminarZip(fn func(models.ExecucaoZip)) {
	q.mu.Lock()
	q.aoTerminarZip = fn
	q.mu.Unlock()
}

// execucaoLocked exige q.mu travado; devolve a chamada a aoTerminarZip a
// fazer depois de destravar. Um pânico no callback não chega à fila.
func (q *Queue) execucaoLocked(item *QueueItem, reprocesso bool, cfg *models.Config, bytesZip int64, classeErro, etapaErro string) func() {
	fn := q.aoTerminarZip
	if fn == nil {
		return func() {}
	}
	e := models.ExecucaoZip{
		ItemID:     item.ID,
		Reprocesso: reprocesso,
		Origem:     item.origem,
		BytesZip:   bytesZip,
		Status:     item.Status,
		ClasseErro: classeErro,
		EtapaErro:  etapaErro,
		Relatorio:  item.Relatorio,
	}
	if cfg != nil {
		e.Config = *cfg
		e.Config.ForcarOCR = forcarOCREfetivo(cfg)
	}
	return func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("pânico no aviso de fim de ZIP", "item_id", e.ItemID, "erro", fmt.Sprint(r))
			}
		}()
		fn(e)
	}
}

// AddFiles adds ZIP and PDF files to the queue and returns how many actually
// entered (duplicates of pending/processing items are dropped). Origem manual.
func (q *Queue) AddFiles(paths []string) int {
	return q.AddFilesComOrigem(paths, "manual")
}

// AddFilesComOrigem é AddFiles com a origem do ZIP ("monitorada",
// "arrastada" ou "manual"), que vai para o log e a telemetria. Só .zip e
// .pdf entram (sem diferença de caixa): cada ZIP vira um item, e todos os
// PDFs da chamada, sem repetidos e sem os que já estão num item pendente ou
// em processamento, viram um item só, que entra depois dos ZIPs. Devolve o
// número de arquivos que entraram.
func (q *Queue) AddFilesComOrigem(paths []string, origem string) int {
	q.mu.Lock()
	hasPending := false
	adicionados := 0
	var pdfs []string
	for _, path := range paths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".zip":
		case ".pdf":
			if slices.ContainsFunc(pdfs, func(p string) bool { return strings.EqualFold(p, path) }) {
				continue
			}
			if q.pdfNaFilaLocked(path) {
				slog.Info("PDF duplicado ignorado", "arquivo", filepath.Base(path))
				continue
			}
			pdfs = append(pdfs, path)
			continue
		default:
			slog.Info("arquivo recusado: extensão não aceita", "arquivo", filepath.Base(path))
			continue
		}
		// Avoid duplicate files currently in queue (not processed yet or currently processing)
		duplicated := false
		for _, item := range q.items {
			if item.Path == path && (item.Status == "pending" || item.Status == "processing") {
				duplicated = true
				break
			}
		}
		if duplicated {
			slog.Info("ZIP duplicado ignorado", "zip", filepath.Base(path))
			continue
		}

		name := filepath.Base(path)
		item := &QueueItem{
			ID:        uuid.NewString(),
			Name:      name,
			Path:      path,
			Kind:      "zip",
			Status:    "pending",
			Progress:  0.0,
			Message:   "Na fila...",
			Timestamp: time.Now().UnixMilli(),
			origem:    origem,
		}
		// Prepend to show newest on top
		q.items = append([]*QueueItem{item}, q.items...)
		q.sessaoComEntrada = true
		hasPending = true
		adicionados++
		slog.Info("ZIP adicionado à fila", "item_id", item.ID, "zip", item.Name, "origem", origem)
	}
	if len(pdfs) > 0 {
		ordem := ordenarPorEvento(pdfs)
		item := &QueueItem{
			ID:        uuid.NewString(),
			Name:      nomeItemPDF(radicalDe(saida.NomeLote(ordem)), len(ordem)),
			Path:      ordem[0],
			Kind:      "pdf",
			Paths:     ordem,
			Status:    "pending",
			Progress:  0.0,
			Message:   "Na fila...",
			Timestamp: time.Now().UnixMilli(),
			origem:    origem,
		}
		q.items = append([]*QueueItem{item}, q.items...)
		q.sessaoComEntrada = true
		hasPending = true
		adicionados += len(ordem)
		slog.Info("PDF adicionado à fila", "item_id", item.ID, "zip", rotuloLog(item), "pdfs", len(ordem), "origem", origem)
	}
	q.mu.Unlock()

	q.broadcast()

	if hasPending {
		q.startProcessing()
	}
	return adicionados
}

// pdfNaFilaLocked diz se path (sem diferença de caixa) já está em Paths de um
// item pendente ou em processamento. Exige q.mu travado.
func (q *Queue) pdfNaFilaLocked(path string) bool {
	for _, item := range q.items {
		if item.Status != "pending" && item.Status != "processing" {
			continue
		}
		if slices.ContainsFunc(item.Paths, func(p string) bool { return strings.EqualFold(p, path) }) {
			return true
		}
	}
	return false
}

// nomeItemPDF é o Name de um item de PDF: base, mais " (N PDFs)" quando n > 1.
func nomeItemPDF(base string, n int) string {
	if n > 1 {
		return fmt.Sprintf("%s (%d PDFs)", base, n)
	}
	return base
}

// radicalDe devolve o nome do arquivo sem pasta e sem a última extensão.
func radicalDe(caminho string) string {
	nome := filepath.Base(caminho)
	return strings.TrimSuffix(nome, filepath.Ext(nome))
}

// rotuloLog é o valor do atributo zip nas linhas da fila: o nome do PDF num
// item de um PDF só, senão o Name do item.
func rotuloLog(item *QueueItem) string {
	if item.Kind == "pdf" && len(item.Paths) == 1 {
		return filepath.Base(item.Path)
	}
	return item.Name
}

// GetItems returns a copy of all queue items.
func (q *Queue) GetItems() []*QueueItem {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.copyItemsLocked()
}

// copyItemsLocked exige q.mu já travado (leitura ou escrita).
func (q *Queue) copyItemsLocked() []*QueueItem {
	copied := make([]*QueueItem, len(q.items))
	for i, item := range q.items {
		copied[i] = copiarItem(item)
	}
	return copied
}

func copiarItem(item *QueueItem) *QueueItem {
	return &QueueItem{
		ID:              item.ID,
		Name:            item.Name,
		Path:            item.Path,
		Kind:            item.Kind,
		Paths:           slices.Clone(item.Paths),
		Status:          item.Status,
		Progress:        item.Progress,
		Message:         item.Message,
		OutputPath:      item.OutputPath,
		DocsCount:       item.DocsCount,
		PagesCount:      item.PagesCount,
		EstimatedTokens: item.EstimatedTokens,
		ProcessedDocs:   item.ProcessedDocs,
		TotalDocs:       item.TotalDocs,
		ProcessedPages:  item.ProcessedPages,
		TotalPages:      item.TotalPages,
		UnreadDocs:      item.UnreadDocs,
		// O relatório não muda depois de pronto: basta o ponteiro.
		Relatorio:     item.Relatorio,
		Error:         item.Error,
		Timestamp:     item.Timestamp,
		OcrMode:       item.OcrMode,
		CleaningLevel: item.CleaningLevel,
		ForcarOCR:     item.ForcarOCR,
		FinishedAt:    item.FinishedAt,
		OutputMissing: item.OutputMissing,
		SourceMissing: item.SourceMissing,
		Restaurado:    item.Restaurado,
	}
}

// Reprocessar devolve à fila um item que já terminou, com OCR e higienização
// próprios e, se forcarOCR não for nil, com Forçar OCR próprio (nil usa o da
// config global; com ocrMode "none" ele não tem efeito). A config global não
// muda; o .txt anterior é sobrescrito. A origem (o Path do ZIP, ou todos os
// Paths do PDF) precisa existir: senão a fila não muda.
func (q *Queue) Reprocessar(id, ocrMode, cleaningLevel string, forcarOCR *bool) error {
	if !config.ModoOCRValido(ocrMode) {
		return fmt.Errorf("modo de OCR inválido: %q", ocrMode)
	}
	if !config.LimpezaValida(cleaningLevel) {
		return fmt.Errorf("nível de higienização inválido: %q", cleaningLevel)
	}
	q.mu.Lock()
	idx, err := q.reprocessavelLocked(id)
	if err != nil {
		q.mu.Unlock()
		return err
	}
	conferir := []string{q.items[idx].Path}
	if q.items[idx].Kind == "pdf" {
		conferir = slices.Clone(q.items[idx].Paths)
	}
	existe := q.existe
	q.mu.Unlock()

	// O disco é lido fora da trava; o item é procurado de novo depois.
	if existe == nil {
		existe = func(c string) bool {
			_, err := os.Stat(c)
			return err == nil
		}
	}
	for _, p := range conferir {
		if !existe(p) {
			return fmt.Errorf("arquivo de origem não encontrado: %s", filepath.Base(p))
		}
	}

	q.mu.Lock()
	idx, err = q.reprocessavelLocked(id)
	if err != nil {
		q.mu.Unlock()
		return err
	}
	item := q.items[idx]
	reabrirLocked(item)
	q.entradaLocked(item)
	// item.ForcarOCR mostra já o efetivo da próxima execução; com nil vale o da
	// global (sem q.cfg, a config só é lida no processItem, que o corrige).
	var forcar *bool
	if forcarOCR != nil {
		v := *forcarOCR
		forcar = &v
		item.ForcarOCR = v && ocrMode != "none"
	} else {
		item.ForcarOCR = q.cfg != nil && q.cfg.ForcarOCR && ocrMode != "none"
	}
	item.ajuste = &ajusteConfig{ocrMode: ocrMode, cleaningLevel: cleaningLevel, forcarOCR: forcar}
	item.OcrMode, item.CleaningLevel = ocrMode, cleaningLevel
	// Vai para o topo, como um arquivo novo: a fila processa de baixo para cima.
	q.items = slices.Delete(q.items, idx, idx+1)
	q.items = append([]*QueueItem{item}, q.items...)
	attrs := []any{"item_id", item.ID, "zip", rotuloLog(item), "ocr_mode", ocrMode, "cleaning_level", cleaningLevel}
	q.mu.Unlock()

	if forcar != nil {
		attrs = append(attrs, "forcar_ocr", *forcar)
	}
	slog.Info("reprocessar", attrs...)
	q.broadcast()
	q.startProcessing()
	return nil
}

// reprocessavelLocked acha o item id e confere que ele já terminou. Exige
// q.mu travado.
func (q *Queue) reprocessavelLocked(id string) (int, error) {
	idx := slices.IndexFunc(q.items, func(it *QueueItem) bool { return it.ID == id })
	if idx < 0 {
		return -1, fmt.Errorf("item não encontrado na fila")
	}
	if st := q.items[idx].Status; st == "pending" || st == "processing" {
		return -1, fmt.Errorf("o item ainda está na fila ou em processamento")
	}
	return idx, nil
}

// reabrirLocked põe um item terminado de volta em "pending", com os números
// da execução anterior zerados. O OutputPath, se houver, passa a
// saidaAnterior, para a próxima execução gravar por cima dele; um item
// cancelado ou com erro (OutputPath vazio) mantém a saidaAnterior que já
// tinha. Exige q.mu travado.
func reabrirLocked(item *QueueItem) {
	pararEnvioPendente(item)
	if item.OutputPath != "" {
		item.saidaAnterior = item.OutputPath
	}
	item.Status = "pending"
	item.Message = "Na fila..."
	item.Progress = 0
	item.Error = ""
	item.OutputPath = ""
	item.DocsCount, item.PagesCount, item.EstimatedTokens = 0, 0, 0
	item.ProcessedDocs, item.TotalDocs, item.ProcessedPages, item.TotalPages = 0, 0, 0, 0
	item.UnreadDocs = 0
	item.Relatorio = nil
	item.FinishedAt = 0
	item.OutputMissing, item.SourceMissing = false, false
	item.Timestamp = time.Now().UnixMilli()
}

// ReprocessarMonitorado trata um PDF da pasta monitorada que mudou depois de
// convertido (q1): se a fila tem um item "success" de um PDF só com esse
// caminho (sem diferença de caixa), o item volta a "pending" na mesma
// posição, com a config global, e a próxima execução grava por cima do TXT
// dele. Devolve false, sem mexer na fila, em qualquer outro caso (o chamador
// segue para AddFiles).
func (q *Queue) ReprocessarMonitorado(path string) bool {
	q.mu.Lock()
	idx := slices.IndexFunc(q.items, func(it *QueueItem) bool {
		return it.Status == "success" && it.Kind == "pdf" && len(it.Paths) == 1 && strings.EqualFold(it.Paths[0], path)
	})
	if idx < 0 {
		q.mu.Unlock()
		return false
	}
	item := q.items[idx]
	reabrirLocked(item)
	// Sem ajuste: OcrMode, CleaningLevel e ForcarOCR são corrigidos pela
	// config global no processItem.
	item.ajuste = nil
	// Para a telemetria, ainda é um reprocessamento (q1).
	item.reaberto = true
	q.entradaLocked(item)
	attrs := []any{"item_id", item.ID, "zip", rotuloLog(item)}
	q.mu.Unlock()

	slog.Info("reprocessar PDF monitorado", attrs...)
	q.broadcast()
	q.startProcessing()
	return true
}

// GetAtividade devolve o estado por documento da execução atual ou da última
// do item, ou nil se o item não existe ou ainda não rodou.
func (q *Queue) GetAtividade(id string) *InstantaneoAtividade {
	q.mu.RLock()
	var atv *Atividade
	for _, it := range q.items {
		if it.ID == id {
			atv = it.atividade
			break
		}
	}
	q.mu.RUnlock()
	return atv.Instantaneo()
}

// Pause pauses processing of the queue.
func (q *Queue) Pause() {
	q.mu.Lock()
	q.isPaused = true
	q.mu.Unlock()
	slog.Info("fila pausada")
	q.broadcast()
}

// Resume resumes processing of the queue.
func (q *Queue) Resume() {
	q.mu.Lock()
	q.isPaused = false
	q.mu.Unlock()
	slog.Info("fila retomada")
	q.broadcast()
	q.startProcessing()
}

// IsPaused returns true if the queue is paused.
func (q *Queue) IsPaused() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.isPaused
}

// Reter impede que a fila inicie itens novos, sem pausá-la: os arquivos
// continuam entrando como "pending", o item em curso termina normalmente e
// nem Resume nem Reprocessar iniciam nada até Soltar. Não emite queue:status.
func (q *Queue) Reter() {
	q.mu.Lock()
	q.retida = true
	q.mu.Unlock()
	slog.Info("fila retida")
}

// Soltar desfaz Reter e inicia o item pendente mais antigo, se houver.
func (q *Queue) Soltar() {
	q.mu.Lock()
	q.retida = false
	q.mu.Unlock()
	slog.Info("fila solta")
	q.startProcessing()
}

// Ociosa diz se a fila não tem item "pending" nem "processing" (nem um
// processItem em curso, caso de um item em processamento já tirado da lista).
func (q *Queue) Ociosa() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.ociosaLocked()
}

// ociosaLocked exige q.mu travado (leitura ou escrita).
func (q *Queue) ociosaLocked() bool {
	if q.isWorking {
		return false
	}
	return !slices.ContainsFunc(q.items, func(it *QueueItem) bool {
		return it.Status == "pending" || it.Status == "processing"
	})
}

// SetAoFicarOciosa registra quem é avisado quando um caminho da fila termina
// sem iniciar item e ela está ociosa (fim do último item, Clear, CancelItem,
// RemoveItem...). O aviso roda numa goroutine, fora de q.mu, e não depende do
// contexto do Wails; pode chegar mais de uma vez para a mesma ociosidade.
func (q *Queue) SetAoFicarOciosa(fn func()) {
	q.mu.Lock()
	q.aoFicarOciosa = fn
	q.mu.Unlock()
}

// avisoSeOciosaLocked exige q.mu travado; devolve a chamada a fazer depois de
// destravar, ou nil quando não há aviso (fila não ociosa ou sem callback).
func (q *Queue) avisoSeOciosaLocked() func() {
	fn := q.aoFicarOciosa
	if fn == nil || !q.ociosaLocked() {
		return nil
	}
	return func() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("pânico no aviso de fila ociosa", "erro", fmt.Sprint(r))
				}
			}()
			fn()
		}()
	}
}

// CancelItem cancels processing of an item.
func (q *Queue) CancelItem(id string) {
	q.mu.Lock()
	var target *QueueItem
	for _, item := range q.items {
		if item.ID == id {
			target = item
			break
		}
	}

	if target != nil {
		slog.Info("cancelamento pedido", "item_id", target.ID, "zip", rotuloLog(target), "status", target.Status)
		if target.Status == "processing" && target.cancel != nil {
			target.cancel()
		} else if target.Status == "pending" {
			target.Status = "cancelled"
			target.Message = "Cancelado"
			target.FinishedAt = time.Now().UnixMilli()
		}
	}
	aviso := q.avisoSeOciosaLocked()
	q.mu.Unlock()
	q.broadcast()
	if aviso != nil {
		aviso()
	}
}

// RemoveItem cancels (if running) and removes an item from the list.
func (q *Queue) RemoveItem(id string) {
	q.mu.Lock()
	idx := -1
	for i, item := range q.items {
		if item.ID == id {
			idx = i
			if item.Status == "processing" && item.cancel != nil {
				item.cancel()
			}
			break
		}
	}

	if idx != -1 {
		q.items = append(q.items[:idx], q.items[idx+1:]...)
	}
	q.mu.Unlock()
	q.broadcast()
	q.startProcessing()
}

// Clear cancels all running/pending tasks and clears the queue list.
func (q *Queue) Clear() {
	q.mu.Lock()
	for _, item := range q.items {
		if item.Status == "processing" && item.cancel != nil {
			item.cancel()
		}
	}
	q.items = make([]*QueueItem, 0)
	aviso := q.avisoSeOciosaLocked()
	q.mu.Unlock()
	q.broadcast()
	if aviso != nil {
		aviso()
	}
}

// LimparConcluidos tira da lista os itens com sucesso, erro, cancelados ou
// interrompidos; o item em processamento e os que aguardam continuam como estão.
func (q *Queue) LimparConcluidos() {
	q.mu.Lock()
	restantes := q.items[:0:0]
	for _, item := range q.items {
		switch item.Status {
		case "success", "error", "cancelled", "interrupted":
		default:
			restantes = append(restantes, item)
		}
	}
	q.items = restantes
	q.mu.Unlock()
	q.broadcast()
}

func (q *Queue) startProcessing() {
	q.mu.Lock()
	slog.Debug("startProcessing", "pausada", q.isPaused, "retida", q.retida, "em_execucao", q.isWorking, "itens", len(q.items))
	if q.encerrada {
		q.mu.Unlock()
		return
	}
	if q.isPaused || q.retida || q.isWorking {
		aviso := q.avisoSeOciosaLocked()
		q.mu.Unlock()
		if aviso != nil {
			aviso()
		}
		return
	}

	// Find the next pending item, starting from the oldest (bottom of stack)
	var nextItem *QueueItem
	for _, v := range slices.Backward(q.items) {
		if v.Status == "pending" {
			nextItem = v
			break
		}
	}

	if nextItem == nil {
		aviso := q.avisoSeOciosaLocked()
		q.mu.Unlock()
		if aviso != nil {
			aviso()
		}
		return
	}

	q.isWorking = true
	// Go sob q.mu com encerrada == false: Shutdown marca encerrada antes de
	// esperar, então nenhum Add acontece com o contador em zero durante o Wait.
	// processItem só pega q.mu depois do Unlock abaixo.
	q.wg.Go(func() { q.processItem(nextItem) })
	q.mu.Unlock()
}

func (q *Queue) processItem(item *QueueItem) {
	// Para a telemetria: a config efetiva, o tamanho do ZIP e se é reprocesso.
	var (
		cfgExec    *models.Config
		bytesZip   int64
		reprocesso bool
	)
	defer func() {
		if r := recover(); r != nil {
			slog.Default().With("item_id", item.ID).Error("pânico no processador da fila", "zip", rotuloLog(item),
				"erro", fmt.Sprint(r), "pilha", string(debug.Stack()))
			q.mu.Lock()
			item.cancel = nil
			pararEnvioPendente(item)
			item.Status = "error"
			item.Message = fmt.Sprintf("Erro crítico (panic): %v", r)
			item.Error = fmt.Sprintf("Panic: %v", r)
			item.FinishedAt = time.Now().UnixMilli()
			q.isWorking = false
			avisar := q.avisarFimLocked(item)
			telemetria := q.execucaoLocked(item, reprocesso, cfgExec, bytesZip, "panico", "")
			q.mu.Unlock()
			q.broadcast()
			avisar()
			telemetria()
			q.startProcessing()
		}
	}()

	q.mu.Lock()
	ctx, cancel := context.WithCancel(q.baseCtx)
	defer cancel()
	// Toda linha do pipeline deste item leva o item_id (o mesmo da telemetria).
	ctx = registro.ComLogger(ctx, slog.Default().With("item_id", item.ID))
	item.cancel = cancel
	item.Status = "processing"
	item.Message = "Iniciando..."
	item.Progress = 0
	item.ProcessedDocs, item.TotalDocs, item.ProcessedPages, item.TotalPages = 0, 0, 0, 0
	// Erro e cancelamento deixam o relatório nil; só o sucesso o preenche.
	item.Relatorio = nil
	atv := NovaAtividade(item.ID)
	item.atividade = atv
	cfg := q.cfg
	ajuste := item.ajuste
	reprocesso = ajuste != nil || item.reaberto
	origem := item.origem
	// A Entrada é montada sob q.mu; saidaAnterior só é zerada no sucesso.
	entrada := Entrada{Kind: item.Kind, Caminhos: slices.Clone(item.Paths), SaidaAnterior: item.saidaAnterior}
	if entrada.Kind == "" {
		entrada.Kind = "zip"
	}
	if len(entrada.Caminhos) == 0 {
		entrada.Caminhos = []string{item.Path}
	}
	rotulo := rotuloLog(item)
	q.mu.Unlock()

	q.broadcast()

	lg := registro.Logger(ctx)
	for _, c := range entrada.Caminhos {
		if info, err := os.Stat(c); err == nil {
			bytesZip += info.Size()
		}
	}
	lg.Info("item na vez", "zip", rotulo, "origem", origem, "reprocesso", reprocesso, "bytes_zip", bytesZip)
	onProgress := func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int) {
		lg.Debug("progresso", "zip", rotulo, "progresso", progress, "mensagem", message,
			"docs", processedDocs, "total_docs", totalDocs, "paginas", processedPages, "total_paginas", totalPages)
		q.mu.Lock()
		if item.Status == "processing" {
			// Os documentos rodam em paralelo e o pipeline lê os contadores sem
			// trava, então um aviso pode chegar depois de outro mais novo: os
			// números só avançam.
			item.Progress = max(item.Progress, progress)
			item.Message = message
			item.ProcessedDocs = max(item.ProcessedDocs, processedDocs)
			item.TotalDocs = totalDocs
			item.ProcessedPages = max(item.ProcessedPages, processedPages)
			item.TotalPages = totalPages
		}
		q.mu.Unlock()
		q.emitirProgresso(item)
	}

	if cfg == nil {
		carregar := q.carregarConfig
		if carregar == nil {
			carregar = config.Load
		}
		var err error
		cfg, err = carregar()
		if err != nil {
			lg.Error("falha ao carregar a configuração do item", "zip", rotulo, "erro", err)
			q.mu.Lock()
			item.Status = "error"
			item.Message = "Erro ao carregar configurações"
			item.Error = err.Error()
			item.FinishedAt = time.Now().UnixMilli()
			q.isWorking = false
			avisar := q.avisarFimLocked(item)
			telemetria := q.execucaoLocked(item, reprocesso, nil, bytesZip, "config", "")
			q.mu.Unlock()
			q.broadcast()
			avisar()
			telemetria()
			q.startProcessing()
			return
		}
	}

	if ajuste != nil {
		copia := *cfg
		copia.OcrMode, copia.CleaningLevel = ajuste.ocrMode, ajuste.cleaningLevel
		if ajuste.forcarOCR != nil {
			copia.ForcarOCR = *ajuste.forcarOCR
		}
		cfg = &copia
	}
	cfgExec = cfg
	q.mu.Lock()
	item.OcrMode, item.CleaningLevel = cfg.OcrMode, cfg.CleaningLevel
	item.ForcarOCR = forcarOCREfetivo(cfg)
	q.mu.Unlock()

	processar := q.processar
	if processar == nil {
		processar = func(ctx context.Context, e Entrada, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			return q.pipeline.ProcessarEntrada(ctx, e, cfg, onProgress, atv)
		}
	}
	pararAtividade := q.enviarAtividade(atv)
	outputPath, docsCount, pagesCount, tokens, err := processar(ctx, entrada, cfg, onProgress)
	pararAtividade()

	q.mu.Lock()
	item.cancel = nil
	q.isWorking = false
	pararEnvioPendente(item)
	item.FinishedAt = time.Now().UnixMilli()

	classeErro, etapaErro := "", ""
	if err != nil {
		if ctx.Err() == context.Canceled {
			// Com a fila encerrada (Shutdown), o item não foi cancelado pelo
			// usuário: fica interrompido, para o histórico oferecer de novo.
			if q.encerrada {
				item.Status = "interrupted"
				item.Message = "Interrompido"
			} else {
				item.Status = "cancelled"
				item.Message = "Cancelado"
			}
			item.Progress = 0.0
		} else {
			item.Status = "error"
			// No item de PDF, o erro cru (o *fs.PathError da gravação traz a
			// pasta, com o nome do usuário) não chega à interface: vai o texto
			// fixo de textoErroSaida, e o erro completo fica no log ("zip com
			// erro", em processarEntrada). O ZIP segue com o erro cru.
			texto := err.Error()
			if entrada.Kind == "pdf" {
				texto = textoErroSaida(err)
			}
			item.Message = texto
			item.Error = texto
			classeErro = string(classificarErro(err))
			if ee, ok := errors.AsType[ErroEtapa](err); ok {
				etapaErro = ee.Etapa
			}
		}
	} else {
		item.Status = "success"
		item.Progress = 1.0
		item.Message = "Concluído"
		item.OutputPath = outputPath
		item.DocsCount = docsCount
		item.PagesCount = pagesCount
		item.EstimatedTokens = tokens
		item.ProcessedDocs = docsCount
		item.TotalDocs = docsCount
		item.ProcessedPages = pagesCount
		item.TotalPages = pagesCount
		item.UnreadDocs = atv.NaoLidos()
		item.Relatorio = atv.Relatorio()
		item.OutputMissing, item.SourceMissing = false, false
		// O novo OutputPath assume o papel da saída anterior.
		item.saidaAnterior = ""
		if entrada.Kind == "pdf" {
			// O _N só se decide na gravação: o nome segue o TXT gravado.
			item.Name = nomeItemPDF(radicalDe(outputPath), len(entrada.Caminhos))
		}
	}
	avisar := q.avisarFimLocked(item)
	telemetria := q.execucaoLocked(item, reprocesso, cfg, bytesZip, classeErro, etapaErro)
	q.mu.Unlock()

	q.broadcast()
	avisar()
	telemetria()
	q.startProcessing()
}

// broadcast envia a lista inteira (mudanças de estado: itens novos,
// removidos, início e fim de processamento) e o status, se mudou.
func (q *Queue) broadcast() {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()

	q.mu.RLock()
	ctx := q.wailsCtx
	gravador := q.gravador
	var gravada []historico.Item
	if gravador != nil {
		gravada = q.listaGravadaLocked()
	}
	if ctx == nil {
		q.mu.RUnlock()
		// A gravação não depende do contexto do Wails.
		if gravador != nil {
			gravador.Agendar(gravada)
		}
		return
	}
	items := q.copyItemsLocked()
	isPaused, isWorking := q.isPaused, q.isWorking
	q.mu.RUnlock()

	if gravador != nil {
		gravador.Agendar(gravada)
	}
	q.emitir(ctx, "queue:update", items)
	if q.statusEmitido && isPaused == q.ultimoPausado && isWorking == q.ultimoTrabalhou {
		return
	}
	q.statusEmitido, q.ultimoPausado, q.ultimoTrabalhou = true, isPaused, isWorking
	q.emitir(ctx, "queue:status", map[string]any{
		"isPaused":  isPaused,
		"isWorking": isWorking,
	})
}

// emitirProgresso envia só o item (queue:item), no máximo uma vez por
// intervaloProgresso. Atualizações dentro do intervalo são coalescidas num
// envio agendado, que lê o estado no disparo e assim leva a mais recente.
func (q *Queue) emitirProgresso(item *QueueItem) {
	q.mu.Lock()
	if q.wailsCtx == nil || item.envioPend != nil {
		q.mu.Unlock()
		return
	}
	espera := intervaloProgresso - time.Since(item.ultimoEnvio)
	if espera > 0 {
		item.geracaoEnvio++
		geracao := item.geracaoEnvio
		item.envioPend = time.AfterFunc(espera, func() { q.enviarItem(item, geracao) })
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()
	q.enviarItem(item, 0)
}

// enviarItem emite o estado atual do item; geracao != 0 indica disparo do
// limitador, descartado se aquele agendamento foi cancelado nesse meio-tempo.
func (q *Queue) enviarItem(item *QueueItem, geracao uint64) {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()

	q.mu.Lock()
	if geracao != 0 {
		if item.envioPend == nil || item.geracaoEnvio != geracao {
			q.mu.Unlock()
			return
		}
		item.envioPend = nil
	}
	ctx := q.wailsCtx
	if ctx == nil || !q.contemLocked(item) {
		q.mu.Unlock()
		return
	}
	item.ultimoEnvio = time.Now()
	copia := copiarItem(item)
	q.mu.Unlock()

	q.emitir(ctx, "queue:item", copia)
}

func (q *Queue) contemLocked(item *QueueItem) bool {
	return slices.Contains(q.items, item)
}

// pararEnvioPendente exige q.mu travado. O broadcast que sucede leva o estado final.
func pararEnvioPendente(item *QueueItem) {
	if item.envioPend != nil {
		item.envioPend.Stop()
		item.envioPend = nil
	}
}

// enviarAtividade emite queue:atividade a cada intervaloAtividade enquanto o
// item processa, só quando algo mudou. A função devolvida para o envio e
// manda o estado final.
func (q *Queue) enviarAtividade(atv *Atividade) func() {
	parar := make(chan struct{})
	feito := make(chan struct{})
	var enviada uint64
	enviar := func() {
		v := atv.Versao()
		if v == enviada {
			return
		}
		q.mu.RLock()
		ctx := q.wailsCtx
		q.mu.RUnlock()
		if ctx == nil {
			return
		}
		enviada = v
		q.emitir(ctx, "queue:atividade", atv.Instantaneo())
	}
	go func() {
		defer close(feito)
		t := time.NewTicker(intervaloAtividade)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				enviar()
			case <-parar:
				enviar()
				return
			}
		}
	}()
	return func() {
		close(parar)
		<-feito
	}
}

// intervaloAtividade limita queue:atividade a 5 por segundo.
var intervaloAtividade = 200 * time.Millisecond

// intervaloProgresso limita queue:item a 10 por segundo por item.
var intervaloProgresso = 100 * time.Millisecond
