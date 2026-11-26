package etl

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"eprocia/internal/config"
	"eprocia/internal/models"
)

func TestReprocessar_UsaAjusteEVoltaParaFila(t *testing.T) {
	var mu sync.Mutex
	var cfgs []models.Config
	q, _ := novaFilaGravada(t, func(_ context.Context, _ Entrada, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
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

	if err := q.Reprocessar(id, "best", "aggressive", nil); err != nil {
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
	q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		<-liberar
		return "saida.txt", 1, 1, 0, nil
	})
	defer close(liberar)
	q.AddFiles([]string{`C:\x\a.zip`})
	id := q.GetItems()[0].ID
	if err := q.Reprocessar(id, "fast", "none", nil); err == nil {
		t.Error("aceitou reprocessar item em andamento")
	}
	if err := q.Reprocessar("nao-existe", "fast", "none", nil); err == nil {
		t.Error("aceitou id inexistente")
	}
	if err := q.Reprocessar(id, "turbo", "none", nil); err == nil {
		t.Error("aceitou modo de OCR inválido")
	}
	if err := q.Reprocessar(id, "fast", "total", nil); err == nil {
		t.Error("aceitou higienização inválida")
	}
}

func TestAtividade_DisponivelPorItem(t *testing.T) {
	q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
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

// filaCapturandoCfg devolve uma fila cujo processador falso guarda a config
// recebida a cada execução.
func filaCapturandoCfg(t *testing.T) (*Queue, func() []models.Config) {
	t.Helper()
	var mu sync.Mutex
	var cfgs []models.Config
	q, _ := novaFilaGravada(t, func(_ context.Context, _ Entrada, cfg *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		mu.Lock()
		cfgs = append(cfgs, *cfg)
		mu.Unlock()
		return "saida.txt", 1, 1, 0, nil
	})
	return q, func() []models.Config {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(cfgs)
	}
}

func TestReprocessar_ForcarOCRSobreGlobal(t *testing.T) {
	casos := []struct {
		nome   string
		global bool
		ajuste *bool
		quer   bool
	}{
		{"global false + true força", false, new(true), true},
		{"global true + false não força", true, new(false), false},
		{"global true + nil força", true, nil, true},
		{"global false + nil não força", false, nil, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			q, cfgs := filaCapturandoCfg(t)
			q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "normal", ForcarOCR: c.global}
			q.AddFiles([]string{`C:\x\a.zip`})
			esperarFim(t, q, "a.zip")
			id := q.GetItems()[0].ID
			if err := q.Reprocessar(id, "normal", "standard", c.ajuste); err != nil {
				t.Fatalf("Reprocessar: %v", err)
			}
			esperarFim(t, q, "a.zip")
			got := cfgs()
			if len(got) != 2 {
				t.Fatalf("processou %d vezes, esperava 2", len(got))
			}
			if f := forcarOCREfetivo(&got[1]); f != c.quer {
				t.Errorf("forçar efetivo = %t, quero %t (cfg %+v)", f, c.quer, got[1])
			}
			if it := q.GetItems()[0]; it.ForcarOCR != c.quer {
				t.Errorf("QueueItem.ForcarOCR = %t, quero %t", it.ForcarOCR, c.quer)
			}
			if q.cfg.ForcarOCR != c.global {
				t.Errorf("o ajuste vazou para a config global: %+v", q.cfg)
			}
		})
	}
}

func TestReprocessar_NaoMudaConfigGlobal(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"monitored_path":"C:\\Users\\fulano\\Downloads","ocr_mode":"fast","cleaning_level":"basic","forcar_ocr":false}`)
	if err := os.WriteFile(caminho, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Config global em memória.
	q, _ := filaCapturandoCfg(t)
	q.cfg = &models.Config{CleaningLevel: "basic", OcrMode: "fast", MonitoredPath: `C:\Users\fulano\Downloads`, OmitirTipos: []string{"EXTR"}}
	antes := *q.cfg
	antesTipos := slices.Clone(q.cfg.OmitirTipos)
	q.AddFiles([]string{`C:\x\a.zip`})
	esperarFim(t, q, "a.zip")
	id := q.GetItems()[0].ID
	for _, ajuste := range []*bool{new(true), new(false), nil} {
		if err := q.Reprocessar(id, "best", "aggressive", ajuste); err != nil {
			t.Fatalf("Reprocessar: %v", err)
		}
		esperarFim(t, q, "a.zip")
	}
	if q.cfg.MonitoredPath != antes.MonitoredPath || q.cfg.OcrMode != antes.OcrMode ||
		q.cfg.CleaningLevel != antes.CleaningLevel || q.cfg.ForcarOCR != antes.ForcarOCR ||
		!slices.Equal(q.cfg.OmitirTipos, antesTipos) {
		t.Errorf("q.cfg mudou: antes %+v, depois %+v", antes, *q.cfg)
	}

	// Config lida do arquivo a cada item (q.cfg nil).
	q2, cfgs := filaCapturandoCfg(t)
	q2.cfg = nil
	q2.carregarConfig = func() (*models.Config, error) { return config.LoadFromFile(caminho) }
	q2.AddFiles([]string{`C:\x\b.zip`})
	esperarFim(t, q2, "b.zip")
	id2 := q2.GetItems()[0].ID
	for _, ajuste := range []*bool{new(true), new(false), nil} {
		if err := q2.Reprocessar(id2, "best", "aggressive", ajuste); err != nil {
			t.Fatalf("Reprocessar: %v", err)
		}
		esperarFim(t, q2, "b.zip")
	}
	if got := cfgs(); len(got) != 4 || !got[1].ForcarOCR {
		t.Fatalf("execuções: %+v", got)
	}
	depois, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(depois, original) {
		t.Errorf("config.json mudou:\nantes  %s\ndepois %s", original, depois)
	}
}

func TestReprocessar_NoneComGlobalTrue(t *testing.T) {
	for _, ajuste := range []*bool{nil, new(true)} {
		q, cfgs := filaCapturandoCfg(t)
		q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "normal", ForcarOCR: true}
		q.AddFiles([]string{`C:\x\a.zip`})
		esperarFim(t, q, "a.zip")
		id := q.GetItems()[0].ID
		if err := q.Reprocessar(id, "none", "standard", ajuste); err != nil {
			t.Fatalf("Reprocessar: %v", err)
		}
		esperarFim(t, q, "a.zip")
		got := cfgs()
		if len(got) != 2 || forcarOCREfetivo(&got[1]) {
			t.Errorf("ajuste %v: com ocr_mode none o reprocessamento forçou o OCR: %+v", ajuste, got)
		}
		if it := q.GetItems()[0]; it.ForcarOCR {
			t.Errorf("ajuste %v: QueueItem.ForcarOCR = true com ocr_mode none", ajuste)
		}
	}
}

// Enquanto aguarda, o item já mostra o Forçar OCR efetivo da próxima execução,
// também quando o Reprocessar não informa o valor (nil usa a global).
func TestReprocessar_ForcarEfetivoNoItemPendente(t *testing.T) {
	q, _ := filaCapturandoCfg(t)
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "normal", ForcarOCR: true}
	q.Pause()
	q.items = append(q.items, &QueueItem{ID: "1", Name: "a.zip", Status: "success", ForcarOCR: true})
	if err := q.Reprocessar("1", "none", "standard", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	if it := q.GetItems()[0]; it.Status != "pending" || it.ForcarOCR {
		t.Errorf("item pendente com none e nil: status %q, ForcarOCR %t; quero pending e false", it.Status, it.ForcarOCR)
	}
	q.items[0].Status = "success"
	if err := q.Reprocessar("1", "normal", "standard", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	if it := q.GetItems()[0]; !it.ForcarOCR {
		t.Error("item pendente com normal e nil (global true): ForcarOCR false; quero true")
	}
}
