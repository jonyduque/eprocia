package etl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"eprocia/internal/models"
)

// esperarExecucao lê uma execução do canal ou falha; um callback chamado com
// q.mu travado faria GetItems travar e o teste estourar o prazo aqui.
func esperarExecucao(t *testing.T, ch <-chan models.ExecucaoZip) models.ExecucaoZip {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(10 * time.Second):
		t.Fatal("o callback de fim de ZIP não foi chamado (ou chamado sob a trava)")
	}
	return models.ExecucaoZip{}
}

func semMaisExecucoes(t *testing.T, ch <-chan models.ExecucaoZip) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("callback chamado de novo: %+v", e)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestQueue_AoTerminarZip(t *testing.T) {
	dir := t.TempDir()
	zipOK := filepath.Join(dir, "ok.zip")
	if err := os.WriteFile(zipOK, make([]byte, 321), 0o600); err != nil {
		t.Fatal(err)
	}
	modo := map[string]string{} // nome -> "ok" ou "erro"
	q, _ := novaFilaGravada(t, func(_ context.Context, zipPath string, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		if modo[filepath.Base(zipPath)] == "erro" {
			return "", 0, 0, 0, errors.New("falha fictícia")
		}
		return "saida.txt", 1, 1, 1, nil
	})
	q.cfg = &models.Config{OcrMode: "normal", CleaningLevel: "standard", ForcarOCR: true, MonitoredPath: `C:\Users\fulano\Processos`}

	ch := make(chan models.ExecucaoZip, 8)
	q.SetAoTerminarZip(func(e models.ExecucaoZip) {
		q.GetItems() // trava se o callback vier sob q.mu
		ch <- e
	})

	// success, com origem arrastada e tamanho do ZIP.
	modo["ok.zip"] = "ok"
	if n := q.AddFilesComOrigem([]string{zipOK}, "arrastada"); n != 1 {
		t.Fatalf("AddFilesComOrigem = %d", n)
	}
	e := esperarExecucao(t, ch)
	semMaisExecucoes(t, ch)
	if e.Status != "success" || e.Origem != "arrastada" || e.Reprocesso || e.BytesZip != 321 || e.ClasseErro != "" {
		t.Errorf("success: %+v", e)
	}
	ok := itemPorNome(t, q, "ok.zip")
	if e.ItemID != ok.ID {
		t.Errorf("ItemID = %q, quer %q", e.ItemID, ok.ID)
	}
	if e.Config.OcrMode != "normal" || e.Config.CleaningLevel != "standard" || !e.Config.ForcarOCR {
		t.Errorf("config efetiva = %+v", e.Config)
	}

	// Reprocessado: Reprocesso verdadeiro, origem mantida, config do ajuste.
	if err := q.Reprocessar(ok.ID, "none", "basic", nil); err != nil {
		t.Fatal(err)
	}
	e = esperarExecucao(t, ch)
	semMaisExecucoes(t, ch)
	if !e.Reprocesso || e.Origem != "arrastada" || e.Config.OcrMode != "none" || e.Config.CleaningLevel != "basic" || e.Config.ForcarOCR {
		t.Errorf("reprocesso: %+v", e)
	}

	// error, com classe; AddFiles é origem manual.
	modo["ruim.zip"] = "erro"
	q.AddFiles([]string{filepath.Join(dir, "ruim.zip")})
	e = esperarExecucao(t, ch)
	semMaisExecucoes(t, ch)
	if e.Status != "error" || e.ClasseErro == "" || e.Origem != "manual" || e.Relatorio != nil {
		t.Errorf("error: %+v", e)
	}

	// cancelled: o processar falso cancela o próprio item.
	q.processar = func(ctx context.Context, zipPath string, _ *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		q.mu.Lock()
		for _, it := range q.items {
			if it.Path == zipPath && it.cancel != nil {
				it.cancel()
			}
		}
		q.mu.Unlock()
		<-ctx.Done()
		return "", 0, 0, 0, ctx.Err()
	}
	q.AddFilesComOrigem([]string{filepath.Join(dir, "cancela.zip")}, "monitorada")
	e = esperarExecucao(t, ch)
	semMaisExecucoes(t, ch)
	if e.Status != "cancelled" || e.Origem != "monitorada" || e.ClasseErro != "" {
		t.Errorf("cancelled: %+v", e)
	}

	// Erro ao carregar configurações (retorno antecipado).
	q.mu.Lock()
	q.cfg = nil
	q.mu.Unlock()
	q.carregarConfig = func() (*models.Config, error) { return nil, errors.New("config corrompida") }
	q.AddFiles([]string{filepath.Join(dir, "semcfg.zip")})
	e = esperarExecucao(t, ch)
	semMaisExecucoes(t, ch)
	if e.Status != "error" || e.ClasseErro == "" || e.Origem != "manual" {
		t.Errorf("erro de config: %+v", e)
	}
}
