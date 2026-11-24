package registro

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// linha40 devolve uma linha de exatamente 40 bytes, com o número n.
func linha40(n int) string {
	s := fmt.Sprintf("linha %02d ", n)
	return s + strings.Repeat("x", 39-len(s)) + "\n"
}

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler %s: %v", caminho, err)
	}
	return string(b)
}

func TestArquivoRotativo_GiraNoLimite(t *testing.T) {
	dir := t.TempDir()
	a, err := AbrirRotativo(dir, "x", ".log", 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Cada arquivo guarda duas linhas de 40 bytes (a terceira passaria de
	// 100): com 3 arquivados cabem 8 linhas, então 9 linhas descartam as
	// duas mais antigas.
	const total = 9
	for i := 1; i <= total; i++ {
		if _, err := a.Write([]byte(linha40(i))); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	ativo := filepath.Join(dir, "x.log")
	info, err := os.Stat(ativo)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() >= 100 {
		t.Errorf("x.log tem %d bytes, quer menos de 100", info.Size())
	}
	for _, n := range []string{"x.1.log", "x.2.log", "x.3.log"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s deveria existir: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "x.4.log")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("x.4.log não deveria existir: %v", err)
	}
	var tudo strings.Builder
	for _, n := range []string{"x.log", "x.1.log", "x.2.log", "x.3.log"} {
		tudo.WriteString(lerArquivo(t, filepath.Join(dir, n)))
	}
	if strings.Contains(tudo.String(), "linha 01") {
		t.Error("a linha mais antiga deveria ter sumido")
	}
	if !strings.Contains(lerArquivo(t, ativo), fmt.Sprintf("linha %02d", total)) {
		t.Error("a linha mais nova deveria estar em x.log")
	}
}

func TestArquivoRotativo_RenomearFalhaSegueGravando(t *testing.T) {
	dir := t.TempDir()
	a, err := AbrirRotativo(dir, "x", ".log", 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	tentativas := 0
	a.renomear = func(de, para string) error {
		tentativas++
		return errors.New("arquivo aberto em outro programa")
	}
	for i := 1; i <= 4; i++ {
		if _, err := a.Write([]byte(linha40(i))); err != nil {
			t.Fatalf("write %d devolveu erro: %v", i, err)
		}
	}
	// Linhas 3 e 4 passam do limite: duas tentativas de girar.
	if tentativas < 2 {
		t.Errorf("tentativas de girar = %d, quer ao menos 2 (uma por escrita além do limite)", tentativas)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	conteudo := lerArquivo(t, filepath.Join(dir, "x.log"))
	for i := 1; i <= 4; i++ {
		if !strings.Contains(conteudo, fmt.Sprintf("linha %02d", i)) {
			t.Errorf("x.log não tem a linha %d", i)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "x.1.log")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("x.1.log não deveria existir: %v", err)
	}
}

func TestApagarAntigos(t *testing.T) {
	dir := t.TempDir()
	agora := time.Now()
	criar := func(nome string, idade time.Duration) string {
		c := filepath.Join(dir, nome)
		if err := os.WriteFile(c, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		m := agora.Add(-idade)
		if err := os.Chtimes(c, m, m); err != nil {
			t.Fatal(err)
		}
		return c
	}
	dia := 24 * time.Hour
	velho := criar("eprocia.1.log", 31*dia)
	recente := criar("eprocia.log", 29*dia)
	outro := criar("outro.txt", 40*dia)

	if err := ApagarAntigos(dir, 30*dia, agora); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(velho); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("eprocia.1.log de 31 dias deveria ter sido apagado: %v", err)
	}
	for _, c := range []string{recente, outro} {
		if _, err := os.Stat(c); err != nil {
			t.Errorf("%s deveria ficar: %v", filepath.Base(c), err)
		}
	}
}
