package etl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"eprocia/internal/models"
	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
)

// errPopplerTeste é o que o segundo leitor devolve nos testes deste pacote que
// não o trocam: nenhum teste de internal/etl roda o Poppler real.
var errPopplerTeste = errors.New("segundo leitor desligado nos testes do pacote etl")

func TestMain(m *testing.M) {
	infoPDF = func(context.Context, string) (int, error) { return 0, errPopplerTeste }
	textoPDF = func(context.Context, string, int) ([]string, error) { return nil, errPopplerTeste }
	os.Exit(m.Run())
}

// trocarPoppler substitui o segundo leitor até o fim do teste.
func trocarPoppler(t *testing.T, info func(context.Context, string) (int, error), texto func(context.Context, string, int) ([]string, error)) {
	t.Helper()
	antigoInfo, antigoTexto := infoPDF, textoPDF
	t.Cleanup(func() { infoPDF, textoPDF = antigoInfo, antigoTexto })
	if info != nil {
		infoPDF = info
	}
	if texto != nil {
		textoPDF = texto
	}
}

func fonteDeBytes(b []byte) *fontePDF {
	return &fontePDF{dados: b, tamanho: int64(len(b)), mem: &memoriaPDF{}}
}

// comoPDF20 troca o cabeçalho de um PDF sadio por %PDF-2.0, que a biblioteca recusa.
func comoPDF20(b []byte) []byte {
	return append([]byte("%PDF-2.0"), bytes.TrimPrefix(b, []byte("%PDF-1.4"))...)
}

func textoPoppler(n int, semente string) string {
	return strings.Repeat(semente+" ", n/(len(semente)+1)+1)[:n]
}

func TestAbrirPDF_BibliotecaAbre(t *testing.T) {
	isolarTemp(t)
	var chamadas atomic.Int32
	trocarPoppler(t, func(context.Context, string) (int, error) { chamadas.Add(1); return 9, nil }, nil)
	f := fonteDeBytes(pdfComTexto("Texto sadio ficticio", "Outra pagina ficticia"))
	defer f.liberar()
	ab, err := abrirPDF(context.Background(), f)
	if err != nil {
		t.Fatalf("abrirPDF: %v", err)
	}
	if ab.poppler || ab.leitor == nil || ab.paginas != 2 {
		t.Errorf("abertura = %+v", ab)
	}
	if n := chamadas.Load(); n != 0 {
		t.Errorf("infoPDF chamado %d vezes", n)
	}
}

func TestAbrirPDF_CaiNoPoppler(t *testing.T) {
	tmp := isolarTemp(t)
	var caminho string
	trocarPoppler(t, func(_ context.Context, c string) (int, error) {
		caminho = c
		if _, err := os.Stat(c); err != nil {
			t.Errorf("infoPDF recebeu um caminho que não existe: %v", err)
		}
		return 3, nil
	}, nil)
	f := fonteDeBytes(comoPDF20(pdfComTexto("Texto sadio ficticio")))
	ab, err := abrirPDF(context.Background(), f)
	if err != nil {
		t.Fatalf("abrirPDF: %v", err)
	}
	if !ab.poppler || ab.paginas != 3 || ab.leitor != nil {
		t.Errorf("abertura = %+v", ab)
	}
	if caminho == "" {
		t.Fatal("infoPDF não foi chamado")
	}
	f.liberar()
	semTemporarios(t, tmp)
}

func TestAbrirPDF_PanicoDaBibliotecaCaiNoPoppler(t *testing.T) {
	isolarTemp(t)
	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) {
		panic("loading {8 0}: found {7 0}")
	}
	trocarPoppler(t, func(context.Context, string) (int, error) { return 3, nil }, nil)
	f := fonteDeBytes(pdfComTexto("Texto sadio ficticio"))
	defer f.liberar()
	ab, err := abrirPDF(context.Background(), f)
	if err != nil {
		t.Fatalf("abrirPDF: %v", err)
	}
	if !ab.poppler || ab.paginas != 3 || ab.leitor != nil {
		t.Errorf("abertura = %+v", ab)
	}
}

func TestAbrirPDF_SenhaNaoTentaOPoppler(t *testing.T) {
	isolarTemp(t)
	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) {
		return nil, 0, pdf.ErrInvalidPassword
	}
	var chamadas atomic.Int32
	trocarPoppler(t, func(context.Context, string) (int, error) { chamadas.Add(1); return 1, nil }, nil)
	f := fonteDeBytes(pdfComTexto("Texto sadio ficticio"))
	defer f.liberar()
	_, err := abrirPDF(context.Background(), f)
	if err == nil {
		t.Fatal("esperava erro de senha")
	}
	if l := classificarErro(err); l != LeituraSenha {
		t.Errorf("classificarErro = %q, esperava senha (%v)", l, err)
	}
	if n := chamadas.Load(); n != 0 {
		t.Errorf("infoPDF chamado %d vezes", n)
	}
}

func TestAbrirPDF_OsDoisFalham(t *testing.T) {
	tmp := isolarTemp(t)
	trocarPoppler(t, func(context.Context, string) (int, error) {
		return 0, fmt.Errorf("%w: pdfinfo sem resposta", ocr.ErrPrazo)
	}, nil)
	f := fonteDeBytes([]byte("não é PDF"))
	_, err := abrirPDF(context.Background(), f)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if l := classificarErro(err); l != LeituraPrazo {
		t.Errorf("classificarErro = %q, esperava prazo (%v)", l, err)
	}
	if !strings.Contains(err.Error(), "invalid header") {
		t.Errorf("o erro não cita a biblioteca: %v", err)
	}
	f.liberar()
	semTemporarios(t, tmp)
}

// parserPoppler monta um parser do caminho do Poppler com um caminho fictício
// em disco (textoPDF e o OCR são trocados, então nada o lê).
func parserPoppler(n int) *PDFParser {
	return NewPDFParserDeLeitor(bytes.NewReader(nil), 0, "doc.pdf",
		func() (string, error) { return "doc-ficticio.pdf", nil }).comPoppler(n)
}

type runnerContado struct {
	mu      sync.Mutex
	paginas []int
}

func (r *runnerContado) run(_ context.Context, _ string, pagina int) (string, error) {
	r.mu.Lock()
	r.paginas = append(r.paginas, pagina)
	r.mu.Unlock()
	return fmt.Sprintf("Texto de OCR ficticio da pagina %d", pagina), nil
}

func paginaPoppler(t *testing.T, pages map[string]any, k string) map[string]any {
	t.Helper()
	p, ok := pages[k].(map[string]any)
	if !ok {
		t.Fatalf("página %s ausente: %v", k, pages)
	}
	return p
}

func TestParsePages_Poppler_TextoNativo(t *testing.T) {
	trocarPoppler(t, nil, func(_ context.Context, _ string, n int) ([]string, error) {
		return []string{textoPoppler(150, "Primeira pagina ficticia"), textoPoppler(150, "Segunda pagina ficticia")}, nil
	})
	var r runnerContado
	pages, err := parserPoppler(2).ParsePages(context.Background(), true, r.run, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("páginas = %d", len(pages))
	}
	for _, k := range []string{"1", "2"} {
		if o, _ := paginaPoppler(t, pages, k)["ocr"].(bool); o {
			t.Errorf("página %s foi ao OCR", k)
		}
	}
	if len(r.paginas) != 0 {
		t.Errorf("runner chamado para %v", r.paginas)
	}
}

func TestParsePages_Poppler_PaginaCurtaVaiAoOCR(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return []string{"", textoPoppler(200, "Pagina nativa ficticia")}, nil
	})
	var r runnerContado
	pages, err := parserPoppler(2).ParsePages(context.Background(), true, r.run, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if o, _ := paginaPoppler(t, pages, "1")["ocr"].(bool); !o {
		t.Errorf("página 1 não foi ao OCR: %v", pages["1"])
	}
	if o, _ := paginaPoppler(t, pages, "2")["ocr"].(bool); o {
		t.Errorf("página 2 foi ao OCR")
	}
	if len(r.paginas) != 1 || r.paginas[0] != 1 {
		t.Errorf("runner chamado para %v, esperava [1]", r.paginas)
	}
}

func TestParsePages_Poppler_SemOCR(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return []string{"", ""}, nil
	})
	pages, err := parserPoppler(2).ParsePages(context.Background(), false, nil, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	for _, k := range []string{"1", "2"} {
		p := paginaPoppler(t, pages, k)
		if so, _ := p["sem_ocr"].(bool); !so || p["content"] != marcadorPaginaSemOCR {
			t.Errorf("página %s = %v", k, p)
		}
	}
	if l := classificarPaginas(pages, true); l != LeituraSemOCR {
		t.Errorf("classificarPaginas = %q", l)
	}
}

// Texto curto com o OCR desligado fica como está e o documento conta como lido.
func TestParsePages_Poppler_SemOCRTextoCurtoFica(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return []string{"  Despacho curto ficticio \n"}, nil
	})
	pages, err := parserPoppler(1).ParsePages(context.Background(), false, nil, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if c := paginaPoppler(t, pages, "1")["content"]; c != "Despacho curto ficticio" {
		t.Errorf("content = %q", c)
	}
	if l := classificarPaginas(pages, true); l != LeituraSegundoLeitor {
		t.Errorf("classificarPaginas = %q", l)
	}
}

func TestParsePages_Poppler_TextoFalhaComOCR(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return nil, errors.New("pdftotext falhou")
	})
	var r runnerContado
	pages, err := parserPoppler(3).ParsePages(context.Background(), true, r.run, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if len(r.paginas) != 3 || len(pages) != 3 {
		t.Errorf("runner chamado para %v; páginas = %d", r.paginas, len(pages))
	}
}

func TestParsePages_Poppler_TextoFalhaSemOCR(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return nil, fmt.Errorf("%w: pdftotext sem resposta", ocr.ErrPrazo)
	})
	_, err := parserPoppler(3).ParsePages(context.Background(), false, nil, nil)
	if classificarErro(err) != LeituraPrazo {
		t.Errorf("err = %v", err)
	}
}

func cfgPoppler(ocrMode string) *models.Config {
	return &models.Config{CleaningLevel: "none", OcrMode: ocrMode}
}

func TestPipeline_Poppler_ProgressoCerto(t *testing.T) {
	tmp := isolarTemp(t)
	trocarPoppler(t,
		func(context.Context, string) (int, error) { return 7, nil },
		func(_ context.Context, _ string, n int) ([]string, error) {
			textos := make([]string, n)
			for i := range textos {
				textos[i] = textoPoppler(150, fmt.Sprintf("Pagina %d ficticia", i+1))
			}
			return textos, nil
		})
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", comoPDF20(pdfComTexto("Texto ficticio"))}})
	var mu sync.Mutex
	var totais, feitas []int
	atv := NovaAtividade("x")
	_, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgPoppler("none"),
		func(_ float64, _ string, _, _, processed, total int) {
			mu.Lock()
			totais, feitas = append(totais, total), append(feitas, processed)
			mu.Unlock()
		}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	if len(totais) == 0 {
		t.Fatal("onProgress não foi chamado")
	}
	for i, tot := range totais {
		if tot != 7 {
			t.Errorf("chamada %d: totalPages = %d", i, tot)
		}
	}
	if ult := feitas[len(feitas)-1]; ult != 7 {
		t.Errorf("última chamada: processedPages = %d", ult)
	}
	if d := atv.Instantaneo().Docs[0]; d.Leitura != string(LeituraSegundoLeitor) {
		t.Errorf("leitura = %q", d.Leitura)
	}
	semTemporarios(t, tmp)
}

func TestPipeline_Poppler_NaoAbreDuasVezes(t *testing.T) {
	tmp := isolarTemp(t)
	var chamadas atomic.Int32
	trocarPoppler(t, func(context.Context, string) (int, error) {
		chamadas.Add(1)
		return 0, errors.New("pdfinfo: arquivo ficticio ilegivel")
	}, nil)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", []byte("não é PDF")}})
	_, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfgPoppler("fast"),
		func(float64, string, int, int, int, int) {})
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	if n := chamadas.Load(); n != 1 {
		t.Errorf("infoPDF chamado %d vezes, esperava 1", n)
	}
	semTemporarios(t, tmp)
}

// documentoDe devolve o bloco <documento ...>...</documento> que contém nome.
func documentoDe(t *testing.T, saida, nome string) string {
	t.Helper()
	i := strings.Index(saida, `nome="`+nome+`"`)
	if i < 0 {
		t.Fatalf("documento %s ausente:\n%s", nome, saida)
	}
	ini := strings.LastIndex(saida[:i], "<documento")
	fim := strings.Index(saida[i:], "</documento>")
	return saida[ini : i+fim]
}

func TestPipeline_BinariosIndisponiveis(t *testing.T) {
	tmp := isolarTemp(t)
	trocarPoppler(t,
		func(context.Context, string) (int, error) { return 0, errors.New("falha ao extrair binários") },
		func(context.Context, string, int) ([]string, error) {
			return nil, errors.New("falha ao extrair binários")
		})
	sadio := pdfComTexto(textoPoppler(150, "Peticao inicial ficticia de FULANO DE TAL"))

	antes := gravarZip(t, []entradaZip{{"Evento 1 - INIC1.pdf", sadio}})
	outAntes, _, _, _, err := NewPipeline().ProcessZip(context.Background(), antes, cfgPoppler("fast"),
		func(float64, string, int, int, int, int) {})
	if err != nil {
		t.Fatalf("ProcessZip (só o sadio): %v", err)
	}
	bAntes, _ := os.ReadFile(outAntes)

	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - INIC1.pdf", sadio},
		{"Evento 2 - DOC2.pdf", comoPDF20(pdfComTexto("Texto ficticio"))},
	})
	atv := NovaAtividade("x")
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgPoppler("fast"),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	if a, d := documentoDe(t, string(bAntes), "Evento 1 - INIC1.pdf"), documentoDe(t, string(b), "Evento 1 - INIC1.pdf"); a != d {
		t.Errorf("o PDF sadio mudou:\nantes:\n%s\ndepois:\n%s", a, d)
	}
	docs := atv.Instantaneo().Docs
	if docs[0].Leitura != string(LeituraLido) {
		t.Errorf("sadio: %+v", docs[0])
	}
	if d := docs[1]; d.Leitura != string(LeituraIlegivel) || !strings.Contains(d.Detalhe, "falha ao extrair binários") {
		t.Errorf("recusado: %+v", d)
	}
	semTemporarios(t, tmp)
}
