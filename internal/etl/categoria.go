package etl

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"strings"
)

// Categorias de página do relatório (spec, "Categorias de página"): cada
// página de documento lido cai em exatamente uma, pela primeira regra que
// servir, na ordem abaixo.
const (
	CatComErro    = "com_erro"
	CatSemOCR     = "sem_ocr"
	CatVazia      = "vazia"
	CatLidaOCR    = "lida_ocr"
	CatLidaNativa = "lida_nativa"
)

// categoriaPagina devolve a categoria de uma página pela primeira regra que
// servir: erro de leitura, ou ocr_erro sem texto de reserva (ocr_aviso), é
// com_erro; a marca sem_ocr é sem_ocr; conteúdo em branco é vazia; o OCR que
// entrou é lida_ocr; o resto (nativo, HTML, OCR que ficou com o nativo) é
// lida_nativa. Pura: só lê o mapa.
func categoriaPagina(p map[string]any) string {
	_, ocrErro := p["ocr_erro"]
	_, ocrAviso := p["ocr_aviso"]
	_, semOCR := p["sem_ocr"]
	erroLeitura, _ := p["erro_leitura"].(bool)
	content, _ := p["content"].(string)
	lidaOCR, _ := p["ocr"].(bool)
	switch {
	case erroLeitura || (ocrErro && !ocrAviso):
		return CatComErro
	case semOCR:
		return CatSemOCR
	case strings.TrimSpace(content) == "":
		return CatVazia
	case lidaOCR:
		return CatLidaOCR
	}
	return CatLidaNativa
}

// classificarCategorias grava "categoria" em cada página de um documento lido.
// Roda na goroutine do documento e só escreve nos mapas das páginas dele; em
// Debug, uma linha "página classificada" por página, em ordem.
func classificarCategorias(ctx context.Context, lg *slog.Logger, paginas map[string]any) {
	debug := lg.Enabled(ctx, slog.LevelDebug)
	var ordem []map[string]any
	for _, v := range paginas {
		p, _ := v.(map[string]any)
		if p == nil {
			continue
		}
		p["categoria"] = categoriaPagina(p)
		if debug {
			ordem = append(ordem, p)
		}
	}
	if !debug {
		return
	}
	slices.SortFunc(ordem, func(a, b map[string]any) int {
		ia, _ := a["index"].(int)
		ib, _ := b["index"].(int)
		return cmp.Compare(ia, ib)
	})
	for _, p := range ordem {
		attrs := []any{"pag", p["index"], "categoria", p["categoria"]}
		if motivo, ok := p["motivo_ocr"].(string); ok {
			attrs = append(attrs, "motivo", motivo)
			if lidaOCR, _ := p["ocr"].(bool); lidaOCR && motivo == motivoImagemDescoberta.String() {
				attrs = append(attrs, "juncao", true)
			}
		}
		lg.Debug("página classificada", attrs...)
	}
}

// String dá o nome do motivo gravado em motivo_ocr e no log.
func (m motivoOCR) String() string {
	switch m {
	case motivoPadrao:
		return "padrao"
	case motivoIlegivel:
		return "ilegivel"
	case motivoImagemDescoberta:
		return "imagem_descoberta"
	case motivoIsca:
		return "isca"
	case motivoForcado:
		return "forcado"
	}
	return "desconhecido"
}
