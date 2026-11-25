package atualizacao

import (
	"os"
	"strings"
	"testing"
)

// TestEnsaio_ReleaseSemTags garante que o build publicado não leva a tag
// ensaio: nenhum "wails build" do release.yml passa -tags.
func TestEnsaio_ReleaseSemTags(t *testing.T) {
	dados, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("ler release.yml: %v", err)
	}
	achou := 0
	for linha := range strings.Lines(string(dados)) {
		if !strings.Contains(linha, "wails build") {
			continue
		}
		achou++
		if strings.Contains(linha, "-tags") {
			t.Errorf("release.yml passa -tags ao wails build: %q", strings.TrimSpace(linha))
		}
	}
	if achou == 0 {
		t.Fatal("release.yml sem nenhuma linha com wails build")
	}
}
