package bench

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eprocia/internal/etl"
	"eprocia/internal/ocr"
	"eprocia/scripts/bench/pdfgen"
)

// The versioned fixtures must match the generator, or the benchmarks would
// measure something other than what the code describes.
func TestFixtures_UpToDate(t *testing.T) {
	for name, want := range pdfgen.Fixtures() {
		got, err := os.ReadFile(filepath.Join("fixtures", name))
		if err != nil {
			t.Fatalf("%s ausente: rode `go run ./scripts/bench/gerar` (%v)", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s desatualizado: rode `go run ./scripts/bench/gerar`", name)
		}
	}
}

func parse(t *testing.T, name string) (pages map[string]any, ocrPages int) {
	t.Helper()
	runner := func(context.Context, string, int) (string, error) {
		ocrPages++
		return "texto ocr", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pages, err := abrirFixture(t, name).ParsePages(ctx, true, runner, nil)
	if err != nil {
		t.Fatalf("%s: ParsePages: %v", name, err)
	}
	return pages, ocrPages
}

func content(pages map[string]any, n string) string {
	return pages[n].(map[string]any)["content"].(string)
}

// Each PDF fixture must drive the extraction path it is named after.
func TestFixtures_ExtractionPaths(t *testing.T) {
	pages, ocrPages := parse(t, "nativo.pdf")
	if len(pages) != 3 || ocrPages != 0 || !strings.Contains(content(pages, "1"), "PROCESSO FICTICIO") {
		t.Errorf("nativo.pdf: %d páginas, %d OCR, conteúdo %q", len(pages), ocrPages, content(pages, "1"))
	}
	for _, name := range []string{"escaneado.pdf", "vetorial.pdf"} {
		if _, ocrPages := parse(t, name); ocrPages != 1 {
			t.Errorf("%s: esperava OCR em 1 página, teve %d", name, ocrPages)
		}
	}
	if pages, ocrPages := parse(t, "contents-array.pdf"); len(pages) != 1 || ocrPages != 0 {
		t.Errorf("contents-array.pdf: %d páginas, %d OCR", len(pages), ocrPages)
	}
}

// The raster and vector fixtures must be legible to the real OCR engine.
func TestFixtures_LegibleToOCR(t *testing.T) {
	if testing.Short() {
		t.Skip("OCR real é lento")
	}
	for _, name := range []string{"escaneado.pdf", "vetorial.pdf"} {
		path, _ := filepath.Abs(filepath.Join("fixtures", name))
		text, err := ocr.RunOCR(context.Background(), path, 1, "fast")
		if err != nil {
			t.Fatalf("%s: RunOCR: %v", name, err)
		}
		if !strings.Contains(strings.ToLower(text), "processo") && !strings.Contains(strings.ToLower(text), "parte") {
			t.Errorf("%s: OCR ilegível: %q", name, text)
		}
	}
}

// abrirFixture monta o parser como o pipeline: leitor aberto e caminho em
// disco só para o OCR.
func abrirFixture(tb testing.TB, name string) *etl.PDFParser {
	tb.Helper()
	caminho := filepath.Join("fixtures", name)
	f, err := os.Open(caminho)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { f.Close() })
	info, err := f.Stat()
	if err != nil {
		tb.Fatal(err)
	}
	return etl.NewPDFParserDeLeitor(f, info.Size(), caminho, func() (string, error) { return caminho, nil })
}
