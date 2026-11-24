package etl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"eprocia/internal/models"
)

func TestCategoriaPagina_Tabela(t *testing.T) {
	casos := []struct {
		nome   string
		pagina map[string]any
		quer   string
	}{
		{"erro de leitura", map[string]any{"content": "[ERRO ao ler página: x]", "ocr": false, "erro_leitura": true}, CatComErro},
		{"ocr_erro sem aviso, com marcador", map[string]any{"content": marcadorErroOCR("o OCR não conseguiu ler a página"), "ocr": false, "ocr_erro": "o OCR não conseguiu ler a página"}, CatComErro},
		{"ocr_erro com aviso", map[string]any{"content": "Texto nativo ficticio", "ocr": false, "ocr_erro": "tempo esgotado", "ocr_aviso": true}, CatLidaNativa},
		{"só aviso", map[string]any{"content": "Texto nativo ficticio", "ocr": false, "ocr_aviso": true}, CatLidaNativa},
		{"erro ganha de sem_ocr", map[string]any{"content": "[ERRO ao ler página: x]", "ocr": false, "erro_leitura": true, "sem_ocr": true}, CatComErro},
		{"sem_ocr com marcador", map[string]any{"content": marcadorPaginaSemOCR, "ocr": false, "sem_ocr": true}, CatSemOCR},
		{"sem_ocr ilegível", map[string]any{"content": "[texto ilegível na página: 40 caracteres omitidos]", "ocr": false, "sem_ocr": true}, CatSemOCR},
		{"vazia", map[string]any{"content": "", "ocr": false}, CatVazia},
		{"vazia só com espaços", map[string]any{"content": " \n\t", "ocr": false}, CatVazia},
		{"OCR em branco", map[string]any{"content": "", "ocr": true, "motivo_ocr": "padrao"}, CatVazia},
		{"lida por OCR", map[string]any{"content": "Texto de OCR", "ocr": true, "motivo_ocr": "padrao"}, CatLidaOCR},
		{"nativa", map[string]any{"content": "Texto nativo ficticio", "ocr": false}, CatLidaNativa},
		{"OCR sem ganho", map[string]any{"content": "Texto nativo ficticio", "ocr": false, "motivo_ocr": "isca", "ocr_sem_ganho": true}, CatLidaNativa},
		{"sem content", map[string]any{"ocr": false}, CatVazia},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := categoriaPagina(c.pagina); got != c.quer {
				t.Errorf("categoriaPagina(%v) = %q, quero %q", c.pagina, got, c.quer)
			}
		})
	}
}

func TestMotivoOCR_String(t *testing.T) {
	for m, quer := range map[motivoOCR]string{
		motivoPadrao: "padrao", motivoIlegivel: "ilegivel",
		motivoImagemDescoberta: "imagem_descoberta", motivoIsca: "isca",
	} {
		if got := m.String(); got != quer {
			t.Errorf("motivoOCR(%d).String() = %q, quero %q", int(m), got, quer)
		}
	}
}

// parsearCategorias roda ParsePages num PDF em memória; o caminho em disco é
// fictício (o runner não o lê).
func parsearCategorias(t *testing.T, dados []byte, ocrLigado bool, runner func(context.Context, string, int) (string, error)) map[string]any {
	t.Helper()
	parser := NewPDFParserDeLeitor(bytes.NewReader(dados), int64(len(dados)), "doc.pdf",
		func() (string, error) { return "doc-ficticio.pdf", nil })
	pages, err := parser.ParsePages(context.Background(), ocrLigado, runner, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	return pages
}

func paginaN(t *testing.T, pages map[string]any, n int) map[string]any {
	t.Helper()
	p, ok := pages[fmt.Sprint(n)].(map[string]any)
	if !ok {
		t.Fatalf("página %d ausente: %v", n, pages)
	}
	return p
}

func conferirCategoria(t *testing.T, p map[string]any, quer string) {
	t.Helper()
	if got := categoriaPagina(p); got != quer {
		t.Errorf("categoria = %q, quero %q; página %v", got, quer, p)
	}
}

// pdfPaginaNula: /Count 2 com um só filho, a segunda página é nula.
func pdfPaginaNula() []byte {
	stream := "BT\n/F1 12 Tf\n72 712 Td\n(Texto nativo ficticio da pagina um) Tj\nET"
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 2 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
	})
}

// objetosLeituraQueFalha: a segunda página é o objeto 6 de
// paginaComObjetoTrocado, gravado como "9 0 obj"; a biblioteca entra em
// pânico ao resolvê-lo e a página sai com [ERRO ao ler página.
func objetosLeituraQueFalha() []string {
	objs := paginaComObjetoTrocado("/MediaBox [0 0 595 842]", corpoSintetico(10, 780), tF(72, 300, "DEPOIS"),
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R >>")
	objs[1] = "2 0 obj\n<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>\nendobj"
	return objs
}

func TestCategorias_PorCaminho(t *testing.T) {
	runnerTexto, _ := ocrFalso(ocrFalsoTexto, nil)
	runnerBranco, _ := ocrFalso("", nil)
	runnerFalha, _ := ocrFalso("", errors.New("falha no tesseract embutido"))
	escaneado := buildPDF(objetosPaginasComImagem(1))

	t.Run("texto nativo", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, pdfComTexto(textoLongo("Nativo", 300)), true, runnerTexto), 1)
		conferirCategoria(t, p, CatLidaNativa)
		if _, ok := p["motivo_ocr"]; ok {
			t.Errorf("página nativa com motivo_ocr: %v", p)
		}
	})
	t.Run("escaneado com OCR", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, escaneado, true, runnerTexto), 1)
		conferirCategoria(t, p, CatLidaOCR)
		if p["motivo_ocr"] != "padrao" {
			t.Errorf("motivo_ocr = %v, quero padrao", p["motivo_ocr"])
		}
	})
	t.Run("escaneado sem OCR", func(t *testing.T) {
		conferirCategoria(t, paginaN(t, parsearCategorias(t, escaneado, false, nil), 1), CatSemOCR)
	})
	t.Run("sem texto e sem imagem", func(t *testing.T) {
		conferirCategoria(t, paginaN(t, parsearCategorias(t, pdfComTexto(""), true, runnerTexto), 1), CatVazia)
	})
	t.Run("OCR falha sem nativo", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, escaneado, true, runnerFalha), 1)
		conferirCategoria(t, p, CatComErro)
		if p["motivo_ocr"] != "padrao" {
			t.Errorf("motivo_ocr = %v, quero padrao", p["motivo_ocr"])
		}
	})
	t.Run("OCR falha com nativo", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, pdfTextoCurtoEImagem(), true, runnerFalha), 1)
		conferirCategoria(t, p, CatLidaNativa)
		if p["ocr_aviso"] != true {
			t.Errorf("sem ocr_aviso: %v", p)
		}
		if _, ok := p["ocr_sem_ganho"]; ok {
			t.Errorf("falha de OCR com ocr_sem_ganho: %v", p)
		}
	})
	t.Run("OCR em branco com nativo", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, pdfTextoCurtoEImagem(), true, runnerBranco), 1)
		conferirCategoria(t, p, CatLidaNativa)
		if p["ocr_sem_ganho"] != true {
			t.Errorf("sem ocr_sem_ganho: %v", p)
		}
		if p["motivo_ocr"] != "padrao" {
			t.Errorf("motivo_ocr = %v, quero padrao", p["motivo_ocr"])
		}
	})
	t.Run("OCR com ganho não marca sem ganho", func(t *testing.T) {
		p := paginaN(t, parsearCategorias(t, pdfTextoCurtoEImagem(), true, runnerTexto), 1)
		conferirCategoria(t, p, CatLidaOCR)
		if _, ok := p["ocr_sem_ganho"]; ok {
			t.Errorf("OCR com texto e ocr_sem_ganho: %v", p)
		}
	})
	t.Run("página nula", func(t *testing.T) {
		pages := parsearCategorias(t, pdfPaginaNula(), true, runnerTexto)
		conferirCategoria(t, paginaN(t, pages, 1), CatLidaNativa)
		conferirCategoria(t, paginaN(t, pages, 2), CatVazia)
	})
	t.Run("leitura que falha", func(t *testing.T) {
		pages := parsePagesWithDeadline(t, objetosLeituraQueFalha(), runnerTexto)
		p := paginaN(t, pages, 2)
		if !strings.HasPrefix(conteudoDe(p), "[ERRO ao ler página") {
			t.Fatalf("página 2 sem o marcador de erro: %v", p)
		}
		if p["erro_leitura"] != true {
			t.Errorf("sem erro_leitura: %v", p)
		}
		conferirCategoria(t, p, CatComErro)
	})
	t.Run("Poppler sem texto e OCR desligado", func(t *testing.T) {
		trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) { return []string{""}, nil })
		pages, err := parserPoppler(1).ParsePages(context.Background(), false, nil, nil)
		if err != nil {
			t.Fatalf("ParsePages: %v", err)
		}
		conferirCategoria(t, paginaN(t, pages, 1), CatSemOCR)
	})
	t.Run("Poppler ilegível vai ao OCR com motivo ilegivel", func(t *testing.T) {
		lixo := strings.Repeat("\x01\x02#$%&", 10)
		trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) { return []string{lixo}, nil })
		pages, err := parserPoppler(1).ParsePages(context.Background(), true, runnerTexto, nil)
		if err != nil {
			t.Fatalf("ParsePages: %v", err)
		}
		p := paginaN(t, pages, 1)
		conferirCategoria(t, p, CatLidaOCR)
		if p["motivo_ocr"] != "ilegivel" {
			t.Errorf("motivo_ocr = %v, quero ilegivel", p["motivo_ocr"])
		}
	})
}

func TestCategorias_PoucoTextoSemImagem(t *testing.T) {
	texto := "Despacho ficticio de 30 chars" // 29
	texto += "."
	if len(texto) != 30 {
		t.Fatalf("texto com %d bytes", len(texto))
	}
	runner, chamadas := ocrFalso(ocrFalsoTexto, nil)
	for _, ligado := range []bool{true, false} {
		r := runner
		if !ligado {
			r = nil
		}
		p := paginaN(t, parsearCategorias(t, pdfComTexto(texto), ligado, r), 1)
		if c := categoriaPagina(p); c != CatLidaNativa {
			t.Errorf("OCR ligado=%t: categoria %q, quero %q", ligado, c, CatLidaNativa)
		}
	}
	if n := chamadas.Load(); n != 0 {
		t.Errorf("OCR chamado %d vezes", n)
	}
}

func TestCategorias_RegrasNovasSemOCR(t *testing.T) {
	var vistos = map[motivoOCR]int{}
	for _, ps := range paginasSinteticas() {
		an, err := analisarPagina(primeiraPagina(t, ps.objetos), 1, true, slog.Default())
		if err != nil {
			t.Fatalf("%s: analisarPagina: %v", ps.nome, err)
		}
		if !an.precisaOcr || (an.motivo != motivoImagemDescoberta && an.motivo != motivoIsca) {
			continue
		}
		vistos[an.motivo]++
		t.Run(ps.nome, func(t *testing.T) {
			p := paginaN(t, parsearCategorias(t, buildPDF(ps.objetos), false, nil), 1)
			conferirCategoria(t, p, CatLidaNativa)
		})
	}
	if vistos[motivoImagemDescoberta] == 0 || vistos[motivoIsca] == 0 {
		t.Fatalf("faltou página de uma das regras novas: %v", vistos)
	}
}

// zipCategorias: nativo (2 páginas), escaneado (2), HTML, omitido por tipo,
// não lido (sem texto) e uma cópia do nativo (duplicata H7), com o OCR trocado por um falso que devolve texto.
func zipCategorias(t *testing.T) (string, *models.Config) {
	t.Helper()
	isolarTemp(t)
	antigoPagina, antigoPaginas := executarOCR, executarOCRPaginas
	t.Cleanup(func() { executarOCR, executarOCRPaginas = antigoPagina, antigoPaginas })
	executarOCR = func(context.Context, string, int, string) (string, error) { return ocrFalsoTexto, nil }
	executarOCRPaginas = func(_ context.Context, _ string, paginas []int, _ string, aoConcluir func(int)) ([]string, error) {
		textos := make([]string, len(paginas))
		for i, p := range paginas {
			textos[i] = ocrFalsoTexto
			if aoConcluir != nil {
				aoConcluir(p)
			}
		}
		return textos, nil
	}
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto(textoLongo("Primeira", 200), textoLongo("Segunda", 200))},
		{"Evento 2 - DOC2.pdf", buildPDF(objetosPaginasComImagem(2))},
		{"Evento 3 - EXTR3.pdf", pdfComTexto(textoLongo("Extrato", 200))},
		{"Evento 4 - DOC4.pdf", pdfComTexto("")},
		{"Evento 5 - PET5.html", []byte("<html><body><p>Peticao ficticia do evento cinco</p></body></html>")},
		// Cópia de DOC1: vira duplicata H7 no .txt e mantém a categoria.
		{"Evento 6 - DOC6.pdf", pdfComTexto(textoLongo("Primeira", 200), textoLongo("Segunda", 200))},
	})
	// standard: a pré-passada H7 só roda nos níveis padrão e máximo.
	cfg := &models.Config{CleaningLevel: "standard", OcrMode: "normal", OmitirTipos: []string{"EXTR"}}
	return zipPath, cfg
}

func TestCategorias_UmaPorPagina(t *testing.T) {
	zipPath, cfg := zipCategorias(t)
	var eventos map[string]any
	antigo := gerarSaida
	t.Cleanup(func() { gerarSaida = antigo })
	gerarSaida = func(ev map[string]any, nivel string) (string, int) {
		eventos = ev
		return antigo(ev, nivel)
	}
	saida, _, _, _, err := NewPipeline().ProcessZip(context.Background(), zipPath, cfg, nil)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	txt, err := os.ReadFile(saida)
	if err != nil {
		t.Fatalf("ler saída: %v", err)
	}
	if !strings.Contains(string(txt), "[documento idêntico a: evento 1, ") {
		t.Fatalf("DOC6 não saiu como duplicata de DOC1:\n%s", txt)
	}
	validas := map[string]bool{CatComErro: true, CatSemOCR: true, CatVazia: true, CatLidaOCR: true, CatLidaNativa: true}
	quer := map[string]string{
		"Evento 1 - DOC1.pdf": CatLidaNativa, "Evento 2 - DOC2.pdf": CatLidaOCR, "Evento 5 - PET5.html": CatLidaNativa,
		"Evento 3 - EXTR3.pdf": "", "Evento 4 - DOC4.pdf": "", "Evento 6 - DOC6.pdf": CatLidaNativa,
	}
	vistos := 0
	for _, ev := range eventos {
		for _, dv := range ev.(map[string]any)["documentos"].(map[string]any) {
			d := dv.(map[string]any)
			nome := d["nome"].(string)
			esperado, ok := quer[nome]
			if !ok {
				t.Fatalf("documento inesperado %q", nome)
			}
			vistos++
			_, omitido := d["omitido"]
			_, naoLido := d["nao_lido"]
			if (esperado == "") != (omitido || naoLido) {
				t.Errorf("%s: omitido=%t nao_lido=%t", nome, omitido, naoLido)
			}
			for k, pv := range d["paginas"].(map[string]any) {
				p := pv.(map[string]any)
				c, tem := p["categoria"].(string)
				if esperado == "" {
					if _, ok := p["categoria"]; ok {
						t.Errorf("%s pág. %s: documento fora do total com categoria %v", nome, k, p["categoria"])
					}
					continue
				}
				if !tem || !validas[c] {
					t.Errorf("%s pág. %s: categoria %v", nome, k, p["categoria"])
				}
				if c != esperado {
					t.Errorf("%s pág. %s: categoria %q, quero %q", nome, k, c, esperado)
				}
			}
		}
	}
	if vistos != len(quer) {
		t.Errorf("%d documentos, quero %d", vistos, len(quer))
	}
}

func TestRegistro_PaginaClassificadaSoEmDebug(t *testing.T) {
	zipPath, cfg := zipCategorias(t)

	ctx, buf := ctxComLogJSON(slog.LevelDebug)
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfg, nil, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	linhas := linhasJSON(t, buf.String())
	classificadas := linhasComMsg(linhas, "página classificada")
	// DOC1 (2) + DOC2 (2) + HTML (1) + DOC6 (2); omitido e não lido ficam fora.
	if len(classificadas) != 7 {
		t.Fatalf("página classificada = %d linhas, quero 7:\n%s", len(classificadas), buf.String())
	}
	comMotivo := 0
	for _, l := range classificadas {
		if l["level"] != "DEBUG" {
			t.Errorf("nível %v: %v", l["level"], l)
		}
		if _, ok := l["pag"].(float64); !ok {
			t.Errorf("sem pag: %v", l)
		}
		if _, ok := l["categoria"].(string); !ok {
			t.Errorf("sem categoria: %v", l)
		}
		if _, ok := l["doc_id"]; !ok {
			t.Errorf("sem doc_id: %v", l)
		}
		if l["categoria"] == CatLidaOCR {
			if l["motivo"] != "padrao" {
				t.Errorf("página de OCR sem motivo: %v", l)
			}
			comMotivo++
		} else if _, ok := l["motivo"]; ok {
			t.Errorf("página sem OCR com motivo: %v", l)
		}
	}
	if comMotivo != 2 {
		t.Errorf("%d linhas com motivo, quero 2", comMotivo)
	}
	analisadas := linhasComMsg(linhas, "página analisada")
	// Só as páginas do PDF lidas pela biblioteca: DOC1 (2), DOC2 (2), DOC4 (1), DOC6 (2).
	if len(analisadas) != 7 {
		t.Errorf("página analisada = %d linhas, quero 7", len(analisadas))
	}
	comFracoes := 0
	for _, l := range analisadas {
		if _, ok := l["pag"]; !ok {
			t.Errorf("página analisada sem pag: %v", l)
		}
		bytesNativo, ok := l["bytes_nativo"].(float64)
		if !ok {
			t.Errorf("página analisada sem bytes_nativo: %v", l)
			continue
		}
		// Imagem e desenho só são examinados com texto curto.
		curto := bytesNativo < 100
		for _, campo := range []string{"imagem", "vetorial"} {
			if _, ok := l[campo]; ok != curto {
				t.Errorf("página analisada com %d bytes: %s presente = %t", int(bytesNativo), campo, ok)
			}
		}
		// As frações saem juntas ou nenhuma (só com métricas válidas).
		_, fi := l["fracao_imagem"]
		_, ft := l["fracao_texto"]
		_, fd := l["fracao_descoberta"]
		if fi != ft || ft != fd {
			t.Errorf("frações incompletas: %v", l)
		}
		if fi {
			comFracoes++
		}
	}
	// DOC1, DOC2 e DOC6 têm métricas válidas; DOC4 (texto vazio, vai ao
	// GetTextByRow) não.
	if comFracoes != 6 {
		t.Errorf("%d linhas com frações, quero 6", comFracoes)
	}

	t.Run("juncao na imagem descoberta", func(t *testing.T) {
		ps := paginaSinteticaDe(t, "legenda-imagem-direta")
		zip := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", buildPDF(ps.objetos)}})
		ctx, buf := ctxComLogJSON(slog.LevelDebug)
		if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zip, cfg, nil, nil); err != nil {
			t.Fatalf("ProcessZip: %v", err)
		}
		cl := linhasComMsg(linhasJSON(t, buf.String()), "página classificada")
		if len(cl) != 1 {
			t.Fatalf("página classificada = %d linhas, quero 1:\n%s", len(cl), buf.String())
		}
		if cl[0]["motivo"] != "imagem_descoberta" || cl[0]["categoria"] != CatLidaOCR || cl[0]["juncao"] != true {
			t.Errorf("linha = %v, quero motivo imagem_descoberta, lida_ocr e juncao", cl[0])
		}
		// Nas páginas da regra padrão não há junção.
		for _, l := range classificadas {
			if _, ok := l["juncao"]; ok {
				t.Errorf("juncao fora da imagem descoberta: %v", l)
			}
		}
	})

	zipPath2, cfg2 := zipCategorias(t)
	ctx, buf = ctxComLogJSON(slog.LevelInfo)
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath2, cfg2, nil, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	linhas = linhasJSON(t, buf.String())
	for _, msg := range []string{"página analisada", "página classificada"} {
		if n := len(linhasComMsg(linhas, msg)); n != 0 {
			t.Errorf("%d linhas %q em INFO", n, msg)
		}
	}
}
