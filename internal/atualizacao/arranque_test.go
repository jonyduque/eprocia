//go:build windows

package atualizacao

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// iniciarDorminhoco sobe o auxiliar no modo dorme; o cleanup encerra o filho.
func iniciarDorminhoco(t *testing.T, ms string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(auxiliar)
	cmd.Env = append(os.Environ(), "EPROCIA_TESTE_AUX=dorme", "EPROCIA_TESTE_MS="+ms)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciar auxiliar: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

func TestEsperarSair(t *testing.T) {
	t.Run("sai antes do prazo", func(t *testing.T) {
		cmd := iniciarDorminhoco(t, "300")
		inicio := time.Now()
		if !EsperarSair(cmd.Process.Pid, 5*time.Second) {
			t.Fatal("EsperarSair devolveu falso com o filho saindo em 300 ms")
		}
		if d := time.Since(inicio); d >= 5*time.Second {
			t.Fatalf("demorou %v", d)
		}
	})
	t.Run("processo já encerrado", func(t *testing.T) {
		cmd := exec.Command(auxiliar)
		cmd.Env = append(os.Environ(), "EPROCIA_TESTE_AUX=dorme", "EPROCIA_TESTE_MS=0")
		if err := cmd.Run(); err != nil {
			t.Fatalf("rodar auxiliar: %v", err)
		}
		inicio := time.Now()
		if !EsperarSair(cmd.Process.Pid, 5*time.Second) {
			t.Fatal("EsperarSair devolveu falso para processo encerrado")
		}
		if d := time.Since(inicio); d > time.Second {
			t.Fatalf("demorou %v para processo encerrado", d)
		}
	})
	t.Run("prazo estourado", func(t *testing.T) {
		cmd := iniciarDorminhoco(t, "3000")
		if EsperarSair(cmd.Process.Pid, 100*time.Millisecond) {
			t.Fatal("EsperarSair devolveu verdadeiro com o filho ainda dormindo")
		}
	})
}

func trocarPrimeiraVersao(t *testing.T, v Versao) {
	t.Helper()
	antiga := PrimeiraVersaoComProtocolo
	PrimeiraVersaoComProtocolo = v
	t.Cleanup(func() { PrimeiraVersaoComProtocolo = antiga })
}

func TestVersaoAnterior(t *testing.T) {
	trocarPrimeiraVersao(t, Versao{0, 4, 0})
	inst := t.TempDir()
	if _, ok := VersaoAnterior(inst, Versao{0, 4, 4}); ok {
		t.Fatal("sem eprocia.anterior.exe devolveu verdadeiro")
	}
	anterior := filepath.Join(inst, NomeAnterior)
	copiarAuxiliar(t, anterior, Versao{0, 4, 3})
	if v, ok := VersaoAnterior(inst, Versao{0, 4, 4}); !ok || v != (Versao{0, 4, 3}) {
		t.Fatalf("VersaoAnterior = %v, %v; quer 0.4.3, true", v, ok)
	}
	for _, v := range []Versao{{0, 4, 4}, {0, 5, 0}} {
		copiarAuxiliar(t, anterior, v)
		if got, ok := VersaoAnterior(inst, Versao{0, 4, 4}); ok {
			t.Errorf("anterior %v com atual 0.4.4 devolveu %v, true", v, got)
		}
	}
}

func TestVersaoAnterior_AntesDoProtocolo(t *testing.T) {
	if PrimeiraVersaoComProtocolo != (Versao{0, 5, 0}) {
		t.Fatalf("PrimeiraVersaoComProtocolo = %v, quer 0.5.0", PrimeiraVersaoComProtocolo)
	}
	inst := t.TempDir()
	anterior := filepath.Join(inst, NomeAnterior)
	copiarAuxiliar(t, anterior, Versao{0, 4, 4})
	if v, ok := VersaoAnterior(inst, Versao{0, 5, 0}); ok {
		t.Errorf("anterior 0.4.4 (sem protocolo) devolveu %v, true", v)
	}
	copiarAuxiliar(t, anterior, Versao{0, 5, 0})
	if v, ok := VersaoAnterior(inst, Versao{0, 5, 1}); !ok || v != (Versao{0, 5, 0}) {
		t.Errorf("VersaoAnterior = %v, %v; quer 0.5.0, true", v, ok)
	}
}
