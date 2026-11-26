package watchdog

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// coletor guarda os caminhos que chegaram ao callback, sem bloquear.
type coletor struct {
	mu       sync.Mutex
	caminhos []string
}

func (c *coletor) add(p string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.caminhos = append(c.caminhos, p)
}

func (c *coletor) lista() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.caminhos)
}

// esperarAte espera até cond ser verdadeira ou o prazo vencer.
func esperarAte(prazo time.Duration, cond func() bool) bool {
	fim := time.Now().Add(prazo)
	for time.Now().Before(fim) {
		if cond() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return cond()
}

func distintos(l []string) []string {
	out := slices.Clone(l)
	slices.Sort(out)
	return slices.Compact(out)
}

func TestExtensoes(t *testing.T) {
	if got := Extensoes(false); !reflect.DeepEqual(got, []string{".zip"}) {
		t.Fatalf("Extensoes(false) = %v", got)
	}
	if got := Extensoes(true); !reflect.DeepEqual(got, []string{".zip", ".pdf"}) {
		t.Fatalf("Extensoes(true) = %v", got)
	}
}

func TestJaConvertido(t *testing.T) {
	agora := time.Now().Truncate(time.Second)
	antes := agora.Add(-time.Hour)
	depois := agora.Add(time.Hour)

	casos := []struct {
		nome     string
		arquivos map[string]time.Time
		entrada  string
		quer     bool
	}{
		{"zip com txt de mesma data", map[string]time.Time{"p.zip": agora, "p.txt": agora}, "p.zip", true},
		{"zip com txt mais novo", map[string]time.Time{"p.zip": agora, "p.txt": depois}, "p.zip", true},
		{"zip com txt mais velho", map[string]time.Time{"p.zip": agora, "p.txt": antes}, "p.zip", false},
		{"zip sem txt", map[string]time.Time{"p.zip": agora}, "p.zip", false},
		{"zip ignora _2", map[string]time.Time{"p.zip": agora, "p_2.txt": depois}, "p.zip", false},
		{"pdf sem txt", map[string]time.Time{"p.pdf": agora}, "p.pdf", false},
		{"pdf com _2 mais novo", map[string]time.Time{"p.pdf": agora, "p_2.txt": depois}, "p.pdf", true},
		{"pdf com _2 de mesma data", map[string]time.Time{"p.pdf": agora, "p_2.txt": agora}, "p.pdf", true},
		{"pdf com txt mais novo sem zip", map[string]time.Time{"p.pdf": agora, "p.txt": depois}, "p.pdf", true},
		{"pdf com caixa diferente", map[string]time.Time{"Processo.PDF": agora, "PROCESSO.txt": depois}, "Processo.PDF", true},
		{"pdf com zip homonimo: txt é do zip", map[string]time.Time{"p.pdf": agora, "p.zip": antes, "p.txt": depois}, "p.pdf", false},
		{"pdf com zip homonimo e _2", map[string]time.Time{"p.pdf": agora, "p.zip": antes, "p.txt": depois, "p_2.txt": depois}, "p.pdf", true},
		{"pdf com _2 mais velho", map[string]time.Time{"p.pdf": agora, "p_2.txt": antes}, "p.pdf", false},
		{"pdf com txt de outro radical", map[string]time.Time{"p.pdf": agora, "pp.txt": depois, "p_02.txt": depois}, "p.pdf", false},
		// Revisão da Task 1: processo_2.txt é a base de processo_2.pdf, não
		// prova que processo.pdf foi convertido.
		{"processo.pdf com processo_2.pdf e só processo_2.txt", map[string]time.Time{"processo.pdf": agora, "processo_2.pdf": agora, "processo_2.txt": depois}, "processo.pdf", false},
		{"processo_2.pdf com seu processo_2.txt", map[string]time.Time{"processo.pdf": agora, "processo_2.pdf": agora, "processo_2.txt": depois}, "processo_2.pdf", true},
		{"processo.pdf com processo_2.zip e processo_2.txt", map[string]time.Time{"processo.pdf": agora, "processo_2.zip": antes, "processo_2.txt": depois}, "processo.pdf", false},
		{"processo.pdf com processo_2.pdf e processo_3.txt", map[string]time.Time{"processo.pdf": agora, "processo_2.pdf": agora, "processo_3.txt": depois}, "processo.pdf", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			for nome, quando := range c.arquivos {
				criarComData(t, filepath.Join(dir, nome), quando)
			}
			caminho := filepath.Join(dir, c.entrada)
			if got := jaConvertido(caminho, c.arquivos[c.entrada]); got != c.quer {
				t.Fatalf("jaConvertido(%s) = %v, esperado %v", c.entrada, got, c.quer)
			}
		})
	}
}

func TestPendingZips_PDF(t *testing.T) {
	dir := t.TempDir()
	agora := time.Now()
	desde := agora.Add(-24 * time.Hour)
	h := func(n float64) time.Time { return agora.Add(-time.Duration(n * float64(time.Hour))) }

	criarComData(t, filepath.Join(dir, "a.zip"), h(5))
	criarComData(t, filepath.Join(dir, "b.pdf"), h(4))
	criarComData(t, filepath.Join(dir, "c.pdf"), h(3))
	criarComData(t, filepath.Join(dir, "c_2.txt"), h(2))
	criarComData(t, filepath.Join(dir, "d.zip"), h(6))
	criarComData(t, filepath.Join(dir, "d.pdf"), h(2.5))
	criarComData(t, filepath.Join(dir, "d.txt"), h(1))
	criarComData(t, filepath.Join(dir, "eprocia_pdf_123.pdf"), h(1.5))
	criarComData(t, filepath.Join(dir, "EPROCIA_PDF_9.PDF"), h(1.5))
	criarComData(t, filepath.Join(dir, "e.PDF"), h(0.5))

	got, err := pendingZips(dir, desde, []string{".zip"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(dir, "a.zip")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("só ZIP: %v, esperado %v", got, want)
	}

	got, err = pendingZips(dir, desde, []string{".zip", ".pdf"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "a.zip"),
		filepath.Join(dir, "b.pdf"),
		filepath.Join(dir, "d.pdf"),
		filepath.Join(dir, "e.PDF"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ZIP e PDF: %v, esperado %v", got, want)
	}
}

func TestWatcher_PDFIgnoradoComOpcaoDesligada(t *testing.T) {
	dir := t.TempDir()
	c := &coletor{}
	w := NewWatcher(c.add)
	if err := w.Start(dir, Extensoes(false)); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if err := os.WriteFile(filepath.Join(dir, "peca.pdf"), []byte("conteudo pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if l := c.lista(); len(l) != 0 {
		t.Fatalf("PDF com a opção desligada chamou o callback: %v", l)
	}

	zip := filepath.Join(dir, "processo.zip")
	if err := os.WriteFile(zip, []byte("conteudo zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !esperarAte(5*time.Second, func() bool { return len(c.lista()) > 0 }) {
		t.Fatal("ZIP não chamou o callback")
	}
	if d := distintos(c.lista()); !reflect.DeepEqual(d, []string{zip}) {
		t.Fatalf("callback recebeu %v, esperado só %s", d, zip)
	}
}

func TestWatcher_PDFsSeparados(t *testing.T) {
	dir := t.TempDir()
	c := &coletor{}
	w := NewWatcher(c.add)
	if err := w.Start(dir, Extensoes(true)); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	a := filepath.Join(dir, "peca-a.pdf")
	b := filepath.Join(dir, "peca-b.pdf")
	if err := os.WriteFile(filepath.Join(dir, "eprocia_pdf_1.pdf"), []byte("temporario"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, []byte("conteudo a"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if err := os.WriteFile(b, []byte("conteudo b"), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []string{a, b}
	if !esperarAte(5*time.Second, func() bool { return reflect.DeepEqual(distintos(c.lista()), want) }) {
		t.Fatalf("callback recebeu %v, esperado %v (um caminho por chamada)", c.lista(), want)
	}
	// Folga para um eventual disparo atrasado do temporário.
	time.Sleep(700 * time.Millisecond)
	if d := distintos(c.lista()); !reflect.DeepEqual(d, want) {
		t.Fatalf("callback recebeu %v, esperado %v", d, want)
	}
}

func TestHandleNewFile_PDFJaConvertido(t *testing.T) {
	dir := t.TempDir()
	agora := time.Now()
	pdf := filepath.Join(dir, "peca.pdf")
	zip := filepath.Join(dir, "processo.zip")
	criarComData(t, pdf, agora.Add(-time.Hour))
	criarComData(t, filepath.Join(dir, "peca.txt"), agora.Add(-time.Minute))
	criarComData(t, zip, agora.Add(-time.Hour))
	criarComData(t, filepath.Join(dir, "processo.txt"), agora.Add(-time.Minute))

	c := &coletor{}
	w := NewWatcher(c.add)
	done := make(chan struct{})
	defer close(done)

	w.handleNewFile(pdf, done)
	w.handleNewFile(pdf, done)
	if l := c.lista(); len(l) != 0 {
		t.Fatalf("PDF já convertido chamou o callback: %v", l)
	}

	w.handleNewFile(zip, done)
	if l := c.lista(); !reflect.DeepEqual(l, []string{zip}) {
		t.Fatalf("ZIP com TXT mais novo: callback %v, esperado [%s]", l, zip)
	}

	// PDF salvo de novo depois do TXT.
	if err := os.Chtimes(pdf, agora, agora); err != nil {
		t.Fatal(err)
	}
	w.handleNewFile(pdf, done)
	if l := c.lista(); !reflect.DeepEqual(l, []string{zip, pdf}) {
		t.Fatalf("PDF mais novo que o TXT: callback %v, esperado [%s %s]", l, zip, pdf)
	}
}

func TestWatcher_StartMesmasExtensoes(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "pendente.zip")
	criarComData(t, zip, time.Now().Add(-10*time.Minute))

	c := &coletor{}
	w := NewWatcher(c.add)
	defer w.Stop()
	atual := func() *fsnotify.Watcher {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.watcher
	}

	if err := w.Start(dir, []string{".zip"}); err != nil {
		t.Fatal(err)
	}
	primeiro := atual()
	if !esperarAte(5*time.Second, func() bool { return len(c.lista()) == 1 }) {
		t.Fatalf("varredura inicial: callback %v", c.lista())
	}

	for _, ext := range [][]string{{".zip"}, {".ZIP"}, nil, {}, {".zip", ".zip"}} {
		if err := w.Start(dir, ext); err != nil {
			t.Fatal(err)
		}
		if atual() != primeiro {
			t.Fatalf("Start com as mesmas extensões (%v) trocou o watcher", ext)
		}
	}
	time.Sleep(time.Second)
	if l := c.lista(); len(l) != 1 {
		t.Fatalf("Start no-op rodou a varredura de novo: %v", l)
	}

	if err := w.Start(dir, []string{".zip", ".pdf"}); err != nil {
		t.Fatal(err)
	}
	if atual() == primeiro {
		t.Fatal("Start com extensões diferentes manteve o watcher")
	}
	if !esperarAte(5*time.Second, func() bool { return len(c.lista()) == 2 }) {
		t.Fatalf("varredura depois da troca de extensões: callback %v", c.lista())
	}
	if l := c.lista(); l[1] != zip {
		t.Fatalf("segunda varredura entregou %q, esperado %q", l[1], zip)
	}
}
