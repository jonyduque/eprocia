package etl

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"eprocia/internal/models"

	"github.com/dslipak/pdf"
)

func TestProcessZip_CanceladoNaoPreCalculaPaginas(t *testing.T) {
	tmpTemp := t.TempDir()
	t.Setenv("TMP", tmpTemp)
	t.Setenv("TEMP", tmpTemp)
	t.Setenv("TMPDIR", tmpTemp)

	zipDir := t.TempDir()
	zipPath := filepath.Join(zipDir, "processo.zip")
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for i := 1; i <= 5; i++ {
		w, err := zw.Create(fmt.Sprintf("Evento %d - DOC%d.pdf", i, i))
		if err != nil {
			t.Fatalf("falha ao criar entrada: %v", err)
		}
		_, _ = w.Write([]byte("%PDF-1.4 conteudo ficticio"))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao gravar zip: %v", err)
	}

	var logBuf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&logBuf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewPipeline()
	cfg := &models.Config{CleaningLevel: "none", OcrMode: "fast"}
	_, _, _, _, err := p.ProcessZip(ctx, zipPath, cfg, func(float64, string, int, int, int, int) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtive %v", err)
	}

	saida := logBuf.String()
	if strings.Contains(saida, "pré-calcular") || strings.Contains(saida, "Pré-cálculo concluído") {
		t.Errorf("pré-cálculo rodou mesmo com o contexto cancelado:\n%s", saida)
	}

	restantes, _ := filepath.Glob(filepath.Join(tmpTemp, "eprocia_pdf_*"))
	if len(restantes) != 0 {
		t.Errorf("temporários não removidos: %v", restantes)
	}
	if _, err := os.Stat(strings.TrimSuffix(zipPath, ".zip") + ".txt"); err == nil {
		t.Errorf("arquivo de saída não deveria ser gravado após cancelamento")
	}
}

func TestProcessZip_PanicNoPreCalculoNaoVazaTemporarios(t *testing.T) {
	tmpTemp := t.TempDir()
	t.Setenv("TMP", tmpTemp)
	t.Setenv("TEMP", tmpTemp)
	t.Setenv("TMPDIR", tmpTemp)

	zipDir := t.TempDir()
	zipPath := filepath.Join(zipDir, "processo.zip")
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for i := 1; i <= 3; i++ {
		w, err := zw.Create(fmt.Sprintf("Evento %d - DOC%d.pdf", i, i))
		if err != nil {
			t.Fatalf("falha ao criar entrada: %v", err)
		}
		_, _ = w.Write([]byte("%PDF-1.4 conteudo ficticio"))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao gravar zip: %v", err)
	}

	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(r io.ReaderAt, size int64) (*pdf.Reader, int, error) {
		panic("pdf corrompido")
	}

	p := NewPipeline()
	cfg := &models.Config{CleaningLevel: "none", OcrMode: "none"}
	_, _, _, _, err := p.ProcessZip(context.Background(), zipPath, cfg, func(float64, string, int, int, int, int) {})
	if err != nil {
		t.Fatalf("panic no pré-cálculo deveria cair no fallback, obtive erro: %v", err)
	}

	restantes, _ := filepath.Glob(filepath.Join(tmpTemp, "eprocia_pdf_*"))
	if len(restantes) != 0 {
		t.Errorf("temporários não removidos: %v", restantes)
	}
}

// A PDF whose xref points page 1 to an object with another number makes the PDF
// library panic inside the per-document goroutine; before the fix that killed
// the whole app instead of marking the page.
func TestProcessZip_PDFComXrefInconsistenteNaoDerrubaApp(t *testing.T) {
	tmpTemp := t.TempDir()
	t.Setenv("TMP", tmpTemp)
	t.Setenv("TEMP", tmpTemp)
	t.Setenv("TMPDIR", tmpTemp)

	pdfBytes := buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"9 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> >>\nendobj",
	})

	zipPath := filepath.Join(t.TempDir(), "processo.zip")
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, err := zw.Create("Evento 1 - DOC1.pdf")
	if err != nil {
		t.Fatalf("falha ao criar entrada: %v", err)
	}
	_, _ = w.Write(pdfBytes)
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao gravar zip: %v", err)
	}

	p := NewPipeline()
	cfg := &models.Config{CleaningLevel: "none", OcrMode: "none"}
	outPath, _, _, _, err := p.ProcessZip(context.Background(), zipPath, cfg, func(float64, string, int, int, int, int) {})
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	saida, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	if !strings.Contains(string(saida), "[ERRO") {
		t.Errorf("esperava a página marcada com [ERRO ...] na saída:\n%s", saida)
	}
}

// Os eventos process:* não tinham ouvinte no frontend; a fila já leva o
// progresso. Com um contexto sem runtime Wails, qualquer EventsEmit encerra
// o processo via log.Fatalf, por isso o teste roda num subprocesso.
func TestProcessZip_NaoEmiteEventosWails(t *testing.T) {
	if os.Getenv("EPROCIA_SUBPROCESSO_EVENTOS") == "1" {
		zipPath := filepath.Join(t.TempDir(), "processo.zip")
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		w, _ := zw.Create("Evento 1 - DOC1.html")
		_, _ = w.Write([]byte("<html><body><p>Texto ficticio do documento.</p></body></html>"))
		_ = zw.Close()
		if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		p := NewPipeline()
		cfg := &models.Config{CleaningLevel: "none", OcrMode: "none"}
		if _, _, _, _, err := p.ProcessZip(context.Background(), zipPath, cfg, func(float64, string, int, int, int, int) {}); err != nil {
			t.Fatal(err)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessZip_NaoEmiteEventosWails$")
	cmd.Env = append(os.Environ(), "EPROCIA_SUBPROCESSO_EVENTOS=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ProcessZip tentou emitir evento Wails: %v\n%s", err, out)
	}
}
