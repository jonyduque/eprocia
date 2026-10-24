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
						"1": map[string]any{"index": 1, "ocr": false, "image": "", "content": "texto"},
					},
				},
			},
		},
	}

	output := GenerateConsolidatedOutput(eventos, "basic")

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
							"image":   "",
							"content": "Initial petition text page 1",
						},
						"2": map[string]any{
							"index":   2,
							"ocr":     false,
							"image":   "",
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
							"image":   "",
							"content": "Judge order text page 1",
						},
					},
				},
			},
		},
	}

	output := GenerateConsolidatedOutput(eventos, "basic")

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
		return map[string]any{"index": i, "ocr": false, "image": "", "content": "texto"}
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

	output := GenerateConsolidatedOutput(eventos, "basic")

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
			name:  "menos de 3 páginas não marca nada",
			pages: []string{"CABEÇALHO\ncorpo a", "CABEÇALHO\ncorpo b"},
			want:  nil,
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
				"a1\na2\na3\nREPETIDA\na4\na5\na6",
				"b1\nb2\nb3\nREPETIDA\nb4\nb5\nb6",
				"c1\nc2\nc3\nREPETIDA\nc4\nc5\nc6",
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
			"index": n, "ocr": false, "image": "",
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

	aggressive := GenerateConsolidatedOutput(eventos, "aggressive")
	for _, body := range bodies {
		if !strings.Contains(aggressive, body) {
			t.Errorf("corpo %q sumiu da saída agressiva:\n%s", body, aggressive)
		}
	}
	if strings.Contains(aggressive, "TRIBUNAL REGIONAL FEDERAL") {
		t.Errorf("cabeçalho repetido não foi removido no modo agressivo:\n%s", aggressive)
	}
	if strings.Contains(aggressive, "Processo 5001234") {
		t.Errorf("rodapé repetido não foi removido no modo agressivo:\n%s", aggressive)
	}

	basic := GenerateConsolidatedOutput(eventos, "basic")
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
		paginas[strconv.Itoa(i)] = map[string]any{"index": i, "ocr": false, "image": "", "content": strings.Repeat("palavra ", 50)}
	}
	eventos := map[string]any{
		"1": map[string]any{
			"index": 1,
			"documentos": map[string]any{
				"x": map[string]any{"index": 1, "nome": "Evento 1 - INIC1.pdf", "paginas": paginas},
			},
		},
	}

	output := GenerateConsolidatedOutput(eventos, "basic")

	contexto, _, ok := strings.Cut(output, "</contexto>")
	if !ok {
		t.Fatalf("saída sem </contexto>:\n%s", output)
	}
	want := "    Tokens estimados: " + strconv.Itoa(EstimateTokens(output)) + "\n"
	if !strings.Contains(contexto, want) {
		t.Errorf("<contexto> deveria ter %q (estimativa da saída inteira):\n%s", want, contexto)
	}
}
