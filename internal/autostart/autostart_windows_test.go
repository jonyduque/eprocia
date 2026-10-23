//go:build windows

package autostart

import (
	"errors"
	"fmt"
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
