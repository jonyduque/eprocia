package atualizacao

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// prazoTeste é o prazo de toda espera dos testes do Atualizador: um impasse
// reprova em vez de travar a suíte.
const prazoTeste = 2 * time.Second

func releaseTeste() *Release {
	return &Release{
		Tag:        "v0.5.0",
		Versao:     Versao{0, 5, 0},
		NotasURL:   "https://github.com/fulano/eprocia/releases/tag/v0.5.0",
		Exe:        Asset{Nome: "eprocia-0.5.0-windows-amd64.exe", Tamanho: 10},
		Instalador: Asset{Nome: "eprocia-0.5.0-windows-amd64-instalador.exe", Tamanho: 10},
	}
}

// cenarioAt monta um Atualizador com Rede, Trocador, Shell, Reabrir e
// Ambiente falsos; as chamadas ao Ambiente, ao Trocador, ao Shell e ao
// Reabrir ficam em ordem.
type cenarioAt struct {
	t       *testing.T
	a       *Atualizador
	atual   Versao
	modo    Modo
	pasta   string
	dados   string
	atualiz string

	mu       sync.Mutex
	ordem    []string
	estados  []Estado
	ociosa   func(n int) bool
	nOciosa  int
	pausada  bool
	oculta   bool
	sair     func()
	rel      *Release
	errRede  error
	nVerif   int
	nBaixar  int
	baixar   func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(feito, total int64)) (Baixado, error)
	baixados []string
	instalar func(ctx context.Context, b Baixado) error
	reinic   func(ctx context.Context, nova Versao, trava sync.Locker) error
	aoCriar  func(extras []string)
	extras   [][]string
	travas   []sync.Locker
	novas    []Versao
	shell    func(string) error
	reabrir  func(Baixado) (*Travado, error)
}

func novoCenarioAt(t *testing.T, modo Modo) *cenarioAt {
	t.Helper()
	return &cenarioAt{
		t:       t,
		atual:   Versao{0, 4, 4},
		modo:    modo,
		pasta:   t.TempDir(),
		dados:   t.TempDir(),
		atualiz: filepath.Join(t.TempDir(), "atualizacao"),
		ociosa:  func(int) bool { return true },
		rel:     releaseTeste(),
	}
}

func (c *cenarioAt) reg(nome string) {
	c.mu.Lock()
	c.ordem = append(c.ordem, nome)
	c.mu.Unlock()
}

func (c *cenarioAt) iniciar() *Atualizador {
	inst := Instalacao{Modo: c.modo, Pasta: c.pasta}
	if c.modo == ModoSoAvisar {
		inst.Motivo = MotivoSemEscrita
	}
	o := Opcoes{
		Atual:            c.atual,
		Instalacao:       inst,
		PastaDados:       c.dados,
		PastaAtualizacao: c.atualiz,
		PID:              1234,
		Rede:             redeAt{c},
		Trocador:         c.fabrica,
		Shell:            c.shellFalso,
		Reabrir:          c.reabrirFalso,
	}
	amb := Ambiente{
		PararMonitor:   func() { c.reg("PararMonitor") },
		ReligarMonitor: func() { c.reg("ReligarMonitor") },
		Reter:          func() { c.reg("Reter") },
		Soltar:         func() { c.reg("Soltar") },
		Ociosa:         c.ociosaFalsa,
		Pausada: func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			return c.pausada
		},
		Oculta: func() bool {
			c.mu.Lock()
			defer c.mu.Unlock()
			return c.oculta
		},
		Gravador: gravadorAt{c},
		Sair: func() {
			c.reg("Sair")
			c.mu.Lock()
			s := c.sair
			c.mu.Unlock()
			if s != nil {
				s()
			}
		},
		Emitir: func(e Estado) {
			c.mu.Lock()
			c.estados = append(c.estados, e)
			c.mu.Unlock()
		},
	}
	c.a = NovoAtualizador(o, amb)
	return c.a
}

func (c *cenarioAt) ociosaFalsa() bool {
	c.mu.Lock()
	c.nOciosa++
	n, f := c.nOciosa, c.ociosa
	c.ordem = append(c.ordem, "Ociosa")
	c.mu.Unlock()
	return f(n)
}

func (c *cenarioAt) definirOciosa(v bool) {
	c.mu.Lock()
	c.ociosa = func(int) bool { return v }
	c.mu.Unlock()
}

type gravadorAt struct{ c *cenarioAt }

func (g gravadorAt) Fechar() error  { g.c.reg("Gravador.Fechar"); return nil }
func (g gravadorAt) Reabrir() error { g.c.reg("Gravador.Reabrir"); return nil }

type redeAt struct{ c *cenarioAt }

func (r redeAt) Verificar(ctx context.Context, atual Versao) (*Release, error) {
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	r.c.nVerif++
	return r.c.rel, r.c.errRede
}

func (r redeAt) Baixar(ctx context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(feito, total int64)) (Baixado, error) {
	r.c.mu.Lock()
	r.c.nBaixar++
	r.c.baixados = append(r.c.baixados, filepath.Join(pasta, nome))
	f := r.c.baixar
	r.c.mu.Unlock()
	if f != nil {
		return f(ctx, rel, asset, pasta, nome, progresso)
	}
	return gravarBaixado(rel, asset, pasta, nome)
}

func gravarBaixado(rel *Release, asset Asset, pasta, nome string) (Baixado, error) {
	caminho := filepath.Join(pasta, nome)
	if err := os.WriteFile(caminho, []byte("novo "+rel.Versao.String()), 0o644); err != nil {
		return Baixado{}, err
	}
	return Baixado{Caminho: caminho, Asset: asset, Versao: rel.Versao}, nil
}

func (c *cenarioAt) fabrica(extras []string, trava sync.Locker) Trocador {
	c.mu.Lock()
	c.extras = append(c.extras, extras)
	c.travas = append(c.travas, trava)
	f := c.aoCriar
	c.mu.Unlock()
	if f != nil {
		f(extras)
	}
	return &trocaAt{c: c, trava: trava}
}

type trocaAt struct {
	c     *cenarioAt
	trava sync.Locker
}

func (f *trocaAt) Instalar(ctx context.Context, b Baixado) error {
	f.c.reg("Trocador.Instalar")
	f.c.mu.Lock()
	fn := f.c.instalar
	f.c.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, b)
}

func (f *trocaAt) Reiniciar(ctx context.Context, nova Versao) error {
	f.c.reg("Trocador.Reiniciar")
	f.c.mu.Lock()
	f.c.novas = append(f.c.novas, nova)
	fn := f.c.reinic
	f.c.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, nova, f.trava)
}

func (c *cenarioAt) shellFalso(caminho string) error {
	c.reg("Shell")
	c.mu.Lock()
	fn := c.shell
	c.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(caminho)
}

func (c *cenarioAt) reabrirFalso(b Baixado) (*Travado, error) {
	c.reg("Reabrir")
	c.mu.Lock()
	fn := c.reabrir
	c.mu.Unlock()
	if fn != nil {
		return fn(b)
	}
	f, err := os.Open(b.Caminho)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNaoConferiu, err)
	}
	return &Travado{f: f, caminho: b.Caminho}, nil
}

func (c *cenarioAt) ordemAgora() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.ordem)
}

func (c *cenarioAt) chamadas(nome string) int {
	n := 0
	for _, o := range c.ordemAgora() {
		if o == nome {
			n++
		}
	}
	return n
}

func (c *cenarioAt) esperar(cond func() bool, oque string) {
	c.t.Helper()
	limite := time.Now().Add(prazoTeste)
	for !cond() {
		if time.Now().After(limite) {
			c.t.Fatalf("esperou %s por %v; estado %+v; ordem %v", oque, prazoTeste, c.a.Estado(), c.ordemAgora())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (c *cenarioAt) esperarFase(fase string) {
	c.t.Helper()
	c.esperar(func() bool { return c.a.Estado().Fase == fase }, "a fase "+fase)
}

func (c *cenarioAt) esperarChamada(nome string, n int) {
	c.t.Helper()
	c.esperar(func() bool { return c.chamadas(nome) >= n }, fmt.Sprintf("%d chamada(s) de %s", n, nome))
}

func (c *cenarioAt) verificado() Info {
	c.t.Helper()
	if c.a == nil {
		c.iniciar()
	}
	info, err := c.a.Verificar(context.Background())
	if err != nil {
		c.t.Fatalf("Verificar: %v", err)
	}
	return info
}

// instalada leva o modo trocar até a fase instalada.
func (c *cenarioAt) instalada() {
	c.t.Helper()
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		c.t.Fatalf("Instalar: %v", err)
	}
	c.esperarFase(FaseInstalada)
}

// devolveEm roda f numa goroutine e diz se ela voltou dentro de d.
func devolveEm(d time.Duration, f func()) bool {
	feito := make(chan struct{})
	go func() {
		f()
		close(feito)
	}()
	select {
	case <-feito:
		return true
	case <-time.After(d):
		return false
	}
}

func receberEm(t *testing.T, ch <-chan struct{}, oque string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(prazoTeste):
		t.Fatalf("esperou %s por %v", oque, prazoTeste)
	}
}

func querOrdem(t *testing.T, c *cenarioAt, quer []string) {
	t.Helper()
	if got := c.ordemAgora(); !slices.Equal(got, quer) {
		t.Fatalf("ordem = %v; quer %v", got, quer)
	}
}

func TestAtualizador_VerificarModos(t *testing.T) {
	t.Run("trocar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		info := c.verificado()
		quer := Info{Versao: "0.5.0", NotasURL: releaseTeste().NotasURL, Modo: "trocar"}
		if info != quer {
			t.Fatalf("info = %+v; quer %+v", info, quer)
		}
		if e := c.a.Estado(); e.Fase != FaseDisponivel || e.Modo != "trocar" || e.Versao != "0.5.0" || e.Atual != "0.4.4" || !e.Ativo {
			t.Fatalf("estado = %+v", e)
		}
		if c.a.NotasURL() != releaseTeste().NotasURL {
			t.Fatalf("NotasURL = %q", c.a.NotasURL())
		}
	})
	t.Run("incompleta", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel = &Release{Tag: "v0.5.0", Versao: Versao{0, 5, 0}, NotasURL: "https://github.com/fulano/eprocia/releases/tag/v0.5.0"}
		c.errRede = ErrIncompleta
		info := c.verificado()
		if info.Modo != "incompleta" || info.Versao != "0.5.0" {
			t.Fatalf("info = %+v", info)
		}
		if f := c.a.Estado().Fase; f != FaseIncompleta {
			t.Fatalf("fase = %q", f)
		}
	})
	t.Run("em dia", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel = nil
		info := c.verificado()
		if info.Modo != "em_dia" || info.Versao != "0.4.4" {
			t.Fatalf("info = %+v", info)
		}
		if f := c.a.Estado().Fase; f != FaseEmDia {
			t.Fatalf("fase = %q", f)
		}
	})
	t.Run("so avisar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoSoAvisar)
		info := c.verificado()
		if info.Modo != "so_avisar" || info.Motivo != MotivoSemEscrita || info.Versao != "0.5.0" {
			t.Fatalf("info = %+v", info)
		}
	})
	for _, caso := range []struct {
		nome  string
		err   error
		frase string
	}{
		{"sem rede", fmt.Errorf("obter https://api.github.com/x: %w", errors.New("dial")), "Não foi possível consultar o GitHub agora"},
		{"recusada", fmt.Errorf("%w: 403", ErrRecusada), "O GitHub recusou a consulta agora; tente mais tarde"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenarioAt(t, ModoTrocar)
			c.rel, c.errRede = nil, caso.err
			c.iniciar()
			_, err := c.a.Verificar(context.Background())
			if err == nil || err.Error() != caso.frase {
				t.Fatalf("erro = %v; quer a frase %q", err, caso.frase)
			}
			if !errors.Is(err, caso.err) {
				t.Fatalf("erro perdeu a origem: %v", err)
			}
			e := c.a.Estado()
			if e.Fase != FaseErro || e.Mensagem != caso.frase || e.Modo != "" {
				t.Fatalf("estado = %+v", e)
			}
		})
	}
}

func TestAtualizador_DesligadoSemVersao(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.atual = Versao{}
	c.iniciar()
	if c.a.Estado().Ativo {
		t.Fatal("Ativo com versão 0.0.0")
	}
	if _, err := c.a.Verificar(context.Background()); !errors.Is(err, ErrDesligado) {
		t.Fatalf("Verificar = %v; quer ErrDesligado", err)
	}
	if c.nVerif != 0 {
		t.Fatal("a Rede foi chamada")
	}
}

func querInstalarRecusado(t *testing.T, c *cenarioAt, quer error) {
	t.Helper()
	if err := c.a.Instalar(); !errors.Is(err, quer) {
		t.Fatalf("Instalar = %v; quer %v", err, quer)
	}
	time.Sleep(20 * time.Millisecond)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nBaixar != 0 {
		t.Fatal("Baixar foi chamado")
	}
}

func TestAtualizador_InstalarSemConsulta(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.iniciar()
	querInstalarRecusado(t, c, ErrSemConsulta)

	// Consulta que falhou também não conta.
	c2 := novoCenarioAt(t, ModoTrocar)
	c2.rel, c2.errRede = nil, errors.New("dial")
	c2.iniciar()
	_, _ = c2.a.Verificar(context.Background())
	querInstalarRecusado(t, c2, ErrSemConsulta)
}

func TestAtualizador_InstalarDepoisDeIncompleta(t *testing.T) {
	t.Run("incompleta", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.errRede = ErrIncompleta
		c.verificado()
		querInstalarRecusado(t, c, ErrNaoInstalavel)
	})
	t.Run("em dia", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel = nil
		c.verificado()
		querInstalarRecusado(t, c, ErrNaoInstalavel)
	})
	t.Run("so avisar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoSoAvisar)
		c.verificado()
		querInstalarRecusado(t, c, ErrNaoInstalavel)
	})
}

func TestAtualizador_InstalarSemPreparacao(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.definirOciosa(false)
	c.instalada()
	e := c.a.Estado()
	if e.Versao != "0.5.0" || e.Atual != "0.4.4" || e.Modo != "trocar" {
		t.Fatalf("estado = %+v", e)
	}
	querOrdem(t, c, []string{"Trocador.Instalar"})
	if len(c.extras) != 1 || c.extras[0] != nil {
		t.Fatalf("Trocador do Instalar criado com extras %v; quer nil", c.extras)
	}
	if c.travas[0] == nil {
		t.Fatal("Trocador do Instalar sem a trava")
	}
}

func TestAtualizador_OrdemDaPreparacao(t *testing.T) {
	t.Run("trocar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatalf("Reiniciar: %v", err)
		}
		c.esperarChamada("Sair", 1)
		querOrdem(t, c, []string{"Trocador.Instalar", "Ociosa", "PararMonitor", "Reter", "Ociosa", "Gravador.Fechar", "Trocador.Reiniciar", "Sair"})
		if len(c.novas) != 1 || c.novas[0] != (Versao{0, 5, 0}) {
			t.Fatalf("Reiniciar com %v; quer 0.5.0", c.novas)
		}
		if f := c.a.Estado().Fase; f != FaseReiniciando {
			t.Fatalf("fase = %q", f)
		}
		if m := c.a.Estado().Mensagem; m != "Abrindo a versão 0.5.0..." {
			t.Fatalf("mensagem = %q", m)
		}
	})
	t.Run("instalador", func(t *testing.T) {
		c := novoCenarioAt(t, ModoInstalador)
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatalf("Instalar: %v", err)
		}
		c.esperarChamada("Sair", 1)
		querOrdem(t, c, []string{"Ociosa", "Reabrir", "PararMonitor", "Reter", "Ociosa", "Gravador.Fechar", "Shell", "Sair"})
		if f := c.a.Estado().Fase; f != FaseAbrindoInstalador {
			t.Fatalf("fase = %q", f)
		}
	})
}

func TestAtualizador_FilaOcupadaEspera(t *testing.T) {
	t.Run("trocar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.definirOciosa(false)
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatal(err)
		}
		c.esperarFase(FaseAguardandoFila)
		time.Sleep(30 * time.Millisecond)
		if c.chamadas("Trocador.Reiniciar") != 0 || c.chamadas("PararMonitor") != 0 {
			t.Fatalf("seguiu com a fila ocupada: %v", c.ordemAgora())
		}
		c.definirOciosa(true)
		c.a.AoFicarOciosa()
		c.esperarChamada("Sair", 1)
		if c.chamadas("Trocador.Reiniciar") != 1 {
			t.Fatalf("ordem %v", c.ordemAgora())
		}
	})
	t.Run("instalador", func(t *testing.T) {
		c := novoCenarioAt(t, ModoInstalador)
		c.definirOciosa(false)
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		c.esperarFase(FaseAguardandoFila)
		time.Sleep(30 * time.Millisecond)
		if c.chamadas("Shell") != 0 || c.chamadas("Reabrir") != 0 {
			t.Fatalf("seguiu com a fila ocupada: %v", c.ordemAgora())
		}
		c.definirOciosa(true)
		c.a.AoFicarOciosa()
		c.esperarChamada("Sair", 1)
		if c.chamadas("Shell") != 1 {
			t.Fatalf("ordem %v", c.ordemAgora())
		}
	})
}

func TestAtualizador_AvisoAntesDaFase(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.instalada()
	c.mu.Lock()
	livre := false
	c.ociosa = func(n int) bool {
		c.mu.Lock()
		primeira := !livre
		livre = true
		c.mu.Unlock()
		if primeira {
			// A fila fica ociosa e avisa antes de a fase aguardando_fila
			// ser gravada: o aviso chega numa fase em que não vale.
			c.a.AoFicarOciosa()
			return false
		}
		return true
	}
	c.mu.Unlock()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarChamada("Sair", 1)
	time.Sleep(30 * time.Millisecond)
	if n := c.chamadas("Trocador.Reiniciar"); n != 1 {
		t.Fatalf("Trocador.Reiniciar chamado %d vezes; ordem %v", n, c.ordemAgora())
	}
}

func TestAtualizador_DoisAvisosSimultaneos(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.definirOciosa(false)
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseAguardandoFila)
	time.Sleep(20 * time.Millisecond)
	c.definirOciosa(true)
	largada := make(chan struct{})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			<-largada
			c.a.AoFicarOciosa()
		})
	}
	close(largada)
	wg.Wait()
	c.esperarChamada("Sair", 1)
	time.Sleep(30 * time.Millisecond)
	if c.chamadas("PararMonitor") != 1 || c.chamadas("Trocador.Reiniciar") != 1 {
		t.Fatalf("preparação repetida: %v", c.ordemAgora())
	}
}

func TestAtualizador_AvisoForaDeHora(t *testing.T) {
	querNada := func(t *testing.T, c *cenarioAt) {
		t.Helper()
		antes := c.ordemAgora()
		c.a.AoFicarOciosa()
		time.Sleep(30 * time.Millisecond)
		if got := c.ordemAgora(); !slices.Equal(got, antes) {
			t.Fatalf("AoFicarOciosa chamou %v", got[len(antes):])
		}
	}
	t.Run("nada", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.iniciar()
		querNada(t, c)
	})
	t.Run("disponivel", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.verificado()
		querNada(t, c)
	})
	t.Run("em dia", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel = nil
		c.verificado()
		querNada(t, c)
	})
	t.Run("instalada", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.instalada()
		querNada(t, c)
	})
	t.Run("baixando", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		liberar := make(chan struct{})
		c.baixar = func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
			<-liberar
			return gravarBaixado(rel, asset, pasta, nome)
		}
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		c.esperarFase(FaseBaixando)
		querNada(t, c)
		close(liberar)
		c.esperarFase(FaseInstalada)
	})
}

func TestAtualizador_PausadaComPendentes(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.definirOciosa(false)
	c.pausada = true
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseAguardandoFila)
	if m := c.a.Estado().Mensagem; m != "Retome ou limpe a fila para atualizar" {
		t.Fatalf("mensagem = %q", m)
	}
}

func TestAtualizador_ItemEntraNaConferencia(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.instalada()
	c.mu.Lock()
	c.ociosa = func(n int) bool { return n == 1 }
	c.mu.Unlock()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseAguardandoFila)
	// A última Ociosa é a reconferência depois de gravar aguardando_fila.
	c.esperar(func() bool { return len(c.ordemAgora()) >= 8 }, "a reconferência")
	time.Sleep(20 * time.Millisecond)
	querOrdem(t, c, []string{"Trocador.Instalar", "Ociosa", "PararMonitor", "Reter", "Ociosa", "Soltar", "ReligarMonitor", "Ociosa"})
	if f := c.a.Estado().Fase; f != FaseAguardandoFila {
		t.Fatalf("fase = %q", f)
	}
}

func TestAtualizador_NaoAbriuDesfaz(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.reinic = func(context.Context, Versao, sync.Locker) error { return ErrNaoAbriu{Versao: "0.5.0"} }
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseErro)
	querOrdem(t, c, []string{"Trocador.Instalar", "Ociosa", "PararMonitor", "Reter", "Ociosa", "Gravador.Fechar", "Trocador.Reiniciar", "Gravador.Reabrir", "Soltar", "ReligarMonitor"})
	e := c.a.Estado()
	if e.Mensagem != "A versão 0.5.0 não abriu; o eprocIA continua na versão 0.4.4" || e.Modo != "trocar" {
		t.Fatalf("estado = %+v", e)
	}
	// "Atualizar" de novo baixa outra vez.
	if err := c.a.Instalar(); err != nil {
		t.Fatalf("Instalar depois da falha: %v", err)
	}
	c.esperarFase(FaseInstalada)
	if c.nBaixar != 2 {
		t.Fatalf("Baixar chamado %d vezes; quer 2", c.nBaixar)
	}
}

func TestAtualizador_InstaladorCanceladoDesfaz(t *testing.T) {
	c := novoCenarioAt(t, ModoInstalador)
	c.shell = func(string) error { return ErrCancelado }
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseErro)
	querOrdem(t, c, []string{"Ociosa", "Reabrir", "PararMonitor", "Reter", "Ociosa", "Gravador.Fechar", "Shell", "Gravador.Reabrir", "Soltar", "ReligarMonitor"})
	if m := c.a.Estado().Mensagem; m != "A instalação foi cancelada; o eprocIA continua na versão 0.4.4" {
		t.Fatalf("mensagem = %q", m)
	}
}

func TestAtualizador_InstaladorAbreESai(t *testing.T) {
	c := novoCenarioAt(t, ModoInstalador)
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	c.esperarChamada("Sair", 1)
	o := c.ordemAgora()
	if slices.Index(o, "Reabrir") > slices.Index(o, "PararMonitor") || slices.Index(o, "Shell") > slices.Index(o, "Sair") {
		t.Fatalf("ordem = %v", o)
	}
}

func TestAtualizador_MensagemPorErro(t *testing.T) {
	const (
		naoAplicou = "A atualização não pôde ser aplicada; o eprocIA continua na versão 0.4.4"
		reversao   = "A atualização falhou e a versão anterior não pôde ser restaurada; reinstale o eprocIA"
		naoConf    = "O download não conferiu; nada foi alterado"
	)
	casos := []struct {
		nome     string
		modo     Modo
		instalar error
		reinic   error
		baixar   error
		reabrir  error
		shell    error
		fase     string
		frase    string
	}{
		{nome: "instalar reversao", modo: ModoTrocar, instalar: ErrReversaoIncompleta, fase: FaseErro, frase: reversao},
		{nome: "instalar nao conferiu", modo: ModoTrocar, instalar: fmt.Errorf("%w: C:\\Users\\fulano\\x", ErrNaoConferiu), fase: FaseErro, frase: naoConf},
		{nome: "instalar nao aplicou", modo: ModoTrocar, instalar: fmt.Errorf("%w: %w", ErrNaoAplicou, errors.New("rename")), fase: FaseErro, frase: naoAplicou},
		{nome: "reiniciar nao abriu", modo: ModoTrocar, reinic: ErrNaoAbriu{Versao: "0.5.0"}, fase: FaseErro, frase: "A versão 0.5.0 não abriu; o eprocIA continua na versão 0.4.4"},
		{nome: "reiniciar reversao", modo: ModoTrocar, reinic: ErrReversaoIncompleta, fase: FaseErro, frase: reversao},
		{nome: "reiniciar nao aplicou", modo: ModoTrocar, reinic: fmt.Errorf("%w: iniciar", ErrNaoAplicou), fase: FaseErro, frase: naoAplicou},
		{nome: "reiniciar sem anterior", modo: ModoTrocar, reinic: ErrSemAnterior, fase: FaseInstalada, frase: "A versão nova abre na próxima vez que o eprocIA for aberto"},
		{nome: "baixar parado", modo: ModoTrocar, baixar: ErrParado, fase: FaseErro, frase: "O download foi interrompido; nada foi alterado"},
		{nome: "reabrir nao conferiu", modo: ModoInstalador, reabrir: fmt.Errorf("%w: reabrir", ErrNaoConferiu), fase: FaseErro, frase: naoConf},
		{nome: "shell cancelado", modo: ModoInstalador, shell: ErrCancelado, fase: FaseErro, frase: "A instalação foi cancelada; o eprocIA continua na versão 0.4.4"},
		{nome: "shell arquivo ausente", modo: ModoInstalador, shell: erroShell(syscall.Errno(2)), fase: FaseErro, frase: naoAplicou},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenarioAt(t, caso.modo)
			if caso.instalar != nil {
				c.instalar = func(context.Context, Baixado) error { return caso.instalar }
			}
			if caso.reinic != nil {
				c.reinic = func(context.Context, Versao, sync.Locker) error { return caso.reinic }
			}
			if caso.baixar != nil {
				c.baixar = func(context.Context, *Release, Asset, string, string, func(int64, int64)) (Baixado, error) {
					return Baixado{}, caso.baixar
				}
			}
			if caso.reabrir != nil {
				c.reabrir = func(Baixado) (*Travado, error) { return nil, caso.reabrir }
			}
			if caso.shell != nil {
				c.shell = func(string) error { return caso.shell }
			}
			c.verificado()
			if err := c.a.Instalar(); err != nil {
				t.Fatal(err)
			}
			if caso.reinic != nil {
				c.esperarFase(FaseInstalada)
				if err := c.a.Reiniciar(); err != nil {
					t.Fatal(err)
				}
				c.esperarChamada("ReligarMonitor", 1)
			}
			c.esperar(func() bool {
				e := c.a.Estado()
				return e.Fase == caso.fase && e.Mensagem == caso.frase
			}, fmt.Sprintf("fase %s com %q", caso.fase, caso.frase))
			if m := c.a.Estado().Modo; m != string(caso.modo) {
				t.Fatalf("Modo = %q; quer %q", m, caso.modo)
			}
			if c.chamadas("Sair") != 0 {
				t.Fatal("Sair chamado numa falha")
			}
		})
	}
}

func TestAtualizador_SairChamaEncerrarNaoTrava(t *testing.T) {
	t.Run("sair chama encerrar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		saiu := make(chan struct{})
		c.sair = func() {
			c.a.Encerrar()
			close(saiu)
		}
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, saiu, "o Encerrar chamado pelo Sair")
	})
	t.Run("encerrar durante o download", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		liberar := make(chan struct{})
		entrou := make(chan context.Context, 1)
		c.baixar = func(ctx context.Context, _ *Release, _ Asset, _, _ string, _ func(int64, int64)) (Baixado, error) {
			entrou <- ctx
			<-liberar // não olha o ctx
			return Baixado{}, context.Canceled
		}
		defer close(liberar)
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		var ctx context.Context
		select {
		case ctx = <-entrou:
		case <-time.After(prazoTeste):
			t.Fatal("Baixar não foi chamado")
		}
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou com o Baixar bloqueado")
		}
		if ctx.Err() == nil {
			t.Fatal("o ctx do Baixar não foi cancelado")
		}
	})
}

func TestAtualizador_DownloadNaoConferiu(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.baixar = func(context.Context, *Release, Asset, string, string, func(int64, int64)) (Baixado, error) {
		return Baixado{}, fmt.Errorf("%w: C:\\Users\\fulano\\AppData\\x", ErrNaoConferiu)
	}
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseErro)
	time.Sleep(20 * time.Millisecond)
	querOrdem(t, c, nil)
	if m := c.a.Estado().Mensagem; m != "O download não conferiu; nada foi alterado" {
		t.Fatalf("mensagem = %q", m)
	}
}

func TestAtualizador_UmaOperacaoPorVez(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	liberar := make(chan struct{})
	c.baixar = func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
		<-liberar
		return gravarBaixado(rel, asset, pasta, nome)
	}
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	c.esperar(func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.nBaixar == 1
	}, "o Baixar")
	if err := c.a.Instalar(); !errors.Is(err, ErrEmCurso) {
		t.Fatalf("segundo Instalar = %v", err)
	}
	if _, err := c.a.Verificar(context.Background()); !errors.Is(err, ErrEmCurso) {
		t.Fatalf("Verificar = %v", err)
	}
	close(liberar)
	c.esperarFase(FaseInstalada)
	if c.nBaixar != 1 || c.nVerif != 1 {
		t.Fatalf("Baixar %d, Verificar %d", c.nBaixar, c.nVerif)
	}
}

// baixarEsperandoCtx grava o arquivo, avisa e espera o ctx; cancelado, apaga
// o arquivo e devolve o erro (como o Baixar real).
func baixarEsperandoCtx(entrou chan<- context.Context) func(context.Context, *Release, Asset, string, string, func(int64, int64)) (Baixado, error) {
	return func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
		caminho := filepath.Join(pasta, nome)
		_ = os.WriteFile(caminho, []byte("parcial"), 0o644)
		entrou <- ctx
		<-ctx.Done()
		_ = os.Remove(caminho)
		return Baixado{}, ctx.Err()
	}
}

func TestAtualizador_CancelarDuranteDownload(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	entrou := make(chan context.Context, 1)
	c.baixar = baixarEsperandoCtx(entrou)
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	ctx := <-entrou
	c.a.Cancelar()
	c.esperarFase(FaseDisponivel)
	if ctx.Err() == nil {
		t.Fatal("ctx do Baixar não cancelado")
	}
	time.Sleep(20 * time.Millisecond)
	if f := c.a.Estado().Fase; f != FaseDisponivel {
		t.Fatalf("fase depois do fim do Baixar = %q", f)
	}
	// Dá para tentar de novo.
	c.mu.Lock()
	c.baixar = nil
	c.mu.Unlock()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseInstalada)
}

func TestAtualizador_EncerrarApagaTemporario(t *testing.T) {
	t.Run("instalador aguardando a fila", func(t *testing.T) {
		c := novoCenarioAt(t, ModoInstalador)
		c.definirOciosa(false)
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		c.esperarFase(FaseAguardandoFila)
		baixado := c.baixados[0]
		if _, err := os.Stat(baixado); err != nil {
			t.Fatalf("baixado ausente antes do Encerrar: %v", err)
		}
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		if _, err := os.Stat(baixado); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("baixado ficou: %v", err)
		}
		if _, err := os.Stat(filepath.Dir(baixado)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pasta do instalador ficou: %v", err)
		}
		c.definirOciosa(true)
		c.a.AoFicarOciosa()
		time.Sleep(30 * time.Millisecond)
		if c.chamadas("Shell") != 0 || c.chamadas("Reabrir") != 0 {
			t.Fatalf("seguiu depois do Encerrar: %v", c.ordemAgora())
		}
	})
	t.Run("trocar baixando", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		entrou := make(chan struct{})
		liberar := make(chan struct{})
		terminou := make(chan struct{})
		c.baixar = func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
			defer close(terminou)
			caminho := filepath.Join(pasta, nome)
			_ = os.WriteFile(caminho, []byte("parcial"), 0o644)
			close(entrou)
			<-liberar // não olha o ctx
			return Baixado{Caminho: caminho, Asset: asset, Versao: rel.Versao}, nil
		}
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, entrou, "o Baixar")
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		baixado := c.baixados[0]
		if _, err := os.Stat(baixado); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporário ficou: %v", err)
		}
		close(liberar)
		receberEm(t, terminou, "o fim do Baixar")
		time.Sleep(50 * time.Millisecond)
		if c.chamadas("Trocador.Instalar") != 0 {
			t.Fatal("Trocador.Instalar chamado depois do Encerrar")
		}
	})
}

// instalarRenomeando é o Trocador.Instalar falso que põe o baixado no lugar
// do eprocia.exe.
func instalarRenomeando(pasta string) func(context.Context, Baixado) error {
	return func(_ context.Context, b Baixado) error {
		return os.Rename(b.Caminho, filepath.Join(pasta, NomeExe))
	}
}

func querSoOExe(t *testing.T, pasta string) {
	t.Helper()
	nomes, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	if len(nomes) != 1 || nomes[0].Name() != NomeExe {
		t.Fatalf("pasta = %v; quer só %s", nomes, NomeExe)
	}
	dado, err := os.ReadFile(filepath.Join(pasta, NomeExe))
	if err != nil || string(dado) != "novo 0.5.0" {
		t.Fatalf("eprocia.exe = %q, %v", dado, err)
	}
}

func TestAtualizador_EncerrarDepoisDeInstalarNaoTocaOExe(t *testing.T) {
	t.Run("sair sem reiniciar", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.instalar = instalarRenomeando(c.pasta)
		c.instalada()
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		querSoOExe(t, c.pasta)
	})
	t.Run("reiniciar e sair", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.instalar = instalarRenomeando(c.pasta)
		saiu := make(chan struct{})
		c.sair = func() {
			c.a.Encerrar()
			close(saiu)
		}
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, saiu, "o Sair")
		querSoOExe(t, c.pasta)
	})
}

func TestAtualizador_ReinicioComJanelaOcultaPassaTray(t *testing.T) {
	for _, oculta := range []bool{true, false} {
		t.Run(fmt.Sprint("oculta=", oculta), func(t *testing.T) {
			c := novoCenarioAt(t, ModoTrocar)
			c.oculta = oculta
			c.instalada()
			if err := c.a.Reiniciar(); err != nil {
				t.Fatal(err)
			}
			c.esperarChamada("Sair", 1)
			c.mu.Lock()
			defer c.mu.Unlock()
			if len(c.extras) != 2 {
				t.Fatalf("fábrica chamada %d vezes", len(c.extras))
			}
			var quer []string
			if oculta {
				quer = []string{"--tray"}
			}
			if got := c.extras[1]; !slices.Equal(got, quer) {
				t.Fatalf("extras = %v; quer %v", got, quer)
			}
		})
	}
}

func TestAtualizador_InstaladaSoReinicia(t *testing.T) {
	t.Run("instalada", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.instalada()
		if _, err := c.a.Verificar(context.Background()); !errors.Is(err, ErrFaltaReiniciar) {
			t.Fatalf("Verificar = %v", err)
		}
		if err := c.a.Instalar(); !errors.Is(err, ErrFaltaReiniciar) {
			t.Fatalf("Instalar = %v", err)
		}
		if c.nVerif != 1 || c.nBaixar != 1 {
			t.Fatalf("Rede chamada: Verificar %d, Baixar %d", c.nVerif, c.nBaixar)
		}
	})
	querNaoInstalavel := func(t *testing.T, c *cenarioAt) {
		t.Helper()
		antes := c.ordemAgora()
		if err := c.a.Reiniciar(); !errors.Is(err, ErrNaoInstalavel) {
			t.Fatalf("Reiniciar = %v; quer ErrNaoInstalavel", err)
		}
		time.Sleep(20 * time.Millisecond)
		if got := c.ordemAgora(); !slices.Equal(got, antes) {
			t.Fatalf("Reiniciar recusado chamou %v", got[len(antes):])
		}
	}
	t.Run("nada", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.iniciar()
		querNaoInstalavel(t, c)
	})
	t.Run("disponivel", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.verificado()
		querNaoInstalavel(t, c)
	})
	t.Run("em dia", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel = nil
		c.verificado()
		querNaoInstalavel(t, c)
	})
	t.Run("erro", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.rel, c.errRede = nil, errors.New("dial")
		c.iniciar()
		_, _ = c.a.Verificar(context.Background())
		querNaoInstalavel(t, c)
	})
	t.Run("instalador", func(t *testing.T) {
		c := novoCenarioAt(t, ModoInstalador)
		c.verificado()
		querNaoInstalavel(t, c)
	})
	t.Run("aguardando a fila e reiniciando", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		c.definirOciosa(false)
		entrou := make(chan struct{})
		liberar := make(chan struct{})
		c.reinic = func(context.Context, Versao, sync.Locker) error {
			close(entrou)
			<-liberar
			return nil
		}
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatal(err)
		}
		c.esperarFase(FaseAguardandoFila)
		time.Sleep(20 * time.Millisecond)
		antes := c.ordemAgora()
		if err := c.a.Reiniciar(); !errors.Is(err, ErrEmCurso) {
			t.Fatalf("Reiniciar em aguardando_fila = %v", err)
		}
		if _, err := c.a.Verificar(context.Background()); !errors.Is(err, ErrFaltaReiniciar) {
			t.Fatalf("Verificar em aguardando_fila = %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		if got := c.ordemAgora(); !slices.Equal(got, antes) {
			t.Fatalf("Reiniciar recusado chamou %v", got[len(antes):])
		}
		c.definirOciosa(true)
		c.a.AoFicarOciosa()
		receberEm(t, entrou, "o Trocador.Reiniciar")
		c.esperarFase(FaseReiniciando)
		antes = c.ordemAgora()
		if err := c.a.Reiniciar(); !errors.Is(err, ErrEmCurso) {
			t.Fatalf("Reiniciar em reiniciando = %v", err)
		}
		if err := c.a.Instalar(); !errors.Is(err, ErrFaltaReiniciar) {
			t.Fatalf("Instalar em reiniciando = %v", err)
		}
		if got := c.ordemAgora(); !slices.Equal(got, antes) {
			t.Fatalf("Reiniciar recusado chamou %v", got[len(antes):])
		}
		close(liberar)
		c.esperarChamada("Sair", 1)
		if n := c.chamadas("Trocador.Reiniciar"); n != 1 {
			t.Fatalf("Trocador.Reiniciar chamado %d vezes", n)
		}
	})
}

func TestAtualizador_SairEmReiniciandoNaoRenomeia(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.instalar = instalarRenomeando(c.pasta)
	entrou := make(chan context.Context, 1)
	liberar := make(chan struct{})
	terminou := make(chan struct{})
	var renomes atomic.Int32
	c.reinic = func(ctx context.Context, _ Versao, trava sync.Locker) error {
		defer close(terminou)
		entrou <- ctx
		<-liberar
		if ctx.Err() != nil {
			renomes.Add(1) // a reversão por cancelamento renomearia aqui
		}
		return ErrNaoAbriu{Versao: "0.5.0"}
	}
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	select {
	case ctx = <-entrou:
	case <-time.After(prazoTeste):
		t.Fatal("Trocador.Reiniciar não foi chamado")
	}
	c.esperarFase(FaseReiniciando)
	if !devolveEm(prazoTeste, c.a.Encerrar) {
		t.Fatal("Encerrar travou em reiniciando")
	}
	if ctx.Err() != nil {
		t.Fatalf("o ctx do Reiniciar foi cancelado: %v", ctx.Err())
	}
	c.mu.Lock()
	nEstados := len(c.estados)
	c.mu.Unlock()
	close(liberar)
	receberEm(t, terminou, "o fim do Trocador.Reiniciar")
	time.Sleep(50 * time.Millisecond)
	if renomes.Load() != 0 {
		t.Fatal("houve rename depois do Encerrar")
	}
	querSoOExe(t, c.pasta)
	c.mu.Lock()
	depois := len(c.estados)
	c.mu.Unlock()
	if depois != nEstados {
		t.Fatalf("emitiu %d estado(s) depois do Encerrar", depois-nEstados)
	}
	if f := c.a.Estado().Fase; f != FaseReiniciando {
		t.Fatalf("fase = %q", f)
	}
	if c.chamadas("Sair") != 0 || c.chamadas("Gravador.Reabrir") != 0 {
		t.Fatalf("ordem = %v", c.ordemAgora())
	}
}

func TestAtualizador_SairEsperaReversao(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	travou := make(chan struct{})
	renomear := make(chan struct{})
	var terminou atomic.Bool
	c.reinic = func(_ context.Context, _ Versao, trava sync.Locker) error {
		trava.Lock()
		close(travou)
		<-renomear // o renomear lento da reversão
		terminou.Store(true)
		trava.Unlock()
		return ErrNaoAbriu{Versao: "0.5.0"}
	}
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	receberEm(t, travou, "a reversão travar")
	voltou := make(chan bool, 1)
	go func() {
		c.a.Encerrar()
		voltou <- terminou.Load()
	}()
	select {
	case <-voltou:
		t.Fatal("Encerrar voltou no meio da reversão")
	case <-time.After(500 * time.Millisecond):
	}
	close(renomear)
	select {
	case fim := <-voltou:
		if !fim {
			t.Fatal("Encerrar voltou antes de a reversão terminar")
		}
	case <-time.After(prazoTeste):
		t.Fatal("Encerrar não voltou depois da reversão")
	}
}

func TestAtualizador_EncerrarEsperaInstalar(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	entrou := make(chan struct{})
	liberar := make(chan struct{})
	var terminou atomic.Bool
	c.instalar = func(context.Context, Baixado) error {
		close(entrou)
		<-liberar // entre os dois renames
		terminou.Store(true)
		return nil
	}
	c.verificado()
	if err := c.a.Instalar(); err != nil {
		t.Fatal(err)
	}
	receberEm(t, entrou, "o Trocador.Instalar")
	voltou := make(chan bool, 1)
	go func() {
		c.a.Encerrar()
		voltou <- terminou.Load()
	}()
	select {
	case <-voltou:
		t.Fatal("Encerrar voltou no meio do Instalar")
	case <-time.After(300 * time.Millisecond):
	}
	close(liberar)
	select {
	case fim := <-voltou:
		if !fim {
			t.Fatal("Encerrar voltou antes do fim do Instalar")
		}
	case <-time.After(prazoTeste):
		t.Fatal("Encerrar não voltou depois do Instalar")
	}
}

func TestAtualizador_EncerrarFicaComATrava(t *testing.T) {
	t.Run("reversao tardia", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		entrou := make(chan struct{})
		liberar := make(chan struct{})
		naoTravou := make(chan struct{})
		var renomes atomic.Int32
		c.reinic = func(_ context.Context, _ Versao, trava sync.Locker) error {
			close(entrou)
			<-liberar // o esperarSinal, antes da reversão
			pegou := make(chan struct{})
			go func() {
				trava.Lock()
				close(pegou)
			}()
			select {
			case <-pegou:
				renomes.Add(1)
				trava.Unlock()
			case <-time.After(500 * time.Millisecond):
				close(naoTravou)
			}
			return ErrNaoAbriu{Versao: "0.5.0"}
		}
		c.instalada()
		if err := c.a.Reiniciar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, entrou, "o Trocador.Reiniciar")
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		close(liberar)
		receberEm(t, naoTravou, "a reversão ficar sem a trava")
		if renomes.Load() != 0 {
			t.Fatal("a reversão pegou a trava depois do Encerrar")
		}
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("segundo Encerrar travou")
		}
	})
	t.Run("instalar tardio", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		entrou := make(chan struct{})
		liberar := make(chan struct{})
		c.baixar = func(ctx context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
			close(entrou)
			<-liberar
			return gravarBaixado(rel, asset, pasta, nome)
		}
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, entrou, "o Baixar")
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		close(liberar)
		time.Sleep(100 * time.Millisecond)
		if c.chamadas("Trocador.Instalar") != 0 {
			t.Fatal("Trocador.Instalar chamado depois do Encerrar")
		}
	})
	t.Run("instalar parado na trava", func(t *testing.T) {
		c := novoCenarioAt(t, ModoTrocar)
		passou := make(chan struct{})
		liberar := make(chan struct{})
		criado := make(chan struct{})
		c.aoCriar = func([]string) {
			// A goroutine já passou da conferência de encerrado.
			close(passou)
			<-liberar
			close(criado)
		}
		c.verificado()
		if err := c.a.Instalar(); err != nil {
			t.Fatal(err)
		}
		receberEm(t, passou, "a fábrica do Trocador")
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("Encerrar travou")
		}
		close(liberar)
		receberEm(t, criado, "a volta da fábrica")
		time.Sleep(100 * time.Millisecond)
		if c.chamadas("Trocador.Instalar") != 0 {
			t.Fatal("Trocador.Instalar passou da trava depois do Encerrar")
		}
		if !devolveEm(prazoTeste, c.a.AoFicarOciosa) {
			t.Fatal("AoFicarOciosa travou")
		}
		if !devolveEm(prazoTeste, func() { _ = c.a.Estado() }) {
			t.Fatal("Estado travou")
		}
		if !devolveEm(prazoTeste, c.a.Encerrar) {
			t.Fatal("segundo Encerrar travou")
		}
	})
}

func TestAtualizador_EncerrarConcorrenteComFimDeInstalar(t *testing.T) {
	for i := range 50 {
		c := novoCenarioAt(t, ModoTrocar)
		// Sem arquivo: a goroutine não escreve na pasta temporária do teste.
		c.baixar = func(_ context.Context, rel *Release, asset Asset, pasta, nome string, _ func(int64, int64)) (Baixado, error) {
			return Baixado{Caminho: filepath.Join(pasta, nome), Asset: asset, Versao: rel.Versao}, nil
		}
		c.verificado()
		ok := devolveEm(prazoTeste, func() {
			var wg sync.WaitGroup
			wg.Go(c.a.Encerrar)
			wg.Go(func() { _ = c.a.Instalar() })
			wg.Wait()
			_ = c.a.Estado()
		})
		if !ok {
			t.Fatalf("rodada %d: impasse entre Encerrar e Instalar", i)
		}
	}
}

func TestAtualizador_ReiniciarSemAnterior(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.reinic = func(context.Context, Versao, sync.Locker) error { return ErrSemAnterior }
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarChamada("ReligarMonitor", 1)
	querOrdem(t, c, []string{"Trocador.Instalar", "Ociosa", "PararMonitor", "Reter", "Ociosa", "Gravador.Fechar", "Trocador.Reiniciar", "Gravador.Reabrir", "Soltar", "ReligarMonitor"})
	c.esperar(func() bool { return c.a.Estado().Mensagem != "" }, "a mensagem")
	e := c.a.Estado()
	if e.Fase != FaseInstalada || e.Modo != "trocar" || e.Versao != "0.5.0" {
		t.Fatalf("estado = %+v", e)
	}
	if e.Mensagem != "A versão nova abre na próxima vez que o eprocIA for aberto" {
		t.Fatalf("mensagem = %q", e.Mensagem)
	}
	if err := c.a.Instalar(); !errors.Is(err, ErrFaltaReiniciar) {
		t.Fatalf("Instalar = %v; quer ErrFaltaReiniciar", err)
	}
}

func TestAtualizador_CancelarReinicioVoltaAInstalada(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.instalar = instalarRenomeando(c.pasta)
	c.definirOciosa(false)
	c.instalada()
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseAguardandoFila)
	time.Sleep(20 * time.Millisecond)
	c.a.Cancelar()
	if e := c.a.Estado(); e.Fase != FaseInstalada || e.Versao != "0.5.0" {
		t.Fatalf("estado = %+v", e)
	}
	querSoOExe(t, c.pasta)
	c.definirOciosa(true)
	c.a.AoFicarOciosa()
	time.Sleep(30 * time.Millisecond)
	if c.chamadas("PararMonitor") != 0 || c.chamadas("Trocador.Reiniciar") != 0 {
		t.Fatalf("reiniciou depois do Cancelar: %v", c.ordemAgora())
	}
	c.definirOciosa(false)
	if err := c.a.Reiniciar(); err != nil {
		t.Fatal(err)
	}
	c.esperarFase(FaseAguardandoFila)
}

func TestAtualizador_ProgressoEmInteiros(t *testing.T) {
	c := novoCenarioAt(t, ModoTrocar)
	c.baixar = func(_ context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(int64, int64)) (Baixado, error) {
		for i := range int64(1000) {
			progresso(i+1, 1000)
		}
		return gravarBaixado(rel, asset, pasta, nome)
	}
	c.instalada()
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ultimo := 0, -1
	for _, e := range c.estados {
		if e.Fase == FaseBaixando {
			n++
			if e.Progresso < ultimo {
				t.Fatalf("progresso voltou: %d depois de %d", e.Progresso, ultimo)
			}
			ultimo = e.Progresso
		}
	}
	if n > 101 || n < 2 {
		t.Fatalf("%d estados baixando; quer de 2 a 101", n)
	}
}
