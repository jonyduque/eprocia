//go:build windows

package atualizacao

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersaoPE_Carimbada(t *testing.T) {
	for _, s := range []string{"0.5.0", "0.4.9"} {
		t.Run(s, func(t *testing.T) {
			v, err := ParseVersao(s)
			if err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(t.TempDir(), "eprocia.exe")
			copiarAuxiliar(t, exe, v)
			lida, err := VersaoPE(exe)
			if err != nil {
				t.Fatalf("VersaoPE: %v", err)
			}
			if lida != v {
				t.Fatalf("VersaoPE = %v, quer %v", lida, v)
			}
		})
	}
}

func TestVersaoPE_SemRecurso(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "eprocia.exe")
	copiarArquivo(t, auxiliar, exe)
	if v, err := VersaoPE(exe); err == nil {
		t.Fatalf("VersaoPE sem recurso = %v, quer erro", v)
	}
}

func TestVersaoPE_NaoPE(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "texto.exe")
	if err := os.WriteFile(arq, []byte("FULANO DE TAL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := VersaoPE(arq); err == nil {
		t.Fatalf("VersaoPE de texto = %v, quer erro", v)
	}
}

func TestCarimbo_AuxiliarContinuaExecutando(t *testing.T) {
	v := Versao{0, 5, 0}
	exe := filepath.Join(t.TempDir(), "eprocia.exe")
	copiarAuxiliar(t, exe, v)
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Env = append(os.Environ(), "EPROCIA_TESTE_AUX=versao")
	var erro strings.Builder
	cmd.Stderr = &erro
	saida, err := cmd.Output()
	if err != nil {
		t.Fatalf("auxiliar carimbado: %v\nstderr: %s", err, erro.String())
	}
	if got := strings.TrimSpace(string(saida)); got != v.String() {
		t.Fatalf("auxiliar imprimiu %q, quer %q", got, v.String())
	}
}
