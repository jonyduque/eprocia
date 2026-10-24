package etl

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"eprocia/internal/models"
)

type finalizados struct {
	mu    sync.Mutex
	itens []*QueueItem
}

func (f *finalizados) registrar(it *QueueItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.itens = append(f.itens, it)
}

func (f *finalizados) esperar(t *testing.T, n int) []*QueueItem {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		f.mu.Lock()
		if len(f.itens) >= n {
			r := append([]*QueueItem(nil), f.itens...)
			f.mu.Unlock()
			return r
		}
		f.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("onFinished não foi chamado %d vez(es)", n)
	return nil
}

func TestFila_AvisaFimComSucessoEErro(t *testing.T) {
	q, _ := novaFilaGravada(t, func(_ context.Context, path string, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		if path == "ruim.zip" {
			return "", 0, 0, 0, errors.New("ZIP corrompido")
		}
		return "C:\\saida\\bom.txt", 3, 12, 5000, nil
	})
	f := &finalizados{}
	q.SetOnFinished(f.registrar)

	q.AddFiles([]string{"bom.zip", "ruim.zip"})
	itens := f.esperar(t, 2)

	porNome := map[string]*QueueItem{}
	for _, it := range itens {
		porNome[it.Name] = it
	}
	bom, ruim := porNome["bom.zip"], porNome["ruim.zip"]
	if bom == nil || bom.Status != "success" || bom.OutputPath != "C:\\saida\\bom.txt" || bom.DocsCount != 3 || bom.PagesCount != 12 || bom.EstimatedTokens != 5000 {
		t.Fatalf("aviso de sucesso: %+v", bom)
	}
	if ruim == nil || ruim.Status != "error" || ruim.Error != "ZIP corrompido" {
		t.Fatalf("aviso de erro: %+v", ruim)
	}
}

func TestFila_NaoAvisaFimQuandoCancelado(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)
	f := &finalizados{}
	q.SetOnFinished(f.registrar)

	q.AddFiles([]string{"a.zip"})
	p.esperarInicio(t, "a.zip")
	id := q.GetItems()[0].ID
	q.CancelItem(id)
	esperarFim(t, q, "a.zip")
	time.Sleep(50 * time.Millisecond)

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.itens) != 0 {
		t.Fatalf("cancelamento gerou aviso de fim: %+v", f.itens[0])
	}
}
