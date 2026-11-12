package etl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"eprocia/internal/models"
)

func parsearSemOCR(t *testing.T, dados []byte, ocr bool) map[string]any {
	t.Helper()
	parser := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf",
		func() (string, error) { return "", fmt.Errorf("sem disco no teste") })
	runner := func(context.Context, string, int) (string, error) { return "", nil }
	if !ocr {
		runner = nil
	}
	pages, err := parser.ParsePages(context.Background(), ocr, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages falhou: %v", err)
	}
	return pages
}

// pdfMisto monta um PDF sintético em que digitalizada[i] diz se a página i+1 é
// uma imagem grande sem texto; as demais trazem um texto curto e distinto.
func pdfMisto(digitalizada []bool) []byte {
	n := len(digitalizada)
	kids := make([]string, n)
	for i := range n {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), n),
		"3 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}
	for i, dig := range digitalizada {
		res, stream := "<< >>", fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(Texto nativo ficticio %d) Tj\nET", i+1)
		if dig {
			res, stream = "<< /XObject << /Im1 3 0 R >> >>", "/Im1 Do"
		}
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources %s /Contents %d 0 R >>\nendobj", 4+2*i, res, 5+2*i),
			fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", 5+2*i, len(stream), stream),
		)
	}
	return buildPDF(objs)
}

func TestSemOCR_PaginaDigitalizada(t *testing.T) {
	pages := parsearSemOCR(t, buildPDF(objetosPaginasComImagem(2)), false)
	for _, k := range []string{"1", "2"} {
		p := pages[k].(map[string]any)
		if so, _ := p["sem_ocr"].(bool); !so {
			t.Errorf("página %s sem a chave sem_ocr: %v", k, p)
		}
		if p["content"] != marcadorPaginaSemOCR {
			t.Errorf("página %s content = %q", k, p["content"])
		}
	}
}

func TestSemOCR_ComOCRLigadoNadaMuda(t *testing.T) {
	pages := parsearSemOCR(t, buildPDF(objetosPaginasComImagem(2)), true)
	for k, v := range pages {
		if _, ok := v.(map[string]any)["sem_ocr"]; ok {
			t.Errorf("página %s ganhou sem_ocr com o OCR ligado", k)
		}
	}
}

func TestSemOCR_PaginaNativaNaoMuda(t *testing.T) {
	pages := parsearSemOCR(t, pdfComTexto("Texto nativo ficticio"), false)
	p := pages["1"].(map[string]any)
	if _, ok := p["sem_ocr"]; ok {
		t.Errorf("página nativa marcada: %v", p)
	}
	if p["content"] != "Texto nativo ficticio" {
		t.Errorf("content = %q", p["content"])
	}
}

func TestSemOCR_PaginaEmBrancoNaoEMarcada(t *testing.T) {
	pages := parsearSemOCR(t, pdfComTexto(""), false)
	p := pages["1"].(map[string]any)
	if _, ok := p["sem_ocr"]; ok {
		t.Errorf("página em branco marcada: %v", p)
	}
	if p["content"] != "" {
		t.Errorf("content = %q, esperava vazio", p["content"])
	}
}

func TestSemOCR_PaginaMistaMantemTextoNativo(t *testing.T) {
	pages := parsearSemOCR(t, pdfMisto([]bool{false, true}), false)
	if p := pages["1"].(map[string]any); p["content"] != "Texto nativo ficticio 1" || p["sem_ocr"] != nil {
		t.Errorf("página 1: %v", p)
	}
	if p := pages["2"].(map[string]any); p["sem_ocr"] != true || p["content"] != marcadorPaginaSemOCR {
		t.Errorf("página 2: %v", p)
	}
}

func TestPipeline_SemOCR_DocumentoNaoLido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfDuasPaginasEscaneadas()}})
	atv := NovaAtividade("x")
	outPath, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if n := atv.NaoLidos(); n != 1 {
		t.Errorf("NaoLidos() = %d, esperava 1", n)
	}
	if l := atv.Instantaneo().Docs[0].Leitura; l != "sem_ocr" {
		t.Errorf("Leitura = %q", l)
	}
	b, _ := os.ReadFile(outPath)
	for _, want := range []string{
		"[documento não lido: digitalizado; o OCR está desativado; consulte o PDF original]",
		"Documentos não lidos: 1",
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf(".txt sem %q:\n%s", want, b)
		}
	}
}

func TestPipeline_SemOCR_DocumentoMisto(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfMisto([]bool{false, true, false, true, true})}})
	atv := NovaAtividade("x")
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "none"}
	outPath, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfg,
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if n := atv.NaoLidos(); n != 0 {
		t.Errorf("NaoLidos() = %d, esperava 0", n)
	}
	b, _ := os.ReadFile(outPath)
	if n := strings.Count(string(b), marcadorPaginaSemOCR); n != 3 {
		t.Errorf("marcador apareceu %d vezes, esperava 3:\n%s", n, b)
	}
}

// pdfTextoCurtoEImagem: uma página com texto nativo curto e imagem grande.
func pdfTextoCurtoEImagem() []byte {
	stream := "BT\n/F1 12 Tf\n72 712 Td\n(Texto nativo curto ficticio) Tj\nET\n/Im1 Do"
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
		"5 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	})
}

func TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfTextoCurtoEImagem()}})
	atv := NovaAtividade("x")
	outPath, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if n := atv.NaoLidos(); n != 0 {
		t.Errorf("NaoLidos() = %d, esperava 0", n)
	}
	if l := atv.Instantaneo().Docs[0].Leitura; l != "lido" {
		t.Errorf("Leitura = %q, esperava lido", l)
	}
	b, _ := os.ReadFile(outPath)
	if !strings.Contains(string(b), "Texto nativo curto ficticio") {
		t.Errorf(".txt perdeu o texto nativo:\n%s", b)
	}
	if strings.Contains(string(b), "[documento não lido") {
		t.Errorf(".txt trata como não lido:\n%s", b)
	}
}

func TestClassificarPaginas_SemOCR(t *testing.T) {
	pg := func(content string) map[string]any {
		return map[string]any{"content": content, "ocr": false, "sem_ocr": true}
	}
	casos := []struct {
		nome    string
		paginas map[string]any
		want    Leitura
	}{
		{"só marcador", map[string]any{"1": pg(marcadorPaginaSemOCR), "2": pg(marcadorPaginaSemOCR)}, LeituraSemOCR},
		{"texto nativo", map[string]any{"1": pg("Texto nativo curto")}, LeituraLido},
		{"ilegível", map[string]any{"1": pg("[texto ilegível na página: 150 caracteres omitidos]")}, LeituraSemOCR},
	}
	for _, c := range casos {
		if got := classificarPaginas(c.paginas, false); got != c.want {
			t.Errorf("%s: %q, esperava %q", c.nome, got, c.want)
		}
	}
}
