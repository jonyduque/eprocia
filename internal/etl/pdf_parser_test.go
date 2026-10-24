package etl

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dslipak/pdf"
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
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		ocrCalled = true
		return "OCR Extracted Text", nil
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
	if _, tem := page["image"]; tem {
		t.Errorf("a página não deveria ter o campo image (caminho de imagem temporária já apagada): %v", page["image"])
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

	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		return "", errors.New("tesseract ausente")
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
	if _, tem := page["image"]; tem {
		t.Errorf("a página não deveria ter o campo image: %v", page["image"])
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
func parsePagesWithDeadline(t *testing.T, objects []string, ocrRunner func(context.Context, string, int) (string, error)) map[string]any {
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
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		return "OCR Extracted Text", nil
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
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		return "OCR Extracted Text", nil
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
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		t.Errorf("OCR must not run on a blank page")
		return "", nil
	}

	parsePagesWithDeadline(t, objects, ocrRunner)
}

// parsePagesSemPanic runs ParsePages and turns a panic into a test failure: in
// the app, ParsePages runs in a pipeline goroutine and a panic kills the process.
func parsePagesSemPanic(t *testing.T, objects []string, ocrRunner func(context.Context, string, int) (string, error)) (pages map[string]any, err error) {
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
	ocrRunner := func(ctx context.Context, path string, pageNum int) (string, error) {
		return "OCR Extracted Text", nil
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

// assertNoLeaks falha o teste se o detector de vazamento do runtime achar
// goroutines bloqueadas para sempre (canal ou trava inalcançável).
func assertNoLeaks(t *testing.T) {
	t.Helper()
	perfil := pprof.Lookup("goroutineleak")
	if perfil == nil {
		t.Skip("perfil goroutineleak indisponível neste runtime")
	}
	var buf bytes.Buffer
	// WriteTo roda o GC com detecção de vazamento antes de Count ler o total.
	if err := perfil.WriteTo(&buf, 1); err != nil {
		t.Fatalf("falha ao gravar o perfil goroutineleak: %v", err)
	}
	if n := perfil.Count(); n > 0 {
		t.Fatalf("%d goroutine(s) vazada(s):\n%s", n, buf.String())
	}
}

// objetosPaginasComImagem monta um PDF com n páginas escaneadas (sem texto,
// com imagem grande), todas candidatas ao OCR.
func objetosPaginasComImagem(n int) []string {
	kids := make([]string, n)
	for i := range n {
		kids[i] = fmt.Sprintf("%d 0 R", 4+i)
	}
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), n),
		"3 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}
	for i := range n {
		objects = append(objects, fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 3 0 R >> >> /Contents %d 0 R >>\nendobj", 4+i, 4+n+i))
	}
	for i := range n {
		objects = append(objects, fmt.Sprintf("%d 0 obj\n<< /Length 8 >>\nstream\n/Im1 Do\nendstream\nendobj", 4+n+i))
	}
	return objects
}

// esperarParsePages roda ParsePages com ctx e falha o teste se não voltar no
// prazo (wg.Wait preso no cancelamento).
func esperarParsePages(t *testing.T, parser *PDFParser, ctx context.Context, ocrRunner func(context.Context, string, int) (string, error), onPage func(int)) (map[string]any, error) {
	t.Helper()
	type resultado struct {
		pages map[string]any
		err   error
	}
	done := make(chan resultado, 1)
	go func() {
		pages, err := parser.ParsePages(ctx, true, ocrRunner, onPage)
		done <- resultado{pages, err}
	}()
	select {
	case r := <-done:
		return r.pages, r.err
	case <-time.After(5 * time.Second):
		t.Fatalf("ParsePages não voltou após o cancelamento")
		return nil, nil
	}
}

func gravarPDF(t *testing.T, objects []string) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(caminho, buildPDF(objects), 0644); err != nil {
		t.Fatalf("falha ao gravar pdf de teste: %v", err)
	}
	return caminho
}

// O runner cancela o contexto no primeiro OCR. Como no pipeline, ele segura a
// vaga de OCR (semáforo de 1) e os demais só esperam vaga ou cancelamento:
// nenhuma outra página pode chegar a fazer OCR, e ParsePages tem de voltar com
// context.Canceled sem deixar goroutine para trás.
func TestPDFParser_ParsePages_CancelamentoDuranteOCR(t *testing.T) {
	const n = 6
	caminho := gravarPDF(t, objetosPaginasComImagem(n))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vaga := make(chan struct{}, 1)
	var trabalhos, depoisDoRetorno atomic.Int32
	var retornou atomic.Bool
	ocrRunner := func(ctx context.Context, _ string, _ int) (string, error) {
		if retornou.Load() {
			depoisDoRetorno.Add(1)
		}
		select {
		case vaga <- struct{}{}:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		trabalhos.Add(1)
		cancel()
		return "", ctx.Err()
	}

	_, err := esperarParsePages(t, parserDeArquivo(t, caminho), ctx, ocrRunner, nil)
	retornou.Store(true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtive %v", err)
	}
	if got := trabalhos.Load(); got != 1 {
		t.Errorf("esperava OCR em 1 página antes do cancelamento, obtive %d", got)
	}
	if got := depoisDoRetorno.Load(); got != 0 {
		t.Errorf("runner chamado %d vez(es) depois de ParsePages voltar", got)
	}
	assertNoLeaks(t)
}

// Quando o OCR em bloco falha por cancelamento, refazer página a página só
// gasta processos com um contexto que já morreu.
func TestPDFParser_ParsePages_CancelamentoNoLoteNaoRefazPorPagina(t *testing.T) {
	caminho := gravarPDF(t, objetosPaginasComImagem(4))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var chamadasRunner atomic.Int32
	ocrRunner := func(ctx context.Context, _ string, _ int) (string, error) {
		chamadasRunner.Add(1)
		return "OCR", nil
	}
	lote := func(ctx context.Context, _ string, paginas []int, _ func(int)) ([]string, error) {
		cancel()
		return nil, ctx.Err()
	}

	parser := parserDeArquivo(t, caminho).comOCREmLote(lote, 1)
	_, err := esperarParsePages(t, parser, ctx, ocrRunner, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtive %v", err)
	}
	if got := chamadasRunner.Load(); got != 0 {
		t.Errorf("OCR por página chamado %d vez(es) após o cancelamento do lote", got)
	}
	assertNoLeaks(t)
}

func TestPDFParser_ParsePages_ContextoJaCancelado(t *testing.T) {
	caminho := gravarPDF(t, objetosPaginasComImagem(3))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var chamadas, concluidas atomic.Int32
	ocrRunner := func(ctx context.Context, _ string, _ int) (string, error) {
		chamadas.Add(1)
		return "OCR", nil
	}
	pages, err := esperarParsePages(t, parserDeArquivo(t, caminho), ctx, ocrRunner, func(int) { concluidas.Add(1) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtive %v", err)
	}
	if pages != nil {
		t.Errorf("esperava nenhuma página com o contexto cancelado, obtive %d", len(pages))
	}
	if chamadas.Load() != 0 || concluidas.Load() != 0 {
		t.Errorf("esperava nenhum OCR nem progresso, obtive %d OCR e %d páginas", chamadas.Load(), concluidas.Load())
	}
	assertNoLeaks(t)
}

// A biblioteca de PDF entra em panic com filtro de stream desconhecido. A
// página sem imagem tem de sair vazia (sem OCR); com imagem, ir ao OCR.
func TestPDFParser_ParsePages_FiltroNaoSuportadoNaoEntraEmPanic(t *testing.T) {
	for _, filtro := range []string{"/Foo", "/JBIG2Decode"} {
		t.Run(filtro, func(t *testing.T) {
			stream := fmt.Sprintf("<< /Length 13 /Filter %s >>\nstream\nqualquercoisa\nendstream", filtro)
			semImagem := []string{
				"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
				"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
				"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
				"4 0 obj\n" + stream + "\nendobj",
			}
			runnerProibido := func(context.Context, string, int) (string, error) {
				t.Errorf("OCR não deveria rodar em página sem imagem nem conteúdo legível")
				return "", nil
			}
			pages, err := parsePagesSemPanic(t, semImagem, runnerProibido)
			if err != nil {
				t.Fatalf("ParsePages falhou: %v", err)
			}
			if p := pages["1"].(map[string]any); p["content"].(string) != "" || p["ocr"].(bool) {
				t.Errorf("esperava página vazia sem OCR, obtive %+v", p)
			}

			comImagem := []string{
				semImagem[0], semImagem[1],
				"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
				semImagem[3],
				"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
			}
			runner := func(context.Context, string, int) (string, error) {
				return "OCR Extracted Text", nil
			}
			pages, err = parsePagesSemPanic(t, comImagem, runner)
			if err != nil {
				t.Fatalf("ParsePages falhou: %v", err)
			}
			if p := pages["1"].(map[string]any); !p["ocr"].(bool) || p["content"].(string) != "OCR Extracted Text" {
				t.Errorf("esperava OCR na página com imagem, obtive %+v", p)
			}
		})
	}
}

func TestExtractNativeText_FiltroNaoSuportadoViraErro(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 13 /Filter /Foo >>\nstream\nqualquercoisa\nendstream\nendobj",
	}
	page := primeiraPagina(t, objects)
	if _, err := ExtractNativeText(page); err == nil || !strings.Contains(err.Error(), "pânico") {
		t.Errorf("esperava erro de pânico recuperado, obtive %v", err)
	}
	if HasDrawnContent(page) {
		t.Errorf("filtro ilegível não pode contar como conteúdo desenhado")
	}
}

// primeiraPagina abre o PDF e devolve a página 1 sem passar por ParsePages.
func primeiraPagina(t *testing.T, objects []string) pdf.Page {
	t.Helper()
	dados := buildPDF(objects)
	reader, n, err := abrirLeitorPDF(bytes.NewReader(dados), int64(len(dados)), 0)
	if err != nil {
		t.Fatalf("falha ao abrir pdf de teste: %v", err)
	}
	paginas, _, err := listarPaginas(reader, n)
	if err != nil || len(paginas) == 0 {
		t.Fatalf("falha ao listar páginas: %v", err)
	}
	return paginas[0]
}

// Página vetorial (iLovePDF) cujo /Contents é um array: nenhum stream passa
// sozinho do limiar, mas a soma passa, e a página tem de ir ao OCR.
func TestPDFParser_ParsePages_VetorialComContentsEmArrayUsaOcr(t *testing.T) {
	var paths strings.Builder
	for paths.Len() < drawnContentMinBytes*3/4 {
		paths.WriteString("40.16 196.8 m\n744.32 196.8 l\n744.32 199.04 l\nh\nf\n")
	}
	content := paths.String()
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents [4 0 R 5 0 R] >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(content), content),
		fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj", len(content), content),
	}
	if !HasDrawnContent(primeiraPagina(t, objects)) {
		t.Fatalf("HasDrawnContent ignorou a soma dos streams do array")
	}
	ocrRunner := func(context.Context, string, int) (string, error) {
		return "OCR Extracted Text", nil
	}

	pages := parsePagesWithDeadline(t, objects, ocrRunner)

	if p := pages["1"].(map[string]any); !p["ocr"].(bool) || p["content"].(string) != "OCR Extracted Text" {
		t.Errorf("esperava OCR na página vetorial com /Contents em array, obtive %+v", p)
	}
}

// /Length maior que os dados do arquivo: o stream não pode travar a leitura.
func TestPDFParser_ParsePages_LengthMaiorQueOsDadosNaoTrava(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 999999 >>\nstream\nBT\n/F1 12 Tf\n72 712 Td\n(Hello World) Tj\nET\nendstream\nendobj",
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParsePages entrou em panic: %v", r)
		}
	}()
	pages := parsePagesWithDeadline(t, objects, nil)
	if _, ok := pages["1"].(map[string]any); !ok {
		t.Errorf("esperava a página 1 no resultado, obtive %+v", pages)
	}
}

// Stream Flate truncado: a leitura falha no meio, e o lexer da biblioteca
// engoliria o erro e giraria para sempre. Tem de virar erro de extração.
func TestPDFParser_ParsePages_StreamTruncadoNaoTrava(t *testing.T) {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	for range 200 {
		fmt.Fprintf(w, "BT\n/F1 12 Tf\n72 712 Td\n(Linha %d) Tj\nET\n", z.Len())
	}
	w.Close()
	truncado := z.Bytes()[:z.Len()/2]
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream\nendobj", len(truncado), truncado),
	}
	if _, err := ExtractNativeText(primeiraPagina(t, objects)); err == nil || !strings.Contains(err.Error(), "ilegível") {
		t.Errorf("esperava erro de stream ilegível, obtive %v", err)
	}

	pages := parsePagesWithDeadline(t, objects, nil)
	if p := pages["1"].(map[string]any); p["content"].(string) != "" {
		t.Errorf("esperava página sem texto, obtive %+v", p)
	}
}
