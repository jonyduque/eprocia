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
	deForm     bool // mostrado dentro de um Form XObject
}

// Limites da interpretação de Form XObjects.
const (
	// formMaxProfundidade: o Form chamado pela página está na profundidade 1;
	// um Do que levaria além desta é ignorado.
	formMaxProfundidade = 4
	// formMaxPorPagina: Forms interpretados por página, contando os aninhados.
	formMaxPorPagina = 256
)

// motivoFormIgnorado diz por que um Form chamado por Do não foi interpretado.
type motivoFormIgnorado int

const (
	formIgnoradoStreamIlegivel motivoFormIgnorado = iota
	formIgnoradoProfundidade
	formIgnoradoLimite
	formIgnoradoPanico
	numMotivosForm
)

// String devolve o texto fixo do motivo, para a linha de log.
func (m motivoFormIgnorado) String() string {
	switch m {
	case formIgnoradoStreamIlegivel:
		return "stream ilegível"
	case formIgnoradoProfundidade:
		return "profundidade"
	case formIgnoradoLimite:
		return "limite por página"
	case formIgnoradoPanico:
		return "pânico"
	}
	return "desconhecido"
}

// passadaPagina é o que uma passada pelos operadores da página produz.
type passadaPagina struct {
	trechos     []trecho // da página e dos Forms, em coordenadas da página
	trechosForm int      // quantos vieram de Form, depois das proteções
	mediaBox    caixa
	temMediaBox bool
	// formsIgnorados conta os Forms não interpretados, por motivo. Form em
	// camada opcional (/OC) não entra: é regra, não falha.
	formsIgnorados [numMotivosForm]int
}

// estadoForm é a parte do interpretador que um Form salva antes e restaura
// depois, como num q … Q.
type estadoForm struct {
	ctm          matriz
	pilhaCTM     []matriz
	tm, tlm      matriz
	fonte        *fonteTexto
	tfs, leading float64
	marcado      []bool // pilha de BDC/BMC: true quando o trecho é /OC
	profundidade int    // 0 na página, 1 no Form chamado por ela...
	recursos     pdf.Value
	fontes       map[string]*fonteTexto // cache de fontes do escopo de recursos
}

// interpretador percorre os operadores da página e dos Forms que ela chama.
type interpretador struct {
	estadoForm
	trechos        []trecho
	formsVistos    int
	formsIgnorados [numMotivosForm]int
}

// fonteDe devolve a fonte do escopo de recursos atual, carregada uma vez.
func (it *interpretador) fonteDe(nome string) *fonteTexto {
	if f, ok := it.fontes[nome]; ok {
		return f
	}
	f := carregarFonte(pdf.Font{V: it.recursos.Key("Font").Key(nome)})
	it.fontes[nome] = f
	return f
}

// emOC diz se o operador atual está dentro de um trecho /OC … BDC … EMC.
func (it *interpretador) emOC() bool {
	for _, oc := range it.marcado {
		if oc {
			return true
		}
	}
	return false
}

func (it *interpretador) td(tx, ty float64) {
	it.tlm = translacao(tx, ty).vezes(it.tlm)
	it.tm = it.tlm
}

func (it *interpretador) proximaLinha() { it.td(0, -it.leading) }

// mostrar decodifica s, registra o trecho na posição atual e avança a matriz
// de texto pela largura do trecho (real ou estimada). Dentro de um Form, o
// texto de um trecho /OC não é registrado.
func (it *interpretador) mostrar(s string, espacoTJ bool) {
	texto := it.fonte.enc.Decode(s)
	m := it.tm.vezes(it.ctm)
	tamanho := it.tfs * math.Hypot(m[2], m[3])
	x, y := m[4], m[5]
	it.tm = translacao(it.fonte.largura(s, texto)*it.tfs, 0).vezes(it.tm)
	if texto == "" || (it.profundidade > 0 && it.emOC()) {
		return
	}
	it.trechos = append(it.trechos, trecho{texto: texto, x: x, y: y, fimX: it.tm.vezes(it.ctm)[4], tamanho: tamanho, espacoTJ: espacoTJ, deForm: it.profundidade > 0})
}

// operador trata um operador do stream de conteúdo.
func (it *interpretador) operador(stk *pdf.Stack, op string) {
	n := stk.Len()
	args := make([]pdf.Value, n)
	for i := n - 1; i >= 0; i-- {
		args[i] = stk.Pop()
	}
	switch op {
	case "q":
		it.pilhaCTM = append(it.pilhaCTM, it.ctm)
	case "Q":
		if len(it.pilhaCTM) > 0 {
			it.ctm = it.pilhaCTM[len(it.pilhaCTM)-1]
			it.pilhaCTM = it.pilhaCTM[:len(it.pilhaCTM)-1]
		}
	case "cm":
		if n == 6 {
			it.ctm = matrizDosArgs(args).vezes(it.ctm)
		}
	case "BT":
		it.tm, it.tlm = identidade, identidade
	case "Tf":
		if n == 2 {
			it.fonte = it.fonteDe(args[0].Name())
			it.tfs = args[1].Float64()
		}
	case "TL":
		if n == 1 {
			it.leading = args[0].Float64()
		}
	case "Td":
		if n == 2 {
			it.td(args[0].Float64(), args[1].Float64())
		}
	case "TD":
		if n == 2 {
			it.leading = -args[1].Float64()
			it.td(args[0].Float64(), args[1].Float64())
		}
	case "Tm":
		if n == 6 {
			it.tm = matrizDosArgs(args)
			it.tlm = it.tm
		}
	case "T*":
		it.proximaLinha()
	case "'":
		if n == 1 {
			it.proximaLinha()
			it.mostrar(args[0].RawString(), false)
		}
	case "\"":
		if n == 3 {
			it.proximaLinha()
			it.mostrar(args[2].RawString(), false)
		}
	case "Tj":
		if n == 1 {
			it.mostrar(args[0].RawString(), false)
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
				it.mostrar(x.RawString(), houveTrecho && acumulado <= espacoTJMilesimos)
				houveTrecho, acumulado = true, 0
			case pdf.Integer, pdf.Real:
				acumulado += x.Float64()
				it.tm = translacao(-x.Float64()/1000*it.tfs, 0).vezes(it.tm)
			}
		}
	case "BDC":
		it.marcado = append(it.marcado, n == 2 && args[0].Name() == "OC")
	case "BMC":
		it.marcado = append(it.marcado, false)
	case "EMC":
		if len(it.marcado) > 0 {
			it.marcado = it.marcado[:len(it.marcado)-1]
		}
	case "Do":
		if n == 1 {
			it.fazerDo(args[0].Name())
		}
	}
}

// fazerDo interpreta o XObject nome quando é um Form. Imagem, nome que não
// existe nos recursos, Form em camada opcional e Do dentro de um trecho /OC
// não fazem nada.
func (it *interpretador) fazerDo(nome string) {
	if it.emOC() {
		return
	}
	xo := it.recursos.Key("XObject").Key(nome)
	if xo.Kind() != pdf.Stream || xo.Key("Subtype").Name() != "Form" || !xo.Key("OC").IsNull() {
		return
	}
	if it.profundidade >= formMaxProfundidade {
		it.formsIgnorados[formIgnoradoProfundidade]++
		return
	}
	if it.formsVistos >= formMaxPorPagina {
		it.formsIgnorados[formIgnoradoLimite]++
		return
	}
	// Conta antes de ler o stream: o limite também segura uma página que
	// chama milhares de vezes um Form ilegível.
	it.formsVistos++
	it.rodarForm(xo)
}

// rodarForm interpreta o stream do Form com o estado salvo antes e restaurado
// depois, mesmo se a interpretação parar no meio. Um pânico descarta os
// trechos do Form (e dos que ele chamou) e a página segue.
func (it *interpretador) rodarForm(xo pdf.Value) {
	salvo := it.estadoForm
	inicio := len(it.trechos)
	defer func() {
		if r := recover(); r != nil {
			it.trechos = it.trechos[:inicio]
			it.formsIgnorados[formIgnoradoPanico]++
		}
		it.estadoForm = salvo
	}()
	// O lexer da biblioteca gira para sempre num stream ilegível; um filtro
	// não suportado entra em pânico já aqui.
	if lerStream(xo) != nil {
		it.formsIgnorados[formIgnoradoStreamIlegivel]++
		return
	}
	if m := xo.Key("Matrix"); m.Kind() == pdf.Array && m.Len() == 6 {
		var mf matriz
		for i := range mf {
			mf[i] = m.Index(i).Float64()
		}
		it.ctm = mf.vezes(it.ctm)
	}
	if r := xo.Key("Resources"); r.Kind() == pdf.Dict {
		it.recursos = r
		it.fontes = map[string]*fonteTexto{}
	}
	it.pilhaCTM = nil
	it.marcado = nil
	it.profundidade++
	pdf.Interpret(xo, it.operador)
}

// interpretarPagina lê os operadores de texto do /Contents da página e dos
// Forms que ele chama. Os trechos de Form passam por duas proteções: o que
// começa fora do MediaBox é descartado (sem MediaBox legível, entra) e o
// mesmo texto na mesma posição, arredondada a 0,5 pt, conta uma vez. Os
// trechos do /Contents da página saem como sempre saíram.
//
// O chamador garante que /Contents é um stream ou um array de streams legíveis
// (o lexer da biblioteca gira para sempre num stream ilegível) e recupera os
// pânicos de fora dos Forms.
func interpretarPagina(page pdf.Page) (passadaPagina, error) {
	contents := page.V.Key("Contents")
	streams := []pdf.Value{contents}
	if contents.Kind() == pdf.Array {
		streams = streams[:0]
		for i := 0; i < contents.Len(); i++ {
			streams = append(streams, contents.Index(i))
		}
	}

	it := &interpretador{
		ctm:      identidade,
		tm:       identidade,
		tlm:      identidade,
		fonte:    carregarFonte(pdf.Font{}),
		recursos: page.Resources(),
		fontes:   map[string]*fonteTexto{},
	}
	for _, strm := range streams {
		pdf.Interpret(strm, it.operador)
	}

	p := passadaPagina{formsIgnorados: it.formsIgnorados}
	p.mediaBox, p.temMediaBox = mediaBoxDe(page)
	type chave struct {
		texto string
		x, y  float64
	}
	vistos := map[chave]bool{}
	p.trechos = it.trechos[:0]
	for _, tr := range it.trechos {
		if tr.deForm {
			if p.temMediaBox && !p.mediaBox.contem(tr.x, tr.y) {
				continue
			}
			k := chave{tr.texto, math.Round(tr.x * 2), math.Round(tr.y * 2)}
			if vistos[k] {
				continue
			}
			vistos[k] = true
			p.trechosForm++
		}
		p.trechos = append(p.trechos, tr)
	}
	return p, nil
}

// extrairTextoPosicional lê os operadores de texto da página (e dos Forms que
// ela chama) e monta as linhas pela posição dos glifos: uma linha por Y, do
// topo para baixo, e trechos ordenados por X, separados por espaço só quando a
// distância entre eles o indica. Junta assim as letras de PDFs com kerning
// ("C O N T E S T A").
//
// O chamador garante que /Contents é um stream ou um array de streams legíveis
// (o lexer da biblioteca gira para sempre num stream ilegível) e recupera pânicos.
func extrairTextoPosicional(page pdf.Page) (string, error) {
	p, err := interpretarPagina(page)
	if err != nil {
		return "", err
	}
	return montarLinhas(p.trechos), nil
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
