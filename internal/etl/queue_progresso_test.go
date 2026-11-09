package etl

import (
	"context"
	"testing"

	"eprocia/internal/models"
)

// Documentos em paralelo entregam o progresso fora de ordem; a barra não
// pode voltar.
func TestProgresso_SoAvanca(t *testing.T) {
	liberar := make(chan struct{})
	lido := make(chan struct{})
	q, _ := novaFilaGravada(t, func(_ context.Context, _ string, _ *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		onProgress(0.6, "doc B", 2, 4, 6, 10)
		onProgress(0.5, "doc A", 1, 4, 5, 10) // chegou atrasado
		lido <- struct{}{}
		<-liberar
		return "saida.txt", 4, 10, 0, nil
	})
	q.AddFiles([]string{`C:\x\a.zip`})
	<-lido
	it := q.GetItems()[0]
	if it.Progress != 0.6 || it.ProcessedDocs != 2 || it.ProcessedPages != 6 {
		t.Errorf("progresso voltou: %.2f docs %d págs %d", it.Progress, it.ProcessedDocs, it.ProcessedPages)
	}
	if it.Message != "doc A" {
		t.Errorf("mensagem = %q, esperava a mais recente", it.Message)
	}
	close(liberar)
	esperarFim(t, q, "a.zip")
}
