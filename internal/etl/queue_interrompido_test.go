package etl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"eprocia/internal/models"
)

// esperarStatusID espera o item id chegar ao status.
func esperarStatusID(t *testing.T, q *Queue, id, status string) *QueueItem {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		for _, it := range q.GetItems() {
			if it.ID == id && it.Status == status {
				return it
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("item %s não chegou a %q: %+v", id, status, q.GetItems())
	return nil
}

// bloqueiaAteCancelar é um processar falso que só volta com ctx.Done().
func bloqueiaAteCancelar(ctx context.Context, _ Entrada, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
	<-ctx.Done()
	return "", 0, 0, 0, ctx.Err()
}

func conferirFinishedAt(t *testing.T, caso string, it *QueueItem) {
	t.Helper()
	if it.FinishedAt <= 0 || it.FinishedAt < it.Timestamp {
		t.Errorf("%s: FinishedAt = %d, Timestamp = %d", caso, it.FinishedAt, it.Timestamp)
	}
}

func TestFinishedAt_TodosOsFins(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			return "saida.txt", 1, 1, 0, nil
		})
		q.AddFiles([]string{`C:\x\a.zip`})
		it := esperarTerminoID(t, q, q.GetItems()[0].ID)
		if it.Status != "success" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "sucesso", it)
	})
	t.Run("erro", func(t *testing.T) {
		q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			return "", 0, 0, 0, errors.New("falhou")
		})
		q.AddFiles([]string{`C:\x\a.zip`})
		it := esperarTerminoID(t, q, q.GetItems()[0].ID)
		if it.Status != "error" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "erro", it)
	})
	t.Run("cancelamento em processamento", func(t *testing.T) {
		q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
		q.AddFiles([]string{`C:\x\a.zip`})
		id := q.GetItems()[0].ID
		esperarStatusID(t, q, id, "processing")
		q.CancelItem(id)
		it := esperarTerminoID(t, q, id)
		if it.Status != "cancelled" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "cancelamento", it)
	})
	t.Run("panic", func(t *testing.T) {
		q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			panic("quebrou")
		})
		q.AddFiles([]string{`C:\x\a.zip`})
		it := esperarTerminoID(t, q, q.GetItems()[0].ID)
		if it.Status != "error" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "panic", it)
	})
	t.Run("falha de config", func(t *testing.T) {
		q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			return "saida.txt", 1, 1, 0, nil
		})
		q.cfg = nil
		q.carregarConfig = func() (*models.Config, error) { return nil, errors.New("config ruim") }
		q.AddFiles([]string{`C:\x\a.zip`})
		it := esperarTerminoID(t, q, q.GetItems()[0].ID)
		if it.Status != "error" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "config", it)
	})
	t.Run("cancelamento de pendente", func(t *testing.T) {
		q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
		q.Reter()
		q.AddFiles([]string{`C:\x\a.zip`})
		id := q.GetItems()[0].ID
		q.CancelItem(id)
		it := q.GetItems()[0]
		if it.Status != "cancelled" {
			t.Fatalf("status %q", it.Status)
		}
		conferirFinishedAt(t, "pendente", it)
	})
}

func TestReprocessar_ZeraFinishedAt(t *testing.T) {
	q, g := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		return "saida.txt", 1, 1, 0, nil
	})
	q.existe = func(string) bool { return true }
	q.AddFiles([]string{`C:\x\a.zip`})
	id := q.GetItems()[0].ID
	esperarTerminoID(t, q, id)
	q.mu.Lock()
	q.items[0].OutputMissing = true
	q.items[0].SourceMissing = true
	q.mu.Unlock()
	if antes := q.GetItems()[0]; antes.FinishedAt == 0 || !antes.OutputMissing || !antes.SourceMissing {
		t.Fatalf("pré-condição: o item terminado deveria ter FinishedAt e as marcas: %+v", antes)
	}
	q.Reter() // a cópia enviada fica em pending
	if err := q.Reprocessar(id, "fast", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	var enviado *QueueItem
	for _, it := range ultimoUpdate(t, g) {
		if it.ID == id {
			enviado = it
		}
	}
	if enviado == nil || enviado.Status != "pending" {
		t.Fatalf("cópia enviada: %+v", enviado)
	}
	if enviado.FinishedAt != 0 || enviado.OutputMissing || enviado.SourceMissing {
		t.Errorf("Reprocessar não zerou: FinishedAt=%d OutputMissing=%v SourceMissing=%v",
			enviado.FinishedAt, enviado.OutputMissing, enviado.SourceMissing)
	}
}

func TestShutdown_ItemEmCursoViraInterrompido(t *testing.T) {
	q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
	q.AddFiles([]string{`C:\x\a.zip`})
	id := q.GetItems()[0].ID
	esperarStatusID(t, q, id, "processing")
	q.Shutdown()
	it := q.GetItems()[0]
	if it.Status != "interrupted" || it.Message != "Interrompido" {
		t.Errorf("depois do Shutdown: status %q, mensagem %q", it.Status, it.Message)
	}
	conferirFinishedAt(t, "interrompido", it)

	// Sem Shutdown, o cancelamento do usuário segue "cancelled".
	q2, _ := novaFilaGravada(t, bloqueiaAteCancelar)
	q2.AddFiles([]string{`C:\x\b.zip`})
	id2 := q2.GetItems()[0].ID
	esperarStatusID(t, q2, id2, "processing")
	q2.CancelItem(id2)
	if it := esperarTerminoID(t, q2, id2); it.Status != "cancelled" {
		t.Errorf("CancelItem: status %q", it.Status)
	}
}

func TestLimparConcluidos_TiraInterrompido(t *testing.T) {
	liberar := make(chan struct{})
	q, _ := novaFilaGravada(t, func(ctx context.Context, _ Entrada, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		select {
		case <-liberar:
		case <-ctx.Done():
			return "", 0, 0, 0, ctx.Err()
		}
		return "saida.txt", 1, 1, 0, nil
	})
	defer close(liberar)
	q.AddFiles([]string{`C:\x\proc.zip`})
	idProc := q.GetItems()[0].ID
	esperarStatusID(t, q, idProc, "processing")
	q.AddFiles([]string{`C:\x\pend.zip`})

	q.mu.Lock()
	for _, st := range []string{"success", "error", "cancelled", "interrupted"} {
		q.items = append(q.items, &QueueItem{ID: st, Name: st + ".zip", Path: `C:\x\` + st + ".zip", Kind: "zip", Status: st})
	}
	q.mu.Unlock()

	q.LimparConcluidos()
	var sobra []string
	for _, it := range q.GetItems() {
		sobra = append(sobra, it.Status)
	}
	if !reflect.DeepEqual(sobra, []string{"pending", "processing"}) {
		t.Errorf("sobraram %v, esperava [pending processing]", sobra)
	}
}

func TestReprocessar_OrigemAusente(t *testing.T) {
	q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
	path := filepath.Join(t.TempDir(), "processo.zip")
	q.mu.Lock()
	q.items = append(q.items,
		&QueueItem{ID: "outro", Name: "outro.zip", Path: `C:\x\outro.zip`, Kind: "zip", Status: "success"},
		&QueueItem{ID: "int", Name: "processo.zip", Path: path, Kind: "zip", Status: "interrupted", Message: "Interrompido", OutputPath: `C:\x\processo.txt`, FinishedAt: 1},
	)
	q.mu.Unlock()
	antes := q.GetItems()

	err := q.Reprocessar("int", "fast", "none", nil)
	if err == nil || !strings.Contains(err.Error(), "arquivo de origem não encontrado: "+filepath.Base(path)) {
		t.Fatalf("erro = %v", err)
	}
	if depois := q.GetItems(); !reflect.DeepEqual(antes, depois) {
		t.Errorf("a fila mudou:\nantes  %+v\ndepois %+v", antes, depois)
	}
}

func TestReprocessar_Interrompido(t *testing.T) {
	q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
	q.Reter()
	path := filepath.Join(t.TempDir(), "processo.zip")
	if err := os.WriteFile(path, []byte("nao e zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	q.items = append(q.items, &QueueItem{ID: "int", Name: "processo.zip", Path: path, Kind: "zip", Status: "interrupted", Message: "Interrompido", FinishedAt: 1})
	q.mu.Unlock()
	if err := q.Reprocessar("int", "fast", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	if it := q.GetItems()[0]; it.Status != "pending" || it.FinishedAt != 0 {
		t.Errorf("item: status %q, FinishedAt %d", it.Status, it.FinishedAt)
	}
}

func TestReprocessar_ExisteFalso(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.AddFiles([]string{`C:\x\a.pdf`, `C:\x\b.pdf`, `C:\x\c.pdf`})
	it := esperarTerminoID(t, q, q.GetItems()[0].ID)
	if it.Status != "success" || len(it.Paths) != 3 {
		t.Fatalf("item: %+v", it)
	}
	ausente := it.Paths[1]
	var vistos []string
	q.existe = func(c string) bool {
		vistos = append(vistos, c)
		return c != ausente
	}
	err := q.Reprocessar(it.ID, "fast", "none", nil)
	if err == nil || err.Error() != "arquivo de origem não encontrado: "+filepath.Base(ausente) {
		t.Fatalf("erro = %v", err)
	}
	if quer := []string{it.Paths[0], it.Paths[1]}; !reflect.DeepEqual(vistos, quer) {
		t.Errorf("existe viu %v, esperado %v", vistos, quer)
	}
}
