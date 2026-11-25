package atualizacao

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestRede_SoAtualizacaoImportaNetHTTP garante que nenhum outro pacote do
// módulo importa net/http diretamente.
func TestRede_SoAtualizacaoImportaNetHTTP(t *testing.T) {
	cmd := exec.Command("go", "list", "-e", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = "../.."
	saida, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("go list: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	linhas := strings.Split(strings.TrimSpace(string(saida)), "\n")
	if len(linhas) < 5 {
		t.Fatalf("go list devolveu só %d pacotes", len(linhas))
	}
	viuEste := false
	for _, l := range linhas {
		campos := strings.Fields(l)
		if len(campos) == 0 {
			continue
		}
		pkg, imports := campos[0], campos[1:]
		if pkg == "eprocia/internal/atualizacao" {
			viuEste = true
			continue
		}
		if slices.Contains(imports, "net/http") {
			t.Errorf("%s importa net/http; só eprocia/internal/atualizacao pode", pkg)
		}
	}
	if !viuEste {
		t.Fatal("go list não listou eprocia/internal/atualizacao")
	}
}
