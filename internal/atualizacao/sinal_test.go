package atualizacao

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSinal_GravarELer(t *testing.T) {
	dir := t.TempDir()
	if err := GravarSinal(dir, 4242, "0.5.0"); err != nil {
		t.Fatalf("GravarSinal: %v", err)
	}
	caminho := CaminhoSinal(dir, 4242)
	if want := filepath.Join(dir, "atualizacao-sinal-4242"); caminho != want {
		t.Fatalf("CaminhoSinal = %q, quer %q", caminho, want)
	}
	if _, err := os.Stat(caminho); err != nil {
		t.Fatalf("sinal não gravado: %v", err)
	}
	v, ok := lerSinal(caminho)
	if !ok || v != "0.5.0" {
		t.Fatalf("lerSinal = %q, %v; quer \"0.5.0\", true", v, ok)
	}
	if _, err := os.Stat(caminho); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sinal continua depois de lido: %v", err)
	}
	if _, ok := lerSinal(caminho); ok {
		t.Fatal("lerSinal sem arquivo devolveu ok")
	}
	sobras, _ := os.ReadDir(dir)
	if len(sobras) != 0 {
		t.Fatalf("sobrou arquivo na pasta: %v", sobras)
	}
}

func TestAposAtualizar(t *testing.T) {
	pid, ok := AposAtualizar([]string{"eprocia.exe", "--apos-atualizar=4242"})
	if !ok || pid != 4242 {
		t.Fatalf("AposAtualizar = %d, %v; quer 4242, true", pid, ok)
	}
	recusados := [][]string{
		{"eprocia.exe"},
		{"eprocia.exe", "--apos-atualizar="},
		{"eprocia.exe", "--apos-atualizar=-1"},
		{"eprocia.exe", "--apos-atualizar=0"},
		{"eprocia.exe", "--apos-atualizar=abc"},
		{"eprocia.exe", "--apos-atualizar=12 34"},
		{"eprocia.exe", "--apos-atualizar", "4242"},
		{"eprocia.exe", "--apos-atualizar 4242"},
		{"eprocia.exe", `--apos-atualizar=C:\x`},
		{"eprocia.exe", "--apos-atualizar=+5"},
		{"eprocia.exe", "--apos-atualizar=99999999999999999999999"},
	}
	for _, args := range recusados {
		if pid, ok := AposAtualizar(args); ok {
			t.Errorf("AposAtualizar(%q) aceitou %d", args, pid)
		}
	}
}

// FlagLimparAnterior é contrato entre versões: o literal fica no teste.
func TestFlagLimparAnterior_Contrato(t *testing.T) {
	if FlagLimparAnterior != "--limpar-anterior" {
		t.Fatalf("FlagLimparAnterior = %q, quer \"--limpar-anterior\"", FlagLimparAnterior)
	}
}
