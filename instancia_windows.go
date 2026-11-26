package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// travarInstancia cria o mutex nomeado. ok=false quando outro processo (ou
// outra chamada) já o tem; liberar fecha o handle e só vale com ok=true.
func travarInstancia(nome string) (liberar func(), ok bool, err error) {
	p, err := windows.UTF16PtrFromString(nome)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, p)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func() { windows.CloseHandle(h) }, true, nil
}

// avisarInstanciaAberta mostra o alerta nativo; a janela do app ainda não
// existe nesse ponto.
func avisarInstanciaAberta() {
	titulo, _ := windows.UTF16PtrFromString("eprocIA")
	texto, _ := windows.UTF16PtrFromString(
		"O eprocIA já está aberto a partir desta pasta.\n\n" +
			"Procure o ícone dele na bandeja, perto do relógio.")
	windows.MessageBox(0, texto, titulo, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
}
