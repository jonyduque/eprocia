//go:build windows

package atualizacao

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
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

// pastaInstalada cria <tmp>/Programas Instalados/eprocIA/eprocia.exe (vazio) e
// devolve a pasta e o exe, já resolvidos como o chamador os passa a Detectar.
func pastaInstalada(t *testing.T) (pasta, exe string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pasta = filepath.Join(base, "Programas Instalados", "eprocIA")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(pasta, "eprocia.exe")
	if err := os.WriteFile(exe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return pasta, exe
}

// criarJuncao cria a junção j -> alvo com o mklink do cmd (não exige
// privilégio) e a remove no fim do teste.
func criarJuncao(t *testing.T, j, alvo string) {
	t.Helper()
	c := exec.Command("cmd")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /c mklink /J "` + j + `" "` + alvo + `"`}
	if out, err := c.CombinedOutput(); err != nil {
		t.Skipf("mklink /J indisponível: %v (%s)", err, out)
	}
	t.Cleanup(func() { os.Remove(j) })
}

func TestDetectar_InstallLocationPorJuncao(t *testing.T) {
	pasta, exe := pastaInstalada(t)
	j := filepath.Join(filepath.Dir(filepath.Dir(pasta)), "atalho")
	criarJuncao(t, j, pasta)

	got := Detectar(exe, &registroFalso{local: j + `\`, semMaq: true}, sempreEscreve)
	if got.Modo != ModoTrocar || got.Pasta != pasta {
		t.Fatalf("InstallLocation por junção: Detectar = %+v, esperado trocar em %q", got, pasta)
	}
	got = Detectar(exe, &registroFalso{semUsr: true, icone: `"` + filepath.Join(j, "eprocia.exe") + `"`}, nuncaEscreve)
	if got.Modo != ModoInstalador {
		t.Fatalf("DisplayIcon por junção: Detectar = %+v, esperado instalador", got)
	}
}

func TestDetectar_InstallLocationNomeCurto(t *testing.T) {
	pasta, exe := pastaInstalada(t)
	p, err := syscall.UTF16PtrFromString(pasta)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	curto := syscall.UTF16ToString(buf[:n])
	if err != nil || strings.EqualFold(curto, pasta) {
		t.Skipf("volume sem nomes 8.3 (%v)", err)
	}
	got := Detectar(exe, &registroFalso{local: curto, semMaq: true}, sempreEscreve)
	if got.Modo != ModoTrocar || got.Pasta != pasta {
		t.Fatalf("InstallLocation %q: Detectar = %+v, esperado trocar em %q", curto, got, pasta)
	}
}

func TestDetectar_InstallLocationRelativo(t *testing.T) {
	pasta, exe := pastaInstalada(t)
	t.Chdir(filepath.Dir(filepath.Dir(pasta)))
	rel := filepath.Join("Programas Instalados", ".", "eprocIA")
	got := Detectar(exe, &registroFalso{local: rel, semMaq: true}, sempreEscreve)
	if got.Modo != ModoTrocar || got.Pasta != pasta {
		t.Fatalf("InstallLocation %q: Detectar = %+v, esperado trocar em %q", rel, got, pasta)
	}
}

// lerFonte lê um arquivo da raiz do repositório como texto.
func lerFonte(t *testing.T, partes ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, partes...)...))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestChavesConferemComAsFontes(t *testing.T) {
	nsh := lerFonte(t, "build", "windows", "installer", "wails_tools.nsh")
	definido := func(nome string) string {
		m := regexp.MustCompile(`!define ` + nome + ` "([^"]*)"`).FindStringSubmatch(nsh)
		if m == nil {
			t.Fatalf("wails_tools.nsh sem !define %s", nome)
		}
		return m[1]
	}
	if got := definido("UNINST_KEY_NAME"); got != "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}" {
		t.Fatalf("UNINST_KEY_NAME = %q", got)
	}
	nomeNSIS := definido("INFO_COMPANYNAME") + definido("INFO_PRODUCTNAME")
	if got := definido("UNINST_KEY"); strings.ReplaceAll(got, "${UNINST_KEY_NAME}", nomeNSIS) != chaveNSIS {
		t.Fatalf("UNINST_KEY %q com %q difere de chaveNSIS %q", got, nomeNSIS, chaveNSIS)
	}
	if !strings.Contains(nsh, "SetRegView 64\n") {
		t.Fatal("wails_tools.nsh não usa SetRegView 64")
	}
	if !strings.Contains(nsh, `WriteRegStr HKLM "${UNINST_KEY}" "`+valorDisplayIcon+`" "$INSTDIR\${PRODUCT_EXECUTABLE}"`) {
		t.Fatalf("wails_tools.nsh não grava %s com o caminho do exe", valorDisplayIcon)
	}

	ps := lerFonte(t, "install.ps1")
	if !strings.Contains(ps, `$uninstallKey = 'HKCU:\`+chaveUsuario+`'`) {
		t.Fatalf("install.ps1 não grava a chave HKCU %q", chaveUsuario)
	}
	if !regexp.MustCompile(`(?m)^\s+` + valorInstallLocation + `\s+= \$installDir$`).MatchString(ps) {
		t.Fatalf("install.ps1 não grava %s = $installDir", valorInstallLocation)
	}
}
