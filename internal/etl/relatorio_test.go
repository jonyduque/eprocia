package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	stdRuntime "runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"eprocia/internal/models"
	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
)

// pdfPorTipo monta um PDF com uma página por letra de tipos: 'T' é texto nativo
// longo, 'I' é página escaneada (só imagem grande, candidata ao OCR) e 'C' é
// texto nativo curto com imagem grande (vai ao OCR com o nativo de reserva).
// marca entra no texto nativo, para o OCR falso reconhecer o PDF pelo arquivo.
func pdfPorTipo(tipos, marca string) []byte {
	n := len(tipos)
	kids := make([]string, n)
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"",
		"3 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj",
	}
	for i := range n {
		pagina, conteudo := 4+2*i, 5+2*i
		kids[i] = fmt.Sprintf("%d 0 R", pagina)
		stream := "/Im1 Do"
		switch tipos[i] {
		case 'T':
			stream = fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", textoLongo(fmt.Sprintf("%s pagina %d", marca, i+1), 300))
		case 'C':
			stream = fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(Texto curto %s) Tj\nET\n/Im1 Do", marca)
		}
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 3 0 R >> >> /Contents %d 0 R >>\nendobj", pagina, conteudo),
			fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", conteudo, len(stream), stream),
		)
	}
	objs[1] = fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), n)
	return buildPDF(objs)
}

// trocarOCR troca o OCR por página e o em lote durante o teste.
func trocarOCR(t *testing.T,
	pagina func(context.Context, string, int, string) (string, error),
	lote func(context.Context, string, []int, string, func(int)) ([]string, error)) {
	t.Helper()
	antigoPagina, antigoLote := executarOCR, executarOCRPaginas
	t.Cleanup(func() { executarOCR, executarOCRPaginas = antigoPagina, antigoLote })
	if pagina != nil {
		executarOCR = pagina
	}
	if lote != nil {
		executarOCRPaginas = lote
	}
}

// loteDe devolve um OCR em lote que chama pagina para cada página, em ordem.
func loteDe(pagina func(context.Context, string, int, string) (string, error)) func(context.Context, string, []int, string, func(int)) ([]string, error) {
	return func(ctx context.Context, caminho string, paginas []int, modo string, aoConcluir func(int)) ([]string, error) {
		textos := make([]string, len(paginas))
		for i, p := range paginas {
			txt, err := pagina(ctx, caminho, p, modo)
			if err != nil {
				return nil, err
			}
			textos[i] = txt
			if aoConcluir != nil {
				aoConcluir(p)
			}
		}
		return textos, nil
	}
}

// processarComRelatorio roda o pipeline com uma atividade e devolve o
// relatório que ela recebeu e o texto da saída.
func processarComRelatorio(t *testing.T, zipPath string, cfg *models.Config) (*models.RelatorioZip, string) {
	t.Helper()
	atv := NovaAtividade("item-teste")
	saida, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfg, nil, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	txt, err := os.ReadFile(saida)
	if err != nil {
		t.Fatalf("ler saída: %v", err)
	}
	r := atv.Relatorio()
	if r == nil {
		t.Fatal("atividade sem relatório")
	}
	return r, string(txt)
}

func somaCategorias(p models.PaginasRelatorio) int {
	return p.LidaNativa + p.LidaOCR + p.Vazia + p.SemOCR + p.ComErro
}

func TestRelatorio_SomaDasCategoriasIgualAoTotal(t *testing.T) {
	isolarTemp(t)
	// Com o OCR ligado não há página sem_ocr: ela fica para o teste do
	// cabeçalho e o de Atencao.
	ocrTexto := func(context.Context, string, int, string) (string, error) { return ocrFalsoTexto, nil }
	trocarOCR(t, ocrTexto, loteDe(ocrTexto))
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("Primeira", 300), textoLongo("Segunda", 300))},
		{"Evento 2 - DOC2.pdf", buildPDF(objetosPaginasComImagem(3))},
		{"Evento 3 - EXTR3.pdf", pdfComTexto(textoLongo("Extrato", 200), textoLongo("Extrato dois", 200))},
		{"Evento 4 - DOC4.pdf", pdfComTexto("", "")},
		{"Evento 5 - DOC5.pdf", pdfComTexto(textoLongo("Quinta", 300), "")},
		{"Evento 6 - PET6.html", []byte("<html><body><p>Peticao ficticia do evento seis</p></body></html>")},
		// Cópia da primeira página de DOC1: duplicata H7.
		{"Evento 7 - DOC7.pdf", pdfComTexto(textoLongo("Primeira", 300), textoLongo("Setima", 300))},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", OmitirTipos: []string{"EXTR"}}
	r, txt := processarComRelatorio(t, zipPath, cfg)

	p := r.Paginas
	if got := somaCategorias(p); got != p.Total {
		t.Errorf("soma das categorias = %d, total = %d (%+v)", got, p.Total, p)
	}
	// DOC1 2 + DOC2 3 + DOC5 2 + HTML 1 + DOC7 2.
	if p.Total != 10 {
		t.Errorf("Total = %d, quero 10 (%+v)", p.Total, p)
	}
	if p.LidaOCR != 3 || p.Vazia != 1 || p.LidaNativa != 6 {
		t.Errorf("categorias = %+v; quero 3 OCR, 1 vazia, 6 nativas", p)
	}
	if p.DeOmitidos != 2 {
		t.Errorf("DeOmitidos = %d, quero 2", p.DeOmitidos)
	}
	if p.DeNaoLidos != 2 {
		t.Errorf("DeNaoLidos = %d, quero 2", p.DeNaoLidos)
	}
	if !strings.Contains(txt, "[conteúdo idêntico a: evento 1, ") {
		t.Fatalf("a saída não tem duplicata H7:\n%s", txt)
	}
	if p.Duplicadas < 1 {
		t.Errorf("Duplicadas = %d, quero >= 1", p.Duplicadas)
	}
	d := r.Documentos
	if d.Total != 7 || d.Lidos != 5 || d.NaoLidos != 1 || d.Omitidos != 1 || d.PDF != 6 || d.HTML != 1 {
		t.Errorf("documentos = %+v", d)
	}
	if !slices.Equal(d.TiposOmitidos, []string{"EXTR"}) {
		t.Errorf("TiposOmitidos = %v", d.TiposOmitidos)
	}
	if d.PorLeitura["sem_texto"] != 1 || len(d.PorLeitura) != 1 {
		t.Errorf("PorLeitura = %v", d.PorLeitura)
	}
	if p.PorMotivo["padrao"] != 3 {
		t.Errorf("PorMotivo = %v", p.PorMotivo)
	}
	if r.Limpeza.Nivel != "standard" || r.BytesTxt != int64(len(txt)) || r.Tokens == 0 {
		t.Errorf("limpeza %+v, bytes %d (arquivo %d), tokens %d", r.Limpeza, r.BytesTxt, len(txt), r.Tokens)
	}
}

func TestRelatorio_SubContadores(t *testing.T) {
	isolarTemp(t)
	// O OCR falha nos PDFs marcados FALHA e volta em branco nos marcados BRANCO.
	ocr := func(_ context.Context, caminho string, _ int, _ string) (string, error) {
		dados, err := os.ReadFile(caminho)
		if err != nil {
			return "", err
		}
		switch {
		case bytes.Contains(dados, []byte("FALHA")):
			return "", errors.New("falha no tesseract embutido")
		case bytes.Contains(dados, []byte("BRANCO")):
			return "", nil
		}
		return ocrFalsoTexto, nil
	}
	trocarOCR(t, ocr, loteDe(ocr))
	zipPath := gravarZip(t, []entradaZip{
		// Lidos: a página nativa segura o documento; a falha com reserva tem
		// ocr_aviso e ocr_erro (regra padrão).
		{"Evento 1 - DOC1.pdf", pdfPorTipo("TC", "FALHA um")},
		{"Evento 2 - DOC2.pdf", pdfPorTipo("TC", "FALHA dois")},
		{"Evento 3 - DOC3.pdf", pdfPorTipo("C", "BRANCO tres")},
		{"Evento 4 - DOC4.pdf", pdfPorTipo("C", "LIDO quatro")},
		// Não lido (o OCR falhou na única página): fora dos sub-contadores.
		{"Evento 5 - DOC5.pdf", pdfPorTipo("C", "FALHA cinco")},
	})
	var eventos map[string]any
	antigo := gerarSaida
	t.Cleanup(func() { gerarSaida = antigo })
	gerarSaida = func(ev map[string]any, nivel string, semOCR int) SaidaConsolidada {
		eventos = ev
		return antigo(ev, nivel, semOCR)
	}
	r, _ := processarComRelatorio(t, zipPath, &models.Config{CleaningLevel: "basic", OcrMode: "normal"})

	aviso, erroComAviso, semGanho := 0, 0, 0
	for _, ev := range eventos {
		for _, dv := range ev.(map[string]any)["documentos"].(map[string]any) {
			if _, ok := dv.(map[string]any)["nao_lido"]; ok {
				continue
			}
			for _, pv := range dv.(map[string]any)["paginas"].(map[string]any) {
				pg := pv.(map[string]any)
				if _, ok := pg["ocr_aviso"]; ok {
					aviso++
					if _, ok := pg["ocr_erro"]; ok {
						erroComAviso++
					}
				}
				if _, ok := pg["ocr_sem_ganho"]; ok {
					semGanho++
				}
			}
		}
	}
	if aviso != 2 || erroComAviso != 2 || semGanho != 1 {
		t.Fatalf("mapa com %d ocr_aviso (%d com ocr_erro) e %d ocr_sem_ganho; quero 2, 2 e 1", aviso, erroComAviso, semGanho)
	}
	p := r.Paginas
	if p.OCRFalhouComReserva != aviso {
		t.Errorf("OCRFalhouComReserva = %d, quero %d", p.OCRFalhouComReserva, aviso)
	}
	if p.OCRSemGanho != semGanho {
		t.Errorf("OCRSemGanho = %d, quero %d", p.OCRSemGanho, semGanho)
	}
	if p.ComErro != 0 {
		t.Errorf("ComErro = %d: página com ocr_aviso contou como erro", p.ComErro)
	}
	// DOC1 e DOC2: 2 nativas cada (a de reserva conta como texto do PDF);
	// DOC3 nativa sem ganho; DOC4 por OCR.
	if p.LidaOCR != 1 || p.LidaNativa != 5 || p.Total != 6 || p.DeNaoLidos != 1 {
		t.Errorf("categorias = %+v", p)
	}
	if r.Documentos.PorLeitura["ocr_falhou"] != 1 {
		t.Errorf("PorLeitura = %v", r.Documentos.PorLeitura)
	}
	if len(r.Atencao) != 2 {
		t.Errorf("Atencao = %+v; quero DOC1 e DOC2", r.Atencao)
	}
	for _, a := range r.Atencao {
		if !slices.Equal(a.OCRFalhouComReserva, []int{2}) {
			t.Errorf("%s: OCRFalhouComReserva = %v", a.Nome, a.OCRFalhouComReserva)
		}
	}
}

func TestRelatorio_LinhaDoCabecalhoSoDosLidos(t *testing.T) {
	isolarTemp(t)
	const linha = "Páginas que precisavam de OCR e ficaram sem:"
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfPorTipo("TII", "Um")},
		{"Evento 2 - DOC2.pdf", buildPDF(objetosPaginasComImagem(3))},
	})
	r, txt := processarComRelatorio(t, zipPath, cfgSemOCR())
	if !strings.Contains(txt, linha+" 2\n") {
		t.Errorf("cabeçalho sem a linha com 2:\n%s", txt[:min(len(txt), 600)])
	}
	if r.Paginas.SemOCR != 2 || r.Paginas.DeNaoLidos != 3 || r.Documentos.PorLeitura["sem_ocr"] != 1 {
		t.Errorf("páginas %+v, documentos %+v", r.Paginas, r.Documentos)
	}

	zipSem := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfPorTipo("TT", "Um")},
		{"Evento 2 - DOC2.pdf", buildPDF(objetosPaginasComImagem(3))},
	})
	_, txt = processarComRelatorio(t, zipSem, cfgSemOCR())
	if strings.Contains(txt, linha) {
		t.Errorf("linha presente sem página sem_ocr de documento lido:\n%s", txt[:min(len(txt), 600)])
	}
}

// numPaginasParaBlocos devolve páginas suficientes para o lote formar dois
// blocos ou mais com o número de workers desta máquina.
func numPaginasParaBlocos() int {
	w := max(stdRuntime.NumCPU()-1, 1)
	return max(2*w+2, 41)
}

func TestRelatorio_TemposEOCR(t *testing.T) {
	const espera = 20 * time.Millisecond
	ocrPagina := func(context.Context, string, int, string) (string, error) {
		time.Sleep(espera)
		return ocrFalsoTexto, nil
	}
	loteLento := func(falhasRestantes *atomic.Int32) func(context.Context, string, []int, string, func(int)) ([]string, error) {
		return func(_ context.Context, _ string, paginas []int, _ string, aoConcluir func(int)) ([]string, error) {
			time.Sleep(espera)
			if falhasRestantes != nil && falhasRestantes.Add(-1) >= 0 {
				return nil, errors.New("falha no tesseract do bloco")
			}
			textos := make([]string, len(paginas))
			for i, p := range paginas {
				textos[i] = ocrFalsoTexto
				if aoConcluir != nil {
					aoConcluir(p)
				}
			}
			return textos, nil
		}
	}
	n := numPaginasParaBlocos()

	t.Run("blocos", func(t *testing.T) {
		isolarTemp(t)
		trocarOCR(t, ocrPagina, loteLento(nil))
		zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", buildPDF(objetosPaginasComImagem(n))}})
		r, _ := processarComRelatorio(t, zipPath, &models.Config{CleaningLevel: "none", OcrMode: "normal"})
		o, tp := r.OCR, r.Tempo
		if o.Blocos < 2 {
			t.Errorf("Blocos = %d, quero >= 2 (%+v)", o.Blocos, o)
		}
		if o.BlocoMedianaMs < 20 || o.BlocoP95Ms < o.BlocoMedianaMs {
			t.Errorf("mediana %d, p95 %d", o.BlocoMedianaMs, o.BlocoP95Ms)
		}
		if tp.OCRTrabalhoMs < 20*int64(o.Processos) {
			t.Errorf("OCRTrabalhoMs = %d com %d processos", tp.OCRTrabalhoMs, o.Processos)
		}
		if o.PaginaEstimadaMs <= 0 || o.PaginaEstimadaMs > o.BlocoMedioMs {
			t.Errorf("PaginaEstimadaMs = %d, BlocoMedioMs = %d", o.PaginaEstimadaMs, o.BlocoMedioMs)
		}
		if tp.TotalMs < tp.LeituraParedeMs || tp.OCRParedeMs <= 0 || tp.LeituraSomaMs <= 0 {
			t.Errorf("tempos = %+v", tp)
		}
		if o.Refacoes != 0 || o.Prazos != 0 {
			t.Errorf("refações %d, prazos %d", o.Refacoes, o.Prazos)
		}
		if r.Paginas.LidaOCR != n {
			t.Errorf("LidaOCR = %d, quero %d", r.Paginas.LidaOCR, n)
		}
	})
	t.Run("bloco que falha uma vez", func(t *testing.T) {
		isolarTemp(t)
		var falhas atomic.Int32
		falhas.Store(1)
		trocarOCR(t, ocrPagina, loteLento(&falhas))
		zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", buildPDF(objetosPaginasComImagem(n))}})
		r, _ := processarComRelatorio(t, zipPath, &models.Config{CleaningLevel: "none", OcrMode: "normal"})
		if r.OCR.Refacoes != 1 {
			t.Errorf("Refacoes = %d, quero 1 (%+v)", r.OCR.Refacoes, r.OCR)
		}
		if r.Paginas.LidaOCR != n {
			t.Errorf("LidaOCR = %d, quero %d", r.Paginas.LidaOCR, n)
		}
	})
}

func TestRelatorio_EstimativaSoPaginaAPagina(t *testing.T) {
	trocarOCR(t, func(context.Context, string, int, string) (string, error) {
		time.Sleep(20 * time.Millisecond)
		return ocrFalsoTexto, nil
	}, nil)
	med := &medidasOCR{}
	ex := &ocrDoZip{sem: make(chan struct{}, 2), modo: "normal", medidas: med}
	runner, _ := ex.doDoc(0)
	dados := buildPDF(objetosPaginasComImagem(3))
	parser := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf",
		func() (string, error) { return "doc-ficticio.pdf", nil }).comMedidas(med)
	pages, err := parser.ParsePages(context.Background(), true, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("%d páginas", len(pages))
	}
	o, trabalho, _ := med.resumo()
	if o.Blocos != 0 || o.Processos != 3 {
		t.Errorf("Blocos = %d, Processos = %d; quero 0 e 3", o.Blocos, o.Processos)
	}
	if o.PaginaEstimadaMs < 20 || trabalho < 60 {
		t.Errorf("PaginaEstimadaMs = %d, trabalho = %d", o.PaginaEstimadaMs, trabalho)
	}
	if o.BlocoMedioMs != 0 || o.BlocoMedianaMs != 0 || o.BlocoP95Ms != 0 {
		t.Errorf("bloco sem blocos: %+v", o)
	}
}

func TestRelatorio_Atencao(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfPorTipo("TITTI", "Um")},
		{"Evento 2 - DOC2.pdf", pdfPorTipo("TT", "Dois")},
	})
	r, _ := processarComRelatorio(t, zipPath, cfgSemOCR())
	if len(r.Atencao) != 1 {
		t.Fatalf("Atencao = %+v; quero só DOC1", r.Atencao)
	}
	a := r.Atencao[0]
	if a.Nome != "Evento 1 - DOC1.pdf" || a.Evento != 1 || !slices.Equal(a.SemOCR, []int{2, 5}) || len(a.ComErro) != 0 || len(a.OCRFalhouComReserva) != 0 {
		t.Errorf("Atencao[0] = %+v", a)
	}
}

func TestRelatorio_PorLeitura(t *testing.T) {
	isolarTemp(t)
	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(r io.ReaderAt, size int64) (*pdf.Reader, int, error) {
		dados := make([]byte, size)
		if _, err := r.ReadAt(dados, 0); err != nil && err != io.EOF {
			return nil, 0, err
		}
		if bytes.Contains(dados, []byte("COMSENHA")) {
			return nil, 0, pdf.ErrInvalidPassword
		}
		return original(r, size)
	}
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto("Documento COMSENHA ficticio")},
		{"Evento 2 - DOC2.pdf", []byte("não é PDF")},
		{"Evento 3 - DOC3.pdf", pdfComTexto(textoLongo("Terceiro", 200))},
	})
	r, _ := processarComRelatorio(t, zipPath, cfgSemOCR())
	quer := map[string]int{"senha": 1, "ilegivel": 1}
	if len(r.Documentos.PorLeitura) != len(quer) {
		t.Errorf("PorLeitura = %v, quero %v", r.Documentos.PorLeitura, quer)
	}
	for k, v := range quer {
		if r.Documentos.PorLeitura[k] != v {
			t.Errorf("PorLeitura = %v, quero %v", r.Documentos.PorLeitura, quer)
		}
	}
	if r.Documentos.NaoLidos != 2 || r.Documentos.Lidos != 1 || r.Paginas.DeNaoLidos != 2 {
		t.Errorf("documentos %+v, páginas %+v", r.Documentos, r.Paginas)
	}
}

func TestRelatorio_ZipSemPaginasLidas(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - EXTR1.pdf", pdfComTexto(textoLongo("Extrato", 200))},
		{"Evento 2 - EXTR2.pdf", pdfComTexto(textoLongo("Extrato dois", 200), textoLongo("tres", 200))},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", OmitirTipos: []string{"EXTR"}}
	r, _ := processarComRelatorio(t, zipPath, cfg)
	if r.Paginas.Total != 0 || r.Paginas.DeOmitidos != 3 || r.Documentos.Omitidos != 2 {
		t.Errorf("páginas %+v, documentos %+v", r.Paginas, r.Documentos)
	}
	if r.OCR.BlocoMedioMs != 0 || r.OCR.PaginaEstimadaMs != 0 {
		t.Errorf("OCR = %+v", r.OCR)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if s := string(b); strings.Contains(s, "NaN") || strings.Contains(s, "Inf") {
		t.Errorf("JSON com NaN ou Inf: %s", s)
	}
}

func TestMedidasOCR_MedianaEP95(t *testing.T) {
	m := &medidasOCR{}
	for _, ms := range []int{10, 50, 20, 40, 30, 100} {
		m.registrar(2, time.Duration(ms)*time.Millisecond, nil)
	}
	m.registrar(1, 5*time.Millisecond, fmt.Errorf("x: %w", ocr.ErrPrazo))
	o, trabalho, _ := m.resumo()
	if o.Processos != 7 || o.Blocos != 6 || o.PaginasEmBloco != 12 || o.Prazos != 1 {
		t.Errorf("contagens = %+v", o)
	}
	if o.BlocoMedianaMs != 35 || o.BlocoP95Ms != 100 || o.BlocoMedioMs != 41 {
		t.Errorf("mediana %d, p95 %d, médio %d", o.BlocoMedianaMs, o.BlocoP95Ms, o.BlocoMedioMs)
	}
	if trabalho != 255 || o.PaginaEstimadaMs != 255/13 {
		t.Errorf("trabalho %d, por página %d", trabalho, o.PaginaEstimadaMs)
	}
	var nilM *medidasOCR
	nilM.refacao(2)
}

func TestRegistro_RelatorioCategoriasEBlocosNoLog(t *testing.T) {
	isolarTemp(t)
	ocrTexto := func(context.Context, string, int, string) (string, error) { return ocrFalsoTexto, nil }
	trocarOCR(t, ocrTexto, loteDe(ocrTexto))
	n := numPaginasParaBlocos()
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", buildPDF(objetosPaginasComImagem(n))},
		{"Evento 2 - DOC2.pdf", buildPDF(objetosPaginasComImagem(1))},
	})
	ctx, buf := ctxComLogJSON(slog.LevelDebug)
	cfg := &models.Config{CleaningLevel: "none", OcrMode: "normal"}
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfg, nil, NovaAtividade("x")); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	linhas := linhasJSON(t, buf.String())
	zips := linhasComMsg(linhas, "zip concluído")
	if len(zips) != 1 {
		t.Fatalf("zip concluído = %d linhas", len(zips))
	}
	rel, ok := zips[0]["relatorio"].(map[string]any)
	if !ok {
		t.Fatalf("zip concluído sem relatorio: %v", zips[0])
	}
	if pg, _ := rel["paginas"].(map[string]any); pg["lida_ocr"] != float64(n+1) {
		t.Errorf("relatorio.paginas = %v", rel["paginas"])
	}
	for _, d := range linhasComMsg(linhas, "documento concluído") {
		cats, ok := d["categorias"].(map[string]any)
		if !ok || cats[CatLidaOCR] == nil {
			t.Errorf("documento concluído sem categorias: %v", d)
		}
	}
	blocos := linhasComMsg(linhas, "bloco de OCR")
	if len(blocos) < 2 {
		t.Fatalf("%d linhas de bloco de OCR", len(blocos))
	}
	for _, b := range blocos {
		if b["level"] != "DEBUG" || b["paginas"] == nil || b["duracao_ms"] == nil || b["doc_id"] == nil {
			t.Errorf("linha de bloco = %v", b)
		}
	}
}

func TestMedidasOCR_RefacaoNaoContaPaginasEmDobro(t *testing.T) {
	m := &medidasOCR{}
	m.registrar(3, 30*time.Millisecond, errors.New("falha no tesseract do bloco"))
	m.refacao(3)
	for range 3 {
		m.registrar(1, 20*time.Millisecond, nil)
	}
	if n := m.paginas.Load(); n != 3 {
		t.Errorf("páginas que foram ao OCR = %d, quero 3", n)
	}
	o, trabalho, _ := m.resumo()
	// O tempo do bloco falho fica no trabalho: 30 + 3 x 20 = 90 ms em 3 páginas.
	if o.Refacoes != 1 || trabalho != 90 || o.PaginaEstimadaMs != 30 {
		t.Errorf("refações %d, trabalho %d, por página %d", o.Refacoes, trabalho, o.PaginaEstimadaMs)
	}
}

func TestRelatorio_AtencaoLimitada(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfPorTipo("T"+strings.Repeat("I", 120), "Um")},
	})
	r, _ := processarComRelatorio(t, zipPath, cfgSemOCR())
	if len(r.Atencao) != 1 {
		t.Fatalf("Atencao = %d documentos", len(r.Atencao))
	}
	a := r.Atencao[0]
	if len(a.SemOCR) != maxPaginasAtencao || a.SemOCR[0] != 2 || a.SemOCR[maxPaginasAtencao-1] != maxPaginasAtencao+1 {
		t.Errorf("SemOCR com %d números: %v", len(a.SemOCR), a.SemOCR)
	}
	if a.TotalSemOCR != 120 || a.TotalComErro != 0 || a.TotalOCRFalhouComReserva != 0 {
		t.Errorf("totais = %d, %d, %d", a.TotalSemOCR, a.TotalComErro, a.TotalOCRFalhouComReserva)
	}
	if r.Paginas.SemOCR != 120 {
		t.Errorf("Paginas.SemOCR = %d", r.Paginas.SemOCR)
	}
}
