//go:build !windows

package atualizacao

import "errors"

var errSemRegistro = errors.New("registro só existe no Windows")

type registroAusente struct{}

// RegistroWindows fora do Windows não acha nenhuma chave.
func RegistroWindows() Registro { return registroAusente{} }

func (registroAusente) InstallLocationUsuario() (string, error) { return "", errSemRegistro }
func (registroAusente) DisplayIconMaquina() (string, error)     { return "", errSemRegistro }
