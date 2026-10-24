package etl

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"eprocia/internal/models"
)

type eventoGravado struct {
	nome   string
	dados  []any
	quando time.Time
}

type gravadorEventos struct {
	mu      sync.Mutex
	eventos []eventoGravado
}

func (g *gravadorEventos) emitir(_ context.Context, nome string, dados ...any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.eventos = append(g.eventos, eventoGravado{nome: nome, dados: dados, quando: time.Now()})
}

func (g *gravadorEventos) doTipo(nome string) []eventoGravado {
	g.mu.Lock()
	defer g.mu.Unlock()
	var r []eventoGravado
	for _, e := range g.eventos {
		if e.nome == nome {
			r = append(r, e)
		}
	}
	return r
}

type processarFunc = func(context.Context, string, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, error)

func novaFilaGravada(t *testing.T, processar processarFunc) (*Queue, *gravadorEventos) {
	t.Helper()
	q := NewQueue(&Pipeline{})
	t.Cleanup(q.Shutdown)
	g := &gravadorEventos{}
	q.emitir = g.emitir
	q.wailsCtx = context.Background()
	q.cfg = &models.Config{CleaningLevel: "none", OcrMode: "none"}
	q.processar = processar
	return q, g
}

func esperarFim(t *testing.T, q *Queue, nome string) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		for _, it := range q.GetItems() {
			if it.Name == nome && (it.Status == "success" || it.Status == "error" || it.Status == "cancelled") {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("item %s não terminou", nome)
}

// Cada página chamava broadcast: lista inteira (histórico incluído) mais o
// status, 200 vezes. O progresso deve sair como item único e limitado.
func TestProgresso_NaoReenviaListaNemStatusPorPagina(t *testing.T) {
	const paginas = 200
	q, g := novaFilaGravada(t, func(_ context.Context, _ string, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, error) {
		for i := 1; i <= paginas; i++ {
			onProgress(float64(i)/paginas, fmt.Sprintf("pág. %d", i), 0, 1, i, paginas)
		}
		return "saida.txt", 1, paginas, nil
	})
	for i := range 30 {
		q.items = append(q.items, &QueueItem{ID: fmt.Sprintf("h%d", i), Name: fmt.Sprintf("antigo%d.zip", i), Status: "success"})
	}

	q.AddFiles([]string{"processo.zip"})
	esperarFim(t, q, "processo.zip")
	time.Sleep(3 * intervaloProgresso)

	if n := len(g.doTipo("queue:update")); n > 4 {
		t.Errorf("queue:update (lista inteira) emitido %d vezes para %d páginas; esperado só nas mudanças de estado", n, paginas)
	}
	if n := len(g.doTipo("queue:status")); n > 3 {
		t.Errorf("queue:status emitido %d vezes; esperado só quando isPaused/isWorking mudam", n)
	}
	itens := g.doTipo("queue:item")
	if len(itens) > 5 {
		t.Errorf("queue:item emitido %d vezes em rajada; esperado no máximo 10/s", len(itens))
	}
	for _, e := range itens {
		if len(e.dados) != 1 {
			t.Fatalf("queue:item com %d argumentos", len(e.dados))
		}
		if _, ok := e.dados[0].(*QueueItem); !ok {
			t.Fatalf("queue:item deveria levar um *QueueItem, levou %T", e.dados[0])
		}
	}
	ups := g.doTipo("queue:update")
	ultimo := ups[len(ups)-1].dados[0].([]*QueueItem)
	for _, it := range ultimo {
		if it.Name == "processo.zip" && (it.Status != "success" || it.ProcessedPages != paginas) {
			t.Errorf("estado final enviado errado: %+v", it)
		}
	}
}

// O limitador coalesce as atualizações, mas a última precisa chegar mesmo
// que nenhuma outra venha depois dela.
func TestProgresso_LimitaTaxaEEnviaUltimaAtualizacao(t *testing.T) {
	const paginas = 60
	var duracao time.Duration
	q, g := novaFilaGravada(t, func(_ context.Context, _ string, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, error) {
		inicio := time.Now()
		for i := 1; i <= paginas; i++ {
			onProgress(float64(i)/paginas, "processando", 0, 1, i, paginas)
			time.Sleep(5 * time.Millisecond)
		}
		duracao = time.Since(inicio)
		time.Sleep(3 * intervaloProgresso)
		return "saida.txt", 1, paginas, nil
	})

	q.AddFiles([]string{"processo.zip"})
	esperarFim(t, q, "processo.zip")

	itens := g.doTipo("queue:item")
	if len(itens) < 2 {
		t.Fatalf("esperava ao menos 2 queue:item, obtive %d", len(itens))
	}
	maximo := int(duracao/intervaloProgresso) + 2
	if len(itens) > maximo {
		t.Errorf("queue:item emitido %d vezes em %v; limite seria %d", len(itens), duracao, maximo)
	}
	for i := 1; i < len(itens); i++ {
		if d := itens[i].quando.Sub(itens[i-1].quando); d < intervaloProgresso*8/10 {
			t.Errorf("dois queue:item com %v de intervalo; mínimo %v", d, intervaloProgresso)
		}
	}
	ultimo := itens[len(itens)-1].dados[0].(*QueueItem)
	if ultimo.ProcessedPages != paginas || ultimo.Status != "processing" {
		t.Errorf("última atualização de progresso não foi entregue: %+v", ultimo)
	}
}

func TestStatus_EmitidoSoQuandoMuda(t *testing.T) {
	q, g := novaFilaGravada(t, nil)
	q.Pause()
	q.Pause()
	q.Resume()
	q.Resume()

	st := g.doTipo("queue:status")
	if len(st) != 2 {
		t.Fatalf("esperava 2 queue:status (pausou, retomou), obtive %d", len(st))
	}
	if st[0].dados[0].(map[string]any)["isPaused"] != true || st[1].dados[0].(map[string]any)["isPaused"] != false {
		t.Errorf("sequência de status errada: %v, %v", st[0].dados, st[1].dados)
	}
}

// Item removido enquanto há envio de progresso agendado não pode reaparecer.
func TestProgresso_ItemRemovidoNaoEhEnviado(t *testing.T) {
	solta := make(chan struct{})
	reportou := make(chan struct{})
	q, g := novaFilaGravada(t, func(ctx context.Context, _ string, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, error) {
		onProgress(0.1, "a", 0, 1, 1, 10)
		onProgress(0.2, "b", 0, 1, 2, 10)
		close(reportou)
		<-solta
		return "", 0, 0, ctx.Err()
	})
	q.AddFiles([]string{"processo.zip"})
	<-reportou
	id := q.GetItems()[0].ID
	q.RemoveItem(id)
	close(solta)
	time.Sleep(3 * intervaloProgresso)

	for _, e := range g.doTipo("queue:item") {
		if it := e.dados[0].(*QueueItem); it.ProcessedPages == 2 {
			t.Errorf("progresso agendado de item removido foi enviado: %+v", it)
		}
	}
}
