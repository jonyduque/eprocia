package etl

import (
	"context"
	"sync"
	"testing"
	"time"

	"eprocia/internal/models"
)

// gravadorChamadas registra os caminhos passados ao processar falso; cada
// chamada espera liberar (fechado = não espera).
type gravadorChamadas struct {
	mu       sync.Mutex
	caminhos []string
	liberar  chan struct{}
}

func (g *gravadorChamadas) processar(ctx context.Context, e Entrada, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
	g.mu.Lock()
	g.caminhos = append(g.caminhos, e.Caminhos[0])
	g.mu.Unlock()
	select {
	case <-g.liberar:
	case <-ctx.Done():
		return "", 0, 0, 0, ctx.Err()
	}
	return "saida.txt", 1, 1, 0, nil
}

func (g *gravadorChamadas) chamadas() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.caminhos...)
}

func novoGravadorChamadas(liberado bool) *gravadorChamadas {
	g := &gravadorChamadas{liberar: make(chan struct{})}
	if liberado {
		close(g.liberar)
	}
	return g
}

// canalOciosa registra um aviso de fila ociosa que manda Ociosa() no canal:
// prova que o aviso chega e que a fila já se diz ociosa quando ele roda. Não
// prova que é chamado fora de q.mu (roda numa goroutine, que só esperaria).
func canalOciosa(q *Queue) chan bool {
	ch := make(chan bool, 16)
	q.SetAoFicarOciosa(func() { ch <- q.Ociosa() })
	return ch
}

func esperarAviso(t *testing.T, ch chan bool, motivo string) {
	t.Helper()
	select {
	case ociosa := <-ch:
		if !ociosa {
			t.Errorf("%s: aviso chegou com Ociosa() falso", motivo)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: o aviso de fila ociosa não chegou", motivo)
	}
}

func semAviso(t *testing.T, ch chan bool, motivo string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s: aviso de fila ociosa inesperado", motivo)
	case <-time.After(200 * time.Millisecond):
	}
}

func statusDe(t *testing.T, q *Queue, nome string) string {
	t.Helper()
	for _, it := range q.GetItems() {
		if it.Name == nome {
			return it.Status
		}
	}
	t.Fatalf("item %s não está na fila", nome)
	return ""
}

func TestQueue_RetidaAceitaMasNaoInicia(t *testing.T) {
	g := novoGravadorChamadas(true)
	q, _ := novaFilaGravada(t, g.processar)
	q.existe = func(string) bool { return true } // caminhos fictícios
	q.items = append(q.items, &QueueItem{ID: "feito", Name: "feito.zip", Path: `C:\x\feito.zip`, Status: "success"})

	q.Reter()
	if n := q.AddFiles([]string{`C:\x\a.zip`, `C:\x\b.zip`}); n != 2 {
		t.Fatalf("AddFiles aceitou %d, esperava 2", n)
	}
	if err := q.Reprocessar("feito", "none", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if c := g.chamadas(); len(c) != 0 {
		t.Fatalf("a fila retida iniciou itens: %v", c)
	}
	for _, nome := range []string{"a.zip", "b.zip", "feito.zip"} {
		if s := statusDe(t, q, nome); s != "pending" {
			t.Errorf("%s: status %q, esperava pending", nome, s)
		}
	}
}

func TestQueue_SoltarIniciaMaisAntigo(t *testing.T) {
	g := novoGravadorChamadas(true)
	q, _ := novaFilaGravada(t, g.processar)

	q.Reter()
	q.AddFiles([]string{`C:\x\primeiro.zip`})
	q.AddFiles([]string{`C:\x\segundo.zip`})
	q.Soltar()
	esperarFim(t, q, "segundo.zip")

	c := g.chamadas()
	if len(c) != 2 || c[0] != `C:\x\primeiro.zip` {
		t.Fatalf("ordem de processamento %v; esperava primeiro.zip antes", c)
	}
}

func TestQueue_ReterNaoMudaStatus(t *testing.T) {
	g := novoGravadorChamadas(true)
	q, ev := novaFilaGravada(t, g.processar)

	q.Reter()
	q.Soltar()
	if n := len(ev.doTipo("queue:status")); n != 0 {
		t.Errorf("Reter/Soltar emitiram queue:status %d vezes", n)
	}
	if q.IsPaused() {
		t.Error("Reter deixou a fila pausada")
	}

	q.Reter()
	q.AddFiles([]string{`C:\x\a.zip`})
	antes := len(ev.doTipo("queue:status"))
	q.Reter()
	if n := len(ev.doTipo("queue:status")); n != antes {
		t.Errorf("Reter emitiu queue:status")
	}
	if q.IsPaused() {
		t.Error("IsPaused() verdadeiro com a fila só retida")
	}
	q.Resume()
	time.Sleep(200 * time.Millisecond)
	if c := g.chamadas(); len(c) != 0 {
		t.Fatalf("Resume com a fila retida iniciou itens: %v", c)
	}
	if s := statusDe(t, q, "a.zip"); s != "pending" {
		t.Errorf("status %q, esperava pending", s)
	}
}

func TestQueue_AvisoOciosa(t *testing.T) {
	g := novoGravadorChamadas(false)
	q, _ := novaFilaGravada(t, g.processar)
	ch := canalOciosa(q)

	q.AddFiles([]string{`C:\x\a.zip`})
	semAviso(t, ch, "item em processamento")
	if q.Ociosa() {
		t.Error("Ociosa() verdadeiro com item em processamento")
	}
	close(g.liberar)
	esperarFim(t, q, "a.zip")
	esperarAviso(t, ch, "fim do último item")
	if !q.Ociosa() {
		t.Error("Ociosa() falso depois do fim do último item")
	}

	q.Pause()
	q.AddFiles([]string{`C:\x\b.zip`})
	semAviso(t, ch, "fila pausada com pending")
	if q.Ociosa() {
		t.Error("Ociosa() verdadeiro com a fila pausada e um pending")
	}

	q2, _ := novaFilaGravada(t, g.processar)
	q2.items = []*QueueItem{
		{ID: "1", Name: "s.zip", Status: "success"},
		{ID: "2", Name: "e.zip", Status: "error"},
		{ID: "3", Name: "c.zip", Status: "cancelled"},
	}
	if !q2.Ociosa() {
		t.Error("Ociosa() falso com só itens success/error/cancelled")
	}
}

func TestQueue_AvisoOciosaSemContexto(t *testing.T) {
	g := novoGravadorChamadas(true)
	q := NewQueue(&Pipeline{})
	t.Cleanup(q.Shutdown)
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "none"}
	q.processar = g.processar
	ch := canalOciosa(q)

	q.AddFiles([]string{`C:\x\a.zip`})
	esperarFim(t, q, "a.zip")
	esperarAviso(t, ch, "fim do último item sem wailsCtx")
}

func TestQueue_AvisoOciosaAoLimparPausada(t *testing.T) {
	g := novoGravadorChamadas(true)
	q, _ := novaFilaGravada(t, g.processar)
	q.Pause()
	q.AddFiles([]string{`C:\x\a.zip`})
	ch := canalOciosa(q)

	q.Clear()
	esperarAviso(t, ch, "Clear com a fila pausada")
}

func TestQueue_AvisoOciosaAoCancelarPendente(t *testing.T) {
	g := novoGravadorChamadas(true)
	q, _ := novaFilaGravada(t, g.processar)
	q.Pause()
	q.AddFiles([]string{`C:\x\a.zip`})
	ch := canalOciosa(q)

	q.CancelItem(q.GetItems()[0].ID)
	esperarAviso(t, ch, "CancelItem do único pending")
}

// Clear ou RemoveItem do item em processamento tiram-no da lista enquanto o
// processItem ainda roda: a fila só fica ociosa quando ele termina.
func TestQueue_NaoOciosaComItemEmCurso(t *testing.T) {
	for _, caso := range []struct {
		nome  string
		tirar func(q *Queue)
	}{
		{"Clear", func(q *Queue) { q.Clear() }},
		{"RemoveItem", func(q *Queue) { q.RemoveItem(q.GetItems()[0].ID) }},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			liberar := make(chan struct{})
			chamou := make(chan struct{})
			q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
				close(chamou)
				<-liberar // ignora o cancelamento: simula o item que demora a voltar
				return "saida.txt", 1, 1, 0, nil
			})
			ch := canalOciosa(q)
			q.AddFiles([]string{`C:\x\a.zip`})
			<-chamou

			caso.tirar(q)
			semAviso(t, ch, caso.nome+" com processar bloqueado")
			if q.Ociosa() {
				t.Errorf("Ociosa() verdadeiro depois de %s com o item ainda em curso", caso.nome)
			}
			close(liberar)
			esperarAviso(t, ch, "fim do item tirado da lista")
		})
	}
}
