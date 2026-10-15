package etl

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"eprocia/internal/models"
)

func TestCompareNatural(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool // true if a < b
	}{
		{"Evento 2", "Evento 10", true},
		{"Evento 10", "Evento 2", false},
		{"Evento 1 - DOC2", "Evento 1 - DOC10", true},
		{"Evento 1 - DOC10", "Evento 1 - DOC2", false},
		{"Evento 1 - DOC2.pdf", "Evento 1 - DOC2.pdf", false},
		{"a", "b", true},
		{"1", "2", true},
		{"10", "2", false},
		{"evento 1", "Evento 2", true}, // Case-insensitivity check
	}

	for _, tt := range tests {
		actual := CompareNatural(tt.a, tt.b)
		if actual != tt.expected {
			t.Errorf("CompareNatural(%q, %q) = %t; expected %t", tt.a, tt.b, actual, tt.expected)
		}
	}
}

func TestSortProcessFiles(t *testing.T) {
	files := []models.ProcessFile{
		{DocName: "Evento 10 - SENTENCA.pdf", EventNum: 10},
		{DocName: "Evento 2 - DESPACHO.pdf", EventNum: 2},
		{DocName: "Outro arquivo.pdf", EventNum: 999999},
		{DocName: "Evento 1 - INIC1.pdf", EventNum: 1},
		{DocName: "Evento 1 - PROC2.pdf", EventNum: 1},
	}

	SortProcessFiles(files)

	expectedOrder := []string{
		"Evento 1 - INIC1.pdf",
		"Evento 1 - PROC2.pdf",
		"Evento 2 - DESPACHO.pdf",
		"Evento 10 - SENTENCA.pdf",
		"Outro arquivo.pdf",
	}

	for i, f := range files {
		if f.DocName != expectedOrder[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expectedOrder[i], f.DocName)
		}
	}
}

func TestReadZipFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "etl_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "processo.zip")

	// Create a mock zip archive
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	filesToAdd := []struct {
		name, content string
	}{
		{"Evento 2 - DESPACHO.pdf", "pdf content"},
		{"Evento 1 - INIC1.html", "html content"},
		{".DS_Store", "should ignore"},
		{"__MACOSX/foo.pdf", "should ignore"},
		{"unsupported.txt", "should ignore"},
	}

	for _, file := range filesToAdd {
		f, err := zipWriter.Create(file.name)
		if err != nil {
			t.Fatalf("failed to create file in zip: %v", err)
		}
		_, _ = f.Write([]byte(file.content))
	}
	_ = zipWriter.Close()

	err = os.WriteFile(zipPath, buf.Bytes(), 0644)
	if err != nil {
		t.Fatalf("failed to write zip file: %v", err)
	}

	processFiles, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		t.Fatalf("ReadZipFiles failed: %v", err)
	}
	defer rc.Close()

	if len(processFiles) != 2 {
		t.Fatalf("expected 2 files extracted, got %d", len(processFiles))
	}

	SortProcessFiles(processFiles)

	if processFiles[0].DocName != "Evento 1 - INIC1.html" {
		t.Errorf("expected index 0 to be Evento 1 - INIC1.html, got %s", processFiles[0].DocName)
	}
	if processFiles[1].DocName != "Evento 2 - DESPACHO.pdf" {
		t.Errorf("expected index 1 to be Evento 2 - DESPACHO.pdf, got %s", processFiles[1].DocName)
	}
}

// zipFileNamed builds an in-memory zip entry so ParseFileInfo can be exercised.
func zipFileNamed(t *testing.T, name string) *zip.File {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	if _, err := zw.Create(name); err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to reopen zip: %v", err)
	}
	return zr.File[0]
}

func TestParseFileInfo_DocNumAndCover(t *testing.T) {
	tests := []struct {
		name       string
		wantEvent  int
		wantDocNum int
	}{
		{"Evento 1 - INIC1.pdf", 1, 1},
		{"Evento 12 - PROC_VIRTUAL_3.pdf", 12, 3},
		{"Evento 7 - SENTENCA10.html", 7, 10},
		{"Capa do processo.pdf", 0, 0},
		{"pasta/Capa do processo.pdf", 0, 0},
		{"Evento 5 - DESPACHO.pdf", 5, noDocNum},
		{"Outro arquivo.pdf", noEventNum, noDocNum},
	}

	for _, tt := range tests {
		got := ParseFileInfo(zipFileNamed(t, tt.name))
		if got.EventNum != tt.wantEvent || got.DocNum != tt.wantDocNum {
			t.Errorf("ParseFileInfo(%q) = event %d, doc %d; want event %d, doc %d",
				tt.name, got.EventNum, got.DocNum, tt.wantEvent, tt.wantDocNum)
		}
	}
}

func TestSortProcessFiles_OrdersByDocumentNumberWithinEvent(t *testing.T) {
	names := []string{
		"Evento 1 - COMP3.pdf",
		"Outro arquivo.pdf",
		"Evento 1 - PROC2.pdf",
		"Evento 2 - DESPACHO4.pdf",
		"Capa do processo.pdf",
		"Evento 1 - INIC1.pdf",
	}
	var files []models.ProcessFile
	for _, n := range names {
		files = append(files, ParseFileInfo(zipFileNamed(t, n)))
	}

	SortProcessFiles(files)

	want := []string{
		"Capa do processo.pdf",
		"Evento 1 - INIC1.pdf",
		"Evento 1 - PROC2.pdf",
		"Evento 1 - COMP3.pdf",
		"Evento 2 - DESPACHO4.pdf",
		"Outro arquivo.pdf",
	}
	for i, f := range files {
		if f.DocName != want[i] {
			t.Errorf("at index %d: expected %s, got %s", i, want[i], f.DocName)
		}
	}
}
