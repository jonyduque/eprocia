package atualizacao

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// auxiliar é o caminho do auxiliar.exe compilado uma vez pelo TestMain.
var auxiliar string

func TestMain(m *testing.M) {
	os.Exit(rodarComAuxiliar(m))
}

func rodarComAuxiliar(m *testing.M) int {
	dir, err := os.MkdirTemp("", "eprocia-aux-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "criar pasta do auxiliar:", err)
		return 2
	}
	defer os.RemoveAll(dir)
	nome := "auxiliar"
	if runtime.GOOS == "windows" {
		nome = "auxiliar.exe"
	}
	auxiliar = filepath.Join(dir, nome)
	cmd := exec.Command("go", "build", "-o", auxiliar, "./testdata/auxiliar")
	if saida, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "compilar o auxiliar: %v\n%s", err, saida)
		return 2
	}
	return m.Run()
}
