package etl

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPipeline_ClassificaNaoLido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto("Texto sadio ficticio")},
		{"Evento 2 - DOC2.pdf", []byte("não é PDF")},
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
	if d := docs[1]; d.Leitura != "ilegivel" || d.Erro != "arquivo ilegível" || !strings.Contains(d.Detalhe, "invalid header") {
		t.Errorf("documento ruim: %+v", d)
	}
	if docs[0].Leitura == "ilegivel" {
		t.Errorf("documento sadio classificado como ilegível: %+v", docs[0])
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "[ERRO ao analisar PDF:") {
		t.Errorf(".txt perdeu o marcador de erro:\n%s", b)
	}
}

func TestPipeline_EntradasDuplicadas_MotivoNaEntradaCerta(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZip(t, []entradaZip{
		{"Evento 1 - DOC1.pdf", pdfComTexto("Texto sadio ficticio")},
		{"Evento 1 - DOC1.pdf", []byte("não é PDF")},
	})
	atv := NovaAtividade("x")
	_, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfgSemOCR(),
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if n := atv.NaoLidos(); n != 1 {
		t.Errorf("NaoLidos() = %d, esperava 1", n)
	}
	docs := atv.Instantaneo().Docs
	if len(docs) != 2 {
		t.Fatalf("docs = %d, esperava 2", len(docs))
	}
	if docs[0].Leitura == "ilegivel" || docs[1].Leitura != "ilegivel" {
		t.Errorf("motivo na entrada errada: %+v / %+v", docs[0], docs[1])
	}
}
