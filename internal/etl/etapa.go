package etl

import (
	"errors"

	"eprocia/internal/ocr"
)

// Etapas do processamento, para o atributo etapa do log.
const (
	EtapaAbrirZip = "abrir_zip"
	EtapaLerPDF   = "ler_pdf"
	EtapaOCR      = "ocr"
	EtapaHTML     = "html"
	EtapaFormatar = "formatar"
	EtapaGravar   = "gravar"
)

// ErroEtapa marca em que etapa do processamento um erro aconteceu. Não muda o
// texto do erro: Error() é o de Err, e a linha e a notificação seguem iguais.
type ErroEtapa struct {
	Etapa string
	Err   error
}

func (e ErroEtapa) Error() string {
	if e.Err == nil {
		return e.Etapa
	}
	return e.Err.Error()
}

func (e ErroEtapa) Unwrap() error { return e.Err }

// comEtapa embrulha err na etapa dada; nil continua nil.
func comEtapa(etapa string, err error) error {
	if err == nil {
		return nil
	}
	return ErroEtapa{Etapa: etapa, Err: err}
}

// atributosErro devolve os atributos de log de um erro: erro (a cadeia
// completa), etapa (se houver ErroEtapa), classe (classificarErro) e, de um
// processo externo, codigo_saida e stderr. O stderr só chega ao log por aqui:
// o Error() do ocr.ErroProcesso não o leva.
func atributosErro(err error) []any {
	if err == nil {
		return nil
	}
	attrs := []any{"erro", err}
	if ee, ok := errors.AsType[ErroEtapa](err); ok {
		attrs = append(attrs, "etapa", ee.Etapa)
	}
	attrs = append(attrs, "classe", string(classificarErro(err)))
	if ep, ok := errors.AsType[*ocr.ErroProcesso](err); ok {
		attrs = append(attrs, "codigo_saida", ep.Codigo)
		if ep.Stderr != "" {
			attrs = append(attrs, "stderr", ep.Stderr)
		}
	}
	return attrs
}
