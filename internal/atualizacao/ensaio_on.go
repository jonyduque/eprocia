//go:build ensaio

package atualizacao

import (
	"log/slog"
	"os"
	"sync"
)

// Build de ensaio da release (go build -tags ensaio): o release.yml nunca
// passa essa tag, então nada daqui existe no executável publicado. Só o nome
// da tag vem do ambiente, e só no formato vX.Y.Z-rcN; a URL continua montada
// a partir de Regras.PrefixoAPI.

var avisoEnsaio sync.Once

func tagEnsaio() (string, bool) {
	tag := os.Getenv("EPROCIA_ENSAIO_TAG")
	if !tagRC.MatchString(tag) {
		return "", false
	}
	avisoEnsaio.Do(func() {
		slog.Default().Warn("build de ensaio da atualização", "tag", tag)
	})
	return tag, true
}

func ensaioIgnoraSinal() bool { return os.Getenv("EPROCIA_ENSAIO_SEM_SINAL") == "1" }
