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

func TestSanear_PerfilForaDeUsers(t *testing.T) {
	s := NovoSaneador(`D:\Perfis\fulano`, `D:\Perfis\fulano\Temp`)
	casos := map[string]string{
		`abrir D:\Perfis\fulano\x.zip`:      `abrir %USERPROFILE%\x.zip`,
		`abrir D:\perfis\FULANO\Temp\y.pdf`: `abrir %TEMP%\y.pdf`,
	}
	for entrada, quero := range casos {
		if got := s.Sanear(entrada); got != quero {
			t.Errorf("Sanear(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

// O perfil seguido de pontuação (mensagens de erro do Windows, parênteses,
// fim de frase) também é trocado: o critério 6 da spec proíbe o nome no log.
func TestSanear_PerfilSeguidoDePontuacao(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	casos := map[string]string{
		`open C:\Users\fulano: Access is denied.`: `open %USERPROFILE%: Access is denied.`,
		`(C:\Users\fulano)`:                       `(%USERPROFILE%)`,
		`C:\Users\fulano.`:                        `%USERPROFILE%.`,
		`C:\Users\FULANO~1: x`:                    `C:\Users\<usuario>: x`,
		`D:\Users\beltrano: x`:                    `D:\Users\<usuario>: x`,
		`D:\Users\beltrano, x`:                    `D:\Users\<usuario>, x`,
		`(D:\Users\beltrano)`:                     `(D:\Users\<usuario>)`,
		`D:\Users\beltrano:`:                      `D:\Users\<usuario>:`,
	}
	for entrada, quero := range casos {
		if got := s.Sanear(entrada); got != quero {
			t.Errorf("Sanear(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}

// As grafias do perfil são trocadas pela regex do perfil (saída exata), não
// só pela regex genérica de <usuario>.
func TestSanear_GrafiasSaidaExata(t *testing.T) {
	s := NovoSaneador(perfilTeste, tempTeste)
	casos := map[string]string{
		`c:\users\FULANO\x`:    `%USERPROFILE%\x`,
		`C:/Users/fulano/x`:    `%USERPROFILE%/x`,
		`C:\\Users\\fulano\\x`: `%USERPROFILE%\\x`,
	}
	for entrada, quero := range casos {
		if got := s.Sanear(entrada); got != quero {
			t.Errorf("Sanear(%q) = %q, quero %q", entrada, got, quero)
		}
	}
}
