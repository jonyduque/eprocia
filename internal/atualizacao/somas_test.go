package atualizacao

import (
	"encoding/hex"
	"strings"
	"testing"
)

const (
	hexMinusculo = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	nomeExe      = "eprocia-0.5.0-windows-amd64.exe"
	nomeInst     = "eprocia-0.5.0-windows-amd64-instalador.exe"
)

func TestSomas_linhaSomaAceitas(t *testing.T) {
	maiusculo := strings.ToUpper(hexMinusculo)
	outro := strings.Repeat("f", 64)
	casos := []struct {
		nome, somas string
	}{
		{"minúsculas com asterisco", hexMinusculo + " *" + nomeExe + "\n"},
		{"maiúsculas sem asterisco", maiusculo + "  " + nomeExe + "\n"},
		{"CRLF", outro + " *" + nomeInst + "\r\n" + hexMinusculo + " *" + nomeExe + "\r\n"},
		{"espaços nas pontas", "  " + hexMinusculo + " *" + nomeExe + "  \n"},
		{"linha de outro nome ignorada", outro + " *" + nomeInst + "\n" + hexMinusculo + " *" + nomeExe},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			got, err := linhaSoma([]byte(cs.somas), nomeExe)
			if err != nil {
				t.Fatalf("erro = %v", err)
			}
			if got != hashDe(t, hexMinusculo) {
				t.Fatalf("soma = %x, quer %s", got, hexMinusculo)
			}
		})
	}
}

func TestSomas_linhaSomaRecusadas(t *testing.T) {
	casos := []struct {
		nome, somas string
	}{
		{"ausente", strings.Repeat("f", 64) + " *" + nomeInst + "\n"},
		{"vazio", ""},
		{"duas linhas do mesmo nome", hexMinusculo + " *" + nomeExe + "\n" + strings.Repeat("f", 64) + " *" + nomeExe + "\n"},
		{"duas linhas iguais", hexMinusculo + " *" + nomeExe + "\n" + hexMinusculo + " *" + nomeExe + "\n"},
		{"hash com 63 caracteres", hexMinusculo[:63] + " *" + nomeExe + "\n"},
		{"hash com 65 caracteres", hexMinusculo + "0 *" + nomeExe + "\n"},
		{"nome com prefixo", hexMinusculo + " *x" + nomeExe + "\n"},
		{"sem separador", hexMinusculo + nomeExe + "\n"},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			if got, err := linhaSoma([]byte(cs.somas), nomeExe); err == nil {
				t.Fatalf("aceitou %x", got)
			}
		})
	}
}

func hashDe(t *testing.T, h string) [32]byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 32 {
		t.Fatalf("hex inválido %q", h)
	}
	return [32]byte(b)
}
