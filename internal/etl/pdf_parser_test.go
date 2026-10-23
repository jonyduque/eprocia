package etl

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

// buildPDF constructs a valid minimal PDF by dynamically calculating offsets for the xref table.
func buildPDF(objects []string) []byte {
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

func TestPDFParser_ParsePages_NativeText(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pdf_parser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "test.pdf")

	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 45 >>\nstream\nBT\n/F1 12 Tf\n72 712 Td\n(Hello World) Tj\nET\nendstream\nendobj",
	}

	err = os.WriteFile(pdfPath, buildPDF(objects), 0644)
	if err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}

	parser := parserDeArquivo(t, pdfPath)
	pages, err := parser.ParsePages(context.Background(), true, nil, nil)
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	page, ok := pages["1"].(map[string]any)
	if !ok {
		t.Fatalf("expected page '1' to exist in map")
	}
	if page["index"].(int) != 1 {
		t.Errorf("expected page index 1, got %v", page["index"])
	}

	if page["ocr"].(bool) {
		t.Errorf("expected IsOcr to be false for native text page")
	}

	content := page["content"].(string)
	if !strings.Contains(content, "Hello") || !strings.Contains(content, "World") {
		t.Errorf("expected extracted text to contain 'Hello World', got %q", content)
	}
}

func TestPDFParser_ParsePages_UsesOcrForImagePage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pdf_parser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "image_test.pdf")

	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 12 >>\nstream\n/Im1 Do\nendstream\nendobj",
		"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}

	err = os.WriteFile(pdfPath, buildPDF(objects), 0644)
	if err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}

	parser := parserDeArquivo(t, pdfPath)

	ocrCalled := false
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		ocrCalled = true
		return "OCR Extracted Text", "mock_image.png", nil
	}

	pages, err := parser.ParsePages(context.Background(), true, ocrRunner, nil)
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	page, ok := pages["1"].(map[string]any)
	if !ok {
		t.Fatalf("expected page '1' to exist in map")
	}
	if !page["ocr"].(bool) {
		t.Errorf("expected IsOcr to be true")
	}

	if !ocrCalled {
		t.Errorf("expected OCR runner to be called")
	}

	content := page["content"].(string)
	if content != "OCR Extracted Text" {
		t.Errorf("expected page text to be 'OCR Extracted Text', got %q", content)
	}
	if page["image"].(string) != "mock_image.png" {
		t.Errorf("expected page image to be 'mock_image.png', got %q", page["image"])
	}
}

// A failed OCR on a scanned page must not silently produce an empty page: the
// output must carry an error marker so the user notices the missing content.
func TestPDFParser_ParsePages_OcrFailureMarksPage(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "image_test.pdf")
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 12 >>\nstream\n/Im1 Do\nendstream\nendobj",
		"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}
	if err := os.WriteFile(pdfPath, buildPDF(objects), 0644); err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}

	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		return "", "", errors.New("tesseract ausente")
	}
	completed := 0
	pages, err := parserDeArquivo(t, pdfPath).ParsePages(context.Background(), true, ocrRunner, func(int) { completed++ })
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}

	page := pages["1"].(map[string]any)
	if page["ocr"].(bool) {
		t.Errorf("expected ocr=false after OCR failure")
	}
	if page["image"].(string) != "" {
		t.Errorf("expected empty image after OCR failure, got %q", page["image"])
	}
	content := page["content"].(string)
	if !strings.HasPrefix(content, "[ERRO de OCR:") || !strings.Contains(content, "tesseract ausente") {
		t.Errorf("expected OCR error marker in content, got %q", content)
	}
	if page["ocr_erro"] != "tesseract ausente" {
		t.Errorf("expected ocr_erro to carry the failure, got %v", page["ocr_erro"])
	}
	if completed != 1 {
		t.Errorf("expected onPageComplete once, got %d", completed)
	}
}

// parsePagesWithDeadline runs ParsePages but fails the test instead of hanging
// forever: the PDF library spins in an infinite loop on unreadable content streams.
func parsePagesWithDeadline(t *testing.T, objects []string, ocrRunner func(context.Context, string, int) (string, string, error)) map[string]any {
	t.Helper()
	pdfPath := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(pdfPath, buildPDF(objects), 0644); err != nil {
		t.Fatalf("failed to write test pdf: %v", err)
	}

	type result struct {
		pages map[string]any
		err   error
	}
	done := make(chan result, 1)
	parser := parserDeArquivo(t, pdfPath)
	go func() {
		pages, err := parser.ParsePages(context.Background(), true, ocrRunner, nil)
		done <- result{pages, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("ParsePages failed: %v", r.err)
		}
		return r.pages
	case <-time.After(5 * time.Second):
		t.Fatalf("ParsePages travou (loop infinito na extração de texto nativo)")
		return nil
	}
}

// "Microsoft: Print To PDF" writes /Contents as an array of streams, which the
// PDF library cannot interpret natively. The page must fall back to OCR instead
// of hanging the whole queue.
func TestPDFParser_ParsePages_ContentsArrayDoesNotHang(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents [ 4 0 R ] >>\nendobj",
		"4 0 obj\n<< /Length 8 >>\nstream\n/Im1 Do\nendstream\nendobj",
		"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		return "OCR Extracted Text", "mock_image.png", nil
	}

	pages := parsePagesWithDeadline(t, objects, ocrRunner)

	page := pages["1"].(map[string]any)
	if !page["ocr"].(bool) || page["content"].(string) != "OCR Extracted Text" {
		t.Errorf("expected OCR fallback for page with /Contents array, got %+v", page)
	}
}

func TestPDFParser_ParsePages_MissingContentsDoesNotHang(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> >>\nendobj",
	}

	pages := parsePagesWithDeadline(t, objects, nil)

	page := pages["1"].(map[string]any)
	if page["content"].(string) != "" {
		t.Errorf("expected empty content for blank page, got %q", page["content"])
	}
}

// Some generators (e.g. iLovePDF) draw text as vector outlines: the page has no
// fonts and no images, only a large path-painting content stream. It has no
// native text, so it must still go through OCR instead of coming out empty.
func TestPDFParser_ParsePages_VectorTextPageUsesOcr(t *testing.T) {
	var paths strings.Builder
	for paths.Len() < 8192 {
		paths.WriteString("40.16 196.8 m\n744.32 196.8 l\n744.32 199.04 l\nh\nf\n")
	}
	content := paths.String()
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(content), content),
	}
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		return "OCR Extracted Text", "mock_image.png", nil
	}

	pages := parsePagesWithDeadline(t, objects, ocrRunner)

	page := pages["1"].(map[string]any)
	if !page["ocr"].(bool) || page["content"].(string) != "OCR Extracted Text" {
		t.Errorf("expected OCR for vector-text page, got %+v", page)
	}
}

// A blank page (tiny content stream) must not waste an OCR run.
func TestPDFParser_ParsePages_BlankPageSkipsOcr(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 4 >>\nstream\nq Q\nendstream\nendobj",
	}
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		t.Errorf("OCR must not run on a blank page")
		return "", "", nil
	}

	parsePagesWithDeadline(t, objects, ocrRunner)
}

// parsePagesSemPanic runs ParsePages and turns a panic into a test failure: in
// the app, ParsePages runs in a pipeline goroutine and a panic kills the process.
func parsePagesSemPanic(t *testing.T, objects []string, ocrRunner func(context.Context, string, int) (string, string, error)) (pages map[string]any, err error) {
	t.Helper()
	pdfPath := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(pdfPath, buildPDF(objects), 0644); err != nil {
		t.Fatalf("falha ao gravar pdf de teste: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParsePages entrou em panic: %v", r)
		}
	}()
	return parserDeArquivo(t, pdfPath).ParsePages(context.Background(), true, ocrRunner, nil)
}

// A xref entry that points to an object with another number makes the PDF
// library panic ("loading 3 0 R: found 9 0") when the page is resolved.
func TestPDFParser_ParsePages_XrefInconsistenteNaPaginaNaoEntraEmPanic(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [4 0 R 3 0 R] /Count 2 >>\nendobj",
		"9 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> >>\nendobj",
		"4 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 5 0 R >>\nendobj",
		"5 0 obj\n<< /Length 45 >>\nstream\nBT\n/F1 12 Tf\n72 712 Td\n(Hello World) Tj\nET\nendstream\nendobj",
	}

	pages, err := parsePagesSemPanic(t, objects, nil)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	p1, ok := pages["1"].(map[string]any)
	if !ok || !strings.Contains(p1["content"].(string), "Hello") {
		t.Errorf("esperava página 1 com o texto nativo, obtive %+v", pages["1"])
	}
	p2, ok := pages["2"].(map[string]any)
	if !ok || !strings.Contains(p2["content"].(string), "[ERRO") {
		t.Errorf("esperava página 2 marcada com [ERRO ...], obtive %+v", pages["2"])
	}
}

func TestPDFParser_ParsePages_XrefInconsistenteEmHasImagesNaoEntraEmPanic(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject 5 0 R >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 4 >>\nstream\nq Q\nendstream\nendobj",
		"9 0 obj\n<< /Im1 6 0 R >>\nendobj",
	}
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		return "OCR Extracted Text", "mock_image.png", nil
	}

	pages, err := parsePagesSemPanic(t, objects, ocrRunner)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	if _, ok := pages["1"].(map[string]any); !ok {
		t.Errorf("esperava a página 1 no resultado, obtive %+v", pages)
	}
}

func TestPDFParser_ParsePages_XrefInconsistenteNoCatalogoNaoEntraEmPanic(t *testing.T) {
	objects := []string{
		"9 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> >>\nendobj",
	}

	if _, err := parsePagesSemPanic(t, objects, nil); err == nil {
		t.Errorf("esperava erro para PDF com catálogo ilegível")
	}
}

// parsePagesComPrazo runs ParsePages and fails the test if it does not return
// within the deadline, returning ParsePages' error to the caller.
func parsePagesComPrazo(t *testing.T, objects []string) (map[string]any, error) {
	t.Helper()
	pdfPath := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(pdfPath, buildPDF(objects), 0644); err != nil {
		t.Fatalf("falha ao gravar pdf de teste: %v", err)
	}
	type resultado struct {
		pages map[string]any
		err   error
	}
	done := make(chan resultado, 1)
	parser := parserDeArquivo(t, pdfPath)
	go func() {
		pages, err := parser.ParsePages(context.Background(), true, nil, nil)
		done <- resultado{pages, err}
	}()
	select {
	case r := <-done:
		return r.pages, r.err
	case <-time.After(5 * time.Second):
		t.Fatalf("ParsePages travou na árvore de páginas")
		return nil, nil
	}
}

// Reader.Page in the PDF library descends /Kids with no visited set, so a
// cyclic /Pages tree spins forever and holds the whole queue.
func TestPDFParser_ParsePages_ArvoreDePaginasCiclicaNaoTrava(t *testing.T) {
	casos := map[string][]string{
		"filho de si mesmo": {
			"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
			"2 0 obj\n<< /Type /Pages /Kids [2 0 R] /Count 1 >>\nendobj",
		},
		"ciclo entre dois nós": {
			"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
			"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
			"3 0 obj\n<< /Type /Pages /Parent 2 0 R /Kids [2 0 R] /Count 1 >>\nendobj",
		},
		"nó repetido entre irmãos": {
			"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
			"2 0 obj\n<< /Type /Pages /Kids [2 0 R 2 0 R] /Count 1 >>\nendobj",
		},
	}
	for nome, objects := range casos {
		t.Run(nome, func(t *testing.T) {
			if _, err := parsePagesComPrazo(t, objects); err == nil {
				t.Errorf("esperava erro para árvore de páginas cíclica")
			}
		})
	}
}

// A well-formed nested tree must keep the document order of its leaves.
func TestPDFParser_ParsePages_ArvoreAninhadaMantemOrdem(t *testing.T) {
	texto := func(s string) string {
		return fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET\n", s)
	}
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 3 >>\nendobj",
		"3 0 obj\n<< /Type /Pages /Parent 2 0 R /Kids [4 0 R 5 0 R] /Count 2 >>\nendobj",
		"4 0 obj\n<< /Type /Page /Parent 3 0 R /Resources << >> /Contents 7 0 R >>\nendobj",
		"5 0 obj\n<< /Type /Page /Parent 3 0 R /Resources << >> /Contents 8 0 R >>\nendobj",
		"6 0 obj\n<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 9 0 R >>\nendobj",
		fmt.Sprintf("7 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(texto("Primeira")), texto("Primeira")),
		fmt.Sprintf("8 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(texto("Segunda")), texto("Segunda")),
		fmt.Sprintf("9 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(texto("Terceira")), texto("Terceira")),
	}

	pages, err := parsePagesComPrazo(t, objects)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	for num, esperado := range map[string]string{"1": "Primeira", "2": "Segunda", "3": "Terceira"} {
		p, ok := pages[num].(map[string]any)
		if !ok || !strings.Contains(p["content"].(string), esperado) {
			t.Errorf("página %s: esperava %q, obtive %+v", num, esperado, pages[num])
		}
	}
}

// parserDeArquivo abre o PDF como o pipeline faz (leitor + caminho para OCR).
func parserDeArquivo(t testing.TB, caminho string) *PDFParser {
	t.Helper()
	f, err := os.Open(caminho)
	if err != nil {
		t.Fatalf("falha ao abrir %s: %v", caminho, err)
	}
	t.Cleanup(func() { f.Close() })
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	return NewPDFParserDeLeitor(f, info.Size(), caminho, func() (string, error) { return caminho, nil })
}

// O erro de abertura vira a mensagem do item e pode ir para o texto de saída.
func TestParsePages_ErroDeAberturaEmPortugues(t *testing.T) {
	dados := []byte("isto não é um PDF")
	_, err := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "falso.pdf", nil).
		ParsePages(context.Background(), false, nil, nil)
	if err == nil {
		t.Fatal("ParsePages aceitou um arquivo que não é PDF")
	}
	if !strings.Contains(err.Error(), "falha ao abrir o PDF") {
		t.Fatalf("mensagem fora do padrão pt-BR: %q", err)
	}
}
