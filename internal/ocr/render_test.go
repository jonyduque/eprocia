package ocr

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// O pdftoppm grava as páginas em PGM (cinza, sem compressão): o PNG custava
// cerca de 0,5 s por página só para codificar o que o tesseract decodifica
// em seguida (B-EF-01).
func TestRenderizar_GravaPGM(t *testing.T) {
	f, err := prepararFerramentas("fast")
	if err != nil {
		t.Fatalf("falha ao preparar as ferramentas: %v", err)
	}
	caminho := pdfDePalavras(t, []string{"PRIMEIRA", "SEGUNDA"})
	dir := t.TempDir()
	if err := f.renderizar(context.Background(), caminho, dir, 1, 2); err != nil {
		t.Fatalf("renderizar: %v", err)
	}
	for _, nome := range []string{"p-1.pgm", "p-2.pgm"} {
		dados, err := os.ReadFile(filepath.Join(dir, nome))
		if err != nil {
			t.Fatalf("imagem %s ausente: %v", nome, err)
		}
		if !bytes.HasPrefix(dados, []byte("P5")) {
			t.Errorf("%s não é PGM binário: começa com %q", nome, dados[:min(len(dados), 8)])
		}
	}
}
