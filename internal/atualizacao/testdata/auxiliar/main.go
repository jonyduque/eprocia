// Auxiliar dos testes de internal/atualizacao. O modo vem de
// EPROCIA_TESTE_AUX. Erro vai para a saída de erro com código 2 (sem o pacote
// log: TestRegistro_SemLogPrintfEmProducao varre internal/ inteiro).
//
// Com EPROCIA_TESTE_DADOS definido, todo modo grava antes, nessa pasta,
// args-<pid>.txt (um argumento por linha) e versao-<pid>.txt (a versão do PE
// do próprio exe).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"eprocia/internal/atualizacao"
)

// espera é quanto os modos sinal e parado ficam no ar.
const espera = 30 * time.Second

func main() {
	registrar()
	switch modo := os.Getenv("EPROCIA_TESTE_AUX"); modo {
	case "versao":
		v, err := versaoPropria()
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
	case "sinal":
		// Faz o papel do processo novo: grava o sinal de vida para o PID de
		// --apos-atualizar e fica no ar até esse processo sair (no máximo
		// espera), como o eprocia.exe real.
		pid, ok := atualizacao.AposAtualizar(os.Args[1:])
		if !ok {
			falhar(fmt.Errorf("sem --apos-atualizar válido: %q", os.Args[1:]))
		}
		v, err := versaoPropria()
		if err != nil {
			falhar(err)
		}
		if err := atualizacao.GravarSinal(os.Getenv("EPROCIA_TESTE_DADOS"), pid, v); err != nil {
			falhar(err)
		}
		atualizacao.EsperarSair(pid, espera)
	case "sai1":
		os.Exit(1)
	case "parado":
		time.Sleep(espera)
	default:
		falhar(fmt.Errorf("modo desconhecido: %q", modo))
	}
}

func versaoPropria() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	v, err := atualizacao.VersaoPE(exe)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func registrar() {
	dados := os.Getenv("EPROCIA_TESTE_DADOS")
	if dados == "" {
		return
	}
	pid := strconv.Itoa(os.Getpid())
	v, err := versaoPropria()
	if err != nil {
		v = "?"
	}
	if err := os.WriteFile(filepath.Join(dados, "versao-"+pid+".txt"), []byte(v), 0o644); err != nil {
		falhar(err)
	}
	if err := os.WriteFile(filepath.Join(dados, "args-"+pid+".txt"), []byte(strings.Join(os.Args[1:], "\n")), 0o644); err != nil {
		falhar(err)
	}
}

func falhar(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
