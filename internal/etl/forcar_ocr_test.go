package etl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"eprocia/internal/models"
)

// ocrForcadoFalso é um OCR injetado que anota cada página despachada com a
// marca do PDF (lida do arquivo em disco) e responde conforme a marca:
// FALHA dá erro, BRANCO volta vazio, LIXO volta ilegível e o resto volta
// ocrFalsoTexto.
type ocrForcadoFalso struct {
	mu        sync.Mutex
	chamadas  []string // "MARCA:pag"
	marcasPDF []string
}

func (o *ocrForcadoFalso) marcaDe(caminho string) string {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return "SEM_ARQUIVO"
	}
	for _, m := range o.marcasPDF {
		if bytes.Contains(dados, []byte(m)) {
			return m
		}
	}
	return "OUTRO"
}

func (o *ocrForcadoFalso) pagina(_ context.Context, caminho string, pag int, _ string) (string, error) {
	marca := o.marcaDe(caminho)
	o.mu.Lock()
	o.chamadas = append(o.chamadas, fmt.Sprintf("%s:%d", marca, pag))
	o.mu.Unlock()
	switch {
	case strings.HasPrefix(marca, "FALHA"):
		return "", errors.New("falha no tesseract embutido")
	case strings.HasPrefix(marca, "BRANCO"):
		return "", nil
	case strings.HasPrefix(marca, "LIXO"):
		return strings.Repeat("\x01\x02#$%&", 10), nil
	}
	return ocrFalsoTexto, nil
}

func (o *ocrForcadoFalso) instalar(t *testing.T) {
	t.Helper()
	trocarOCR(t, o.pagina, loteDe(o.pagina))
}

func (o *ocrForcadoFalso) lista() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	l := slices.Clone(o.chamadas)
	slices.Sort(l)
	return l
}

// processarCapturando roda o pipeline e devolve o mapa de eventos que chegou ao
// formatter.
func processarCapturando(t *testing.T, zipPath string, cfg *models.Config) map[string]any {
	t.Helper()
	var eventos map[string]any
	antigo := gerarSaida
	t.Cleanup(func() { gerarSaida = antigo })
	gerarSaida = func(ev map[string]any, nivel string, semOCR int) SaidaConsolidada {
		eventos = ev
		return antigo(ev, nivel, semOCR)
	}
	processarComRelatorio(t, zipPath, cfg)
	if eventos == nil {
		t.Fatal("o formatter não recebeu os eventos")
	}
	return eventos
}

// docForcado acha no mapa de eventos o documento cujo nome contém nome.
func docForcado(t *testing.T, eventos map[string]any, nome string) map[string]any {
	t.Helper()
	for _, ev := range eventos {
		docs, _ := ev.(map[string]any)["documentos"].(map[string]any)
		for _, dv := range docs {
			d, _ := dv.(map[string]any)
			if n, _ := d["nome"].(string); strings.Contains(n, nome) {
				return d
			}
		}
	}
	t.Fatalf("documento %s ausente", nome)
	return nil
}

func paginasDe(d map[string]any) map[string]any {
	p, _ := d["paginas"].(map[string]any)
	return p
}

func TestForcarOCR_TodaPaginaDePDFLidoVaiAoOCR(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"FORCADO", "OMITIDO"}}
	o.instalar(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("FORCADO um", 300), textoLongo("FORCADO dois", 300), textoLongo("FORCADO tres", 300))},
		{"Evento 2 - PET2.html", []byte("<html><body><p>Peticao ficticia do evento dois de FULANO DE TAL</p></body></html>")},
		{"Evento 3 - EXTR3.pdf", pdfComTexto(textoLongo("OMITIDO", 300))},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", OmitirTipos: []string{"EXTR"}, ForcarOCR: true}
	eventos := processarCapturando(t, zipPath, cfg)

	if got, quer := o.lista(), []string{"FORCADO:1", "FORCADO:2", "FORCADO:3"}; !slices.Equal(got, quer) {
		t.Errorf("páginas despachadas = %v, quero %v", got, quer)
	}
	for k, pv := range paginasDe(docForcado(t, eventos, "DOC1")) {
		p := pv.(map[string]any)
		if p["motivo_ocr"] != "forcado" {
			t.Errorf("DOC1 pág. %s: motivo_ocr = %v, quero forcado", k, p["motivo_ocr"])
		}
		if p["categoria"] != CatLidaOCR {
			t.Errorf("DOC1 pág. %s: categoria = %v, quero %s", k, p["categoria"], CatLidaOCR)
		}
	}
	for _, nome := range []string{"PET2", "EXTR3"} {
		for k, pv := range paginasDe(docForcado(t, eventos, nome)) {
			if m, ok := pv.(map[string]any)["motivo_ocr"]; ok {
				t.Errorf("%s pág. %s com motivo_ocr %v", nome, k, m)
			}
		}
	}
}

func TestForcarOCR_ResultadoSegueMotivoPadrao(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"LIDO", "BRANCO", "LIXO", "FALHA"}}
	o.instalar(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("LIDO um", 300))},
		{"Evento 2 - DOC2.pdf", pdfComTexto(textoLongo("BRANCO dois", 300))},
		{"Evento 3 - DOC3.pdf", pdfComTexto(textoLongo("LIXO tres", 300))},
		{"Evento 4 - DOC4.pdf", pdfComTexto(textoLongo("FALHA quatro", 300))},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", ForcarOCR: true}
	eventos := processarCapturando(t, zipPath, cfg)

	if got := len(o.lista()); got != 4 {
		t.Fatalf("despachadas %d páginas, quero 4: %v", got, o.lista())
	}
	pag := func(nome string) map[string]any {
		t.Helper()
		d := docForcado(t, eventos, nome)
		if nl, ok := d["nao_lido"]; ok {
			t.Errorf("%s não lido: %v", nome, nl)
		}
		p, ok := paginasDe(d)["1"].(map[string]any)
		if !ok {
			t.Fatalf("%s sem página 1: %v", nome, d)
		}
		if p["motivo_ocr"] != "forcado" {
			t.Errorf("%s: motivo_ocr = %v, quero forcado", nome, p["motivo_ocr"])
		}
		return p
	}

	if p := pag("DOC1"); p["categoria"] != CatLidaOCR || conteudoDe(p) != ocrFalsoTexto {
		t.Errorf("OCR legível não substituiu o nativo: %v", p)
	}
	for _, nome := range []string{"DOC2", "DOC3"} {
		p := pag(nome)
		if p["categoria"] != CatLidaNativa || p["ocr_sem_ganho"] != true || !strings.Contains(conteudoDe(p), "FULANO DE TAL") {
			t.Errorf("%s: OCR em branco ou ilegível não manteve o nativo: %v", nome, p)
		}
	}
	p := pag("DOC4")
	if p["ocr_aviso"] != true || !strings.Contains(conteudoDe(p), "FULANO DE TAL") {
		t.Errorf("falha do OCR forçado não manteve o nativo com ocr_aviso: %v", p)
	}
	if _, ok := p["ocr_erro"]; ok {
		t.Errorf("falha do OCR forçado com ocr_erro: %v", p)
	}
	if p["categoria"] != CatLidaNativa {
		t.Errorf("DOC4: categoria = %v, quero %s", p["categoria"], CatLidaNativa)
	}
}

func TestForcarOCR_PrecedenciaDoMotivo(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"FALHA"}}
	o.instalar(t)
	// O OCR falso reconhece o PDF pela marca; o escaneado não tem texto, então
	// a marca vai num objeto solto (o 6, depois dos 5 do escaneado).
	escaneado := buildPDF(append(objetosPaginasComImagem(1), "6 0 obj\n(FALHA)\nendobj"))
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", escaneado},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", ForcarOCR: true}
	eventos := processarCapturando(t, zipPath, cfg)

	p, ok := paginasDe(docForcado(t, eventos, "DOC1"))["1"].(map[string]any)
	if !ok {
		t.Fatal("DOC1 sem página 1")
	}
	if p["motivo_ocr"] != "padrao" {
		t.Errorf("motivo_ocr = %v, quero padrao (o forçar não troca o motivo)", p["motivo_ocr"])
	}
	if _, ok := p["ocr_erro"]; !ok {
		t.Errorf("falha sem nativo no motivo padrão sem ocr_erro: %v", p)
	}
}

// parsearForcado roda ParsePages com Forçar OCR num PDF em memória; o caminho
// em disco é fictício (o runner não o lê).
func parsearForcado(t *testing.T, dados []byte, runner func(context.Context, string, int) (string, error)) map[string]any {
	t.Helper()
	parser := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf",
		func() (string, error) { return "doc-ficticio.pdf", nil }).comForcarOCR(true)
	pages, err := parser.ParsePages(context.Background(), true, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	return pages
}

// runnerPaginas conta as páginas despachadas e devolve texto.
type runnerPaginas struct {
	mu       sync.Mutex
	paginas  []int
	resposta string
}

func (r *runnerPaginas) run(_ context.Context, _ string, pag int) (string, error) {
	r.mu.Lock()
	r.paginas = append(r.paginas, pag)
	r.mu.Unlock()
	return r.resposta, nil
}

func TestForcarOCR_PaginasSemConteudo(t *testing.T) {
	t.Run("página em branco vai ao OCR e sai vazia", func(t *testing.T) {
		r := &runnerPaginas{}
		p := paginaN(t, parsearForcado(t, pdfComTexto(""), r.run), 1)
		if !slices.Equal(r.paginas, []int{1}) {
			t.Errorf("despachadas %v, quero [1]", r.paginas)
		}
		if p["motivo_ocr"] != "forcado" {
			t.Errorf("motivo_ocr = %v, quero forcado", p["motivo_ocr"])
		}
		conferirCategoria(t, p, CatVazia)
	})
	t.Run("página nula não é despachada", func(t *testing.T) {
		r := &runnerPaginas{resposta: ocrFalsoTexto}
		pages := parsearForcado(t, pdfPaginaNula(), r.run)
		if slices.Contains(r.paginas, 2) {
			t.Errorf("página nula despachada: %v", r.paginas)
		}
		p := paginaN(t, pages, 2)
		if _, ok := p["motivo_ocr"]; ok {
			t.Errorf("página nula com motivo_ocr: %v", p)
		}
		conferirCategoria(t, p, CatVazia)
	})
	t.Run("página com erro de leitura não é despachada", func(t *testing.T) {
		r := &runnerPaginas{resposta: ocrFalsoTexto}
		pages := parsearForcado(t, buildPDF(objetosLeituraQueFalha()), r.run)
		if slices.Contains(r.paginas, 2) {
			t.Errorf("página com erro despachada: %v", r.paginas)
		}
		p := paginaN(t, pages, 2)
		if _, ok := p["motivo_ocr"]; ok {
			t.Errorf("página com erro com motivo_ocr: %v", p)
		}
		conferirCategoria(t, p, CatComErro)
	})
}

func TestForcarOCR_NoneIgnora(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"FORCADO"}}
	o.instalar(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("FORCADO um", 300), textoLongo("FORCADO dois", 300))},
	})
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "none", ForcarOCR: true}
	eventos := processarCapturando(t, zipPath, cfg)
	if got := o.lista(); len(got) != 0 {
		t.Errorf("com ocr_mode none foram despachadas %v", got)
	}
	for k, pv := range paginasDe(docForcado(t, eventos, "DOC1")) {
		p := pv.(map[string]any)
		if _, ok := p["motivo_ocr"]; ok || p["categoria"] != CatLidaNativa {
			t.Errorf("pág. %s: %v", k, p)
		}
	}
	if !cfg.ForcarOCR || cfg.OcrMode != "none" {
		t.Errorf("o pipeline mudou a config: %+v", cfg)
	}
}

func TestForcarOCR_Poppler(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return []string{textoPoppler(150, "Primeira pagina ficticia"), ""}, nil
	})
	var r runnerContado
	pages, err := parserPoppler(2).comForcarOCR(true).ParsePages(context.Background(), true, r.run, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	slices.Sort(r.paginas)
	if !slices.Equal(r.paginas, []int{1, 2}) {
		t.Errorf("despachadas %v, quero [1 2]", r.paginas)
	}
	if p := paginaPoppler(t, pages, "1"); p["motivo_ocr"] != "forcado" {
		t.Errorf("pág. 1 (texto longo): motivo_ocr = %v, quero forcado", p["motivo_ocr"])
	}
	if p := paginaPoppler(t, pages, "2"); p["motivo_ocr"] != "padrao" {
		t.Errorf("pág. 2 (sem texto): motivo_ocr = %v, quero padrao", p["motivo_ocr"])
	}
	// Sem forçar, a de texto longo fica com o nativo.
	var r2 runnerContado
	if _, err := parserPoppler(2).ParsePages(context.Background(), true, r2.run, nil); err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if !slices.Equal(r2.paginas, []int{2}) {
		t.Errorf("sem forçar, despachadas %v, quero [2]", r2.paginas)
	}
}

// pdfEmBrancoComMarca monta um PDF de uma página sem texto e sem imagem (fora
// do OCR sem forçar), com a marca num objeto solto para o OCR falso.
func pdfEmBrancoComMarca(marca string) []byte {
	stream := "BT\n/F1 12 Tf\n72 712 Td\n() Tj\nET"
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
		fmt.Sprintf("5 0 obj\n(%s)\nendobj", marca),
	})
}

// A página que só foi ao OCR por Forçar OCR nunca fica pior que sem forçar:
// com o OCR falho, em branco ou ilegível, fica o nativo (mesmo vazio), sem
// ocr_erro, na categoria que teria sem forçar, e o documento não muda de
// leitura.
func TestForcarOCR_NuncaPioraAPagina(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"FALHA", "LIXO", "BRANCO"}}
	o.instalar(t)
	entradas := []entradaZip{
		// Página em branco e página de texto, as duas com o OCR falhando.
		{"Evento 1 - DOC1.pdf", pdfComTexto("", textoLongo("FALHA um", 300))},
		// Documento de uma página em branco, OCR falhando.
		{"Evento 2 - DOC2.pdf", pdfEmBrancoComMarca("FALHA dois")},
		// Página em branco com OCR ilegível e com OCR em branco.
		{"Evento 3 - DOC3.pdf", pdfEmBrancoComMarca("LIXO tres")},
		{"Evento 4 - DOC4.pdf", pdfEmBrancoComMarca("BRANCO quatro")},
	}
	zipPath := gravarZip(t, entradas)
	forcado := processarCapturando(t, zipPath, &models.Config{CleaningLevel: "standard", OcrMode: "normal", ForcarOCR: true})
	if got := len(o.lista()); got != 5 {
		t.Fatalf("despachadas %d páginas, quero 5: %v", got, o.lista())
	}
	semForcar := processarCapturando(t, gravarZip(t, entradas), &models.Config{CleaningLevel: "standard", OcrMode: "normal"})

	doc1 := docForcado(t, forcado, "DOC1")
	if nl, ok := doc1["nao_lido"]; ok {
		t.Errorf("DOC1 não lido: %v", nl)
	}
	p := paginaN(t, paginasDe(doc1), 1)
	if _, ok := p["ocr_erro"]; ok {
		t.Errorf("página em branco forçada com ocr_erro: %v", p)
	}
	if p["ocr_aviso"] != true || p["motivo_ocr"] != "forcado" || conteudoDe(p) != "" {
		t.Errorf("página em branco forçada com OCR falho: %v", p)
	}
	if p["categoria"] != CatVazia {
		t.Errorf("categoria = %v, quero %s", p["categoria"], CatVazia)
	}

	for _, nome := range []string{"DOC2", "DOC3", "DOC4"} {
		f, s := docForcado(t, forcado, nome), docForcado(t, semForcar, nome)
		if f["nao_lido"] != s["nao_lido"] {
			t.Errorf("%s: nao_lido forçado = %v, sem forçar = %v", nome, f["nao_lido"], s["nao_lido"])
		}
		pf, ps := paginaN(t, paginasDe(f), 1), paginaN(t, paginasDe(s), 1)
		if pf["categoria"] != ps["categoria"] || conteudoDe(pf) != conteudoDe(ps) {
			t.Errorf("%s: forçado %v, sem forçar %v", nome, pf, ps)
		}
		if _, ok := pf["ocr_erro"]; ok {
			t.Errorf("%s com ocr_erro: %v", nome, pf)
		}
		if pf["ocr"] != false {
			t.Errorf("%s: ocr = %v, quero false (ficou o nativo)", nome, pf["ocr"])
		}
	}
	if got := docForcado(t, forcado, "DOC2")["nao_lido"]; got != LeituraSemTexto.Frase() {
		t.Errorf("DOC2: nao_lido = %v, quero %q", got, LeituraSemTexto.Frase())
	}
}

func TestForcarOCR_LogZipIniciadoValorEfetivo(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{}
	o.instalar(t)
	for _, c := range []struct {
		modo           string
		efetivo, pedido bool
	}{
		{"none", false, true},
		{"normal", true, true},
	} {
		t.Run(c.modo, func(t *testing.T) {
			zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("Log", 300))}})
			ctx, buf := ctxComLogJSON(slog.LevelInfo)
			cfg := &models.Config{CleaningLevel: "none", OcrMode: c.modo, ForcarOCR: true}
			if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfg, nil, nil); err != nil {
				t.Fatalf("ProcessZip: %v", err)
			}
			ini := linhasComMsg(linhasJSON(t, buf.String()), "zip iniciado")
			if len(ini) != 1 {
				t.Fatalf("zip iniciado = %d linhas:\n%s", len(ini), buf.String())
			}
			g, _ := ini[0]["config"].(map[string]any)
			if g["forcar_ocr"] != c.efetivo || g["forcar_ocr_pedido"] != c.pedido {
				t.Errorf("config = %v; quero forcar_ocr=%t forcar_ocr_pedido=%t", g, c.efetivo, c.pedido)
			}
		})
	}
}

// O relatório do ZIP com Forçar OCR: por_motivo conta "forcado" à parte do
// motivo padrão, e a página forçada cujo OCR falhou conta como reserva
// (ocr_falhou_com_reserva) mesmo com o nativo vazio; o OCR em branco conta
// como sem ganho.
func TestForcarOCR_RelatorioDoZip(t *testing.T) {
	isolarTemp(t)
	o := &ocrForcadoFalso{marcasPDF: []string{"FALHA", "BRANCO", "LIDO"}}
	o.instalar(t)
	zipPath := gravarZip(t, []entradaZip{
		// Em branco e texto, o OCR falha nas duas.
		{"Evento 1 - DOC1.pdf", pdfComTexto("", textoLongo("FALHA um", 300))},
		// Texto, OCR legível.
		{"Evento 2 - DOC2.pdf", pdfComTexto(textoLongo("LIDO dois", 300))},
		// Escaneado: motivo padrão, OCR legível.
		{"Evento 3 - DOC3.pdf", buildPDF(append(objetosPaginasComImagem(1), "6 0 obj\n(LIDO)\nendobj"))},
		// Em branco e texto, o OCR volta em branco nas duas.
		{"Evento 4 - DOC4.pdf", pdfComTexto("", textoLongo("BRANCO quatro", 300))},
	})
	r, _ := processarComRelatorio(t, zipPath, &models.Config{CleaningLevel: "standard", OcrMode: "normal", ForcarOCR: true})

	p := r.Paginas
	if p.PorMotivo["forcado"] != 5 || p.PorMotivo["padrao"] != 1 || len(p.PorMotivo) != 2 {
		t.Errorf("PorMotivo = %v; quero forcado 5 e padrao 1", p.PorMotivo)
	}
	if p.Total != 6 || p.LidaOCR != 2 || p.Vazia != 2 || p.LidaNativa != 2 || p.ComErro != 0 || p.SemOCR != 0 {
		t.Errorf("categorias = %+v; quero total 6: 2 OCR, 2 vazias, 2 nativas", p)
	}
	if p.OCRFalhouComReserva != 2 {
		t.Errorf("OCRFalhouComReserva = %d, quero 2 (a em branco forçada conta)", p.OCRFalhouComReserva)
	}
	if p.OCRSemGanho != 2 {
		t.Errorf("OCRSemGanho = %d, quero 2", p.OCRSemGanho)
	}
	if r.Documentos.Lidos != 4 || r.Documentos.NaoLidos != 0 {
		t.Errorf("documentos = %+v; quero 4 lidos", r.Documentos)
	}
	var doc1 *models.DocAtencao
	for i := range r.Atencao {
		if strings.Contains(r.Atencao[i].Nome, "DOC1") {
			doc1 = &r.Atencao[i]
		} else {
			t.Errorf("documento em Atencao sem caso de atenção: %+v", r.Atencao[i])
		}
	}
	if doc1 == nil {
		t.Fatalf("DOC1 fora de Atencao: %+v", r.Atencao)
	}
	if !slices.Equal(doc1.OCRFalhouComReserva, []int{1, 2}) || doc1.TotalOCRFalhouComReserva != 2 || len(doc1.ComErro) != 0 {
		t.Errorf("Atencao DOC1 = %+v; quero ocr_falhou_com_reserva [1 2]", *doc1)
	}
}
