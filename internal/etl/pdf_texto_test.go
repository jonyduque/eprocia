package etl

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"eprocia/scripts/bench/pdfgen"
)

// textoDoStream monta uma página com a fonte /F1 Helvetica e o stream dado e
// devolve o texto que ExtractNativeText extrai dela.
func textoDoStream(t *testing.T, stream string) string {
	t.Helper()
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream)+1, stream),
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj",
	}
	texto, err := ExtractNativeText(primeiraPagina(t, objects))
	if err != nil {
		t.Fatalf("ExtractNativeText: %v", err)
	}
	return strings.TrimSpace(texto)
}

func emLinha(corpo string) string {
	return "BT /F1 12 Tf 72 700 Td " + corpo + " ET"
}

// Kerning pequeno entre letras de um TJ não é espaço de palavra.
func TestTextoPosicional_TJLetraALetra(t *testing.T) {
	got := textoDoStream(t, emLinha("[(C) -20 (O) -20 (N) -20 (T) -20 (E) -20 (S) -20 (T) -20 (A)] TJ"))
	if got != "CONTESTA" {
		t.Errorf("texto = %q, quero %q", got, "CONTESTA")
	}
}

// Um deslocamento grande dentro do TJ separa palavras, inclusive as curtas.
func TestTextoPosicional_TJEspacoDePalavra(t *testing.T) {
	got := textoDoStream(t, emLinha("[(casa) -280 (e) -280 (campo)] TJ"))
	if got != "casa e campo" {
		t.Errorf("texto = %q, quero %q", got, "casa e campo")
	}
}

// Glifos posicionados um a um com Td: valor junto, palavra afastada separada.
func TestTextoPosicional_TjPorLetraComTd(t *testing.T) {
	got := textoDoStream(t, emLinha("(R) Tj 7 0 Td ($) Tj 7 0 Td (1) Tj 7 0 Td (.) Tj 4 0 Td (2) Tj 7 0 Td (3) Tj 7 0 Td (4) Tj 20 0 Td (OK) Tj"))
	if got != "R$1.234 OK" {
		t.Errorf("texto = %q, quero %q", got, "R$1.234 OK")
	}
}

func TestTextoPosicional_DuasLinhas(t *testing.T) {
	got := textoDoStream(t, emLinha("(Primeira linha) Tj 0 -14 Td (Segunda linha) Tj"))
	if got != "Primeira linha\nSegunda linha" {
		t.Errorf("texto = %q, quero %q", got, "Primeira linha\nSegunda linha")
	}
}

// /Contents em array ("Microsoft: Print To PDF"): os streams são lidos em ordem,
// com o estado de texto compartilhado, sem travar.
func TestTextoPosicional_ContentsEmArray(t *testing.T) {
	s1 := "BT /F1 12 Tf 72 700 Td (Primeira linha) Tj"
	s2 := "0 -14 Td (Segunda linha) Tj ET"
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 6 0 R >> >> /Contents [4 0 R 5 0 R] >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(s1)+1, s1),
		fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(s2)+1, s2),
		"6 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj",
	}
	feito := make(chan string, 1)
	go func() {
		texto, err := ExtractNativeText(primeiraPagina(t, objects))
		if err != nil {
			t.Errorf("ExtractNativeText: %v", err)
		}
		feito <- strings.TrimSpace(texto)
	}()
	select {
	case got := <-feito:
		if want := "Primeira linha\nSegunda linha"; got != want {
			t.Errorf("texto = %q, quero %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExtractNativeText travou com /Contents em array")
	}
}

// Texto comum, uma linha por Tj com T*, sai como hoje: uma linha por linha.
func TestTextoPosicional_TextoNormalIgualAoAtual(t *testing.T) {
	linhas := []string{
		"EXCELENTISSIMO SENHOR DOUTOR JUIZ DE DIREITO",
		"FULANO DE TAL, ja qualificado nos autos 5000000-00.2026.8.26.0001,",
		"vem apresentar CONTESTACAO pelos fatos a seguir.",
	}
	got := textoDoStream(t, pdfgen.NativeTextPage(linhas).Content)
	if want := strings.Join(linhas, "\n"); got != want {
		t.Errorf("texto = %q, quero %q", got, want)
	}
}
