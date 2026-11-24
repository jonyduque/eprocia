package registro

import (
	"os"
	"regexp"
	"strings"
)

// sep casa um separador de caminho em qualquer grafia: \, / ou repetido
// (\\ de um caminho escapado em JSON ou em %q).
const sep = `[\\/]+`

// limite é o que pode vir depois de um prefixo de caminho para ele valer
// como segmento inteiro: qualquer caractere que não continue um nome
// (separador, aspas, pontuação, espaço) ou o fim do texto. Letra, dígito,
// '_', '~' e '-' continuam o nome: C:\Users\fulano2 não é o perfil fulano.
// Fica num grupo porque o RE2 não tem lookahead; a troca o devolve com ${1}.
const limite = `([^\p{L}\p{N}_~\-]|$)`

// usuarioEmCaminho casa o segmento depois de \Users\ (ou /Users/) quando ele
// é seguido por um separador, por ':' com espaço ou fim depois (mensagem de
// erro do Windows), por ',' ou por ')'. Um segmento no fim do texto é nome de
// arquivo e fica como está. O segmento pode ter espaço (nome de usuário com
// espaço), mas não aspas, quebra de linha nem caracteres proibidos em nome.
var usuarioEmCaminho = regexp.MustCompile(`(?i)(` + sep + `Users` + sep + `)[^\\/"'` + "`" + `<>|:*?\r\n\t]+(` + sep + `|:(?:\s|$)|[,)])`)

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
	if s.temp != nil {
		texto = s.temp.ReplaceAllString(texto, `%TEMP%${1}`)
	}
	if s.perfil != nil {
		texto = s.perfil.ReplaceAllString(texto, `%USERPROFILE%${1}`)
	}
	return usuarioEmCaminho.ReplaceAllString(texto, `${1}<usuario>${2}`)
}

// regexCaminho monta, para um caminho, uma regex sem caixa que aceita
// qualquer grafia de separador e só casa até um limite de segmento.
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
	return regexp.MustCompile(`(?i)` + padrao + limite)
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
