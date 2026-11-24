package etl

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"eprocia/internal/config"
	"eprocia/internal/models"
	"eprocia/internal/registro"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// QueueItem represents an item in the execution queue.
type QueueItem struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	Status          string  `json:"status"` // "pending", "processing", "paused", "cancelled", "success", "error"
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
	// UnreadDocs conta os documentos que ficaram sem texto aproveitável; o item
	// continua "success".
	UnreadDocs int `json:"unread_docs"`
	// Relatorio é o relatório da última execução com sucesso; nil enquanto o
	// item aguarda ou processa, e depois de erro ou cancelamento.
	Relatorio *models.RelatorioZip `json:"relatorio,omitempty"`
	Error     string               `json:"error,omitempty"`
	Timestamp int64                `json:"timestamp"`
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

	// processar substitui pipeline.ProcessZip nos testes.
	processar func(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error)

	// carregarConfig substitui config.Load nos testes.
	carregarConfig func() (*models.Config, error)

	// onFinished recebe uma cópia do item que terminou com sucesso ou erro
	// (cancelamentos não avisam), para notificar com a janela oculta.
	onFinished func(item *QueueItem)

	// aoTerminarZip recebe cada execução terminada (sucesso, erro ou
	// cancelamento), para a telemetria; sempre chamado fora de q.mu.
	aoTerminarZip func(models.ExecucaoZip)
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
	select {
	case <-terminou:
		return true
	case <-time.After(prazo):
		slog.Warn("encerramento: o item em processamento não terminou no prazo; saindo mesmo assim", "prazo_ms", prazo.Milliseconds())
		return false
	}
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
func (q *Queue) execucaoLocked(item *QueueItem, reprocesso bool, cfg *models.Config, bytesZip int64, classeErro string) func() {
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

// AddFiles adds multiple zip files to the queue and returns how many actually
// entered (duplicates of pending/processing items are dropped). Origem manual.
func (q *Queue) AddFiles(paths []string) int {
	return q.AddFilesComOrigem(paths, "manual")
}

// AddFilesComOrigem é AddFiles com a origem do ZIP ("monitorada",
// "arrastada" ou "manual"), que vai para o log e a telemetria.
func (q *Queue) AddFilesComOrigem(paths []string, origem string) int {
	q.mu.Lock()
	hasPending := false
	adicionados := 0
	for _, path := range paths {
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
			Status:    "pending",
			Progress:  0.0,
			Message:   "Na fila...",
			Timestamp: time.Now().UnixMilli(),
			origem:    origem,
		}
		// Prepend to show newest on top
		q.items = append([]*QueueItem{item}, q.items...)
		hasPending = true
		adicionados++
		slog.Info("ZIP adicionado à fila", "item_id", item.ID, "zip", item.Name, "origem", origem)
	}
	q.mu.Unlock()

	q.broadcast()

	if hasPending {
		q.startProcessing()
	}
	return adicionados
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
	}
}

// Reprocessar devolve à fila um item que já terminou, com OCR e higienização
// próprios e, se forcarOCR não for nil, com Forçar OCR próprio (nil usa o da
// config global; com ocrMode "none" ele não tem efeito). A config global não
// muda; o .txt anterior é sobrescrito.
func (q *Queue) Reprocessar(id, ocrMode, cleaningLevel string, forcarOCR *bool) error {
	if !config.ModoOCRValido(ocrMode) {
		return fmt.Errorf("modo de OCR inválido: %q", ocrMode)
	}
	if !config.LimpezaValida(cleaningLevel) {
		return fmt.Errorf("nível de higienização inválido: %q", cleaningLevel)
	}
	q.mu.Lock()
	idx := slices.IndexFunc(q.items, func(it *QueueItem) bool { return it.ID == id })
	if idx < 0 {
		q.mu.Unlock()
		return fmt.Errorf("item não encontrado na fila")
	}
	item := q.items[idx]
	if item.Status == "pending" || item.Status == "processing" {
		q.mu.Unlock()
		return fmt.Errorf("o item ainda está na fila ou em processamento")
	}
	pararEnvioPendente(item)
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
	item.Status = "pending"
	item.Message = "Na fila..."
	item.Progress = 0
	item.Error = ""
	item.OutputPath = ""
	item.DocsCount, item.PagesCount, item.EstimatedTokens = 0, 0, 0
	item.ProcessedDocs, item.TotalDocs, item.ProcessedPages, item.TotalPages = 0, 0, 0, 0
	item.UnreadDocs = 0
	item.Relatorio = nil
	item.Timestamp = time.Now().UnixMilli()
	// Vai para o topo, como um arquivo novo: a fila processa de baixo para cima.
	q.items = slices.Delete(q.items, idx, idx+1)
	q.items = append([]*QueueItem{item}, q.items...)
	q.mu.Unlock()

	attrs := []any{"item_id", item.ID, "zip", item.Name, "ocr_mode", ocrMode, "cleaning_level", cleaningLevel}
	if forcar != nil {
		attrs = append(attrs, "forcar_ocr", *forcar)
	}
	slog.Info("reprocessar", attrs...)
	q.broadcast()
	q.startProcessing()
	return nil
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
		slog.Info("cancelamento pedido", "item_id", target.ID, "zip", target.Name, "status", target.Status)
		if target.Status == "processing" && target.cancel != nil {
			target.cancel()
		} else if target.Status == "pending" {
			target.Status = "cancelled"
			target.Message = "Cancelado"
		}
	}
	q.mu.Unlock()
	q.broadcast()
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
	q.mu.Unlock()
	q.broadcast()
}

// LimparConcluidos tira da lista os itens com sucesso, erro ou cancelados; o
// item em processamento e os que aguardam continuam como estão.
func (q *Queue) LimparConcluidos() {
	q.mu.Lock()
	restantes := q.items[:0:0]
	for _, item := range q.items {
		switch item.Status {
		case "success", "error", "cancelled":
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
	slog.Debug("startProcessing", "pausada", q.isPaused, "em_execucao", q.isWorking, "itens", len(q.items))
	if q.encerrada {
		q.mu.Unlock()
		return
	}
	if q.isPaused || q.isWorking {
		q.mu.Unlock()
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
		q.mu.Unlock()
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
			slog.Default().With("item_id", item.ID).Error("pânico no processador da fila", "zip", item.Name,
				"erro", fmt.Sprint(r), "pilha", string(debug.Stack()))
			q.mu.Lock()
			item.cancel = nil
			pararEnvioPendente(item)
			item.Status = "error"
			item.Message = fmt.Sprintf("Erro crítico (panic): %v", r)
			item.Error = fmt.Sprintf("Panic: %v", r)
			q.isWorking = false
			avisar := q.avisarFimLocked(item)
			telemetria := q.execucaoLocked(item, reprocesso, cfgExec, bytesZip, "panico")
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
	reprocesso = ajuste != nil
	origem := item.origem
	q.mu.Unlock()

	q.broadcast()

	lg := registro.Logger(ctx)
	if info, err := os.Stat(item.Path); err == nil {
		bytesZip = info.Size()
	}
	lg.Info("item na vez", "zip", item.Name, "origem", origem, "reprocesso", reprocesso, "bytes_zip", bytesZip)
	onProgress := func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int) {
		lg.Debug("progresso", "zip", item.Name, "progresso", progress, "mensagem", message,
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
			lg.Error("falha ao carregar a configuração do item", "zip", item.Name, "erro", err)
			q.mu.Lock()
			item.Status = "error"
			item.Message = "Erro ao carregar configurações"
			item.Error = err.Error()
			q.isWorking = false
			avisar := q.avisarFimLocked(item)
			telemetria := q.execucaoLocked(item, reprocesso, nil, bytesZip, "config")
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
		processar = func(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			return q.pipeline.ProcessZipComAtividade(ctx, zipPath, cfg, onProgress, atv)
		}
	}
	pararAtividade := q.enviarAtividade(atv)
	outputPath, docsCount, pagesCount, tokens, err := processar(ctx, item.Path, cfg, onProgress)
	pararAtividade()

	q.mu.Lock()
	item.cancel = nil
	q.isWorking = false
	pararEnvioPendente(item)

	classeErro := ""
	if err != nil {
		if ctx.Err() == context.Canceled {
			item.Status = "cancelled"
			item.Message = "Cancelado"
			item.Progress = 0.0
		} else {
			item.Status = "error"
			item.Message = err.Error()
			item.Error = err.Error()
			classeErro = string(classificarErro(err))
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
	}
	avisar := q.avisarFimLocked(item)
	telemetria := q.execucaoLocked(item, reprocesso, cfg, bytesZip, classeErro)
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
	if ctx == nil {
		q.mu.RUnlock()
		return
	}
	items := q.copyItemsLocked()
	isPaused, isWorking := q.isPaused, q.isWorking
	q.mu.RUnlock()

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
