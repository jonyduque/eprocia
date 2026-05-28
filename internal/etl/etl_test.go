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
