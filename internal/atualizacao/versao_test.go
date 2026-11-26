package atualizacao

import "testing"

func TestVersao_Comparar(t *testing.T) {
	casos := []struct {
		a, b Versao
		want int
	}{
		{Versao{0, 10, 0}, Versao{0, 9, 9}, 1},
		{Versao{1, 0, 0}, Versao{0, 99, 99}, 1},
		{Versao{0, 4, 4}, Versao{0, 4, 4}, 0},
		{Versao{0, 4, 3}, Versao{0, 4, 4}, -1},
	}
	for _, c := range casos {
		if got := c.a.Comparar(c.b); got != c.want {
			t.Errorf("%v.Comparar(%v) = %d, quer %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseVersao(t *testing.T) {
	v, err := ParseVersao("0.5.0")
	if err != nil || v != (Versao{0, 5, 0}) {
		t.Fatalf("ParseVersao(0.5.0) = %v, %v", v, err)
	}
	if v.String() != "0.5.0" {
		t.Errorf("String = %q", v.String())
	}
}

func TestParseVersao_Recusa(t *testing.T) {
	for _, s := range []string{"", "0.4", "0.4.4.1", "0.4.4-rc1", "0.4.4+x", "v0.4.4", " 0.4.4", "a.b.c", "99999999999999999999.0.0", "-1.0.0", "+1.0.0", "1..0"} {
		if _, err := ParseVersao(s); err == nil {
			t.Errorf("ParseVersao(%q) deveria falhar", s)
		}
	}
}

func TestParseTag(t *testing.T) {
	v, err := ParseTag("v0.5.0")
	if err != nil || v != (Versao{0, 5, 0}) {
		t.Fatalf("ParseTag(v0.5.0) = %v, %v", v, err)
	}
	for _, s := range []string{"0.5.0", "v0.5.0-rc1", "V0.5.0", "v", ""} {
		if _, err := ParseTag(s); err == nil {
			t.Errorf("ParseTag(%q) deveria falhar", s)
		}
	}
}

func TestVersao_Valida(t *testing.T) {
	if (Versao{0, 0, 0}).Valida() {
		t.Error("0.0.0 não deve ser válida")
	}
	if !(Versao{0, 0, 1}).Valida() {
		t.Error("0.0.1 deve ser válida")
	}
}
