package etl

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// comPoppler faz ParsePages ler o texto pelo pdftotext, para o PDF que a
// biblioteca recusou e o pdfinfo abriu com numPaginas páginas. Nesse caminho a
// biblioteca não é tocada: o PDF só é lido do disco (caminhoOCR).
func (p *PDFParser) comPoppler(numPaginas int) *PDFParser {
	p.poppler, p.numPaginas = true, numPaginas
	return p
}

// parsePoppler é ParsePages para o segundo leitor. Sem a biblioteca não há como
// ver imagens nem conteúdo vetorial, então o critério de OCR é só o texto:
// curto (menos de 100 bytes, como no caminho nativo) ou ilegível.
func (p *PDFParser) parsePoppler(
	ctx context.Context,
	ocrEnabled bool,
	ocrRunner func(context.Context, string, int) (string, error),
	onPageComplete func(pageNum int),
) (map[string]any, error) {
	numPages := p.numPaginas
	comOCR := ocrEnabled && ocrRunner != nil
	log.Printf("[PDFParser] PDF %s lido pelo segundo leitor: %d páginas", p.filePath, numPages)

	caminho, err := p.caminhoParaOCR()
	var textos []string
	if err == nil {
		textos, err = textoPDF(ctx, caminho, numPages)
	}
	if err == nil && len(textos) != numPages {
		err = fmt.Errorf("pdftotext devolveu %d textos para %d páginas", len(textos), numPages)
	}
	if err != nil {
		if errCtx := ctx.Err(); errCtx != nil {
			return nil, errCtx
		}
		if !comOCR {
			log.Printf("[PDFParser] Segundo leitor não extraiu o texto de %s: %v", p.filePath, err)
			return nil, fmt.Errorf("segundo leitor: %w", err)
		}
		log.Printf("[PDFParser] Segundo leitor não extraiu o texto de %s: %v. Todas as páginas vão ao OCR.", p.filePath, err)
		textos = make([]string, numPages)
	}

	d := p.novoDespachoOCR(ctx, numPages, ocrRunner, onPageComplete)
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		if err := ctx.Err(); err != nil {
			log.Printf("[PDFParser] Contexto cancelado na leitura da página %d: %v", pageNum, err)
			d.aguardar()
			return nil, err
		}
		texto := strings.TrimSpace(textos[pageNum-1])
		ilegivel := textoIlegivel(texto)
		curto := len(texto) < 100
		if ilegivel {
			texto = marcadorIlegivel(texto)
		}
		if (ilegivel || curto) && comOCR {
			d.agendar(ocrTask{pageNum: pageNum, fallback: texto})
			continue
		}
		pageData := map[string]any{
			"index":   pageNum,
			"ocr":     false,
			"content": texto,
		}
		// Com o OCR desligado, a página ilegível e a sem texto ficam marcadas
		// como no caminho nativo; o texto curto legível fica como está.
		if !ocrEnabled && (ilegivel || texto == "") {
			pageData["sem_ocr"] = true
			if texto == "" {
				pageData["content"] = marcadorPaginaSemOCR
			}
		}
		d.paginaPronta(pageNum, pageData)
	}

	paginasMap := d.esperar()
	if err := ctx.Err(); err != nil {
		log.Printf("[PDFParser] Análise abortada devido a cancelamento do contexto: %v", err)
		return nil, err
	}
	if n := d.falhas.Load(); n > 0 {
		log.Printf("[PDFParser] ATENÇÃO: %d página(s) de %s com falha de OCR.", n, p.filePath)
	}
	return paginasMap, nil
}
