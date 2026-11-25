package etl

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
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
	lg := p.logger(ctx)
	lg.Debug("PDF lido pelo segundo leitor", "paginas", numPages)

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
			return nil, resumirSegundoLeitor(ctx, filepath.Base(p.filePath), "", err)
		}
		lg.Warn("segundo leitor não extraiu o texto; todas as páginas vão ao OCR", atributosErro(comEtapa(EtapaLerPDF, err))...)
		textos = make([]string, numPages)
	}

	d := p.novoDespachoOCR(ctx, numPages, ocrRunner, onPageComplete)
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		if err := ctx.Err(); err != nil {
			lg.Debug("leitura cancelada", "pag", pageNum, "erro", err)
			d.esperarDespachados()
			return nil, err
		}
		texto := strings.TrimSpace(textos[pageNum-1])
		bytesNativo := len(texto)
		ilegivel := textoIlegivel(texto)
		curto := len(texto) < 100
		motivo := motivoPadrao
		if ilegivel {
			texto = marcadorIlegivel(texto)
			motivo = motivoIlegivel
		}
		vaiAoOCR := (ilegivel || curto) && comOCR
		// Forçar OCR: a página com texto que ficaria com o nativo vai também.
		if p.forcar && comOCR && !vaiAoOCR {
			vaiAoOCR, motivo = true, motivoForcado
		}
		if lg.Enabled(ctx, slog.LevelDebug) {
			attrs := []any{"pag", pageNum, "bytes_nativo", bytesNativo}
			if vaiAoOCR {
				attrs = append(attrs, "motivo", motivo.String())
			}
			lg.Debug("página analisada", attrs...)
		}
		if vaiAoOCR {
			// O motivo não muda a decisão aqui: só o nome em motivo_ocr.
			d.agendar(ocrTask{pageNum: pageNum, fallback: texto, motivo: motivo})
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
		lg.Debug("análise cancelada", "erro", err)
		return nil, err
	}
	if n := d.falhas.Load(); n > 0 {
		lg.Warn("página com falha de OCR", append([]any{"paginas", n}, atributosErro(comEtapa(EtapaOCR, d.erroOCR()))...)...)
	}
	return paginasMap, nil
}
