package etl

import (
	"strconv"
	"strings"
	"testing"
)

func TestExtractTextFromHTML(t *testing.T) {
	htmlData := `<html>
<head>
<title>Ignore Title</title>
<style>body { color: red; }</style>
</head>
<body>
<h1>Visible Heading</h1>
<script>console.log("ignore script");</script>
<p>Visible paragraph text.</p>
</body>
</html>`

	r := strings.NewReader(htmlData)
	text, err := ExtractTextFromHTML(r)
	if err != nil {
		t.Fatalf("ExtractTextFromHTML failed: %v", err)
	}

	// Verify Visible Content is present
	if !strings.Contains(text, "Visible Heading") {
		t.Error("expected text to contain 'Visible Heading'")
	}
	if !strings.Contains(text, "Visible paragraph text.") {
		t.Error("expected text to contain 'Visible paragraph text.'")
	}

	// Verify Hidden Content (styles/scripts/head) is ignored
	if strings.Contains(text, "Ignore Title") {
		t.Error("unexpectedly extracted title from head")
	}
	if strings.Contains(text, "body { color: red; }") {
		t.Error("unexpectedly extracted style block content")
	}
	if strings.Contains(text, "console.log") {
		t.Error("unexpectedly extracted script block content")
	}
}

func TestCleanPage(t *testing.T) {
	rawText := `   
Line 1 with spaces   
Página 12 de 35
Documento assinado eletronicamente por Fulano
Line 2 with text
`

	// CleanLevel: none
	if cleaned := cleanPage(rawText, "none", nil); cleaned != rawText {
		t.Errorf("cleanPage('none') modified text; expected unchanged")
	}

	// CleanLevel: basic
	basicCleaned := cleanPage(rawText, "basic", nil)
	if !strings.Contains(basicCleaned, "Página 12 de 35") {
		t.Error("cleanPage('basic') should preserve page markers")
	}
	if strings.HasPrefix(basicCleaned, " ") || strings.HasSuffix(basicCleaned, " ") {
		t.Error("cleanPage('basic') should trim line whitespace")
	}

	// CleanLevel: aggressive
	aggCleaned := cleanPage(rawText, "aggressive", nil)
	if strings.Contains(aggCleaned, "Página 12 de 35") {
		t.Error("cleanPage('aggressive') should strip page markers")
	}
	if strings.Contains(aggCleaned, "Documento assinado") {
		t.Error("cleanPage('aggressive') should strip digital signatures")
	}
	if strings.Contains(aggCleaned, "\n") {
		t.Error("cleanPage('aggressive') should merge lines into space-separated string")
	}
}

func TestCleanPage_AggressiveKeepsLegitimateContent(t *testing.T) {
	raw := `O autor requer a juntada, conforme a página inicial dos autos.
Pág. 3/10
página 4
Página 12 de 35
Alega a parte que a assinatura eletrônica do contrato é nula.
Assinatura eletrônica realizada por Fulano de Tal
Documento assinado eletronicamente por Beltrano
A paginação do processo foi suspensa.`

	got := cleanPage(raw, "aggressive", nil)

	mustKeep := []string{
		"conforme a página inicial dos autos",
		"a assinatura eletrônica do contrato é nula",
		"A paginação do processo foi suspensa.",
	}
	for _, s := range mustKeep {
		if !strings.Contains(got, s) {
			t.Errorf("aggressive cleaning dropped legitimate content %q; got %q", s, got)
		}
	}

	mustDrop := []string{
		"Pág. 3/10",
		"página 4",
		"Página 12 de 35",
		"Assinatura eletrônica realizada por",
		"Documento assinado eletronicamente",
	}
	for _, s := range mustDrop {
		if strings.Contains(got, s) {
			t.Errorf("aggressive cleaning kept boilerplate %q; got %q", s, got)
		}
	}
}

func TestGenerateConsolidatedOutput_EscapesDocumentName(t *testing.T) {
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"x": map[string]any{
					"index": 1,
					"nome":  `A & B "x" <y>.pdf`,
					"paginas": map[string]any{
						"1": map[string]any{"index": 1, "ocr": false, "content": "texto"},
					},
				},
			},
		},
	}

	output, _ := GenerateConsolidatedOutput(eventos, "basic")

	want := `nome="A &amp; B &quot;x&quot; &lt;y&gt;.pdf"`
	if !strings.Contains(output, want) {
		t.Errorf("expected escaped attribute %s in output:\n%s", want, output)
	}
}

func TestGenerateConsolidatedOutput(t *testing.T) {
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"zip_path_1": map[string]any{
					"index": 1,
					"nome":  "Evento 1 - INIC1.pdf",
					"paginas": map[string]any{
						"1": map[string]any{
							"index":   1,
							"ocr":     false,
							"content": "Initial petition text page 1",
						},
						"2": map[string]any{
							"index":   2,
							"ocr":     false,
							"content": "Initial petition text page 2",
						},
					},
				},
			},
		},
		"2": map[string]any{
			"index": 2,
			"documentos": map[string]any{
				"zip_path_2": map[string]any{
					"index": 1,
					"nome":  "Evento 2 - DESPACHO.pdf",
					"paginas": map[string]any{
						"1": map[string]any{
							"index":   1,
							"ocr":     false,
							"content": "Judge order text page 1",
						},
					},
				},
			},
		},
	}

	output, _ := GenerateConsolidatedOutput(eventos, "basic")

	// Verify XML structure tags exist
	if !strings.Contains(output, "<contexto>") || !strings.Contains(output, "</contexto>") {
		t.Error("missing <contexto> container tags")
	}
	if !strings.Contains(output, "<evento num=\"1\">") || !strings.Contains(output, "</evento>") {
		t.Error("missing <evento num=\"1\"> container tags")
	}
	if !strings.Contains(output, "<documento index=\"1\" nome=\"Evento 1 - INIC1.pdf\">") || !strings.Contains(output, "</documento>") {
		t.Error("missing <documento> tag for Evento 1")
	}
	if !strings.Contains(output, "<pag num=\"2\">") || !strings.Contains(output, "</pag>") {
		t.Error("missing <pag num=\"2\"> tag")
	}
	if !strings.Contains(output, "Initial petition text page 2") {
		t.Error("missing page content")
	}
}

// The summary follows PRD §6: "- Evento 1 - INIC1: 3 páginas" (no repeated
// event prefix, no extension, singular for one page).
func TestGenerateConsolidatedOutput_SummaryLineFormat(t *testing.T) {
	page := func(i int) map[string]any {
		return map[string]any{"index": i, "ocr": false, "content": "texto"}
	}
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"a": map[string]any{
					"index": 1, "nome": "Evento 1 - INIC1.pdf",
					"paginas": map[string]any{"1": page(1), "2": page(2), "3": page(3)},
				},
				"b": map[string]any{
					"index": 2, "nome": "Evento 1 - PROC2.PDF",
					"paginas": map[string]any{"1": page(1)},
				},
			},
		},
	}

	output, _ := GenerateConsolidatedOutput(eventos, "basic")

	for _, want := range []string{"    - Evento 1 - INIC1: 3 páginas\n", "    - Evento 1 - PROC2: 1 página\n"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected summary line %q in output:\n%s", want, output)
		}
	}
}

func TestRepeatedEdgeLines(t *testing.T) {
	cases := []struct {
		name  string
		pages []string
		want  []string
	}{
		{
			name:  "uma página não marca nada",
			pages: []string{"CABEÇALHO\ncorpo a"},
			want:  nil,
		},
		{
			name:  "duas páginas marcam o que se repete nas duas",
			pages: []string{"CABEÇALHO\ncorpo a", "CABEÇALHO\ncorpo b"},
			want:  []string{"CABEÇALHO"},
		},
		{
			name: "cabeçalho e rodapé com número variável",
			pages: []string{
				"PODER JUDICIÁRIO\ncorpo um\nmais texto\nmeio um\nfim do corpo\nProcesso 5001234-10.2024 - fl. 1",
				"PODER JUDICIÁRIO\ncorpo dois\noutro\nmeio dois\nfim\nProcesso 5001234-10.2024 - fl. 2",
				"  PODER JUDICIÁRIO  \ncorpo três\nlinha\nmeio três\nfinal\nProcesso 5001234-10.2024 - fl. 3",
			},
			want: []string{"PODER JUDICIÁRIO", "Processo #######-##.#### - fl. #"},
		},
		{
			name: "linha repetida só no meio não conta",
			pages: []string{
				"a1\na2\na3\na4\na5\nREPETIDA\na6\na7\na8\na9\na10",
				"b1\nb2\nb3\nb4\nb5\nREPETIDA\nb6\nb7\nb8\nb9\nb10",
				"c1\nc2\nc3\nc4\nc5\nREPETIDA\nc6\nc7\nc8\nc9\nc10",
			},
			want: nil,
		},
		{
			name: "abaixo de 60% das páginas não é marcada",
			pages: []string{
				"TOPO\nx", "TOPO\ny", "outro\nz", "mais\nw", "fim\nv",
			},
			want: nil,
		},
		{
			name: "exatamente 60% das páginas é marcada",
			pages: []string{
				"TOPO\nx", "TOPO\ny", "TOPO\nz", "mais\nw", "fim\nv",
			},
			want: []string{"TOPO"},
		},
		{
			name: "linha repetida na mesma página conta uma vez",
			pages: []string{
				"TOPO\nTOPO\nx", "TOPO\nTOPO\ny", "a\nb", "c\nd",
			},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := repeatedEdgeLines(tc.pages)
			if len(got) != len(tc.want) {
				t.Fatalf("esperava %d linhas, obteve %d: %v", len(tc.want), len(got), got)
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("esperava %q marcada; obteve %v", w, got)
				}
			}
		})
	}
}

func TestGenerateConsolidatedOutput_AggressiveDropsRepeatedHeadersFooters(t *testing.T) {
	paginas := map[string]any{}
	bodies := []string{"Primeira página do corpo.", "Segunda página do corpo.", "Terceira página do corpo."}
	for i, body := range bodies {
		n := i + 1
		paginas[strconv.Itoa(n)] = map[string]any{
			"index": n, "ocr": false,
			"content": "TRIBUNAL REGIONAL FEDERAL\n" + body + "\nProcesso 5001234-10.2024 - fl. " + strconv.Itoa(n),
		}
	}
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"d": map[string]any{"index": 1, "nome": "Evento 1 - INIC1.pdf", "paginas": paginas},
			},
		},
	}

	aggressive, _ := GenerateConsolidatedOutput(eventos, "aggressive")
	for _, body := range bodies {
		if !strings.Contains(aggressive, body) {
			t.Errorf("corpo %q sumiu da saída agressiva:\n%s", body, aggressive)
		}
	}
	if got := strings.Count(aggressive, "TRIBUNAL REGIONAL FEDERAL"); got != 1 {
		t.Errorf("cabeçalho repetido devia ficar só na primeira ocorrência, ficou %d vezes:\n%s", got, aggressive)
	}
	if got := strings.Count(aggressive, "Processo 5001234"); got != 1 {
		t.Errorf("rodapé repetido devia ficar só na primeira ocorrência, ficou %d vezes:\n%s", got, aggressive)
	}

	basic, _ := GenerateConsolidatedOutput(eventos, "basic")
	if strings.Count(basic, "TRIBUNAL REGIONAL FEDERAL") != 3 {
		t.Errorf("modo básico não deve remover cabeçalhos repetidos:\n%s", basic)
	}
}

func TestEstimateTokens(t *testing.T) {
	casos := []struct {
		texto string
		want  int
	}{
		{"", 0},
		{"abc", 0},
		{"abcd", 1},
		{"ação", 1}, // conta runas, não bytes
		{strings.Repeat("x", 4001), 1000},
	}
	for _, c := range casos {
		if got := EstimateTokens(c.texto); got != c.want {
			t.Errorf("EstimateTokens(%q) = %d, esperava %d", c.texto, got, c.want)
		}
	}
}

func TestGenerateConsolidatedOutput_TokensEstimadosNoContexto(t *testing.T) {
	paginas := map[string]any{}
	for i := 1; i <= 40; i++ {
		paginas[strconv.Itoa(i)] = map[string]any{"index": i, "ocr": false, "content": strings.Repeat("palavra ", 50)}
	}
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"x": map[string]any{"index": 1, "nome": "Evento 1 - INIC1.pdf", "paginas": paginas},
			},
		},
	}

	output, tokens := GenerateConsolidatedOutput(eventos, "basic")

	contexto, _, ok := strings.Cut(output, "</contexto>")
	if !ok {
		t.Fatalf("saída sem </contexto>:\n%s", output)
	}
	if tokens != EstimateTokens(output) {
		t.Errorf("tokens devolvidos = %d, estimativa da saída inteira = %d", tokens, EstimateTokens(output))
	}
	want := "    Tokens estimados: " + strconv.Itoa(tokens) + "\n"
	if !strings.Contains(contexto, want) {
		t.Errorf("<contexto> deveria ter %q (o mesmo número devolvido):\n%s", want, contexto)
	}
}

// O ponto fixo da linha de tokens converge em qualquer tamanho de saída,
// inclusive quando a própria linha faz o número ganhar um dígito.
func TestGenerateConsolidatedOutput_TokensBatemEmTodoTamanho(t *testing.T) {
	for n := range 1200 {
		eventos := map[string]any{
			"1": map[string]any{
				"index": 1,
				"documentos": map[string]any{
					"x": map[string]any{"index": 1, "nome": "Evento 1 - INIC1.pdf", "paginas": map[string]any{
						"1": map[string]any{"index": 1, "ocr": false, "content": strings.Repeat("a", 3400+n)},
					}},
				},
			},
		}
		output, tokens := GenerateConsolidatedOutput(eventos, "none")
		if tokens != EstimateTokens(output) || !strings.Contains(output, "Tokens estimados: "+strconv.Itoa(tokens)+"\n") {
			t.Fatalf("n=%d: tokens = %d, estimativa = %d", n, tokens, EstimateTokens(output))
		}
	}
}

// docPadrao monta um documento de uma só peça com as páginas dadas.
func docPadrao(paginas ...string) map[string]any {
	pags := map[string]any{}
	for i, c := range paginas {
		pags[strconv.Itoa(i+1)] = map[string]any{"index": i + 1, "ocr": false, "content": c}
	}
	return map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"d": map[string]any{"index": 1, "nome": "Evento 1 - INIC1.pdf", "paginas": pags},
			},
		},
	}
}

// pagina devolve o trecho de saída da página n (entre <pag num="n"> e </pag>).
func pagina(t *testing.T, saida string, n int) string {
	t.Helper()
	_, resto, ok := strings.Cut(saida, "<pag num=\""+strconv.Itoa(n)+"\">")
	if !ok {
		t.Fatalf("página %d ausente:\n%s", n, saida)
	}
	corpo, _, _ := strings.Cut(resto, "</pag>")
	return corpo
}

func TestPadrao_TimbreFicaSoNaPrimeiraPagina(t *testing.T) {
	timbre := "ESCRITÓRIO MODELO ADVOGADOS\nOAB/SP 000.000\n"
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		timbre+"corpo um", timbre+"corpo dois", timbre+"corpo tres"), NivelPadrao)
	p1 := pagina(t, saida, 1)
	if !strings.Contains(p1, "ESCRITÓRIO MODELO ADVOGADOS") || !strings.Contains(p1, "OAB/SP 000.000") {
		t.Errorf("pág. 1 devia manter o timbre: %q", p1)
	}
	for _, n := range []int{2, 3} {
		p := pagina(t, saida, n)
		if strings.Contains(p, "ESCRITÓRIO") || strings.Contains(p, "OAB/SP") {
			t.Errorf("pág. %d devia ficar sem timbre: %q", n, p)
		}
	}
}

func TestPadrao_BordaDe5Linhas(t *testing.T) {
	rodape := "r1 rodape\nr2 rodape\nr3 rodape\nr4 rodape\nr5 rodape"
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		"corpo um\n"+rodape, "corpo dois\n"+rodape, "corpo tres\n"+rodape), NivelPadrao)
	if got := strings.Count(saida, "r5 rodape"); got != 1 {
		t.Errorf("rodapé de 5 linhas devia ficar só na pág. 1, apareceu %d vezes:\n%s", got, saida)
	}
	if !strings.Contains(pagina(t, saida, 1), "r5 rodape") {
		t.Errorf("rodapé devia ficar na pág. 1")
	}
}

func TestPadrao_DocumentoDe2Paginas(t *testing.T) {
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		"CABEÇALHO FICTÍCIO\ncorpo um", "CABEÇALHO FICTÍCIO\ncorpo dois"), NivelPadrao)
	if !strings.Contains(pagina(t, saida, 1), "CABEÇALHO FICTÍCIO") {
		t.Errorf("pág. 1 devia manter o cabeçalho")
	}
	if strings.Contains(pagina(t, saida, 2), "CABEÇALHO FICTÍCIO") {
		t.Errorf("pág. 2 devia perder o cabeçalho repetido")
	}
}

func TestPadrao_FraseRepetidaNoMeioFica(t *testing.T) {
	mk := func(s string) string {
		return "a1 " + s + "\na2 " + s + "\na3 " + s + "\na4 " + s + "\na5 " + s +
			"\nFrase igual no meio.\nb1 " + s + "\nb2 " + s + "\nb3 " + s + "\nb4 " + s + "\nb5 " + s
	}
	saida, _ := GenerateConsolidatedOutput(docPadrao(mk("x"), mk("y"), mk("z")), NivelPadrao)
	if got := strings.Count(saida, "Frase igual no meio."); got != 3 {
		t.Errorf("frase do meio devia ficar nas 3 páginas, ficou em %d", got)
	}
}

func TestPadrao_LinhaSoSimbolos(t *testing.T) {
	got := cleanPage("texto antes\n| : \"\n§ 1º\n10%\nPágina 3 de 9\ntexto depois", NivelPadrao, &ctxDocumento{jaVistas: map[string]bool{}})
	for _, quer := range []string{"§ 1º", "10%", "texto antes", "texto depois"} {
		if !strings.Contains(got, quer) {
			t.Errorf("%q devia ficar: %q", quer, got)
		}
	}
	for _, nao := range []string{"| :", "Página 3"} {
		if strings.Contains(got, nao) {
			t.Errorf("%q devia sair: %q", nao, got)
		}
	}
}

func TestLinhaSoSimbolos(t *testing.T) {
	for _, l := range []string{`| : "`, "—", "|||"} {
		if !linhaSoSimbolos(l) {
			t.Errorf("%q devia ser só símbolos", l)
		}
	}
	for _, l := range []string{"§ 1º", "10%", "a", "5", "§", "º", "ª", "%"} {
		if linhaSoSimbolos(l) {
			t.Errorf("%q não devia ser só símbolos", l)
		}
	}
}
