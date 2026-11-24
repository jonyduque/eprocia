package etl

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var atualizarSaidaAntes = flag.Bool("atualizar", false, "regrava os goldens de testdata/saida_antes")

// dataConsolidacaoGolden troca a data da consolidação, que muda a cada execução.
var dataConsolidacaoGolden = regexp.MustCompile(`Data de Consolidação: [^\n]*`)

func semDataConsolidacao(s string) string {
	return dataConsolidacaoGolden.ReplaceAllString(s, "Data de Consolidação: DATA")
}

// mapasSaidaAntes são os mapas dos goldens: texto nativo com bordas repetidas,
// duplicatas de H7 (página e documento) e o processo sintético, que tem
// ocr_aviso, sem_ocr, omitido e não lido.
func mapasSaidaAntes() map[string]map[string]any {
	a, b, c := textoLongo("a", 250), textoLongo("b", 250), textoLongo("c", 250)
	return map[string]map[string]any{
		"nativo": eventosDocs(
			[]string{
				paginaLonga("PODER JUDICIÁRIO", "Rua Fictícia, 100 - fl. 1", "um"),
				paginaLonga("PODER JUDICIÁRIO", "Rua Fictícia, 100 - fl. 2", "dois"),
				paginaLonga("PODER JUDICIÁRIO", "Rua Fictícia, 100 - fl. 3", "tres"),
			},
			[]string{"  Página única do documento dois, de FULANO DE TAL.  \nPágina 1 de 1\n----------\n"},
		),
		"h7":    eventosDocs([]string{a, b}, []string{a, b}, []string{c, a}),
		"aviso": processoSintetico(4, 6),
	}
}

// A saída de GerarSaida sem páginas sem OCR é a mesma de antes da mudança,
// gravada pelo GenerateConsolidatedOutput antigo com -atualizar.
func TestGerarSaida_SemLinhaIgualAHoje(t *testing.T) {
	dir := filepath.Join("testdata", "saida_antes")
	for nome, mapa := range mapasSaidaAntes() {
		for _, nivel := range niveisDeLimpeza {
			arq := filepath.Join(dir, nome+"-"+nivel+".txt")
			if *atualizarSaidaAntes {
				saida, _ := GenerateConsolidatedOutput(mapa, nivel)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(arq, []byte(semDataConsolidacao(saida)), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			quer, err := os.ReadFile(arq)
			if err != nil {
				t.Fatalf("golden ausente (rode com -atualizar na versão antiga): %v", err)
			}
			if got := semDataConsolidacao(GerarSaida(mapa, nivel, 0).Texto); got != string(quer) {
				t.Errorf("%s/%s: saída mudou\n--- quer\n%s\n--- veio\n%s", nome, nivel, quer, got)
			}
		}
	}
}

func TestGerarSaida_LinhaSemOCR(t *testing.T) {
	const linha = "Páginas que precisavam de OCR e ficaram sem: 3\n"
	s := GerarSaida(processoSintetico(2, 3), NivelPadrao, 3)
	i := strings.Index(s.Texto, linha)
	antes := strings.Index(s.Texto, "Documentos não lidos: ")
	depois := strings.Index(s.Texto, "Data de Consolidação: ")
	if i < 0 || antes < 0 || depois < 0 || !(antes < i && i < depois) {
		t.Errorf("linha fora do lugar (lidos %d, linha %d, data %d):\n%s", antes, i, depois, s.Texto)
	}
	if strings.Count(s.Texto, "ficaram sem:") != 1 {
		t.Errorf("a linha devia aparecer uma vez:\n%s", s.Texto)
	}
	if s.Tokens != EstimateTokens(s.Texto) {
		t.Errorf("Tokens = %d, EstimateTokens = %d", s.Tokens, EstimateTokens(s.Texto))
	}
	if !strings.Contains(s.Texto, tokensLine(s.Tokens)) {
		t.Errorf("a linha de tokens não traz %d", s.Tokens)
	}
	if z := GerarSaida(processoSintetico(2, 3), NivelPadrao, 0); strings.Contains(z.Texto, "ficaram sem:") {
		t.Errorf("com N = 0 a linha não sai:\n%s", z.Texto)
	}
}

var (
	pagRegex       = regexp.MustCompile(`(?s)<pag n="\d+">\n(.*?)\n</pag>\n`)
	documentoRegex = regexp.MustCompile(`(?s)<documento index="\d+" nome="[^"]*">\n(.*?)</documento>\n`)
)

// bytesEmitidos é o oráculo de BytesDepois: lê o .txt, pega o texto de cada
// <pag> (ou o do documento de uma página), tira a linha de aviso de OCR e as
// referências de H7 e soma os bytes. Não usa a limpeza do formatter.
func bytesEmitidos(texto string) int64 {
	var total int64
	somar := func(p string) {
		if strings.HasPrefix(p, "[conteúdo idêntico a: ") || strings.HasPrefix(p, "[documento idêntico a: ") {
			return
		}
		if p == avisoOCRComReserva {
			return
		}
		p = strings.TrimPrefix(p, avisoOCRComReserva+"\n")
		total += int64(len(p))
	}
	for _, m := range documentoRegex.FindAllStringSubmatch(texto, -1) {
		corpo := m[1]
		if strings.HasPrefix(corpo, "[documento omitido: ") || strings.HasPrefix(corpo, "[documento não lido: ") {
			continue
		}
		if strings.HasPrefix(corpo, "<pag n=") {
			for _, p := range pagRegex.FindAllStringSubmatch(corpo, -1) {
				somar(p[1])
			}
			continue
		}
		if corpo != "" {
			somar(strings.TrimSuffix(corpo, "\n"))
		}
	}
	return total
}

func TestGerarSaida_DepoisIgualTextoEmitido(t *testing.T) {
	a, b, c := textoLongo("a", 250), textoLongo("b", 250), textoLongo("c", 250)
	mapas := map[string]func() map[string]any{
		"nativo": func() map[string]any { return mapasSaidaAntes()["nativo"] },
		"h7":     func() map[string]any { return eventosDocs([]string{a, b}, []string{a, b}, []string{c, a}, []string{b}) },
		// Sem o evento 4, o processo sintético não tem página sem OCR, cujo
		// marcador sai no texto mas fica fora das somas.
		"aviso": func() map[string]any { return processoSintetico(3, 6) },
	}
	for nome, mapa := range mapas {
		for _, nivel := range niveisDeLimpeza {
			s := GerarSaida(mapa(), nivel, 0)
			if got := bytesEmitidos(s.Texto); got != s.Limpeza.BytesDepois {
				t.Errorf("%s/%s: BytesDepois = %d, texto emitido = %d", nome, nivel, s.Limpeza.BytesDepois, got)
			}
			if s.Limpeza.Nivel != nivel {
				t.Errorf("%s/%s: Nivel = %q", nome, nivel, s.Limpeza.Nivel)
			}
			if s.Limpeza.BytesDepois == 0 {
				t.Errorf("%s/%s: BytesDepois zerado", nome, nivel)
			}
		}
	}
}

func TestGerarSaida_AntesMaiorOuIgual(t *testing.T) {
	for _, nivel := range niveisDeLimpeza {
		l := GerarSaida(processoSintetico(4, 6), nivel, 0).Limpeza
		if l.BytesAntes < l.BytesDepois || l.BytesAntes == 0 {
			t.Errorf("%s: BytesAntes %d < BytesDepois %d", nivel, l.BytesAntes, l.BytesDepois)
		}
		if l.TokensAntes < l.TokensDepois || l.TokensAntes == 0 {
			t.Errorf("%s: TokensAntes %d < TokensDepois %d", nivel, l.TokensAntes, l.TokensDepois)
		}
	}
	p1 := "Primeira página do caso FULANO DE TAL.\nSegunda linha com texto."
	p2 := "Outra página, ação e decisão.\nFim."
	l := GerarSaida(eventosDocs([]string{p1, p2}, []string{p1}), NivelNenhum, 0).Limpeza
	bytes := int64(2*len(p1) + len(p2))
	tokens := (2*utf8.RuneCountInString(p1) + utf8.RuneCountInString(p2)) / 4
	if l.BytesAntes != bytes || l.BytesDepois != bytes || l.TokensAntes != tokens || l.TokensDepois != tokens {
		t.Errorf("none: %+v, quer bytes %d e tokens %d nos dois lados", l, bytes, tokens)
	}
}

func TestGerarSaida_ForaDasSomas(t *testing.T) {
	a, b, c := textoLongo("a", 250), textoLongo("b", 250), textoLongo("c", 250)
	const normal = "Texto de uma página comum do caso FULANO DE TAL."
	montar := func(comSemOCR bool) map[string]any {
		ultimo := []string{normal}
		if comSemOCR {
			ultimo = append(ultimo, "[página digitalizada: o OCR está desativado]")
		}
		e := eventosDocs([]string{a, b}, []string{a, b}, []string{c, a}, ultimo)
		if comSemOCR {
			d := e["4"].(map[string]any)["documentos"].(map[string]any)["d"].(map[string]any)
			marcaSemOCR(d, "2")
		}
		return e
	}
	s := GerarSaida(montar(true), NivelPadrao, 0)
	l := s.Limpeza
	if quer := int64(len(a) + len(b) + len(c) + len(normal)); l.BytesAntes != quer {
		t.Errorf("BytesAntes = %d, quer %d (só a, b do doc 1, c e a página comum)", l.BytesAntes, quer)
	}
	if quer := int64(2*len(a) + len(b)); l.BytesDuplicatas != quer {
		t.Errorf("BytesDuplicatas = %d, quer %d", l.BytesDuplicatas, quer)
	}
	if s.DocsDuplicados != 1 || s.PaginasDuplicadas != 3 {
		t.Errorf("DocsDuplicados = %d, PaginasDuplicadas = %d, quer 1 e 3", s.DocsDuplicados, s.PaginasDuplicadas)
	}
	sem := GerarSaida(montar(false), NivelPadrao, 0).Limpeza
	if sem.BytesAntes != l.BytesAntes || sem.BytesDepois != l.BytesDepois || sem.TokensAntes != l.TokensAntes || sem.TokensDepois != l.TokensDepois {
		t.Errorf("a página sem OCR mudou as somas: com %+v, sem %+v", l, sem)
	}
	if l.BytesDepois > l.BytesAntes || l.BytesDepois == 0 {
		t.Errorf("BytesDepois = %d fora de (0, %d]", l.BytesDepois, l.BytesAntes)
	}
}
