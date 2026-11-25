//go:build windows

package atualizacao

import (
	"os"
	"path/filepath"
	"testing"
)

// alvoComArquivo cria em t.TempDir() uma pasta com um arquivo e devolve os dois.
func alvoComArquivo(t *testing.T) (pasta, arquivo string) {
	t.Helper()
	pasta = t.TempDir()
	arquivo = filepath.Join(pasta, "precioso.txt")
	gravarVazio(t, arquivo)
	return pasta, arquivo
}

func TestLimpar_JuncaoDentroDaPasta(t *testing.T) {
	alvo, arquivo := alvoComArquivo(t)
	atual := PastaAtualizacao(t.TempDir())
	if err := os.MkdirAll(atual, 0o755); err != nil {
		t.Fatal(err)
	}
	criarJuncao(t, filepath.Join(atual, "juncao"), alvo)

	Limpar(t.TempDir(), t.TempDir(), atual)

	if _, err := os.Stat(arquivo); err != nil {
		t.Fatalf("alvo da junção perdeu o arquivo: %v", err)
	}
	if got := nomesEm(t, atual); len(got) != 0 {
		t.Errorf("pasta de atualização com sobras: %v", got)
	}
}

func TestLimpar_PastaDeAtualizacaoEhJuncao(t *testing.T) {
	alvo, arquivo := alvoComArquivo(t)
	atual := PastaAtualizacao(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(atual), 0o755); err != nil {
		t.Fatal(err)
	}
	criarJuncao(t, atual, alvo)

	Limpar(t.TempDir(), t.TempDir(), atual)

	if _, err := os.Stat(arquivo); err != nil {
		t.Fatalf("Limpar seguiu a junção e apagou o alvo: %v", err)
	}
}
