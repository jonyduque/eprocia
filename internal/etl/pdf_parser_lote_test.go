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
	"time"
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
	lote := func(ctx context.Context, path string, paginas []int, _ func(int)) ([]string, error) {
		mu.Lock()
		chamadas = append(chamadas, append([]int(nil), paginas...))
		mu.Unlock()
		textos := make([]string, len(paginas))
		for i, p := range paginas {
			textos[i] = fmt.Sprintf("ocr da página %d", p)
		}
		return textos, nil
	}
	porPagina := func(ctx context.Context, path string, pageNum int) (string, error) {
		t.Errorf("OCR por página chamado para a página %d com o lote disponível", pageNum)
		return "", nil
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

	lote := func(ctx context.Context, path string, paginas []int, _ func(int)) ([]string, error) {
		return nil, errors.New("tesseract caiu")
	}
	var mu sync.Mutex
	var refeitas []int
	porPagina := func(ctx context.Context, path string, pageNum int) (string, error) {
		mu.Lock()
		refeitas = append(refeitas, pageNum)
		mu.Unlock()
		if pageNum == 2 {
			return "", errors.New("página 2 ilegível")
		}
		return fmt.Sprintf("página %d sozinha", pageNum), nil
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

// O progresso anda página a página dentro do bloco: cada página avisada pelo
// OCR em lote chega ao onPageComplete antes de o bloco terminar.
func TestParsePages_OCREmLoteAvisaCadaPagina(t *testing.T) {
	caminho := pdfComPaginas(t, 4, map[int]bool{2: true})

	var mu sync.Mutex
	concluidas := map[int]int{}
	lote := func(ctx context.Context, path string, paginas []int, aoConcluir func(int)) ([]string, error) {
		textos := make([]string, len(paginas))
		for i, p := range paginas {
			textos[i] = fmt.Sprintf("ocr da página %d", p)
			aoConcluir(p)
			mu.Lock()
			n := concluidas[p]
			mu.Unlock()
			if n != 1 {
				t.Errorf("página %d avisada pelo lote, mas onPageComplete visto %d vez(es) antes do fim do bloco", p, n)
			}
		}
		return textos, nil
	}

	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 1).
		ParsePages(context.Background(), true, porPaginaProibido(t), func(p int) { mu.Lock(); concluidas[p]++; mu.Unlock() })
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	for p := 1; p <= 4; p++ {
		if concluidas[p] != 1 {
			t.Errorf("onPageComplete da página %d chamado %d vez(es), quero 1", p, concluidas[p])
		}
		if _, tem := pages[fmt.Sprint(p)].(map[string]any)["image"]; tem {
			t.Errorf("página %d não deveria ter o campo image", p)
		}
	}
}

// Um bloco cheio vai ao OCR assim que as páginas dele são conhecidas, sem
// esperar a leitura do texto nativo das páginas seguintes.
func TestParsePages_OCREmLoteComecaAntesDoFimDaLeitura(t *testing.T) {
	caminho := pdfComPaginas(t, 8, map[int]bool{5: true, 6: true, 7: true, 8: true})

	começou := make(chan struct{})
	var once sync.Once
	lote := func(ctx context.Context, path string, paginas []int, aoConcluir func(int)) ([]string, error) {
		once.Do(func() { close(começou) })
		return make([]string, len(paginas)), nil
	}
	onPage := func(p int) {
		if p != 5 {
			return
		}
		// A página 5 é nativa e só é lida depois do bloco [1 2 3 4] estar
		// completo; o OCR dele já tem de estar em andamento.
		select {
		case <-começou:
		case <-time.After(2 * time.Second):
			t.Errorf("o OCR do bloco das páginas 1 a 4 não começou durante a leitura das páginas seguintes")
		}
	}

	if _, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 2).
		ParsePages(context.Background(), true, porPaginaProibido(t), onPage); err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
}

// Se o bloco cai depois de avisar algumas páginas, a repetição página a página
// não conta essas páginas de novo no progresso.
func TestParsePages_OCREmLoteFalhaNaoRepeteProgresso(t *testing.T) {
	caminho := pdfComPaginas(t, 3, nil)

	lote := func(ctx context.Context, path string, paginas []int, aoConcluir func(int)) ([]string, error) {
		aoConcluir(paginas[0])
		aoConcluir(paginas[1])
		return nil, errors.New("tesseract caiu na terceira página")
	}
	porPagina := func(ctx context.Context, path string, pageNum int) (string, error) {
		return fmt.Sprintf("página %d sozinha", pageNum), nil
	}
	var mu sync.Mutex
	concluidas := map[int]int{}
	pages, err := parserDeArquivo(t, caminho).comOCREmLote(lote, 1).
		ParsePages(context.Background(), true, porPagina, func(p int) { mu.Lock(); concluidas[p]++; mu.Unlock() })
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	for p := 1; p <= 3; p++ {
		if concluidas[p] != 1 {
			t.Errorf("onPageComplete da página %d chamado %d vez(es), quero 1", p, concluidas[p])
		}
		if c := pages[fmt.Sprint(p)].(map[string]any)["content"]; c != fmt.Sprintf("página %d sozinha", p) {
			t.Errorf("página %d = %q", p, c)
		}
	}
}

func porPaginaProibido(t *testing.T) func(context.Context, string, int) (string, error) {
	return func(_ context.Context, _ string, pageNum int) (string, error) {
		t.Errorf("OCR por página chamado para a página %d com o lote disponível", pageNum)
		return "", nil
	}
}
