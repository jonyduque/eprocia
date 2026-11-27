//go:build windows

package atualizacao

import (
	"errors"
	"math"
	"os"
	"syscall"
	"time"
	"unsafe"

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

// PaiDoProcesso devolve o pid do processo pai e o nome da imagem dele (sem
// pasta), por um instantâneo dos processos. Pai ausente do instantâneo: exe
// vazio e err nil.
//
// O pai pode vir antes ou depois do próprio processo no instantâneo, por isso
// a decisão só vem depois de uma passada inteira.
func PaiDoProcesso() (ppid int, exe string, err error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, "", err
	}
	defer windows.CloseHandle(snap)
	type entrada struct {
		pai  uint32
		nome string
	}
	processos := map[uint32]entrada{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		processos[e.ProcessID] = entrada{e.ParentProcessID, windows.UTF16ToString(e.ExeFile[:])}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return 0, "", err
	}
	eu, achou := processos[uint32(os.Getpid())]
	if !achou {
		return 0, "", errors.New("processo atual ausente do instantâneo")
	}
	return int(eu.pai), processos[eu.pai].nome, nil
}

// atributosFilho: o processo novo da troca nasce sem console e sem herdar
// handles, fora do grupo do antigo, para sobreviver a ele.
func atributosFilho() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags:    windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		NoInheritHandles: true,
	}
}
