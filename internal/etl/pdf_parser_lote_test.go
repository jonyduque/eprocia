package etl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// pdfComPaginas grava um PDF em que cada página é escaneada (imagem grande, sem
// texto) ou, se o número estiver em nativas, tem texto nativo suficiente.
func pdfComPaginas(t *testing.T, total int, nativas map[int]bool) string {
	t.Helper()
	objs := []string{"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj", ""}
	imagem := 3
	objs = append(objs, fmt.Sprintf("%d 0 obj\n<< /Type /XObject /Subtype /Image /Width 200 /Height 200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 0 >>\nstream\nendstream\nendobj", imagem))
	var kids []string
	for i := 1; i <= total; i++ {
		pag, cont := len(objs)+1, len(objs)+2
		kids = append(kids, fmt.Sprintf("%d 0 R", pag))
		var stream string
		if nativas[i] {
			stream = fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", strings.Repeat("texto nativo ", 12))
		} else {
			stream = "/Im1 Do"
		}
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 %d 0 R >> >> /Contents %d 0 R >>\nendobj", pag, imagem, cont),
			fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", cont, len(stream)+1, stream),
		)
	}
	objs[1] = fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), total)
	caminho := filepath.Join(t.TempDir(), "lote.pdf")
	if err := os.WriteFile(caminho, buildPDF(objs), 0644); err != nil {
		t.Fatal(err)
	}
	return caminho
}

// Com o OCR em lote, as páginas que vão ao OCR são divididas em no máximo
// "blocos" chamadas (B-EF-04), e o OCR por página não é usado.
func TestParsePages_OCREmLoteAgrupaPaginas(t *testing.T) {
	caminho := pdfComPaginas(t, 6, map[int]bool{3: true})

	var mu sync.Mutex
	var chamadas [][]int
	lote := func(ctx context.Context, path string, paginas []int) ([]string, error) {
		mu.Lock()
		chamadas = append(chamadas, append([]int(nil), paginas...))
		mu.Unlock()
		textos := make([]string, len(paginas))
		for i, p := range paginas {
			textos[i] = fmt.Sprintf("ocr da página %d", p)
		}
		return textos, nil
	}
	porPagina := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		t.Errorf("OCR por página chamado para a página %d com o lote disponível", pageNum)
		return "", "", nil
	}

	var concluidas int
	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 2).
		ParsePages(context.Background(), true, porPagina, func(int) { mu.Lock(); concluidas++; mu.Unlock() })
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}

	mu.Lock()
	got := map[string]bool{}
	for _, c := range chamadas {
		got[fmt.Sprint(c)] = true
	}
	mu.Unlock()
	want := map[string]bool{fmt.Sprint([]int{1, 2, 4}): true, fmt.Sprint([]int{5, 6}): true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocos de OCR = %v, quero [1 2 4] e [5 6]", chamadas)
	}
	for _, n := range []int{1, 2, 4, 5, 6} {
		pag := pages[fmt.Sprint(n)].(map[string]any)
		if pag["content"] != fmt.Sprintf("ocr da página %d", n) || pag["ocr"] != true {
			t.Errorf("página %d = %v", n, pag)
		}
	}
	if pag := pages["3"].(map[string]any); pag["ocr"] != false || !strings.Contains(pag["content"].(string), "texto nativo") {
		t.Errorf("página nativa 3 = %v", pag)
	}
	if concluidas != 6 {
		t.Errorf("onPageComplete chamado %d vezes, quero 6", concluidas)
	}
}

// Se o bloco falhar (uma página corrompida derruba o tesseract do bloco todo),
// cada página do bloco tenta de novo sozinha, para que uma não leve as outras.
func TestParsePages_OCREmLoteFalhaRefazPorPagina(t *testing.T) {
	caminho := pdfComPaginas(t, 3, nil)

	lote := func(ctx context.Context, path string, paginas []int) ([]string, error) {
		return nil, errors.New("tesseract caiu")
	}
	var mu sync.Mutex
	var refeitas []int
	porPagina := func(ctx context.Context, path string, pageNum int) (string, string, error) {
		mu.Lock()
		refeitas = append(refeitas, pageNum)
		mu.Unlock()
		if pageNum == 2 {
			return "", "", errors.New("página 2 ilegível")
		}
		return fmt.Sprintf("página %d sozinha", pageNum), "", nil
	}

	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 1).
		ParsePages(context.Background(), true, porPagina, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	if len(refeitas) != 3 {
		t.Fatalf("páginas refeitas uma a uma = %v, quero 1, 2 e 3", refeitas)
	}
	if c := pages["1"].(map[string]any)["content"]; c != "página 1 sozinha" {
		t.Errorf("página 1 = %q", c)
	}
	if c := pages["2"].(map[string]any)["content"].(string); !strings.Contains(c, "página 2 ilegível") {
		t.Errorf("página 2 deveria trazer o erro do OCR, veio %q", c)
	}
}
