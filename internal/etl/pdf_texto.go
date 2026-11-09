package etl

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dslipak/pdf"
)

// Limiares do extrator posicional.
const (
	// larguraGlifoEm estima a largura de um glifo, em frações do tamanho da
	// fonte, quando a fonte não traz /Widths (as 14 padrão sem /Widths e as
	// compostas). O plano dava 0,5; calibrado para 0,4, que fica dentro da
	// faixa [0,334; 0,447] em que passam "R$1.234 OK" (glifos a 7 pt, Helvetica
	// 12) e palavras em minúscula posicionadas uma a uma ("estimativa" + espaço
	// = 57,3 pt). Abaixo da média das maiúsculas (~0,67), então maiúsculas
	// posicionadas letra a letra por Td numa fonte sem /Widths podem ganhar
	// espaço falso.
	larguraGlifoEm = 0.4
	// espacoTJMilesimos: deslocamento acumulado num TJ, em milésimos de em, a
	// partir do qual (≤) dois trechos são palavras distintas.
	espacoTJMilesimos = -200
	// espacoPosicaoEm: folga horizontal, em frações do tamanho da fonte, além do
	// fim estimado do trecho anterior que conta como espaço entre palavras.
	espacoPosicaoEm = 0.25
	// mesmaLinhaEm: diferença vertical, em frações do tamanho, dentro da mesma linha.
	mesmaLinhaEm = 0.5
)

// matriz é uma matriz afim do PDF [a b c d e f] (vetor-linha: p' = p × M).
type matriz [6]float64

var identidade = matriz{1, 0, 0, 1, 0, 0}

// vezes devolve m × n.
func (m matriz) vezes(n matriz) matriz {
	return matriz{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func translacao(tx, ty float64) matriz { return matriz{1, 0, 0, 1, tx, ty} }

func matrizDosArgs(args []pdf.Value) matriz {
	var m matriz
	for i := range m {
		m[i] = args[i].Float64()
	}
	return m
}

// fonteTexto é o que o extrator usa de uma fonte: o decodificador e, se a
// fonte trouxer /Widths, as larguras dos códigos em milésimos de em.
type fonteTexto struct {
	enc      pdf.TextEncoding
	larguras []float64
	primeiro int
}

// carregarFonte valida os streams que Encoder() interpreta antes de chamá-lo:
// a biblioteca lê o /ToUnicode com o mesmo lexer que gira para sempre num
// stream ilegível. Com /ToUnicode ou /Encoding ilegível, a fonte é decodificada
// como sem ToUnicode (pdfDocEncoding, o Encoder de uma fonte vazia).
func carregarFonte(f pdf.Font) *fonteTexto {
	ft := &fonteTexto{larguras: f.Widths(), primeiro: f.FirstChar()}
	if fonteLegivel(f) {
		ft.enc = f.Encoder()
	} else {
		ft.enc = pdf.Font{}.Encoder()
	}
	return ft
}

// fonteLegivel diz se o /ToUnicode e o /Encoding da fonte, quando são
// streams, podem ser lidos por inteiro.
func fonteLegivel(f pdf.Font) bool {
	for _, chave := range []string{"ToUnicode", "Encoding"} {
		if v := f.V.Key(chave); v.Kind() == pdf.Stream && lerStream(v) != nil {
			return false
		}
	}
	return true
}

// largura devolve a largura de um trecho em frações do tamanho da fonte: a
// soma de /Widths dos códigos crus, se a fonte as tiver; senão, a estimativa
// por runa decodificada. Um código fora de /Widths (ou com largura 0) usa a
// estimativa.
func (f *fonteTexto) largura(cru, texto string) float64 {
	if len(f.larguras) == 0 {
		return float64(utf8.RuneCountInString(texto)) * larguraGlifoEm
	}
	var soma float64
	for i := 0; i < len(cru); i++ {
		if j := int(cru[i]) - f.primeiro; j >= 0 && j < len(f.larguras) && f.larguras[j] > 0 {
			soma += f.larguras[j] / 1000
		} else {
			soma += larguraGlifoEm
		}
	}
	return soma
}

// streamMaximo limita o stream descomprimido que lerStream aceita guardar em
// memória: um conteúdo de página real tem poucos MB, e uma bomba de
// descompressão vira página sem texto nativo.
const streamMaximo = 64 << 20

// lerStream lê o stream por inteiro e devolve o erro de leitura, se houver,
// ou um erro quando o lexer da biblioteca giraria para sempre nele (dicionário,
// array ou string sem fecho; veja pdf_lexer.go).
func lerStream(v pdf.Value) error {
	rc := v.Reader()
	defer rc.Close()
	dados, err := io.ReadAll(io.LimitReader(rc, streamMaximo+1))
	if err != nil {
		return err
	}
	if len(dados) > streamMaximo {
		return fmt.Errorf("stream maior que %d MiB", streamMaximo>>20)
	}
	if !lexerTermina(dados) {
		return errors.New("dicionário, array ou string sem fecho")
	}
	return nil
}

// trecho é um pedaço de texto mostrado por um Tj ou por uma string de TJ, em
// coordenadas do usuário.
type trecho struct {
	texto      string
	x, y, fimX float64
	tamanho    float64
	espacoTJ   bool // o TJ pede espaço antes deste trecho (regra a)
}

// extrairTextoPosicional lê os operadores de texto da página e monta as linhas
// pela posição dos glifos: uma linha por Y, do topo para baixo, e trechos
// ordenados por X, separados por espaço só quando a distância entre eles o
// indica. Junta assim as letras de PDFs com kerning ("C O N T E S T A").
//
// O chamador garante que /Contents é um stream ou um array de streams legíveis
// (o lexer da biblioteca gira para sempre num stream ilegível) e recupera pânicos.
func extrairTextoPosicional(page pdf.Page) (string, error) {
	contents := page.V.Key("Contents")
	streams := []pdf.Value{contents}
	if contents.Kind() == pdf.Array {
		streams = streams[:0]
		for i := 0; i < contents.Len(); i++ {
			streams = append(streams, contents.Index(i))
		}
	}

	cache := map[string]*fonteTexto{}
	fonteDe := func(nome string) *fonteTexto {
		if f, ok := cache[nome]; ok {
			return f
		}
		f := carregarFonte(page.Font(nome))
		cache[nome] = f
		return f
	}

	var (
		trechos  []trecho
		ctm      = identidade
		pilhaCTM []matriz
		tm, tlm  = identidade, identidade
		fonte    = carregarFonte(pdf.Font{})
		tfs      float64
		leading  float64
	)

	td := func(tx, ty float64) {
		tlm = translacao(tx, ty).vezes(tlm)
		tm = tlm
	}
	// mostrar decodifica s, registra o trecho na posição atual e avança a matriz
	// de texto pela largura do trecho (real ou estimada).
	mostrar := func(s string, espacoTJ bool) {
		texto := fonte.enc.Decode(s)
		m := tm.vezes(ctm)
		tamanho := tfs * math.Hypot(m[2], m[3])
		x, y := m[4], m[5]
		tm = translacao(fonte.largura(s, texto)*tfs, 0).vezes(tm)
		if texto == "" {
			return
		}
		trechos = append(trechos, trecho{texto: texto, x: x, y: y, fimX: tm.vezes(ctm)[4], tamanho: tamanho, espacoTJ: espacoTJ})
	}
	proximaLinha := func() { td(0, -leading) }

	for _, strm := range streams {
		pdf.Interpret(strm, func(stk *pdf.Stack, op string) {
			n := stk.Len()
			args := make([]pdf.Value, n)
			for i := n - 1; i >= 0; i-- {
				args[i] = stk.Pop()
			}
			switch op {
			case "q":
				pilhaCTM = append(pilhaCTM, ctm)
			case "Q":
				if len(pilhaCTM) > 0 {
					ctm = pilhaCTM[len(pilhaCTM)-1]
					pilhaCTM = pilhaCTM[:len(pilhaCTM)-1]
				}
			case "cm":
				if n == 6 {
					ctm = matrizDosArgs(args).vezes(ctm)
				}
			case "BT":
				tm, tlm = identidade, identidade
			case "Tf":
				if n == 2 {
					fonte = fonteDe(args[0].Name())
					tfs = args[1].Float64()
				}
			case "TL":
				if n == 1 {
					leading = args[0].Float64()
				}
			case "Td":
				if n == 2 {
					td(args[0].Float64(), args[1].Float64())
				}
			case "TD":
				if n == 2 {
					leading = -args[1].Float64()
					td(args[0].Float64(), args[1].Float64())
				}
			case "Tm":
				if n == 6 {
					tm = matrizDosArgs(args)
					tlm = tm
				}
			case "T*":
				proximaLinha()
			case "'":
				if n == 1 {
					proximaLinha()
					mostrar(args[0].RawString(), false)
				}
			case "\"":
				if n == 3 {
					proximaLinha()
					mostrar(args[2].RawString(), false)
				}
			case "Tj":
				if n == 1 {
					mostrar(args[0].RawString(), false)
				}
			case "TJ":
				if n != 1 {
					return
				}
				v := args[0]
				var acumulado float64
				houveTrecho := false
				for i := 0; i < v.Len(); i++ {
					x := v.Index(i)
					switch x.Kind() {
					case pdf.String:
						mostrar(x.RawString(), houveTrecho && acumulado <= espacoTJMilesimos)
						houveTrecho, acumulado = true, 0
					case pdf.Integer, pdf.Real:
						acumulado += x.Float64()
						tm = translacao(-x.Float64()/1000*tfs, 0).vezes(tm)
					}
				}
			}
		})
	}
	return montarLinhas(trechos), nil
}

// montarLinhas agrupa os trechos por Y, do maior para o menor, e junta cada
// linha em ordem de X.
func montarLinhas(trechos []trecho) string {
	sort.SliceStable(trechos, func(i, j int) bool { return trechos[i].y > trechos[j].y })

	var sb strings.Builder
	for ini := 0; ini < len(trechos); {
		fim := ini + 1
		for fim < len(trechos) && math.Abs(trechos[ini].y-trechos[fim].y) < mesmaLinhaEm*trechos[ini].tamanho {
			fim++
		}
		linha := trechos[ini:fim]
		ini = fim

		sort.SliceStable(linha, func(i, j int) bool { return linha[i].x < linha[j].x })
		var lb strings.Builder
		for i, tr := range linha {
			if i > 0 {
				ant := linha[i-1]
				afastado := tr.x > ant.fimX+espacoPosicaoEm*ant.tamanho
				if (tr.espacoTJ || afastado) && !strings.HasSuffix(lb.String(), " ") && !strings.HasPrefix(tr.texto, " ") {
					lb.WriteByte(' ')
				}
			}
			lb.WriteString(tr.texto)
		}
		if s := strings.TrimSpace(lb.String()); s != "" {
			sb.WriteString(lb.String())
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
