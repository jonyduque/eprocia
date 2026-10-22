package ocr

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEmbed_SemOsdNemEngDuplicado(t *testing.T) {
	for _, modo := range []string{"fast", "normal", "best"} {
		if _, err := fs.Stat(EmbeddedFiles, "embed/tessdata/"+modo+"/osd.traineddata"); err == nil {
			t.Errorf("osd.traineddata do modo %s está embutido e nunca é usado", modo)
		}
		if _, err := fs.Stat(EmbeddedFiles, "embed/tessdata/"+modo+"/por.traineddata"); err != nil {
			t.Errorf("por.traineddata do modo %s ausente: %v", modo, err)
		}
	}
	engs := 0
	_ = fs.WalkDir(EmbeddedFiles, "embed/tessdata", func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Name() == "eng.traineddata" {
			engs++
		}
		return nil
	})
	if engs != 1 {
		t.Errorf("esperava um eng.traineddata compartilhado, há %d", engs)
	}
}

func TestVersaoEmbutidos_Atualizada(t *testing.T) {
	v, err := CalcularVersaoEmbutidos()
	if err != nil {
		t.Fatal(err)
	}
	if v != versaoEmbutidos {
		t.Fatalf("versao_embed.go desatualizado (%s, calculado %s): rode go generate ./internal/ocr", versaoEmbutidos, v)
	}
}

func alocado(f func()) uint64 {
	var a, d runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	f()
	runtime.ReadMemStats(&d)
	return d.TotalAlloc - a.TotalAlloc
}

// A extração lia cada arquivo embutido para o heap e o arquivo já extraído
// também, para comparar: ≈350 MB a cada processo, mesmo sem nada a gravar.
func TestExtrairBase_JaExtraidoNaoLeConteudo(t *testing.T) {
	dest := t.TempDir()
	var err error
	primeira := alocado(func() { err = extrairBase(dest) })
	if err != nil {
		t.Fatal(err)
	}
	if primeira > 4<<20 {
		t.Errorf("primeira extração alocou %d B; a cópia deveria ser em streaming", primeira)
	}
	exe := filepath.Join(dest, "tesseract", "tesseract.exe")
	antes, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}

	segunda := alocado(func() { err = extrairBase(dest) })
	if err != nil {
		t.Fatal(err)
	}
	if segunda > 1<<20 {
		t.Errorf("verificação com tudo extraído alocou %d B", segunda)
	}
	depois, _ := os.Stat(exe)
	if !depois.ModTime().Equal(antes.ModTime()) {
		t.Errorf("tesseract.exe regravado sem necessidade")
	}
	if _, err := os.Stat(filepath.Join(dest, "tessdata")); err == nil {
		t.Errorf("extrairBase não deveria extrair tessdata")
	}
}

func TestExtrairTessdata_SoModoPedidoComEng(t *testing.T) {
	dest := t.TempDir()
	dir, err := extrairTessdata(dest, "fast")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(dest, "tessdata", "fast") {
		t.Errorf("diretório inesperado: %s", dir)
	}
	for _, nome := range []string{"por.traineddata", "eng.traineddata"} {
		if _, err := os.Stat(filepath.Join(dir, nome)); err != nil {
			t.Errorf("%s não extraído: %v", nome, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "osd.traineddata")); err == nil {
		t.Errorf("osd.traineddata extraído")
	}
	for _, outro := range []string{"normal", "best"} {
		if _, err := os.Stat(filepath.Join(dest, "tessdata", outro)); err == nil {
			t.Errorf("tessdata do modo %s extraído sem ser pedido", outro)
		}
	}
}

func TestExtrairTessdata_RegravaQuandoDesatualizado(t *testing.T) {
	dest := t.TempDir()
	dir, err := extrairTessdata(dest, "fast")
	if err != nil {
		t.Fatal(err)
	}
	por := filepath.Join(dir, "por.traineddata")
	original, _ := fs.ReadFile(EmbeddedFiles, "embed/tessdata/fast/por.traineddata")

	// Mesmo tamanho, carimbo de outra versão: precisa regravar.
	if err := os.WriteFile(por, make([]byte, len(original)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, arquivoVersao), []byte("versao-antiga"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := extrairTessdata(dest, "fast"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(por); !bytes.Equal(got, original) {
		t.Errorf("por.traineddata não regravado com carimbo diferente")
	}

	// Carimbo certo, tamanho errado (gravação interrompida): precisa regravar.
	if err := os.WriteFile(por, []byte("truncado"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := extrairTessdata(dest, "fast"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(por); !bytes.Equal(got, original) {
		t.Errorf("por.traineddata truncado não foi regravado")
	}
}
