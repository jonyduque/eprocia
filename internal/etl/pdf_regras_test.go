package etl

import (
	"log/slog"
	"strings"
	"testing"
)

func TestRegraNova(t *testing.T) {
	isca := contagemLingua{palavras: 80, implausiveis: 66, funcionais: 0}
	normal := contagemLingua{palavras: 200, implausiveis: 0, funcionais: 60}
	m := func(valida bool, fImg, fTxt, fDesc float64) metricasPagina {
		return metricasPagina{valida: valida, fImg: fImg, fTxt: fTxt, fDesc: fDesc}
	}
	casos := []struct {
		nome     string
		m        metricasPagina
		c        contagemLingua
		motivo   motivoOCR
		esperado bool
	}{
		{"descoberta cheia", m(true, 0.96, 0.02, 0.95), normal, motivoImagemDescoberta, true},
		{"descoberta nos limites", m(true, 0.40, 0.10, 0.30), normal, motivoImagemDescoberta, true},
		{"fDesc abaixo", m(true, 0.40, 0.10, 0.29), normal, 0, false},
		{"fTxt acima", m(true, 0.40, 0.11, 0.35), normal, 0, false},
		{"isca no limite de fImg", m(true, 0.10, 0.50, 0.02), isca, motivoIsca, true},
		{"isca sem imagem bastante", m(true, 0.09, 0.50, 0.02), isca, 0, false},
		{"as duas valem, vale a isca", m(true, 0.90, 0.05, 0.80), isca, motivoIsca, true},
		{"métricas inválidas", m(false, 0.96, 0.02, 0.95), isca, 0, false},
		{"timbre com texto", m(true, 1.00, 0.55, 0.39), normal, 0, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			motivo, ok := regraNova(c.m, c.c)
			if ok != c.esperado {
				t.Fatalf("regraNova(%+v, %+v) = (%v, %t), esperado ok = %t", c.m, c.c, motivo, ok, c.esperado)
			}
			if ok && motivo != c.motivo {
				t.Fatalf("regraNova(%+v, %+v): motivo %v, esperado %v", c.m, c.c, motivo, c.motivo)
			}
		})
	}
}

func TestAnalisarPagina_Sinteticas(t *testing.T) {
	for _, p := range paginasSinteticas() {
		t.Run(p.nome, func(t *testing.T) {
			page := primeiraPagina(t, p.objetos)
			nativo, err := ExtractNativeText(primeiraPagina(t, p.objetos))
			if err != nil {
				t.Fatalf("ExtractNativeText: %v", err)
			}
			an, err := analisarPagina(page, 1, true, slog.Default())
			if err != nil {
				t.Fatalf("analisarPagina: %v", err)
			}
			switch {
			case p.nome == "mosaico-legenda-curta":
				// A regra 2 atual pega antes: legenda com menos de 100 bytes e desenho vetorial.
				if !an.precisaOcr || an.motivo != motivoPadrao {
					t.Errorf("precisaOcr=%t motivo=%v, esperado OCR pela regra 2 (motivoPadrao)", an.precisaOcr, an.motivo)
				}
			case p.isca:
				if !an.precisaOcr || an.motivo != motivoIsca {
					t.Errorf("isca: precisaOcr=%t motivo=%v, esperado OCR por isca", an.precisaOcr, an.motivo)
				}
			case p.candidataImagem:
				if !an.precisaOcr || an.motivo != motivoImagemDescoberta {
					t.Errorf("candidata: precisaOcr=%t motivo=%v, esperado OCR por imagem descoberta", an.precisaOcr, an.motivo)
				}
			default:
				if an.precisaOcr {
					t.Errorf("precisaOcr=true (motivo %v), esperado texto nativo", an.motivo)
				}
			}
			if an.semOcr {
				t.Errorf("semOcr=true com o OCR ligado")
			}
			if an.texto != strings.TrimSpace(nativo) {
				t.Errorf("texto difere do nativo:\n%q\n%q", an.texto, strings.TrimSpace(nativo))
			}
			switch p.nome {
			case "legenda-acima-imagem":
				if an.linhasAcima != 2 {
					t.Errorf("linhasAcima = %d, esperado 2", an.linhasAcima)
				}
			case "legenda-imagem-direta":
				if an.linhasAcima != 0 {
					t.Errorf("linhasAcima = %d, esperado 0", an.linhasAcima)
				}
			}
		})
	}
}

func TestAnalisarPagina_SinteticasComOCRDesligado(t *testing.T) {
	for _, p := range paginasSinteticas() {
		if !p.isca && !p.candidataImagem {
			continue
		}
		t.Run(p.nome, func(t *testing.T) {
			nativo, err := ExtractNativeText(primeiraPagina(t, p.objetos))
			if err != nil {
				t.Fatalf("ExtractNativeText: %v", err)
			}
			an, err := analisarPagina(primeiraPagina(t, p.objetos), 1, false, slog.Default())
			if err != nil {
				t.Fatalf("analisarPagina: %v", err)
			}
			if p.nome == "mosaico-legenda-curta" {
				// A regra 2 atual pega antes: legenda com menos de 100 bytes e desenho vetorial.
				if an.precisaOcr || !an.semOcr {
					t.Errorf("precisaOcr=%t semOcr=%t, esperado semOcr pela regra 2", an.precisaOcr, an.semOcr)
				}
			} else if an.precisaOcr || an.semOcr {
				t.Errorf("precisaOcr=%t semOcr=%t, esperado os dois falsos", an.precisaOcr, an.semOcr)
			}
			if an.texto != strings.TrimSpace(nativo) {
				t.Errorf("texto difere do nativo:\n%q\n%q", an.texto, strings.TrimSpace(nativo))
			}
		})
	}
}
