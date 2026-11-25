//go:build windows

package atualizacao

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// registroFalso substitui o registro do Windows; semUsr/semMaq simulam a
// chave ausente.
type registroFalso struct {
	local  string
	icone  string
	semUsr bool
	semMaq bool
}

var errAusente = errors.New("chave ausente")

func (r *registroFalso) InstallLocationUsuario() (string, error) {
	if r.semUsr {
		return "", errAusente
	}
	return r.local, nil
}

func (r *registroFalso) DisplayIconMaquina() (string, error) {
	if r.semMaq {
		return "", errAusente
	}
	return r.icone, nil
}

func sempreEscreve(string) bool { return true }
func nuncaEscreve(string) bool  { return false }

func TestDetectar_PorUsuario(t *testing.T) {
	pasta := `C:\Users\fulano\AppData\Local\Programs\eprocIA`
	reg := &registroFalso{local: pasta, semMaq: true}
	got := Detectar(pasta+`\eprocia.exe`, reg, sempreEscreve)
	if got.Modo != ModoTrocar || got.Pasta != pasta || got.Motivo != "" {
		t.Fatalf("Detectar = %+v, esperado trocar em %q", got, pasta)
	}
}

func TestDetectar_CaminhoComAcentoECaixa(t *testing.T) {
	casos := []struct{ local, exe string }{
		{`C:\Users\José\AppData\Local\Programs\eprocIA\`, `c:\users\josé\appdata\local\programs\EPROCIA\eprocia.exe`},
		{`D:\Área de Trabalho\eprocIA`, `d:\área de trabalho\EPROCIA\eprocia.exe`},
	}
	for _, c := range casos {
		reg := &registroFalso{local: c.local, semMaq: true}
		got := Detectar(c.exe, reg, sempreEscreve)
		if got.Modo != ModoTrocar {
			t.Errorf("Detectar(%q) com InstallLocation %q = %+v, esperado trocar", c.exe, c.local, got)
		}
		if got.Pasta != filepath.Dir(c.exe) {
			t.Errorf("Pasta = %q, esperado %q", got.Pasta, filepath.Dir(c.exe))
		}
	}
}

func TestDetectar_PorUsuarioSemEscrita(t *testing.T) {
	pasta := `C:\Users\fulano\AppData\Local\Programs\eprocIA`
	var pedida string
	reg := &registroFalso{local: pasta, semMaq: true}
	got := Detectar(pasta+`\eprocia.exe`, reg, func(p string) bool { pedida = p; return false })
	if got.Modo != ModoSoAvisar || got.Motivo != MotivoSemEscrita {
		t.Fatalf("Detectar = %+v, esperado so_avisar com %q", got, MotivoSemEscrita)
	}
	if pedida != pasta {
		t.Fatalf("podeEscrever recebeu %q, esperado %q", pedida, pasta)
	}
}

func TestDetectar_NSIS(t *testing.T) {
	casos := []struct{ icone, exe string }{
		{`C:\Program Files\eprocia\eprocia.exe`, `C:\Program Files\eprocia\eprocia.exe`},
		{`"C:\Program Files\eprocia\eprocia.exe"`, `C:\Program Files\eprocia\eprocia.exe`},
		{`"C:\Program Files\eprocia\eprocia.exe",0`, `c:\program files\EPROCIA\eprocia.exe`},
		{`C:\Program Files\eprocia\eprocia.exe,0`, `C:\Program Files\eprocia\eprocia.exe`},
		{`D:\Apps\eprocia\eprocia.exe`, `D:\Apps\eprocia\eprocia.exe`},
	}
	for _, c := range casos {
		reg := &registroFalso{semUsr: true, icone: c.icone}
		got := Detectar(c.exe, reg, nuncaEscreve)
		if got.Modo != ModoInstalador || got.Motivo != "" {
			t.Errorf("Detectar(%q) com DisplayIcon %q = %+v, esperado instalador", c.exe, c.icone, got)
		}
		if got.Pasta != filepath.Dir(c.exe) {
			t.Errorf("Pasta = %q, esperado %q", got.Pasta, filepath.Dir(c.exe))
		}
	}
}

func TestDetectar_SoAvisar(t *testing.T) {
	casos := []struct {
		nome string
		exe  string
		reg  *registroFalso
	}{
		{"downloads", `C:\Users\fulano\Downloads\eprocia.exe`, &registroFalso{semUsr: true, semMaq: true}},
		{"build", `C:\Users\fulano\Projetos\eprocia\build\bin\eprocia.exe`, &registroFalso{semUsr: true, semMaq: true}},
		{"hkcu em outra pasta", `C:\Users\fulano\Downloads\eprocia.exe`,
			&registroFalso{local: `C:\Users\fulano\AppData\Local\Programs\eprocIA`, semMaq: true}},
		{"hklm em outra pasta", `C:\Users\fulano\Downloads\eprocia.exe`,
			&registroFalso{semUsr: true, icone: `C:\Program Files\eprocia\eprocia.exe`}},
		{"hkcu vazio", `C:\Users\fulano\Downloads\eprocia.exe`, &registroFalso{local: "", semMaq: true}},
	}
	for _, c := range casos {
		got := Detectar(c.exe, c.reg, sempreEscreve)
		if got.Modo != ModoSoAvisar || got.Motivo != MotivoForaDaPasta {
			t.Errorf("%s: Detectar = %+v, esperado so_avisar com %q", c.nome, got, MotivoForaDaPasta)
		}
	}
}

func TestPodeEscrever(t *testing.T) {
	dir := t.TempDir()
	if !PodeEscrever(dir) {
		t.Fatal("PodeEscrever(TempDir) = false")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("diretório ficou com %d entradas", len(ents))
	}
	if PodeEscrever(filepath.Join(dir, "nao-existe")) {
		t.Fatal("PodeEscrever(pasta inexistente) = true")
	}
}
