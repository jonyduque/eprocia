package etl

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"eprocia/internal/models"
)

type entradaZip struct {
	nome     string
	conteudo []byte
}

// gravarZip grava um ZIP com as entradas na ordem dada (fora da ordem final,
// para que o teste veja a ordenação do pipeline).
func gravarZip(t *testing.T, entradas []entradaZip) string {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, e := range entradas {
		w, err := zw.Create(e.nome)
		if err != nil {
			t.Fatalf("falha ao criar entrada %s: %v", e.nome, err)
		}
		if _, err := w.Write(e.conteudo); err != nil {
			t.Fatalf("falha ao escrever entrada %s: %v", e.nome, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "processo.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao gravar zip: %v", err)
	}
	return zipPath
}

// isolarTemp aponta o diretório temporário do processo para um diretório do
// teste, para conferir que nenhum eprocia_pdf_* sobra.
func isolarTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Setenv("TMPDIR", dir)
	return dir
}

func semTemporarios(t *testing.T, dir string) {
	t.Helper()
	restantes, _ := filepath.Glob(filepath.Join(dir, "eprocia_pdf_*"))
	if len(restantes) != 0 {
		t.Errorf("temporários não removidos: %v", restantes)
	}
}

func saidaDe(zipPath string) string {
	return strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".txt"
}

// pdfComTexto monta um PDF de texto nativo com uma página por texto.
func pdfComTexto(textos ...string) []byte {
	kids := make([]string, len(textos))
	objs := []string{"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj", ""}
	for i, txt := range textos {
		pagina := 3 + 2*i
		conteudo := pagina + 1
		kids[i] = fmt.Sprintf("%d 0 R", pagina)
		stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", txt)
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents %d 0 R >>\nendobj", pagina, conteudo),
			fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", conteudo, len(stream), stream),
		)
	}
	objs[1] = fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), len(textos))
	return buildPDF(objs)
}

func cfgSemOCR() *models.Config {
	return &models.Config{CleaningLevel: "none", OcrMode: "none"}
}

func TestProcessZip_SaidaOrdenadaEContagens(t *testing.T) {
	tmp := isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 10 - DESPADEC1.pdf", pdfComTexto("Despacho do evento dez")},
		{"Evento 2 - PET2.html", []byte("<html><body><p>Peticao segunda do evento dois</p></body></html>")},
		{"Evento 2 - INIC1.pdf", pdfComTexto("Inicial pagina um", "Inicial pagina dois")},
		{"leiame.txt", []byte("ignorado")},
	})

	// onProgress é chamado por vários goroutines; guarda o maior estado visto.
	var mu sync.Mutex
	var maior [4]int
	outPath, docs, pages, tokens, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(),
		func(_ float64, _ string, pd, td, pp, tp int) {
			mu.Lock()
			defer mu.Unlock()
			if pd > maior[0] || (pd == maior[0] && pp > maior[2]) {
				maior = [4]int{pd, td, pp, tp}
			}
		})
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if outPath != saidaDe(zipPath) {
		t.Errorf("caminho de saída = %q, esperava %q", outPath, saidaDe(zipPath))
	}
	if docs != 3 || pages != 4 {
		t.Errorf("contagens = %d docs, %d págs; esperava 3 docs, 4 págs", docs, pages)
	}
	if maior != [4]int{3, 3, 4, 4} {
		t.Errorf("progresso final = %v, esperava [3 3 4 4]", maior)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	saida := string(b)
	if want := EstimateTokens(saida); tokens != want || tokens == 0 {
		t.Errorf("tokens estimados = %d, esperava %d (estimativa do .txt gravado)", tokens, want)
	}
	if want := "Tokens estimados: " + strconv.Itoa(tokens) + "\n"; !strings.Contains(saida, want) {
		t.Errorf("o .txt gravado deveria trazer %q, o mesmo número devolvido:\n%s", want, saida)
	}

	if !strings.Contains(saida, "Total de Documentos: 3") {
		t.Errorf("cabeçalho sem o total de documentos:\n%s", saida)
	}
	if strings.Contains(saida, "leiame") || strings.Contains(saida, "ignorado") {
		t.Errorf("entrada .txt não deveria entrar na saída:\n%s", saida)
	}

	tags := regexp.MustCompile(`<evento num="\d+">|<documento index="\d+" nome="[^"]*">|<pag num="\d+">`).FindAllString(saida, -1)
	esperado := []string{
		`<evento num="2">`,
		`<documento index="1" nome="Evento 2 - INIC1.pdf">`,
		`<pag num="1">`,
		`<pag num="2">`,
		`<documento index="2" nome="Evento 2 - PET2.html">`,
		`<pag num="1">`,
		`<evento num="10">`,
		`<documento index="1" nome="Evento 10 - DESPADEC1.pdf">`,
		`<pag num="1">`,
	}
	if strings.Join(tags, "\n") != strings.Join(esperado, "\n") {
		t.Errorf("tags fora da ordem esperada:\nobtido:\n%s\nesperado:\n%s", strings.Join(tags, "\n"), strings.Join(esperado, "\n"))
	}

	ordem := []string{"Inicial pagina um", "Inicial pagina dois", "Peticao segunda do evento dois", "Despacho do evento dez"}
	pos := -1
	for _, trecho := range ordem {
		i := strings.Index(saida, trecho)
		if i < 0 {
			t.Fatalf("texto %q ausente da saída:\n%s", trecho, saida)
		}
		if i < pos {
			t.Errorf("texto %q fora de ordem", trecho)
		}
		pos = i
	}
	semTemporarios(t, tmp)
}

// O ZIP real do benchmark: o número de páginas devolvido precisa bater com as
// tags <pag> gravadas, e os documentos com as tags <documento>.
func TestProcessZip_FixtureMistoContagensBatemComSaida(t *testing.T) {
	tmp := isolarTemp(t)
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bench", "fixtures", "misto.zip"))
	if err != nil {
		t.Fatalf("falha ao ler fixture: %v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "misto.zip")
	if err := os.WriteFile(zipPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	outPath, docs, pages, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	saida := string(b)

	if docs != 50 {
		t.Errorf("docs = %d, esperava 50", docs)
	}
	if n := strings.Count(saida, "<documento "); n != docs {
		t.Errorf("%d tags <documento> na saída, ProcessZip devolveu %d", n, docs)
	}
	if n := strings.Count(saida, "<pag "); n != pages {
		t.Errorf("%d tags <pag> na saída, ProcessZip devolveu %d", n, pages)
	}
	if n := strings.Count(saida, "<evento "); n != 25 {
		t.Errorf("%d tags <evento> na saída, esperava 25", n)
	}
	if strings.Contains(saida, "[ERRO") {
		t.Errorf("fixture válida não deveria gerar página de erro")
	}
	semTemporarios(t, tmp)
}

func TestProcessZip_ZipSemPDFNemHTMLDevolveErro(t *testing.T) {
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - NOTA1.txt", []byte("texto")},
		{"__MACOSX/Evento 1 - DOC1.pdf", pdfComTexto("lixo do mac")},
	})
	outPath, docs, pages, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil)
	if err == nil {
		t.Fatal("esperava erro para ZIP sem PDF nem HTML")
	}
	if outPath != "" || docs != 0 || pages != 0 {
		t.Errorf("retorno com erro deveria ser vazio, obtive %q %d %d", outPath, docs, pages)
	}
	if _, err := os.Stat(saidaDe(zipPath)); err == nil {
		t.Error("não deveria gravar saída para ZIP sem documentos")
	}
}

func TestProcessZip_ZipIlegivelDevolveErro(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "processo.zip")
	if err := os.WriteFile(zipPath, []byte("isto nao e um zip"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil); err == nil {
		t.Fatal("esperava erro para ZIP ilegível")
	}
	if _, err := os.Stat(saidaDe(zipPath)); err == nil {
		t.Error("não deveria gravar saída para ZIP ilegível")
	}

	ausente := filepath.Join(t.TempDir(), "nao-existe.zip")
	if _, _, _, _, err := NewPipeline().ProcessZip(context.Background(), ausente, cfgSemOCR(), nil); err == nil {
		t.Fatal("esperava erro para ZIP inexistente")
	}
}

func TestProcessZip_PDFTruncadoViraPlaceholderSemAbortarOsDemais(t *testing.T) {
	tmp := isolarTemp(t)
	inteiro := pdfComTexto("Documento integro do evento um")
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", inteiro},
		{"Evento 2 - DOC1.pdf", pdfComTexto("Texto que nao chega")[:40]},
		{"Evento 3 - CERT1.html", []byte("<html><body>Certidao do evento tres</body></html>")},
	})

	outPath, docs, pages, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil)
	if err != nil {
		t.Fatalf("PDF truncado não deveria abortar o processamento: %v", err)
	}
	if docs != 3 || pages != 3 {
		t.Errorf("contagens = %d docs, %d págs; esperava 3 e 3", docs, pages)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}
	saida := string(b)

	ev2 := saida[strings.Index(saida, `<evento num="2">`):]
	ev2 = ev2[:strings.Index(ev2, "</evento>")]
	if !strings.Contains(ev2, "[ERRO ao analisar PDF:") {
		t.Errorf("evento 2 deveria trazer o placeholder de erro:\n%s", ev2)
	}
	for _, trecho := range []string{"Documento integro do evento um", "Certidao do evento tres"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("documento válido %q sumiu da saída:\n%s", trecho, saida)
		}
	}
	if strings.Count(saida, "[ERRO") != 1 {
		t.Errorf("só o PDF truncado deveria virar erro:\n%s", saida)
	}
	semTemporarios(t, tmp)
}

// Cancelar no meio, depois de algum documento já ter sido processado, não
// pode gravar saída parcial nem deixar PDFs pré-extraídos no TEMP.
func TestProcessZip_CanceladoNoMeioNaoGravaSaidaNemDeixaTemporarios(t *testing.T) {
	tmp := isolarTemp(t)
	var entradas []entradaZip
	for i := 1; i <= 12; i++ {
		entradas = append(entradas, entradaZip{
			fmt.Sprintf("Evento %d - DOC1.pdf", i),
			pdfComTexto(fmt.Sprintf("Pagina do evento %d", i), "Segunda pagina"),
		})
	}
	zipPath := gravarZip(t, entradas)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var chamadas atomic.Int32
	outPath, docs, pages, _, err := NewPipeline().ProcessZip(ctx, zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {
			chamadas.Add(1)
			cancel()
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, obtive %v", err)
	}
	if chamadas.Load() == 0 {
		t.Fatal("o cancelamento deveria ocorrer durante o processamento dos documentos")
	}
	if outPath != "" || docs != 0 || pages != 0 {
		t.Errorf("retorno cancelado deveria ser vazio, obtive %q %d %d", outPath, docs, pages)
	}
	if _, err := os.Stat(saidaDe(zipPath)); err == nil {
		t.Error("arquivo de saída não deveria ser gravado após cancelamento")
	}
	semTemporarios(t, tmp)
}

func TestProcessZip_FalhaAoGravarSaidaDevolveErro(t *testing.T) {
	tmp := isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto("Conteudo")}})
	// Um diretório com o nome da saída faz os.WriteFile falhar.
	if err := os.Mkdir(saidaDe(zipPath), 0755); err != nil {
		t.Fatal(err)
	}

	outPath, docs, pages, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgSemOCR(), nil)
	if err == nil {
		t.Fatal("esperava erro quando a saída não pode ser gravada")
	}
	if outPath != "" || docs != 0 || pages != 0 {
		t.Errorf("retorno com erro deveria ser vazio, obtive %q %d %d", outPath, docs, pages)
	}
	semTemporarios(t, tmp)
}
