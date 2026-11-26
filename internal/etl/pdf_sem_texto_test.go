package etl

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dslipak/pdf"
)

// fixturesSemTexto são as fixtures do benchmark cuja página não tem nenhum
// operador de texto: imagem escaneada e texto desenhado em vetores.
var fixturesSemTexto = []string{"escaneado.pdf", "vetorial.pdf"}

func lerFixtureBench(t *testing.T, nome string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bench", "fixtures", nome))
	if err != nil {
		t.Fatalf("falha ao ler a fixture: %v", err)
	}
	return b
}

func primeiraPaginaDe(t *testing.T, dados []byte) pdf.Page {
	t.Helper()
	reader, n, err := abrirLeitorPDF(bytes.NewReader(dados), int64(len(dados)), 0)
	if err != nil {
		t.Fatalf("falha ao abrir pdf: %v", err)
	}
	paginas, _, err := listarPaginas(reader, n)
	if err != nil || len(paginas) == 0 {
		t.Fatalf("falha ao listar páginas: %v", err)
	}
	return paginas[0]
}

// Página sem Tj/TJ/'/" não passa pelo GetTextByRow: as métricas da passada
// posicional continuam válidas.
func TestSemTexto_MetricasValidasSemGetTextByRow(t *testing.T) {
	for _, nome := range fixturesSemTexto {
		t.Run(nome, func(t *testing.T) {
			texto, m, err := extrairNativoSeguro(primeiraPaginaDe(t, lerFixtureBench(t, nome)))
			if err != nil {
				t.Fatalf("extrairNativoSeguro: %v", err)
			}
			if texto != "" {
				t.Errorf("texto = %q, quer vazio", texto)
			}
			if !m.valida {
				t.Errorf("métricas inválidas (%+v): a página caiu no GetTextByRow", m)
			}
			if nome == "escaneado.pdf" && m.fImg <= 0 {
				t.Errorf("fImg = %v, quer a imagem da página medida", m.fImg)
			}
		})
	}
}

func TestSemTexto_ContinuaIndoAoOCR(t *testing.T) {
	for _, nome := range fixturesSemTexto {
		t.Run(nome, func(t *testing.T) {
			dados := lerFixtureBench(t, nome)
			an, err := analisarPagina(primeiraPaginaDe(t, dados), 1, true, slog.Default())
			if err != nil {
				t.Fatalf("analisarPagina: %v", err)
			}
			if !an.precisaOcr || an.motivo != motivoPadrao || an.semOcr {
				t.Errorf("OCR ligado: precisaOcr=%t motivo=%v semOcr=%t, quer OCR pela regra padrão", an.precisaOcr, an.motivo, an.semOcr)
			}

			pages := parsearSemOCR(t, dados, false)
			p := pages["1"].(map[string]any)
			if so, _ := p["sem_ocr"].(bool); !so || p["content"] != marcadorPaginaSemOCR {
				t.Errorf("OCR desligado: página = %v, quer o marcador de página digitalizada", p)
			}
		})
	}
}
