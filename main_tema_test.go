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
