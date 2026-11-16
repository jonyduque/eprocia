package etl

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

const ocrFalsoTexto = "TEXTO LIDO PELO OCR linha um\nTEXTO LIDO PELO OCR linha dois"

// ocrFalso devolve texto fixo (ou erro) e conta as chamadas.
func ocrFalso(texto string, err error) (func(context.Context, string, int) (string, error), *atomic.Int32) {
	var n atomic.Int32
	return func(context.Context, string, int) (string, error) {
		n.Add(1)
		return texto, err
	}, &n
}

func paginaUm(t *testing.T, pages map[string]any) map[string]any {
	t.Helper()
	p, ok := pages["1"].(map[string]any)
	if !ok {
		t.Fatalf("página 1 ausente: %v", pages)
	}
	return p
}

func conteudoDe(p map[string]any) string {
	c, _ := p["content"].(string)
	return c
}

// ehMosaicoCurta: a legenda tem menos de 100 bytes e a página é desenhada em
// vetores, então vai ao OCR pela regra antiga, que substitui o texto nativo.
func ehMosaicoCurta(p paginaSintetica) bool { return p.nome == "mosaico-legenda-curta" }

func TestConteudoPerdido_ControlesNaoVaoAoOCR(t *testing.T) {
	for _, p := range paginasSinteticas() {
		if p.isca || p.candidataImagem {
			continue
		}
		t.Run(p.nome, func(t *testing.T) {
			runner, n := ocrFalso(ocrFalsoTexto, nil)
			pages := parsePagesWithDeadline(t, p.objetos, runner)
			pg := paginaUm(t, pages)
			if n.Load() != 0 {
				t.Errorf("OCR chamado %d vez(es)", n.Load())
			}
			if quer := extrairComPrazo(t, p.objetos); conteudoDe(pg) != quer {
				t.Errorf("conteúdo = %q, quer %q", conteudoDe(pg), quer)
			}
			if o, _ := pg["ocr"].(bool); o {
				t.Errorf("ocr = true")
			}
		})
	}
}

func TestConteudoPerdido_ImagemDescobertaJuntaOCRENativo(t *testing.T) {
	for _, p := range paginasSinteticas() {
		if !p.candidataImagem || p.isca || ehMosaicoCurta(p) { // mosaico-legenda-curta: regra antiga, ver MosaicoLegendaCurta
			continue
		}
		t.Run(p.nome, func(t *testing.T) {
			runner, n := ocrFalso(ocrFalsoTexto, nil)
			pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
			if n.Load() != 1 {
				t.Fatalf("OCR chamado %d vez(es), quer 1", n.Load())
			}
			if o, _ := pg["ocr"].(bool); !o {
				t.Errorf("ocr = false")
			}
			nativo := extrairComPrazo(t, p.objetos)
			quer := ocrFalsoTexto + "\n\n" + nativo
			if p.nome == "legenda-acima-imagem" {
				quer = nativo + "\n\n" + ocrFalsoTexto
			}
			if conteudoDe(pg) != quer {
				t.Errorf("conteúdo = %q, quer %q", conteudoDe(pg), quer)
			}
		})
	}
}

func TestConteudoPerdido_LinhaQueFaltaVaiParaJuntoDaVizinha(t *testing.T) {
	p := paginaSinteticaDe(t, "legenda-acima-imagem")
	linhas := strings.Split(extrairComPrazo(t, p.objetos), "\n")
	if len(linhas) < 2 {
		t.Fatalf("legenda com %d linha(s)", len(linhas))
	}
	runner, _ := ocrFalso(linhas[0]+"\nCONTEUDO DA FIGURA", nil)
	pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
	quer := linhas[0] + "\n" + linhas[1] + "\nCONTEUDO DA FIGURA"
	if conteudoDe(pg) != quer {
		t.Errorf("conteúdo = %q, quer %q", conteudoDe(pg), quer)
	}
}

func TestConteudoPerdido_TimbreTresLinhasMantemONativo(t *testing.T) {
	p := paginaSinteticaDe(t, "timbre-3-linhas")
	runner, n := ocrFalso(ocrFalsoTexto, nil)
	pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
	if n.Load() != 1 {
		t.Fatalf("OCR chamado %d vez(es), quer 1 (falso positivo documentado)", n.Load())
	}
	c := conteudoDe(pg)
	for _, l := range []string{"Termos em que pede deferimento.", "Cidade Exemplo, 01 de fevereiro de 2026.", "FULANO DE TAL - OAB/SP 000.000"} {
		if !strings.Contains(c, l) {
			t.Errorf("linha nativa %q ausente em %q", l, c)
		}
	}
}

func TestConteudoPerdido_IscaTrocaPeloOCR(t *testing.T) {
	palavras := palavrasIsca(80, 42)
	for _, p := range paginasSinteticas() {
		if !p.isca {
			continue
		}
		t.Run(p.nome, func(t *testing.T) {
			runner, n := ocrFalso(ocrFalsoTexto, nil)
			pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
			if n.Load() != 1 {
				t.Fatalf("OCR chamado %d vez(es), quer 1", n.Load())
			}
			c := conteudoDe(pg)
			if c != ocrFalsoTexto {
				t.Errorf("conteúdo = %q, quer o texto do OCR", c)
			}
			for _, w := range palavras {
				if strings.Contains(c, w) {
					t.Fatalf("palavra da isca %q no conteúdo", w)
				}
			}
		})
	}
}

// paginasComNativoEOCR percorre as páginas que vão ao OCR e mantêm o nativo.
func paginasComNativoEOCR() []paginaSintetica {
	var r []paginaSintetica
	for _, p := range paginasSinteticas() {
		if (p.isca || p.candidataImagem) && !ehMosaicoCurta(p) { // mosaico-legenda-curta: regra antiga, o OCR substitui o nativo
			r = append(r, p)
		}
	}
	return r
}

func TestConteudoPerdido_OCREmBrancoMantemONativo(t *testing.T) {
	for _, p := range paginasComNativoEOCR() {
		t.Run(p.nome, func(t *testing.T) {
			runner, _ := ocrFalso("\n\f", nil)
			pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
			if conteudoDe(pg) != extrairComPrazo(t, p.objetos) {
				t.Errorf("conteúdo = %q, quer o nativo", conteudoDe(pg))
			}
			if o, _ := pg["ocr"].(bool); o {
				t.Errorf("ocr = true")
			}
			if _, tem := pg["ocr_erro"]; tem {
				t.Errorf("ocr_erro presente: %v", pg["ocr_erro"])
			}
		})
	}
}

func TestConteudoPerdido_OCRComErroMantemONativo(t *testing.T) {
	for _, p := range paginasComNativoEOCR() {
		t.Run(p.nome, func(t *testing.T) {
			runner, _ := ocrFalso("", errors.New("falha simulada"))
			pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
			if conteudoDe(pg) != extrairComPrazo(t, p.objetos) {
				t.Errorf("conteúdo = %q, quer o nativo", conteudoDe(pg))
			}
			if e, _ := pg["ocr_erro"].(string); e == "" {
				t.Errorf("ocr_erro vazio")
			}
		})
	}
}

func TestConteudoPerdido_OCRDesligado(t *testing.T) {
	for _, p := range paginasComNativoEOCR() {
		t.Run(p.nome, func(t *testing.T) {
			pages := parsearSemOCR(t, buildPDF(p.objetos), false)
			c := conteudoDe(paginaUm(t, pages))
			if c != extrairComPrazo(t, p.objetos) {
				t.Errorf("conteúdo = %q, quer o nativo", c)
			}
			if strings.Contains(c, "[página digitalizada") {
				t.Errorf("conteúdo tem marcador de página digitalizada: %q", c)
			}
			if l := classificarPaginas(pages, false); l != LeituraLido {
				t.Errorf("leitura = %v, quer LeituraLido", l)
			}
		})
	}
}

func TestConteudoPerdido_CorpoEmFormSemOCR(t *testing.T) {
	runner, n := ocrFalso(ocrFalsoTexto, nil)
	c := conteudoDe(paginaUm(t, parsePagesWithDeadline(t, objsCorpoEmForm(), runner)))
	if n.Load() != 0 {
		t.Errorf("OCR chamado %d vez(es)", n.Load())
	}
	for _, l := range []string{"PRIMEIRA LINHA DO CORPO", "SEGUNDA LINHA DO CORPO", "TERCEIRA LINHA DO CORPO", "Legenda da figura"} {
		if !strings.Contains(c, l) {
			t.Errorf("%q ausente em %q", l, c)
		}
	}
}

// A legenda tem menos de 100 bytes e a página é desenhada em vetores: vai ao
// OCR pela regra antiga (motivoPadrao), que substitui o texto nativo.
func TestConteudoPerdido_MosaicoLegendaCurta(t *testing.T) {
	p := paginaSinteticaDe(t, "mosaico-legenda-curta")
	runner, n := ocrFalso(ocrFalsoTexto, nil)
	pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
	if n.Load() != 1 {
		t.Fatalf("OCR chamado %d vez(es), quer 1", n.Load())
	}
	if o, _ := pg["ocr"].(bool); !o {
		t.Errorf("ocr = false")
	}
	if conteudoDe(pg) != ocrFalsoTexto {
		t.Errorf("conteúdo = %q, quer o texto do OCR", conteudoDe(pg))
	}
}
