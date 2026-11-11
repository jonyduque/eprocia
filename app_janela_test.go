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
