package etl

import (
	"strings"
	"testing"

	"eprocia/internal/models"
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

func TestGenerateConsolidatedOutput(t *testing.T) {
	files := []models.ProcessFile{
		{DocName: "Evento 1 - INIC1.pdf", EventNum: 1, Name: "zip_path_1"},
		{DocName: "Evento 2 - DESPACHO.pdf", EventNum: 2, Name: "zip_path_2"},
	}

	fileContents := map[string][]PageData{
		"zip_path_1": {
			{PageNum: 1, Text: "Initial petition text page 1"},
			{PageNum: 2, Text: "Initial petition text page 2"},
		},
		"zip_path_2": {
			{PageNum: 1, Text: "Judge order text page 1"},
		},
	}

	output := GenerateConsolidatedOutput(files, fileContents, "basic")

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
