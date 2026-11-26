//go:build !windows

package atualizacao

import (
	"errors"
	"fmt"
	"syscall"
)

var errSemShell = errors.New("o instalador só é aberto no Windows")

// ReabrirConferido só existe no Windows; fora dele devolve erro.
func ReabrirConferido(b Baixado) (*Travado, error) {
	return nil, fmt.Errorf("%w: %v", ErrNaoConferiu, errSemShell)
}

// ShellExecuteEx só existe no Windows; fora dele devolve erro.
func ShellExecuteEx(caminho string) error { return errSemShell }

func erroShell(errno syscall.Errno) error {
	return fmt.Errorf("abrir o instalador: %w", errno)
}
