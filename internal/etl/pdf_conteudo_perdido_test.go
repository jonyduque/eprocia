package etl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"eprocia/internal/ocr"
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

// sinteticasOnde filtra paginasSinteticas() e reprova se sobrarem menos de
// minimo: um filtro que deixa de casar faria o laço do teste passar vazio.
func sinteticasOnde(t *testing.T, minimo int, filtro func(paginaSintetica) bool) []paginaSintetica {
	t.Helper()
	var r []paginaSintetica
	for _, p := range paginasSinteticas() {
		if filtro(p) {
			r = append(r, p)
		}
	}
	if len(r) < minimo {
		t.Fatalf("filtro selecionou %d página(s) sintética(s), quer ao menos %d", len(r), minimo)
	}
	return r
}

func TestConteudoPerdido_ControlesNaoVaoAoOCR(t *testing.T) {
	for _, p := range sinteticasOnde(t, 11, func(p paginaSintetica) bool { return !p.isca && !p.candidataImagem }) {
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
	// mosaico-legenda-curta: regra antiga, ver MosaicoLegendaCurta
	for _, p := range sinteticasOnde(t, 7, func(p paginaSintetica) bool { return p.candidataImagem && !p.isca && !ehMosaicoCurta(p) }) {
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
	for _, p := range sinteticasOnde(t, 3, func(p paginaSintetica) bool { return p.isca }) {
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
		})
	}
}

// paginasComNativoEOCR percorre as páginas que vão ao OCR e mantêm o nativo.
// mosaico-legenda-curta fica de fora: regra antiga, o OCR substitui o nativo.
func paginasComNativoEOCR(t *testing.T, minimo int) []paginaSintetica {
	t.Helper()
	return sinteticasOnde(t, minimo, func(p paginaSintetica) bool { return (p.isca || p.candidataImagem) && !ehMosaicoCurta(p) })
}

func TestConteudoPerdido_OCREmBrancoMantemONativo(t *testing.T) {
	for _, p := range paginasComNativoEOCR(t, 10) {
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

// errosOCR são as duas classes de falha do OCR: erro comum e prazo esgotado.
var errosOCR = []struct {
	nome string
	err  error
}{
	{"erro", errors.New("falha simulada")},
	{"prazo", fmt.Errorf("%w: simulado", ocr.ErrPrazo)},
}

// Ruling 4 e 5: com a reserva nativa, a falha do OCR numa página das regras
// novas grava o texto nativo sem ocr_erro nem ocr_prazo (a página conta como
// lida) e com ocr_aviso, que faz o formatter pôr o aviso antes do texto.
func TestConteudoPerdido_OCRComErroMantemONativo(t *testing.T) {
	for _, p := range paginasComNativoEOCR(t, 10) {
		for _, e := range errosOCR {
			t.Run(p.nome+"/"+e.nome, func(t *testing.T) {
				runner, _ := ocrFalso("", e.err)
				pg := paginaUm(t, parsePagesWithDeadline(t, p.objetos, runner))
				if conteudoDe(pg) != extrairComPrazo(t, p.objetos) {
					t.Errorf("conteúdo = %q, quer o nativo", conteudoDe(pg))
				}
				if o, _ := pg["ocr"].(bool); o {
					t.Errorf("ocr = true")
				}
				for _, k := range []string{"ocr_erro", "ocr_prazo"} {
					if v, tem := pg[k]; tem {
						t.Errorf("%s presente: %v", k, v)
					}
				}
				if av, _ := pg["ocr_aviso"].(bool); !av {
					t.Errorf("ocr_aviso ausente: %v", pg)
				}
			})
		}
	}
}

// Ponta a ponta: o documento de uma página das regras novas cujo OCR falhou
// sai lido, com o aviso antes do texto nativo.
func TestConteudoPerdido_OCRComErroSaiComAviso(t *testing.T) {
	p := paginaSinteticaDe(t, "legenda-imagem-direta")
	runner, _ := ocrFalso("", errosOCR[0].err)
	pages := parsePagesWithDeadline(t, p.objetos, runner)
	doc := map[string]any{"index": 1, "nome": "Evento 1 - A.pdf", "paginas": pages, "total_paginas": 1}
	saida, _ := GenerateConsolidatedOutput(eventoNL(doc), NivelNenhum)
	if !strings.Contains(saida, "nome=\"Evento 1 - A.pdf\">\n"+avisoOCRComReserva+"\n") {
		t.Errorf("saída sem o aviso logo depois da abertura do documento:\n%s", saida)
	}
	if !strings.Contains(saida, "Documentos não lidos: 0\n") {
		t.Errorf("documento contou como não lido:\n%s", saida)
	}
}

// No nível do documento: a falha do OCR numa página das regras novas não deixa
// o documento como não lido.
func TestConteudoPerdido_OCRComErroDocumentoLido(t *testing.T) {
	for _, nome := range []string{"legenda-imagem-direta", "isca-preta"} {
		p := paginaSinteticaDe(t, nome)
		for _, e := range errosOCR {
			t.Run(nome+"/"+e.nome, func(t *testing.T) {
				runner, n := ocrFalso("", e.err)
				pages := parsePagesWithDeadline(t, p.objetos, runner)
				if n.Load() == 0 {
					t.Fatalf("OCR não foi chamado")
				}
				l := classificarPaginas(pages, false)
				if l != LeituraLido {
					t.Errorf("leitura = %q, quer %q", l, LeituraLido)
				}
				if l.NaoLido() {
					t.Errorf("NaoLido() = true")
				}
				if c := conteudoDe(paginaUm(t, pages)); c != extrairComPrazo(t, p.objetos) {
					t.Errorf("conteúdo = %q, quer o nativo", c)
				}
			})
		}
	}
}

// Controle: a regra antiga (motivoPadrao, reserva curta) continua marcando a
// falha do OCR.
func TestConteudoPerdido_OCRComErroRegraAntigaMarca(t *testing.T) {
	for _, e := range errosOCR {
		t.Run(e.nome, func(t *testing.T) {
			runner, _ := ocrFalso("", e.err)
			pg := paginaUm(t, parsePagesWithDeadline(t, objetosTextoCurtoEImagem(), runner))
			if c := conteudoDe(pg); !strings.Contains(c, "Texto nativo curto ficticio") {
				t.Errorf("conteúdo = %q, quer a reserva", c)
			}
			if f, _ := pg["ocr_erro"].(string); f == "" {
				t.Errorf("ocr_erro ausente: %v", pg)
			}
			if _, tem := pg["ocr_prazo"]; tem != (e.nome == "prazo") {
				t.Errorf("ocr_prazo presente = %t, quer %t", tem, e.nome == "prazo")
			}
			if av, _ := pg["ocr_aviso"].(bool); !av {
				t.Errorf("ocr_aviso ausente: %v", pg)
			}
		})
	}
}

func TestConteudoPerdido_OCRDesligado(t *testing.T) {
	for _, p := range paginasComNativoEOCR(t, 10) {
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
