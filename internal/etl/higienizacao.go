package etl

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Níveis de higienização aceitos em config.Limpeza.
const (
	NivelNenhum = "none"
	NivelBasico = "basic"
	NivelPadrao = "standard"
	NivelMaximo = "aggressive"
)

// ctxDocumento é o estado de higienização por documento.
type ctxDocumento struct {
	repetidas map[string]bool
	jaVistas  map[string]bool
	html      bool
	// assinados guarda os marcadores de H6 já emitidos: um marcador idêntico
	// fica só na primeira ocorrência do documento, como o timbre.
	assinados map[string]bool
}

// linhaSeparadora informa se l tem 5 ou mais repetições de -, _, =, . , * ou •,
// com espaços opcionais entre elas, e nada mais.
func linhaSeparadora(l string) bool {
	n := 0
	for _, r := range l {
		switch r {
		case '-', '_', '=', '.', '*', '•':
			n++
		case ' ', '\t', ' ':
		default:
			return false
		}
	}
	return n >= 5
}

// linhasBasicas tira caracteres de controle (exceto \n e \t), colapsa espaços,
// tabs e NBSP, apara as linhas e descarta as vazias e as separadoras.
func linhasBasicas(text string) []string {
	var out []string
	for linha := range strings.SplitSeq(text, "\n") {
		linha = strings.Map(func(r rune) rune {
			switch {
			case r == '\t' || r == ' ' || r == '\v' || r == '\f' || r == '\u0085':
				return ' '
			case r == '\r' || unicode.IsControl(r):
				return -1
			}
			return r
		}, linha)
		linha = strings.Join(strings.Fields(linha), " ")
		if linha == "" || linhaSeparadora(linha) {
			continue
		}
		out = append(out, linha)
	}
	return out
}

// Janelas de H6: a chancela só é procurada nas últimas linhas da página e, no
// carimbo de margem do e-SAJ, também nas primeiras.
const (
	chancelaJanelaFim  = 8
	chancelaJanelaTopo = 5
)

// chancela é uma família de H6. A regex casa no começo de uma linha (\A) sobre
// as linhas da janela juntadas com "\n", então \s atravessa a quebra e [^\n]
// não. Os grupos nomeados nome, cargo e data alimentam o marcador. Uma família
// sem marcador apaga as linhas; uma secundária só deixa marcador se nenhuma
// outra chancela da página deixou. maxLinhas, se maior que zero, limita quantas
// linhas da janela a regex enxerga a partir da linha candidata.
type chancela struct {
	re         *regexp.Regexp
	marcador   bool
	secundaria bool
	topo       bool
	maxLinhas  int
}

const reData = `(?P<data>\d{2}/\d{2}/\d{4})`

var chancelas = []chancela{
	// 1. eproc: "Documento eletrônico assinado por NOME, CARGO, na forma do
	// artigo 1º, inciso III, da Lei 11.419 ... código CRC X." e o rodapé
	// "<CNJ> <N> .V<N>" da linha seguinte.
	{re: regexp.MustCompile(`(?i)\ADocumento\s+eletr[ôo]nico\s+assinado\s+por\s+(?P<nome>[^,]+?),\s+(?P<cargo>[^,]+?),\s+na\s+forma\s+do\s+artigo\s+1[º°o]?,\s+inciso\s+III,\s+da\s+Lei\s+11\.419[\s\S]*?c[óo]digo\s+CRC\s+\S+(?:\s*\n\d{7}-?\d{2}\.?\d{4}\.?\d\.?\d{2}\.?\d{4}\s+\d+\s*\.V\d+)?`), marcador: true},
	// 2. e-SAJ, na borda de baixo ou no carimbo de margem do topo.
	{re: regexp.MustCompile(`(?i)\AEste\s+documento\s+[ée]\s+c[óo]pia\s+do\s+original,\s+assinado\s+digitalmente\s+por\s+(?P<nome>[^,]+?)(?:\s+e\s+Tribunal[^,]*)?,\s+protocolado\s+em\s+` + reData + `(?:[^\n]*?\s+sob\s+o\s+n[úu]mero\s+\S+)?`), marcador: true, topo: true},
	{re: regexp.MustCompile(`(?i)\APara\s+conferir\s+o\s+original,\s+acesse\s+o\s+site(?:[^\n]*?\s+informe\s+o\s+processo[\s\S]*?\s+e\s+c[óo]digo\s+\S+)?`), topo: true, maxLinhas: 2},
	// 3. cartório: "Assinado digitalmente por NOME - CARGO" e "Protocolo nº ...
	// Documento registrado eletronicamente ...", que só vira marcador sozinho.
	// "Assinado digitalmente por" ou tudo em caixa alta, no começo da linha; o
	// NOME começa com maiúscula, tem de 3 a 80 caracteres, sem vírgula, e não
	// termina em ponto nem hífen; sem " - CARGO" nem data, a linha tem de
	// terminar no nome. Com "NOME -" no fim da linha, a linha seguinte é o cargo
	// (cargoLinha) se tiver até 40 caracteres, começar com maiúscula, não ter
	// dígito e não terminar em . ! ? ; ou :. Senão ela fica no texto.
	{re: regexp.MustCompile(`(?m)\A(?:Assinado[ \t]+digitalmente[ \t]+por|ASSINADO[ \t]+DIGITALMENTE[ \t]+POR)[ \t]+(?P<nome>\p{Lu}[^\n,]{1,78}?[^\n,.\s-])(?:[ \t]+-[ \t]+(?P<cargo>[^\n]+?)|[ \t]+-[ \t]*\n(?P<cargoLinha>\p{Lu}(?:[^\n\d]{0,38}[^\n\d.!?;:\s])?)|[ \t]+-)?(?:[ \t]+(?:-|em)[ \t]+` + reData + `[^\n]*)?[ \t]*$`), marcador: true},
	{re: regexp.MustCompile(`(?i)\A(?:Protocolo\s+n[^\n]{0,60}?)?Documento\s+registrado\s+eletronicamente(?:[^\n]*?` + reData + `)?[^\n]*`), marcador: true, secundaria: true},
	// 4. "Documento eletrônico ID assinado eletronicamente nos termos do Art.1º
	// §2º inciso III da Lei 11.419/2006 Signatário(a): NOME".
	{re: regexp.MustCompile(`(?im)\ADocumento\s+eletr[ôo]nico\s+[^\n]{0,40}?assinado\s+eletronicamente\s+nos\s+termos\s+do\s+Art\.?\s*1[º°o]?\s*§\s*2[º°o]?\s+inciso\s+III\s+da\s+Lei\s+11\.419/2006\s+Signat[áa]rio(?:\(a\))?:\s*(?P<nome>[^\n]+?)(?:\s+(?:em|Data(?:\s+e\s+Hora)?:?)\s+` + reData + `[^\n]*)?\s*$`), marcador: true},
	// 5. somem sem marcador próprio, só quando a linha termina na frase fixa:
	// com cauda (o assinante, por exemplo), a linha fica.
	{re: regexp.MustCompile(`(?im)\AEste\s+laudo\s+foi\s+assinado\s+eletronicamente[ \t]*\.?[ \t]*$`)},
	{re: regexp.MustCompile(`(?im)\AVoc[êe]\s+pode\s+conferir\s+a\s+autenticidade\s+do\s+documento\s+em\s+\S+\s+com\s+o\s+c[óo]digo\s+\S+?[ \t]*\.?[ \t]*$`)},
}

// ehChancela informa se a linha l, sozinha, casa alguma família de H6.
func ehChancela(l string) bool {
	for _, c := range chancelas {
		if c.re.MatchString(l) {
			return true
		}
	}
	return false
}

// trocaChancela diz o que fazer com as linhas de inicio a fim (inclusive):
// saem todas e, se marcador não for vazio, ele entra no lugar de inicio.
type trocaChancela struct {
	inicio, fim int
	marcador    string
	secundaria  bool
}

// grupo devolve o grupo nomeado de m, com os espaços e quebras colapsados.
func grupo(re *regexp.Regexp, m []string, nome string) string {
	i := re.SubexpIndex(nome)
	if i < 0 || i >= len(m) {
		return ""
	}
	return strings.Trim(strings.Join(strings.Fields(m[i]), " "), " ,.;")
}

// marcadorChancela monta "[assinado: NOME, CARGO, DATA]" com as partes
// presentes; em HTML, só o nome.
func marcadorChancela(re *regexp.Regexp, m []string, html bool) string {
	grupos := []string{"nome", "cargo", "data"}
	if html {
		grupos = grupos[:1]
	}
	var partes []string
	for _, g := range grupos {
		v := grupo(re, m, g)
		if v == "" && g == "cargo" {
			v = grupo(re, m, "cargoLinha")
		}
		if v != "" {
			partes = append(partes, v)
		}
	}
	if len(partes) == 0 {
		return "[assinado]"
	}
	return "[assinado: " + strings.Join(partes, ", ") + "]"
}

// acharChancelas procura as chancelas de H6 nas janelas de borda de linhas.
// Cada troca cobre linhas inteiras: a linha onde o casamento termina sai toda.
func acharChancelas(linhas []string, html bool) []trocaChancela {
	n := len(linhas)
	usada := make([]bool, n)
	var trocas []trocaChancela
	varrer := func(de, ate int, soTopo bool) {
		for i := de; i < ate; i++ {
			if usada[i] {
				continue
			}
			j := i
			for j < ate && !usada[j] {
				j++
			}
			for _, c := range chancelas {
				if soTopo && !c.topo {
					continue
				}
				lim := j
				if c.maxLinhas > 0 {
					lim = min(j, i+c.maxLinhas)
				}
				texto := strings.Join(linhas[i:lim], "\n")
				loc := c.re.FindStringSubmatchIndex(texto)
				if loc == nil {
					continue
				}
				// Cargo na linha seguinte: vale só se ela não for outra chancela;
				// se for, casa de novo só a linha i, e a seguinte fica para a
				// família dela.
				if k := c.re.SubexpIndex("cargoLinha"); k > 0 && loc[2*k] >= 0 && ehChancela(texto[loc[2*k]:loc[2*k+1]]) {
					texto = linhas[i]
					if loc = c.re.FindStringSubmatchIndex(texto); loc == nil {
						continue
					}
				}
				fim := i + strings.Count(texto[:loc[1]], "\n")
				if loc[1] > 0 && texto[loc[1]-1] == '\n' {
					fim-- // casou até a quebra: a linha seguinte não entra.
				}
				m := make([]string, len(loc)/2)
				for g := range m {
					if loc[2*g] >= 0 {
						m[g] = texto[loc[2*g]:loc[2*g+1]]
					}
				}
				t := trocaChancela{inicio: i, fim: fim, secundaria: c.secundaria}
				if c.marcador {
					t.marcador = marcadorChancela(c.re, m, html)
				}
				trocas = append(trocas, t)
				for k := i; k <= fim; k++ {
					usada[k] = true
				}
				i = fim
				break
			}
		}
	}
	varrer(max(0, n-chancelaJanelaFim), n, false)
	varrer(0, min(chancelaJanelaTopo, n), true)

	primaria := false
	for _, t := range trocas {
		if t.marcador != "" && !t.secundaria {
			primaria = true
		}
	}
	if primaria {
		for k := range trocas {
			if trocas[k].secundaria {
				trocas[k].marcador = ""
			}
		}
	}
	return trocas
}

// resumirChancelas troca as chancelas de assinatura e validação nas bordas da
// página (H6) por "[assinado: NOME, CARGO, DATA]" (em HTML, "[assinado: NOME]")
// ou as apaga, conforme a família.
func resumirChancelas(linhas []string, html bool) []string {
	saem, marcadores := planoChancelas(linhas, html)
	var out []string
	for i, l := range linhas {
		if m, ok := marcadores[i]; ok {
			out = append(out, m)
		} else if !saem[i] {
			out = append(out, l)
		}
	}
	return out
}

// planoChancelas devolve, por índice de linha, se ela sai e o marcador que
// entra no lugar dela.
func planoChancelas(linhas []string, html bool) ([]bool, map[int]string) {
	saem := make([]bool, len(linhas))
	marcadores := map[int]string{}
	for _, t := range acharChancelas(linhas, html) {
		for k := t.inicio; k <= t.fim; k++ {
			saem[k] = true
		}
		if t.marcador != "" {
			marcadores[t.inicio] = t.marcador
		}
	}
	return saem, marcadores
}

// siglasUF são as 27 siglas de unidade federativa, que H3b nunca remove.
var siglasUF = map[string]bool{
	"AC": true, "AL": true, "AP": true, "AM": true, "BA": true, "CE": true, "DF": true,
	"ES": true, "GO": true, "MA": true, "MT": true, "MS": true, "MG": true, "PA": true,
	"PB": true, "PR": true, "PE": true, "PI": true, "RJ": true, "RN": true, "RS": true,
	"RO": true, "RR": true, "SC": true, "SP": true, "SE": true, "TO": true,
}

var (
	romanoRegex = regexp.MustCompile(`^[IVXLCDM]+$`)
	alineaRegex = regexp.MustCompile(`^[a-z]\)$`)
)

// linhaLetraSolta (H3b) informa se l é ruído de OCR: 1 ou 2 letras, sem dígito
// nem outro caractere, que não são numeral romano (D de débito incluído),
// alínea, sigla de UF, crase ou "é" soltos, nem contêm º ou ª ("nº").
func linhaLetraSolta(l string) bool {
	if romanoRegex.MatchString(l) || alineaRegex.MatchString(l) || siglasUF[l] || strings.ContainsAny(l, "ºª") {
		return false
	}
	switch l {
	case "à", "À", "é", "É":
		return false
	}
	n := 0
	for _, r := range l {
		if !unicode.IsLetter(r) {
			return false
		}
		n++
	}
	return n >= 1 && n <= 2
}

// marcadorCertificado substitui, na Máxima, a página de certificado de
// assinatura de plataforma privada.
const marcadorCertificado = "[certificado de assinatura eletrônica]"

var (
	certificadoTituloRegex = regexp.MustCompile(`(?i)Termos\s+de\s+Assinatura\s+e\s+Registro\s+Eletr[ôo]nico`)
	certificadoCampoRegex  = regexp.MustCompile(`(?i)^(Enviado:|Visualizado:|Assinado:|Aceito:|N[íi]vel\s+de\s+seguran[çc]a)`)
)

// paginaCertificado informa se a página (linhas já básicas) é um certificado de
// assinatura de plataforma: traz o título "Termos de Assinatura e Registro
// Eletrônico" e pelo menos 1 linha que começa com Enviado:, Visualizado:,
// Assinado:, Aceito: ou Nível de segurança. Sem título, pede 3 ou mais dessas
// linhas, de pelo menos 2 tipos, para que uma sequência de e-mails ("Enviado:"
// repetido) não conte.
func paginaCertificado(linhas []string) bool {
	tipos := map[string]bool{}
	n := 0
	titulo := false
	for _, l := range linhas {
		if certificadoTituloRegex.MatchString(l) {
			titulo = true
		}
		if m := certificadoCampoRegex.FindStringSubmatch(l); m != nil {
			n++
			tipos[strings.ToLower(strings.Join(strings.Fields(m[1]), " "))] = true
		}
	}
	return (titulo && n >= 1) || (n >= 3 && len(tipos) >= 2)
}

// Limites de H9.
const refluirMinRunas = 40

var (
	// listaRegex: "1 -", "1.1 –", "2.", "3)", "a)", "a.", "ii)", "iv.", "IV -",
	// "•" e "-". O número ou a letra com ponto pedem espaço ou fim depois, para
	// que "8.078/90" e "art." não contem como lista.
	listaRegex = regexp.MustCompile(`^(\d+(\.\d+)*\s*[-–.)](\s|$)|(?:[a-z]|[ivxlc]+) ?[.)\-–](\s|$)|[a-z]\)|[IVXLC]+ ?[-–.)]|•|-)`)
	// dataInicioRegex: extrato que começa com data ("01/01/2026", "01/01") ou
	// competência ("01/2026").
	dataInicioRegex = regexp.MustCompile(`^(\d{1,2}/\d{1,2}(/\d{2,4})?\b|\d{2}/\d{4})`)
)

// linhaTabela informa se l parece linha de tabela ou extrato: tem R$, começa
// com data ou competência, ou tem 2 ou mais palavras com dígito.
func linhaTabela(l string) bool {
	if strings.Contains(l, "R$") || dataInicioRegex.MatchString(l) {
		return true
	}
	n := 0
	for f := range strings.FieldsSeq(l) {
		if strings.ContainsFunc(f, unicode.IsDigit) {
			n++
		}
	}
	return n >= 2
}

// linhaMarcador informa se l é um marcador da higienização ("[assinado: ...]",
// "[certificado ...]", referências), que nunca reflui.
func linhaMarcador(l string) bool {
	return strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]")
}

// juncao é o que H9 faz com a linha seguinte.
type juncao int

const (
	naoJunta    juncao = iota
	juntaEspaco        // parágrafo: junta com um espaço
	juntaHifen         // fim em hífen: junta sem espaço e mantém o hífen
)

// antesDeNumero são as palavras finais depois das quais uma linha que começa
// com dígito continua a frase ("art." + "5º").
var antesDeNumero = map[string]bool{"nº": true, "n.": true, "art.": true, "arts.": true, "lei": true, "inciso": true, "§": true}

// fimDeFrase informa se l termina em . : ; ? !, ignorando aspas, parênteses e
// colchetes de fechamento depois da pontuação.
func fimDeFrase(l string) bool {
	l = strings.TrimRight(l, `"'”’»)]`)
	r, _ := utf8.DecodeLastRuneInString(l)
	return strings.ContainsRune(".:;?!", r)
}

// ultimaPalavra devolve a última palavra de l.
func ultimaPalavra(l string) string {
	if i := strings.LastIndexByte(l, ' '); i >= 0 {
		return l[i+1:]
	}
	return l
}

// juntaLinhas diz como a linha b continua a linha a (H9). Na dúvida, não junta.
func juntaLinhas(a, b string) juncao {
	if a == "" || b == "" || linhaMarcador(a) || linhaMarcador(b) || linhaTabela(a) || linhaTabela(b) || listaRegex.MatchString(b) {
		return naoJunta
	}
	prim, _ := utf8.DecodeRuneInString(b)
	if semHifen, ok := strings.CutSuffix(a, "-"); ok {
		antes, _ := utf8.DecodeLastRuneInString(semHifen)
		if unicode.IsLetter(antes) && unicode.IsLower(prim) {
			return juntaHifen
		}
		return naoJunta
	}
	if utf8.RuneCountInString(a) < refluirMinRunas {
		return naoJunta
	}
	if unicode.IsDigit(prim) {
		if strings.HasSuffix(a, ",") || antesDeNumero[strings.ToLower(ultimaPalavra(a))] {
			return juntaEspaco
		}
		return naoJunta
	}
	if fimDeFrase(a) || !unicode.IsLower(prim) {
		return naoJunta
	}
	return juntaEspaco
}

// refluir (H9) junta as linhas de um mesmo parágrafo. A linha i+1 entra na i
// quando nenhuma das duas parece tabela (R$, data ou competência no começo, ou
// 2+ palavras com dígito) nem é marcador, a i+1 não começa com marcador de
// lista, a i tem 40 ou mais runas e:
//   - a i+1 começa com minúscula e a i não termina em . : ; ? ! (aspas e
//     parêntese de fechamento não contam); ou
//   - a i+1 começa com dígito e a i termina em vírgula ou em nº, n., art.,
//     arts., Lei, inciso ou §.
//
// Uma linha que termina em letra seguida de hífen junta com a próxima, que
// começa com minúscula, sem espaço e mantendo o hífen ("pala-vra",
// "trata-se"): separar a quebra silábica do hífen legítimo pede dicionário, e
// o hífen a mais não atrapalha a leitura. As regras olham as linhas
// originais, não o parágrafo já juntado.
func refluir(linhas []string) []string {
	var out []string
	for i, l := range linhas {
		if i > 0 {
			ult := &out[len(out)-1]
			switch juntaLinhas(linhas[i-1], l) {
			case juntaEspaco:
				*ult += " " + l
				continue
			case juntaHifen:
				*ult += l
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

// linhaSoSimbolos informa se l não tem letra nem dígito e não contém §, º, ª ou %.
func linhaSoSimbolos(l string) bool {
	if strings.ContainsAny(l, "§ºª%") {
		return false
	}
	for _, r := range l {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
