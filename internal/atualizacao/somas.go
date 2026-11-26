package atualizacao

import (
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

// LimiteSomas é o maior SHA256SUMS.txt aceito.
const LimiteSomas = 64 << 10

// linhaSomaRe segue a regra do install.ps1 (`^\s*(64 hex)\s+\*?<nome>\s*$`):
// maiúsculas aceitas, asterisco opcional, espaços nas pontas.
var linhaSomaRe = regexp.MustCompile(`^\s*([0-9a-fA-F]{64})\s+\*?(.+?)\s*$`)

// linhaSoma acha no SHA256SUMS.txt o hash de nome. Diferente do install.ps1,
// que usa a primeira linha que casa, duas linhas do mesmo nome são recusadas.
func linhaSoma(somas []byte, nome string) ([32]byte, error) {
	var soma [32]byte
	achou := 0
	for linha := range strings.SplitSeq(string(somas), "\n") {
		m := linhaSomaRe.FindStringSubmatch(strings.TrimSuffix(linha, "\r"))
		if m == nil || m[2] != nome {
			continue
		}
		achou++
		if achou > 1 {
			return [32]byte{}, errors.New("SHA256SUMS.txt lista o asset mais de uma vez")
		}
		b, err := hex.DecodeString(m[1])
		if err != nil || len(b) != len(soma) {
			return [32]byte{}, errors.New("hash inválido no SHA256SUMS.txt")
		}
		copy(soma[:], b)
	}
	if achou == 0 {
		return [32]byte{}, errors.New("SHA256SUMS.txt não lista o asset")
	}
	return soma, nil
}
