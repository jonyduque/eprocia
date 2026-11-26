package atualizacao

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNaoConferiu: o arquivo baixado não bate com a release (tamanho, SHA-256,
// cabeçalho MZ ou versão do PE).
var ErrNaoConferiu = errors.New("o download não conferiu")

// conferir é a única conferência do arquivo baixado: recalcula o SHA-256
// lendo f do início e confere tamanho, cabeçalho MZ e a versão do PE de
// b.Caminho. A atestação de build, quando vier, entra aqui.
func conferir(b Baixado, f *os.File) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("%w: voltar ao início: %v", ErrNaoConferiu, err)
	}
	h := sha256.New()
	var inicio bytes.Buffer
	// Lê no máximo um byte além do tamanho esperado: o bastante para notar
	// um arquivo maior sem ler um arquivo enorme inteiro.
	leitor := io.TeeReader(io.LimitReader(f, b.Asset.Tamanho+1), h)
	if _, err := io.CopyN(&inicio, leitor, 2); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: ler: %v", ErrNaoConferiu, err)
	}
	resto, err := io.Copy(io.Discard, leitor)
	if err != nil {
		return fmt.Errorf("%w: ler: %v", ErrNaoConferiu, err)
	}
	if lido := int64(inicio.Len()) + resto; lido != b.Asset.Tamanho {
		return fmt.Errorf("%w: tamanho difere do asset", ErrNaoConferiu)
	}
	if [32]byte(h.Sum(nil)) != b.SHA256 {
		return fmt.Errorf("%w: SHA-256 difere do SHA256SUMS.txt", ErrNaoConferiu)
	}
	if !bytes.Equal(inicio.Bytes(), []byte("MZ")) {
		return fmt.Errorf("%w: não é um executável", ErrNaoConferiu)
	}
	v, err := VersaoPE(b.Caminho)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNaoConferiu, err)
	}
	if v != b.Versao {
		return fmt.Errorf("%w: versão do executável (%s) difere da release (%s)", ErrNaoConferiu, v, b.Versao)
	}
	return nil
}
