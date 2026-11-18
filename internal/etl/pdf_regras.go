package etl

// Limiares das duas regras novas de OCR para páginas com texto nativo.
const (
	// descobertaMinFDesc: fração mínima da página coberta por imagem útil
	// sem texto nativo por cima (imagem descoberta).
	descobertaMinFDesc = 0.30
	// descobertaMaxFTxt: fração máxima da página coberta por texto nativo
	// para a imagem descoberta valer.
	descobertaMaxFTxt = 0.10
	// iscaMinFImg: fração mínima da página coberta por imagem útil para o
	// teste de texto-isca valer (e para a língua ser contada).
	iscaMinFImg = 0.10
)

// regraNova diz se uma das duas regras novas manda a página ao OCR. Pura.
// Isca vem antes de imagem descoberta. Com m.valida == false devolve false.
func regraNova(m metricasPagina, c contagemLingua) (motivoOCR, bool) {
	if !m.valida {
		return motivoPadrao, false
	}
	if m.fImg >= iscaMinFImg && c.isca() {
		return motivoIsca, true
	}
	if m.fDesc >= descobertaMinFDesc && m.fTxt <= descobertaMaxFTxt {
		return motivoImagemDescoberta, true
	}
	return motivoPadrao, false
}

// contagemParaRegra conta a língua do texto nativo só quando a isca pode
// valer (métricas válidas e imagem bastante); fora disso a contagem é zero.
func contagemParaRegra(m metricasPagina, texto string) contagemLingua {
	if !m.valida || m.fImg < iscaMinFImg {
		return contagemLingua{}
	}
	return contarLingua(texto)
}
