package atualizacao

import (
	"path/filepath"
	"strconv"
	"strings"
)

// PrimeiraVersaoComProtocolo é a primeira release que conhece a flag
// --apos-atualizar e a trava adiada. Se essa release sair com outro número,
// a constante muda antes dela (a skill eprocia-release diz isso). É var só
// para o teste; nunca muda em produção.
var PrimeiraVersaoComProtocolo = Versao{0, 5, 0}

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

// VersaoAnterior lê a versão do eprocia.anterior.exe da pasta de instalação.
// Só é verdadeiro se ela for menor que a atual e já conhecer o protocolo:
// uma versão sem ele (como a 0.4.4) não entende --apos-atualizar, sai na hora
// por achar a trava ocupada, e o "Voltar" sempre reverteria.
func VersaoAnterior(pastaInstalacao string, atual Versao) (Versao, bool) {
	v, err := VersaoPE(filepath.Join(pastaInstalacao, NomeAnterior))
	if err != nil || !v.Valida() {
		return Versao{}, false
	}
	if v.Comparar(atual) >= 0 || v.Comparar(PrimeiraVersaoComProtocolo) < 0 {
		return Versao{}, false
	}
	return v, true
}
