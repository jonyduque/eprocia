package etl

import (
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
