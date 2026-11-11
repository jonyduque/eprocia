package janela

import "testing"

// Sem janela do Wails no processo de teste, a animação é pulada e Ocultar
// ainda chega ao runtime.
func TestNativa_SemJanelaPulaAnimacao(t *testing.T) {
	ocultou := false
	j := Nativa(func() { ocultou = true })
	if !j.Visivel() {
		t.Fatal("sem janela, Visivel deve deixar o Animador seguir até Preparar")
	}
	if _, ok := j.Preparar(); ok {
		t.Fatal("Preparar aceitou sem janela")
	}
	_ = j.EfeitosAtivos() // só não pode falhar
	NovoAnimador(j).Ocultar()
	if !ocultou {
		t.Fatal("Ocultar não chamou o runtime")
	}
	j.Repor(Estado{}) // sem janela, não faz nada
}
