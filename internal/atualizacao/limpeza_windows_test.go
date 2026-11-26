//go:build windows

package atualizacao

import (
	"bytes"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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

func TestLimpar_ArquivoQueFicaNaoLogaCaminho(t *testing.T) {
	buf := capturarLogPadrao(t)
	dir := t.TempDir()
	preso := filepath.Join(dir, NomeFalhou)
	gravarVazio(t, preso)
	// Aberto sem compartilhamento: o RemoveAll falha com um erro que carrega
	// o caminho.
	nome, err := windows.UTF16PtrFromString(preso)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(nome, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)

	Limpar(dir, "", "")

	saida := buf.String()
	if !strings.Contains(saida, "arquivo ficou") {
		t.Fatalf("sem o aviso esperado no log: %q", saida)
	}
	if temCaminho(saida, dir) {
		t.Errorf("log com o caminho cru: %q", saida)
	}
	if !strings.Contains(saida, "codigo=") {
		t.Errorf("log sem o código do sistema: %q", saida)
	}
}

// temCaminho procura no log o trecho único do t.TempDir() (o nome da pasta do
// teste), que sobrevive às barras escapadas do TextHandler.
func temCaminho(saida, dir string) bool {
	return strings.Contains(saida, filepath.Base(filepath.Dir(dir)))
}

func TestLimpar_PastaNaoLidaNaoLogaCaminho(t *testing.T) {
	buf := capturarLogPadrao(t)
	dir := t.TempDir()
	// Nome inválido no Windows: ReadDir falha com ERROR_INVALID_NAME, que
	// carrega o caminho e não é ErrNotExist.
	invalida := filepath.Join(dir, "a<b")

	Limpar(invalida, "", "")

	saida := buf.String()
	if !strings.Contains(saida, "pasta não lida") {
		t.Fatalf("sem o aviso esperado no log: %q", saida)
	}
	if temCaminho(saida, dir) {
		t.Errorf("log com o caminho cru: %q", saida)
	}
	if !strings.Contains(saida, "codigo=") {
		t.Errorf("log sem o código do sistema: %q", saida)
	}
}

// capturarLogPadrao troca o slog.Default() por um que escreve num buffer e
// restaura, no fim do teste, o logger e a saída do pacote log (SetDefault
// também redireciona o log padrão).
func capturarLogPadrao(t *testing.T) *bytes.Buffer {
	t.Helper()
	anterior, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(anterior)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}
