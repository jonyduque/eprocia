package main

import "golang.org/x/sys/windows/registry"

// windowsUsaTemaClaro lê a mesma preferência que o WebView2 expõe como
// prefers-color-scheme. Sem a chave (Windows antigo), o padrão é claro.
func windowsUsaTemaClaro() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return true
	}
	return v != 0
}
