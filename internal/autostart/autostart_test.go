package autostart

import "testing"

func TestComando_AspasEFlagTray(t *testing.T) {
	got := Comando(`C:\Program Files\eprocIA\eprocia.exe`)
	want := `"C:\Program Files\eprocIA\eprocia.exe" --tray`
	if got != want {
		t.Fatalf("Comando = %q, esperado %q", got, want)
	}
}

func TestIniciarOculto(t *testing.T) {
	if !IniciarOculto([]string{"eprocia.exe", "--tray"}) {
		t.Fatal("--tray deveria iniciar oculto")
	}
	if IniciarOculto([]string{"eprocia.exe"}) {
		t.Fatal("sem --tray não deveria iniciar oculto")
	}
}
