package etl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Sem OCR, um PDF de texto nativo não precisa de arquivo temporário: com o
// TEMP apontando para um diretório inexistente, qualquer os.CreateTemp falha e
// o documento viraria placeholder de erro.
func TestProcessZip_SemOCRNaoGravaPDFTemporario(t *testing.T) {
	inexistente := filepath.Join(t.TempDir(), "nao-existe")
	t.Setenv("TMP", inexistente)
	t.Setenv("TEMP", inexistente)
	t.Setenv("TMPDIR", inexistente)

	var entradas []entradaZip
	for i := 1; i <= 6; i++ {
		entradas = append(entradas, entradaZip{
			fmt.Sprintf("Evento %d - DOC1.pdf", i),
			pdfComTexto(fmt.Sprintf("Texto do evento %d", i), "Segunda pagina"),
		})
	}
	zipPath := gravarZip(t, entradas)

	outPath, docs, pages, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if docs != 6 || pages != 12 {
		t.Errorf("contagens = %d docs, %d págs; esperava 6 e 12", docs, pages)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	saida := string(b)
	if strings.Contains(saida, "[ERRO") {
		t.Errorf("PDF sem OCR não deveria depender de temporário:\n%s", saida)
	}
	for i := 1; i <= 6; i++ {
		if !strings.Contains(saida, fmt.Sprintf("Texto do evento %d", i)) {
			t.Errorf("texto do evento %d ausente da saída", i)
		}
	}
}

// Reter em memória todos os PDFs (e seus leitores) entre a pré-contagem e o
// processamento elevava o pico de memória do benchmark em 17%. Ao fim da
// pré-contagem nenhum PDF em memória pode continuar reservado.
func TestProcessZip_PreContagemNaoRetemPDFsEmMemoria(t *testing.T) {
	var entradas []entradaZip
	for i := 1; i <= 8; i++ {
		entradas = append(entradas, entradaZip{
			fmt.Sprintf("Evento %d - DOC1.pdf", i),
			pdfComTexto(fmt.Sprintf("Texto do evento %d", i)),
		})
	}
	zipPath := gravarZip(t, entradas)

	var retido int64 = -1
	t.Cleanup(func() { aoFimDaPreContagem = nil })
	aoFimDaPreContagem = func(mem *memoriaPDF) {
		retido = maxTotalPDFEmMemoria - mem.disponivel.Load()
	}

	if _, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil); err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if retido != 0 {
		t.Errorf("%d bytes de PDF continuaram em memória após a pré-contagem; esperava 0", retido)
	}
}

func pdfDuasPaginasEscaneadas() []byte {
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 12 >>\nstream\n/Im1 Do\nendstream\nendobj",
		"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
		"6 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
	})
}

// O PDF lido da memória só vai a disco quando alguma página precisa de OCR, e
// uma única vez por documento, por mais páginas que o OCR leia.
func TestPDFParser_DeLeitor_MaterializaUmaVezSoComOCR(t *testing.T) {
	dados := pdfDuasPaginasEscaneadas()
	caminho := filepath.Join(t.TempDir(), "doc.pdf")

	var materializacoes atomic.Int32
	materializar := func() (string, error) {
		materializacoes.Add(1)
		return caminho, os.WriteFile(caminho, dados, 0644)
	}
	var mu sync.Mutex
	vistos := map[string]int{}
	runner := func(_ context.Context, path string, pageNum int) (string, string, error) {
		mu.Lock()
		vistos[path]++
		mu.Unlock()
		return fmt.Sprintf("OCR pagina %d", pageNum), "", nil
	}

	parser := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf", materializar)
	pages, err := parser.ParsePages(context.Background(), true, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	if n := materializacoes.Load(); n != 1 {
		t.Errorf("materializou %d vezes, esperava 1", n)
	}
	if vistos[caminho] != 2 || len(vistos) != 1 {
		t.Errorf("OCR deveria ler as 2 páginas do arquivo materializado, viu %v", vistos)
	}
	if c := pages["2"].(map[string]any)["content"]; c != "OCR pagina 2" {
		t.Errorf("página 2 = %q", c)
	}

	materializacoes.Store(0)
	parser = NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf", materializar)
	if _, err := parser.ParsePages(context.Background(), false, runner, nil); err != nil {
		t.Fatalf("ParsePages sem OCR falhou: %v", err)
	}
	if n := materializacoes.Load(); n != 0 {
		t.Errorf("sem OCR não deveria materializar, materializou %d vezes", n)
	}
}

// Falha ao materializar vira falha de OCR da página, com o texto nativo (vazio
// aqui) substituído pelo marcador de erro, sem abortar o documento.
func TestPDFParser_DeLeitor_FalhaAoMaterializarMarcaPagina(t *testing.T) {
	dados := pdfDuasPaginasEscaneadas()
	materializar := func() (string, error) { return "", fmt.Errorf("disco cheio") }
	runner := func(context.Context, string, int) (string, string, error) {
		t.Error("OCR não deveria rodar sem arquivo")
		return "", "", nil
	}
	pages, err := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf", materializar).
		ParsePages(context.Background(), true, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	for _, k := range []string{"1", "2"} {
		c := pages[k].(map[string]any)["content"].(string)
		if !strings.Contains(c, "disco cheio") {
			t.Errorf("página %s deveria trazer o erro de materialização, veio %q", k, c)
		}
	}
}

// Com OCR real, o PDF escaneado em memória é gravado em disco para o
// pdftoppm, lido, e o temporário some ao final.
func TestProcessZip_OCRRealMaterializaERemoveTemporario(t *testing.T) {
	if testing.Short() {
		t.Skip("OCR real é lento")
	}
	escaneado, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bench", "fixtures", "escaneado.pdf"))
	if err != nil {
		t.Fatalf("falha ao ler fixture: %v", err)
	}
	tmp := isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", escaneado}})

	cfg := cfgSemOCR()
	cfg.OcrMode = "fast"
	outPath, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg, nil)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	saida := strings.ToLower(string(b))
	if strings.Contains(saida, "[erro") || (!strings.Contains(saida, "processo") && !strings.Contains(saida, "parte")) {
		t.Errorf("OCR não leu o PDF materializado:\n%s", b)
	}
	semTemporarios(t, tmp)
}

// Sem orçamento de memória, o PDF vai para um temporário mantido aberto: a
// contagem lê pelo mesmo handle, o OCR usa esse caminho e liberar o remove.
func TestExtrairPDF_SemOrcamentoUsaTemporarioAberto(t *testing.T) {
	tmp := isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto("Pagina um", "Pagina dois")}})
	files, rc, err := ReadZipFiles(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	var mem memoriaPDF
	fonte, err := extrairPDF(files[0], &mem)
	if err != nil {
		t.Fatalf("extrairPDF falhou: %v", err)
	}
	if fonte.arq == nil || fonte.dados != nil {
		t.Fatalf("sem orçamento o PDF deveria ir para disco")
	}
	if n, err := fonte.contarPaginas(); err != nil || n != 2 {
		t.Errorf("contarPaginas = %d, %v; esperava 2", n, err)
	}
	caminho, err := fonte.materializar()
	if err != nil || caminho != fonte.caminho {
		t.Errorf("materializar = %q, %v; esperava o temporário já gravado %q", caminho, err, fonte.caminho)
	}
	fonte.liberar()
	semTemporarios(t, tmp)
	if mem.disponivel.Load() != 0 {
		t.Errorf("orçamento alterado para %d", mem.disponivel.Load())
	}
}
