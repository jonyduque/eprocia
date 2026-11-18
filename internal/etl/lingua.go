package etl

import (
	"strings"
	"unicode"
)

// Limiares do texto-isca: letras aleatórias postas sobre uma imagem para
// atrapalhar a leitura. A exigência de imagem fica com quem chama.
const (
	iscaMinPalavras     = 30
	iscaMinImplausiveis = 0.35
	iscaMaxFuncionais   = 0.05
)

// contagemLingua resume um texto: quantas palavras contam, quantas delas
// parecem implausíveis e quantas são palavras funcionais do português.
type contagemLingua struct{ palavras, implausiveis, funcionais int }

// isca: palavras >= 30, implausíveis >= 35% delas e funcionais < 5% delas.
// A exigência de imagem (fImg >= 0,10) fica com quem chama (Task 5).
func (c contagemLingua) isca() bool {
	if c.palavras < iscaMinPalavras {
		return false
	}
	p := float64(c.palavras)
	return float64(c.implausiveis) >= iscaMinImplausiveis*p &&
		float64(c.funcionais) < iscaMaxFuncionais*p
}

// contarLingua percorre o texto uma vez, palavra a palavra (pedaços entre
// espaços ou caracteres de controle), sem alocar por palavra.
func contarLingua(texto string) contagemLingua {
	var c contagemLingua
	inicio := -1
	for i, r := range texto {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			if inicio >= 0 {
				c.somar(texto[inicio:i])
				inicio = -1
			}
		} else if inicio < 0 {
			inicio = i
		}
	}
	if inicio >= 0 {
		c.somar(texto[inicio:])
	}
	return c
}

func ehAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// somar classifica uma palavra e atualiza as contagens.
func (c *contagemLingua) somar(w string) {
	w = strings.TrimFunc(w, func(r rune) bool { return !ehAlnum(r) })
	if w == "" {
		return
	}

	var (
		letras, latinas, vogais, digitos, trocas int
		hex                                      = true
		minusMaius                               bool
		maiusculas                               = true
		prevClasse                               int // 0 nada, 1 letra, 2 dígito
		prev                                     rune
		total                                    int
	)
	for _, r := range w {
		total++
		switch {
		case unicode.IsLetter(r):
			letras++
			if unicode.Is(unicode.Latin, r) {
				latinas++
				if ehVogal(r) {
					vogais++
				}
			}
			if !unicode.IsUpper(r) {
				maiusculas = false
			}
			if prevClasse == 2 {
				trocas++
			}
			if unicode.IsUpper(r) && unicode.IsLower(prev) {
				minusMaius = true
			}
			if !(r < 128 && strings.ContainsRune("abcdefABCDEF", r)) {
				hex = false
			}
			prevClasse = 1
		case unicode.IsDigit(r):
			digitos++
			if prevClasse == 1 {
				trocas++
			}
			prevClasse = 2
		default:
			// hífen, ponto, apóstrofo etc. no meio da palavra: separam
			// os blocos de troca, e só o hífen preserva a hipótese de hex
			// (UUID).
			if r != '-' {
				hex = false
			}
			prevClasse = 0
		}
		prev = r
	}

	// Neutras: só dígitos e pontuação, ou hexadecimal com dígito (hash).
	if letras == 0 || (hex && digitos > 0) {
		return
	}

	funcional := ehFuncional(w)
	if !funcional && total < 3 {
		return
	}
	c.palavras++
	if funcional {
		c.funcionais++
		return
	}

	implausivel := trocas >= 2 || minusMaius
	if !implausivel && latinas >= 4 && latinas == letras {
		sigla := maiusculas && letras <= 5
		if !sigla && vogais*100 < 15*latinas {
			implausivel = true
		}
	}
	if implausivel {
		c.implausiveis++
	}
}

// ehFuncional diz se w é uma das palavras funcionais do português. A
// comparação usa a forma de normalizarPalavra (minúsculas, sem acento), então
// "não", "NAO" e "nao" valem igual; a lista abaixo já está normalizada. Só
// olha palavras curtas (nenhuma passa de 5 bytes em UTF-8), então não aloca
// para as longas.
func ehFuncional(w string) bool {
	if len(w) > 5 {
		return false
	}
	switch normalizarPalavra(w) {
	case "de", "da", "do", "das", "dos", "a", "o", "as", "os", "e", "em", "no",
		"na", "nos", "nas", "para", "por", "com", "que", "nao", "ao", "aos",
		"se", "um", "uma":
		return true
	}
	return false
}

func ehVogal(r rune) bool {
	switch semAcento(unicode.ToLower(r)) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// semAcento tira o acento das letras do português (tabela própria, sem
// dependência nova). O resto passa como veio.
func semAcento(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ã', 'ä':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	}
	return r
}

// normalizarPalavra devolve a palavra em minúsculas e sem acento, por uma
// tabela própria dos acentos do português (sem dependência nova).
func normalizarPalavra(s string) string {
	return strings.Map(func(r rune) rune { return semAcento(unicode.ToLower(r)) }, s)
}
