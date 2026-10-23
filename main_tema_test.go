package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// O fundo nativo da janela aparece antes do WebView pintar; se divergir do
// --bg-main do tema, a abertura pisca.
func TestCorDeFundo_IgualAoBgMainDoTema(t *testing.T) {
	css, err := os.ReadFile("frontend/src/assets/css/style.css")
	if err != nil {
		t.Fatal(err)
	}
	blocos := map[bool]*regexp.Regexp{
		false: regexp.MustCompile(`(?s):root\s*\{[^}]*?--bg-main:\s*#([0-9A-Fa-f]{6})`),
		true:  regexp.MustCompile(`(?s):root\.light\s*\{[^}]*?--bg-main:\s*#([0-9A-Fa-f]{6})`),
	}
	for claro, re := range blocos {
		m := re.FindSubmatch(css)
		if m == nil {
			t.Fatalf("--bg-main não encontrado (claro=%v)", claro)
		}
		c := corDeFundo(claro)
		got := fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B)
		if !strings.EqualFold(got, string(m[1])) {
			t.Errorf("claro=%v: BackgroundColour %s, --bg-main %s", claro, got, m[1])
		}
		if c.A != 255 {
			t.Errorf("claro=%v: alpha %d, quero 255", claro, c.A)
		}
	}
}

// O tema escolhido no app vale mais que o do Windows; só "system" (ou vazio,
// em config antiga) consulta o Windows.
func TestTemaClaro_PrefereEscolhaGravada(t *testing.T) {
	consultas := 0
	windows := func(claro bool) func() bool {
		return func() bool { consultas++; return claro }
	}
	casos := []struct {
		tema    string
		windows bool
		quer    bool
	}{
		{"light", false, true},
		{"dark", true, false},
		{"system", true, true},
		{"system", false, false},
		{"", true, true},
		{"", false, false},
	}
	for _, c := range casos {
		if got := temaClaro(c.tema, windows(c.windows)); got != c.quer {
			t.Errorf("temaClaro(%q, windows=%v) = %v, quero %v", c.tema, c.windows, got, c.quer)
		}
	}
	consultas = 0
	temaClaro("dark", windows(true))
	if consultas != 0 {
		t.Error("tema fixo não deveria consultar o Windows")
	}
}
