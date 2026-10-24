package etl

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"eprocia/internal/models"
)

// ctxObservado registra quando um filho criado com context.WithCancel é
// liberado: o runtime de context chama a função stop devolvida por AfterFunc.
type ctxObservado struct {
	context.Context
	done     chan struct{}
	liberado atomic.Bool
}

func (c *ctxObservado) Done() <-chan struct{} { return c.done }

func (c *ctxObservado) AfterFunc(f func()) func() bool {
	return func() bool {
		c.liberado.Store(true)
		return true
	}
}

func TestProcessItem_PanicLiberaContexto(t *testing.T) {
	base := &ctxObservado{Context: context.Background(), done: make(chan struct{})}
	q := &Queue{
		items:   make([]*QueueItem, 0),
		baseCtx: base,
		cfg:     &models.Config{CleaningLevel: "none", OcrMode: "none"},
	}
	// pipeline nil provoca panic dentro de ProcessZip.
	item := &QueueItem{ID: "1", Name: "processo.zip", Path: "processo.zip", Status: "pending"}
	q.items = append(q.items, item)

	q.processItem(item)

	if item.Status != "error" {
		t.Fatalf("esperava status error após panic, obtive %q", item.Status)
	}
	if !base.liberado.Load() {
		t.Errorf("contexto do item não foi cancelado após panic (WithCancel vazou)")
	}
}

// Roda com -race: SetContext concorrente com broadcast (via Pause/Resume)
// não pode acessar wailsCtx sem q.mu.
func TestSetContext_ConcorrenteComBroadcast(t *testing.T) {
	q := NewQueue(&Pipeline{})
	defer q.Shutdown()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			q.SetContext(nil)
		}
	}()
	for range 200 {
		q.Pause()
		q.Resume()
	}
	<-done
}

// Shutdown precisa esperar o processItem em curso terminar (e rodar seus
// defers de limpeza) antes de devolver, senão main encerra no meio do OCR.
func TestShutdown_EsperaItemEmProcessamento(t *testing.T) {
	q := NewQueue(&Pipeline{})
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "none"}

	iniciou := make(chan struct{})
	var limpou atomic.Bool
	q.processar = func(ctx context.Context, _ string, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, error) {
		close(iniciou)
		<-ctx.Done()
		time.Sleep(200 * time.Millisecond)
		limpou.Store(true)
		return "", 0, 0, ctx.Err()
	}

	q.AddFiles([]string{"processo.zip", "outro.zip"})
	select {
	case <-iniciou:
	case <-time.After(2 * time.Second):
		t.Fatal("processamento não começou")
	}

	if !q.shutdownComPrazo(2 * time.Second) {
		t.Fatal("Shutdown estourou o prazo")
	}
	if !limpou.Load() {
		t.Fatal("Shutdown devolveu antes de o item em processamento terminar")
	}
	for _, it := range q.GetItems() {
		if it.Name == "outro.zip" && it.Status != "pending" {
			t.Fatalf("item pendente foi iniciado após Shutdown: status %q", it.Status)
		}
	}
}

func TestShutdown_DevolveAoEstourarPrazo(t *testing.T) {
	q := NewQueue(&Pipeline{})
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "none"}

	iniciou := make(chan struct{})
	solta := make(chan struct{})
	defer close(solta)
	q.processar = func(context.Context, string, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, error) {
		close(iniciou)
		<-solta
		return "", 0, 0, nil
	}

	q.AddFiles([]string{"processo.zip"})
	<-iniciou

	inicio := time.Now()
	if q.shutdownComPrazo(100 * time.Millisecond) {
		t.Fatal("shutdownComPrazo devolveu true com item ainda rodando")
	}
	if d := time.Since(inicio); d > time.Second {
		t.Fatalf("shutdownComPrazo demorou %v, prazo era 100ms", d)
	}
}
