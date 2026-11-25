//go:build windows

package atualizacao

import (
	"errors"
	"math"
	"time"

	"golang.org/x/sys/windows"
)

// EsperarSair espera o processo pid terminar, até o prazo. Processo que não
// existe mais conta como saído na hora. Se o processo existe mas não pode ser
// aberto para espera (outro usuário, por exemplo), devolve falso: não dá para
// confirmar a saída.
func EsperarSair(pid int, prazo time.Duration) bool {
	if pid <= 0 || pid > math.MaxUint32 {
		return true
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER: não há processo com esse PID.
		return errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer windows.CloseHandle(h)
	ms := min(max(prazo.Milliseconds(), 0), windows.INFINITE-1)
	ev, err := windows.WaitForSingleObject(h, uint32(ms))
	return err == nil && ev == windows.WAIT_OBJECT_0
}
