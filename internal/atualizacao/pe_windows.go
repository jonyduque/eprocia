//go:build windows

package atualizacao

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const assinaturaVersao = 0xFEEF04BD

// VersaoPE lê major.minor.build do VS_FIXEDFILEINFO do executável
// (FileVersionMS/LS); a quarta parte é ignorada.
func VersaoPE(caminho string) (Versao, error) {
	tam, err := windows.GetFileVersionInfoSize(caminho, nil)
	if err != nil {
		return Versao{}, fmt.Errorf("executável sem recurso de versão: %w", err)
	}
	buf := make([]byte, tam)
	if err := windows.GetFileVersionInfo(caminho, 0, tam, unsafe.Pointer(&buf[0])); err != nil {
		return Versao{}, fmt.Errorf("ler recurso de versão: %w", err)
	}
	var fixo *windows.VS_FIXEDFILEINFO
	var tamFixo uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&fixo), &tamFixo); err != nil {
		return Versao{}, fmt.Errorf("recurso de versão sem VS_FIXEDFILEINFO: %w", err)
	}
	if fixo == nil || uintptr(tamFixo) < unsafe.Sizeof(*fixo) || fixo.Signature != assinaturaVersao {
		return Versao{}, errors.New("VS_FIXEDFILEINFO inválido")
	}
	return Versao{
		int(fixo.FileVersionMS >> 16),
		int(fixo.FileVersionMS & 0xffff),
		int(fixo.FileVersionLS >> 16),
	}, nil
}
