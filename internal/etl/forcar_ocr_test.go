package etl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
