package etl

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"testing"

	"github.com/dslipak/pdf"
)

func TestLeitura_NaoLido(t *testing.T) {
	casos := map[Leitura]bool{
		LeituraLido: false, LeituraSegundoLeitor: false, LeituraOCR: false,
		LeituraSemOCR: true, LeituraSenha: true, LeituraIlegivel: true,
		LeituraPrazo: true, LeituraOCRFalhou: true,
	}
	for l, esperado := range casos {
		if l.NaoLido() != esperado {
			t.Errorf("%q.NaoLido() = %v, esperava %v", l, l.NaoLido(), esperado)
		}
	}
}

func TestLeitura_FraseEAcao(t *testing.T) {
	casos := []struct {
		l     Leitura
		frase string
	}{
		{LeituraLido, ""},
		{LeituraSegundoLeitor, "lido pelo segundo leitor"},
		{LeituraOCR, "texto obtido por OCR"},
		{LeituraSemOCR, "digitalizado; o OCR está desativado"},
		{LeituraSenha, "protegido por senha"},
		{LeituraIlegivel, "arquivo ilegível"},
		{LeituraPrazo, "prazo de leitura esgotado"},
		{LeituraOCRFalhou, "o OCR falhou em todas as páginas"},
	}
	for _, c := range casos {
		if got := c.l.Frase(); got != c.frase {
			t.Errorf("%q.Frase() = %q, esperava %q", c.l, got, c.frase)
		}
		acao := ""
		if c.l == LeituraSemOCR {
			acao = "Reprocessar com OCR"
		}
		if got := c.l.Acao(); got != acao {
			t.Errorf("%q.Acao() = %q, esperava %q", c.l, got, acao)
		}
	}
}

func TestClassificarErro(t *testing.T) {
	casos := []struct {
		nome string
		err  error
		l    Leitura
	}{
		{"senha", fmt.Errorf("falha ao abrir o PDF: %w", pdf.ErrInvalidPassword), LeituraSenha},
		{"prazo", fmt.Errorf("leitura: %w", context.DeadlineExceeded), LeituraPrazo},
		{"ilegivel", errors.New("not a PDF file: invalid header"), LeituraIlegivel},
	}
	for _, c := range casos {
		if got := classificarErro(c.err); got != c.l {
			t.Errorf("%s: classificarErro = %q, esperava %q", c.nome, got, c.l)
		}
	}
}

func TestClassificarPaginas(t *testing.T) {
	pg := func(content string, extra map[string]any) map[string]any {
		m := map[string]any{"content": content, "ocr": false}
		maps.Copy(m, extra)
		return m
	}
	doc := func(ps ...map[string]any) map[string]any {
		m := map[string]any{}
		for i, p := range ps {
			m[fmt.Sprint(i+1)] = p
		}
		return m
	}
	ocr := map[string]any{"ocr": true}
	semOCR := map[string]any{"sem_ocr": true}
	erroOCR := map[string]any{"ocr_erro": "falhou"}
	prazo := map[string]any{"ocr_erro": "prazo", "ocr_prazo": true}

	casos := []struct {
		nome  string
		pags  map[string]any
		segue bool
		l     Leitura
	}{
		{"a nativas", doc(pg("a", nil), pg("b", nil)), false, LeituraLido},
		{"b ocr", doc(pg("a", ocr), pg("b", ocr)), false, LeituraOCR},
		{"c mista", doc(pg("a", nil), pg("b", ocr)), false, LeituraLido},
		{"d segundo leitor", doc(pg("a", nil), pg("b", nil)), true, LeituraSegundoLeitor},
		{"e sem_ocr", doc(pg("", semOCR), pg("", semOCR)), false, LeituraSemOCR},
		{"f um texto e dois sem_ocr", doc(pg("a", nil), pg("x", semOCR), pg("y", semOCR)), false, LeituraLido},
		{"g ocr_erro", doc(pg("", erroOCR), pg("", erroOCR)), false, LeituraOCRFalhou},
		{"h prazo", doc(pg("", erroOCR), pg("", prazo)), false, LeituraPrazo},
		{"i erro", doc(pg("[ERRO ao ler página 1: x]", nil)), false, LeituraIlegivel},
		{"i ilegivel", doc(pg(marcadorIlegivel("x"), nil)), false, LeituraIlegivel},
		{"j vazio", map[string]any{}, false, LeituraLido},
		{"k conteudo vazio", doc(pg("  \n", nil), pg("", nil)), false, LeituraLido},
	}
	for _, c := range casos {
		if got := classificarPaginas(c.pags, c.segue); got != c.l {
			t.Errorf("%s: classificarPaginas = %q, esperava %q", c.nome, got, c.l)
		}
	}
}
