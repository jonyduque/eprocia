package registro

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// linha100 devolve uma linha de exatamente 100 bytes, com o escritor e o
// número.
func linha100(escritor, n int) string {
	s := fmt.Sprintf("escritor %d linha %04d ", escritor, n)
	return s + strings.Repeat("x", 99-len(s)) + "\n"
}

// Na atualização, o processo antigo e o novo escrevem no mesmo eprocia.log
// por alguns segundos: sem O_APPEND, um sobrescreve as linhas do outro.
func TestArquivoRotativo_DoisEscritoresNoMesmoArquivo(t *testing.T) {
	dir := t.TempDir()
	const porEscritor = 1000
	var abertos [2]*ArquivoRotativo
	for i := range abertos {
		a, err := AbrirRotativo(dir, "eprocia", ".log", 1<<30, 5)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Close() })
		abertos[i] = a
	}

	// Os dois escritores se revezam linha a linha: cada um espera a vez do
	// outro, o que força a intercalação no arquivo.
	vez := [2]chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1)}
	vez[0] <- struct{}{}
	var wg sync.WaitGroup
	erros := make(chan error, 2)
	for e := range 2 {
		wg.Go(func() {
			for n := range porEscritor {
				<-vez[e]
				_, err := abertos[e].Write([]byte(linha100(e, n)))
				vez[1-e] <- struct{}{}
				if err != nil {
					erros <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(erros)
	for err := range erros {
		t.Fatal(err)
	}

	conteudo := lerArquivo(t, filepath.Join(dir, "eprocia.log"))
	linhas := strings.Split(strings.TrimSuffix(conteudo, "\n"), "\n")
	if len(linhas) != 2*porEscritor {
		t.Fatalf("o arquivo tem %d linhas; quer %d", len(linhas), 2*porEscritor)
	}
	vistas := map[string]bool{}
	for i, l := range linhas {
		if len(l) != 99 || !strings.HasPrefix(l, "escritor ") || !strings.HasSuffix(l, "x") {
			t.Fatalf("linha %d partida: %q", i, l)
		}
		vistas[l] = true
	}
	for e := range 2 {
		for n := range porEscritor {
			if l := strings.TrimSuffix(linha100(e, n), "\n"); !vistas[l] {
				t.Fatalf("faltou a linha %q", l)
			}
		}
	}
}
