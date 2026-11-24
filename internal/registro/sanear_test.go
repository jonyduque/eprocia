package registro

import (
	"strings"
	"testing"
)

const (
	perfilTeste = `C:\Users\fulano`
	tempTeste   = `C:\Users\fulano\AppData\Local\Temp`
)

func TestSanear_PerfilTempEUsuario(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	casos := map[string]string{
		`C:\Users\fulano\AppData\Local\Temp\eprocia_123\doc.pdf`:  `%TEMP%\eprocia_123\doc.pdf`,
		`C:\Users\fulano\Downloads\5000000-00.2026.8.26.0001.zip`: `%USERPROFILE%\Downloads\5000000-00.2026.8.26.0001.zip`,
		`D:\Users\beltrano\x.zip`:                                 `D:\Users\<usuario>\x.zip`,
	}
	for entrada, quero := range casos {
		if got := s.Sanear(entrada); got != quero {
			t.Errorf("Sanear(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

func TestSanear_GrafiasDoPerfil(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	semUsuario := []string{
		`c:\users\FULANO\x`,
		`C:/Users/fulano/x`,
		`C:\\Users\\fulano\\x`,
		`C:\Users\FULANO~1\AppData\Local\Temp\y`,
		`abrir "C:\Users\fulano" falhou`,
		`erro em C:\Users\fulano`,
	}
	for _, entrada := range semUsuario {
		got := s.Sanear(entrada)
		if strings.Contains(strings.ToLower(got), "fulano") {
			t.Errorf("Sanear(%q) = %q, ainda contém o usuário", entrada, got)
		}
	}
	iguais := map[string]string{
		`C:\Users\fulano2\x`:   `C:\Users\<usuario>\x`,
		`C:\Users\5000000.zip`: `C:\Users\5000000.zip`,
		`Users-fulano.zip`:     `Users-fulano.zip`,
		`a fulano disse`:       `a fulano disse`,
	}
	for entrada, quero := range iguais {
		if got := s.Sanear(entrada); got != quero {
			t.Errorf("Sanear(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

func TestSanear_TextoSemCaminho(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	texto := "processo 5000000-00.2026.8.26.0001 de FULANO DE TAL: 12 páginas, OCR em 3"
	if got := s.Sanear(texto); got != texto {
		t.Errorf("Sanear(%q) = %q, quero igual", texto, got)
	}
}

func TestSanear_SemUsersNaoAloca(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	texto := "documento 5000000-00.2026.8.26.0001 lido em 2 s"
	if n := testing.AllocsPerRun(100, func() { _ = s.Sanear(texto) }); n != 0 {
		t.Errorf("Sanear sem 'users' alocou %v vezes", n)
	}
}

func TestSanear_TempForaDoPerfil(t *testing.T) {
	s := NovoSaneador(perfilTeste, `D:\Temp`)
	if got := s.Sanear(`falha em D:\temp\eprocia_1\a.pdf`); got != `falha em %TEMP%\eprocia_1\a.pdf` {
		t.Errorf("got %q", got)
	}
}
