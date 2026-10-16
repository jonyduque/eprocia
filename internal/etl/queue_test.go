package etl

import (
	"context"
	"sync/atomic"
	"testing"

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
		for i := 0; i < 200; i++ {
			q.SetContext(nil)
		}
	}()
	for i := 0; i < 200; i++ {
		q.Pause()
		q.Resume()
	}
	<-done
}
