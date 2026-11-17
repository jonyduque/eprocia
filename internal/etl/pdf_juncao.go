package etl

import (
	"strings"
	"unicode"
)

// motivoOCR diz por que uma página foi ao OCR; decide o que fazer com o texto
// nativo guardado como reserva quando o OCR termina.
type motivoOCR int

const (
	motivoPadrao           motivoOCR = iota // valor zero: texto curto ou ausente (regra de hoje) e o caminho do segundo leitor
	motivoIlegivel                          // texto nativo ilegível (regra de hoje)
	motivoImagemDescoberta                  // regra nova
	motivoIsca                              // regra nova
)

// juncaoMinPresenca é a fração das palavras de uma linha nativa que precisa
// estar no texto do OCR para a linha contar como presente.
const juncaoMinPresenca = 0.60

// juncaoMinPresencaPct é juncaoMinPresenca em pontos percentuais, para a
// comparação em inteiros.
const juncaoMinPresencaPct = int(juncaoMinPresenca * 100)

// juncaoMinRunasPalavra é o tamanho mínimo, em caracteres, de uma palavra.
const juncaoMinRunasPalavra = 3

// conteudoAposOCR decide o conteúdo de uma página cujo OCR terminou sem erro.
// Com reserva, um OCR em branco ou ilegível não a apaga: a página fica com a
// reserva e ocr == false.
func conteudoAposOCR(task ocrTask, ocrText string) (content string, ocr bool) {
	if task.fallback == "" {
		return ocrText, true
	}
	if strings.TrimSpace(ocrText) == "" || textoIlegivel(ocrText) {
		return task.fallback, false
	}
	if task.motivo == motivoImagemDescoberta {
		return juntarOCRComNativo(ocrText, task.fallback, task.linhasAcima), true
	}
	return ocrText, true
}

// palavrasDoTexto devolve as palavras de s (pedaços de letras e dígitos com
// juncaoMinRunasPalavra caracteres ou mais, normalizados), com repetição.
func palavrasDoTexto(s string) []string {
	var palavras []string
	inicio, runas := -1, 0
	fechar := func(fim int) {
		if inicio >= 0 && runas >= juncaoMinRunasPalavra {
			palavras = append(palavras, normalizarPalavra(s[inicio:fim]))
		}
		inicio, runas = -1, 0
	}
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if inicio < 0 {
				inicio = i
			}
			runas++
			continue
		}
		fechar(i)
	}
	fechar(len(s))
	return palavras
}

// linhasNativas: as linhas de nativo sem espaços nas pontas e não vazias.
func linhasNativas(nativo string) []string {
	var linhas []string
	for l := range strings.SplitSeq(nativo, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			linhas = append(linhas, l)
		}
	}
	return linhas
}

// linhasDoOCR: o texto do OCR sem o espaço do fim, cortado em \n (as linhas
// vazias do meio ficam). Um OCR só de espaços não tem linha nenhuma.
func linhasDoOCR(ocrText string) []string {
	ocrText = strings.TrimRightFunc(ocrText, unicode.IsSpace)
	if ocrText == "" {
		return nil
	}
	return strings.Split(ocrText, "\n")
}

// conjuntoDePalavras: o conjunto das palavras de s.
func conjuntoDePalavras(s string) map[string]struct{} {
	conjunto := make(map[string]struct{})
	for _, p := range palavrasDoTexto(s) {
		conjunto[p] = struct{}{}
	}
	return conjunto
}

// linhaNativaPresente diz se a linha nativa (com as palavras dela) já está no
// texto do OCR: 60% ou mais das palavras (com repetição) estão no conjunto do
// OCR; sem palavras, se o texto dela aparece no texto do OCR.
func linhaNativaPresente(linha string, palavras []string, conjuntoOCR map[string]struct{}, ocrText string) bool {
	if len(palavras) == 0 {
		return strings.Contains(ocrText, linha)
	}
	return contarPresentes(palavras, conjuntoOCR)*100 >= len(palavras)*juncaoMinPresencaPct
}

func contarPresentes(palavras []string, conjunto map[string]struct{}) int {
	n := 0
	for _, p := range palavras {
		if _, ok := conjunto[p]; ok {
			n++
		}
	}
	return n
}

// juntarOCRComNativo devolve o texto do OCR com as linhas de nativo que o OCR
// não trouxe encaixadas na ordem da página: junto da linha do OCR que casa com
// a linha nativa vizinha, ou, sem vizinha, antes do texto do OCR (linha acima
// das imagens) ou depois dele. linhasAcima diz quantas das primeiras linhas de
// nativo ficam acima das imagens. Só entra a linha nativa ausente do OCR: a que
// conta como presente (linhaNativaPresente: 60% das palavras de 3 ou mais
// caracteres aparecem no texto do OCR) não é inserida, e o pedaço dela que o
// OCR não trouxe se perde.
func juntarOCRComNativo(ocrText, nativo string, linhasAcima int) string {
	nat := linhasNativas(nativo)
	if len(nat) == 0 {
		return ocrText
	}
	conjuntoOCR := conjuntoDePalavras(ocrText)
	palavrasNat := make([][]string, len(nat))
	presente := make([]bool, len(nat))
	faltando := false
	for i, l := range nat {
		palavrasNat[i] = palavrasDoTexto(l)
		presente[i] = linhaNativaPresente(l, palavrasNat[i], conjuntoOCR, ocrText)
		faltando = faltando || !presente[i]
	}
	if !faltando {
		return ocrText
	}

	linhasOCR := linhasDoOCR(ocrText)
	a := novaAncoragem(linhasOCR)
	ancora := make([]int, len(nat)) // -2: não calculada; -1: sem âncora
	for i := range ancora {
		ancora[i] = -2
	}
	ancoraDe := func(i int) int {
		if ancora[i] == -2 {
			ancora[i] = a.ancorar(nat[i], palavrasNat[i])
		}
		return ancora[i]
	}
	acima := func(i int) bool { return i < linhasAcima }

	antes := make([][]string, len(linhasOCR))
	depois := make([][]string, len(linhasOCR))
	var comeco, fim []string
	anterior := -1 // linha nativa presente mais próxima antes da atual
	for i, l := range nat {
		if presente[i] {
			anterior = i
			continue
		}
		// 1. depois da âncora da presente anterior, do mesmo lado.
		if anterior >= 0 && acima(anterior) == acima(i) {
			if k := ancoraDe(anterior); k >= 0 {
				depois[k] = append(depois[k], l)
				continue
			}
		}
		// 2. no começo, se está acima das imagens.
		if acima(i) {
			comeco = append(comeco, l)
			continue
		}
		// 3. antes da âncora da próxima presente.
		if k := proximaAncora(presente, i, ancoraDe); k >= 0 {
			antes[k] = append(antes[k], l)
			continue
		}
		// 4. no fim.
		fim = append(fim, l)
	}

	var meio []string
	for k, l := range linhasOCR {
		meio = append(meio, antes[k]...)
		meio = append(meio, l)
		meio = append(meio, depois[k]...)
	}
	var blocos []string
	for _, b := range [][]string{comeco, meio, fim} {
		if len(b) > 0 {
			blocos = append(blocos, strings.Join(b, "\n"))
		}
	}
	return strings.Join(blocos, "\n\n")
}

// proximaAncora devolve a âncora da primeira linha nativa presente depois de
// i, ou -1.
func proximaAncora(presente []bool, i int, ancoraDe func(int) int) int {
	for j := i + 1; j < len(presente); j++ {
		if presente[j] {
			return ancoraDe(j)
		}
	}
	return -1
}

// ancoragem acha a linha do OCR que corresponde a uma linha nativa. O índice
// invertido (palavra -> linhas do OCR, sem repetição) limita o custo às linhas
// que têm alguma palavra em comum.
type ancoragem struct {
	linhas []string
	indice map[string][]int
	// contagem e tocadas são reaproveitadas entre chamadas de ancorar.
	contagem []int
	tocadas  []int
}

func novaAncoragem(linhas []string) *ancoragem {
	a := &ancoragem{linhas: linhas, indice: make(map[string][]int), contagem: make([]int, len(linhas))}
	for k, l := range linhas {
		for _, p := range palavrasDoTexto(l) {
			ls := a.indice[p]
			if len(ls) == 0 || ls[len(ls)-1] != k {
				a.indice[p] = append(ls, k)
			}
		}
	}
	return a
}

// ancorar: a linha do OCR com mais palavras distintas da linha nativa (no
// empate, a primeira); sem palavras, a primeira linha do OCR que contém o texto
// dela. -1 se nenhuma.
func (a *ancoragem) ancorar(linha string, palavras []string) int {
	if len(palavras) == 0 {
		for k, l := range a.linhas {
			if strings.Contains(l, linha) {
				return k
			}
		}
		return -1
	}
	vistas := make(map[string]struct{}, len(palavras))
	a.tocadas = a.tocadas[:0]
	for _, p := range palavras {
		if _, ok := vistas[p]; ok {
			continue
		}
		vistas[p] = struct{}{}
		for _, k := range a.indice[p] {
			if a.contagem[k] == 0 {
				a.tocadas = append(a.tocadas, k)
			}
			a.contagem[k]++
		}
	}
	melhor, melhorN := -1, 0
	for _, k := range a.tocadas {
		if n := a.contagem[k]; n > melhorN || (n == melhorN && k < melhor) {
			melhor, melhorN = k, n
		}
		a.contagem[k] = 0
	}
	return melhor
}
