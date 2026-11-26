package ocr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// ErrPrazo embrulha o erro de um processo externo (pdftoppm, tesseract...) que
// passou do prazo e foi encerrado. Cancelamento por quem chamou não é prazo.
var ErrPrazo = errors.New("prazo do processo externo esgotado")

// ErroProcesso é a falha de um processo externo: o nome do executável, o código
// de saída (-1 se não houve), o começo do stderr (até maxStderr bytes) e o erro
// do exec. Error() não leva o stderr, que pode trazer caminhos do usuário: quem
// quiser registrá-lo o lê do campo, por errors.As.
type ErroProcesso struct {
	Programa string
	Codigo   int
	Stderr   string
	Err      error
}

func (e *ErroProcesso) Error() string {
	return fmt.Sprintf("%s saiu com código %d: %v", e.Programa, e.Codigo, e.Err)
}

func (e *ErroProcesso) Unwrap() error { return e.Err }

// Prazo de cada chamada externa: base + porPagina × páginas da chamada. São
// variáveis para os testes encurtarem.
var (
	prazoRenderBase, prazoRenderPorPagina = 30 * time.Second, 10 * time.Second
	prazoOCRBase, prazoOCRPorPagina       = 60 * time.Second, 60 * time.Second
	prazoInfo                             = 20 * time.Second
	prazoTextoBase, prazoTextoPorPagina   = 30 * time.Second, 2 * time.Second
)

// esperaDepoisDeMatar limita quanto o Wait espera pelas saídas depois de o
// processo ser morto (um neto que herdou o pipe o seguraria aberto).
const esperaDepoisDeMatar = 2 * time.Second

// prazoDe devolve base + porPagina × paginas.
func prazoDe(base, porPagina time.Duration, paginas int) time.Duration {
	return base + porPagina*time.Duration(max(paginas, 0))
}

// rodarComPrazo roda o comando que novo monta sobre um contexto filho de ctx
// com o prazo dado. A falha vem embrulhada em *ErroProcesso, com o stderr se o
// comando não o desviou (cmd.Stderr nil). Se o filho expirou e ctx não, o erro
// embrulha também ErrPrazo; se ctx acabou, o erro de ctx.
func rodarComPrazo(ctx context.Context, prazo time.Duration, novo func(ctx context.Context) *exec.Cmd) error {
	filho, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()
	cmd := novo(filho)
	cmd.WaitDelay = esperaDepoisDeMatar
	var erroPadrao *limitado
	if cmd.Stderr == nil {
		erroPadrao = &limitado{max: maxStderr}
		cmd.Stderr = erroPadrao
	}
	err := cmd.Run()
	if err == nil {
		return nil
	}
	ep := &ErroProcesso{Programa: filepath.Base(cmd.Path), Codigo: -1, Err: err}
	if erroSaida, ok := errors.AsType[*exec.ExitError](err); ok {
		ep.Codigo = erroSaida.ExitCode()
	}
	if erroPadrao != nil {
		ep.Stderr = erroPadrao.String()
	}
	if errPai := ctx.Err(); errPai != nil {
		if errors.Is(err, errPai) {
			return ep
		}
		return fmt.Errorf("%w: %w", errPai, ep)
	}
	if errors.Is(filho.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w (%s): %w", ErrPrazo, prazo, ep)
	}
	return ep
}
