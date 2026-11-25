// Auxiliar dos testes de internal/atualizacao. O modo vem de
// EPROCIA_TESTE_AUX. Erro vai para a saída de erro com código 2 (sem o pacote
// log: TestRegistro_SemLogPrintfEmProducao varre internal/ inteiro).
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"eprocia/internal/atualizacao"
)

func main() {
	switch modo := os.Getenv("EPROCIA_TESTE_AUX"); modo {
	case "versao":
		exe, err := os.Executable()
		if err != nil {
			falhar(err)
		}
		v, err := atualizacao.VersaoPE(exe)
		if err != nil {
			falhar(err)
		}
		fmt.Println(v)
	case "dorme":
		// Dorme EPROCIA_TESTE_MS milissegundos e sai 0.
		ms, err := strconv.Atoi(os.Getenv("EPROCIA_TESTE_MS"))
		if err != nil || ms < 0 {
			falhar(fmt.Errorf("EPROCIA_TESTE_MS inválido: %q", os.Getenv("EPROCIA_TESTE_MS")))
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
	default:
		falhar(fmt.Errorf("modo desconhecido: %q", modo))
	}
}

func falhar(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
