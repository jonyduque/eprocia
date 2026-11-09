package etl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestTextoIlegivel(t *testing.T) {
	casos := map[string]bool{
		"Texto jurídico normal com palavras e números 123.": false,
		"R$ 1.234,56 | R$ 2.345,67 | 31/12/2026 | 10,00%":   false, // tabela numérica
		strings.Repeat("\x01\x05\x08", 8):                   true,  // 24 runas
		"\x01\x05\x08\x01\x05\x08\x01\x05\x08\x01\x05\x08":  false, // abaixo de 20 runas
		"#$%&*@#$%&*@ ¨¨ ~~ ^^ ## a":                        true,
		"":                                                  false,
	}
	for s, want := range casos {
		if textoIlegivel(s) != want {
			t.Errorf("%q: want %v", s, want)
		}
	}
}

func TestMarcadorIlegivel_ContaRunasNaoBytes(t *testing.T) {
	texto := "ação é ônus" // 11 runas, 14 bytes
	if got, want := marcadorIlegivel(texto), "[texto ilegível na página: 11 caracteres omitidos]"; got != want {
		t.Errorf("marcador = %q, quero %q", got, want)
	}
}

// Páginas legítimas dominadas por caracteres de preenchimento não são lixo.
func TestTextoIlegivel_PreenchimentoLegitimo(t *testing.T) {
	sublinha := strings.Repeat("_", 22)
	paginas := map[string]string{
		"assinatura": strings.Join([]string{
			sublinha + " FULANO DE TAL " + sublinha,
			sublinha + " Beltrano da Silva " + sublinha,
			sublinha,
			sublinha + " Advogado " + sublinha,
		}, "\n"),
		"indice": strings.Join([]string{
			"Introdução " + strings.Repeat(".", 28) + " 3",
			"Dos fatos " + strings.Repeat(".", 30) + " 5",
			"Do direito " + strings.Repeat(".", 29) + " 9",
			"Conclusão " + strings.Repeat(".", 30) + " 12",
		}, "\n"),
		"intercalado": strings.Join([]string{
			"Assinatura: " + strings.Repeat("_ ", 30), "Fulano",
			"Assinatura: " + strings.Repeat("_ ", 30), "Data: " + strings.Repeat(". ", 10),
		}, "'+chr(92)+'n"),
		"separadores": strings.Join([]string{
			strings.Repeat("-", 40), strings.Repeat("=", 40), "ANEXO I",
			strings.Repeat("*", 40), strings.Repeat("-", 40),
		}, "\n"),
	}
	for nome, s := range paginas {
		if textoIlegivel(s) {
			t.Errorf("%s: página legítima marcada como ilegível", nome)
		}
	}
}

// pdfPaginaIlegivel grava um PDF de uma página sem imagem, cujo Tj tem 150
// bytes de controle (fonte com codificação quebrada).
func pdfPaginaIlegivel(t *testing.T) string {
	t.Helper()
	lixo := strings.Repeat(`\001\005\010`, 50)
	stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", lixo)
	return gravarPDF(t, []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream)+1, stream),
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj",
	})
}

func TestParsePages_IlegivelSemOCRViraMarcador(t *testing.T) {
	caminho := pdfPaginaIlegivel(t)
	pages, err := parserDeArquivo(t, caminho).ParsePages(context.Background(), false, nil, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	pag := pages["1"].(map[string]any)
	if pag["content"] != marcadorIlegivel(strings.Repeat("x", 150)) {
		t.Errorf("conteúdo = %q, quero %q", pag["content"], marcadorIlegivel(strings.Repeat("x", 150)))
	}
}

func TestParsePages_IlegivelVaiAoOCR(t *testing.T) {
	caminho := pdfPaginaIlegivel(t)
	lote := func(ctx context.Context, path string, paginas []int, _ func(int)) ([]string, error) {
		return []string{"texto do ocr"}, nil
	}
	porPagina := func(ctx context.Context, path string, n int) (string, error) {
		return "texto do ocr", nil // bloco de uma página só usa o OCR por página
	}
	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 1).ParsePages(context.Background(), true, porPagina, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	pag := pages["1"].(map[string]any)
	if pag["content"] != "texto do ocr" || pag["ocr"] != true {
		t.Errorf("página = %v", pag)
	}
}

func TestParsePages_IlegivelComOCRFalhoViraMarcador(t *testing.T) {
	caminho := pdfPaginaIlegivel(t)
	lote := func(ctx context.Context, path string, paginas []int, _ func(int)) ([]string, error) {
		return nil, errors.New("tesseract caiu")
	}
	porPagina := func(ctx context.Context, path string, n int) (string, error) {
		return "", errors.New("falhou")
	}
	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 1).ParsePages(context.Background(), true, porPagina, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	pag := pages["1"].(map[string]any)
	if pag["content"] != marcadorIlegivel(strings.Repeat("x", 150)) {
		t.Errorf("conteúdo = %q, quero o marcador", pag["content"])
	}
}

func TestTextoIlegivel_SimbolosRepetidosEmLinhas(t *testing.T) {
	linha := strings.Repeat("#$%&*@", 2)
	if !textoIlegivel(strings.Join([]string{linha, linha, linha}, "'+chr(92)+'n")) {
		t.Error("símbolos repetidos em várias linhas deveriam ser ilegíveis")
	}
}
