//go:build windows

package atualizacao

import (
	"os"
	"strings"
	"testing"
)

func TestPaiDoProcesso(t *testing.T) {
	ppid, exe, err := PaiDoProcesso()
	if err != nil {
		t.Fatalf("PaiDoProcesso: %v", err)
	}
	if ppid != os.Getppid() {
		t.Errorf("ppid = %d, quer %d", ppid, os.Getppid())
	}
	if exe == "" || strings.Contains(exe, `\`) || !strings.HasSuffix(strings.ToLower(exe), ".exe") {
		t.Errorf("exe = %q, quer só o nome de um .exe", exe)
	}
}
