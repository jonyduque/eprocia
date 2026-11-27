package etl

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// gravarPDFSintetico grava bytes quaisquer com nome de PDF: LerEntrada não
// confere a assinatura %PDF.
func gravarPDFSintetico(t *testing.T, dir, nome, conteudo string) string {
	t.Helper()
	caminho := filepath.Join(dir, nome)
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func lerTudo(t *testing.T, abrir func() (io.ReadCloser, error)) []byte {
	t.Helper()
	if abrir == nil {
		t.Fatal("Abrir é nil")
	}
	r, err := abrir()
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return b
}

func TestLerEntrada_ExtensaoNaoSuportada(t *testing.T) {
	dir := t.TempDir()
	for _, nome := range []string{"nota.txt", "peticao.docx"} {
		c := gravarPDFSintetico(t, dir, nome, "x")
		if _, _, err := LerEntrada([]string{c}); !errors.Is(err, ErrTipoNaoSuportado) {
			t.Errorf("%s: err = %v, quer ErrTipoNaoSuportado", nome, err)
		}
	}
}

func TestLerEntrada_MisturaZipEPDF(t *testing.T) {
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - INIC1.pdf", []byte("%PDF-1.4")}})
	pdf := gravarPDFSintetico(t, t.TempDir(), "anexo.pdf", "%PDF-1.4")
	if _, _, err := LerEntrada([]string{zipPath, pdf}); !errors.Is(err, ErrTipoNaoSuportado) {
		t.Errorf("err = %v, quer ErrTipoNaoSuportado", err)
	}
}

func TestLerEntrada_ListaVazia(t *testing.T) {
	if _, _, err := LerEntrada(nil); !errors.Is(err, ErrTipoNaoSuportado) {
		t.Errorf("err = %v, quer ErrTipoNaoSuportado", err)
	}
}

func TestLerEntrada_DoisZips(t *testing.T) {
	a := gravarZip(t, []entradaZip{{"Evento 1 - INIC1.pdf", []byte("%PDF-1.4")}})
	b := gravarZip(t, []entradaZip{{"Evento 2 - INIC1.pdf", []byte("%PDF-1.4")}})
	if _, _, err := LerEntrada([]string{a, b}); !errors.Is(err, ErrTipoNaoSuportado) {
		t.Errorf("err = %v, quer ErrTipoNaoSuportado", err)
	}
}

func TestLerEntrada_Zip(t *testing.T) {
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 2 - SENT1.pdf", []byte("%PDF-1.4 sentenca")},
		{"Evento 1 - INIC1.html", []byte("<p>FULANO DE TAL</p>")},
	})
	maiusculo := strings.TrimSuffix(zipPath, ".zip") + ".ZIP"
	if err := os.Rename(zipPath, maiusculo); err != nil {
		t.Fatal(err)
	}
	arquivos, closer, err := LerEntrada([]string{maiusculo})
	if err != nil {
		t.Fatalf("LerEntrada: %v", err)
	}
	rc, ok := closer.(*zip.ReadCloser)
	if !ok {
		t.Fatalf("Closer = %T, quer *zip.ReadCloser", closer)
	}
	defer rc.Close()
	if len(arquivos) != 2 {
		t.Fatalf("len = %d, quer 2", len(arquivos))
	}
	for _, f := range arquivos {
		if f.Avulso {
			t.Errorf("%s: Avulso numa entrada de ZIP", f.Name)
		}
		if f.ZipFileRef == nil {
			t.Fatalf("%s: sem ZipFileRef", f.Name)
		}
		if f.Tamanho != int64(f.ZipFileRef.UncompressedSize64) {
			t.Errorf("%s: Tamanho = %d, quer %d", f.Name, f.Tamanho, f.ZipFileRef.UncompressedSize64)
		}
		quer := lerTudo(t, f.ZipFileRef.Open)
		if got := lerTudo(t, f.Abrir); !bytes.Equal(got, quer) {
			t.Errorf("%s: Abrir leu %q, quer %q", f.Name, got, quer)
		}
	}
}

func TestLerEntrada_PDFs(t *testing.T) {
	dir := t.TempDir()
	conteudos := map[string]string{
		"5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf": "%PDF-1.4 sentenca de FULANO DE TAL",
		"anexo.PDF": "%PDF-1.7 anexo",
		"5000000-00.2026.8.26.0001 - Evento 3 - INIC1.pdf": "%PDF-1.4 inicial",
	}
	ordem := []string{
		"5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf",
		"anexo.PDF",
		"5000000-00.2026.8.26.0001 - Evento 3 - INIC1.pdf",
	}
	var caminhos []string
	for _, nome := range ordem {
		caminhos = append(caminhos, gravarPDFSintetico(t, dir, nome, conteudos[nome]))
	}
	arquivos, closer, err := LerEntrada(caminhos)
	if err != nil {
		t.Fatalf("LerEntrada: %v", err)
	}
	if closer == nil {
		t.Fatal("Closer nil")
	}
	if len(arquivos) != 3 {
		t.Fatalf("len = %d, quer 3", len(arquivos))
	}
	for i, f := range arquivos {
		nome := ordem[i]
		if !f.Avulso {
			t.Errorf("%s: sem Avulso", nome)
		}
		if f.Name != nome || f.DocName != nome {
			t.Errorf("posição %d: Name/DocName = %q/%q, quer %q", i, f.Name, f.DocName, nome)
		}
		if f.Extension != ".pdf" {
			t.Errorf("%s: Extension = %q", nome, f.Extension)
		}
		if f.ZipFileRef != nil {
			t.Errorf("%s: ZipFileRef num PDF avulso", nome)
		}
		if f.Tamanho != int64(len(conteudos[nome])) {
			t.Errorf("%s: Tamanho = %d, quer %d", nome, f.Tamanho, len(conteudos[nome]))
		}
		if got := lerTudo(t, f.Abrir); string(got) != conteudos[nome] {
			t.Errorf("%s: Abrir leu %q", nome, got)
		}
	}
	if err := closer.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func conferirErroSemCaminho(t *testing.T, err error, frase, dir string) {
	t.Helper()
	if err == nil {
		t.Fatalf("sem erro, quer %q", frase)
	}
	if err.Error() != frase {
		t.Errorf("erro = %q, quer %q", err.Error(), frase)
	}
	if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), `\`) {
		t.Errorf("erro traz caminho: %q", err.Error())
	}
}

func TestLerEntrada_UmPDFInexistente(t *testing.T) {
	dir := t.TempDir()
	_, _, err := LerEntrada([]string{filepath.Join(dir, "sumiu.pdf")})
	conferirErroSemCaminho(t, err, "arquivo não encontrado", dir)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("errors.Is(err, fs.ErrNotExist) = false: %v", err)
	}
}

func TestLerEntrada_UmPDFVazio(t *testing.T) {
	dir := t.TempDir()
	c := gravarPDFSintetico(t, dir, "vazio.pdf", "")
	_, _, err := LerEntrada([]string{c})
	conferirErroSemCaminho(t, err, "arquivo vazio", dir)
}

func TestLerEntrada_DiretorioComNomePDF(t *testing.T) {
	dir := t.TempDir()
	pasta := filepath.Join(dir, "x.pdf")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := LerEntrada([]string{pasta})
	conferirErroSemCaminho(t, err, "arquivo não encontrado", dir)

	bom := gravarPDFSintetico(t, dir, "bom.pdf", "%PDF-1.4 bom")
	arquivos, _, err := LerEntrada([]string{pasta, bom})
	if err != nil {
		t.Fatalf("com um PDF bom: %v", err)
	}
	if len(arquivos) != 2 {
		t.Fatalf("len = %d, quer 2", len(arquivos))
	}
	if arquivos[0].DocName != "x.pdf" || arquivos[0].Tamanho != 0 {
		t.Errorf("pasta: DocName/Tamanho = %q/%d, quer x.pdf/0", arquivos[0].DocName, arquivos[0].Tamanho)
	}
	if arquivos[1].Tamanho != int64(len("%PDF-1.4 bom")) {
		t.Errorf("bom: Tamanho = %d", arquivos[1].Tamanho)
	}
}

func TestLerEntrada_UmPDFTravado(t *testing.T) {
	dir := t.TempDir()
	c := gravarPDFSintetico(t, dir, "travado.pdf", "%PDF-1.4 travado")
	p, err := windows.UTF16PtrFromString(c)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0 /* dwShareMode */, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	defer windows.CloseHandle(h)

	_, _, err = LerEntrada([]string{c})
	conferirErroSemCaminho(t, err, "falha de leitura ou gravação em disco", dir)
}

func TestLerEntrada_VariosComUmInexistente(t *testing.T) {
	dir := t.TempDir()
	a := gravarPDFSintetico(t, dir, "Evento 1 - INIC1.pdf", "%PDF-1.4 a")
	sumiu := filepath.Join(dir, "Evento 2 - PET1.pdf")
	b := gravarPDFSintetico(t, dir, "Evento 3 - SENT1.pdf", "%PDF-1.4 bb")
	arquivos, _, err := LerEntrada([]string{a, sumiu, b})
	if err != nil {
		t.Fatalf("LerEntrada: %v", err)
	}
	if len(arquivos) != 3 {
		t.Fatalf("len = %d, quer 3", len(arquivos))
	}
	if arquivos[1].DocName != "Evento 2 - PET1.pdf" || arquivos[1].Tamanho != 0 {
		t.Errorf("ausente: %q/%d", arquivos[1].DocName, arquivos[1].Tamanho)
	}
	if arquivos[0].Tamanho != 10 || arquivos[2].Tamanho != 11 {
		t.Errorf("tamanhos = %d, %d", arquivos[0].Tamanho, arquivos[2].Tamanho)
	}
}

func TestLerEntrada_VariosTodosInexistentes(t *testing.T) {
	dir := t.TempDir()
	_, _, err := LerEntrada([]string{filepath.Join(dir, "a.pdf"), filepath.Join(dir, "b.pdf")})
	conferirErroSemCaminho(t, err, "arquivo não encontrado", dir)
}
