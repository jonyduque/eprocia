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
// (*historico.Gravador a implementa). A fila chama os três métodos com
// q.emitMu travado, o que garante a ordem das listas; por isso o AoFalhar do
// gravador, síncrono em GravarAgora e Fechar, não pode chamar a fila
// (broadcast nem os métodos *Historico): só emitir um evento ou registrar no
// log, e nada além disso.
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
		SaidaAnterior:   item.saidaAnterior,
	}
	if kind == "pdf" {
		h.Paths = slices.Clone(item.Paths)
	}
	return h
}

// deHistorico monta o QueueItem de uma linha restaurada: ID novo, Restaurado,
// pending e processing viram interrupted, e a saida_anterior gravada (ou, sem
// ela, o TXT gravado) vira saidaAnterior.
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
	switch {
	case h.SaidaAnterior != "":
		item.saidaAnterior = h.SaidaAnterior
	case h.OutputPath != "":
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

// caminhoIgual compara dois caminhos sem diferença de caixa, depois de
// filepath.Clean.
func caminhoIgual(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// origensDe são os arquivos de origem do item: os Paths, ou o Path quando
// não há Paths.
func origensDe(item *QueueItem) []string {
	if len(item.Paths) > 0 {
		return item.Paths
	}
	return []string{item.Path}
}

// mesmoConjunto diz se a e b têm os mesmos caminhos (caminhoIgual), em
// qualquer ordem, cada um casado uma vez só.
func mesmoConjunto(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	usado := make([]bool, len(b))
	for _, x := range a {
		achou := false
		for j, y := range b {
			if !usado[j] && caminhoIgual(x, y) {
				usado[j], achou = true, true
				break
			}
		}
		if !achou {
			return false
		}
	}
	return true
}

// interrompidoLocked acha a linha "interrupted" do kind com as mesmas
// origens (um ZIP: o Path; PDFs: o mesmo conjunto de Paths). -1 se não há.
// Exige q.mu travado.
func (q *Queue) interrompidoLocked(kind string, origens []string) int {
	return slices.IndexFunc(q.items, func(it *QueueItem) bool {
		return it.Status == "interrupted" && it.Kind == kind && mesmoConjunto(origensDe(it), origens)
	})
}

// reaproveitarLocked devolve à fila, na mesma posição, a linha interrompida
// que entrou de novo por AddFiles: zerada como no Reprocessar, com a config
// global (sem ajuste) e a origem nova. Exige q.mu travado.
func (q *Queue) reaproveitarLocked(item *QueueItem, origem string) {
	reabrirLocked(item)
	item.ajuste = nil
	item.reaberto = false
	item.origem = origem
	q.entradaLocked(item)
}

// ConferirArquivos marca, nos itens restaurados, SourceMissing (o Path ou um
// dos Paths não existe) e OutputMissing (o OutputPath não existe). O disco é
// lido fora de q.mu; ao aplicar, um item que saiu da fila ou deixou de ser
// Restaurado nesse meio-tempo fica como está. Um só broadcast, e só se havia
// item restaurado.
func (q *Queue) ConferirArquivos(existe func(caminho string) bool) {
	type conferir struct {
		id      string
		origens []string
		saida   string
	}
	q.mu.RLock()
	var lista []conferir
	for _, it := range q.items {
		if it.Restaurado {
			lista = append(lista, conferir{it.ID, slices.Clone(origensDe(it)), it.OutputPath})
		}
	}
	q.mu.RUnlock()
	if len(lista) == 0 {
		return
	}

	type marca struct{ origem, saida bool }
	marcas := make(map[string]marca, len(lista))
	for _, c := range lista {
		m := marca{origem: slices.ContainsFunc(c.origens, func(p string) bool { return !existe(p) })}
		m.saida = c.saida != "" && !existe(c.saida)
		marcas[c.id] = m
	}

	q.mu.Lock()
	for _, it := range q.items {
		if m, ok := marcas[it.ID]; ok && it.Restaurado {
			it.SourceMissing = it.SourceMissing || m.origem
			it.OutputMissing = it.OutputMissing || m.saida
		}
	}
	q.mu.Unlock()
	q.broadcast()
}

// MarcarSaidaAusente marca OutputMissing em todo item cujo OutputPath é
// outputPath (caminhoIgual) e faz broadcast.
func (q *Queue) MarcarSaidaAusente(outputPath string) {
	q.mu.Lock()
	for _, it := range q.items {
		if it.OutputPath != "" && caminhoIgual(it.OutputPath, outputPath) {
			it.OutputMissing = true
		}
	}
	q.mu.Unlock()
	q.broadcast()
}

// SetGravador liga o gravador do histórico; nil o solta. Não grava nada.
// O AoFalhar do gravador roda de forma síncrona, com q.emitMu travado, dentro
// de GravarAgora e Fechar: não pode chamar a fila (broadcast nem os métodos
// *Historico), só emitir um evento ou registrar no log.
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
