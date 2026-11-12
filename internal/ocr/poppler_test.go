package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gerarPDF monta um PDF 1.4 mínimo com uma página por item de frases; frase
// vazia dá uma página sem texto. Grava em dir/nome e devolve o caminho.
func gerarPDF(t *testing.T, dir, nome string, frases ...string) string {
	t.Helper()
	var b bytes.Buffer
	offs := []int{}
	obj := func(n int, corpo string) {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, corpo)
	}
	b.WriteString("%PDF-1.4\n")
	// 1 catálogo, 2 páginas, 3 fonte; página i: 4+2i, conteúdo: 5+2i.
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	var kids []string
	for i := range frases {
		kids = append(kids, fmt.Sprintf("%d 0 R", 4+2*i))
	}
	obj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(frases)))
	obj(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, f := range frases {
		obj(4+2*i, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i))
		fluxo := ""
		if f != "" {
			fluxo = fmt.Sprintf("BT /F1 18 Tf 72 700 Td (%s) Tj ET", f)
		}
		obj(5+2*i, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(fluxo), fluxo))
	}
	xref := b.Len()
	n := len(offs) + 1
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", n)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", n, xref)
	p := filepath.Join(dir, nome)
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func pularCurto(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("usa os binários reais do Poppler")
	}
}

func TestInfoPDF_ContaPaginas(t *testing.T) {
	pularCurto(t)
	p := gerarPDF(t, t.TempDir(), "tres.pdf", "PAGINA UM", "PAGINA DOIS", "PAGINA TRES")
	n, err := InfoPDF(context.Background(), p)
	if err != nil || n != 3 {
		t.Fatalf("InfoPDF = %d, %v; quero 3, nil", n, err)
	}
}

func TestTextoPDF_UmTextoPorPagina(t *testing.T) {
	pularCurto(t)
	p := gerarPDF(t, t.TempDir(), "tres.pdf", "PAGINA UM", "PAGINA DOIS", "PAGINA TRES")
	textos, err := TextoPDF(context.Background(), p, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(textos) != 3 {
		t.Fatalf("%d textos, quero 3: %q", len(textos), textos)
	}
	for i, frase := range []string{"PAGINA UM", "PAGINA DOIS", "PAGINA TRES"} {
		if !strings.Contains(textos[i], frase) {
			t.Errorf("texto %d = %q, quero conter %q", i, textos[i], frase)
		}
		if strings.ContainsAny(textos[i], "\r\f") {
			t.Errorf("texto %d carrega \r ou \f: %q", i, textos[i])
		}
	}
}

func TestTextoPDF_PaginaSemTexto(t *testing.T) {
	pularCurto(t)
	p := gerarPDF(t, t.TempDir(), "meio.pdf", "PAGINA UM", "", "PAGINA TRES")
	textos, err := TextoPDF(context.Background(), p, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(textos) != 3 {
		t.Fatalf("%d textos, quero 3: %q", len(textos), textos)
	}
	if strings.TrimSpace(textos[1]) != "" {
		t.Errorf("página do meio = %q, quero vazia", textos[1])
	}
}

func TestTextoPDF_UltimaPaginaSemTexto(t *testing.T) {
	pularCurto(t)
	p := gerarPDF(t, t.TempDir(), "fim.pdf", "PAGINA UM", "")
	textos, err := TextoPDF(context.Background(), p, 2)
	if err != nil || len(textos) != 2 {
		t.Fatalf("TextoPDF = %q, %v; quero 2 textos", textos, err)
	}
}

func TestPoppler_CaminhoComEspacoEAcento(t *testing.T) {
	pularCurto(t)
	dir := filepath.Join(t.TempDir(), "Área de Trabalho")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := gerarPDF(t, dir, "ação número 1.pdf", "PAGINA UM", "PAGINA DOIS")
	n, err := InfoPDF(context.Background(), p)
	if err != nil || n != 2 {
		t.Fatalf("InfoPDF = %d, %v; quero 2, nil", n, err)
	}
	textos, err := TextoPDF(context.Background(), p, n)
	if err != nil || len(textos) != 2 || !strings.Contains(textos[1], "PAGINA DOIS") {
		t.Fatalf("TextoPDF = %q, %v", textos, err)
	}
}

func TestInfoPDF_NaoEPDF(t *testing.T) {
	pularCurto(t)
	p := filepath.Join(t.TempDir(), "lixo.pdf")
	if err := os.WriteFile(p, bytes.Repeat([]byte{0x9f, 0x03, 0x41, 0xee}, 300), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := InfoPDF(context.Background(), p)
	if err == nil {
		t.Fatal("quero erro para um arquivo que não é PDF")
	}
	if errors.Is(err, ErrSenha) || errors.Is(err, ErrPrazo) {
		t.Errorf("erro = %v, não pode ser ErrSenha nem ErrPrazo", err)
	}
}

func TestInfoPDF_Prazo(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_DORMIR", "1")
	encurtar(t, &prazoInfo, 200*time.Millisecond)
	// Poppler falso: a pasta de verdade, com pdfinfo.exe trocado pelo binário
	// de teste, que dorme.
	dir := t.TempDir()
	if err := copiarArquivo(os.Args[0], filepath.Join(dir, "pdfinfo.exe")); err != nil {
		t.Fatal(err)
	}
	inicio := time.Now()
	_, err := rodarPoppler(context.Background(), dir, "pdfinfo.exe", prazoInfo, "x.pdf")
	if d := time.Since(inicio); d > 5*time.Second {
		t.Errorf("voltou em %v, quero menos de 5 s", d)
	}
	if !errors.Is(err, ErrPrazo) {
		t.Fatalf("erro = %v, quero ErrPrazo", err)
	}
}

// Prova que InfoPDF e TextoPDF não extraem os tessdata (dezenas de MB), numa
// pasta de binários própria do teste.
func TestPoppler_SemTessdata(t *testing.T) {
	pularCurto(t)
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	initMu.Lock()
	antigoBin, antigoPoppler, antigosTess := binDir, popplerDir, tessdataDirs
	binDir, popplerDir, tessdataDirs = "", "", map[string]string{}
	initMu.Unlock()
	t.Cleanup(func() {
		initMu.Lock()
		binDir, popplerDir, tessdataDirs = antigoBin, antigoPoppler, antigosTess
		initMu.Unlock()
	})

	p := gerarPDF(t, t.TempDir(), "um.pdf", "PAGINA UM", "PAGINA DOIS")
	n, err := InfoPDF(context.Background(), p)
	if err != nil || n != 2 {
		t.Fatalf("InfoPDF = %d, %v", n, err)
	}
	if _, err := TextoPDF(context.Background(), p, n); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(tmp, "eprocIA_bin")
	if _, err := os.Stat(filepath.Join(base, "poppler", "pdfinfo.exe")); err != nil {
		t.Errorf("poppler não foi extraído em %s: %v", base, err)
	}
	for _, sub := range []string{"tessdata", "tesseract"} {
		if _, err := os.Stat(filepath.Join(base, sub)); err == nil {
			t.Errorf("%s foi extraído, mas InfoPDF e TextoPDF só precisam do Poppler", sub)
		}
	}
}

func copiarArquivo(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}
