//go:build !windows

package atualizacao

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// EsperarSair fora do Windows consulta o processo com o sinal 0 até ele
// sumir ou o prazo acabar. O aplicativo só roda no Windows; isto existe para
// o pacote compilar nas outras plataformas.
func EsperarSair(pid int, prazo time.Duration) bool {
	if pid <= 0 {
		return true
	}
	fim := time.Now().Add(prazo)
	for {
		p, err := os.FindProcess(pid)
		if err != nil {
			return true
		}
		if err := p.Signal(syscall.Signal(0)); err != nil && !errors.Is(err, syscall.EPERM) {
			return true
		}
		if !time.Now().Before(fim) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// atributosFilho fora do Windows: nada a ajustar.
func atributosFilho() *syscall.SysProcAttr { return nil }
