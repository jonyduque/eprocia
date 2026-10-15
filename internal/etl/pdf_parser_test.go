package etl

import (
	"bytes"
	"context"
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

	parser := NewPDFParser(pdfPath)
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

func TestPDFParser_ParsePages_OcrFallback(t *testing.T) {
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

	parser := NewPDFParser(pdfPath)

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
	go func() {
		pages, err := NewPDFParser(pdfPath).ParsePages(context.Background(), true, ocrRunner, nil)
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
