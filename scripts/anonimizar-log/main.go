// Comando anonimizar-log prepara logs do eprocIA para serem partilhados:
// troca os nomes de ZIP e de documento por rótulos estáveis (zip-N, doc-N)
// e confere se sobrou algum caminho com o nome do usuário do Windows.
//
//	go run ./scripts/anonimizar-log <eprocia.log> [eprocia.1.log ...]
//
// Vários arquivos numa execução partilham a mesma tabela de rótulos (na
// ordem dos argumentos). Cada um grava <nome>.anon.log ao lado; o comando
// lista as linhas com caminho de usuário que sobrou e sai com 1 se houver.
//
// Atenção: um .anon.log gravado dentro da pasta de logs do eprocIA casa com
// eprocia*.log e é apagado pela limpeza de 30 dias. Copie-o para fora.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: anonimizar-log <arquivo de log> [outro ...]")
		os.Exit(2)
	}
	res, err := executar(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(2)
	}
	sobrou := false
	for _, r := range res {
		fmt.Println("gravado:", r.saida)
		if len(r.sobras) > 0 {
			sobrou = true
			fmt.Printf("  %d linha(s) ainda com caminho de usuário: %v\n", len(r.sobras), r.sobras)
		}
	}
	if sobrou {
		fmt.Println("confira essas linhas antes de partilhar os arquivos.")
		os.Exit(1)
	}
}

// resultado é o arquivo gravado para uma entrada e as linhas com sobra.
type resultado struct {
	saida  string
	sobras []int
}

// executar lê todas as entradas, monta uma só tabela de rótulos na ordem
// dos argumentos e grava caminhoSaida(entrada) para cada uma.
func executar(entradas []string) ([]resultado, error) {
	rot := novosRotulos()
	conteudos := make([][]string, len(entradas))
	for i, e := range entradas {
		f, err := os.Open(e)
		if err != nil {
			return nil, err
		}
		linhas, err := lerLinhas(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e, err)
		}
		conteudos[i] = linhas
		rot.coletarLinhas(linhas)
	}
	troca := rot.substituidor()
	res := make([]resultado, 0, len(entradas))
	for i, e := range entradas {
		saida := caminhoSaida(e)
		out, err := os.Create(saida)
		if err != nil {
			return res, err
		}
		sobras, err := escrever(conteudos[i], troca, out)
		if errFechar := out.Close(); err == nil {
			err = errFechar
		}
		if err != nil {
			os.Remove(saida)
			return res, err
		}
		res = append(res, resultado{saida: saida, sobras: sobras})
	}
	return res, nil
}

// caminhoSaida devolve <nome sem extensão>.anon.log na pasta da entrada.
func caminhoSaida(entrada string) string {
	base := filepath.Base(entrada)
	nome := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(entrada), nome+".anon.log")
}
