package registro

import (
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// sep casa um separador de caminho em qualquer grafia: \, / ou repetido
// (\\ de um caminho escapado em JSON ou em %q).
const sep = `[\\/]+`

// paradasSegmento são os caracteres que encerram um segmento de caminho num
// texto de log sem ser separador: aspas, proibidos em nome, quebra de linha,
// ',' e ')'.
const paradasSegmento = ":\"'`<>|*?\r\n\t,)"

// usuarioEmCaminho casa o segmento depois de \Users\ (ou /Users/) quando ele
// é seguido por um separador, por ':' com espaço ou fim depois (mensagem de
// erro do Windows), por ',' ou por ')'. O quantificador é preguiçoso: o
// segmento acaba no primeiro terminador, não no último da linha. Um segmento
// no fim do texto é nome de arquivo e fica como está. O segmento pode ter
// espaço (nome de usuário com espaço), mas não aspas, quebra de linha nem
// caracteres proibidos em nome.
var usuarioEmCaminho = regexp.MustCompile(`(?i)(` + sep + `Users` + sep + `)[^\\/"'` + "`" + `<>|:*?\r\n\t]+?(` + sep + `|:(?:\s|$)|[,)])`)

// Saneador troca, num texto de log, o perfil e o diretório temporário do
// usuário do Windows por %USERPROFILE% e %TEMP%, e o nome do usuário que
// aparece como segmento de caminho depois de \Users\ por <usuario>.
type Saneador struct {
	temp   *regexp.Regexp
	perfil *regexp.Regexp
	// tempSemUsers e perfilSemUsers: o caminho não fica sob \Users\, então
	// um texto sem "users" ainda pode contê-lo.
	tempSemUsers   bool
	perfilSemUsers bool
}

// NovoSaneador prepara o saneamento para o perfil e o temporário dados.
// Um caminho vazio não é trocado.
func NovoSaneador(perfil, temp string) *Saneador {
	s := &Saneador{
		temp:   regexCaminho(temp),
		perfil: regexCaminho(perfil),
	}
	s.tempSemUsers = s.temp != nil && !contemUsers(temp)
	s.perfilSemUsers = s.perfil != nil && !contemUsers(perfil)
	return s
}

// SaneadorDoSistema usa o perfil e o temporário do usuário atual.
func SaneadorDoSistema() *Saneador {
	perfil, err := os.UserHomeDir()
	if err != nil {
		perfil = ""
	}
	return NovoSaneador(perfil, os.TempDir())
}

// Sanear devolve o texto sem o perfil, o temporário nem o nome do usuário.
// O temporário vem antes do perfil porque costuma ficar dentro dele.
func (s *Saneador) Sanear(texto string) string {
	if !contemUsers(texto) &&
		!(s.tempSemUsers && s.temp.MatchString(texto)) &&
		!(s.perfilSemUsers && s.perfil.MatchString(texto)) {
		return texto
	}
	texto = trocarCaminho(s.temp, texto, "%TEMP%")
	texto = trocarCaminho(s.perfil, texto, "%USERPROFILE%")
	return usuarioEmCaminho.ReplaceAllString(texto, `${1}<usuario>${2}`)
}

// trocarCaminho troca por troca cada ocorrência de re que termina num limite
// de segmento (terminaSegmento). O RE2 não tem lookahead, por isso o limite
// é conferido aqui e não na regex.
func trocarCaminho(re *regexp.Regexp, texto, troca string) string {
	if re == nil {
		return texto
	}
	var b strings.Builder
	ult := 0
	for _, m := range re.FindAllStringIndex(texto, -1) {
		if !terminaSegmento(texto[m[1]:]) {
			continue
		}
		b.WriteString(texto[ult:m[0]])
		b.WriteString(troca)
		ult = m[1]
	}
	if ult == 0 {
		return texto
	}
	b.WriteString(texto[ult:])
	return b.String()
}

// terminaSegmento diz se o caminho casado termina um segmento, dado o resto
// do texto. Fim do texto, separador e pontuação terminam. Letra, dígito, '_',
// '~' e '-' continuam o nome (C:\Users\fulano2 não é o perfil fulano). '.' e
// espaço são ambíguos: terminam (fim de frase, "fulano: erro"), salvo quando
// o resto do segmento chega a um separador, caso em que o segmento é outro
// nome (fulano.bak\x, Fulano de Tal\x) e fica para a regex de <usuario>.
func terminaSegmento(resto string) bool {
	if resto == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(resto)
	switch {
	case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '~' || r == '-':
		return false
	case r != '.' && r != ' ':
		return true
	}
	for i := 0; i < len(resto); i++ {
		c := resto[i]
		if c == '\\' || c == '/' {
			return false
		}
		if strings.IndexByte(paradasSegmento, c) >= 0 {
			return true
		}
	}
	return true
}

// regexCaminho monta, para um caminho, uma regex sem caixa que aceita
// qualquer grafia de separador. O limite de segmento fica em terminaSegmento.
func regexCaminho(caminho string) *regexp.Regexp {
	partes := strings.FieldsFunc(caminho, func(r rune) bool { return r == '\\' || r == '/' })
	if len(partes) == 0 {
		return nil
	}
	for i, p := range partes {
		partes[i] = regexp.QuoteMeta(p)
	}
	padrao := strings.Join(partes, sep)
	if strings.ContainsAny(caminho[:1], `\/`) {
		padrao = sep + padrao // UNC ou caminho absoluto sem unidade
	}
	return regexp.MustCompile(`(?i)` + padrao)
}

// contemUsers diz, sem alocar, se o texto contém "users" em qualquer caixa.
func contemUsers(texto string) bool {
	const alvo = "users"
	for i := 0; i+len(alvo) <= len(texto); i++ {
		if (texto[i]|0x20) == 'u' && strings.EqualFold(texto[i:i+len(alvo)], alvo) {
			return true
		}
	}
	return false
}
