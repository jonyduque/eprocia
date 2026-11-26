package registro

import "testing"

func TestLerModo(t *testing.T) {
	casos := []struct {
		valor       string
		modo        Modo
		reconhecido bool
	}{
		{"", ModoDesligado, true},
		{"   ", ModoDesligado, true},
		{"1", ModoNormal, true},
		{"detalhado", ModoDetalhado, true},
		{"DETALHADO", ModoDetalhado, true},
		{"sim", ModoNormal, false},
	}
	for _, c := range casos {
		m, ok := LerModo(c.valor)
		if m != c.modo || ok != c.reconhecido {
			t.Errorf("LerModo(%q) = (%v, %v), quero (%v, %v)", c.valor, m, ok, c.modo, c.reconhecido)
		}
	}
}
