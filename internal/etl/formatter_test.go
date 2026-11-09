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
	if !strings.Contains(aggCleaned, "Documento assinado") {
		t.Error("cleanPage('aggressive') não tira mais linha solta de assinatura (H6)")
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
		// A regra antiga de prefixos saiu (H6): sem chancela reconhecida, fica.
		"Assinatura eletrônica realizada por",
		"Documento assinado eletronicamente",
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
	if !strings.Contains(output, "<pag n=\"2\">") || !strings.Contains(output, "</pag>") {
		t.Error("missing <pag n=\"2\"> tag")
	}
	// o evento 2 tem uma página só: sem <pag>
	if n := strings.Count(output, "<pag "); n != 2 {
		t.Errorf("esperava 2 tags <pag> (só o documento de 2 páginas), obteve %d:\n%s", n, output)
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

	for _, want := range []string{"\n- Evento 1 - INIC1: 3 páginas\n", "\n- Evento 1 - PROC2: 1 página\n"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected summary line %q in output:\n%s", want, output)
		}
	}
}

// pg monta uma página longa (mais de 2×edgeLineCount linhas): topo, 11 linhas
// de miolo únicas (tag) e rodapé; topo e rodapé vazios são omitidos.
func pg(topo, tag, rodape string) string {
	var ls []string
	if topo != "" {
		ls = append(ls, topo)
	}
	for i := 1; i <= 11; i++ {
		ls = append(ls, tag+" miolo "+strconv.Itoa(i))
	}
	if rodape != "" {
		ls = append(ls, rodape)
	}
	return strings.Join(ls, "\n")
}

func TestRepeatedEdgeLines(t *testing.T) {
	cases := []struct {
		name  string
		pages []string
		want  []string
	}{
		{
			name:  "uma página não marca nada",
			pages: []string{pg("CABEÇALHO", "a", "")},
			want:  nil,
		},
		{
			name:  "duas páginas marcam o que se repete nas duas",
			pages: []string{pg("CABEÇALHO", "a", ""), pg("CABEÇALHO", "b", "")},
			want:  []string{"CABEÇALHO"},
		},
		{
			name:  "páginas curtas ficam fora da contagem",
			pages: []string{"CABEÇALHO\ncorpo a\nNestes termos", "CABEÇALHO\ncorpo b\nNestes termos"},
			want:  nil,
		},
		{
			name: "cabeçalho e rodapé com número variável",
			pages: []string{
				pg("PODER JUDICIÁRIO", "um", "Processo 5001234-10.2024 - fl. 1"),
				pg("PODER JUDICIÁRIO", "dois", "Processo 5001234-10.2024 - fl. 2"),
				pg("  PODER JUDICIÁRIO  ", "três", "Processo 5001234-10.2024 - fl. 3"),
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
				pg("TOPO", "x", ""), pg("TOPO", "y", ""), pg("outro", "z", ""), pg("mais", "w", ""), pg("fim", "v", ""),
			},
			want: nil,
		},
		{
			name: "exatamente 60% das páginas é marcada",
			pages: []string{
				pg("TOPO", "x", ""), pg("TOPO", "y", ""), pg("TOPO", "z", ""), pg("mais", "w", ""), pg("fim", "v", ""),
			},
			want: []string{"TOPO"},
		},
		{
			name: "linha repetida na mesma página conta uma vez",
			pages: []string{
				pg("TOPO\nTOPO", "x", ""), pg("TOPO\nTOPO", "y", ""), pg("a", "z", ""), pg("c", "w", ""),
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
			"content": pg("TRIBUNAL REGIONAL FEDERAL", "p"+strconv.Itoa(n), "Processo 5001234-10.2024 - fl. "+strconv.Itoa(n)) + "\n" + body,
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
	want := "\nTokens estimados: " + strconv.Itoa(tokens) + "\n"
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
		if tokens != EstimateTokens(output) || !strings.Contains(output, "\nTokens estimados: "+strconv.Itoa(tokens)+"\n") {
			t.Fatalf("n=%d: tokens = %d, estimativa = %d", n, tokens, EstimateTokens(output))
		}
	}
}

func TestFormato_DocUmaPaginaSemPag(t *testing.T) {
	niveis := []string{NivelNenhum, NivelBasico, NivelPadrao, NivelMaximo}
	for _, nivel := range niveis {
		// Duas páginas: <pag n="2"> aparece.
		saida, _ := GenerateConsolidatedOutput(docPadrao("Primeira página sintética.", "Segunda página sintética."), nivel)
		for i, linha := range strings.Split(saida, "\n") {
			if strings.HasPrefix(linha, " ") {
				t.Errorf("%s: linha %d começa com espaço: %q", nivel, i+1, linha)
			}
		}
		if !strings.Contains(saida, "<pag n=\"2\">\nSegunda página sintética.\n</pag>\n") {
			t.Errorf("%s: faltou <pag n=\"2\">:\n%s", nivel, saida)
		}
		if strings.Contains(saida, "<pag num") {
			t.Errorf("%s: <pag num> antigo:\n%s", nivel, saida)
		}

		// Uma página: conteúdo direto no documento.
		saida, _ = GenerateConsolidatedOutput(docPadrao("Única página sintética."), nivel)
		if strings.Contains(saida, "<pag") {
			t.Errorf("%s: documento de uma página sem <pag>:\n%s", nivel, saida)
		}
		if !strings.Contains(saida, "nome=\"Evento 1 - INIC1.pdf\">\nÚnica página sintética.\n</documento>\n") {
			t.Errorf("%s: conteúdo devia ficar direto no documento:\n%s", nivel, saida)
		}
		if !strings.Contains(saida, "\nTokens estimados: ") || !strings.HasPrefix(saida, "<contexto>\nPROCESSO") {
			t.Errorf("%s: <contexto> devia vir sem recuo:\n%s", nivel, saida)
		}
	}
}

func TestFormato_PaginaVaziaNaoEmitePag(t *testing.T) {
	for _, nivel := range []string{NivelNenhum, NivelBasico, NivelPadrao, NivelMaximo} {
		// Duas páginas na origem, uma vazia: a que sobra mantém o <pag n>.
		saida, _ := GenerateConsolidatedOutput(docPadrao("Texto sintético.", "  \n \n"), nivel)
		if strings.Contains(saida, "<pag n=\"2\">") || !strings.Contains(saida, "<pag n=\"1\">\nTexto sintético.\n</pag>") {
			t.Errorf("%s: só a pág. 1 emite <pag>:\n%s", nivel, saida)
		}
		// 3 páginas na origem, só a 2 com conteúdo: o 2 não se perde.
		saida, _ = GenerateConsolidatedOutput(docPadrao("", "Texto sintético.", ""), nivel)
		if !strings.Contains(saida, "<pag n=\"2\">\nTexto sintético.\n</pag>") || strings.Contains(saida, "<pag n=\"1\">") {
			t.Errorf("%s: pág. 2 devia sair com n=2:\n%s", nivel, saida)
		}
		// Todas vazias: sem corpo.
		saida, _ = GenerateConsolidatedOutput(docPadrao("", "\n"), nivel)
		if strings.Contains(saida, "<pag") || !strings.Contains(saida, "INIC1.pdf\">\n</documento>\n") {
			t.Errorf("%s: documento sem conteúdo sai sem corpo:\n%s", nivel, saida)
		}
	}
	// A vazia no meio mantém o n original das outras.
	saida, _ := GenerateConsolidatedOutput(docPadrao("A sintética.", "", "C sintética."), NivelBasico)
	if !strings.Contains(saida, "<pag n=\"1\">") || strings.Contains(saida, "<pag n=\"2\">") || !strings.Contains(saida, "<pag n=\"3\">") {
		t.Errorf("páginas 1 e 3 devem manter o n:\n%s", saida)
	}
}

func TestFormato_MarcadoresDeDuplicataSobrevivem(t *testing.T) {
	igual := textoLongo("igual", 250)
	saida, _ := GenerateConsolidatedOutput(eventosDocs(
		[]string{igual, textoLongo("outra", 250)}, []string{textoLongo("terceira", 250), igual}), NivelPadrao)
	if !strings.Contains(saida, "<pag n=\"2\">\n[conteúdo idêntico a: evento 1, DOC1, pág. 1]\n</pag>") {
		t.Errorf("página duplicada mantém <pag> com a referência:\n%s", saida)
	}
	saida, _ = GenerateConsolidatedOutput(eventosDocs([]string{igual}, []string{igual}), NivelPadrao)
	if !strings.Contains(saida, ">\n[documento idêntico a: evento 1, DOC1]\n</documento>") {
		t.Errorf("documento duplicado: referência direto no documento:\n%s", saida)
	}
	if strings.Contains(saida, "<pag") {
		t.Errorf("documento duplicado sem <pag>:\n%s", saida)
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

// pagina devolve o trecho de saída da página n (entre <pag n="n"> e </pag>).
func pagina(t *testing.T, saida string, n int) string {
	t.Helper()
	_, resto, ok := strings.Cut(saida, "<pag n=\""+strconv.Itoa(n)+"\">")
	if !ok {
		t.Fatalf("página %d ausente:\n%s", n, saida)
	}
	corpo, _, _ := strings.Cut(resto, "</pag>")
	return corpo
}

// textoLongo devolve texto sintético de pelo menos n runas, diferente por semente.
func textoLongo(semente string, n int) string {
	var sb strings.Builder
	for sb.Len() < n {
		sb.WriteString("Texto sintético " + semente + " do caso FULANO DE TAL. ")
	}
	return sb.String()
}

// eventosDocs monta eventos com um documento por evento, cada um com as páginas dadas.
func eventosDocs(docs ...[]string) map[string]any {
	eventos := map[string]any{}
	for i, paginas := range docs {
		pags := map[string]any{}
		for j, c := range paginas {
			pags[strconv.Itoa(j+1)] = map[string]any{"index": j + 1, "ocr": false, "content": c}
		}
		eventos[strconv.Itoa(i+1)] = map[string]any{
			"index": i + 1,
			"documentos": map[string]any{
				"d": map[string]any{"index": 1, "nome": "Evento " + strconv.Itoa(i+1) + " - DOC" + strconv.Itoa(i+1) + ".pdf", "paginas": pags},
			},
		}
	}
	return eventos
}

func TestDuplicadas_PaginaViraReferencia(t *testing.T) {
	igual := textoLongo("igual", 250)
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		saida, _ := GenerateConsolidatedOutput(eventosDocs(
			[]string{igual, textoLongo("outra", 250)},
			[]string{textoLongo("terceira", 250), igual},
		), nivel)
		if !strings.Contains(saida, "[conteúdo idêntico a: evento 1, DOC1, pág. 1]") {
			t.Errorf("%s: faltou a referência de página:\n%s", nivel, saida)
		}
		if got, want := strings.Count(saida, "Texto sintético igual"), strings.Count(igual, "Texto sintético igual"); got != want {
			t.Errorf("%s: o texto repetido devia aparecer uma vez (%d, esperado %d):\n%s", nivel, got, want, saida)
		}
		if strings.Contains(saida, "documento idêntico") {
			t.Errorf("%s: documento só em parte duplicado não vira referência:\n%s", nivel, saida)
		}
	}
}

func TestDuplicadas_DocumentoInteiro(t *testing.T) {
	a, b := textoLongo("a", 250), textoLongo("b", 250)
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		saida, _ := GenerateConsolidatedOutput(eventosDocs([]string{a, b}, []string{a, b}), nivel)
		if !strings.Contains(saida, "[documento idêntico a: evento 1, DOC1]") {
			t.Errorf("%s: faltou a referência de documento:\n%s", nivel, saida)
		}
		if strings.Contains(saida, "conteúdo idêntico") {
			t.Errorf("%s: documento inteiro não deixa referência por página:\n%s", nivel, saida)
		}
		if strings.Count(saida, "Texto sintético a ") != strings.Count(a, "Texto sintético a ") ||
			strings.Count(saida, "Texto sintético b ") != strings.Count(b, "Texto sintético b ") {
			t.Errorf("%s: o texto devia aparecer uma vez:\n%s", nivel, saida)
		}
	}
}

func TestDuplicadas_DocumentoDeOrigensDiferentesNaoColapsa(t *testing.T) {
	a, b := textoLongo("a", 250), textoLongo("b", 250)
	saida, _ := GenerateConsolidatedOutput(eventosDocs([]string{a}, []string{b}, []string{a, b}), NivelPadrao)
	if strings.Contains(saida, "documento idêntico") {
		t.Errorf("páginas de origens diferentes não colapsam o documento:\n%s", saida)
	}
	if !strings.Contains(saida, "[conteúdo idêntico a: evento 1, DOC1, pág. 1]") ||
		!strings.Contains(saida, "[conteúdo idêntico a: evento 2, DOC2, pág. 1]") {
		t.Errorf("cada página devia referenciar a sua origem:\n%s", saida)
	}
}

// Documento só colapsa se for idêntico página a página: mesma quantidade e mesma ordem.
func TestDuplicadas_DocumentoNaoIdenticoNaoColapsa(t *testing.T) {
	a, b := textoLongo("a", 250), textoLongo("b", 250)
	casos := map[string][]string{
		"fora de ordem":   {b, a},
		"pagina repetida": {a, a},
		"pagina a menos":  {a},
	}
	for nome, segundo := range casos {
		saida, _ := GenerateConsolidatedOutput(eventosDocs([]string{a, b}, segundo), NivelPadrao)
		if strings.Contains(saida, "documento idêntico") {
			t.Errorf("%s: documento não idêntico colapsou:\n%s", nome, saida)
		}
		if !strings.Contains(saida, "[conteúdo idêntico a: evento 1, DOC1, pág. ") {
			t.Errorf("%s: as páginas deviam virar referência por página:\n%s", nome, saida)
		}
	}
}

// Dois documentos de mesmo nome no mesmo evento são origens distintas.
func TestDuplicadas_MesmoNomeNoMesmoEventoNaoSeConfunde(t *testing.T) {
	a, b := textoLongo("a", 250), textoLongo("b", 250)
	doc := func(idx int, paginas ...string) map[string]any {
		pags := map[string]any{}
		for j, c := range paginas {
			pags[strconv.Itoa(j+1)] = map[string]any{"index": j + 1, "ocr": false, "content": c}
		}
		return map[string]any{"index": idx, "nome": "Evento 1 - ANEXO.pdf", "paginas": pags}
	}
	eventos := map[string]any{"1": map[string]any{"index": 1, "documentos": map[string]any{
		"x": doc(1, a), "y": doc(2, b), "z": doc(3, a, b),
	}}}
	saida, _ := GenerateConsolidatedOutput(eventos, NivelPadrao)
	if strings.Contains(saida, "documento idêntico") {
		t.Errorf("origens em documentos diferentes (mesmo nome) não colapsam:\n%s", saida)
	}
}

// A página duplicada não imprime o timbre, mas ele já saiu na origem: a página
// seguinte, não duplicada, continua sem ele.
func TestDuplicadas_PaginaDuplicadaConsomeOTimbre(t *testing.T) {
	timbre := "ESCRITÓRIO MODELO ADVOGADOS\nOAB/SP 000.000"
	dupl := pg(timbre, textoLongo("repetida", 250), "")
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		pg(timbre, "um", ""), dupl, dupl, pg(timbre, "quatro", "")), NivelPadrao)
	if p := pagina(t, saida, 4); strings.Contains(p, "ESCRITÓRIO") {
		t.Errorf("o timbre já apareceu na pág. 1: %q", p)
	}
}

func TestDuplicadas_UmDigitoDiferenteNaoColapsa(t *testing.T) {
	base := textoLongo("valor", 250)
	saida, _ := GenerateConsolidatedOutput(eventosDocs(
		[]string{base + "Valor R$ 1.234,56"}, []string{base + "Valor R$ 1.234,57"}), NivelPadrao)
	if strings.Contains(saida, "idêntico") {
		t.Errorf("um dígito diferente não é duplicata:\n%s", saida)
	}
	if !strings.Contains(saida, "1.234,56") || !strings.Contains(saida, "1.234,57") {
		t.Errorf("os dois valores deviam ficar:\n%s", saida)
	}
}

func TestDuplicadas_PaginaCurtaNaoColapsa(t *testing.T) {
	curta := textoLongo("curta", 150)[:150]
	saida, _ := GenerateConsolidatedOutput(eventosDocs([]string{curta}, []string{curta}), NivelPadrao)
	if strings.Contains(saida, "idêntico") {
		t.Errorf("página de 150 runas não colapsa:\n%s", saida)
	}
}

func TestDuplicadas_BasicaNaoColapsa(t *testing.T) {
	igual := textoLongo("igual", 250)
	for _, nivel := range []string{NivelNenhum, NivelBasico} {
		saida, _ := GenerateConsolidatedOutput(eventosDocs([]string{igual}, []string{igual}), nivel)
		if strings.Contains(saida, "idêntico") || strings.Count(saida, "Texto sintético igual") < 2*strings.Count(igual, "Texto sintético igual") {
			t.Errorf("%s: não devia colapsar:\n%s", nivel, saida)
		}
	}
}

func TestPadrao_TimbreFicaSoNaPrimeiraPagina(t *testing.T) {
	timbre := "ESCRITÓRIO MODELO ADVOGADOS\nOAB/SP 000.000"
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		pg(timbre, "um", ""), pg(timbre, "dois", ""), pg(timbre, "tres", "")), NivelPadrao)
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
		pg("", "um", rodape), pg("", "dois", rodape), pg("", "tres", rodape)), NivelPadrao)
	if got := strings.Count(saida, "r5 rodape"); got != 1 {
		t.Errorf("rodapé de 5 linhas devia ficar só na pág. 1, apareceu %d vezes:\n%s", got, saida)
	}
	if !strings.Contains(pagina(t, saida, 1), "r5 rodape") {
		t.Errorf("rodapé devia ficar na pág. 1")
	}
}

func TestPadrao_DocumentoDe2Paginas(t *testing.T) {
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		pg("CABEÇALHO FICTÍCIO", "um", ""), pg("CABEÇALHO FICTÍCIO", "dois", "")), NivelPadrao)
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

func TestPadrao_PaginaCurtaNaoPerdeLinhas(t *testing.T) {
	fim := "Nestes termos, pede deferimento.\nFULANO DE TAL OAB/SP 000.000"
	curta := func(tag string) string {
		return "a " + tag + "\nb " + tag + "\nc " + tag + "\nd " + tag + "\n" + fim
	}
	saida, _ := GenerateConsolidatedOutput(docPadrao(curta("um"), curta("dois")), NivelPadrao)
	for n := 1; n <= 2; n++ {
		p := pagina(t, saida, n)
		if !strings.Contains(p, "Nestes termos, pede deferimento.") || !strings.Contains(p, "FULANO DE TAL OAB/SP 000.000") {
			t.Errorf("pág. %d curta perdeu o fecho: %q", n, p)
		}
	}
}

func TestPadrao_ValoresNaBordaNaoSaoNormalizados(t *testing.T) {
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		pg("", "um", "Total: R$ 100,00\nfl. 1"),
		pg("", "dois", "Total: R$ 200,00\nfl. 2"),
		pg("", "tres", "Total: R$ 300,00\nfl. 3")), NivelPadrao)
	for _, v := range []string{"R$ 100,00", "R$ 200,00", "R$ 300,00"} {
		if got := strings.Count(saida, v); got != 1 {
			t.Errorf("%s devia aparecer 1 vez, apareceu %d", v, got)
		}
	}
	if got := strings.Count(saida, "fl. "); got != 1 {
		t.Errorf("fl. N devia ficar só na pág. 1, apareceu %d vezes", got)
	}
	if !strings.Contains(pagina(t, saida, 1), "fl. 1") {
		t.Errorf("fl. 1 devia ficar na pág. 1")
	}
}

func TestChaveBorda_ValorMonetarioEPercentualUsamChaveExata(t *testing.T) {
	for _, l := range []string{"Total: R$ 100,00", "US$ 5", "saldo 12,50", "taxa 10%"} {
		if normalizeEdgeLine(l) != l {
			t.Errorf("%q devia usar a chave exata, deu %q", l, normalizeEdgeLine(l))
		}
	}
	if got := normalizeEdgeLine("fl. 12"); got != "fl. ##" {
		t.Errorf("fl. 12 devia trocar dígitos, deu %q", got)
	}
}

func TestPadraoEMaxima_MantemAssinaturaSolta(t *testing.T) {
	// Sem chancela reconhecida (H6), a linha e o nome do assinante ficam nos dois níveis.
	ass := "Documento assinado eletronicamente por FULANO DE TAL"
	pag := "inicio\n" + ass + "\nfim"
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		if got := cleanPage(pag, nivel, &ctxDocumento{jaVistas: map[string]bool{}}); !strings.Contains(got, ass) {
			t.Errorf("%s devia manter a assinatura: %q", nivel, got)
		}
	}
}

func TestMaxima_JuntaNumaLinhaComTimbreSoNaPagina1(t *testing.T) {
	saida, _ := GenerateConsolidatedOutput(docPadrao(
		pg("TIMBRE FICTÍCIO", "um", ""), pg("TIMBRE FICTÍCIO", "dois", ""), pg("TIMBRE FICTÍCIO", "tres", "")), NivelMaximo)
	if got := strings.Count(saida, "TIMBRE FICTÍCIO"); got != 1 {
		t.Errorf("timbre devia aparecer 1 vez, apareceu %d", got)
	}
	for n := 1; n <= 3; n++ {
		if p := strings.TrimSpace(pagina(t, saida, n)); strings.Contains(p, "\n") {
			t.Errorf("pág. %d devia ficar numa linha só: %q", n, p)
		}
	}
	if !strings.Contains(pagina(t, saida, 1), "TIMBRE FICTÍCIO") {
		t.Errorf("timbre devia ficar na pág. 1")
	}
}
