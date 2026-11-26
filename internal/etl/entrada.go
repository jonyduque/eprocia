package etl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"eprocia/internal/models"
)

// ErrTipoNaoSuportado é o erro de uma entrada que não é um ZIP só nem uma
// lista só de PDFs.
var ErrTipoNaoSuportado = errors.New("tipo de arquivo não suportado")

const (
	fraseArquivoNaoEncontrado = "arquivo não encontrado"
	fraseArquivoVazio         = "arquivo vazio"
)

// erroEntrada é a falha de um PDF avulso em LerEntrada: Error() é a frase
// curta que vai ao painel (nunca um caminho); o erro inteiro fica em Unwrap.
type erroEntrada struct {
	frase string
	err   error
}

func (e *erroEntrada) Error() string { return e.frase }
func (e *erroEntrada) Unwrap() error { return e.err }

// semFechar é o Closer de uma entrada de PDFs avulsos: cada Abrir devolve o
// próprio arquivo, e não há nada aberto para fechar no fim.
type semFechar struct{}

func (semFechar) Close() error { return nil }

// LerEntrada lista os documentos de uma entrada: um ZIP só (sem diferença de
// caixa na extensão), lido por ReadZipFiles, ou uma lista só de PDFs avulsos.
// Mistura, lista vazia ou outra extensão dão ErrTipoNaoSuportado. Não confere
// a assinatura %PDF: abrirPDF trata o que não for PDF.
//
// Cada PDF passa por os.Stat (diretório conta como não encontrado), uma
// abertura de prova e o tamanho. Com um PDF só, a falha vira o erro do item;
// com vários, o erro só sai se nenhum passar (com a frase do primeiro na
// lista), e os que falharam ficam na lista com Tamanho 0, para o worker.
func LerEntrada(caminhos []string) ([]models.ProcessFile, io.Closer, error) {
	if len(caminhos) == 0 {
		return nil, nil, ErrTipoNaoSuportado
	}
	if len(caminhos) == 1 && temExtensao(caminhos[0], ".zip") {
		arquivos, rc, err := ReadZipFiles(caminhos[0])
		if err != nil {
			return nil, nil, err
		}
		return arquivos, rc, nil
	}
	for _, c := range caminhos {
		if !temExtensao(c, ".pdf") {
			return nil, nil, ErrTipoNaoSuportado
		}
	}

	arquivos := make([]models.ProcessFile, 0, len(caminhos))
	var primeiraFalha error
	algumBom := false
	for _, c := range caminhos {
		pf := ParseNome(filepath.Base(c))
		pf.Avulso = true
		pf.Abrir = abrirArquivo(c)
		tamanho, err := conferirPDF(c)
		if err != nil {
			if primeiraFalha == nil {
				primeiraFalha = err
			}
		} else {
			pf.Tamanho = tamanho
			algumBom = true
		}
		arquivos = append(arquivos, pf)
	}
	if !algumBom {
		return nil, nil, primeiraFalha
	}
	return arquivos, semFechar{}, nil
}

func temExtensao(caminho, ext string) bool {
	return strings.EqualFold(filepath.Ext(caminho), ext)
}

func abrirArquivo(caminho string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) { return os.Open(caminho) }
}

// conferirPDF devolve o tamanho de um PDF avulso que existe, é arquivo, abre
// e não está vazio; senão um *erroEntrada.
func conferirPDF(caminho string) (int64, error) {
	info, err := os.Stat(caminho)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, &erroEntrada{frase: fraseArquivoNaoEncontrado, err: err}
		}
		return 0, &erroEntrada{frase: textoErroSaida(err), err: err}
	}
	if info.IsDir() {
		return 0, &erroEntrada{frase: fraseArquivoNaoEncontrado,
			err: fmt.Errorf("%s é uma pasta: %w", caminho, fs.ErrNotExist)}
	}
	f, err := os.Open(caminho)
	if err != nil {
		return 0, &erroEntrada{frase: textoErroSaida(err), err: err}
	}
	f.Close()
	if info.Size() == 0 {
		return 0, &erroEntrada{frase: fraseArquivoVazio, err: fmt.Errorf("%s tem 0 bytes", caminho)}
	}
	return info.Size(), nil
}
