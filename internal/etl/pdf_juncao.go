package etl

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// motivoOCR diz por que uma página foi ao OCR; decide o que fazer com o texto
// nativo guardado como reserva quando o OCR termina.
type motivoOCR int

const (
	motivoPadrao           motivoOCR = iota // valor zero: texto curto ou ausente (regra de hoje) e o caminho do segundo leitor
	motivoIlegivel                          // texto nativo ilegível (regra de hoje)
	motivoImagemDescoberta                  // regra nova
	motivoIsca                              // regra nova
	motivoForcado                           // Forçar OCR: página que analisarPagina não mandou ao OCR
)

// juncaoMinRunasPalavra é o tamanho mínimo, em caracteres, de uma palavra.
const juncaoMinRunasPalavra = 3

// conteudoAposOCR decide o conteúdo de uma página cujo OCR terminou sem erro.
// Com reserva, um OCR em branco ou ilegível não a apaga: a página fica com a
// reserva e ocr == false. motivoForcado segue a regra de motivoPadrao (o OCR
// legível substitui o nativo, sem junção), mas nunca piora a página: mesmo
// com a reserva vazia, um OCR em branco ou ilegível deixa o nativo.
func conteudoAposOCR(task ocrTask, ocrText string) (content string, ocr bool) {
	if task.fallback == "" && task.motivo != motivoForcado {
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
// Servem à âncora.
func palavrasDoTexto(s string) []string {
	return pedacosDoTexto(s, false)
}

// fichasDoTexto devolve as fichas de s: as palavras de palavrasDoTexto mais os
// pedaços curtos que têm algum dígito (dia, mês, centavos), com repetição.
// Servem à presença da linha nativa.
func fichasDoTexto(s string) []string {
	return pedacosDoTexto(s, true)
}

// pedacosDoTexto corta s em pedaços de letras e dígitos e guarda, normalizados,
// os de juncaoMinRunasPalavra runas ou mais e, com curtosComDigito, também os
// mais curtos que têm algum dígito.
func pedacosDoTexto(s string, curtosComDigito bool) []string {
	var pedacos []string
	inicio, runas, digito := -1, 0, false
	fechar := func(fim int) {
		if inicio >= 0 && (runas >= juncaoMinRunasPalavra || (curtosComDigito && digito)) {
			pedacos = append(pedacos, normalizarPalavra(s[inicio:fim]))
		}
		inicio, runas, digito = -1, 0, false
	}
	for i, r := range s {
		ehDigito := unicode.IsDigit(r)
		if ehDigito || unicode.IsLetter(r) {
			if inicio < 0 {
				inicio = i
			}
			runas++
			digito = digito || ehDigito
			continue
		}
		fechar(i)
	}
	fechar(len(s))
	return pedacos
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

// contarFichas: quantas vezes cada ficha aparece.
func contarFichas(fichas []string) map[string]int {
	contagem := make(map[string]int, len(fichas))
	for _, f := range fichas {
		contagem[f]++
	}
	return contagem
}

// linhaNativaPresente diz se a linha nativa (com as fichas dela) já está no
// texto do OCR e, se está, consome as ocorrências que usou: cada ficha da linha,
// com as vezes que ela aparece na linha, precisa de ocorrências próprias no OCR
// (noOCR) além das já consumidas pelas linhas presentes anteriores (consumido).
// Sem fichas, vale se o texto dela aparece no texto do OCR. A linha ausente não
// consome nada.
//
// Exigir todas as palavras (e não uma fração) evita perder o pedaço que o OCR
// não leu, quase sempre um número (decisão do dono em 2026-11-17, medida em
// ZIPs reais: com 60%, 182 de 4.971 palavras nativas sumiam; com todas,
// nenhuma, ao custo de 4,7% de texto a mais nessas páginas). Contar as
// ocorrências e incluir os pedaços curtos com dígito (regra R100m, medida em
// 2026-11-22 em 117 páginas reais) fecha as duas perdas que sobravam, uma
// ocorrência do OCR cobrindo várias linhas e o dia ou o mês lido errado:
// palavras nativas perdidas 18 -> 0 e fichas com dígito perdidas 32 -> 0, ao
// custo de 9 linhas inseridas a mais.
func linhaNativaPresente(linha string, fichas []string, noOCR, consumido map[string]int, ocrText string) bool {
	if len(fichas) == 0 {
		return strings.Contains(ocrText, linha)
	}
	naLinha := contarFichas(fichas)
	for f, v := range naLinha {
		if noOCR[f] < consumido[f]+v {
			return false
		}
	}
	for f, v := range naLinha {
		consumido[f] += v
	}
	return true
}

// juntarOCRComNativo devolve o texto do OCR com as linhas de nativo que o OCR
// não trouxe encaixadas na ordem da página: junto da linha do OCR que casa com
// a linha nativa vizinha, ou, sem vizinha, antes do texto do OCR (linha acima
// das imagens) ou depois dele. linhasAcima diz quantas das primeiras linhas de
// nativo ficam acima das imagens. Só entra a linha nativa ausente do OCR: a que
// conta como presente (linhaNativaPresente: cada ficha da linha, palavra de 3
// ou mais caracteres ou pedaço com dígito, tem uma ocorrência própria no texto
// do OCR, consumida na ordem das linhas nativas) não é inserida. Uma linha
// inserida pode repetir parte do que o OCR já trouxe.
//
// Toda comparação (fichas, Contains, âncora) usa o texto em NFC, para que um
// acento decomposto (NFD) não parta a palavra nas marcas; o que sai são sempre
// as linhas originais do OCR e da página nativa.
func juntarOCRComNativo(ocrText, nativo string, linhasAcima int) string {
	nat := linhasNativas(nativo)
	if len(nat) == 0 {
		return ocrText
	}
	ocrNFC := emNFC(ocrText)
	noOCR := contarFichas(fichasDoTexto(ocrNFC))
	consumido := make(map[string]int)
	natNFC := make([]string, len(nat))
	palavrasNat := make([][]string, len(nat))
	presente := make([]bool, len(nat))
	faltando := false
	for i, l := range nat {
		natNFC[i] = emNFC(l)
		palavrasNat[i] = palavrasDoTexto(natNFC[i])
		presente[i] = linhaNativaPresente(natNFC[i], fichasDoTexto(natNFC[i]), noOCR, consumido, ocrNFC)
		faltando = faltando || !presente[i]
	}
	if !faltando {
		return ocrText
	}

	linhasOCR := linhasDoOCR(ocrText)
	linhasOCRNFC := make([]string, len(linhasOCR))
	for k, l := range linhasOCR {
		linhasOCRNFC[k] = emNFC(l)
	}
	a := novaAncoragem(linhasOCRNFC)
	ancora := make([]int, len(nat)) // -2: não calculada; -1: sem âncora
	for i := range ancora {
		ancora[i] = -2
	}
	ancoraDe := func(i int) int {
		if ancora[i] == -2 {
			ancora[i] = a.ancorar(natNFC[i], palavrasNat[i])
		}
		return ancora[i]
	}
	prox := proximasPresentes(presente)
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
		if prox[i] >= 0 {
			if k := ancoraDe(prox[i]); k >= 0 {
				antes[k] = append(antes[k], l)
				continue
			}
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

// emNFC devolve s na forma NFC, sem converter o que já está nela.
func emNFC(s string) string {
	if norm.NFC.IsNormalString(s) {
		return s
	}
	return norm.NFC.String(s)
}

// proximasPresentes devolve, para cada i, o índice da primeira linha j > i com
// presente[j], ou -1. Uma passada de trás para frente.
func proximasPresentes(presente []bool) []int {
	prox := make([]int, len(presente))
	proxima := -1
	for i := len(presente) - 1; i >= 0; i-- {
		prox[i] = proxima
		if presente[i] {
			proxima = i
		}
	}
	return prox
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
