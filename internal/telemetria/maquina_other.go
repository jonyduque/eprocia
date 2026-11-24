//go:build !windows

package telemetria

import "runtime"

// ColetarMaquina fora do Windows informa só CPUs e arquitetura.
func ColetarMaquina() Maquina {
	return Maquina{CPUs: runtime.NumCPU(), Arquitetura: runtime.GOARCH}
}
