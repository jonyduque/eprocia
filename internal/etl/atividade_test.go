package etl

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"eprocia/internal/models"
)

func zipDeTeste(t *testing.T, entradas map[string][]byte, ordem []string) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "processo.zip")
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, nome := range ordem {
		w, err := zw.Create(nome)
		if err != nil {
			t.Fatalf("falha ao criar entrada: %v", err)
		}
		_, _ = w.Write(entradas[nome])
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("falha ao fechar zip: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao gravar zip: %v", err)
	}
	return zipPath
}

// Documentos com erro também terminam: sem contá-los, "Docs: x/y" nunca
// chegava ao total enquanto o ZIP processava.
func TestProcessZip_DocumentosComErroEntramNosContadores(t *testing.T) {
	zipPath := zipDeTeste(t, map[string][]byte{
		"Evento 1 - DOC1.pdf":  []byte("%PDF-1.4 corrompido"),
		"Evento 2 - DOC1.html": []byte("<html><body><p>texto</p></body></html>"),
	}, []string{"Evento 1 - DOC1.pdf", "Evento 2 - DOC1.html"})

	var mu sync.Mutex
	var ultimoDocs, totalDocs, ultimasPags, totalPags int
	atv := NovaAtividade("item-1")
	cfg := &models.Config{CleaningLevel: "none", OcrMode: "none"}
	_, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfg,
		func(_ float64, _ string, pd, td, pp, tp int) {
			mu.Lock()
			defer mu.Unlock()
			ultimoDocs, totalDocs = max(ultimoDocs, pd), td
			ultimasPags, totalPags = max(ultimasPags, pp), tp
		}, atv)
	if err != nil {
		t.Fatalf("ProcessZip falhou: %v", err)
	}
	if ultimoDocs != totalDocs || totalDocs != 2 {
		t.Errorf("docs %d/%d, esperava 2/2", ultimoDocs, totalDocs)
	}
	if ultimasPags != totalPags {
		t.Errorf("páginas %d/%d, esperava chegar ao total", ultimasPags, totalPags)
	}

	inst := atv.Instantaneo()
	if inst.ItemID != "item-1" || len(inst.Docs) != 2 {
		t.Fatalf("instantâneo inesperado: %+v", inst)
	}
	if d := inst.Docs[0]; d.Estado != DocErro || d.Erro == "" || d.Evento != 1 {
		t.Errorf("PDF corrompido: %+v, esperava estado erro com mensagem", d)
	}
	if d := inst.Docs[1]; d.Estado != DocConcluido || d.Feitas != 1 || d.Paginas != 1 {
		t.Errorf("HTML: %+v, esperava concluído com 1/1 página", d)
	}
	if inst.LimiteDocs < 1 || inst.LimiteOCR < 1 || inst.OCRAtivos != 0 {
		t.Errorf("limites/OCR inesperados: %+v", inst)
	}
}

func TestAtividade_OCRMudaEstadoEConta(t *testing.T) {
	a := NovaAtividade("x")
	a.iniciar([]AtividadeDoc{{Nome: "a.pdf", Estado: DocAguardando, Paginas: 3}}, 2, 2)
	a.docLendo(0)
	a.ocrInicio(0, 2)
	if inst := a.Instantaneo(); inst.OCRAtivos != 1 || inst.Docs[0].Estado != DocOCR || inst.Docs[0].OCR != 2 {
		t.Fatalf("após ocrInicio: %+v", inst)
	}
	v := a.Versao()
	a.ocrFim(0, 2)
	if a.Versao() == v {
		t.Error("versão não mudou")
	}
	if inst := a.Instantaneo(); inst.OCRAtivos != 0 || inst.Docs[0].Estado != DocLendo || inst.Docs[0].OCR != 0 {
		t.Fatalf("após ocrFim: %+v", inst)
	}
	// O instantâneo é cópia: mudar a atividade depois não o altera.
	inst := a.Instantaneo()
	a.docFim(0, "")
	if inst.Docs[0].Estado != DocLendo {
		t.Error("instantâneo compartilha memória com a atividade")
	}
	var nada *Atividade
	nada.docPagina(0)
	if nada.Instantaneo() != nil {
		t.Error("atividade nil deveria devolver instantâneo nil")
	}
}

func TestAtividade_NaoLidos(t *testing.T) {
	a := NovaAtividade("x")
	a.iniciar(make([]AtividadeDoc, 3), 1, 1)
	a.docLeitura(0, LeituraIlegivel, "detalhe")
	a.docLeitura(1, LeituraOCR, "")
	a.docLeitura(2, LeituraLido, "")
	if n := a.NaoLidos(); n != 1 {
		t.Errorf("NaoLidos() = %d, esperava 1", n)
	}
	if d := a.Instantaneo().Docs[0]; d.Leitura != "ilegivel" || d.Detalhe != "detalhe" {
		t.Errorf("doc 0: %+v", d)
	}
	var nula *Atividade
	nula.docLeitura(0, LeituraIlegivel, "")
	if nula.NaoLidos() != 0 {
		t.Error("atividade nil deveria ter 0 não lidos")
	}
}
