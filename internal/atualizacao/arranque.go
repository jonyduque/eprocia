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

// Motivos fixos de recusa do --apos-atualizar (vão só ao log).
const (
	MotivoPidNaoEPai    = "o pid informado não é o processo pai"
	MotivoPaiNaoEprocia = "o processo pai não é o eprocIA"
)

// ConfirmarAposAtualizar decide se o --apos-atualizar=pid veio mesmo da troca:
// o pid tem de ser o do processo pai, e o pai, um eprocIA (eprocia.exe ou,
// já renomeado, eprocia.anterior.exe). Sem conseguir ler o pai (errPai) ou
// com o pai fora do instantâneo, confia na flag: na dúvida, abrir como antes.
func ConfirmarAposAtualizar(pid, ppid int, exe string, errPai error) (ok bool, motivo string) {
	switch {
	case pid <= 0:
		return false, MotivoPidNaoEPai
	case errPai != nil:
		return true, ""
	case ppid != pid:
		return false, MotivoPidNaoEPai
	case exe == "", strings.EqualFold(exe, NomeExe), strings.EqualFold(exe, NomeAnterior):
		return true, ""
	}
	return false, MotivoPaiNaoEprocia
}
