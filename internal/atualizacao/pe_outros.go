//go:build !windows

package atualizacao

import "errors"

// VersaoPE só existe no Windows; fora dele devolve erro.
func VersaoPE(caminho string) (Versao, error) {
	return Versao{}, errors.New("versão do executável só é lida no Windows")
}
