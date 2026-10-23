// Package autostart registra o eprocIA para iniciar com o logon do usuário.
package autostart

import "slices"

// FlagTray faz o app abrir só no tray, sem mostrar a janela.
const FlagTray = "--tray"

// Comando monta a linha gravada na entrada de inicialização. O caminho vai
// entre aspas porque costuma ter espaços (Program Files).
func Comando(exe string) string {
	return `"` + exe + `" ` + FlagTray
}

// IniciarOculto informa se o processo foi aberto pela entrada de inicialização.
func IniciarOculto(args []string) bool {
	return slices.Contains(args, FlagTray)
}
