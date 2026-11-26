package historico

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const atrasoTeste = 50 * time.Millisecond

// contarEscritas liga o gancho aoGravarTeste e devolve o contador.
func contarEscritas(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	aoGravarTeste = func() { n.Add(1) }
	t.Cleanup(func() { aoGravarTeste = nil })
	return &n
}

// novoGravadorTeste cria um gravador com Atraso de teste e, no fim do teste,
// sincroniza com o mutex dele antes de os ganchos voltarem ao padrão.
func novoGravadorTeste(t *testing.T, arq string, o Opcoes) *Gravador {
	t.Helper()
	if o.Atraso == 0 {
		o.Atraso = atrasoTeste
	}
	g := NovoGravador(arq, o)
	t.Cleanup(func() {
		time.Sleep(3 * o.Atraso)
		g.mu.Lock()
		g.mu.Unlock() // só sincroniza com o timer
	})
	return g
}

// lerItens carrega o arquivo gravado e devolve os itens (falha se não der).
func lerItens(t *testing.T, arq string) []Item {
	t.Helper()
	dados, err := os.ReadFile(arq)
	if err != nil {
		t.Fatal(err)
	}
	var h arquivoHistorico
	if err := json.Unmarshal(dados, &h); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	return h.Itens
}

func lista(nome string) []Item {
	return []Item{{Name: nome + ".zip", Path: em(nome + ".zip"), Status: "success", Kind: "zip"}}
}

func TestGravador_IdaEVolta(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(tresItens()); err != nil {
		t.Fatal(err)
	}
	got, err := Carregar(arq)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, tresItens()) {
		t.Fatalf("ida e volta:\n got %+v\nwant %+v", got, tresItens())
	}
	dados, _ := os.ReadFile(arq)
	if n := strings.Count(string(dados), `"kind": "zip"`); n != 1 {
		t.Fatalf(`"kind": "zip" aparece %d vezes`, n)
	}
	if n := strings.Count(string(dados), `"kind": "pdf"`); n != 2 {
		t.Fatalf(`"kind": "pdf" aparece %d vezes`, n)
	}
	if !strings.Contains(string(dados), `"error": ""`) {
		t.Fatalf(`falta "error": "" no arquivo`)
	}
}

func TestGravador_Debounce(t *testing.T) {
	n := contarEscritas(t)
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	for _, nome := range []string{"a", "b", "c", "d"} {
		g.Agendar(lista(nome))
	}
	time.Sleep(3 * atrasoTeste)
	if got := n.Load(); got != 1 {
		t.Fatalf("escritas: %d; want 1", got)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("d")) {
		t.Fatalf("arquivo: %+v", got)
	}
}

func TestGravador_CriaPasta(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "nao", "existe", "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(lista("a")); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("a")) {
		t.Fatalf("arquivo: %+v", got)
	}
}

func TestGravador_SemBOM(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(nil); err != nil {
		t.Fatal(err)
	}
	dados, _ := os.ReadFile(arq)
	if bytes.HasPrefix(dados, bom) {
		t.Fatal("arquivo começa com BOM")
	}
	if !bytes.Contains(dados, []byte(`"versao": 1`)) {
		t.Fatalf("sem \"versao\": 1: %s", dados)
	}
	if !bytes.Contains(dados, []byte(`"itens": []`)) {
		t.Fatalf("lista nil não gravada como []: %s", dados)
	}
}

func TestGravador_RenameFalha(t *testing.T) {
	antigoRenomear, antigaEspera := renomear, esperaRenomear
	t.Cleanup(func() { renomear, esperaRenomear = antigoRenomear, antigaEspera })
	esperaRenomear = time.Millisecond

	t.Run("sempre falha", func(t *testing.T) {
		dir := t.TempDir()
		arq := filepath.Join(dir, "historico.json")
		antes := []byte(`{"versao": 1, "itens": []}`)
		escreverArquivo(t, arq, antes)
		renomear = func(_, _ string) error { return errors.New("acesso negado") }
		var falhas atomic.Int32
		g := novoGravadorTeste(t, arq, Opcoes{AoFalhar: func(error) { falhas.Add(1) }})
		if err := g.GravarAgora(lista("a")); err == nil {
			t.Fatal("GravarAgora não devolveu erro")
		}
		if got := falhas.Load(); got != 1 {
			t.Fatalf("AoFalhar chamado %d vezes; want 1", got)
		}
		depois, _ := os.ReadFile(arq)
		if !bytes.Equal(depois, antes) {
			t.Fatal("o arquivo anterior mudou")
		}
		todos, _ := os.ReadDir(dir)
		if len(todos) != 1 {
			t.Fatalf("sobraram %d arquivos na pasta; want 1", len(todos))
		}
	})

	t.Run("falha só na primeira", func(t *testing.T) {
		arq := filepath.Join(t.TempDir(), "historico.json")
		var chamadas atomic.Int32
		renomear = func(de, para string) error {
			if chamadas.Add(1) == 1 {
				return errors.New("em uso")
			}
			return os.Rename(de, para)
		}
		g := novoGravadorTeste(t, arq, Opcoes{AoFalhar: func(err error) { t.Errorf("AoFalhar: %v", err) }})
		if err := g.GravarAgora(lista("a")); err != nil {
			t.Fatal(err)
		}
		if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("a")) {
			t.Fatalf("arquivo: %+v", got)
		}
	})
}

func TestGravador_Concorrencia(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{AoFalhar: func(error) {}})

	enviadas := map[string]bool{}
	var listas [][]Item
	for i := range 8 {
		for j := range 20 {
			l := lista(fmt.Sprintf("g%d-%d", i, j))
			listas = append(listas, l)
			b, _ := json.Marshal(l)
			enviadas[string(b)] = true
		}
	}
	agora := lista("agora")
	b, _ := json.Marshal(agora)
	enviadas[string(b)] = true

	var wg sync.WaitGroup
	parar := make(chan struct{})
	for i := range 8 {
		wg.Go(func() {
			for j := range 20 {
				g.Agendar(listas[i*20+j])
			}
		})
	}
	wg.Go(func() {
		for range 30 {
			_ = g.GravarAgora(agora)
		}
	})
	leitora := make(chan error, 1)
	go func() {
		defer close(leitora)
		for {
			select {
			case <-parar:
				return
			default:
			}
			dados, err := os.ReadFile(arq)
			if err != nil {
				continue // ausente ou em uso pelo rename: pula
			}
			var h arquivoHistorico
			if err := json.Unmarshal(dados, &h); err != nil {
				leitora <- fmt.Errorf("leitura não é JSON válido: %v", err)
				return
			}
			c, _ := json.Marshal(h.Itens)
			if !enviadas[string(c)] {
				leitora <- fmt.Errorf("lista lida não foi enviada: %s", c)
				return
			}
		}
	}()
	wg.Wait()
	time.Sleep(3 * atrasoTeste)
	close(parar)
	if err := <-leitora; err != nil {
		t.Fatal(err)
	}

	l, m := lista("L"), lista("M")
	if err := g.GravarAgora(l); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, l) {
		t.Fatalf("depois de GravarAgora(L): %+v", got)
	}
	g.Agendar(m)
	time.Sleep(3 * atrasoTeste)
	if got := lerItens(t, arq); !reflect.DeepEqual(got, m) {
		t.Fatalf("depois de Agendar(M): %+v", got)
	}
}

func TestGravador_FecharComTimerPendente(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	a, b, c := lista("A"), lista("B"), lista("C")
	g.Agendar(a)
	if err := g.Fechar(b); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, b) {
		t.Fatalf("depois de Fechar(B): %+v", got)
	}
	time.Sleep(3 * atrasoTeste)
	if got := lerItens(t, arq); !reflect.DeepEqual(got, b) {
		t.Fatalf("o timer regravou: %+v", got)
	}
	g.Agendar(c)
	if err := g.GravarAgora(c); err != nil {
		t.Fatalf("GravarAgora depois de Fechar: %v", err)
	}
	time.Sleep(3 * atrasoTeste)
	if got := lerItens(t, arq); !reflect.DeepEqual(got, b) {
		t.Fatalf("gravou depois de fechado: %+v", got)
	}
}

func TestGravador_FecharDuasVezes(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.Fechar(lista("A")); err != nil {
		t.Fatal(err)
	}
	if err := g.Fechar(lista("B")); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("A")) {
		t.Fatalf("arquivo: %+v", got)
	}
}

func TestGravador_FecharEApagar(t *testing.T) {
	t.Run("timer pendente", func(t *testing.T) {
		arq := filepath.Join(t.TempDir(), "historico.json")
		g := novoGravadorTeste(t, arq, Opcoes{})
		if err := g.GravarAgora(lista("A")); err != nil {
			t.Fatal(err)
		}
		g.Agendar(lista("B"))
		if err := g.FecharEApagar(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * atrasoTeste)
		if _, err := os.Stat(arq); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("arquivo existe: %v", err)
		}
	})
	t.Run("já fechado", func(t *testing.T) {
		arq := filepath.Join(t.TempDir(), "historico.json")
		g := novoGravadorTeste(t, arq, Opcoes{})
		if err := g.Fechar(lista("A")); err != nil {
			t.Fatal(err)
		}
		if err := g.FecharEApagar(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(arq); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("arquivo existe: %v", err)
		}
	})
	t.Run("sem arquivo", func(t *testing.T) {
		arq := filepath.Join(t.TempDir(), "historico.json")
		g := novoGravadorTeste(t, arq, Opcoes{})
		if err := g.FecharEApagar(); err != nil {
			t.Fatalf("erro sem arquivo: %v", err)
		}
	})
}

func TestGravador_Reabrir(t *testing.T) {
	n := contarEscritas(t)
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.Fechar(lista("A")); err != nil {
		t.Fatal(err)
	}
	if err := g.Reabrir(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * atrasoTeste)
	if got := n.Load(); got != 1 {
		t.Fatalf("Reabrir gravou: %d escritas; want 1", got)
	}
	g.Agendar(lista("B"))
	time.Sleep(3 * atrasoTeste)
	if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("B")) {
		t.Fatalf("arquivo: %+v", got)
	}
}

func TestGravador_Protegido(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	novo := []byte(`{"versao":2,"itens":[]}`)
	escreverArquivo(t, arq, novo)
	g := novoGravadorTeste(t, arq, Opcoes{Protegido: true})
	mesmo := func(passo string) {
		t.Helper()
		dados, _ := os.ReadFile(arq)
		if !bytes.Equal(dados, novo) {
			t.Fatalf("%s mudou o arquivo protegido: %s", passo, dados)
		}
	}
	g.Agendar([]Item{})
	time.Sleep(3 * atrasoTeste)
	mesmo("Agendar([])")
	if err := g.GravarAgora([]Item{}); err != nil {
		t.Fatal(err)
	}
	mesmo("GravarAgora([])")
	if err := g.GravarAgora(lista("X")); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); !reflect.DeepEqual(got, lista("X")) {
		t.Fatalf("depois de GravarAgora([X]): %+v", got)
	}
	if err := g.GravarAgora([]Item{}); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); len(got) != 0 {
		t.Fatalf("depois de GravarAgora([]): %+v", got)
	}
}

func TestGravador_ProtegidoFechar(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	novo := []byte(`{"versao":2,"itens":[]}`)
	escreverArquivo(t, arq, novo)
	g := novoGravadorTeste(t, arq, Opcoes{Protegido: true})
	if err := g.Fechar([]Item{}); err != nil {
		t.Fatal(err)
	}
	dados, _ := os.ReadFile(arq)
	if !bytes.Equal(dados, novo) {
		t.Fatalf("Fechar([]) mudou o arquivo protegido: %s", dados)
	}
}

func TestGravador_Teto(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "historico.json")
	g := novoGravadorTeste(t, arq, Opcoes{})
	if err := g.GravarAgora(itensNumerados(501)); err != nil {
		t.Fatal(err)
	}
	if got := lerItens(t, arq); len(got) != 500 || got[499].Name != "p499.zip" {
		t.Fatalf("gravou %d itens", len(got))
	}
}
