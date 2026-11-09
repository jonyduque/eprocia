package main

import (
	"slices"
	"testing"
	"time"
)

type bandejaFalsa struct {
	icone   []byte
	titulo  string
	dica    string
	itens   []string
	cliques map[string]func()
	duplo   func()
	saiu    chan struct{}
}

func novaBandejaFalsa() *bandejaFalsa {
	return &bandejaFalsa{cliques: map[string]func(){}, saiu: make(chan struct{}, 1)}
}

func (b *bandejaFalsa) SetIcon(i []byte)      { b.icone = i }
func (b *bandejaFalsa) SetTitle(t string)     { b.titulo = t }
func (b *bandejaFalsa) SetTooltip(t string)   { b.dica = t }
func (b *bandejaFalsa) SetOnDClick(fn func()) { b.duplo = fn }
func (b *bandejaFalsa) Quit()                 { b.saiu <- struct{}{} }
func (b *bandejaFalsa) AddMenuItem(titulo, _ string, clique func()) {
	b.itens = append(b.itens, titulo)
	b.cliques[titulo] = clique
}

type chamadas chan string

func (c chamadas) esperar(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-c:
		if got != want {
			t.Fatalf("chamou %q, esperado %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%q não foi chamado", want)
	}
}

func acoesFalsas() (acoesBandeja, chamadas) {
	c := make(chamadas, 8)
	return acoesBandeja{
		abrir:   func() { c <- "abrir" },
		ocultar: func() { c <- "ocultar" },
		sair:    func() { c <- "sair" },
	}, c
}

func TestMontarMenu_AbrirOcultarSairComIcone(t *testing.T) {
	b := novaBandejaFalsa()
	ac, c := acoesFalsas()
	montarMenu(b, ac)

	if len(b.icone) == 0 || !slices.Equal(b.icone, DefaultIcon) {
		t.Fatal("ícone ICO não aplicado")
	}
	if b.dica == "" {
		t.Fatal("sem tooltip")
	}
	if want := []string{"Abrir", "Ocultar", "Sair"}; !slices.Equal(b.itens, want) {
		t.Fatalf("menu %v, esperado %v", b.itens, want)
	}
	b.cliques["Abrir"]()
	c.esperar(t, "abrir")
	b.cliques["Ocultar"]()
	c.esperar(t, "ocultar")

	b.cliques["Sair"]()
	select {
	case <-b.saiu:
	case <-time.After(5 * time.Second):
		t.Fatal("Sair não encerrou a bandeja")
	}
	c.esperar(t, "sair")
}

// Duplo clique esquerdo abre a janela; o clique simples não é ligado (o
// menu fica só no botão direito, padrão da biblioteca).
func TestPrepararBandeja_DuploCliqueAbre(t *testing.T) {
	b := novaBandejaFalsa()
	ac, c := acoesFalsas()
	prepararBandeja(b, ac)
	if b.duplo == nil {
		t.Fatal("duplo clique não ligado")
	}
	b.duplo()
	c.esperar(t, "abrir")
}
