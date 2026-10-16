package etl

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"eprocia/internal/config"
	"eprocia/internal/models"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// QueueItem represents an item in the execution queue.
type QueueItem struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Path           string             `json:"path"`
	Status         string             `json:"status"` // "pending", "processing", "paused", "cancelled", "success", "error"
	Progress       float64            `json:"progress"`
	Message        string             `json:"message"`
	OutputPath     string             `json:"output_path"`
	DocsCount      int                `json:"docs_count"`
	PagesCount     int                `json:"pages_count"`
	ProcessedDocs  int                `json:"processed_docs"`
	TotalDocs      int                `json:"total_docs"`
	ProcessedPages int                `json:"processed_pages"`
	TotalPages     int                `json:"total_pages"`
	Error          string             `json:"error,omitempty"`
	Timestamp      int64              `json:"timestamp"`
	cancel         context.CancelFunc `json:"-"`
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

	// processar substitui pipeline.ProcessZip nos testes.
	processar func(ctx context.Context, zipPath string, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, error)
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
		log.Printf("[Queue] Encerramento: o item em processamento não terminou em %v; saindo mesmo assim", prazo)
		return false
	}
}

// SetContext sets the Wails context for event emission.
func (q *Queue) SetContext(ctx context.Context) {
	q.mu.Lock()
	q.wailsCtx = ctx
	q.mu.Unlock()
	q.pipeline.SetContext(ctx)
}

// SetConfig sets/updates the configuration settings for the queue.
func (q *Queue) SetConfig(cfg *models.Config) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cfg = cfg
}

// AddFiles adds multiple zip files to the queue.
func (q *Queue) AddFiles(paths []string) {
	log.Printf("[Queue] AddFiles chamada com caminhos: %v", paths)
	q.mu.Lock()
	hasPending := false
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
			log.Printf("[Queue] Arquivo duplicado ignorado: %s", path)
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
			Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		}
		// Prepend to show newest on top
		q.items = append([]*QueueItem{item}, q.items...)
		hasPending = true
		log.Printf("[Queue] Arquivo adicionado à fila: %s (Status: %s)", item.Name, item.Status)
	}
	q.mu.Unlock()

	q.broadcast()

	if hasPending {
		log.Printf("[Queue] Fila possui novos itens pendentes. Iniciando processamento...")
		q.startProcessing()
	} else {
		log.Printf("[Queue] Nenhum item novo pendente na fila.")
	}
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
		copied[i] = &QueueItem{
			ID:             item.ID,
			Name:           item.Name,
			Path:           item.Path,
			Status:         item.Status,
			Progress:       item.Progress,
			Message:        item.Message,
			OutputPath:     item.OutputPath,
			DocsCount:      item.DocsCount,
			PagesCount:     item.PagesCount,
			ProcessedDocs:  item.ProcessedDocs,
			TotalDocs:      item.TotalDocs,
			ProcessedPages: item.ProcessedPages,
			TotalPages:     item.TotalPages,
			Error:          item.Error,
			Timestamp:      item.Timestamp,
		}
	}
	return copied
}

// Pause pauses processing of the queue.
func (q *Queue) Pause() {
	q.mu.Lock()
	q.isPaused = true
	q.mu.Unlock()
	q.broadcast()
}

// Resume resumes processing of the queue.
func (q *Queue) Resume() {
	q.mu.Lock()
	q.isPaused = false
	q.mu.Unlock()
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

func (q *Queue) startProcessing() {
	q.mu.Lock()
	log.Printf("[Queue] startProcessing chamado. isPaused: %t, isWorking: %t, total_items: %d", q.isPaused, q.isWorking, len(q.items))
	if q.encerrada {
		q.mu.Unlock()
		return
	}
	if q.isPaused || q.isWorking {
		log.Printf("[Queue] Processador pausado ou já em execução. Retornando.")
		q.mu.Unlock()
		return
	}

	// Find the next pending item, starting from the oldest (bottom of stack)
	var nextItem *QueueItem
	for i := len(q.items) - 1; i >= 0; i-- {
		if q.items[i].Status == "pending" {
			nextItem = q.items[i]
			break
		}
	}

	if nextItem == nil {
		log.Printf("[Queue] Nenhum item pendente encontrado na fila.")
		q.mu.Unlock()
		return
	}

	log.Printf("[Queue] Próximo item para processamento: %s (%s)", nextItem.Name, nextItem.Path)
	q.isWorking = true
	// Add sob q.mu com encerrada == false: Shutdown marca encerrada antes de
	// esperar, então nenhum Add acontece com o contador em zero durante o Wait.
	q.wg.Add(1)
	q.mu.Unlock()

	go func() {
		defer q.wg.Done()
		q.processItem(nextItem)
	}()
}

func (q *Queue) processItem(item *QueueItem) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Queue] ERRO CRÍTICO (panic) no processador de fila para %s: %v", item.Name, r)
			q.mu.Lock()
			item.cancel = nil
			item.Status = "error"
			item.Message = fmt.Sprintf("Erro crítico (panic): %v", r)
			item.Error = fmt.Sprintf("Panic: %v", r)
			q.isWorking = false
			q.mu.Unlock()
			q.broadcast()
			q.startProcessing()
		}
	}()

	log.Printf("[Queue] Iniciando processamento do arquivo: %s", item.Path)

	q.mu.Lock()
	ctx, cancel := context.WithCancel(q.baseCtx)
	defer cancel()
	item.cancel = cancel
	item.Status = "processing"
	item.Message = "Iniciando..."
	cfg := q.cfg
	q.mu.Unlock()

	q.broadcast()

	onProgress := func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int) {
		log.Printf("[Queue] Progresso [%s]: %.2f%% - %s (Docs: %d/%d, Páginas: %d/%d)",
			item.Name, progress*100, message, processedDocs, totalDocs, processedPages, totalPages)
		q.mu.Lock()
		if item.Status == "processing" {
			item.Progress = progress
			item.Message = message
			item.ProcessedDocs = processedDocs
			item.TotalDocs = totalDocs
			item.ProcessedPages = processedPages
			item.TotalPages = totalPages
		}
		q.mu.Unlock()
		q.broadcast()
	}

	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			log.Printf("[Queue] Erro ao carregar configurações para %s: %v", item.Name, err)
			q.mu.Lock()
			item.Status = "error"
			item.Message = "Erro ao carregar configurações"
			item.Error = err.Error()
			q.isWorking = false
			q.mu.Unlock()
			q.broadcast()
			q.startProcessing()
			return
		}
	}

	processar := q.processar
	if processar == nil {
		processar = q.pipeline.ProcessZip
	}
	outputPath, docsCount, pagesCount, err := processar(ctx, item.Path, cfg, onProgress)

	q.mu.Lock()
	item.cancel = nil
	q.isWorking = false

	if err != nil {
		log.Printf("[Queue] Erro no processamento de %s: %v", item.Name, err)
		if ctx.Err() == context.Canceled {
			item.Status = "cancelled"
			item.Message = "Cancelado"
			item.Progress = 0.0
		} else {
			item.Status = "error"
			item.Message = err.Error()
			item.Error = err.Error()
		}
	} else {
		log.Printf("[Queue] Sucesso no processamento de %s. Docs: %d, Páginas: %d", item.Name, docsCount, pagesCount)
		item.Status = "success"
		item.Progress = 1.0
		item.Message = "Concluído"
		item.OutputPath = outputPath
		item.DocsCount = docsCount
		item.PagesCount = pagesCount
		item.ProcessedDocs = docsCount
		item.TotalDocs = docsCount
		item.ProcessedPages = pagesCount
		item.TotalPages = pagesCount
	}
	q.mu.Unlock()

	q.broadcast()
	q.startProcessing()
}

func (q *Queue) broadcast() {
	q.mu.RLock()
	ctx := q.wailsCtx
	if ctx == nil {
		q.mu.RUnlock()
		return
	}
	items := q.copyItemsLocked()
	isPaused, isWorking := q.isPaused, q.isWorking
	q.mu.RUnlock()

	runtime.EventsEmit(ctx, "queue:update", items)
	runtime.EventsEmit(ctx, "queue:status", map[string]interface{}{
		"isPaused":  isPaused,
		"isWorking": isWorking,
	})
}
