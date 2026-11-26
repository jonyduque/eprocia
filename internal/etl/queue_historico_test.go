package etl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"eprocia/internal/historico"
	"eprocia/internal/models"
)

// gravadorFalso guarda cada lista recebida, por método.
type gravadorFalso struct {
	mu      sync.Mutex
	agendar [][]historico.Item
	gravar  [][]historico.Item
	fechar  [][]historico.Item
}

func (g *gravadorFalso) Agendar(itens []historico.Item) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agendar = append(g.agendar, itens)
}

func (g *gravadorFalso) GravarAgora(itens []historico.Item) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gravar = append(g.gravar, itens)
	return nil
}

func (g *gravadorFalso) Fechar(itens []historico.Item) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fechar = append(g.fechar, itens)
	return nil
}

func (g *gravadorFalso) nAgendar() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.agendar)
}

func (g *gravadorFalso) ultimoAgendar(t *testing.T) []historico.Item {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.agendar) == 0 {
		t.Fatal("nenhum Agendar recebido")
	}
	return g.agendar[len(g.agendar)-1]
}

func (g *gravadorFalso) todas() [][]historico.Item {
	g.mu.Lock()
	defer g.mu.Unlock()
	var r [][]historico.Item
	r = append(r, g.agendar...)
	r = append(r, g.gravar...)
	r = append(r, g.fechar...)
	return r
}

func nomesHist(itens []historico.Item) []string {
	r := make([]string, len(itens))
	for i, it := range itens {
		r[i] = it.Name
	}
	return r
}

func sucesso(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
	return "saida.txt", 1, 1, 0, nil
}

// doisRestauraveis são dois ZIPs concluídos da sessão anterior.
func doisRestauraveis() []historico.Item {
	return []historico.Item{
		{Name: "a.zip", Path: `C:\Users\fulano\Downloads\a.zip`, Kind: "zip", Status: "success", Message: "Concluído", OutputPath: `C:\Users\fulano\Downloads\a.txt`, Timestamp: 1, FinishedAt: 2},
		{Name: "b.zip", Path: `C:\Users\fulano\Downloads\b.zip`, Kind: "zip", Status: "error", Message: "falhou", Error: "falhou", Timestamp: 1, FinishedAt: 2},
	}
}

func idPorNome(t *testing.T, q *Queue, nome string) string {
	t.Helper()
	for _, it := range q.GetItems() {
		if it.Name == nome {
			return it.ID
		}
	}
	t.Fatalf("nenhum item %q", nome)
	return ""
}

func sessaoComEntrada(q *Queue) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.sessaoComEntrada
}

func TestHistorico_BroadcastAgenda(t *testing.T) {
	t.Run("com contexto, progresso não agenda", func(t *testing.T) {
		g := &gravadorFalso{}
		var antes, depois atomic.Int64
		q, ev := novaFilaGravada(t, func(_ context.Context, _ Entrada, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
			antes.Store(int64(g.nAgendar()))
			onProgress(0.5, "pág. 1", 0, 1, 1, 2)
			depois.Store(int64(g.nAgendar()))
			return "saida.txt", 1, 2, 0, nil
		})
		q.SetGravador(g)
		q.AddFiles([]string{`C:\Users\fulano\Downloads\a.zip`})
		esperarTerminoID(t, q, q.GetItems()[0].ID)
		if antes.Load() == 0 {
			t.Fatal("AddFiles não gerou Agendar")
		}
		if depois.Load() != antes.Load() {
			t.Errorf("onProgress gerou Agendar: %d antes, %d depois", antes.Load(), depois.Load())
		}
		if len(ev.doTipo("queue:item")) == 0 {
			t.Error("o progresso falso não emitiu queue:item; o teste não prova nada")
		}
	})
	t.Run("sem contexto do Wails", func(t *testing.T) {
		g := &gravadorFalso{}
		q, _ := novaFilaGravada(t, sucesso)
		q.wailsCtx = nil
		q.Reter()
		q.SetGravador(g)
		q.AddFiles([]string{`C:\Users\fulano\Downloads\a.zip`})
		if g.nAgendar() == 0 {
			t.Fatal("com wailsCtx nil o broadcast não agendou")
		}
		if l := g.ultimoAgendar(t); len(l) != 1 || l[0].Name != "a.zip" {
			t.Errorf("lista agendada %v", nomesHist(l))
		}
	})
	t.Run("sem gravador", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.AddFiles([]string{`C:\Users\fulano\Downloads\a.zip`})
		it := esperarTerminoID(t, q, q.GetItems()[0].ID)
		if it.Status != "success" {
			t.Fatalf("status %q", it.Status)
		}
		q.AgendarHistorico()
		if err := q.GravarHistoricoAgora(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHistorico_ShutdownGravaInterrompido(t *testing.T) {
	g := &gravadorFalso{}
	q, _ := novaFilaGravada(t, bloqueiaAteCancelar)
	q.SetGravador(g)
	q.AddFiles([]string{`C:\Users\fulano\Downloads\a.zip`})
	esperarStatusID(t, q, idPorNome(t, q, "a.zip"), "processing")
	q.AddFiles([]string{`C:\Users\fulano\Downloads\b.zip`})
	q.Shutdown()

	// Na fila, b.zip continua pending: só a lista gravada o marca interrupted.
	for _, it := range q.GetItems() {
		if it.Name == "b.zip" && it.Status != "pending" {
			t.Errorf("b.zip na fila com status %q, esperado pending", it.Status)
		}
	}
	idPorNome(t, q, "b.zip")

	g.mu.Lock()
	fechar := append([][]historico.Item(nil), g.fechar...)
	g.mu.Unlock()
	if len(fechar) == 0 {
		t.Fatal("Shutdown não chamou Fechar")
	}
	l := fechar[0]
	if len(l) != 2 {
		t.Fatalf("Fechar com %v", nomesHist(l))
	}
	for _, it := range l {
		if it.Status != "interrupted" {
			t.Errorf("%s: status %q no Fechar, esperado interrupted", it.Name, it.Status)
		}
	}
	for _, lista := range g.todas() {
		for _, it := range lista {
			if it.Status == "cancelled" {
				t.Errorf("lista gravada com %s cancelado", it.Name)
			}
		}
	}
}

func lerHistorico(t *testing.T, arquivo string) []byte {
	t.Helper()
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	return dados
}

func TestHistorico_BroadcastTardioNaoRegrava(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "historico.json")
	g := historico.NovoGravador(arquivo, historico.Opcoes{Atraso: 20 * time.Millisecond})
	terminou := make(chan struct{})
	q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		defer close(terminou)
		time.Sleep(300 * time.Millisecond)
		return "saida.txt", 1, 1, 0, nil
	})
	q.SetGravador(g)
	q.AddFiles([]string{`C:\Users\fulano\Downloads\a.zip`})
	esperarStatusID(t, q, idPorNome(t, q, "a.zip"), "processing")
	if q.shutdownComPrazo(50 * time.Millisecond) {
		t.Fatal("o item terminou dentro do prazo; o teste não prova nada")
	}
	noFechar := lerHistorico(t, arquivo)
	var h struct {
		Itens []historico.Item `json:"itens"`
	}
	if err := json.Unmarshal(noFechar, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Itens) != 1 || h.Itens[0].Status != "interrupted" {
		t.Fatalf("arquivo do Fechar: %s", noFechar)
	}

	<-terminou
	q.wg.Wait()
	time.Sleep(100 * time.Millisecond)
	if it := q.GetItems()[0]; it.Status != "success" {
		t.Fatalf("o processItem tardio deveria ter concluído: %q", it.Status)
	}
	if depois := lerHistorico(t, arquivo); string(depois) != string(noFechar) {
		t.Errorf("o broadcast tardio regravou o arquivo:\nantes %s\ndepois %s", noFechar, depois)
	}
}

func TestRestaurar_Converte(t *testing.T) {
	var chamadas atomic.Int64
	q, ev := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		chamadas.Add(1)
		return "saida.txt", 1, 1, 0, nil
	})
	q.items = []*QueueItem{{ID: "ja", Name: "ja.zip", Path: `C:\x\ja.zip`, Kind: "zip", Status: "success"}}
	q.Restaurar([]historico.Item{
		{Name: "p.zip", Path: `C:\x\p.zip`, Kind: "zip", Status: "pending", Message: "Na fila..."},
		{Name: "r.zip", Path: `C:\x\r.zip`, Kind: "zip", Status: "processing", Message: "Iniciando..."},
		{Name: "s.zip", Path: `C:\x\s.zip`, Kind: "zip", Status: "success", Message: "Concluído", OutputPath: `C:\x\s.txt`, DocsCount: 3, PagesCount: 7},
	})
	time.Sleep(200 * time.Millisecond)
	if n := chamadas.Load(); n != 0 {
		t.Errorf("processar chamado %d vezes", n)
	}
	itens := q.GetItems()
	if len(itens) != 4 || itens[0].ID != "ja" {
		t.Fatalf("itens: %+v", itens)
	}
	esperados := []struct{ nome, status string }{{"p.zip", "interrupted"}, {"r.zip", "interrupted"}, {"s.zip", "success"}}
	ids := map[string]bool{"ja": true}
	for i, e := range esperados {
		it := itens[i+1]
		if it.Name != e.nome || it.Status != e.status || !it.Restaurado {
			t.Errorf("item %d: %s %q restaurado=%v, esperado %s %q", i+1, it.Name, it.Status, it.Restaurado, e.nome, e.status)
		}
		if it.ID == "" || ids[it.ID] {
			t.Errorf("item %d: ID %q vazio ou repetido", i+1, it.ID)
		}
		ids[it.ID] = true
		if it.Kind != "zip" {
			t.Errorf("item %d: Kind %q", i+1, it.Kind)
		}
	}
	if s := itens[3]; s.Progress != 1 || s.ProcessedDocs != 3 || s.TotalDocs != 3 || s.ProcessedPages != 7 || s.TotalPages != 7 {
		t.Errorf("s.zip: Progress %v, docs %d/%d, páginas %d/%d; esperado 1, 3/3, 7/7", s.Progress, s.ProcessedDocs, s.TotalDocs, s.ProcessedPages, s.TotalPages)
	}
	if itens[1].Message != "Interrompido" || itens[2].Message != "Interrompido" {
		t.Errorf("mensagens %q, %q", itens[1].Message, itens[2].Message)
	}
	if n := len(ev.doTipo("queue:update")); n != 1 {
		t.Errorf("queue:update emitido %d vezes, esperado 1", n)
	}
	if sessaoComEntrada(q) {
		t.Error("Restaurar ligou sessaoComEntrada")
	}
}

func TestRestaurar_KindVazioViraZip(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "historico.json")
	conteudo := `{"versao":1,"itens":[{"name":"a.zip","path":"C:\\Users\\fulano\\Downloads\\a.zip","status":"success","message":"Concluído","output_path":"C:\\Users\\fulano\\Downloads\\a.txt"}]}`
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	carregados, err := historico.Carregar(arquivo)
	if err != nil || len(carregados) != 1 {
		t.Fatalf("Carregar: %v, %d itens", err, len(carregados))
	}
	q, _ := novaFilaGravada(t, sucesso)
	q.Restaurar(carregados)
	itens := q.GetItems()
	if len(itens) != 1 || itens[0].Kind != "zip" {
		t.Fatalf("itens: %+v", itens)
	}

	// Sem passar por Carregar, a conversão também não deixa Kind vazio.
	q2, _ := novaFilaGravada(t, sucesso)
	q2.Restaurar([]historico.Item{{Name: "b.pdf", Path: `C:\x\b.pdf`, Status: "error"}, {Name: "c.zip", Path: `C:\x\c.zip`, Status: "error"}})
	itens = q2.GetItems()
	if len(itens) != 2 || itens[0].Kind != "pdf" || itens[1].Kind != "zip" {
		t.Fatalf("itens sem kind: %+v %+v", itens[0], itens[1])
	}
	if len(itens[0].Paths) != 1 || itens[0].Paths[0] != `C:\x\b.pdf` {
		t.Errorf("PDF sem paths: Paths = %v", itens[0].Paths)
	}
}

func TestRestaurar_SaidaAnterior(t *testing.T) {
	recebida := make(chan string, 1)
	q, _ := novaFilaGravada(t, func(_ context.Context, e Entrada, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		recebida <- e.SaidaAnterior
		return e.SaidaAnterior, 1, 1, 0, nil
	})
	q.existe = func(string) bool { return true }
	q.Restaurar([]historico.Item{{
		Name: "a", Path: `C:\Users\fulano\Downloads\a.pdf`, Paths: []string{`C:\Users\fulano\Downloads\a.pdf`},
		Kind: "pdf", Status: "success", Message: "Concluído", OutputPath: `C:\Users\fulano\Downloads\a.txt`,
	}})
	id := q.GetItems()[0].ID
	if err := q.Reprocessar(id, "none", "none", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-recebida:
		if s != `C:\Users\fulano\Downloads\a.txt` {
			t.Errorf("SaidaAnterior = %q", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("processar não foi chamado")
	}
	esperarTerminoID(t, q, id)
}

func TestListaGravada_SemEntrada(t *testing.T) {
	g := &gravadorFalso{}
	q, _ := novaFilaGravada(t, sucesso)
	q.Restaurar(doisRestauraveis())
	q.SetGravador(g)
	q.RemoveItem(idPorNome(t, q, "a.zip"))
	l := g.ultimoAgendar(t)
	if len(l) != 1 || l[0].Name != "b.zip" {
		t.Errorf("lista agendada %v, esperado [b.zip]", nomesHist(l))
	}
}

func TestListaGravada_ComEntrada(t *testing.T) {
	g := &gravadorFalso{}
	q, _ := novaFilaGravada(t, sucesso)
	q.Restaurar(doisRestauraveis())
	q.SetGravador(g)
	q.Reter()
	q.AddFiles([]string{`C:\Users\fulano\Downloads\novo.zip`})
	l := g.ultimoAgendar(t)
	if len(l) != 1 || l[0].Name != "novo.zip" {
		t.Fatalf("lista agendada %v, esperado [novo.zip]", nomesHist(l))
	}
	if l[0].Kind != "zip" || l[0].Path != `C:\Users\fulano\Downloads\novo.zip` {
		t.Errorf("item gravado %+v", l[0])
	}
	q.RemoveItem(idPorNome(t, q, "novo.zip"))
	l = g.ultimoAgendar(t)
	if l == nil || len(l) != 0 {
		t.Errorf("depois de remover o novo: %#v, esperado [] não nil", l)
	}
}

func TestListaGravada_SinalLiga(t *testing.T) {
	t.Run("Reprocessar de restaurado", func(t *testing.T) {
		g := &gravadorFalso{}
		q, _ := novaFilaGravada(t, sucesso)
		q.existe = func(string) bool { return true }
		q.Restaurar(doisRestauraveis())
		q.SetGravador(g)
		q.Reter()
		id := idPorNome(t, q, "a.zip")
		if err := q.Reprocessar(id, "none", "none", nil); err != nil {
			t.Fatal(err)
		}
		if !sessaoComEntrada(q) {
			t.Error("sinal desligado")
		}
		for _, it := range q.GetItems() {
			if it.ID == id && it.Restaurado {
				t.Error("o item reprocessado continua Restaurado")
			}
		}
		l := g.ultimoAgendar(t)
		if len(l) != 1 || l[0].Name != "a.zip" {
			t.Errorf("lista agendada %v, esperado [a.zip]", nomesHist(l))
		}
	})
	t.Run("Reprocessar de item da sessão", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.existe = func(string) bool { return true }
		q.Reter()
		q.items = []*QueueItem{{ID: "x", Name: "x.zip", Path: `C:\x\x.zip`, Kind: "zip", Status: "success"}}
		if err := q.Reprocessar("x", "none", "none", nil); err != nil {
			t.Fatal(err)
		}
		if !sessaoComEntrada(q) {
			t.Error("sinal desligado")
		}
	})
	t.Run("ReprocessarMonitorado de restaurado", func(t *testing.T) {
		g := &gravadorFalso{}
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		pdf := `C:\Users\fulano\Downloads\a.pdf`
		q.Restaurar([]historico.Item{{Name: "a", Path: pdf, Paths: []string{pdf}, Kind: "pdf", Status: "success", OutputPath: `C:\Users\fulano\Downloads\a.txt`}})
		q.SetGravador(g)
		if !q.ReprocessarMonitorado(`c:\users\fulano\downloads\a.pdf`) {
			t.Fatal("ReprocessarMonitorado devolveu false")
		}
		if !sessaoComEntrada(q) {
			t.Error("sinal desligado")
		}
		if q.GetItems()[0].Restaurado {
			t.Error("o item continua Restaurado")
		}
		if l := g.ultimoAgendar(t); len(l) != 1 || l[0].Name != "a" {
			t.Errorf("lista agendada %v", nomesHist(l))
		}
	})
}

func TestListaGravada_LimparTudoSemEntrada(t *testing.T) {
	g := &gravadorFalso{}
	q, _ := novaFilaGravada(t, sucesso)
	q.Restaurar(doisRestauraveis())
	q.SetGravador(g)
	q.Clear()
	l := g.ultimoAgendar(t)
	if l == nil || len(l) != 0 {
		t.Errorf("depois de Limpar tudo: %#v, esperado [] não nil", l)
	}
}

func TestFecharHistorico_SemGravador(t *testing.T) {
	q, _ := novaFilaGravada(t, sucesso)
	if err := q.FecharHistorico(); err != nil {
		t.Errorf("FecharHistorico sem gravador: %v", err)
	}
}
