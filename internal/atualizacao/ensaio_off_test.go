//go:build !ensaio

package atualizacao

import (
	"context"
	"testing"
)

// TestEnsaio_ProducaoIgnoraVariavel: sem a tag de build, as variáveis do
// ensaio não mudam nada.
func TestEnsaio_ProducaoIgnoraVariavel(t *testing.T) {
	t.Setenv("EPROCIA_ENSAIO_TAG", "v0.5.0-rc1")
	t.Setenv("EPROCIA_ENSAIO_SEM_SINAL", "1")
	c := novoCenario(t)
	c.rel = c.releaseCompleta("v0.5.0-rc1", "0.5.0-rc1")
	c.rel.Prerelease = true
	rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
	if rel != nil || err != nil {
		t.Fatalf("Verificar = (%+v, %v), quer (nil, nil)", rel, err)
	}
	if c.caminho != "/api/releases/latest" {
		t.Errorf("caminho = %q, quer /api/releases/latest", c.caminho)
	}
	if ensaioIgnoraSinal() {
		t.Error("ensaioIgnoraSinal() verdadeiro no build de produção")
	}
}
