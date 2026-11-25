package atualizacao

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func gravarVazio(t *testing.T, caminho string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func nomesEm(t *testing.T, dir string) []string {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, e := range entradas {
		nomes = append(nomes, e.Name())
	}
	slices.Sort(nomes)
	return nomes
}

func TestLimpar(t *testing.T) {
	inst, dados, local := t.TempDir(), t.TempDir(), t.TempDir()
	for _, n := range []string{NomeExe, NomeAnterior, NomeFalhou, PrefixoTemp + "abc", "install.ps1"} {
		gravarVazio(t, filepath.Join(inst, n))
	}
	gravarVazio(t, filepath.Join(dados, PrefixoSinal+"1"))
	gravarVazio(t, filepath.Join(dados, "config.json"))
	atual := PastaAtualizacao(local)
	if want := filepath.Join(local, "eprocIA", "atualizacao"); atual != want {
		t.Fatalf("PastaAtualizacao = %q, quer %q", atual, want)
	}
	if err := os.MkdirAll(filepath.Join(atual, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	gravarVazio(t, filepath.Join(atual, "sub", "dentro.bin"))
	gravarVazio(t, filepath.Join(atual, "solto.bin"))

	Limpar(inst, dados, atual)

	if got, want := nomesEm(t, inst), []string{NomeAnterior, NomeExe, "install.ps1"}; !slices.Equal(got, want) {
		t.Errorf("instalação = %v, quer %v", got, want)
	}
	if got, want := nomesEm(t, dados), []string{"config.json"}; !slices.Equal(got, want) {
		t.Errorf("dados = %v, quer %v", got, want)
	}
	entradas, err := os.ReadDir(atual)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entradas) != 0 {
		t.Errorf("pasta de atualização com sobras: %v", entradas)
	}
}

func TestLimpar_PastasInexistentes(t *testing.T) {
	base := t.TempDir()
	// Não pode entrar em pânico nem criar as pastas.
	Limpar(filepath.Join(base, "a"), filepath.Join(base, "b"), filepath.Join(base, "c"))
	if got := nomesEm(t, base); len(got) != 0 {
		t.Errorf("Limpar criou %v", got)
	}
}
