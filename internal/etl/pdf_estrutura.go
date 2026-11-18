package etl

import (
	"math"

	"github.com/dslipak/pdf"
)

// caixa é um retângulo em pontos, já normalizado: x0 <= x1 e y0 <= y1.
type caixa struct{ x0, y0, x1, y1 float64 }

// mediaBoxDe lê o /MediaBox da página, subindo por /Parent (no máximo 32
// níveis). Devolve false quando não há, não tem 4 números ou tem área zero.
func mediaBoxDe(page pdf.Page) (caixa, bool) {
	v := page.V
	for range 33 { // a própria página e até 32 níveis de /Parent
		if v.IsNull() {
			return caixa{}, false
		}
		if mb := v.Key("MediaBox"); !mb.IsNull() {
			return caixaDe(mb)
		}
		v = v.Key("Parent")
	}
	return caixa{}, false
}

// caixaDe lê um retângulo [x0 y0 x1 y1] e o normaliza. Devolve false quando
// não são 4 números ou a área é zero.
func caixaDe(v pdf.Value) (caixa, bool) {
	if v.Kind() != pdf.Array || v.Len() != 4 {
		return caixa{}, false
	}
	var n [4]float64
	for i := range n {
		e := v.Index(i)
		if e.Kind() != pdf.Integer && e.Kind() != pdf.Real {
			return caixa{}, false
		}
		n[i] = e.Float64()
	}
	c := caixa{min(n[0], n[2]), min(n[1], n[3]), max(n[0], n[2]), max(n[1], n[3])}
	if c.x0 == c.x1 || c.y0 == c.y1 {
		return caixa{}, false
	}
	return c, true
}

// contem diz se o ponto (x, y) está dentro da caixa, bordas incluídas.
func (c caixa) contem(x, y float64) bool {
	return x >= c.x0 && x <= c.x1 && y >= c.y0 && y <= c.y1
}

// Grade e limiares da estrutura da página.
const (
	gradeColunas    = 48
	gradeLinhas     = 68
	imagemMinPixels = 256
	imagemMinDPI    = 50.0
	// imagensMaxPorPagina: imagens registradas por página; as seguintes não
	// entram (um mosaico real tem algumas centenas).
	imagensMaxPorPagina = 4096
)

// imagemColocada é uma imagem pintada na página: a CTM leva o quadrado
// unitário ao lugar onde ela aparece.
type imagemColocada struct {
	ctm           matriz
	largPx, altPx int
}

// metricasPagina resume a estrutura da página: frações da grade com imagem
// útil, com texto e com imagem sem texto por perto.
type metricasPagina struct {
	valida            bool // false: sem MediaBox legível, ou o texto veio do GetTextByRow
	fImg, fTxt, fDesc float64
	linhasAcima       int // linhas do texto nativo acima das imagens úteis; só a junção usa (Task 4)
	// formsIgnorados vem da passada (zerado quando não houve passada); a
	// linha de log é de quem chama.
	formsIgnorados [numMotivosForm]int
}

// finita diz se nenhum dos números é NaN ou infinito.
func finita(ns ...float64) bool {
	for _, n := range ns {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}

// util diz se a imagem conta: área de pelo menos imagemMinPixels, pelo menos
// imagemMinDPI nos dois eixos (o tamanho de cada eixo em pontos é o
// comprimento do vetor transformado, o que vale com espelho e giro) e CTM
// finita e inversível.
func (im imagemColocada) util() bool {
	c := im.ctm
	if !finita(c[:]...) || c[0]*c[3]-c[1]*c[2] == 0 {
		return false
	}
	if float64(im.largPx)*float64(im.altPx) < imagemMinPixels {
		return false
	}
	larg, alt := math.Hypot(c[0], c[1]), math.Hypot(c[2], c[3])
	return float64(im.largPx)/(larg/72) >= imagemMinDPI && float64(im.altPx)/(alt/72) >= imagemMinDPI
}

// cantos devolve os quatro cantos do quadrado unitário levados pela CTM.
func (im imagemColocada) cantos() (xs, ys [4]float64) {
	c := im.ctm
	for i, p := range [4][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		xs[i] = p[0]*c[0] + p[1]*c[2] + c[4]
		ys[i] = p[0]*c[1] + p[1]*c[3] + c[5]
	}
	return xs, ys
}

// cobre diz se o ponto (x, y) cai dentro da imagem, pelo inverso da CTM.
func (im imagemColocada) cobre(x, y float64) bool {
	c := im.ctm
	det := c[0]*c[3] - c[1]*c[2]
	dx, dy := x-c[4], y-c[5]
	u := (dx*c[3] - dy*c[2]) / det
	v := (dy*c[0] - dx*c[1]) / det
	return u >= 0 && u <= 1 && v >= 0 && v <= 1
}

// indiceGrade devolve a célula em que cai a coordenada v (já relativa à
// origem), limitada a [0, n-1], com a conta feita em float para não
// estourar a conversão.
func indiceGrade(v, passo float64, n int) int {
	return int(max(0, min(float64(n-1), math.Floor(v/passo))))
}

// medirPagina é pura. Sem imagem útil devolve valida=true e as três frações em
// zero, sem montar a grade. Não preenche linhasAcima.
func medirPagina(mediaBox caixa, imagens []imagemColocada, textos []caixa) metricasPagina {
	largura, altura := mediaBox.x1-mediaBox.x0, mediaBox.y1-mediaBox.y0
	if !finita(mediaBox.x0, mediaBox.y0, largura, altura) || largura <= 0 || altura <= 0 {
		return metricasPagina{}
	}
	m := metricasPagina{valida: true}
	uteis := 0
	for _, im := range imagens {
		if im.util() {
			uteis++
		}
	}
	if uteis == 0 {
		return m
	}

	cw, ch := largura/gradeColunas, altura/gradeLinhas
	var img, txt [gradeLinhas][gradeColunas]bool
	for _, im := range imagens {
		if !im.util() {
			continue
		}
		xs, ys := im.cantos()
		x0, x1 := min(xs[0], xs[1], xs[2], xs[3])-mediaBox.x0, max(xs[0], xs[1], xs[2], xs[3])-mediaBox.x0
		y0, y1 := min(ys[0], ys[1], ys[2], ys[3])-mediaBox.y0, max(ys[0], ys[1], ys[2], ys[3])-mediaBox.y0
		if x1 < 0 || y1 < 0 || x0 > largura || y0 > altura {
			continue
		}
		for r := indiceGrade(y0, ch, gradeLinhas); r <= indiceGrade(y1, ch, gradeLinhas); r++ {
			cy := mediaBox.y0 + (float64(r)+0.5)*ch
			for c := indiceGrade(x0, cw, gradeColunas); c <= indiceGrade(x1, cw, gradeColunas); c++ {
				if !img[r][c] && im.cobre(mediaBox.x0+(float64(c)+0.5)*cw, cy) {
					img[r][c] = true
				}
			}
		}
	}
	for _, t := range textos {
		x0, x1 := t.x0-mediaBox.x0, t.x1-mediaBox.x0
		y0, y1 := t.y0-mediaBox.y0, t.y1-mediaBox.y0
		if !finita(x0, x1, y0, y1) || x1 < 0 || y1 < 0 || x0 > largura || y0 > altura {
			continue
		}
		for r := indiceGrade(y0, ch, gradeLinhas); r <= indiceGrade(y1, ch, gradeLinhas); r++ {
			for c := indiceGrade(x0, cw, gradeColunas); c <= indiceGrade(x1, cw, gradeColunas); c++ {
				txt[r][c] = true
			}
		}
	}

	var nImg, nTxt, nDesc int
	for r := range gradeLinhas {
		for c := range gradeColunas {
			if txt[r][c] {
				nTxt++
			}
			if !img[r][c] {
				continue
			}
			nImg++
			if !textoPerto(&txt, r, c) {
				nDesc++
			}
		}
	}
	total := float64(gradeLinhas * gradeColunas)
	m.fImg, m.fTxt, m.fDesc = float64(nImg)/total, float64(nTxt)/total, float64(nDesc)/total
	return m
}

// textoPerto diz se a célula (r, c) ou uma das 8 vizinhas tem texto.
func textoPerto(txt *[gradeLinhas][gradeColunas]bool, r, c int) bool {
	for rr := max(r-1, 0); rr <= min(r+1, gradeLinhas-1); rr++ {
		for cc := max(c-1, 0); cc <= min(c+1, gradeColunas-1); cc++ {
			if txt[rr][cc] {
				return true
			}
		}
	}
	return false
}

// topoImagens devolve o maior Y dos cantos das imagens úteis, e false quando
// nenhuma é útil.
func topoImagens(imagens []imagemColocada) (float64, bool) {
	topo, achou := math.Inf(-1), false
	for _, im := range imagens {
		if !im.util() {
			continue
		}
		_, ys := im.cantos()
		topo, achou = max(topo, ys[0], ys[1], ys[2], ys[3]), true
	}
	return topo, achou
}

// linhasAcimaDasImagens conta as linhas do texto nativo (as que montarLinhas
// emite) cuja linha de base fica na altura do topo da imagem útil mais alta,
// ou acima dele. Pura. Sem imagem útil devolve 0. extrairNativo não a usa:
// conta na mesma passada que monta o texto (montarEContar).
func linhasAcimaDasImagens(trechos []trecho, imagens []imagemColocada) int {
	topo, achou := topoImagens(imagens)
	if !achou {
		return 0
	}
	copia := append([]trecho(nil), trechos...) // paraCadaLinha ordena no lugar
	_, n := montarEContar(copia, topo)
	return n
}

// temImagemUtil diz se alguma das imagens é útil.
func temImagemUtil(imagens []imagemColocada) bool {
	for _, im := range imagens {
		if im.util() {
			return true
		}
	}
	return false
}

// metricas mede a estrutura da passada. Sem MediaBox legível, valida=false.
// As caixas de texto só são montadas quando há imagem útil. Não preenche
// linhasAcima: quem conta é extrairNativo, na passada que monta o texto.
func (p passadaPagina) metricas() metricasPagina {
	m := metricasPagina{}
	if p.temMediaBox {
		var textos []caixa
		if temImagemUtil(p.imagens) {
			textos = make([]caixa, 0, len(p.trechos))
			for _, t := range p.trechos {
				textos = append(textos, caixa{min(t.x, t.fimX), min(t.y, t.y+t.tamanho), max(t.x, t.fimX), max(t.y, t.y+t.tamanho)})
			}
		}
		m = medirPagina(p.mediaBox, p.imagens, textos)
	}
	m.formsIgnorados = p.formsIgnorados
	return m
}
