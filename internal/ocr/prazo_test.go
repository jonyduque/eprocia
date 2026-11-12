package ocr

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// encurtar troca *v por d até o fim do teste. Os testes que usam isto não podem
// rodar com t.Parallel: as variáveis de prazo são do pacote.
func encurtar(t *testing.T, v *time.Duration, d time.Duration) {
	t.Helper()
	antigo := *v
	*v = d
	t.Cleanup(func() { *v = antigo })
}

// presos devolve ferramentas cujos pdftoppm e tesseract são o próprio binário
// de teste, que dorme 1 minuto (ver TestMain).
func presos(t *testing.T) ferramentas {
	t.Helper()
	t.Setenv("EPROCIA_TESTE_DORMIR", "1")
	dir := t.TempDir()
	return ferramentas{tess: os.Args[0], pdftoppm: os.Args[0], tessDir: dir, popplerDir: dir, tessdata: dir}
}

func TestRenderizar_EstouraOPrazo(t *testing.T) {
	encurtar(t, &prazoRenderBase, 200*time.Millisecond)
	encurtar(t, &prazoRenderPorPagina, 0)
	f := presos(t)

	inicio := time.Now()
	err := f.renderizar(context.Background(), "doc.pdf", t.TempDir(), 1, 1)
	if d := time.Since(inicio); d > 5*time.Second {
		t.Errorf("renderizar voltou em %v, quero menos de 5 s", d)
	}
	if !errors.Is(err, ErrPrazo) {
		t.Fatalf("erro = %v, quero ErrPrazo", err)
	}
}

func TestReconhecer_EstouraOPrazo(t *testing.T) {
	encurtar(t, &prazoOCRBase, 200*time.Millisecond)
	encurtar(t, &prazoOCRPorPagina, 0)
	f := presos(t)

	inicio := time.Now()
	_, err := f.reconhecer(context.Background(), t.TempDir(), "p-1.png", 1, nil)
	if d := time.Since(inicio); d > 5*time.Second {
		t.Errorf("reconhecer voltou em %v, quero menos de 5 s (o fallback não deve rodar depois do prazo)", d)
	}
	if !errors.Is(err, ErrPrazo) {
		t.Fatalf("erro = %v, quero ErrPrazo", err)
	}
}

func TestRenderizar_CancelamentoNaoEPrazo(t *testing.T) {
	f := presos(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)

	inicio := time.Now()
	err := f.renderizar(ctx, "doc.pdf", t.TempDir(), 1, 1)
	if d := time.Since(inicio); d > 5*time.Second {
		t.Errorf("renderizar voltou em %v depois do cancelamento", d)
	}
	if err == nil || errors.Is(err, ErrPrazo) {
		t.Fatalf("erro = %v, quero um erro que não seja ErrPrazo", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("erro = %v, quero o erro do contexto pai (context.Canceled)", err)
	}
}

func TestPrazoDe_SomaPorPagina(t *testing.T) {
	if got := prazoDe(30*time.Second, 10*time.Second, 3); got != time.Minute {
		t.Errorf("prazoDe = %v, quero 1m0s", got)
	}
	if got := prazoDe(30*time.Second, 10*time.Second, 0); got != 30*time.Second {
		t.Errorf("prazoDe sem páginas = %v, quero a base", got)
	}
}
