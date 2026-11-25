//go:build windows

package atualizacao

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// O x/sys v0.49.0 não expõe as funções de atualização de recurso.
var (
	kernel32Recurso         = windows.NewLazySystemDLL("kernel32.dll")
	procBeginUpdateResource = kernel32Recurso.NewProc("BeginUpdateResourceW")
	procUpdateResource      = kernel32Recurso.NewProc("UpdateResourceW")
	procEndUpdateResource   = kernel32Recurso.NewProc("EndUpdateResourceW")
)

const rtVersion = 16

// copiarAuxiliar copia o auxiliar.exe para destino e carimba a versão v.
func copiarAuxiliar(t *testing.T, destino string, v Versao) {
	t.Helper()
	copiarArquivo(t, auxiliar, destino)
	carimbarVersao(t, destino, v)
}

func copiarArquivo(t *testing.T, origem, destino string) {
	t.Helper()
	dados, err := os.ReadFile(origem)
	if err != nil {
		t.Fatalf("ler %s: %v", origem, err)
	}
	if err := os.WriteFile(destino, dados, 0o755); err != nil {
		t.Fatalf("gravar %s: %v", destino, err)
	}
}

// partesFixas monta FileVersionMS/LS; cada parte precisa caber em 16 bits.
func partesFixas(v Versao) (ms, ls uint32, err error) {
	for i, p := range v {
		if p < 0 || p > 0xffff {
			return 0, 0, fmt.Errorf("parte %d da versão (%d) fora de 0..65535", i, p)
		}
	}
	return uint32(v[0])<<16 | uint32(v[1]), uint32(v[2]) << 16, nil
}

// blocoVersao monta um VS_VERSIONINFO só com o VS_FIXEDFILEINFO (sem
// StringFileInfo nem VarFileInfo).
func blocoVersao(ms, ls uint32) []byte {
	fixo := windows.VS_FIXEDFILEINFO{
		Signature:        0xFEEF04BD,
		StrucVersion:     0x00010000,
		FileVersionMS:    ms,
		FileVersionLS:    ls,
		ProductVersionMS: ms,
		ProductVersionLS: ls,
		FileFlagsMask:    0x3F,
		FileOS:           0x00040004, // VOS_NT_WINDOWS32
		FileType:         1,          // VFT_APP
	}
	var chave bytes.Buffer
	for _, u := range utf16.Encode([]rune("VS_VERSION_INFO\x00")) {
		_ = binary.Write(&chave, binary.LittleEndian, u)
	}
	cabecalho := 6 + chave.Len()
	pad := (4 - cabecalho%4) % 4
	tamFixo := int(unsafe.Sizeof(fixo))
	total := cabecalho + pad + tamFixo

	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint16(total))
	_ = binary.Write(&b, binary.LittleEndian, uint16(tamFixo))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0)) // binário
	b.Write(chave.Bytes())
	b.Write(make([]byte, pad))
	_ = binary.Write(&b, binary.LittleEndian, fixo)
	return b.Bytes()
}

// carimbarVersao grava em exe um recurso RT_VERSION id 1, idioma 0.
func carimbarVersao(t *testing.T, exe string, v Versao) {
	t.Helper()
	ms, ls, err := partesFixas(v)
	if err != nil {
		t.Fatalf("carimbar %v: %v", v, err)
	}
	caminho, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		t.Fatalf("caminho %s: %v", exe, err)
	}
	h, _, errBegin := procBeginUpdateResource.Call(uintptr(unsafe.Pointer(caminho)), 0)
	if h == 0 {
		t.Fatalf("BeginUpdateResourceW: %v", errBegin)
	}
	dados := blocoVersao(ms, ls)
	r, _, errUpd := procUpdateResource.Call(h, rtVersion, 1, 0,
		uintptr(unsafe.Pointer(&dados[0])), uintptr(len(dados)))
	if r == 0 {
		procEndUpdateResource.Call(h, 1)
		t.Fatalf("UpdateResourceW: %v", errUpd)
	}
	if r, _, errEnd := procEndUpdateResource.Call(h, 0); r == 0 {
		t.Fatalf("EndUpdateResourceW: %v", errEnd)
	}
}
