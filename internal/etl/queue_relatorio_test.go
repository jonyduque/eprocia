package etl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"eprocia/internal/models"
)

func TestQueue_RelatorioZeradoAoReprocessarECancelar(t *testing.T) {
	var mu sync.Mutex
	var eventos []string // queue:item e queue:update serializados
	q := NewQueue(NewPipeline())
	defer q.Shutdown()
	q.cfg = cfgSemOCR()
	q.existe = func(string) bool { return true } // caminhos fictícios
	q.SetContext(context.Background())
	q.emitir = func(_ context.Context, nome string, dados ...any) {
		if nome != "queue:item" && nome != "queue:update" {
			return
		}
		b, _ := json.Marshal(dados[0])
		mu.Lock()
		eventos = append(eventos, nome+" "+string(b))
		mu.Unlock()
	}

	// O processar falso acha a atividade do item (criada pela fila antes de
	// chamá-lo) e grava o relatório; o modo diz como a execução termina.
	var modo string // "ok", "erro" ou "cancelar"
	var vistoAntes *models.RelatorioZip
	q.processar = func(ctx context.Context, e Entrada, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		q.mu.Lock()
		var item *QueueItem
		for _, it := range q.items {
			if it.Path == e.Caminhos[0] {
				item = it
			}
		}
		atv := item.atividade
		vistoAntes = item.Relatorio
		q.mu.Unlock()
		atv.definirRelatorio(&models.RelatorioZip{Tokens: 42, Paginas: models.PaginasRelatorio{Total: 3}})
		onProgress(0.5, "meio", 1, 2, 1, 3)
		switch modo {
		case "erro":
			return "", 0, 0, 0, errors.New("falha fictícia")
		case "cancelar":
			item.cancel()
			<-ctx.Done()
			return "", 0, 0, 0, ctx.Err()
		}
		return "saida.txt", 2, 3, 42, nil
	}

	modo = "ok"
	q.AddFiles([]string{"processo.zip"})
	item := esperarItemComStatus(t, q, "success")
	if item.Relatorio == nil || item.Relatorio.Tokens != 42 {
		t.Fatalf("item success com Relatorio = %+v", item.Relatorio)
	}
	b, _ := json.Marshal(item)
	if !strings.Contains(string(b), `"relatorio":{`) {
		t.Errorf("JSON do item sem a chave relatorio: %s", b)
	}
	mu.Lock()
	achou := false
	for _, e := range eventos {
		if strings.HasPrefix(e, "queue:update") && strings.Contains(e, `"relatorio":{`) {
			achou = true
		}
	}
	mu.Unlock()
	if !achou {
		t.Errorf("nenhum queue:update com relatorio")
	}

	reprocessar := func(m string) {
		t.Helper()
		modo = m
		q.Pause()
		if err := q.Reprocessar(item.ID, "none", "none", nil); err != nil {
			t.Fatalf("Reprocessar: %v", err)
		}
		if got := q.GetItems()[0]; got.Relatorio != nil {
			t.Errorf("Reprocessar deixou Relatorio = %+v", got.Relatorio)
		}
		q.Resume()
	}

	reprocessar("erro")
	it := esperarItemComStatus(t, q, "error")
	if it.Relatorio != nil {
		t.Errorf("execução com erro terminou com Relatorio = %+v", it.Relatorio)
	}
	if vistoAntes != nil {
		t.Errorf("processar viu Relatorio = %+v antes de rodar", vistoAntes)
	}

	// Um item com relatório (de uma execução anterior) que é cancelado termina sem.
	modo = "ok"
	q.Pause()
	if err := q.Reprocessar(item.ID, "none", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	q.Resume()
	esperarItemComStatus(t, q, "success")
	reprocessar("cancelar")
	it = esperarItemComStatus(t, q, "cancelled")
	if it.Relatorio != nil {
		t.Errorf("execução cancelada terminou com Relatorio = %+v", it.Relatorio)
	}

	// processItem zera o relatório no início mesmo sem Reprocessar.
	q.mu.Lock()
	q.items[0].Relatorio = &models.RelatorioZip{Tokens: 7}
	q.items[0].Status = "pending"
	q.mu.Unlock()
	modo = "erro"
	q.startProcessing()
	it = esperarItemComStatus(t, q, "error")
	if it.Relatorio != nil || vistoAntes != nil {
		t.Errorf("Relatorio = %+v, visto antes = %+v", it.Relatorio, vistoAntes)
	}
}

func TestAtividade_RelatorioReceptorNil(t *testing.T) {
	var a *Atividade
	a.definirRelatorio(&models.RelatorioZip{})
	if a.Relatorio() != nil {
		t.Error("atividade nil devolveu relatório")
	}
	b := NovaAtividade("x")
	r := &models.RelatorioZip{Tokens: 1}
	b.definirRelatorio(r)
	if b.Relatorio() != r {
		t.Error("relatório não guardado")
	}
}
