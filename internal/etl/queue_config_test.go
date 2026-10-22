package etl

import (
	"context"
	"errors"
	"sync"
	"testing"

	"eprocia/internal/models"
)

// Sem config no startup, cada item tenta carregá-la; se falhar, o item vira
// erro e a fila segue para o próximo em vez de travar.
func TestFila_SemConfigEComCarregamentoFalhandoMarcaErroESegue(t *testing.T) {
	var chamadas int
	var mu sync.Mutex
	q, _ := novaFilaGravada(t, func(context.Context, string, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, error) {
		mu.Lock()
		chamadas++
		mu.Unlock()
		return "x.txt", 1, 1, nil
	})
	q.cfg = nil
	errCfg := errors.New("invalid character 'o' looking for beginning of object key string")
	q.carregarConfig = func() (*models.Config, error) { return nil, errCfg }

	q.AddFiles([]string{"a.zip", "b.zip"})
	esperarFim(t, q, "a.zip")
	esperarFim(t, q, "b.zip")
	esperarOcioso(t, q)

	for _, nome := range []string{"a.zip", "b.zip"} {
		it := itemPorNome(t, q, nome)
		if it.Status != "error" || it.Message != "Erro ao carregar configurações" || it.Error != errCfg.Error() {
			t.Fatalf("item %s: status %q, mensagem %q, erro %q", nome, it.Status, it.Message, it.Error)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if chamadas != 0 {
		t.Fatalf("pipeline chamado %d vezes sem configuração", chamadas)
	}
}

func TestFila_SemConfigUsaAQueFoiCarregada(t *testing.T) {
	carregada := &models.Config{OcrMode: "fast", CleaningLevel: "aggressive"}
	recebida := make(chan *models.Config, 1)
	q, _ := novaFilaGravada(t, func(_ context.Context, _ string, cfg *models.Config, _ func(float64, string, int, int, int, int)) (string, int, int, error) {
		recebida <- cfg
		return "x.txt", 1, 1, nil
	})
	q.cfg = nil
	q.carregarConfig = func() (*models.Config, error) { return carregada, nil }

	q.AddFiles([]string{"a.zip"})
	esperarFim(t, q, "a.zip")

	if it := itemPorNome(t, q, "a.zip"); it.Status != "success" {
		t.Fatalf("status %q, erro %q", it.Status, it.Error)
	}
	if cfg := <-recebida; cfg != carregada {
		t.Fatalf("pipeline recebeu %+v, esperado a config carregada %+v", cfg, carregada)
	}
}
