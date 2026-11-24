// Package registro concentra o log em arquivo do eprocIA: a leitura única da
// variável EPROCIA_LOG, o saneamento do perfil do usuário e, nas próximas
// etapas, o manipulador slog, a rotação e a retenção.
package registro

import "strings"

// Modo é o efeito da variável de ambiente EPROCIA_LOG.
type Modo int

const (
	// ModoDesligado: variável ausente ou vazia; nada de log nem telemetria.
	ModoDesligado Modo = iota
	// ModoNormal: EPROCIA_LOG=1 (ou valor não reconhecido); nível INFO.
	ModoNormal
	// ModoDetalhado: EPROCIA_LOG=detalhado; nível DEBUG, com registro por página.
	ModoDetalhado
)

// LerModo interpreta o valor de EPROCIA_LOG. Um valor não reconhecido vale
// como ModoNormal com reconhecido=false, para quem chama registrar um aviso.
func LerModo(valor string) (m Modo, reconhecido bool) {
	v := strings.TrimSpace(valor)
	switch {
	case v == "":
		return ModoDesligado, true
	case v == "1":
		return ModoNormal, true
	case strings.EqualFold(v, "detalhado"):
		return ModoDetalhado, true
	}
	return ModoNormal, false
}
