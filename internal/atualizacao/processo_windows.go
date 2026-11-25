//go:build windows

package atualizacao

import (
	"errors"
	"math"
	"time"

	"golang.org/x/sys/windows"
)

// openProcess é trocado nos testes.
var openProcess = windows.OpenProcess

// stillActive é o código de saída de um processo que ainda roda (STILL_ACTIVE,
// que o x/sys não exporta).
const stillActive = 259

// intervaloSonda é o passo da sondagem quando o processo não abre com
// SYNCHRONIZE.
const intervaloSonda = 200 * time.Millisecond

// EsperarSair espera o processo pid terminar, até o prazo. Processo que não
// existe mais conta como saído na hora. Se ele não abre com SYNCHRONIZE (outro
// usuário, antigo elevado), sonda GetExitCodeProcess por um handle de
// PROCESS_QUERY_LIMITED_INFORMATION; se nem esse abre, espera o prazo inteiro
// e devolve falso: quem chama segue mesmo assim e registra no log.
//
// Risco aceito: se o antigo saiu e o PID foi reaproveitado por outro processo,
// a espera passa a valer para o novo dono do PID; no pior caso só atrasa até
// o prazo.
func EsperarSair(pid int, prazo time.Duration) bool {
	if pid <= 0 || pid > math.MaxUint32 {
		return true
	}
	prazo = max(prazo, 0)
	h, err := openProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == nil {
		defer windows.CloseHandle(h)
		ms := min(prazo.Milliseconds(), windows.INFINITE-1)
		ev, err := windows.WaitForSingleObject(h, uint32(ms))
		return err == nil && ev == windows.WAIT_OBJECT_0
	}
	if inexistente(err) {
		return true
	}
	fim := time.Now().Add(prazo)
	h, err = openProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if inexistente(err) {
			return true
		}
		time.Sleep(prazo)
		return false
	}
	defer windows.CloseHandle(h)
	for {
		var codigo uint32
		if err := windows.GetExitCodeProcess(h, &codigo); err == nil && codigo != stillActive {
			return true
		}
		resta := time.Until(fim)
		if resta <= 0 {
			return false
		}
		time.Sleep(min(intervaloSonda, resta))
	}
}

// inexistente diz se o erro do OpenProcess é o de PID sem processo.
func inexistente(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_PARAMETER)
}
