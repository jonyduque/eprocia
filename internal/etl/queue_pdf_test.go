package etl

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"eprocia/internal/models"
)

const cnjTeste = "5000000-00.2026.8.26.0001"

// chamadaGravada é uma chamada ao processar falso.
type chamadaGravada struct {
	e   Entrada
	cfg models.Config
}

// procEntradas grava cada Entrada recebida e devolve a saída de resp (por
// Caminhos[0]) ou Base do primeiro caminho.
type procEntradas struct {
	mu       sync.Mutex
	chamadas []chamadaGravada
	resp     func(n int, e Entrada) (string, error)
}

func (p *procEntradas) processar(ctx context.Context, e Entrada, cfg *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
	p.mu.Lock()
	e.Caminhos = slices.Clone(e.Caminhos)
	p.chamadas = append(p.chamadas, chamadaGravada{e: e, cfg: *cfg})
	n := len(p.chamadas)
	resp := p.resp
	p.mu.Unlock()
	if resp == nil {
		return strings.TrimSuffix(e.Caminhos[0], filepath.Ext(e.Caminhos[0])) + ".txt", 1, 1, 1, nil
	}
	out, err := resp(n, e)
	if err != nil {
		if ctx.Err() != nil {
			return "", 0, 0, 0, ctx.Err()
		}
		return "", 0, 0, 0, err
	}
	return out, 1, 1, 1, nil
}

func (p *procEntradas) lista() []chamadaGravada {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.chamadas)
}

// itemComPath devolve a cópia do item cujo Path é path.
func itemComPath(t *testing.T, q *Queue, path string) *QueueItem {
	t.Helper()
	for _, it := range q.GetItems() {
		if it.Path == path {
			return it
		}
	}
	t.Fatalf("nenhum item com Path %q", path)
	return nil
}

// esperarTerminoID espera o item id sair de pending/processing.
func esperarTerminoID(t *testing.T, q *Queue, id string) *QueueItem {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		for _, it := range q.GetItems() {
			if it.ID == id && it.Status != "pending" && it.Status != "processing" {
				esperarOcioso(t, q)
				return it
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("item %s não terminou", id)
	return nil
}

func ultimoUpdate(t *testing.T, g *gravadorEventos) []*QueueItem {
	t.Helper()
	evs := g.doTipo("queue:update")
	if len(evs) == 0 {
		t.Fatal("nenhum queue:update")
	}
	return evs[len(evs)-1].dados[0].([]*QueueItem)
}

// esperarUpdateFinal espera o último queue:update trazer o item id já
// terminado: o broadcast sai depois do Unlock, então o estado final aparece
// em GetItems antes de aparecer no registrador.
func esperarUpdateFinal(t *testing.T, g *gravadorEventos, id string) *QueueItem {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if evs := g.doTipo("queue:update"); len(evs) > 0 {
			for _, it := range evs[len(evs)-1].dados[0].([]*QueueItem) {
				if it.ID == id && it.Status != "pending" && it.Status != "processing" {
					return it
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("o último queue:update não trouxe o item %s terminado", id)
	return nil
}

func TestNomeItemPDF(t *testing.T) {
	if got := nomeItemPDF("processo", 1); got != "processo" {
		t.Errorf("nomeItemPDF(processo, 1) = %q", got)
	}
	if got := nomeItemPDF(cnjTeste, 3); got != cnjTeste+" (3 PDFs)" {
		t.Errorf("nomeItemPDF(cnj, 3) = %q", got)
	}
}

func TestAddFiles_RecusaExtensoes(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	if n := q.AddFiles([]string{`C:\x\a.txt`, `C:\x\b.exe`, `C:\x\c.docx`}); n != 0 {
		t.Errorf("AddFiles de extensões recusadas = %d, esperava 0", n)
	}
	if itens := q.GetItems(); len(itens) != 0 {
		t.Fatalf("criou %d itens com extensões recusadas", len(itens))
	}
	if n := q.AddFiles([]string{`C:\x\d.pdf`, `C:\x\E.PDF`}); n != 2 {
		t.Errorf("AddFiles de dois PDFs = %d, esperava 2", n)
	}
	itens := q.GetItems()
	if len(itens) != 1 || itens[0].Kind != "pdf" || len(itens[0].Paths) != 2 {
		t.Fatalf("itens = %+v, esperava um item de PDF com dois caminhos", itens)
	}
}

func TestAddFiles_DoisZipsETresPDFs(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	dir := `C:\Users\fulano\Downloads`
	p12 := filepath.Join(dir, cnjTeste+" - Evento 12 - SENT1.pdf")
	p3 := filepath.Join(dir, cnjTeste+" - Evento 3 - INIC1.pdf")
	p7 := filepath.Join(dir, cnjTeste+" - Evento 7 - LAUDO1.pdf")
	za, zb := filepath.Join(dir, "a.zip"), filepath.Join(dir, "b.zip")
	if n := q.AddFiles([]string{za, p12, zb, p3, p7}); n != 5 {
		t.Errorf("AddFiles = %d, esperava 5", n)
	}
	itens := q.GetItems()
	if len(itens) != 3 {
		t.Fatalf("%d itens, esperava 3", len(itens))
	}
	pdf := itens[0]
	if pdf.Kind != "pdf" {
		t.Fatalf("item do topo Kind = %q, esperava pdf (entra depois dos ZIPs)", pdf.Kind)
	}
	if want := []string{p3, p7, p12}; !slices.Equal(pdf.Paths, want) {
		t.Errorf("Paths = %v, esperava %v", pdf.Paths, want)
	}
	if pdf.Path != pdf.Paths[0] {
		t.Errorf("Path = %q, esperava Paths[0] %q", pdf.Path, pdf.Paths[0])
	}
	for _, it := range itens[1:] {
		if it.Kind != "zip" || it.Paths != nil {
			t.Errorf("item ZIP %q: Kind %q, Paths %v", it.Path, it.Kind, it.Paths)
		}
	}
}

func TestAddFiles_PDFJaPendenteSaiDoItemNovo(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	if n := q.AddFiles([]string{`C:\x\a.pdf`}); n != 1 {
		t.Fatalf("primeiro AddFiles = %d", n)
	}
	if n := q.AddFiles([]string{`C:\x\a.pdf`, `C:\x\b.pdf`}); n != 1 {
		t.Errorf("AddFiles com a.pdf já pendente = %d, esperava 1", n)
	}
	itens := q.GetItems()
	if len(itens) != 2 || !slices.Equal(itens[0].Paths, []string{`C:\x\b.pdf`}) {
		t.Fatalf("itens = %+v, esperava item novo só com b.pdf", itens)
	}
	if n := q.AddFiles([]string{`C:\X\A.PDF`}); n != 0 {
		t.Errorf("AddFiles de A.PDF (outra caixa) = %d, esperava 0", n)
	}
	if itens := q.GetItems(); len(itens) != 2 {
		t.Errorf("%d itens depois de A.PDF, esperava 2", len(itens))
	}
}

func TestAddFiles_MesmoPDFRepetidoNaChamada(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	if n := q.AddFiles([]string{`C:\x\a.pdf`, `C:\x\a.pdf`, `C:\x\A.pdf`}); n != 1 {
		t.Errorf("AddFiles = %d, esperava 1", n)
	}
	itens := q.GetItems()
	if len(itens) != 1 || len(itens[0].Paths) != 1 {
		t.Fatalf("itens = %+v, esperava um item com um caminho", itens)
	}
}

func TestAddFiles_NomeProvisorio(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	dir := `C:\Users\fulano\Downloads`
	q.AddFiles([]string{
		filepath.Join(dir, cnjTeste+" - Evento 3 - INIC1.pdf"),
		filepath.Join(dir, cnjTeste+" - Evento 7 - LAUDO1.pdf"),
		filepath.Join(dir, cnjTeste+" - Evento 12 - SENT1.pdf"),
	})
	q.AddFiles([]string{filepath.Join(dir, "processo.pdf")})
	itens := q.GetItems()
	if got := itens[1].Name; got != cnjTeste+" (3 PDFs)" {
		t.Errorf("Name do lote = %q", got)
	}
	if got := itens[0].Name; got != "processo" {
		t.Errorf("Name de um PDF = %q, esperava processo", got)
	}
}

func TestCopiarItem_KindEPaths(t *testing.T) {
	q, g := novaFilaGravada(t, (&procEntradas{}).processar)
	q.Reter()
	q.AddFiles([]string{`C:\x\a.pdf`, `C:\x\b.pdf`})
	q.AddFiles([]string{`C:\x\z.zip`})
	copia := q.GetItems()[1]
	if copia.Kind != "pdf" || !slices.Equal(copia.Paths, []string{`C:\x\a.pdf`, `C:\x\b.pdf`}) {
		t.Fatalf("cópia = Kind %q, Paths %v", copia.Kind, copia.Paths)
	}
	copia.Paths[0] = `C:\x\outro.pdf`
	if got := q.GetItems()[1].Paths[0]; got != `C:\x\a.pdf` {
		t.Errorf("mudar a cópia mudou o item: Paths[0] = %q", got)
	}
	enviados := ultimoUpdate(t, g)
	if enviados[1].Kind != "pdf" || len(enviados[1].Paths) != 2 || enviados[0].Kind != "zip" {
		t.Errorf("queue:update sem Kind/Paths: %+v %+v", enviados[0], enviados[1])
	}
	js, err := json.Marshal(q.GetItems()[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"kind":"pdf"`) || !strings.Contains(string(js), `"paths":[`) {
		t.Errorf("JSON do item de PDF sem kind/paths: %s", js)
	}
	js, _ = json.Marshal(q.GetItems()[0])
	if !strings.Contains(string(js), `"kind":"zip"`) || strings.Contains(string(js), `"paths"`) {
		t.Errorf("JSON do item ZIP: %s", js)
	}
}

func TestProcessItem_EntradaPassadaAoProcessar(t *testing.T) {
	p := &procEntradas{}
	q, _ := novaFilaGravada(t, p.processar)
	q.AddFiles([]string{`C:\x\a.zip`})
	esperarTerminoID(t, q, itemComPath(t, q, `C:\x\a.zip`).ID)
	q.AddFiles([]string{`C:\x\p2.pdf`, `C:\x\p1.pdf`})
	pdf := itemComPath(t, q, `C:\x\p1.pdf`)
	esperarTerminoID(t, q, pdf.ID)

	ch := p.lista()
	if len(ch) != 2 {
		t.Fatalf("%d chamadas, esperava 2", len(ch))
	}
	if want := (Entrada{Kind: "zip", Caminhos: []string{`C:\x\a.zip`}}); !reflect.DeepEqual(ch[0].e, want) {
		t.Errorf("Entrada do ZIP = %+v, esperava %+v", ch[0].e, want)
	}
	if want := (Entrada{Kind: "pdf", Caminhos: pdf.Paths}); !reflect.DeepEqual(ch[1].e, want) {
		t.Errorf("Entrada do PDF = %+v, esperava %+v", ch[1].e, want)
	}
}

func TestProcessItem_NomeTrocadoPeloTXT(t *testing.T) {
	dir := `C:\Users\fulano\Downloads`
	saidas := map[string]string{
		filepath.Join(dir, "processo.pdf"):                          filepath.Join(dir, "processo_2.txt"),
		filepath.Join(dir, cnjTeste+" - Evento 3 - INIC1.pdf"):      filepath.Join(dir, cnjTeste+".txt"),
		filepath.Join(dir, cnjTeste+" - Evento 20 - PET1.pdf"):      filepath.Join(dir, cnjTeste+".txt"),
		filepath.Join(dir, cnjTeste+" - Evento 30 - DESPADEC1.pdf"): filepath.Join(dir, cnjTeste+"_2.txt"),
		filepath.Join(dir, "a.zip"):                                 filepath.Join(dir, "outro.txt"),
	}
	p := &procEntradas{resp: func(_ int, e Entrada) (string, error) { return saidas[e.Caminhos[0]], nil }}
	q, g := novaFilaGravada(t, p.processar)

	casos := []struct {
		paths []string
		nome  string
	}{
		{[]string{filepath.Join(dir, "processo.pdf")}, "processo_2"},
		{[]string{
			filepath.Join(dir, cnjTeste+" - Evento 3 - INIC1.pdf"),
			filepath.Join(dir, cnjTeste+" - Evento 7 - LAUDO1.pdf"),
			filepath.Join(dir, cnjTeste+" - Evento 12 - SENT1.pdf"),
		}, cnjTeste + " (3 PDFs)"},
		{[]string{
			filepath.Join(dir, cnjTeste+" - Evento 20 - PET1.pdf"),
			filepath.Join(dir, cnjTeste+" - Evento 21 - PET2.pdf"),
		}, cnjTeste + " (2 PDFs)"},
		{[]string{
			filepath.Join(dir, cnjTeste+" - Evento 30 - DESPADEC1.pdf"),
			filepath.Join(dir, cnjTeste+" - Evento 31 - PET3.pdf"),
		}, cnjTeste + "_2 (2 PDFs)"},
		{[]string{filepath.Join(dir, "a.zip")}, "a.zip"},
	}
	for _, c := range casos {
		q.AddFiles(c.paths)
		id := itemComPath(t, q, c.paths[0]).ID
		fim := esperarTerminoID(t, q, id)
		if fim.Name != c.nome {
			t.Errorf("Name final = %q, esperava %q", fim.Name, c.nome)
		}
		if it := esperarUpdateFinal(t, g, id); it.Name != c.nome {
			t.Errorf("último queue:update traz Name %q, esperava %q", it.Name, c.nome)
		}
	}
}

// pdfSintetico cria um arquivo .pdf sintético (o processar falso não o lê):
// o Reprocessar de item de PDF confere que os Paths existem.
func pdfSintetico(t *testing.T, nome string) string {
	t.Helper()
	c := filepath.Join(t.TempDir(), nome)
	if err := os.WriteFile(c, []byte("sintético"), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestReprocessar_PassaSaidaAnterior(t *testing.T) {
	pdf := pdfSintetico(t, "processo.pdf")
	p := &procEntradas{resp: func(int, Entrada) (string, error) {
		return `C:\x\processo.txt`, nil
	}}
	q, g := novaFilaGravada(t, p.processar)
	q.AddFiles([]string{pdf})
	id := itemComPath(t, q, pdf).ID
	if it := esperarTerminoID(t, q, id); it.OutputPath != `C:\x\processo.txt` {
		t.Fatalf("OutputPath = %q", it.OutputPath)
	}
	q.Reter()
	if err := q.Reprocessar(id, "none", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	for _, it := range ultimoUpdate(t, g) {
		if it.ID == id && it.OutputPath != "" {
			t.Errorf("cópia enviada traz OutputPath %q, esperava vazio", it.OutputPath)
		}
	}
	q.Soltar()
	esperarTerminoID(t, q, id)
	ch := p.lista()
	if len(ch) != 2 || ch[0].e.SaidaAnterior != "" || ch[1].e.SaidaAnterior != `C:\x\processo.txt` {
		t.Errorf("chamadas = %+v, esperava SaidaAnterior vazia e depois processo.txt", ch)
	}
}

func TestReprocessar_CanceladoMantemSaidaAnterior(t *testing.T) {
	iniciou := make(chan struct{}, 1)
	var ctxs sync.Map
	p := &procEntradas{}
	p.resp = func(n int, _ Entrada) (string, error) {
		switch n {
		case 1:
			return `C:\x\processo.txt`, nil
		case 2:
			iniciou <- struct{}{}
			ctx, _ := ctxs.Load("atual")
			<-ctx.(context.Context).Done()
			return "", errors.New("cancelado")
		case 3:
			return `C:\x\processo_2.txt`, nil
		}
		return `C:\x\processo_3.txt`, nil
	}
	base := p.processar
	q, _ := novaFilaGravada(t, func(ctx context.Context, e Entrada, cfg *models.Config, on func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		ctxs.Store("atual", ctx)
		return base(ctx, e, cfg, on)
	})
	pdf := pdfSintetico(t, "processo.pdf")
	q.AddFiles([]string{pdf})
	id := itemComPath(t, q, pdf).ID
	esperarTerminoID(t, q, id)

	if err := q.Reprocessar(id, "none", "none", nil); err != nil {
		t.Fatalf("Reprocessar: %v", err)
	}
	select {
	case <-iniciou:
	case <-time.After(2 * time.Second):
		t.Fatal("o segundo processamento não começou")
	}
	q.CancelItem(id)
	if it := esperarTerminoID(t, q, id); it.Status != "cancelled" {
		t.Fatalf("Status = %q, esperava cancelled", it.Status)
	}
	if err := q.Reprocessar(id, "none", "none", nil); err != nil {
		t.Fatalf("segundo Reprocessar: %v", err)
	}
	if it := esperarTerminoID(t, q, id); it.OutputPath != `C:\x\processo_2.txt` {
		t.Fatalf("OutputPath = %q", it.OutputPath)
	}
	if err := q.Reprocessar(id, "none", "none", nil); err != nil {
		t.Fatalf("terceiro Reprocessar: %v", err)
	}
	esperarTerminoID(t, q, id)

	ch := p.lista()
	if len(ch) != 4 {
		t.Fatalf("%d chamadas, esperava 4", len(ch))
	}
	for i, want := range []string{"", `C:\x\processo.txt`, `C:\x\processo.txt`, `C:\x\processo_2.txt`} {
		if got := ch[i].e.SaidaAnterior; got != want {
			t.Errorf("chamada %d: SaidaAnterior = %q, esperava %q", i+1, got, want)
		}
	}
}

func TestReprocessar_PDFComOrigemApagada(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.pdf"), filepath.Join(dir, "b.pdf")
	for _, c := range []string{a, b} {
		if err := os.WriteFile(c, []byte("sintético"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	q.AddFiles([]string{a, b})
	id := itemComPath(t, q, a).ID
	esperarTerminoID(t, q, id)
	q.AddFiles([]string{filepath.Join(dir, "z.zip")})
	esperarTerminoID(t, q, itemComPath(t, q, filepath.Join(dir, "z.zip")).ID)

	antes := q.GetItems()
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	err := q.Reprocessar(id, "none", "none", nil)
	if err == nil || err.Error() != "arquivo de origem não encontrado: b.pdf" {
		t.Fatalf("Reprocessar = %v, esperava arquivo de origem não encontrado: b.pdf", err)
	}
	if depois := q.GetItems(); !reflect.DeepEqual(antes, depois) {
		t.Errorf("a fila mudou:\nantes  %+v\ndepois %+v", antes, depois)
	}
	if it := q.GetItems()[1]; it.ID != id || it.Status != "success" {
		t.Errorf("item na posição 1 = %+v", it)
	}
}

func TestReprocessarMonitorado_ItemConcluidoDeUmPDF(t *testing.T) {
	p := &procEntradas{resp: func(n int, _ Entrada) (string, error) {
		if n == 1 {
			return `C:\x\processo.txt`, nil
		}
		return `C:\x\processo_2.txt`, nil
	}}
	q, _ := novaFilaGravada(t, p.processar)
	q.cfg = &models.Config{OcrMode: "fast", CleaningLevel: "basic"}
	pdf := pdfSintetico(t, "processo.pdf")
	q.AddFiles([]string{pdf})
	id := itemComPath(t, q, pdf).ID
	esperarTerminoID(t, q, id)
	// Um Reprocessar com ajuste antes: o monitorado volta à config global.
	if err := q.Reprocessar(id, "best", "aggressive", nil); err != nil {
		t.Fatal(err)
	}
	esperarTerminoID(t, q, id)
	q.AddFiles([]string{`C:\x\z.zip`})
	esperarTerminoID(t, q, itemComPath(t, q, `C:\x\z.zip`).ID)

	q.Reter()
	antes := q.GetItems()
	saidaAntes := antes[1].OutputPath
	if antes[1].ID != id || saidaAntes == "" {
		t.Fatalf("posição 1 = %+v", antes[1])
	}
	if !q.ReprocessarMonitorado(strings.ToUpper(pdf)) {
		t.Fatal("ReprocessarMonitorado devolveu false")
	}
	depois := q.GetItems()
	if len(depois) != len(antes) || depois[1].ID != id || depois[1].Status != "pending" {
		t.Fatalf("depois = %+v, esperava o item pending na posição 1", depois)
	}
	q.Soltar()
	esperarTerminoID(t, q, id)
	ch := p.lista()
	ult := ch[len(ch)-1]
	if ult.e.SaidaAnterior != saidaAntes {
		t.Errorf("SaidaAnterior = %q, esperava %q", ult.e.SaidaAnterior, saidaAntes)
	}
	if ult.cfg.OcrMode != "fast" || ult.cfg.CleaningLevel != "basic" {
		t.Errorf("cfg = %s/%s, esperava a global fast/basic", ult.cfg.OcrMode, ult.cfg.CleaningLevel)
	}
}

func TestReprocessarMonitorado_DevolveFalso(t *testing.T) {
	p := &procEntradas{resp: func(_ int, e Entrada) (string, error) {
		if e.Caminhos[0] == `C:\x\erro.pdf` {
			return "", errors.New("falha fictícia")
		}
		return strings.TrimSuffix(e.Caminhos[0], filepath.Ext(e.Caminhos[0])) + ".txt", nil
	}}
	q, _ := novaFilaGravada(t, p.processar)
	if q.ReprocessarMonitorado(`C:\x\nada.pdf`) {
		t.Error("fila vazia: devolveu true")
	}
	if len(q.GetItems()) != 0 {
		t.Error("fila vazia mudou")
	}

	q.AddFiles([]string{`C:\x\erro.pdf`})
	esperarTerminoID(t, q, itemComPath(t, q, `C:\x\erro.pdf`).ID)
	q.AddFiles([]string{`C:\x\m1.pdf`, `C:\x\m2.pdf`})
	esperarTerminoID(t, q, itemComPath(t, q, `C:\x\m1.pdf`).ID)
	q.AddFiles([]string{`C:\x\z.zip`})
	esperarTerminoID(t, q, itemComPath(t, q, `C:\x\z.zip`).ID)
	q.Reter()
	q.AddFiles([]string{`C:\x\cancel.pdf`})
	q.CancelItem(itemComPath(t, q, `C:\x\cancel.pdf`).ID)

	if it := itemComPath(t, q, `C:\x\erro.pdf`); it.Status != "error" {
		t.Fatalf("erro.pdf Status = %q", it.Status)
	}
	if it := itemComPath(t, q, `C:\x\cancel.pdf`); it.Status != "cancelled" {
		t.Fatalf("cancel.pdf Status = %q", it.Status)
	}
	for _, c := range []string{`C:\x\erro.pdf`, `C:\x\cancel.pdf`, `C:\x\m1.pdf`, `C:\x\z.zip`} {
		antes := q.GetItems()
		if q.ReprocessarMonitorado(c) {
			t.Errorf("%s: devolveu true", c)
		}
		if depois := q.GetItems(); !reflect.DeepEqual(antes, depois) {
			t.Errorf("%s: a fila mudou", c)
		}
	}
}

// O item reaberto por ReprocessarMonitorado vai para a telemetria como
// reprocesso (q1), embora rode sem ajuste.
func TestReprocessarMonitorado_TelemetriaReprocesso(t *testing.T) {
	q, _ := novaFilaGravada(t, (&procEntradas{}).processar)
	ch := make(chan models.ExecucaoZip, 4)
	q.SetAoTerminarZip(func(e models.ExecucaoZip) { ch <- e })
	pdf := pdfSintetico(t, "processo.pdf")
	q.AddFiles([]string{pdf})
	if e := esperarExecucao(t, ch); e.Reprocesso || e.Status != "success" {
		t.Fatalf("primeira execução = %+v, esperava success sem reprocesso", e)
	}
	esperarOcioso(t, q)
	if !q.ReprocessarMonitorado(pdf) {
		t.Fatal("ReprocessarMonitorado devolveu false")
	}
	if e := esperarExecucao(t, ch); !e.Reprocesso {
		t.Errorf("execução do monitorado = %+v, esperava Reprocesso true", e)
	}
}
