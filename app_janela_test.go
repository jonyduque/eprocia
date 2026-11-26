package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"eprocia/internal/janela"
)

type janelaFalsa struct {
	mu       sync.Mutex
	ocultou  int
	ocultada chan struct{}
}

func (j *janelaFalsa) Visivel() bool                        { return true }
func (j *janelaFalsa) EfeitosAtivos() bool                  { return false }
func (j *janelaFalsa) Preparar() (janela.Estado, bool)      { return janela.Estado{}, false }
func (j *janelaFalsa) Aplicar(janela.Estado, janela.Quadro) {}
func (j *janelaFalsa) Repor(janela.Estado)                  {}
func (j *janelaFalsa) Ocultar() {
	j.mu.Lock()
	j.ocultou++
	j.mu.Unlock()
	if j.ocultada != nil {
		j.ocultada <- struct{}{}
	}
}

func (j *janelaFalsa) vezes() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.ocultou
}

func appComJanela() (*App, *janelaFalsa) {
	j := &janelaFalsa{ocultada: make(chan struct{}, 4)}
	app := NewApp()
	app.animador.Store(janela.NovoAnimador(j))
	return app, j
}

// Alt+F4 e o fechar da barra de tarefas viram ocultar para a bandeja.
func TestAntesDeFechar_OcultaECancelaASaida(t *testing.T) {
	app, j := appComJanela()
	if !app.antesDeFechar(context.Background()) {
		t.Fatal("antesDeFechar deixou o app sair sem Sair")
	}
	select {
	case <-j.ocultada:
	case <-time.After(5 * time.Second):
		t.Fatal("a janela não foi ocultada")
	}
}

// Sair marca a saída antes do runtime.Quit, que passa por OnBeforeClose.
func TestAntesDeFechar_DeixaSairDepoisDeQuit(t *testing.T) {
	app, j := appComJanela()
	app.Quit()
	if app.antesDeFechar(context.Background()) {
		t.Fatal("antesDeFechar segurou a saída depois de Quit")
	}
	app.Hide()
	time.Sleep(20 * time.Millisecond)
	if n := j.vezes(); n != 0 {
		t.Fatalf("ocultou %d vezes saindo; Sair não pode esperar a animação", n)
	}
}

func TestHide_UsaOAnimador(t *testing.T) {
	app, j := appComJanela()
	app.Hide()
	if n := j.vezes(); n != 1 {
		t.Fatalf("ocultou %d vezes", n)
	}
}

func TestShowHideQuit_SemContextoNaoQuebram(t *testing.T) {
	app := NewApp()
	app.Show()
	app.Hide()
	app.Quit()
}

// emissoes grava os avisos de visibilidade mandados ao frontend.
type emissoes struct {
	mu  sync.Mutex
	vis []bool
}

func (e *emissoes) emitir(evento string, dados ...any) {
	if evento != eventoVisibilidade {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.vis = append(e.vis, dados[0].(bool))
}

func (e *emissoes) lista() []bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]bool(nil), e.vis...)
}

// Ocultar para a bandeja avisa o frontend só quando a janela some de fato, e
// abrir avisa que ela voltou: com a janela oculta as animações param.
func TestVisibilidade_AvisaAoOcultarEAoAbrir(t *testing.T) {
	e := &emissoes{}
	app := NewApp()
	app.emitir = e.emitir
	app.animador.Store(janela.NovoAnimador(janelaQueOculta{app}))

	app.Hide()
	app.Show()
	got := e.lista()
	if len(got) != 2 || got[0] != false || got[1] != true {
		t.Fatalf("avisos de visibilidade %v, esperado [false true]", got)
	}
}

func TestVisibilidade_SemAnimadorAvisaAoOcultar(t *testing.T) {
	e := &emissoes{}
	app := NewApp()
	app.emitir = e.emitir
	app.Hide()
	if got := e.lista(); len(got) != 1 || got[0] != false {
		t.Fatalf("avisos de visibilidade %v, esperado [false]", got)
	}
}

// Sair não avisa nada: o frontend vai embora junto.
func TestVisibilidade_SairNaoAvisa(t *testing.T) {
	e := &emissoes{}
	app := NewApp()
	app.emitir = e.emitir
	app.Quit()
	app.Hide()
	if got := e.lista(); len(got) != 0 {
		t.Fatalf("avisos de visibilidade %v ao sair", got)
	}
}

// janelaQueOculta faz o papel da janela nativa: o ocultar real é o do App.
type janelaQueOculta struct{ app *App }

func (j janelaQueOculta) Visivel() bool                        { return true }
func (j janelaQueOculta) EfeitosAtivos() bool                  { return false }
func (j janelaQueOculta) Preparar() (janela.Estado, bool)      { return janela.Estado{}, false }
func (j janelaQueOculta) Aplicar(janela.Estado, janela.Quadro) {}
func (j janelaQueOculta) Repor(janela.Estado)                  {}
func (j janelaQueOculta) Ocultar()                             { j.app.ocultarJanela() }

// Iniciado oculto (autostart com --tray), o frontend começa sem animar.
func TestJanelaVisivel_AcompanhaOcultarEAbrir(t *testing.T) {
	app := NewApp()
	app.emitir = func(string, ...any) {}
	if !app.JanelaVisivel() {
		t.Fatal("janela nova deveria estar visível")
	}
	app.iniciarOculta(true)
	if app.JanelaVisivel() {
		t.Fatal("iniciada oculta e JanelaVisivel deu true")
	}
	app.Show()
	if !app.JanelaVisivel() {
		t.Fatal("depois de Show JanelaVisivel deu false")
	}
	app.Hide()
	if app.JanelaVisivel() {
		t.Fatal("depois de Hide JanelaVisivel deu true")
	}
}
