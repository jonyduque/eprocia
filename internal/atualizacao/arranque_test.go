//go:build windows

package atualizacao

import (
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/windows"
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

func TestLimparAnteriorPedido(t *testing.T) {
	if !LimparAnteriorPedido([]string{"eprocia.exe", "--apos-atualizar=4242", "--limpar-anterior"}) {
		t.Error("com --limpar-anterior devolveu falso")
	}
	recusados := [][]string{
		{"eprocia.exe", "--apos-atualizar=4242"},
		{"eprocia.exe", "--limpar-anterior=1"},
		{"eprocia.exe", "--LIMPAR-ANTERIOR"},
		{"eprocia.exe", "limpar-anterior"},
		{"eprocia.exe", "--limpar-anterior "},
	}
	for _, args := range recusados {
		if LimparAnteriorPedido(args) {
			t.Errorf("LimparAnteriorPedido(%q) = verdadeiro", args)
		}
	}
}

// trocarOpenProcess faz OpenProcess negar acesso para os direitos em negados
// e chama o real para os demais.
func trocarOpenProcess(t *testing.T, negados ...uint32) {
	t.Helper()
	real := openProcess
	openProcess = func(acesso uint32, herdar bool, pid uint32) (windows.Handle, error) {
		if slices.Contains(negados, acesso) {
			return 0, windows.ERROR_ACCESS_DENIED
		}
		return real(acesso, herdar, pid)
	}
	t.Cleanup(func() { openProcess = real })
}

func TestEsperarSair_AcessoNegado(t *testing.T) {
	t.Run("sonda e vê a saída antes do prazo", func(t *testing.T) {
		trocarOpenProcess(t, windows.SYNCHRONIZE)
		cmd := iniciarDorminhoco(t, "300")
		inicio := time.Now()
		if !EsperarSair(cmd.Process.Pid, 5*time.Second) {
			t.Fatal("EsperarSair devolveu falso com o filho saindo em 300 ms")
		}
		if d := time.Since(inicio); d >= 5*time.Second {
			t.Fatalf("demorou %v", d)
		}
	})
	t.Run("sonda e respeita o prazo", func(t *testing.T) {
		trocarOpenProcess(t, windows.SYNCHRONIZE)
		cmd := iniciarDorminhoco(t, "3000")
		inicio := time.Now()
		if EsperarSair(cmd.Process.Pid, 300*time.Millisecond) {
			t.Fatal("EsperarSair devolveu verdadeiro com o filho dormindo")
		}
		if d := time.Since(inicio); d < 300*time.Millisecond || d > 2*time.Second {
			t.Fatalf("voltou em %v; quer o prazo de 300 ms", d)
		}
	})
	t.Run("nada abre: espera o prazo inteiro", func(t *testing.T) {
		trocarOpenProcess(t, windows.SYNCHRONIZE, windows.PROCESS_QUERY_LIMITED_INFORMATION)
		cmd := iniciarDorminhoco(t, "3000")
		inicio := time.Now()
		if EsperarSair(cmd.Process.Pid, 300*time.Millisecond) {
			t.Fatal("EsperarSair devolveu verdadeiro sem poder abrir o processo")
		}
		if d := time.Since(inicio); d < 300*time.Millisecond || d > 2*time.Second {
			t.Fatalf("voltou em %v; quer o prazo de 300 ms", d)
		}
	})
}
