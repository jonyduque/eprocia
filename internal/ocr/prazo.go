package ocr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrPrazo embrulha o erro de um processo externo (pdftoppm, tesseract...) que
// passou do prazo e foi encerrado. Cancelamento por quem chamou não é prazo.
var ErrPrazo = errors.New("prazo do processo externo esgotado")

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
// com o prazo dado. Se o filho expirou e ctx não, o erro embrulha ErrPrazo; se
// ctx acabou, o erro embrulha o erro de ctx.
func rodarComPrazo(ctx context.Context, prazo time.Duration, novo func(ctx context.Context) *exec.Cmd) error {
	filho, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()
	cmd := novo(filho)
	cmd.WaitDelay = esperaDepoisDeMatar
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if errPai := ctx.Err(); errPai != nil {
		if errors.Is(err, errPai) {
			return err
		}
		return fmt.Errorf("%w: %v", errPai, err)
	}
	if errors.Is(filho.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w (%s): %v", ErrPrazo, prazo, err)
	}
	return err
}
