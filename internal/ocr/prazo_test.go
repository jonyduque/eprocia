package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eprocia/internal/registro"
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
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
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
	_, err := f.reconhecer(context.Background(), t.TempDir(), "p-1.pgm", 1, nil)
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

// comandoDeTeste monta o próprio binário de teste como processo externo.
func comandoDeTeste(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0])
	prepareCmd(cmd)
	return cmd
}

func TestRodarComPrazo_StderrNoErro(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
	t.Setenv("EPROCIA_TESTE_STDERR", "1")

	err := rodarComPrazo(context.Background(), time.Minute, comandoDeTeste)
	var ep *ErroProcesso
	if !errors.As(err, &ep) {
		t.Fatalf("erro = %v (%T), quero *ErroProcesso na cadeia", err, err)
	}
	if ep.Codigo != 3 {
		t.Errorf("Codigo = %d, quero 3", ep.Codigo)
	}
	if !strings.Contains(ep.Stderr, "erro de teste") {
		t.Errorf("Stderr = %q, quero o stderr do processo", ep.Stderr)
	}
	if want := filepath.Base(os.Args[0]); ep.Programa != want {
		t.Errorf("Programa = %q, quero %q", ep.Programa, want)
	}
	if strings.Contains(err.Error(), "erro de teste") {
		t.Errorf("a mensagem leva o stderr: %v", err)
	}
	if !strings.Contains(err.Error(), "código 3") {
		t.Errorf("a mensagem não traz o código de saída: %v", err)
	}
}

func TestRodarComPrazo_PrazoContinuaErrPrazo(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
	t.Setenv("EPROCIA_TESTE_DORMIR", "1")

	inicio := time.Now()
	err := rodarComPrazo(context.Background(), 200*time.Millisecond, comandoDeTeste)
	if d := time.Since(inicio); d > 5*time.Second {
		t.Errorf("voltou em %v, quero menos de 5 s", d)
	}
	if !errors.Is(err, ErrPrazo) {
		t.Errorf("erro = %v, quero ErrPrazo", err)
	}
	if _, ok := errors.AsType[*ErroProcesso](err); !ok {
		t.Errorf("erro = %v, quero *ErroProcesso na cadeia", err)
	}
}

// O erro que reconhecer devolve leva o código de saída no texto e o stderr só
// no campo do *ErroProcesso (o log o extrai de lá; no texto sairia duplicado).
func TestReconhecer_StderrSoNoErroProcesso(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
	t.Setenv("EPROCIA_TESTE_STDERR", "1")
	dir := t.TempDir()
	f := ferramentas{tess: os.Args[0], pdftoppm: os.Args[0], tessDir: dir, popplerDir: dir, tessdata: dir}

	_, err := f.reconhecer(context.Background(), dir, "p-1.pgm", 1, nil)
	var ep *ErroProcesso
	if !errors.As(err, &ep) || ep.Codigo != 3 {
		t.Fatalf("erro = %v, quero *ErroProcesso com código 3", err)
	}
	if !strings.Contains(err.Error(), "código 3") {
		t.Errorf("erro = %v, quero o código de saída no texto", err)
	}
	if strings.Contains(err.Error(), "erro de teste") {
		t.Errorf("erro = %v, o texto não pode levar o stderr", err)
	}
	if !strings.Contains(ep.Stderr, "erro de teste") {
		t.Errorf("Stderr = %q, quero o stderr do tesseract", ep.Stderr)
	}
}

func TestRodarComPrazo_StderrCortado(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
	t.Setenv("EPROCIA_TESTE_STDERR_GRANDE", "1")

	err := rodarComPrazo(context.Background(), time.Minute, comandoDeTeste)
	ep, ok := errors.AsType[*ErroProcesso](err)
	if !ok {
		t.Fatalf("erro = %v, quero *ErroProcesso na cadeia", err)
	}
	if ep.Stderr == "" || len(ep.Stderr) > maxStderr {
		t.Errorf("len(Stderr) = %d, quero entre 1 e %d", len(ep.Stderr), maxStderr)
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

// O fallback do modelo "por" é o único lugar em que o erro do primeiro
// tesseract aparece: a linha Warn leva código de saída e stderr.
func TestReconhecer_FallbackLevaStderrNoWarn(t *testing.T) {
	t.Setenv("EPROCIA_TESTE_FILHO", "1")
	t.Setenv("EPROCIA_TESTE_STDERR", "1")
	dir := t.TempDir()
	f := ferramentas{tess: os.Args[0], pdftoppm: os.Args[0], tessDir: dir, popplerDir: dir, tessdata: dir}
	var buf bytes.Buffer
	ctx := registro.ComLogger(context.Background(),
		slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if _, err := f.reconhecer(ctx, dir, "p-1.pgm", 1, nil); err == nil {
		t.Fatal("esperava erro")
	}
	achou := false
	for l := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("linha não é JSON: %q", l)
		}
		if r["level"] != "WARN" {
			continue
		}
		achou = true
		if r["codigo_saida"] != 3.0 {
			t.Errorf("codigo_saida = %v, quero 3", r["codigo_saida"])
		}
		if s, _ := r["stderr"].(string); !strings.Contains(s, "erro de teste") {
			t.Errorf("stderr = %q, quero o stderr do processo", s)
		}
	}
	if !achou {
		t.Fatalf("sem linha Warn do fallback:\n%s", buf.String())
	}
}
