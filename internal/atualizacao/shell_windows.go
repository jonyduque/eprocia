//go:build windows

package atualizacao

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// O x/sys v0.49.0 não tem ShellExecuteExW nem SHELLEXECUTEINFOW.
var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW = shell32.NewProc("ShellExecuteExW")
)

const (
	seeMaskFlagNoUI = 0x00000400 // SEE_MASK_FLAG_NO_UI
	swShowNormal    = 1          // SW_SHOWNORMAL
)

// shellExecuteInfoW tem o layout de SHELLEXECUTEINFOW (112 bytes em 64 bits).
type shellExecuteInfoW struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

// ReabrirConferido reabre o instalador baixado só com compartilhamento de
// leitura (ninguém escreve, renomeia nem apaga enquanto o handle vive) e o
// confere de novo sobre esse handle. Com erro, o handle já está fechado.
func ReabrirConferido(b Baixado) (*Travado, error) {
	nome, err := windows.UTF16PtrFromString(b.Caminho)
	if err != nil {
		return nil, fmt.Errorf("%w: caminho inválido", ErrNaoConferiu)
	}
	h, err := windows.CreateFile(nome, windows.GENERIC_READ, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		// Só o errno: o caminho tem o nome do usuário.
		return nil, fmt.Errorf("%w: reabrir: %w", ErrNaoConferiu, err)
	}
	f := os.NewFile(uintptr(h), b.Caminho)
	if err := conferir(b, f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Travado{f: f, caminho: b.Caminho}, nil
}

// ShellExecuteEx abre caminho com o verbo "open" (o manifesto do instalador
// pede a elevação), sem caixa de erro do Windows. UAC recusado devolve
// ErrCancelado. Nunca roda em teste.
func ShellExecuteEx(caminho string) error {
	verbo, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	arquivo, err := windows.UTF16PtrFromString(caminho)
	if err != nil {
		return fmt.Errorf("abrir o instalador: caminho inválido")
	}
	info := shellExecuteInfoW{
		fMask:  seeMaskFlagNoUI,
		lpVerb: verbo,
		lpFile: arquivo,
		nShow:  swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, errCall := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return nil
	}
	errno, ok := errCall.(syscall.Errno)
	if !ok {
		return fmt.Errorf("abrir o instalador: %T", errCall)
	}
	return erroShell(errno)
}

// erroShell traduz o errno do ShellExecuteExW: ERROR_CANCELLED (UAC
// recusado) vira ErrCancelado; os outros seguem embrulhados, sem caminho.
func erroShell(errno syscall.Errno) error {
	if errno == windows.ERROR_CANCELLED {
		return ErrCancelado
	}
	return fmt.Errorf("abrir o instalador: %w", errno)
}
