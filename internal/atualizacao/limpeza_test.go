package atualizacao

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"time"
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

func TestLimparAnterior(t *testing.T) {
	t.Run("apaga só o anterior", func(t *testing.T) {
		inst := t.TempDir()
		gravarVazio(t, filepath.Join(inst, NomeExe))
		gravarVazio(t, filepath.Join(inst, NomeAnterior))
		if err := LimparAnterior(inst); err != nil {
			t.Fatalf("LimparAnterior = %v", err)
		}
		if got, want := nomesEm(t, inst), []string{NomeExe}; !slices.Equal(got, want) {
			t.Errorf("instalação = %v, quer %v", got, want)
		}
	})
	t.Run("sem anterior", func(t *testing.T) {
		inst := t.TempDir()
		gravarVazio(t, filepath.Join(inst, NomeExe))
		if err := LimparAnterior(inst); err != nil {
			t.Fatalf("LimparAnterior = %v", err)
		}
		if got, want := nomesEm(t, inst), []string{NomeExe}; !slices.Equal(got, want) {
			t.Errorf("instalação = %v, quer %v", got, want)
		}
	})
	t.Run("pasta vazia", func(t *testing.T) {
		if err := LimparAnterior(""); err != nil {
			t.Fatalf("LimparAnterior(\"\") = %v", err)
		}
	})
	t.Run("anterior em execução", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("só o Windows recusa apagar um exe em execução")
		}
		inst := t.TempDir()
		anterior := filepath.Join(inst, NomeAnterior)
		dados, err := os.ReadFile(auxiliar)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(anterior, dados, 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(anterior)
		cmd.Env = append(os.Environ(), "EPROCIA_TESTE_AUX=dorme", "EPROCIA_TESTE_MS=60000", "EPROCIA_TESTE_DADOS=")
		if err := cmd.Start(); err != nil {
			t.Fatalf("iniciar o anterior: %v", err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		inicio := time.Now()
		if err := LimparAnterior(inst); err == nil {
			t.Fatal("LimparAnterior = nil com o anterior em execução")
		}
		if d := time.Since(inicio); d >= 5*time.Second {
			t.Errorf("demorou %v", d)
		}
		if _, err := os.Stat(anterior); err != nil {
			t.Errorf("o anterior sumiu: %v", err)
		}
	})
}

func TestLimparAnterior_RepeteEmViolacaoDeCompartilhamento(t *testing.T) {
	antiga := pausaTentativa
	pausaTentativa = time.Millisecond
	t.Cleanup(func() { pausaTentativa = antiga })

	t.Run("apaga na quarta", func(t *testing.T) {
		inst := t.TempDir()
		anterior := filepath.Join(inst, NomeAnterior)
		gravarVazio(t, anterior)
		chamadas := 0
		err := limparAnterior(inst, func(caminho string) error {
			chamadas++
			if caminho != anterior {
				t.Errorf("remover(%q), quer o anterior", caminho)
			}
			if chamadas <= 3 {
				return &os.PathError{Op: "remove", Path: caminho, Err: syscall.Errno(32)}
			}
			return os.Remove(caminho)
		})
		if err != nil {
			t.Fatalf("limparAnterior = %v", err)
		}
		if chamadas != 4 {
			t.Errorf("chamadas = %d, quer 4", chamadas)
		}
		if _, err := os.Lstat(anterior); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("o anterior continua (err=%v)", err)
		}
	})
	t.Run("erro sempre", func(t *testing.T) {
		inst := t.TempDir()
		chamadas := 0
		err := limparAnterior(inst, func(caminho string) error {
			chamadas++
			return &os.PathError{Op: "remove", Path: caminho, Err: syscall.Errno(32)}
		})
		if !errors.Is(err, syscall.Errno(32)) {
			t.Fatalf("limparAnterior = %v, quer o erro 32", err)
		}
		if chamadas != tentativasArquivo {
			t.Errorf("chamadas = %d, quer %d", chamadas, tentativasArquivo)
		}
	})
}
