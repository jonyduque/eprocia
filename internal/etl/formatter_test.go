package etl

import (
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

func TestCleanText(t *testing.T) {
	rawText := `   
Line 1 with spaces   
Página 12 de 35
Documento assinado eletronicamente por Fulano
Line 2 with text
`

	// CleanLevel: none
	if cleaned := CleanText(rawText, "none"); cleaned != rawText {
		t.Errorf("CleanText('none') modified text; expected unchanged")
	}

	// CleanLevel: basic
	basicCleaned := CleanText(rawText, "basic")
	if !strings.Contains(basicCleaned, "Página 12 de 35") {
		t.Error("CleanText('basic') should preserve page markers")
	}
	if strings.HasPrefix(basicCleaned, " ") || strings.HasSuffix(basicCleaned, " ") {
		t.Error("CleanText('basic') should trim line whitespace")
	}

	// CleanLevel: aggressive
	aggCleaned := CleanText(rawText, "aggressive")
	if strings.Contains(aggCleaned, "Página 12 de 35") {
		t.Error("CleanText('aggressive') should strip page markers")
	}
	if strings.Contains(aggCleaned, "Documento assinado") {
		t.Error("CleanText('aggressive') should strip digital signatures")
	}
	if strings.Contains(aggCleaned, "\n") {
		t.Error("CleanText('aggressive') should merge lines into space-separated string")
	}
}

func TestCleanText_AggressiveKeepsLegitimateContent(t *testing.T) {
	raw := `O autor requer a juntada, conforme a página inicial dos autos.
Pág. 3/10
página 4
Página 12 de 35
Alega a parte que a assinatura eletrônica do contrato é nula.
Assinatura eletrônica realizada por Fulano de Tal
Documento assinado eletronicamente por Beltrano
A paginação do processo foi suspensa.`

	got := CleanText(raw, "aggressive")

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
