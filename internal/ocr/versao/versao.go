// Package versao calcula o carimbo dos binários embutidos no pacote ocr. Fica
// separado para o gerador (go generate) usá-lo sem depender do próprio ocr,
// que só compila depois que versao_embed.go existe.
package versao

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
)

// Calcular devolve um hash do caminho, tamanho e conteúdo de cada arquivo sob
// raiz, em ordem lexical. Lê em streaming.
func Calcular(fsys fs.FS, raiz string) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, info.Size())
		f, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(h, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)[:16]), nil
}
