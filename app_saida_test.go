package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eprocia/internal/etl"
)

func TestLerSaidaParaCopiar_SoAceitaTxtDeItemConcluido(t *testing.T) {
	dir := t.TempDir()
	saida := filepath.Join(dir, "processo.txt")
	if err := os.WriteFile(saida, []byte("<contexto>conteúdo</contexto>"), 0o644); err != nil {
		t.Fatal(err)
	}
	outro := filepath.Join(dir, "segredo.txt")
	if err := os.WriteFile(outro, []byte("não copiar"), 0o644); err != nil {
		t.Fatal(err)
	}
	naoTxt := filepath.Join(dir, "processo.zip")
	if err := os.WriteFile(naoTxt, []byte("PK"), 0o644); err != nil {
		t.Fatal(err)
	}
	itens := []*etl.QueueItem{
		{Status: "success", Kind: "zip", Path: filepath.Join(dir, "processo.zip"), OutputPath: saida},
		{Status: "success", Kind: "zip", Path: filepath.Join(dir, "processo.zip"), OutputPath: naoTxt},
		{Status: "error", Kind: "zip", Path: filepath.Join(dir, "segredo.zip"), OutputPath: outro},
	}

	texto, err := lerSaidaParaCopiar(strings.ToUpper(saida[:1])+saida[1:], itens)
	if err != nil {
		t.Fatalf("saída válida recusada: %v", err)
	}
	if texto != "<contexto>conteúdo</contexto>" {
		t.Fatalf("texto lido: %q", texto)
	}

	for nome, caminho := range map[string]string{
		"fora da fila":      filepath.Join(dir, "nao-listado.txt"),
		"item com erro":     outro,
		"extensão não .txt": naoTxt,
		"vazio":             "",
	} {
		if _, err := lerSaidaParaCopiar(caminho, itens); err == nil {
			t.Errorf("%s: aceitou %q", nome, caminho)
		}
	}
}

func TestCopyOutput_EnviaTextoParaAreaDeTransferencia(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	var copiado string
	app.clipboard = func(_ context.Context, texto string) error {
		copiado = texto
		return nil
	}

	if err := app.CopyOutput(filepath.Join(t.TempDir(), "x.txt")); err == nil {
		t.Fatal("CopyOutput sem fila aceitou caminho arbitrário")
	}
	if copiado != "" {
		t.Fatalf("copiou sem validar: %q", copiado)
	}
}

func TestMensagemFim(t *testing.T) {
	titulo, corpo := mensagemFim(&etl.QueueItem{
		Name: "5001234-56.2026.zip", Status: "success",
		OutputPath: `C:\Downloads\5001234-56.2026.txt`, DocsCount: 4, PagesCount: 37,
	})
	if titulo != "eprocIA" || corpo != "5001234-56.2026.txt pronto (4 docs, 37 págs)" {
		t.Fatalf("sucesso: %q / %q", titulo, corpo)
	}

	_, corpo = mensagemFim(&etl.QueueItem{Name: "ruim.zip", Status: "error", Error: "ZIP corrompido"})
	if corpo != "Erro ao processar ruim.zip: ZIP corrompido" {
		t.Fatalf("erro: %q", corpo)
	}
}

func TestAoTerminar_NotificaSoQuandoDisponivel(t *testing.T) {
	app := NewApp()
	item := &etl.QueueItem{Name: "a.zip", Status: "success", OutputPath: `C:\a.txt`, DocsCount: 1, PagesCount: 2}

	app.aoTerminar(item) // sem notificador: não pode entrar em pânico

	var recebidas []string
	app.notificar = func(titulo, corpo string) error {
		recebidas = append(recebidas, titulo+"|"+corpo)
		return nil
	}
	app.aoTerminar(item)
	if len(recebidas) != 1 || recebidas[0] != "eprocIA|a.txt pronto (1 docs, 2 págs)" {
		t.Fatalf("notificações: %v", recebidas)
	}
}

func TestMensagemFim_NaoLidos(t *testing.T) {
	item := &etl.QueueItem{Name: "saida.zip", Status: "success", OutputPath: "saida.txt", DocsCount: 10, PagesCount: 40}
	casos := []struct {
		naoLidos int
		esperado string
	}{
		{2, "saida.txt pronto, com 2 documentos não lidos (10 docs, 40 págs)"},
		{1, "saida.txt pronto, com 1 documento não lido (10 docs, 40 págs)"},
		{0, "saida.txt pronto (10 docs, 40 págs)"},
	}
	for _, c := range casos {
		item.UnreadDocs = c.naoLidos
		if _, corpo := mensagemFim(item); corpo != c.esperado {
			t.Errorf("não lidos %d: %q, esperava %q", c.naoLidos, corpo, c.esperado)
		}
	}
}

func escreverTxt(t *testing.T, caminho, texto string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLerSaidaParaCopiar_SaidaDoItem(t *testing.T) {
	dir := t.TempDir()
	cnj := "5000000-00.2026.8.26.0001"
	lote := filepath.Join(dir, cnj+".txt")
	escreverTxt(t, lote, "lote")
	umPDF := filepath.Join(dir, "processo_2.txt")
	escreverTxt(t, umPDF, "um pdf")
	b := filepath.Join(dir, "b.txt")
	escreverTxt(t, b, "alheio")
	semKind := filepath.Join(dir, "sem-kind.txt")
	escreverTxt(t, semKind, "sem kind")

	pdfs := []string{
		filepath.Join(dir, cnj+" Evento 1 PET1.pdf"),
		filepath.Join(dir, cnj+" Evento 2 DESPADEC1.pdf"),
		filepath.Join(dir, cnj+" Evento 3 SENT1.pdf"),
	}
	itens := []*etl.QueueItem{
		{Status: "success", Kind: "pdf", Path: pdfs[0], Paths: pdfs, OutputPath: lote},
		{Status: "success", Kind: "pdf", Path: filepath.Join(dir, "processo.pdf"), Paths: []string{filepath.Join(dir, "processo.pdf")}, OutputPath: umPDF},
		{Status: "success", Kind: "zip", Path: filepath.Join(dir, "a.zip"), OutputPath: b},
		{Status: "success", Path: filepath.Join(dir, "sem-kind.zip"), OutputPath: semKind},
	}

	for caminho, esperado := range map[string]string{lote: "lote", umPDF: "um pdf"} {
		texto, err := lerSaidaParaCopiar(caminho, itens)
		if err != nil || texto != esperado {
			t.Errorf("%s: texto %q, erro %v", filepath.Base(caminho), texto, err)
		}
	}
	for nome, caminho := range map[string]string{"zip com saída alheia": b, "item sem Kind": semKind} {
		if _, err := lerSaidaParaCopiar(caminho, itens); err == nil {
			t.Errorf("%s: aceitou %q", nome, caminho)
		}
	}
}

func TestLerSaidaParaCopiar_ErroDeLeituraSemCaminho(t *testing.T) {
	dir := t.TempDir()
	saida := filepath.Join(dir, "processo.txt") // não existe: a leitura falha
	itens := []*etl.QueueItem{{Status: "success", Kind: "zip", Path: filepath.Join(dir, "processo.zip"), OutputPath: saida}}

	_, err := lerSaidaParaCopiar(saida, itens)
	if err == nil {
		t.Fatal("leitura de arquivo inexistente não falhou")
	}
	if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "processo.txt") {
		t.Fatalf("mensagem leva o caminho: %q", err.Error())
	}
	if err.Error() != "não foi possível ler o TXT gerado" {
		t.Fatalf("mensagem: %q", err.Error())
	}
}

func TestCopiarSaida_ErroDaAreaDeTransferenciaSemDetalhe(t *testing.T) {
	dir := t.TempDir()
	saida := filepath.Join(dir, "processo.txt")
	escreverTxt(t, saida, "texto")
	itens := []*etl.QueueItem{{Status: "success", Kind: "zip", Path: filepath.Join(dir, "processo.zip"), OutputPath: saida}}

	app := NewApp()
	app.ctx = context.Background()
	app.clipboard = func(context.Context, string) error {
		return errors.New("OpenClipboard falhou em " + dir)
	}
	err := app.copiarSaida(saida, itens)
	if err == nil {
		t.Fatal("falha da área de transferência não chegou")
	}
	if err.Error() != "não foi possível copiar para a área de transferência" {
		t.Fatalf("mensagem: %q", err.Error())
	}
}
