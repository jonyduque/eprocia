package etl

import (
	"slices"
	"testing"
)

func TestLinhasBasicas(t *testing.T) {
	in := "  a   b\t\tc  \n\n\x01d  e\n_ _ _ _ _ _\n-----\nItem ........ 12"
	want := []string{"a b c", "d e", "Item ........ 12"}
	if got := linhasBasicas(in); !slices.Equal(got, want) {
		t.Errorf("linhasBasicas = %q, quer %q", got, want)
	}
}

func TestLinhaSeparadora(t *testing.T) {
	for _, l := range []string{"-----", "_ _ _ _ _", "=====", ".....", "*****", "• • • • •"} {
		if !linhaSeparadora(l) {
			t.Errorf("%q deveria ser separadora", l)
		}
	}
	for _, l := range []string{"----", "", "--- a ---", "Item ........ 12"} {
		if linhaSeparadora(l) {
			t.Errorf("%q não deveria ser separadora", l)
		}
	}
}

func TestLinhasBasicas_NBSP(t *testing.T) {
	got := linhasBasicas("a  b")
	if !slices.Equal(got, []string{"a b"}) {
		t.Errorf("got %q", got)
	}
}

func TestCleanPage_BasicaNaoJuntaLinhas(t *testing.T) {
	got := cleanPage("linha  um\nlinha dois", NivelBasico, &ctxDocumento{})
	if want := "linha um\nlinha dois"; got != want {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestCleanPage_NivelDesconhecidoViraBasica(t *testing.T) {
	got := cleanPage("a  b\nc", "xyz", nil)
	if want := "a b\nc"; got != want {
		t.Errorf("got %q, quer %q", got, want)
	}
}
