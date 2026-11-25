package atualizacao

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"eprocia/internal/registro"
)

const (
	// LimiteAsset é o maior asset aceito (no Content-Length e na contagem).
	LimiteAsset = 300 << 20
	// PrazoSemDados é quanto o download pode ficar sem receber bytes.
	PrazoSemDados = 60 * time.Second
)

// ErrParado: o download ficou PrazoSemDados sem receber bytes.
var ErrParado = errors.New("o download parou de receber dados")

// Baixado é o asset gravado e conferido. SHA256 é o do SHA256SUMS.txt (igual
// ao recalculado sobre o arquivo, depois de conferir).
type Baixado struct {
	Caminho string
	Asset   Asset
	Versao  Versao
	SHA256  [32]byte
}

// Baixar baixa asset da release rel para pasta\nome (criado com O_EXCL),
// depois de achar o hash dele no SHA256SUMS.txt da mesma release, e só
// devolve o arquivo fechado e conferido (conferir). Qualquer erro,
// cancelamento incluído, apaga o arquivo. progresso, se não nil, recebe os
// bytes gravados e o tamanho do asset a cada bloco.
func (c *Cliente) Baixar(ctx context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(feito, total int64)) (Baixado, error) {
	b, err := c.baixar(ctx, rel, asset, pasta, nome, progresso)
	resultado := "conferido"
	switch {
	case errors.Is(err, ErrNaoConferiu):
		resultado = "nao_conferiu"
	case errors.Is(err, ErrParado):
		resultado = "parado"
	case errors.Is(err, context.Canceled):
		resultado = "cancelado"
	case err != nil:
		resultado = "erro"
	}
	registro.Logger(ctx).Info("download de atualização", "resultado", resultado)
	return b, err
}

func (c *Cliente) baixar(ctx context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(feito, total int64)) (Baixado, error) {
	if rel == nil {
		return Baixado{}, errors.New("baixar sem release")
	}
	if asset.URL != c.regras.PrefixoDownload+rel.Tag+"/"+asset.Nome {
		return Baixado{}, fmt.Errorf("%w: endereço do asset fora do esperado", ErrURLRecusada)
	}
	if rel.Somas.Nome != nomeSomas || rel.Somas.URL != c.regras.PrefixoDownload+rel.Tag+"/"+nomeSomas {
		return Baixado{}, fmt.Errorf("%w: endereço do SHA256SUMS.txt fora do esperado", ErrURLRecusada)
	}
	limite := c.limiteAsset
	if limite <= 0 {
		limite = LimiteAsset
	}
	if asset.Tamanho <= 0 || asset.Tamanho > limite {
		return Baixado{}, errors.New("tamanho do asset fora do teto")
	}

	soma, err := c.lerSomas(ctx, rel.Somas.URL, asset.Nome)
	if err != nil {
		return Baixado{}, err
	}

	caminho := filepath.Join(pasta, nome)
	f, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return Baixado{}, fmt.Errorf("criar arquivo do download: %w", err)
	}
	pronto := false
	defer func() {
		if !pronto {
			_ = os.Remove(caminho)
		}
	}()

	errCopia := c.copiar(ctx, f, asset, limite, progresso)
	errFechar := f.Close()
	if errCopia != nil {
		return Baixado{}, errCopia
	}
	if errFechar != nil {
		return Baixado{}, fmt.Errorf("fechar arquivo do download: %w", errFechar)
	}
	if c.depoisDeCopiar != nil {
		c.depoisDeCopiar(caminho)
	}

	b := Baixado{Caminho: caminho, Asset: asset, Versao: rel.Versao, SHA256: soma}
	leitura, err := os.Open(caminho)
	if err != nil {
		return Baixado{}, fmt.Errorf("%w: reabrir: %v", ErrNaoConferiu, err)
	}
	err = conferir(b, leitura)
	_ = leitura.Close()
	if err != nil {
		return Baixado{}, err
	}
	pronto = true
	return b, nil
}

func (c *Cliente) prazoParado() time.Duration {
	if c.prazoSemDados > 0 {
		return c.prazoSemDados
	}
	return PrazoSemDados
}

// lerSomas baixa o SHA256SUMS.txt (teto LimiteSomas, prazo total
// PrazoSemDados) e acha a linha de nome.
func (c *Cliente) lerSomas(ctx context.Context, endereco, nome string) ([32]byte, error) {
	ctxSomas, cancelar := context.WithTimeout(ctx, c.prazoParado())
	defer cancelar()
	traduzir := func(err error) error {
		if ctx.Err() != nil {
			return fmt.Errorf("baixar SHA256SUMS.txt: %w", ctx.Err())
		}
		if ctxSomas.Err() != nil {
			return fmt.Errorf("%w (SHA256SUMS.txt)", ErrParado)
		}
		return fmt.Errorf("baixar SHA256SUMS.txt: %w", err)
	}
	resp, err := c.obter(ctxSomas, endereco, urlDownload, "")
	if err != nil {
		return [32]byte{}, traduzir(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return [32]byte{}, fmt.Errorf("baixar SHA256SUMS.txt: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > LimiteSomas {
		return [32]byte{}, errors.New("SHA256SUMS.txt acima do teto")
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, LimiteSomas+1))
	if err != nil {
		return [32]byte{}, traduzir(err)
	}
	if len(corpo) > LimiteSomas {
		return [32]byte{}, errors.New("SHA256SUMS.txt acima do teto")
	}
	soma, err := linhaSoma(corpo, nome)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrNaoConferiu, err)
	}
	return soma, nil
}

// copiar grava o corpo do asset em f. Um temporizador renovado a cada bloco
// recebido cancela a requisição quando passa PrazoSemDados sem bytes.
func (c *Cliente) copiar(ctx context.Context, f *os.File, asset Asset, limite int64, progresso func(feito, total int64)) error {
	ctxReq, cancelar := context.WithCancel(ctx)
	defer cancelar()
	prazo := c.prazoParado()
	var parado atomic.Bool
	vigia := time.AfterFunc(prazo, func() {
		parado.Store(true)
		cancelar()
	})
	defer vigia.Stop()

	traduzir := func(err error) error {
		if parado.Load() {
			return ErrParado
		}
		if ctx.Err() != nil {
			return fmt.Errorf("download interrompido: %w", ctx.Err())
		}
		return fmt.Errorf("download interrompido: %w", err)
	}

	resp, err := c.obter(ctxReq, asset.URL, urlDownload, "application/octet-stream")
	if err != nil {
		return traduzir(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("baixar asset: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limite {
		return errors.New("Content-Length acima do teto")
	}
	if resp.ContentLength >= 0 && resp.ContentLength != asset.Tamanho {
		return errors.New("Content-Length difere do tamanho do asset")
	}

	buf := make([]byte, 32<<10)
	var feito int64
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("download interrompido: %w", err)
		}
		n, errLer := resp.Body.Read(buf)
		if n > 0 {
			vigia.Reset(prazo)
			feito += int64(n)
			if feito > asset.Tamanho || feito > limite {
				return errors.New("corpo maior que o tamanho do asset")
			}
			if _, err := f.Write(buf[:n]); err != nil {
				return fmt.Errorf("gravar download: %w", err)
			}
			if progresso != nil {
				progresso(feito, asset.Tamanho)
			}
		}
		if errors.Is(errLer, io.EOF) {
			break
		}
		if errLer != nil {
			return traduzir(errLer)
		}
	}
	if feito != asset.Tamanho {
		return errors.New("corpo menor que o tamanho do asset")
	}
	return nil
}
