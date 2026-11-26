package etl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"eprocia/internal/models"
	"eprocia/internal/ocr"
)

// caminhoFulano imita o caminho do temporário com o nome de usuário do Windows.
const caminhoFulano = `C:\Users\fulano\AppData\Local\Temp\eprocIA_bin\tesseract.exe`

// semCaminho falha se o texto traz um caminho ou o nome do usuário.
func semCaminho(t *testing.T, onde, texto string) {
	t.Helper()
	for _, proibido := range []string{`:\`, "fulano"} {
		if strings.Contains(texto, proibido) {
			t.Errorf("%s contém %q:\n%s", onde, proibido, texto)
		}
	}
}

func TestTextoErroSaida(t *testing.T) {
	curto := errors.New("malformed PDF: missing endobj")
	casos := []struct {
		nome string
		err  error
		quer string
	}{
		{"PathError", fmt.Errorf("embrulho: %w", &fs.PathError{Op: "open", Path: `C:\Users\fulano\AppData\Local\Temp\eprocia_pdf_1.pdf`, Err: fs.ErrNotExist}), textoErroDisco},
		{"exec.Error", &exec.Error{Name: caminhoFulano, Err: exec.ErrNotFound}, textoErroDisco},
		{"LinkError", &os.LinkError{Op: "rename", Old: `C:\a`, New: `C:\b`, Err: fs.ErrExist}, textoErroDisco},
		{"SyscallError", os.NewSyscallError("CreateFile", fs.ErrPermission), textoErroDisco},
		{"biblioteca curto passa igual", curto, curto.Error()},
		{"caminho no texto vai à rede de segurança", errors.New(`falha em C:\Users\fulano\x.pdf`), textoErroInterno},
		{"caminho com barra normal", errors.New(`falha em D:/dados/fulano/x.pdf`), textoErroInterno},
		{"caminho UNC", errors.New(`falha em \\servidor\fulano\x.pdf`), textoErroInterno},
		{"segundo leitor mantém a frase", fmt.Errorf("parse: %w", &erroSegundoLeitor{resumo: resumoSegundoLeitorPrazo, causa: errors.New(caminhoFulano)}), "segundo leitor: " + resumoSegundoLeitorPrazo},
		{"URL não é caminho", errors.New("ver http://exemplo"), "ver http://exemplo"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := textoErroSaida(c.err); got != c.quer {
				t.Errorf("textoErroSaida = %q, quero %q", got, c.quer)
			}
		})
	}
}

// I1: o OCR que falha com um caminho no erro escreve só a frase fixa na
// página, em ocr_erro e no .txt.
func TestRegistrarOCR_FraseFixaSemCaminho(t *testing.T) {
	casos := []struct {
		nome     string
		err      error
		marcador string
		leitura  string
	}{
		{"prazo", fmt.Errorf("falha no tesseract embutido em %s: %w", caminhoFulano, ocr.ErrPrazo), marcadorOCRPrazo, "prazo"},
		{"outro", fmt.Errorf("tesseract.exe embutido não encontrado em %s", caminhoFulano), marcadorOCRFalha, "ocr_falhou"},
	}
	for _, c := range casos {
		t.Run(c.nome+"/página", func(t *testing.T) {
			caminho := filepath.Join(t.TempDir(), "doc.pdf")
			if err := os.WriteFile(caminho, pdfDuasPaginasEscaneadas(), 0644); err != nil {
				t.Fatal(err)
			}
			runner := func(context.Context, string, int) (string, error) { return "", c.err }
			pages, err := parserDeArquivo(t, caminho).ParsePages(context.Background(), true, runner, nil)
			if err != nil {
				t.Fatalf("ParsePages: %v", err)
			}
			pag := pages["1"].(map[string]any)
			content, _ := pag["content"].(string)
			ocrErro, _ := pag["ocr_erro"].(string)
			semCaminho(t, "content", content)
			semCaminho(t, "ocr_erro", ocrErro)
			if content != c.marcador {
				t.Errorf("content = %q, quero %q", content, c.marcador)
			}
			if ocrErro == "" {
				t.Errorf("ocr_erro vazio: %v", pag)
			}
		})
		t.Run(c.nome+"/txt", func(t *testing.T) {
			isolarTemp(t)
			antigoPagina, antigoPaginas := executarOCR, executarOCRPaginas
			t.Cleanup(func() { executarOCR, executarOCRPaginas = antigoPagina, antigoPaginas })
			executarOCR = func(context.Context, string, int, string) (string, error) { return "", c.err }
			executarOCRPaginas = func(context.Context, string, []int, string, func(int)) ([]string, error) {
				return nil, c.err
			}
			zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfDuasPaginasEscaneadas()}})
			atv := NovaAtividade("x")
			out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath,
				&models.Config{CleaningLevel: "none", OcrMode: "fast"}, func(float64, string, int, int, int, int) {}, atv)
			if err != nil {
				t.Fatalf("ProcessZip: %v", err)
			}
			b, _ := os.ReadFile(out)
			d := atv.Instantaneo().Docs[0]
			semCaminho(t, ".txt", string(b))
			semCaminho(t, "Erro", d.Erro)
			semCaminho(t, "Detalhe", d.Detalhe)
			if !strings.Contains(string(b), c.marcador) {
				t.Errorf(".txt sem %q:\n%s", c.marcador, b)
			}
			if d.Leitura != c.leitura {
				t.Errorf("leitura = %q, quero %q", d.Leitura, c.leitura)
			}
		})
	}
}

// I2: o panic recuperado com um caminho no valor não leva o caminho ao .txt
// nem ao painel.
func TestPipeline_PanicComCaminhoNaoVaiAoTxt(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto("Texto ficticio de FULANO DE TAL")}})
	atv := NovaAtividade("x")
	progresso := func(_ float64, msg string, _, _, _, _ int) {
		if strings.HasPrefix(msg, "Processando ") {
			panic(`falha ao ler C:\Users\fulano\AppData\Local\Temp\eprocia_pdf_1.pdf`)
		}
	}
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath,
		&models.Config{CleaningLevel: "none", OcrMode: "none"}, progresso, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	d := atv.Instantaneo().Docs[0]
	semCaminho(t, ".txt", string(b))
	semCaminho(t, "Erro", d.Erro)
	semCaminho(t, "Detalhe", d.Detalhe)
	if d.Leitura != string(LeituraIlegivel) {
		t.Errorf("leitura = %q", d.Leitura)
	}
	if !strings.Contains(string(b), "[ERRO ao processar documento: "+textoErroInterno+"]") {
		t.Errorf(".txt sem o marcador fixo:\n%s", b)
	}
}

// M6: um panic depois de o documento ter sido encerrado não o reescreve como
// não lido.
func TestPipeline_PanicDepoisDeEncerrarNaoReescreve(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto("Texto ficticio de FULANO DE TAL")}})
	atv := NovaAtividade("x")
	progresso := func(_ float64, msg string, _, _, _, _ int) {
		if strings.HasPrefix(msg, "Concluído ") {
			panic("panic tardio")
		}
	}
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath,
		&models.Config{CleaningLevel: "none", OcrMode: "none"}, progresso, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, _ := os.ReadFile(out)
	d := atv.Instantaneo().Docs[0]
	if d.Leitura != string(LeituraLido) {
		t.Errorf("leitura = %q, quero lido (%+v)", d.Leitura, d)
	}
	if strings.Contains(string(b), "[documento não lido") || !strings.Contains(string(b), "FULANO DE TAL") {
		t.Errorf(".txt reescrito pelo panic tardio:\n%s", b)
	}
}
