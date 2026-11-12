package etl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

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
	var infoChamadas atomic.Int32
	trocarPoppler(t,
		func(context.Context, string) (int, error) { infoChamadas.Add(1); return 7, nil },
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
	if n := infoChamadas.Load(); n != 1 {
		t.Errorf("infoPDF chamado %d vezes, esperava 1", n)
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
	if d := docs[1]; d.Leitura != string(LeituraIlegivel) || !strings.Contains(d.Detalhe, "invalid header") || !strings.Contains(d.Detalhe, resumoSegundoLeitorFalha) || strings.Contains(d.Detalhe, "falha ao extrair binários") {
		t.Errorf("recusado: %+v", d)
	}
	semTemporarios(t, tmp)
}

// erroComCaminho imita o erro do Poppler real: caminho do temporário com o
// nome de usuário e uma linha de stderr.
var erroComCaminho = errors.New(`falha no pdfinfo.exe em C:\Users\fulano\AppData\Local\Temp\eprocia_pdf_1.pdf: Syntax Error: Couldn't find trailer dictionary`)

func semDadosDoPoppler(t *testing.T, onde, texto string) {
	t.Helper()
	for _, proibido := range []string{"fulano", "AppData", "trailer dictionary"} {
		if strings.Contains(texto, proibido) {
			t.Errorf("%s contém %q:\n%s", onde, proibido, texto)
		}
	}
}

func TestPipeline_Poppler_ErroDoInfoSemCaminho(t *testing.T) {
	isolarTemp(t)
	trocarPoppler(t, func(context.Context, string) (int, error) { return 0, erroComCaminho }, nil)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", []byte("não é PDF")}})
	atv := NovaAtividade("x")
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgPoppler("fast"),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	d := atv.Instantaneo().Docs[0]
	for onde, texto := range map[string]string{".txt": string(b), "Detalhe": d.Detalhe} {
		semDadosDoPoppler(t, onde, texto)
		if !strings.Contains(texto, "invalid header") || !strings.Contains(texto, resumoSegundoLeitorFalha) {
			t.Errorf("%s sem o texto da biblioteca ou o resumo:\n%s", onde, texto)
		}
	}
	if d.Leitura != string(LeituraIlegivel) {
		t.Errorf("leitura = %q", d.Leitura)
	}
}

func TestPipeline_Poppler_ErroDoTextoSemCaminho(t *testing.T) {
	isolarTemp(t)
	trocarPoppler(t,
		func(context.Context, string) (int, error) { return 2, nil },
		func(context.Context, string, int) ([]string, error) { return nil, erroComCaminho })
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", comoPDF20(pdfComTexto("Texto ficticio"))}})
	atv := NovaAtividade("x")
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgPoppler("none"),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	d := atv.Instantaneo().Docs[0]
	for onde, texto := range map[string]string{".txt": string(b), "Detalhe": d.Detalhe} {
		semDadosDoPoppler(t, onde, texto)
		if !strings.Contains(texto, resumoSegundoLeitorFalha) {
			t.Errorf("%s sem o resumo:\n%s", onde, texto)
		}
	}
	if d.Leitura != string(LeituraIlegivel) {
		t.Errorf("leitura = %q", d.Leitura)
	}
}

func TestAbrirPDF_ResumoMantemPrazoESenha(t *testing.T) {
	isolarTemp(t)
	for _, c := range []struct {
		causa error
		quer  Leitura
	}{
		{fmt.Errorf("%w: em C:\\Users\\fulano\\x.pdf", ocr.ErrPrazo), LeituraPrazo},
		{fmt.Errorf("%w: em C:\\Users\\fulano\\x.pdf", ocr.ErrSenha), LeituraSenha},
	} {
		trocarPoppler(t, func(context.Context, string) (int, error) { return 0, c.causa }, nil)
		f := fonteDeBytes([]byte("não é PDF"))
		_, err := abrirPDF(context.Background(), f)
		f.liberar()
		if classificarErro(err) != c.quer {
			t.Errorf("classificarErro(%v) != %q", err, c.quer)
		}
		semDadosDoPoppler(t, "erro", err.Error())
	}
}

func TestAbrirPDF_ZeroPaginasEFalha(t *testing.T) {
	isolarTemp(t)
	trocarPoppler(t, func(context.Context, string) (int, error) { return 0, nil }, nil)
	f := fonteDeBytes(comoPDF20(pdfComTexto("Texto ficticio")))
	defer f.liberar()
	if _, err := abrirPDF(context.Background(), f); err == nil {
		t.Fatal("pdfinfo com 0 páginas deveria ser falha do segundo leitor")
	}
}

func TestPipeline_OmitidoNaoPassaPeloPoppler(t *testing.T) {
	tmp := isolarTemp(t)
	var chamadas atomic.Int32
	trocarPoppler(t, func(context.Context, string) (int, error) { chamadas.Add(1); return 3, nil }, nil)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - INIC1.pdf", pdfComTexto("Texto da inicial ficticia")},
		{"Evento 2 - EXTR1.pdf", comoPDF20(pdfComTexto("TEXTO-DO-EXTRATO-FICTICIO"))},
	})
	cfg := cfgPoppler("fast")
	cfg.OmitirTipos = []string{"EXTR"}
	out, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg,
		func(float64, string, int, int, int, int) {})
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	if n := chamadas.Load(); n != 0 {
		t.Errorf("infoPDF chamado %d vezes para o omitido", n)
	}
	b, _ := os.ReadFile(out)
	if !strings.Contains(string(b), "[documento omitido: EXTR, 1 página]") || strings.Contains(string(b), "TEXTO-DO-EXTRATO") {
		t.Errorf("omitido fora do esperado:\n%s", b)
	}
	semTemporarios(t, tmp)
}

// pdfAcimaDoTeto é um PDF de texto de uma página com um objeto solto que o leva
// além de maxPDFEmMemoria, para que extrairPDF o grave num temporário em disco.
func pdfAcimaDoTeto(texto string) []byte {
	stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", texto)
	enchimento := strings.Repeat(" ", maxPDFEmMemoria+1)
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
		fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(enchimento), enchimento),
	})
}

// O omitido acima do teto de memória vai a um temporário em disco só para a
// contagem; nada o lê depois, então o temporário sai já na pré-contagem.
func TestPipeline_OmitidoEmDiscoLiberaNaPreContagem(t *testing.T) {
	tmp := isolarTemp(t)
	trocarPoppler(t, func(context.Context, string) (int, error) {
		t.Error("infoPDF chamado para o omitido")
		return 1, nil
	}, nil)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - INIC1.pdf", pdfComTexto("Texto da inicial ficticia")},
		{"Evento 2 - EXTR1.pdf", pdfAcimaDoTeto("TEXTO-DO-EXTRATO-FICTICIO")},
	})
	var restantes []string
	t.Cleanup(func() { aoFimDaPreContagem = nil })
	aoFimDaPreContagem = func(*memoriaPDF) {
		restantes, _ = filepath.Glob(filepath.Join(tmp, "eprocia_pdf_*"))
	}
	cfg := cfgPoppler("none")
	cfg.OmitirTipos = []string{"EXTR"}
	out, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg, nil)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	if len(restantes) != 0 {
		t.Errorf("temporário do omitido aberto depois da pré-contagem: %v", restantes)
	}
	b, _ := os.ReadFile(out)
	if !strings.Contains(string(b), "[documento omitido: EXTR, 1 página]") || strings.Contains(string(b), "TEXTO-DO-EXTRATO") {
		t.Errorf("omitido fora do esperado:\n%s", b)
	}
	semTemporarios(t, tmp)
}

// A falha ao gravar o temporário (CreateTemp num TEMP inexistente) não leva o
// caminho de %TEMP% ao erro que vai para o .txt e o painel.
func TestAbrirPDF_FalhaAoGravarTemporarioSemCaminho(t *testing.T) {
	inexistente := filepath.Join(t.TempDir(), "fulano-nao-existe")
	t.Setenv("TMP", inexistente)
	t.Setenv("TEMP", inexistente)
	t.Setenv("TMPDIR", inexistente)
	trocarPoppler(t, func(context.Context, string) (int, error) {
		t.Error("infoPDF chamado sem temporário")
		return 1, nil
	}, nil)
	f := fonteDeBytes(comoPDF20(pdfComTexto("Texto ficticio")))
	defer f.liberar()
	_, err := abrirPDF(context.Background(), f)
	if err == nil {
		t.Fatal("abrirPDF deveria falhar sem temporário")
	}
	msg := err.Error()
	if strings.Contains(msg, "fulano") || strings.Contains(msg, inexistente) {
		t.Errorf("erro traz o caminho do temporário: %q", msg)
	}
	if !strings.Contains(msg, "segundo leitor: "+resumoSegundoLeitorTemporario) {
		t.Errorf("erro sem o resumo do temporário: %q", msg)
	}
}

// trocarBiblioteca faz a biblioteca recusar todo PDF com o erro dado.
func trocarBiblioteca(t *testing.T, err error) {
	t.Helper()
	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) { return nil, 0, err }
}

const erroCifraLongo = "unsupported PDF: encryption version V=4; <</CF <</StdCF <</CFM /AESV2>>>> /O (abc\x01def) /U (ghi)>>"

func TestAbrirPDF_ErroDeCifraSaiCurto(t *testing.T) {
	isolarTemp(t)
	trocarBiblioteca(t, errors.New(erroCifraLongo))
	f := fonteDeBytes(pdfComTexto("Texto ficticio"))
	defer f.liberar()
	_, err := abrirPDF(context.Background(), f)
	const esperado = "falha ao abrir o PDF: unsupported PDF: encryption version V=4; segundo leitor: " + resumoSegundoLeitorFalha
	if err == nil || err.Error() != esperado {
		t.Fatalf("erro = %v; esperava %q", err, esperado)
	}
}

func TestAbrirPDF_ErroLongoDaBibliotecaTemTeto(t *testing.T) {
	isolarTemp(t)
	trocarBiblioteca(t, errors.New("malformed PDF: cross-reference table not found: "+strings.Repeat("x", 500)))
	f := fonteDeBytes(pdfComTexto("Texto ficticio"))
	defer f.liberar()
	_, err := abrirPDF(context.Background(), f)
	if err == nil {
		t.Fatal("abrirPDF deveria falhar")
	}
	msg := strings.TrimPrefix(err.Error(), "falha ao abrir o PDF: ")
	lib, _, ok := strings.Cut(msg, "; segundo leitor: ")
	if !ok {
		t.Fatalf("erro sem o resumo do segundo leitor: %q", err)
	}
	if n := utf8.RuneCountInString(lib); n > 121 || !strings.HasSuffix(lib, "…") {
		t.Errorf("trecho da biblioteca com %d runas: %q", n, lib)
	}
	if !errors.Is(err, errPopplerTeste) {
		t.Errorf("errors.Is não chega à causa do segundo leitor: %v", err)
	}
}

func TestAbrirPDF_ErroCurtoFicaIgual(t *testing.T) {
	for _, texto := range []string{"not a PDF file: invalid header", "loading {8 0}: found {7 0}", "not a PDF file: missing %%EOF"} {
		if got := resumirErroBiblioteca(errors.New(texto)); got != texto {
			t.Errorf("resumirErroBiblioteca(%q) = %q", texto, got)
		}
	}
	isolarTemp(t)
	trocarBiblioteca(t, errors.New("not a PDF file: invalid header"))
	f := fonteDeBytes(pdfComTexto("Texto ficticio"))
	defer f.liberar()
	_, err := abrirPDF(context.Background(), f)
	const esperado = "falha ao abrir o PDF: not a PDF file: invalid header; segundo leitor: " + resumoSegundoLeitorFalha
	if err == nil || err.Error() != esperado {
		t.Fatalf("erro = %v; esperava %q", err, esperado)
	}
}

func TestResumirErroBiblioteca_ControleViraEspaco(t *testing.T) {
	if got := resumirErroBiblioteca(errors.New("linha um\nlinha\x01dois\r")); got != "linha um linha dois " {
		t.Errorf("resumo = %q", got)
	}
}

func TestPipeline_ErroDeCifraSemDicionario(t *testing.T) {
	tmp := isolarTemp(t)
	trocarBiblioteca(t, errors.New(erroCifraLongo))
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - INIC1.pdf", pdfComTexto("Texto ficticio")}})
	atv := NovaAtividade("x")
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgPoppler("none"),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	if strings.Contains(string(b), "/O (") || !strings.Contains(string(b), "encryption version V=4") {
		t.Errorf(".txt com o dicionário /Encrypt ou sem o resumo:\n%s", b)
	}
	if d := atv.Instantaneo().Docs[0].Detalhe; strings.Contains(d, "/O (") {
		t.Errorf("Detalhe com o dicionário /Encrypt: %q", d)
	}
	semTemporarios(t, tmp)
}
