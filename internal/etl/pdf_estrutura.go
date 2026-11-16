package etl

import "github.com/dslipak/pdf"

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
