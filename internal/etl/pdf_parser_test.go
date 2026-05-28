package etl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	pages, err := parser.ParsePages(true, nil)
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	page := pages[0]
	if page.PageNum != 1 {
		t.Errorf("expected page number 1, got %d", page.PageNum)
	}

	if page.IsOcr {
		t.Errorf("expected IsOcr to be false for native text page")
	}

	if !strings.Contains(page.Text, "Hello") || !strings.Contains(page.Text, "World") {
		t.Errorf("expected extracted text to contain 'Hello World', got %q", page.Text)
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
	ocrRunner := func(path string, pageNum int) (string, error) {
		ocrCalled = true
		return "OCR Extracted Text", nil
	}

	pages, err := parser.ParsePages(true, ocrRunner)
	if err != nil {
		t.Fatalf("ParsePages failed: %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	page := pages[0]
	if !page.IsOcr {
		t.Errorf("expected IsOcr to be true")
	}

	if !ocrCalled {
		t.Errorf("expected OCR runner to be called")
	}

	if page.Text != "OCR Extracted Text" {
		t.Errorf("expected page text to be 'OCR Extracted Text', got %q", page.Text)
	}
}
