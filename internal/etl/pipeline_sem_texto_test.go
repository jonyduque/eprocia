package etl

import (
	"context"
	"os"
	"strings"
	"testing"
)

// pdfEmBranco monta um PDF sintético de uma página sem texto e sem imagem.
func pdfEmBranco() []byte {
	stream := "q Q"
	return buildPDF([]string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length " + string(rune('0'+len(stream))) + " >>\nstream\n" + stream + "\nendstream\nendobj",
	})
}

func TestPipeline_DocumentoSemTextoViraNaoLido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto("Texto sadio ficticio")},
		{"Evento 2 - DOC2.pdf", pdfEmBranco()},
	})
	atv := NovaAtividade("x")
	outPath, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if n := atv.NaoLidos(); n != 1 {
		t.Errorf("NaoLidos() = %d, esperava 1", n)
	}
	docs := atv.Instantaneo().Docs
	if d := docs[1]; d.Leitura != "sem_texto" || d.Erro != "nenhum texto encontrado" {
		t.Errorf("documento em branco: %+v", d)
	}
	if d := docs[0]; d.Leitura != "lido" {
		t.Errorf("documento com texto: %+v", d)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "Documentos não lidos: 1") {
		t.Errorf(".txt sem o contador:\n%s", s)
	}
	if !strings.Contains(s, "[documento não lido: nenhum texto encontrado; consulte o PDF original]") {
		t.Errorf(".txt sem a linha do documento em branco:\n%s", s)
	}
	if !strings.Contains(s, "Texto sadio ficticio") {
		t.Errorf(".txt perdeu o texto do outro documento:\n%s", s)
	}
	naoLidoNoSumario := false
	for linha := range strings.SplitSeq(s, "\n") {
		if strings.Contains(linha, "DOC2") && strings.Contains(linha, "NÃO LIDO") {
			naoLidoNoSumario = true
		}
		if strings.Contains(linha, "DOC1") && strings.Contains(linha, "NÃO LIDO") {
			t.Errorf("documento sadio marcado NÃO LIDO: %s", linha)
		}
	}
	if !naoLidoNoSumario {
		t.Errorf("sumário sem NÃO LIDO para o documento em branco:\n%s", s)
	}
}

func TestPipeline_HTMLSemTextoViraNaoLido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - VAZIO.html", []byte("<html><body></body></html>")},
	})
	atv := NovaAtividade("x")
	outPath, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if d := atv.Instantaneo().Docs[0]; d.Leitura != "sem_texto" {
		t.Errorf("HTML vazio: %+v", d)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "[documento não lido: nenhum texto encontrado; consulte o arquivo original]") {
		t.Errorf(".txt sem a linha do HTML vazio:\n%s", b)
	}
}
