//go:build windows

package atualizacao

import "golang.org/x/sys/windows/registry"

const (
	// chaveUsuario é a chave que o install.ps1 grava em HKCU.
	chaveUsuario = `Software\Microsoft\Windows\CurrentVersion\Uninstall\eprocIA`
	// chaveNSIS é a UNINST_KEY do wails_tools.nsh (empresa + produto), em HKLM, visão 64 bits.
	chaveNSIS = `Software\Microsoft\Windows\CurrentVersion\Uninstall\eprociaeprocia`
)

type registroWindows struct{}

// RegistroWindows lê o registro real, só com QUERY_VALUE.
func RegistroWindows() Registro { return registroWindows{} }

func (registroWindows) InstallLocationUsuario() (string, error) {
	return lerValor(registry.CURRENT_USER, chaveUsuario, "InstallLocation", 0)
}

func (registroWindows) DisplayIconMaquina() (string, error) {
	return lerValor(registry.LOCAL_MACHINE, chaveNSIS, "DisplayIcon", registry.WOW64_64KEY)
}

func lerValor(raiz registry.Key, chave, nome string, visao uint32) (string, error) {
	k, err := registry.OpenKey(raiz, chave, registry.QUERY_VALUE|visao)
	if err != nil {
		return "", err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(nome)
	return v, err
}
