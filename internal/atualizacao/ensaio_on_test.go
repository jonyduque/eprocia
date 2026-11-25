//go:build ensaio && windows

package atualizacao

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEnsaio_ConsultaTagRC(t *testing.T) {
	const tag = "v0.5.0-rc1"
	t.Setenv("EPROCIA_ENSAIO_TAG", tag)

	t.Run("aceita", func(t *testing.T) {
		c := novoCenario(t)
		c.rel = c.releaseCompleta(tag, "0.5.0-rc1")
		c.rel.Prerelease = true
		rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
		if err != nil || rel == nil {
			t.Fatalf("Verificar = (%+v, %v)", rel, err)
		}
		if c.caminho != "/api/releases/tags/"+tag {
			t.Errorf("caminho = %q", c.caminho)
		}
		if rel.Tag != tag || rel.Versao != (Versao{0, 5, 0}) {
			t.Errorf("release = %+v", rel)
		}
		if rel.Exe.Nome != "eprocia-0.5.0-rc1-windows-amd64.exe" ||
			rel.Instalador.Nome != "eprocia-0.5.0-rc1-windows-amd64-instalador.exe" ||
			rel.Somas.Nome != nomeSomas {
			t.Errorf("assets = %+v %+v %+v", rel.Exe, rel.Instalador, rel.Somas)
		}
	})

	t.Run("draft", func(t *testing.T) {
		c := novoCenario(t)
		c.rel = c.releaseCompleta(tag, "0.5.0-rc1")
		c.rel.Prerelease = true
		c.rel.Draft = true
		rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
		if rel != nil || err != nil {
			t.Fatalf("Verificar = (%+v, %v), quer (nil, nil)", rel, err)
		}
	})

	t.Run("asset de outra rc", func(t *testing.T) {
		c := novoCenario(t)
		c.rel = c.releaseCompleta(tag, "0.5.0-rc1")
		c.rel.Prerelease = true
		c.rel.Assets[0] = c.assetDe(tag, "eprocia-0.5.0-rc2-windows-amd64.exe", 10)
		_, err := c.cliente.Verificar(context.Background(), versaoTeste)
		if !errors.Is(err, ErrIncompleta) {
			t.Fatalf("erro = %v, quer ErrIncompleta", err)
		}
	})

	t.Run("tag da resposta difere da pedida", func(t *testing.T) {
		c := novoCenario(t)
		c.rel = c.releaseCompleta("v0.5.0-rc2", "0.5.0-rc2")
		c.rel.Prerelease = true
		rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
		if err == nil || rel != nil {
			t.Fatalf("Verificar = (%+v, %v), quer erro", rel, err)
		}
		if errors.Is(err, ErrIncompleta) || errors.Is(err, ErrRecusada) {
			t.Fatalf("erro = %v, quer erro comum", err)
		}
	})

	t.Run("sinal", func(t *testing.T) {
		if ensaioIgnoraSinal() {
			t.Error("ensaioIgnoraSinal() verdadeiro sem a variável")
		}
		t.Setenv("EPROCIA_ENSAIO_SEM_SINAL", "1")
		if !ensaioIgnoraSinal() {
			t.Error("ensaioIgnoraSinal() falso com EPROCIA_ENSAIO_SEM_SINAL=1")
		}
	})
}

func TestEnsaio_TagForaDoFormato(t *testing.T) {
	for _, tag := range []string{"v0.5.0", "v0.5.0-beta", "../x", ""} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("EPROCIA_ENSAIO_TAG", tag)
			c := novoCenario(t)
			c.rel = c.releaseCompleta("v0.5.0", "0.5.0")
			rel, err := c.cliente.Verificar(context.Background(), versaoTeste)
			if err != nil || rel == nil {
				t.Fatalf("Verificar = (%+v, %v)", rel, err)
			}
			if c.caminho != "/api/releases/latest" {
				t.Errorf("caminho = %q, quer /api/releases/latest", c.caminho)
			}
		})
	}
}

// No build de ensaio com EPROCIA_ENSAIO_SEM_SINAL=1 a troca ignora o sinal
// (o novo grava e fica no ar) e reverte no prazo.
func TestAplicar_EnsaioIgnoraSinal(t *testing.T) {
	t.Setenv("EPROCIA_ENSAIO_SEM_SINAL", "1")
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	err := c.troca(time.Second).Aplicar(context.Background(), b)
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Aplicar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
	conferirRevertido(t, c, antes, v050)
	naoExiste(t, c.caminho(NomeAnterior))
}
