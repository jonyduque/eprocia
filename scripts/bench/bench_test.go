// Package bench holds the review harness benchmarks (skill eprocia-bench).
//
//	go test ./scripts/bench -run '^$' -bench . -count=10 -benchmem > antes.txt
//	EPROCIA_BENCH_OCR=fast go test ./scripts/bench -run '^$' -bench Sintetico -count=10 -benchmem
//	EPROCIA_BENCH_DIR=D:\corpus go test ./scripts/bench -run '^$' -bench Real -count=10 -benchmem
//
// Besides ns/op, B/op and allocs/op, each pipeline benchmark reports the peak of
// runtime memory (/memory/classes/total:bytes, sampled), OCR child processes
// and total GC pause per operation.
package bench

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"eprocia/internal/etl"
	"eprocia/internal/models"
	"eprocia/internal/ocr"
	"eprocia/scripts/bench/pdfgen"
)

// sink discards log output but, unlike io.Discard, keeps the formatting cost
// that the real app pays.
type sink struct{}

func (sink) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = sink{}

func TestMain(m *testing.M) {
	log.SetOutput(sink{})
	os.Exit(m.Run())
}

func ocrMode() string {
	if m := os.Getenv("EPROCIA_BENCH_OCR"); m != "" {
		return m
	}
	return "none"
}

// peakSampler samples the runtime's total mapped memory until stopped.
type peakSampler struct {
	peak atomic.Uint64
	stop chan struct{}
	wg   sync.WaitGroup
}

func startPeak() *peakSampler {
	s := &peakSampler{stop: make(chan struct{})}
	sample := []metrics.Sample{{Name: "/memory/classes/total:bytes"}}
	read := func() {
		metrics.Read(sample)
		if v := sample[0].Value.Uint64(); v > s.peak.Load() {
			s.peak.Store(v)
		}
	}
	s.wg.Go(func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			read()
			select {
			case <-s.stop:
				read()
				return
			case <-t.C:
			}
		}
	})
	return s
}

func (s *peakSampler) end() uint64 {
	close(s.stop)
	s.wg.Wait()
	return s.peak.Load()
}

// copyToTemp puts the ZIP in a temp dir, so the .txt output never lands next to
// the source (the repository or the real corpus).
func copyToTemp(b *testing.B, src string) string {
	b.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		// Only the cause: the path may hold a real process name (EPROCIA_BENCH_DIR).
		if pe, ok := errors.AsType[*fs.PathError](err); ok {
			err = pe.Err
		}
		b.Fatalf("falha ao ler o ZIP de entrada: %v", err)
	}
	dst := filepath.Join(b.TempDir(), "processo.zip")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		b.Fatal(err)
	}
	return dst
}

func benchZip(b *testing.B, src string) {
	zipPath := copyToTemp(b, src)
	cfg := &models.Config{OcrMode: ocrMode(), CleaningLevel: "aggressive"}
	pipe := etl.NewPipeline()

	var gcBefore, gcAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&gcBefore)
	procsBefore := ocr.SpawnedProcesses()
	peak := startPeak()

	b.ResetTimer()
	for b.Loop() {
		if _, _, _, err := pipe.ProcessZip(context.Background(), zipPath, cfg, nil); err != nil {
			b.Fatalf("ProcessZip: %v", err)
		}
	}
	b.StopTimer()

	peakBytes := peak.end()
	runtime.ReadMemStats(&gcAfter)
	n := float64(b.N)
	b.ReportMetric(float64(peakBytes), "pico-mem-B")
	b.ReportMetric(float64(ocr.SpawnedProcesses()-procsBefore)/n, "procs/op")
	b.ReportMetric(float64(gcAfter.PauseTotalNs-gcBefore.PauseTotalNs)/n, "gc-pausa-ns/op")
}

// BenchmarkSintetico runs the whole pipeline on the versioned mixed ZIP.
func BenchmarkSintetico(b *testing.B) {
	b.Run("ocr="+ocrMode(), func(b *testing.B) {
		benchZip(b, filepath.Join("fixtures", "misto.zip"))
	})
	// Um documento escaneado longo, como as petições digitalizadas reais: é o
	// caso em que o OCR agrupa páginas por processo (B-EF-04).
	b.Run(fmt.Sprintf("escaneado%d/ocr=%s", paginasEscaneado, ocrMode()), func(b *testing.B) {
		if ocrMode() == "none" {
			b.Skip("sem OCR o documento escaneado só mede a extração nativa")
		}
		benchZip(b, zipEscaneado(b, paginasEscaneado))
	})
}

const paginasEscaneado = 24

// zipEscaneado grava em um diretório temporário um ZIP com um único PDF de n
// páginas escaneadas.
func zipEscaneado(b *testing.B, n int) string {
	b.Helper()
	pages := make([]pdfgen.Page, n)
	for i := range pages {
		pages[i] = pdfgen.ScannedPage(pdfgen.Text(12, i))
	}
	data := pdfgen.Zip([]pdfgen.Entry{{Name: "Evento 1 - INIC1.pdf", Data: pdfgen.Document(pages)}})
	path := filepath.Join(b.TempDir(), "escaneado.zip")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.Fatal(err)
	}
	return path
}

// BenchmarkParsePDF isolates the Go-side cost of each extraction path (no OCR).
func BenchmarkParsePDF(b *testing.B) {
	for _, name := range []string{"nativo.pdf", "escaneado.pdf", "vetorial.pdf", "contents-array.pdf"} {
		b.Run(strings.TrimSuffix(name, ".pdf"), func(b *testing.B) {
			parser := abrirFixture(b, name)
			for b.Loop() {
				if _, err := parser.ParsePages(context.Background(), false, nil, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkReal runs every ZIP in EPROCIA_BENCH_DIR. Sub-benchmarks are named
// zip01, zip02... (sorted by file name) so real process names never appear in
// benchmark output that may be pasted into the report.
func BenchmarkReal(b *testing.B) {
	dir := os.Getenv("EPROCIA_BENCH_DIR")
	if dir == "" {
		b.Skip("EPROCIA_BENCH_DIR não definido; corpus real pulado")
	}
	zips, _ := filepath.Glob(filepath.Join(dir, "*.zip"))
	if len(zips) == 0 {
		b.Skipf("nenhum .zip em EPROCIA_BENCH_DIR")
	}
	sort.Strings(zips)
	for i, z := range zips {
		b.Run(fmt.Sprintf("zip%02d/ocr=%s", i+1, ocrMode()), func(b *testing.B) {
			benchZip(b, z)
		})
	}
}
