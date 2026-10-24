package etl

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"eprocia/internal/models"
)

// procFalso substitui pipeline.ProcessZip: avisa em iniciou e bloqueia até o
// teste liberar o caminho (libera) ou o contexto do item ser cancelado.
type procFalso struct {
	iniciou chan string

	mu         sync.Mutex
	liberados  map[string]chan error
	emCurso    int
	maxEmCurso int
	ordem      []string
}

func novoProcFalso() *procFalso {
	return &procFalso{iniciou: make(chan string, 16), liberados: map[string]chan error{}}
}

func (p *procFalso) canal(path string) chan error {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.liberados[path]
	if !ok {
		c = make(chan error, 1)
		p.liberados[path] = c
	}
	return c
}

// libera termina o processamento de path com o erro dado (nil = sucesso).
func (p *procFalso) libera(path string, err error) { p.canal(path) <- err }

func (p *procFalso) processar(ctx context.Context, path string, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
	p.mu.Lock()
	p.emCurso++
	if p.emCurso > p.maxEmCurso {
		p.maxEmCurso = p.emCurso
	}
	p.ordem = append(p.ordem, path)
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.emCurso--
		p.mu.Unlock()
	}()

	onProgress(0.5, "Metade", 1, 2, 3, 6)
	p.iniciou <- path

	select {
	case err := <-p.canal(path):
		if err != nil {
			return "", 0, 0, 0, err
		}
		return path + ".txt", 2, 6, 0, nil
	case <-ctx.Done():
		return "", 0, 0, 0, ctx.Err()
	}
}

func (p *procFalso) esperarInicio(t *testing.T, esperado string) {
	t.Helper()
	select {
	case path := <-p.iniciou:
		if path != esperado {
			t.Fatalf("iniciou %q, esperava %q", path, esperado)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%q não começou a ser processado", esperado)
	}
}

func (p *procFalso) nenhumInicio(t *testing.T) {
	t.Helper()
	select {
	case path := <-p.iniciou:
		t.Fatalf("%q começou a ser processado e não devia", path)
	case <-time.After(100 * time.Millisecond):
	}
}

func itemPorNome(t *testing.T, q *Queue, nome string) *QueueItem {
	t.Helper()
	for _, it := range q.GetItems() {
		if it.Name == nome {
			return it
		}
	}
	t.Fatalf("item %s não está na fila", nome)
	return nil
}

func esperarOcioso(t *testing.T, q *Queue) {
	t.Helper()
	limite := time.Now().Add(2 * time.Second)
	for time.Now().Before(limite) {
		q.mu.RLock()
		trabalhando := q.isWorking
		q.mu.RUnlock()
		if !trabalhando {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("a fila não ficou ociosa")
}

func TestFila_CancelarEmProcessamentoZeraProgresso(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip"})
	p.esperarInicio(t, "a.zip")
	if it := itemPorNome(t, q, "a.zip"); it.Status != "processing" || it.Progress != 0.5 {
		t.Fatalf("antes do cancelamento: status %q, progresso %v", it.Status, it.Progress)
	}

	q.CancelItem(itemPorNome(t, q, "a.zip").ID)
	esperarFim(t, q, "a.zip")

	it := itemPorNome(t, q, "a.zip")
	if it.Status != "cancelled" || it.Progress != 0 || it.Message != "Cancelado" || it.Error != "" {
		t.Fatalf("depois do cancelamento: status %q, progresso %v, mensagem %q, erro %q",
			it.Status, it.Progress, it.Message, it.Error)
	}
}

func TestFila_CancelarPendenteImpedeProcessamento(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip"})
	p.esperarInicio(t, "a.zip")
	q.AddFiles([]string{"b.zip"})

	q.CancelItem(itemPorNome(t, q, "b.zip").ID)
	if it := itemPorNome(t, q, "b.zip"); it.Status != "cancelled" || it.Message != "Cancelado" {
		t.Fatalf("pendente cancelado: status %q, mensagem %q", it.Status, it.Message)
	}

	p.libera("a.zip", nil)
	esperarFim(t, q, "a.zip")
	p.nenhumInicio(t)
	esperarOcioso(t, q)
	if it := itemPorNome(t, q, "b.zip"); it.Status != "cancelled" {
		t.Fatalf("pendente cancelado mudou para %q", it.Status)
	}
}

func TestFila_ProcessaDoMaisAntigoAoMaisNovoUmPorVez(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)
	q.Pause()
	q.AddFiles([]string{"a.zip", "b.zip"})
	q.AddFiles([]string{"c.zip"})
	p.nenhumInicio(t)

	if itens := q.GetItems(); len(itens) != 3 || itens[0].Name != "c.zip" || itens[2].Name != "a.zip" {
		t.Fatalf("a lista devia mostrar o mais novo no topo: %v", nomes(itens))
	}

	q.Resume()
	for _, nome := range []string{"a.zip", "b.zip", "c.zip"} {
		p.esperarInicio(t, nome)
		p.nenhumInicio(t)
		p.libera(nome, nil)
		esperarFim(t, q, nome)
	}
	esperarOcioso(t, q)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.maxEmCurso != 1 {
		t.Fatalf("itens processados em paralelo: máximo %d", p.maxEmCurso)
	}
	for _, it := range q.GetItems() {
		if it.Status != "success" || it.Progress != 1 || it.OutputPath != it.Path+".txt" ||
			it.DocsCount != 2 || it.PagesCount != 6 || it.ProcessedPages != 6 || it.TotalPages != 6 {
			t.Fatalf("item %s ao terminar: %+v", it.Name, *it)
		}
	}
}

func TestFila_PausaImpedeProximoEResumeRetoma(t *testing.T) {
	p := novoProcFalso()
	q, g := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")
	q.Pause()
	if !q.IsPaused() {
		t.Fatal("IsPaused devia ser true depois de Pause")
	}

	// A pausa não interrompe o item em curso, só o encadeamento.
	p.libera("a.zip", nil)
	esperarFim(t, q, "a.zip")
	p.nenhumInicio(t)
	if it := itemPorNome(t, q, "b.zip"); it.Status != "pending" {
		t.Fatalf("b.zip devia seguir pendente durante a pausa, está %q", it.Status)
	}

	q.Resume()
	if q.IsPaused() {
		t.Fatal("IsPaused devia ser false depois de Resume")
	}
	p.esperarInicio(t, "b.zip")
	p.libera("b.zip", nil)
	esperarFim(t, q, "b.zip")

	pausou := false
	for _, e := range g.doTipo("queue:status") {
		if st, _ := e.dados[0].(map[string]any); st["isPaused"] == true {
			pausou = true
		}
	}
	if !pausou {
		t.Fatal("nenhum queue:status com isPaused=true foi emitido")
	}
}

func TestFila_PanicViraErroEFilaSegue(t *testing.T) {
	p := novoProcFalso()
	processar := func(ctx context.Context, path string, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		if path == "a.zip" {
			panic("pdf corrompido")
		}
		return p.processar(ctx, path, cfg, onProgress)
	}
	q, _ := novaFilaGravada(t, processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "b.zip")

	if it := itemPorNome(t, q, "a.zip"); it.Status != "error" || it.Error != "Panic: pdf corrompido" {
		t.Fatalf("item com panic: status %q, erro %q", it.Status, it.Error)
	}
	p.libera("b.zip", nil)
	esperarFim(t, q, "b.zip")
	if it := itemPorNome(t, q, "b.zip"); it.Status != "success" {
		t.Fatalf("item seguinte ao panic: status %q", it.Status)
	}
	esperarOcioso(t, q)
}

func TestFila_ErroDoProcessadorViraErroEFilaSegue(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")
	p.libera("a.zip", errors.New("zip inválido"))
	p.esperarInicio(t, "b.zip")

	if it := itemPorNome(t, q, "a.zip"); it.Status != "error" || it.Error != "zip inválido" || it.Message != "zip inválido" {
		t.Fatalf("item com erro: status %q, mensagem %q, erro %q", it.Status, it.Message, it.Error)
	}
	p.libera("b.zip", nil)
	esperarFim(t, q, "b.zip")
}

func TestFila_ShutdownCancelaItemEmCurso(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")

	if !q.shutdownComPrazo(2 * time.Second) {
		t.Fatal("Shutdown estourou o prazo")
	}
	if it := itemPorNome(t, q, "a.zip"); it.Status != "cancelled" {
		t.Fatalf("item em curso após Shutdown: status %q", it.Status)
	}
	if it := itemPorNome(t, q, "b.zip"); it.Status != "pending" {
		t.Fatalf("pendente após Shutdown: status %q", it.Status)
	}

	q.AddFiles([]string{"c.zip"})
	q.Resume()
	p.nenhumInicio(t)
}

func TestFila_RemoverEmProcessamentoCancelaEEncadeia(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")

	q.RemoveItem(itemPorNome(t, q, "a.zip").ID)
	if itens := q.GetItems(); len(itens) != 1 || itens[0].Name != "b.zip" {
		t.Fatalf("depois de remover a.zip: %v", nomes(itens))
	}

	// a.zip só sai de cena pelo cancelamento do contexto; b.zip vem em seguida.
	p.esperarInicio(t, "b.zip")
	p.libera("b.zip", nil)
	esperarFim(t, q, "b.zip")
	esperarOcioso(t, q)
	if itens := q.GetItems(); len(itens) != 1 {
		t.Fatalf("o item removido voltou à lista: %v", nomes(itens))
	}
}

func TestFila_RemoverPendenteEIdInexistente(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip"})
	p.esperarInicio(t, "a.zip")
	q.AddFiles([]string{"b.zip"})

	q.RemoveItem("nao-existe")
	q.CancelItem("nao-existe")
	if itens := q.GetItems(); len(itens) != 2 {
		t.Fatalf("id inexistente alterou a lista: %v", nomes(itens))
	}

	q.RemoveItem(itemPorNome(t, q, "b.zip").ID)
	p.libera("a.zip", nil)
	esperarFim(t, q, "a.zip")
	p.nenhumInicio(t)
	esperarOcioso(t, q)
}

func TestFila_ClearCancelaEmCursoEDescartaPendentes(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")

	q.Clear()
	if itens := q.GetItems(); len(itens) != 0 {
		t.Fatalf("Clear deixou itens: %v", nomes(itens))
	}
	esperarOcioso(t, q)
	p.nenhumInicio(t)
}

func TestFila_AddFilesIgnoraDuplicadoEmAndamento(t *testing.T) {
	p := novoProcFalso()
	q, _ := novaFilaGravada(t, p.processar)

	q.AddFiles([]string{"a.zip", "b.zip"})
	p.esperarInicio(t, "a.zip")
	q.AddFiles([]string{"a.zip", "b.zip", "b.zip"})
	if itens := q.GetItems(); len(itens) != 2 {
		t.Fatalf("duplicados de itens em processamento/pendentes entraram: %v", nomes(itens))
	}

	p.libera("a.zip", nil)
	p.esperarInicio(t, "b.zip")
	p.libera("b.zip", nil)
	esperarFim(t, q, "b.zip")
	esperarOcioso(t, q)

	// Concluído não conta como duplicado: o mesmo ZIP pode ser reprocessado.
	q.AddFiles([]string{"a.zip"})
	p.esperarInicio(t, "a.zip")
	if itens := q.GetItems(); len(itens) != 3 {
		t.Fatalf("reprocessamento de item concluído não entrou: %v", nomes(itens))
	}
	p.libera("a.zip", nil)
	esperarOcioso(t, q)
}

func TestFila_SetConfigChegaAoProcessador(t *testing.T) {
	recebida := make(chan *models.Config, 1)
	q, _ := novaFilaGravada(t, func(_ context.Context, _ string, cfg *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		recebida <- cfg
		return "", 0, 0, 0, nil
	})
	cfg := &models.Config{CleaningLevel: "aggressive", OcrMode: "best"}
	q.SetConfig(cfg)

	q.AddFiles([]string{"a.zip"})
	select {
	case got := <-recebida:
		if got != cfg {
			t.Fatalf("processador recebeu %+v, esperava %+v", got, cfg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("processamento não começou")
	}
	esperarFim(t, q, "a.zip")
}

func nomes(itens []*QueueItem) []string {
	r := make([]string, len(itens))
	for i, it := range itens {
		r[i] = it.Name + ":" + it.Status
	}
	return r
}
