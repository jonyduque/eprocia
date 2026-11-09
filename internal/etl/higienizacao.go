package etl

import (
	"regexp"
	"strings"
	"unicode"
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
	// "Assinado" com A maiúsculo no começo da linha; o NOME tem de 3 a 80
	// caracteres, sem vírgula, e não termina em ponto nem hífen; sem " - CARGO"
	// nem data, a linha tem de terminar no nome. Com "NOME -" no fim da linha, a
	// linha seguinte, se tiver até 40 caracteres, é o cargo (cargoLinha).
	{re: regexp.MustCompile(`(?m)\AAssinado[ \t]+(?i:digitalmente)[ \t]+por[ \t]+(?P<nome>[^\n,]{2,79}?[^\n,.\s-])(?:[ \t]+-[ \t]+(?P<cargo>[^\n]+?)|[ \t]+-[ \t]*\n(?P<cargoLinha>[^\n]{1,40}?)|[ \t]+-)?(?:[ \t]+(?:-|em)[ \t]+` + reData + `[^\n]*)?[ \t]*$`), marcador: true},
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
				// Cargo na linha seguinte: vale só se ela não for outra chancela.
				if k := c.re.SubexpIndex("cargoLinha"); k > 0 && m[k] != "" && ehChancela(m[k]) {
					m[k] = ""
					fim = i
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
