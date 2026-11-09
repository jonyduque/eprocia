package etl

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"testing"
	"time"

	"eprocia/scripts/bench/pdfgen"
)

const helvetica = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

// paginaComFonte monta os objetos de uma página com o stream dado e a fonte
// /F1 (objeto 5); extras viram os objetos 6, 7...
func paginaComFonte(stream, fonte string, extras ...string) []string {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream)+1, stream),
		"5 0 obj\n" + fonte + "\nendobj",
	}
	for i, e := range extras {
		objects = append(objects, fmt.Sprintf("%d 0 obj\n%s\nendobj", 6+i, e))
	}
	return objects
}

// extrairComPrazo roda ExtractNativeText sobre a primeira página e falha se ele
// travar (o lexer da biblioteca gira para sempre num stream ilegível).
func extrairComPrazo(t *testing.T, objects []string) string {
	t.Helper()
	page := primeiraPagina(t, objects)
	type resultado struct {
		texto string
		err   error
	}
	feito := make(chan resultado, 1)
	go func() {
		texto, err := ExtractNativeText(page)
		feito <- resultado{texto, err}
	}()
	select {
	case r := <-feito:
		if r.err != nil {
			t.Fatalf("ExtractNativeText: %v", r.err)
		}
		return strings.TrimSpace(r.texto)
	case <-time.After(5 * time.Second):
		t.Fatal("ExtractNativeText travou")
		return ""
	}
}

// textoDoStream devolve o texto que ExtractNativeText extrai de uma página com
// a fonte /F1 Helvetica padrão (sem /Widths) e o stream dado.
func textoDoStream(t *testing.T, stream string) string {
	t.Helper()
	return extrairComPrazo(t, paginaComFonte(stream, helvetica))
}

// helveticaComWidths é a Helvetica com /Widths reais (AFM) de 32 a 126.
func helveticaComWidths() string {
	larguras := map[byte]int{
		' ': 278, '$': 556, '.': 278, 'C': 722, 'K': 667, 'M': 833, 'N': 722, 'O': 778, 'R': 722, 'W': 944,
		'a': 556, 'e': 556, 'g': 556, 'i': 222, 'l': 222, 'm': 833, 'n': 556, 'p': 556, 'r': 333, 's': 500,
		't': 278, 'u': 556, 'v': 500,
	}
	var ws []string
	for c := 32; c <= 126; c++ {
		w, ok := larguras[byte(c)]
		if !ok {
			w = 556
		}
		ws = append(ws, fmt.Sprint(w))
	}
	return "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FirstChar 32 /LastChar 126 /Widths [" + strings.Join(ws, " ") + "] >>"
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

// Palavras em minúscula posicionadas uma a uma por Td, com o espaço real da
// Helvetica 12 entre elas ("estimativa" mede 54 pt), numa fonte sem /Widths.
func TestTextoPosicional_PalavraPorPalavraComTd(t *testing.T) {
	got := textoDoStream(t, emLinha("(estimativa) Tj 57.3 0 Td (seguinte) Tj 46 0 Td (palavra) Tj"))
	if want := "estimativa seguinte palavra"; got != want {
		t.Errorf("texto = %q, quero %q", got, want)
	}
}

// Com /Widths na fonte, o fim de cada trecho vem das larguras reais: maiúsculas
// largas encostadas não ganham espaço, e palavras com o espaço real separam.
func TestTextoPosicional_LargurasReaisDaFonte(t *testing.T) {
	casos := []struct{ stream, want string }{
		// C = 8,66 pt e O = 9,34 pt na Helvetica 12: a estimativa (4,8 + 3) separaria.
		{"(C) Tj 8.7 0 Td (O) Tj 9.3 0 Td (N) Tj", "CON"},
		// "estimativa" = 54,0 pt e "seguinte" = 45,3 pt, mais o espaço de 3,3 pt.
		{"(estimativa) Tj 57.3 0 Td (seguinte) Tj 48.6 0 Td (palavra) Tj", "estimativa seguinte palavra"},
		{"(R) Tj 7 0 Td ($) Tj 7 0 Td (1) Tj 7 0 Td (.) Tj 4 0 Td (2) Tj 7 0 Td (3) Tj 7 0 Td (4) Tj 20 0 Td (OK) Tj", "R$1.234 OK"},
		{"[(C) -20 (O) -20 (N) -20 (T) -20 (E) -20 (S) -20 (T) -20 (A)] TJ", "CONTESTA"},
		{"[(casa) -280 (e) -280 (campo)] TJ", "casa e campo"},
	}
	for _, c := range casos {
		if got := extrairComPrazo(t, paginaComFonte(emLinha(c.stream), helveticaComWidths())); got != c.want {
			t.Errorf("%s: texto = %q, quero %q", c.stream, got, c.want)
		}
	}
}

// /ToUnicode corrompido (Flate truncado): o extrator não pode travar no lexer
// da biblioteca; a fonte é lida sem ToUnicode.
func TestTextoPosicional_ToUnicodeCorrompidoNaoTrava(t *testing.T) {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	for i := range 200 {
		fmt.Fprintf(w, "1 beginbfchar\n<%02X> <%04X>\nendbfchar\n", i%256, 0x41+i%26)
	}
	w.Close()
	truncado := z.Bytes()[:z.Len()/2]
	fonte := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /ToUnicode 6 0 R >>"
	toUnicode := fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", len(truncado), truncado)

	got := extrairComPrazo(t, paginaComFonte(emLinha("(Teste) Tj"), fonte, toUnicode))
	if got != "Teste" {
		t.Errorf("texto = %q, quero %q", got, "Teste")
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
	if got, want := extrairComPrazo(t, objects), "Primeira linha\nSegunda linha"; got != want {
		t.Errorf("texto = %q, quero %q", got, want)
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
