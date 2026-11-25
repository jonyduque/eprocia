//go:build windows

package telemetria

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// osVersionInfoW é a OSVERSIONINFOW do Windows.
type osVersionInfoW struct {
	tamanho    uint32
	maior      uint32
	menor      uint32
	build      uint32
	plataforma uint32
	csdVersion [128]uint16
}

// memoryStatusEx é a MEMORYSTATUSEX do Windows; dwLength precisa ser preenchido.
type memoryStatusEx struct {
	tamanho               uint32
	cargaMemoria          uint32
	totalFisica           uint64
	livreFisica           uint64
	totalPaginacao        uint64
	livrePaginacao        uint64
	totalVirtual          uint64
	livreVirtual          uint64
	livreVirtualEstendida uint64
}

var (
	ntdll                    = syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion        = ntdll.NewProc("RtlGetVersion")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// ColetarMaquina lê CPUs, memória total, versão e build do Windows
// (RtlGetVersion, que não mente a versão como GetVersionEx) e arquitetura.
// Uma chamada que falha deixa o campo zerado.
func ColetarMaquina() Maquina {
	m := Maquina{CPUs: runtime.NumCPU(), Arquitetura: runtime.GOARCH}

	var v osVersionInfoW
	v.tamanho = uint32(unsafe.Sizeof(v))
	if procRtlGetVersion.Find() == nil {
		if st, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&v))); st == 0 {
			m.Windows = fmt.Sprintf("%d.%d", v.maior, v.menor)
			m.Build = v.build
		}
	}

	var ms memoryStatusEx
	ms.tamanho = uint32(unsafe.Sizeof(ms))
	if procGlobalMemoryStatusEx.Find() == nil {
		if ok, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); ok != 0 {
			m.MemoriaTotal = ms.totalFisica
		}
	}
	return m
}
