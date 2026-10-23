package main

import (
	"context"
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
		{Status: "success", OutputPath: saida},
		{Status: "success", OutputPath: naoTxt},
		{Status: "error", OutputPath: outro},
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
