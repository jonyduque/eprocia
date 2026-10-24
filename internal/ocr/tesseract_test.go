package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestPDF constructs a valid minimal PDF by dynamically calculating offsets for the xref table.
func buildTestPDF(objects []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		buf.WriteString(obj)
		buf.WriteString("\n")
	}

	xrefOffset := buf.Len()
	buf.WriteString("xref\n")
	buf.WriteString(fmt.Sprintf("0 %d\n", len(objects)+1))
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
	}

	buf.WriteString("trailer\n")
	buf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	buf.WriteString("%%EOF")

	return buf.Bytes()
}

func TestRunOCR(t *testing.T) {
	// Initialize and extract embedded binaries
	binPath, errInit := InitBinaries()
	if errInit != nil {
		t.Skipf("Skipping OCR integration test: failed to initialize embedded binaries: %v", errInit)
	}

	tmpDir, err := os.MkdirTemp("", "ocr_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "test.pdf")

	// Create a PDF with clear text "TESSERACT TEST" and font resources defined so it can be rendered
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 50 >>\nstream\nBT\n/F1 24 Tf\n72 712 Td\n(TESSERACT TEST) Tj\nET\nendstream\nendobj",
	}

	err = os.WriteFile(pdfPath, buildTestPDF(objects), 0644)
	if err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}

	// Run using the fast mode
	text, _, err := RunOCR(context.Background(), pdfPath, 1, "fast")
	if err != nil {
		t.Fatalf("RunOCR failed: %v", err)
	}

	// Tesseract OCR should be able to recognize "TESSERACT" or "TEST" from the rendered image.
	normalized := strings.ToUpper(text)
	if !strings.Contains(normalized, "TESSERACT") && !strings.Contains(normalized, "TEST") {
		t.Errorf("expected OCR text to contain 'TESSERACT' or 'TEST', got %q (binPath: %s)", text, binPath)
	}
}

// A transient failure while extracting the binaries must not stick for the
// whole process lifetime: the next call has to try again.
func TestInitBinaries_RetriesAfterFailure(t *testing.T) {
	resetInitForTest()
	t.Cleanup(resetInitForTest)

	blocker := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(blocker, nil, 0644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}
	// TempDir pointing at a regular file makes MkdirAll fail.
	t.Setenv("TMP", blocker)
	t.Setenv("TEMP", blocker)
	t.Setenv("TMPDIR", blocker)
	if _, err := InitBinaries(); err == nil {
		t.Fatalf("expected InitBinaries to fail with TEMP pointing to a file")
	}

	good := t.TempDir()
	t.Setenv("TMP", good)
	t.Setenv("TEMP", good)
	t.Setenv("TMPDIR", good)
	dir, err := InitBinaries()
	if err != nil {
		t.Fatalf("expected InitBinaries to retry and succeed, got %v", err)
	}
	if dir != filepath.Join(good, "eprocIA_bin") {
		t.Errorf("expected bin dir under the new TEMP, got %q", dir)
	}
}

func resetInitForTest() {
	initMu.Lock()
	defer initMu.Unlock()
	binDir = ""
	tessdataDirs = map[string]string{}
}

// pdfDePalavras grava um PDF com uma palavra grande por página.
func pdfDePalavras(t *testing.T, palavras []string) string {
	t.Helper()
	objs := []string{"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj", ""}
	var kids []string
	for _, p := range palavras {
		pag, cont := len(objs)+1, len(objs)+2
		kids = append(kids, fmt.Sprintf("%d 0 R", pag))
		stream := fmt.Sprintf("BT\n/F1 36 Tf\n72 700 Td\n(%s) Tj\nET", p)
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents %d 0 R >>\nendobj", pag, cont),
			fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", cont, len(stream)+1, stream),
		)
	}
	objs[1] = fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), len(palavras))
	caminho := filepath.Join(t.TempDir(), "palavras.pdf")
	if err := os.WriteFile(caminho, buildTestPDF(objs), 0644); err != nil {
		t.Fatal(err)
	}
	return caminho
}

// Várias páginas de um documento vão ao OCR com um pdftoppm por trecho
// contíguo e um único tesseract (B-EF-04), e cada texto volta na página certa.
func TestRunOCRPaginas_UmTesseractPorBloco(t *testing.T) {
	if _, err := InitBinaries(); err != nil {
		t.Fatalf("falha ao extrair os binários embutidos: %v", err)
	}
	palavras := []string{"PRIMEIRA", "SEGUNDA", "TERCEIRA", "QUARTA"}
	caminho := pdfDePalavras(t, palavras)

	casos := []struct {
		paginas []int
		procs   int64
	}{
		{[]int{1, 2, 3, 4}, 2}, // um trecho contíguo: 1 pdftoppm + 1 tesseract
		{[]int{1, 3, 4}, 3},    // dois trechos: 2 pdftoppm + 1 tesseract
	}
	for _, c := range casos {
		antes := SpawnedProcesses()
		textos, err := RunOCRPaginas(context.Background(), caminho, c.paginas, "fast")
		if err != nil {
			t.Fatalf("RunOCRPaginas(%v): %v", c.paginas, err)
		}
		if got := SpawnedProcesses() - antes; got != c.procs {
			t.Errorf("RunOCRPaginas(%v) abriu %d processos, quero %d", c.paginas, got, c.procs)
		}
		if len(textos) != len(c.paginas) {
			t.Fatalf("RunOCRPaginas(%v) devolveu %d textos", c.paginas, len(textos))
		}
		for i, p := range c.paginas {
			got := strings.ToUpper(textos[i])
			for j, w := range palavras {
				if tem := strings.Contains(got, w); tem != (j == p-1) {
					t.Errorf("texto da página %d = %q (palavra %s presente: %t)", p, textos[i], w, tem)
				}
			}
		}
	}
}
