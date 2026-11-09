package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// nomeTravaInstancia dá o nome do mutex que impede duas instâncias do mesmo
// executável. O nome vem do caminho (sem diferença de maiúsculas, como no
// Windows), então cópias do eprocIA em outras pastas rodam ao mesmo tempo.
func nomeTravaInstancia(exe string) string {
	soma := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(exe))))
	return `Local\eprocIA-instancia-` + hex.EncodeToString(soma[:8])
}

// caminhoExecutavel resolve o caminho real do exe em execução; sem ele, usa
// os.Args[0] para ainda travar algo estável.
func caminhoExecutavel() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real
	}
	return exe
}
