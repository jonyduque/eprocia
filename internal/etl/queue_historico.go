package etl

import (
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"eprocia/internal/historico"

	"github.com/google/uuid"
)

// GravadorHistorico é o que a fila usa do gravador do histórico
// (*historico.Gravador a implementa).
type GravadorHistorico interface {
	Agendar(itens []historico.Item)
	GravarAgora(itens []historico.Item) error
	Fechar(itens []historico.Item) error
}

// paraHistorico converte um item da fila na linha gravada no histórico.
func paraHistorico(item *QueueItem) historico.Item {
	kind := item.Kind
	if kind == "" {
		kind = "zip"
	}
	h := historico.Item{
		Name:            item.Name,
		Path:            item.Path,
		Status:          item.Status,
		Message:         item.Message,
		OutputPath:      item.OutputPath,
		DocsCount:       item.DocsCount,
		PagesCount:      item.PagesCount,
		EstimatedTokens: item.EstimatedTokens,
		UnreadDocs:      item.UnreadDocs,
		Error:           item.Error,
		Timestamp:       item.Timestamp,
		FinishedAt:      item.FinishedAt,
		OcrMode:         item.OcrMode,
		CleaningLevel:   item.CleaningLevel,
		Kind:            kind,
	}
	if kind == "pdf" {
		h.Paths = slices.Clone(item.Paths)
	}
	return h
}

// deHistorico monta o QueueItem de uma linha restaurada: ID novo, Restaurado,
// pending e processing viram interrupted, e o TXT gravado vira saidaAnterior.
func deHistorico(h historico.Item) *QueueItem {
	kind := h.Kind
	if kind == "" {
		kind = "zip"
		if strings.EqualFold(filepath.Ext(h.Path), ".pdf") {
			kind = "pdf"
		}
	}
	item := &QueueItem{
		ID:              uuid.NewString(),
		Name:            h.Name,
		Path:            h.Path,
		Kind:            kind,
		Status:          h.Status,
		Message:         h.Message,
		OutputPath:      h.OutputPath,
		DocsCount:       h.DocsCount,
		PagesCount:      h.PagesCount,
		EstimatedTokens: h.EstimatedTokens,
		UnreadDocs:      h.UnreadDocs,
		Error:           h.Error,
		Timestamp:       h.Timestamp,
		FinishedAt:      h.FinishedAt,
		OcrMode:         h.OcrMode,
		CleaningLevel:   h.CleaningLevel,
		Restaurado:      true,
	}
	if kind == "pdf" {
		item.Paths = slices.Clone(h.Paths)
		if len(item.Paths) == 0 {
			item.Paths = []string{h.Path}
		}
	}
	if h.OutputPath != "" {
		item.saidaAnterior = h.OutputPath
	}
	switch item.Status {
	case "pending", "processing":
		item.Status = "interrupted"
		item.Message = "Interrompido"
	case "success":
		item.Progress = 1
		item.ProcessedDocs, item.TotalDocs = h.DocsCount, h.DocsCount
		item.ProcessedPages, item.TotalPages = h.PagesCount, h.PagesCount
	}
	return item
}

// listaGravadaLocked é a lista que vai ao gravador: com sessaoComEntrada, os
// itens da sessão (sem Restaurado); sem ele, os restaurados que restam.
// Nunca nil. Exige q.mu travado (leitura ou escrita).
func (q *Queue) listaGravadaLocked() []historico.Item {
	itens := make([]historico.Item, 0, len(q.items))
	for _, it := range q.items {
		if it.Restaurado == q.sessaoComEntrada {
			continue
		}
		itens = append(itens, paraHistorico(it))
	}
	return itens
}

// entradaLocked marca o item que volta à fila como entrada da sessão atual:
// liga sessaoComEntrada e tira Restaurado. Exige q.mu travado.
func (q *Queue) entradaLocked(item *QueueItem) {
	q.sessaoComEntrada = true
	item.Restaurado = false
}

// Restaurar acrescenta ao fim da fila (depois dos itens que já estavam nela)
// os itens da sessão anterior, na mesma ordem, e emite um só broadcast. Não
// inicia processamento nem liga sessaoComEntrada.
func (q *Queue) Restaurar(itens []historico.Item) {
	q.mu.Lock()
	for _, h := range itens {
		q.items = append(q.items, deHistorico(h))
	}
	q.mu.Unlock()
	slog.Info("histórico restaurado", "itens", len(itens))
	q.broadcast()
}

// SetGravador liga o gravador do histórico; nil o solta. Não grava nada.
func (q *Queue) SetGravador(g GravadorHistorico) {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()
	q.mu.Lock()
	q.gravador = g
	q.mu.Unlock()
}

// gravadorELista exige q.emitMu travado; copia, sob q.mu, o gravador e a
// lista gravada atual.
func (q *Queue) gravadorELista() (GravadorHistorico, []historico.Item) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.gravador == nil {
		return nil, nil
	}
	return q.gravador, q.listaGravadaLocked()
}

// AgendarHistorico manda a lista gravada atual ao gravador, com debounce.
// Sem gravador, não faz nada.
func (q *Queue) AgendarHistorico() {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()
	if g, itens := q.gravadorELista(); g != nil {
		g.Agendar(itens)
	}
}

// GravarHistoricoAgora grava a lista gravada atual já. Sem gravador, nil.
func (q *Queue) GravarHistoricoAgora() error {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()
	if g, itens := q.gravadorELista(); g != nil {
		return g.GravarAgora(itens)
	}
	return nil
}

// FecharHistorico faz a escrita final e fecha o gravador, com os itens ainda
// pending ou processing marcados como interrupted na lista (a fila não muda).
// Sem gravador, nil.
func (q *Queue) FecharHistorico() error {
	q.emitMu.Lock()
	defer q.emitMu.Unlock()
	g, itens := q.gravadorELista()
	if g == nil {
		return nil
	}
	for i := range itens {
		if itens[i].Status == "pending" || itens[i].Status == "processing" {
			itens[i].Status = "interrupted"
			itens[i].Message = "Interrompido"
		}
	}
	return g.Fechar(itens)
}
