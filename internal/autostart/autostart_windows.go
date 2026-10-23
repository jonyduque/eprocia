//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	chaveRun  = `Software\Microsoft\Windows\CurrentVersion\Run`
	nomeValor = "eprocIA"
)

// Aplicar grava (ativo) ou apaga a entrada em HKCU\...\Run apontando para o
// executável atual.
func Aplicar(ativo bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("não foi possível localizar o executável: %w", err)
	}
	return aplicarEm(chaveRun, nomeValor, exe, ativo)
}

func aplicarEm(chave, nome, exe string, ativo bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, chave, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("não foi possível abrir a chave de inicialização: %w", err)
	}
	defer k.Close()

	if ativo {
		if err := k.SetStringValue(nome, Comando(exe)); err != nil {
			return fmt.Errorf("não foi possível ativar a inicialização com o Windows: %w", err)
		}
		return nil
	}
	if err := k.DeleteValue(nome); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("não foi possível desativar a inicialização com o Windows: %w", err)
	}
	return nil
}
