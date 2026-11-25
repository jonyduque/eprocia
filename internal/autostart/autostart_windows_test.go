//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestAplicarEm_GravaEApagaValor(t *testing.T) {
	chave := fmt.Sprintf(`Software\eprocIA_teste_%d`, time.Now().UnixNano())
	t.Cleanup(func() { registry.DeleteKey(registry.CURRENT_USER, chave) })
	exe := `C:\eprocIA\eprocia.exe`

	if err := aplicarEm(chave, "eprocIA", exe, true); err != nil {
		t.Fatalf("ativar: %v", err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := k.GetStringValue("eprocIA")
	k.Close()
	if err != nil || v != Comando(exe) {
		t.Fatalf("valor gravado = %q (%v), esperado %q", v, err, Comando(exe))
	}

	if err := aplicarEm(chave, "eprocIA", exe, false); err != nil {
		t.Fatalf("desativar: %v", err)
	}
	// Desativar de novo, sem o valor, não é erro.
	if err := aplicarEm(chave, "eprocIA", exe, false); err != nil {
		t.Fatalf("desativar sem valor: %v", err)
	}
	k, err = registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if _, _, err := k.GetStringValue("eprocIA"); !errors.Is(err, registry.ErrNotExist) {
		t.Fatalf("valor ainda presente depois de desativar: %v", err)
	}
}

func TestGravarVersaoDesinstalacao(t *testing.T) {
	chave := fmt.Sprintf(`Software\eprocIA_teste_desinst_%d`, time.Now().UnixNano())
	t.Cleanup(func() { registry.DeleteKey(registry.CURRENT_USER, chave) })

	// Chave ausente: nada a gravar, sem erro, e a chave não é criada.
	if err := gravarVersaoEm(chave, "0.5.0"); err != nil {
		t.Fatalf("chave ausente: %v", err)
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE); !errors.Is(err, registry.ErrNotExist) {
		if err == nil {
			k.Close()
		}
		t.Fatalf("chave ausente foi criada (err = %v)", err)
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, chave, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	k.Close()
	if err := gravarVersaoEm(chave, "0.5.0"); err != nil {
		t.Fatalf("chave existente: %v", err)
	}
	k, err = registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if v, _, err := k.GetStringValue("DisplayVersion"); err != nil || v != "0.5.0" {
		t.Fatalf("DisplayVersion = %q (%v), esperado 0.5.0", v, err)
	}
}

func TestChaveDesinstalacao_ConfereComInstallPs1(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	ps := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.Contains(ps, `$uninstallKey = 'HKCU:\`+chaveDesinstalacao+`'`) {
		t.Fatalf("install.ps1 não grava a chave HKCU %q", chaveDesinstalacao)
	}
	if !regexp.MustCompile(`(?m)^\s+` + valorVersao + `\s+= \$version$`).MatchString(ps) {
		t.Fatalf("install.ps1 não grava %s = $version", valorVersao)
	}
}
