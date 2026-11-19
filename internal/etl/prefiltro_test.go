package etl

import (
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// Linhas que exercitam as bordas dos pré-filtros (B-EF-04): ligaduras, ſ (que
// casa com s sob (?i)), caixa mista, valores e marcadores de página.
var linhasPrefiltro = []string{
	"", "a", "A", "ab", "abc", "ﬁ", "ﬁﬁ", "ﬁﬁﬁ", "Aé", "à", "É", "IV", "a)", "SP", "1º", "x1",
	"$ 100 pagos", "R$ 100", "US$ 5", "U S$ 5", "12,50", ",50", "1,5", "1,5a", "abc 3,14 def", "10%", "%",
	"pág. 3", "Página 3 de 10", "PAGINA 4/5", "pag12", " página 3", "xpágina 3", "Pág.  7 de 9 ",
	"Termos de Assinatura e Registro Eletrônico", "TERMOS DE ASSINATURA E REGISTRO ELETRONICO",
	"termos  de aſſinatura e regiſtro eLeTrônico", "Termos de Assinatura e Registro",
	"Enviado: 01/01/2026", "enviado: x", "Visualizado: hoje", "ASSINADO: sim", "Aceito: sim",
	"Nível de segurança: alto", "nivel  de  seguranÇa", "Enviado sem dois-pontos", "  Enviado: x",
	"Considerando os fatos narrados na petição inicial, o pedido é procedente.",
}

// A regra antiga de valorEdgeRegex, guardada aqui só para conferir a equivalência.
var valorEdgeRegexAntiga = regexp.MustCompile(`R\$|US\$|\d+,\d{2}|%`)

func TestTemValorEdge_EquivaleARegex(t *testing.T) {
	for _, l := range linhasPrefiltro {
		if got, want := temValorEdge(l), valorEdgeRegexAntiga.MatchString(l); got != want {
			t.Errorf("temValorEdge(%q) = %v, regex = %v", l, got, want)
		}
	}
	if temValorEdge("$ 100 pagos") {
		t.Error(`"$ 100 pagos" não tem valor para a regra antiga`)
	}
}

// linhaLetraSoltaAntiga é linhaLetraSolta antes do pré-filtro.
func linhaLetraSoltaAntiga(l string) bool {
	if romanoRegex.MatchString(l) || alineaRegex.MatchString(l) || siglasUF[l] || strings.ContainsAny(l, "ºª") {
		return false
	}
	switch l {
	case "à", "À", "é", "É":
		return false
	}
	n := 0
	for _, r := range l {
		if !unicode.IsLetter(r) {
			return false
		}
		n++
	}
	return n >= 1 && n <= 2
}

func TestLinhaLetraSolta_PrefiltroEquivalente(t *testing.T) {
	for _, l := range linhasPrefiltro {
		if got, want := linhaLetraSolta(l), linhaLetraSoltaAntiga(l); got != want {
			t.Errorf("linhaLetraSolta(%q) = %v, antes = %v", l, got, want)
		}
	}
	if !linhaLetraSolta("ﬁﬁ") {
		t.Error(`"ﬁﬁ" (duas runas, 6 bytes) continua letra solta`)
	}
}

func TestPrefiltrosRegex_Equivalentes(t *testing.T) {
	for _, l := range linhasPrefiltro {
		if got, want := marcadorPagina(l), pageMarkerRegex.MatchString(l); got != want {
			t.Errorf("marcadorPagina(%q) = %v, regex = %v", l, got, want)
		}
		if got, want := tituloCertificado(l), certificadoTituloRegex.MatchString(l); got != want {
			t.Errorf("tituloCertificado(%q) = %v, regex = %v", l, got, want)
		}
		m := certificadoCampoRegex.FindStringSubmatch(l)
		got, ok := campoCertificado(l)
		if ok != (m != nil) || (ok && got != m[1]) {
			t.Errorf("campoCertificado(%q) = %q,%v, regex = %v", l, got, ok, m)
		}
	}
	if !tituloCertificado("termos  de aſſinatura e regiſtro eLeTrônico") {
		t.Error("ſ casa com s sob (?i): o pré-filtro não pode depender de s")
	}
}

// As mesmas equivalências sobre todas as linhas do processo sintético.
func TestPrefiltros_EquivalentesNoSintetico(t *testing.T) {
	var linhas []string
	for _, ev := range processoSintetico(4, 10) {
		docs, _ := ev.(map[string]any)["documentos"].(map[string]any)
		for _, d := range docs {
			pags, _ := d.(map[string]any)["paginas"].(map[string]any)
			for _, p := range pags {
				if s, ok := p.(map[string]any)["content"].(string); ok {
					linhas = append(linhas, linhasBasicas(s)...)
				}
			}
		}
	}
	if len(linhas) == 0 {
		t.Fatal("nenhuma linha no processo sintético")
	}
	for _, l := range linhas {
		if marcadorPagina(l) != pageMarkerRegex.MatchString(l) || temValorEdge(l) != valorEdgeRegexAntiga.MatchString(l) ||
			linhaLetraSolta(l) != linhaLetraSoltaAntiga(l) || tituloCertificado(l) != certificadoTituloRegex.MatchString(l) {
			t.Errorf("pré-filtro diverge da regra antiga em %q", l)
		}
	}
}
