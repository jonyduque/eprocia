package atualizacao

import (
	"slices"
	"strconv"
	"strings"
)

// AposAtualizar devolve o PID do processo antigo quando o exe foi aberto pela
// troca, com --apos-atualizar=<inteiro positivo>. Qualquer outra forma
// (vazio, sinal, espaço, caminho, valor em argumento separado) é recusada.
func AposAtualizar(args []string) (pid int, ok bool) {
	for _, a := range args {
		valor, achou := strings.CutPrefix(a, FlagAposAtualizar)
		if !achou {
			continue
		}
		if valor == "" || strings.Trim(valor, "0123456789") != "" {
			return 0, false
		}
		n, err := strconv.Atoi(valor)
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// LimparAnteriorPedido diz se o exe foi aberto pelo Reiniciar, que pede ao
// processo novo para apagar o eprocia.anterior.exe depois de o antigo sair.
// Só aceita o argumento exato, sem valor e na mesma caixa.
func LimparAnteriorPedido(args []string) bool {
	return slices.Contains(args, FlagLimparAnterior)
}
