// Comando anonimizar-log prepara um log do eprocIA para ser partilhado:
// troca os nomes de ZIP e de documento por rótulos estáveis (zip-N, doc-N)
// e confere se sobrou algum caminho com o nome do usuário do Windows.
//
//	go run ./scripts/anonimizar-log <caminho do eprocia.log>
//
// Escreve <nome>.anon.log ao lado do arquivo, lista as linhas com caminho de
// usuário que sobrou e sai com 1 se houver alguma.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: anonimizar-log <arquivo de log>")
		os.Exit(2)
	}
	sobras, saida, err := executar(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(2)
	}
	fmt.Println("gravado:", saida)
	if len(sobras) > 0 {
		fmt.Printf("%d linha(s) ainda com caminho de usuário: %v\n", len(sobras), sobras)
		fmt.Println("confira essas linhas antes de partilhar o arquivo.")
		os.Exit(1)
	}
}

// executar anonimiza o arquivo de entrada para caminhoSaida(entrada).
func executar(entrada string) (sobras []int, saida string, err error) {
	saida = caminhoSaida(entrada)
	if strings.EqualFold(filepath.Clean(saida), filepath.Clean(entrada)) {
		return nil, "", fmt.Errorf("a entrada já é um arquivo anonimizado: %s", entrada)
	}
	in, err := os.Open(entrada)
	if err != nil {
		return nil, "", err
	}
	defer in.Close()
	out, err := os.Create(saida)
	if err != nil {
		return nil, "", err
	}
	sobras, err = anonimizar(in, out)
	if errFechar := out.Close(); err == nil {
		err = errFechar
	}
	if err != nil {
		os.Remove(saida)
		return nil, "", err
	}
	return sobras, saida, nil
}

// caminhoSaida devolve <nome sem extensão>.anon.log na pasta da entrada.
func caminhoSaida(entrada string) string {
	base := filepath.Base(entrada)
	nome := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(entrada), nome+".anon.log")
}
