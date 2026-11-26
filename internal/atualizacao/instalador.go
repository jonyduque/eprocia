package atualizacao

import (
	"errors"
	"log/slog"
	"os"
	"sync"
)

// ErrCancelado: o usuário recusou o pedido de elevação (UAC) do instalador.
var ErrCancelado = errors.New("instalação cancelada pelo usuário")

// NovaPastaInstalador cria pastaAtualizacao, se faltar, e devolve uma
// subpasta nova e de nome aleatório para um download do instalador. O
// instalador guarda nela o nome do asset: o shell precisa da extensão .exe.
func NovaPastaInstalador(pastaAtualizacao string) (string, error) {
	if err := os.MkdirAll(pastaAtualizacao, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(pastaAtualizacao, "")
}

// Travado é o instalador baixado, conferido e mantido aberto sem
// compartilhamento de escrita nem de remoção até o shell abri-lo: ninguém
// troca o arquivo entre a conferência e a abertura.
type Travado struct {
	f       *os.File
	caminho string
	fechar  sync.Once
}

// Abrir chama shell com o caminho do instalador e fecha o handle depois que
// shell devolve, com erro ou não.
func (t *Travado) Abrir(shell func(caminho string) error) error {
	defer t.Fechar()
	err := shell(t.caminho)
	resultado := "aberto"
	switch {
	case errors.Is(err, ErrCancelado):
		resultado = "cancelado"
	case err != nil:
		resultado = "erro"
	}
	// Texto fixo: o erro do shell pode carregar o caminho do perfil.
	slog.Default().Info("abertura do instalador", "resultado", resultado)
	return err
}

// Fechar solta o arquivo; pode ser chamado mais de uma vez.
func (t *Travado) Fechar() {
	t.fechar.Do(func() {
		if t.f != nil {
			_ = t.f.Close()
		}
	})
}
