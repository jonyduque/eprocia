package etl

import (
	"context"
	"sync"
	"testing"

	"eprocia/internal/models"
)

func TestReprocessar_UsaAjusteEVoltaParaFila(t *testing.T) {
	var mu sync.Mutex
	var cfgs []models.Config
	q, _ := novaFilaGravada(t, func(_ context.Context, _ string, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		mu.Lock()
		cfgs = append(cfgs, *cfg)
		mu.Unlock()
		onProgress(1, "fim", 1, 1, 3, 3)
		return "saida.txt", 1, 3, 0, nil
	})
	q.cfg = &models.Config{CleaningLevel: "basic", OcrMode: "normal", MonitoredPath: `C:\pasta`}
	q.AddFiles([]string{`C:\x\a.zip`})
	esperarFim(t, q, "a.zip")
	id := q.GetItems()[0].ID
	if it := q.GetItems()[0]; it.OcrMode != "normal" || it.CleaningLevel != "basic" {
		t.Errorf("item não registrou a config usada: %+v", it)
	}

	if err := q.Reprocessar(id, "best", "aggressive"); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	esperarFim(t, q, "a.zip")

	mu.Lock()
	defer mu.Unlock()
	if len(cfgs) != 2 {
		t.Fatalf("processou %d vezes, esperava 2", len(cfgs))
	}
	if c := cfgs[1]; c.OcrMode != "best" || c.CleaningLevel != "aggressive" || c.MonitoredPath != `C:\pasta` {
		t.Errorf("config do reprocessamento: %+v", c)
	}
	if q.cfg.OcrMode != "normal" || q.cfg.CleaningLevel != "basic" {
		t.Errorf("o ajuste vazou para a config global: %+v", q.cfg)
	}
	itens := q.GetItems()
	if len(itens) != 1 || itens[0].ID != id || itens[0].Status != "success" {
		t.Fatalf("esperava o mesmo item concluído de novo: %+v", itens)
	}
	if itens[0].OcrMode != "best" || itens[0].CleaningLevel != "aggressive" {
		t.Errorf("item não registrou a config do reprocessamento: %+v", itens[0])
	}
}

func TestReprocessar_Recusa(t *testing.T) {
	liberar := make(chan struct{})
	q, _ := novaFilaGravada(t, func(context.Context, string, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		<-liberar
		return "saida.txt", 1, 1, 0, nil
	})
	defer close(liberar)
	q.AddFiles([]string{`C:\x\a.zip`})
	id := q.GetItems()[0].ID
	if err := q.Reprocessar(id, "fast", "none"); err == nil {
		t.Error("aceitou reprocessar item em andamento")
	}
	if err := q.Reprocessar("nao-existe", "fast", "none"); err == nil {
		t.Error("aceitou id inexistente")
	}
	if err := q.Reprocessar(id, "turbo", "none"); err == nil {
		t.Error("aceitou modo de OCR inválido")
	}
	if err := q.Reprocessar(id, "fast", "total"); err == nil {
		t.Error("aceitou higienização inválida")
	}
}

func TestAtividade_DisponivelPorItem(t *testing.T) {
	q, _ := novaFilaGravada(t, func(context.Context, string, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		return "saida.txt", 1, 1, 0, nil
	})
	q.AddFiles([]string{`C:\x\a.zip`})
	esperarFim(t, q, "a.zip")
	id := q.GetItems()[0].ID
	if inst := q.GetAtividade(id); inst == nil || inst.ItemID != id {
		t.Errorf("atividade do item: %+v", inst)
	}
	if q.GetAtividade("nao-existe") != nil {
		t.Error("atividade de id inexistente deveria ser nil")
	}
}
