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

// recebido é o que o processar falso de processarQueEntrega viu.
type recebido struct {
	e   Entrada
	cfg *models.Config
}

func processarQueEntrega(c chan<- recebido) processarFunc {
	return func(_ context.Context, e Entrada, cfg *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		c <- recebido{e, cfg}
		return "saida.txt", 1, 1, 0, nil
	}
}

func esperarRecebido(t *testing.T, c <-chan recebido) recebido {
	t.Helper()
	select {
	case r := <-c:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("processar não foi chamado")
	}
	return recebido{}
}

func origemDe(q *Queue, id string) string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, it := range q.items {
		if it.ID == id {
			return it.origem
		}
	}
	return ""
}

const dl = `C:\Users\fulano\Downloads\`

func interrompidoZip(path string) historico.Item {
	return historico.Item{
		Name: "x.zip", Path: path, Kind: "zip", Status: "interrupted", Message: "Interrompido",
		OutputPath: dl + "x.txt", Error: "falhou antes", DocsCount: 3, PagesCount: 9,
		EstimatedTokens: 100, UnreadDocs: 1, Timestamp: 1, FinishedAt: 2,
		OcrMode: "best", CleaningLevel: "aggressive",
	}
}

func conferirReaberto(t *testing.T, it *QueueItem) {
	t.Helper()
	if it.Status != "pending" || it.Message != "Na fila..." {
		t.Errorf("status %q, mensagem %q", it.Status, it.Message)
	}
	if it.OutputPath != "" || it.Error != "" || it.DocsCount != 0 || it.PagesCount != 0 || it.EstimatedTokens != 0 || it.UnreadDocs != 0 || it.FinishedAt != 0 {
		t.Errorf("campos não zerados: %+v", it)
	}
	if it.Timestamp <= 2 {
		t.Errorf("Timestamp %d não é novo", it.Timestamp)
	}
	if it.Restaurado {
		t.Error("continua Restaurado")
	}
}

func TestAddFiles_ReaproveitaInterrompido(t *testing.T) {
	t.Run("zip", func(t *testing.T) {
		c := make(chan recebido, 1)
		q, _ := novaFilaGravada(t, processarQueEntrega(c))
		q.Reter()
		zip := dl + "x.zip"
		q.Restaurar([]historico.Item{
			{Name: "a.zip", Path: dl + "a.zip", Kind: "zip", Status: "success", OutputPath: dl + "a.txt"},
			interrompidoZip(zip),
			{Name: "b.zip", Path: dl + "b.zip", Kind: "zip", Status: "success", OutputPath: dl + "b.txt"},
		})
		id := q.GetItems()[1].ID
		if n := q.AddFilesComOrigem([]string{zip}, "monitorada"); n != 1 {
			t.Fatalf("AddFilesComOrigem devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 3 || itens[1].ID != id {
			t.Fatalf("itens: %+v", itens)
		}
		conferirReaberto(t, itens[1])
		if o := origemDe(q, id); o != "monitorada" {
			t.Errorf("origem %q", o)
		}
		if !sessaoComEntrada(q) {
			t.Error("sessaoComEntrada desligado")
		}
		q.Soltar()
		r := esperarRecebido(t, c)
		if r.cfg.OcrMode != "none" || r.cfg.CleaningLevel != "none" {
			t.Errorf("config recebida %+v, esperada a global", r.cfg)
		}
		esperarTerminoID(t, q, id)
	})
	t.Run("pdf", func(t *testing.T) {
		c := make(chan recebido, 1)
		q, _ := novaFilaGravada(t, processarQueEntrega(c))
		q.Reter()
		pdf := dl + "a.pdf"
		q.Restaurar([]historico.Item{{
			Name: "a", Path: pdf, Paths: []string{pdf}, Kind: "pdf", Status: "interrupted",
			OutputPath: dl + "a.txt", Timestamp: 1, FinishedAt: 2,
		}})
		id := q.GetItems()[0].ID
		if n := q.AddFilesComOrigem([]string{pdf}, "monitorada"); n != 1 {
			t.Fatalf("AddFilesComOrigem devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 1 || itens[0].ID != id {
			t.Fatalf("itens: %+v", itens)
		}
		conferirReaberto(t, itens[0])
		q.Soltar()
		if r := esperarRecebido(t, c); r.e.SaidaAnterior != dl+"a.txt" {
			t.Errorf("SaidaAnterior = %q", r.e.SaidaAnterior)
		}
		esperarTerminoID(t, q, id)
	})
}

func TestAddFiles_ReaproveitaMantemSaidaAnterior(t *testing.T) {
	c := make(chan recebido, 1)
	q, _ := novaFilaGravada(t, processarQueEntrega(c))
	pdf := dl + "a.pdf"
	q.mu.Lock()
	q.items = []*QueueItem{{ID: "i", Name: "a", Path: pdf, Paths: []string{pdf}, Kind: "pdf", Status: "interrupted", saidaAnterior: dl + "a_2.txt"}}
	q.mu.Unlock()
	if n := q.AddFiles([]string{pdf}); n != 1 {
		t.Fatalf("AddFiles devolveu %d", n)
	}
	if itens := q.GetItems(); len(itens) != 1 || itens[0].ID != "i" {
		t.Fatalf("itens: %+v", itens)
	}
	if r := esperarRecebido(t, c); r.e.SaidaAnterior != dl+"a_2.txt" {
		t.Errorf("SaidaAnterior = %q", r.e.SaidaAnterior)
	}
	esperarTerminoID(t, q, "i")
}

func TestAddFiles_ReaproveitaSemCaixa(t *testing.T) {
	t.Run("zip", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		q.Restaurar([]historico.Item{interrompidoZip(`c:\users\fulano\downloads\x.zip`)})
		id := q.GetItems()[0].ID
		if n := q.AddFilesComOrigem([]string{`C:\Users\fulano\Downloads\x.zip`}, "monitorada"); n != 1 {
			t.Fatalf("AddFilesComOrigem devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 1 || itens[0].ID != id {
			t.Fatalf("itens: %+v", itens)
		}
		conferirReaberto(t, itens[0])
	})
	t.Run("pdf", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		pdf := `c:\users\fulano\downloads\a.pdf`
		q.Restaurar([]historico.Item{{Name: "a", Path: pdf, Paths: []string{pdf}, Kind: "pdf", Status: "interrupted", Timestamp: 1, FinishedAt: 2}})
		id := q.GetItems()[0].ID
		if n := q.AddFiles([]string{`C:\Users\fulano\Downloads\A.pdf`}); n != 1 {
			t.Fatalf("AddFiles devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 1 || itens[0].ID != id {
			t.Fatalf("itens: %+v", itens)
		}
		conferirReaberto(t, itens[0])
	})
}

func TestAddFiles_ReaproveitadaNaoDuplica(t *testing.T) {
	t.Run("zip", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		q.Restaurar([]historico.Item{interrompidoZip(`c:\users\fulano\downloads\x.zip`)})
		zip := `C:\Users\fulano\Downloads\x.zip`
		if n := q.AddFiles([]string{zip}); n != 1 {
			t.Fatalf("primeiro AddFiles devolveu %d", n)
		}
		if n := q.AddFiles([]string{zip}); n != 0 {
			t.Errorf("segundo AddFiles devolveu %d, esperado 0", n)
		}
		if itens := q.GetItems(); len(itens) != 1 || itens[0].Status != "pending" {
			t.Fatalf("itens: %d, %+v", len(itens), itens[0])
		}
	})
	t.Run("pdf", func(t *testing.T) {
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		pdf := `c:\users\fulano\downloads\a.pdf`
		q.Restaurar([]historico.Item{{Name: "a", Path: pdf, Paths: []string{pdf}, Kind: "pdf", Status: "interrupted", Timestamp: 1, FinishedAt: 2}})
		novo := `C:\Users\fulano\Downloads\.\A.pdf`
		if n := q.AddFiles([]string{novo}); n != 1 {
			t.Fatalf("primeiro AddFiles devolveu %d", n)
		}
		if n := q.AddFiles([]string{novo}); n != 0 {
			t.Errorf("segundo AddFiles devolveu %d, esperado 0", n)
		}
		if itens := q.GetItems(); len(itens) != 1 || itens[0].Status != "pending" {
			t.Fatalf("itens: %d, %+v", len(itens), itens[0])
		}
	})
}

func TestConferirArquivos_NaoDesligaMarca(t *testing.T) {
	q, _ := novaFilaGravada(t, sucesso)
	q.Restaurar([]historico.Item{{Name: "a.zip", Path: dl + "a.zip", Kind: "zip", Status: "success", OutputPath: dl + "a.txt"}})
	q.mu.Lock()
	q.items[0].SourceMissing, q.items[0].OutputMissing = true, true
	q.mu.Unlock()
	q.ConferirArquivos(func(string) bool { return true })
	if it := q.GetItems()[0]; !it.SourceMissing || !it.OutputMissing {
		t.Errorf("marca desligada: SourceMissing %v, OutputMissing %v", it.SourceMissing, it.OutputMissing)
	}
}

func TestAddFiles_SoAInterrompida(t *testing.T) {
	q, _ := novaFilaGravada(t, sucesso)
	q.Reter()
	zip := dl + "x.zip"
	q.Restaurar([]historico.Item{
		{Name: "x.zip", Path: zip, Kind: "zip", Status: "success", Message: "Concluído", OutputPath: dl + "x.txt", DocsCount: 2, Timestamp: 5, FinishedAt: 6},
		interrompidoZip(zip),
	})
	antes := q.GetItems()
	if n := q.AddFilesComOrigem([]string{zip}, "monitorada"); n != 1 {
		t.Fatalf("AddFilesComOrigem devolveu %d", n)
	}
	itens := q.GetItems()
	if len(itens) != 2 || itens[0].ID != antes[0].ID || itens[1].ID != antes[1].ID {
		t.Fatalf("itens: %+v", itens)
	}
	if s := itens[0]; s.Status != "success" || s.OutputPath != dl+"x.txt" || s.DocsCount != 2 || s.FinishedAt != 6 || !s.Restaurado {
		t.Errorf("a linha success mudou: %+v", s)
	}
	conferirReaberto(t, itens[1])
}

func TestAddFiles_SuccessCriaNova(t *testing.T) {
	q, _ := novaFilaGravada(t, sucesso)
	q.Reter()
	zip := dl + "x.zip"
	q.Restaurar([]historico.Item{{Name: "x.zip", Path: zip, Kind: "zip", Status: "success", OutputPath: dl + "x.txt", FinishedAt: 6}})
	antigo := q.GetItems()[0].ID
	if n := q.AddFiles([]string{zip}); n != 1 {
		t.Fatalf("AddFiles devolveu %d", n)
	}
	itens := q.GetItems()
	if len(itens) != 2 || itens[1].ID != antigo || itens[0].ID == antigo {
		t.Fatalf("itens: %+v", itens)
	}
	if itens[0].Status != "pending" || itens[0].Restaurado {
		t.Errorf("linha nova: %+v", itens[0])
	}
	if s := itens[1]; s.Status != "success" || s.OutputPath != dl+"x.txt" || !s.Restaurado {
		t.Errorf("a linha success mudou: %+v", s)
	}
}

func TestAddFiles_PDFsDoLote(t *testing.T) {
	lote := []string{dl + "a.pdf", dl + "b.pdf", dl + "c.pdf"}
	restaurar := func(t *testing.T) (*Queue, string) {
		q, _ := novaFilaGravada(t, sucesso)
		q.Reter()
		q.Restaurar([]historico.Item{{Name: "a (3 PDFs)", Path: lote[0], Paths: lote, Kind: "pdf", Status: "interrupted", OutputPath: dl + "a.txt", Timestamp: 1, FinishedAt: 2}})
		return q, q.GetItems()[0].ID
	}
	t.Run("um dos PDFs cria item novo", func(t *testing.T) {
		q, id := restaurar(t)
		if n := q.AddFiles([]string{lote[1]}); n != 1 {
			t.Fatalf("AddFiles devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 2 || itens[1].ID != id || itens[0].ID == id {
			t.Fatalf("itens: %+v", itens)
		}
		if len(itens[0].Paths) != 1 || itens[0].Paths[0] != lote[1] {
			t.Errorf("item novo com Paths %v", itens[0].Paths)
		}
		if l := itens[1]; l.Status != "interrupted" || l.OutputPath != dl+"a.txt" || !l.Restaurado || l.FinishedAt != 2 {
			t.Errorf("a linha interrompida mudou: %+v", l)
		}
	})
	t.Run("os três, em outra ordem e caixa, reaproveitam", func(t *testing.T) {
		q, id := restaurar(t)
		n := q.AddFiles([]string{`C:\USERS\FULANO\DOWNLOADS\C.PDF`, `c:\users\fulano\downloads\a.pdf`, `C:\Users\fulano\Downloads\B.pdf`})
		if n != 3 {
			t.Fatalf("AddFiles devolveu %d", n)
		}
		itens := q.GetItems()
		if len(itens) != 1 || itens[0].ID != id {
			t.Fatalf("itens: %+v", itens)
		}
		conferirReaberto(t, itens[0])
	})
}

func TestConferirArquivos(t *testing.T) {
	t.Run("com restaurados", func(t *testing.T) {
		q, ev := novaFilaGravada(t, sucesso)
		q.Reter()
		presentes := map[string]bool{
			dl + "ok.zip": true, dl + "ok.txt": true,
			dl + "semtxt.zip": true,
			dl + "semzip.txt": true,
			dl + "p1.pdf":     true, dl + "lote.txt": true,
		}
		q.Restaurar([]historico.Item{
			{Name: "ok.zip", Path: dl + "ok.zip", Kind: "zip", Status: "success", OutputPath: dl + "ok.txt"},
			{Name: "semtxt.zip", Path: dl + "semtxt.zip", Kind: "zip", Status: "success", OutputPath: dl + "semtxt.txt"},
			{Name: "semzip.zip", Path: dl + "semzip.zip", Kind: "zip", Status: "success", OutputPath: dl + "semzip.txt"},
			{Name: "lote", Path: dl + "p1.pdf", Paths: []string{dl + "p1.pdf", dl + "p2.pdf"}, Kind: "pdf", Status: "success", OutputPath: dl + "lote.txt"},
		})
		q.AddFiles([]string{dl + "novo.zip"})
		antes := len(ev.doTipo("queue:update"))
		q.ConferirArquivos(func(c string) bool { return presentes[c] })
		if n := len(ev.doTipo("queue:update")) - antes; n != 1 {
			t.Errorf("queue:update emitido %d vezes, esperado 1", n)
		}
		// SourceMissing, OutputMissing esperados por nome.
		quer := map[string][2]bool{
			"ok.zip": {false, false}, "semtxt.zip": {false, true}, "semzip.zip": {true, false}, "lote": {true, false}, "novo.zip": {false, false},
		}
		for _, it := range q.GetItems() {
			esp, ok := quer[it.Name]
			if !ok {
				t.Errorf("item inesperado %s", it.Name)
				continue
			}
			if it.SourceMissing != esp[0] || it.OutputMissing != esp[1] {
				t.Errorf("%s: SourceMissing %v, OutputMissing %v; esperado %v, %v", it.Name, it.SourceMissing, it.OutputMissing, esp[0], esp[1])
			}
		}
	})
	t.Run("sem restaurados", func(t *testing.T) {
		q, ev := novaFilaGravada(t, sucesso)
		q.Reter()
		q.AddFiles([]string{dl + "novo.zip"})
		antes := len(ev.doTipo("queue:update"))
		q.ConferirArquivos(func(string) bool { return false })
		if n := len(ev.doTipo("queue:update")) - antes; n != 0 {
			t.Errorf("queue:update emitido %d vezes sem restaurados", n)
		}
		if it := q.GetItems()[0]; it.SourceMissing || it.OutputMissing {
			t.Errorf("item novo marcado: %+v", it)
		}
	})
}

func TestConferirArquivos_ItemQueVoltouAFila(t *testing.T) {
	q, _ := novaFilaGravada(t, sucesso)
	q.Reter()
	zip := dl + "x.zip"
	q.Restaurar([]historico.Item{interrompidoZip(zip)})
	id := q.GetItems()[0].ID
	var uma sync.Once
	chamou := false
	q.ConferirArquivos(func(string) bool {
		uma.Do(func() {
			chamou = true
			q.AddFilesComOrigem([]string{zip}, "monitorada")
		})
		return false
	})
	if !chamou {
		t.Fatal("ConferirArquivos não chamou existe; o teste não prova nada")
	}
	itens := q.GetItems()
	if len(itens) != 1 || itens[0].ID != id {
		t.Fatalf("itens: %+v", itens)
	}
	if it := itens[0]; it.Restaurado || it.SourceMissing || it.OutputMissing || it.Status != "pending" {
		t.Errorf("linha reaproveitada: %+v", it)
	}
}

func TestMarcarSaidaAusente(t *testing.T) {
	q, ev := novaFilaGravada(t, sucesso)
	q.Restaurar([]historico.Item{
		{Name: "a.zip", Path: dl + "a.zip", Kind: "zip", Status: "success", OutputPath: dl + "a.txt"},
		{Name: "b.zip", Path: dl + "b.zip", Kind: "zip", Status: "success", OutputPath: dl + "b.txt"},
	})
	antes := len(ev.doTipo("queue:update"))
	q.MarcarSaidaAusente(`c:\users\fulano\downloads\.\A.TXT`)
	if n := len(ev.doTipo("queue:update")) - antes; n != 1 {
		t.Errorf("queue:update emitido %d vezes, esperado 1", n)
	}
	for _, it := range q.GetItems() {
		if quer := it.Name == "a.zip"; it.OutputMissing != quer {
			t.Errorf("%s: OutputMissing %v, esperado %v", it.Name, it.OutputMissing, quer)
		}
	}
}

// Um PDF reprocessado que ainda aguardava (OutputPath vazio, saidaAnterior
// preenchida) passa pelo arquivo e volta com a mesma saidaAnterior; o
// "Processar de novo" grava por cima do TXT, sem criar <nome>_2.txt.
func TestHistorico_SaidaAnteriorIdaEVolta(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "peticao.pdf")
	txt := filepath.Join(dir, "peticao.txt")
	for _, f := range []string{pdf, txt} {
		if err := os.WriteFile(f, []byte("antigo"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	item := &QueueItem{
		ID: "i", Name: "peticao", Path: pdf, Paths: []string{pdf}, Kind: "pdf",
		Status: "pending", Message: "Na fila...", saidaAnterior: txt,
	}
	arquivo := filepath.Join(t.TempDir(), "historico.json")
	g := historico.NovoGravador(arquivo, historico.Opcoes{})
	if err := g.Fechar([]historico.Item{paraHistorico(item)}); err != nil {
		t.Fatal(err)
	}
	carregados, err := historico.Carregar(arquivo)
	if err != nil || len(carregados) != 1 {
		t.Fatalf("Carregar: %+v, %v", carregados, err)
	}

	q, _ := novaFilaGravada(t, func(_ context.Context, e Entrada, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		saidaTXT, err := gravarSaidaEntrada(e, false, "novo")
		return saidaTXT, 1, 1, 0, err
	})
	q.Restaurar(carregados)
	it := q.GetItems()[0]
	q.mu.RLock()
	anterior := q.items[0].saidaAnterior
	q.mu.RUnlock()
	if it.Status != "interrupted" || it.OutputPath != "" || anterior != txt {
		t.Fatalf("restaurado: status %q, OutputPath %q, saidaAnterior %q; want interrupted, vazio, %q", it.Status, it.OutputPath, anterior, txt)
	}

	if err := q.Reprocessar(it.ID, "none", "none", nil); err != nil {
		t.Fatal(err)
	}
	fim := esperarTerminoID(t, q, it.ID)
	if fim.Status != "success" || fim.OutputPath != txt {
		t.Fatalf("fim: status %q, OutputPath %q; want success, %q", fim.Status, fim.OutputPath, txt)
	}
	if b, _ := os.ReadFile(txt); string(b) != "novo" {
		t.Errorf("TXT não sobrescrito: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "peticao_2.txt")); err == nil {
		t.Error("criou peticao_2.txt")
	}
}
