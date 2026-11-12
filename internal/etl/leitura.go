package etl

import (
	"context"
	"errors"
	"strings"

	"github.com/dslipak/pdf"
)

// Leitura diz como o texto de um documento foi obtido, ou por que não foi.
type Leitura string

const (
	LeituraLido          Leitura = "lido"
	LeituraSegundoLeitor Leitura = "segundo_leitor"
	LeituraOCR           Leitura = "ocr"
	LeituraSemOCR        Leitura = "sem_ocr"
	LeituraSenha         Leitura = "senha"
	LeituraIlegivel      Leitura = "ilegivel"
	LeituraPrazo         Leitura = "prazo"
	LeituraOCRFalhou     Leitura = "ocr_falhou"
)

// NaoLido informa se o documento ficou sem texto aproveitável.
func (l Leitura) NaoLido() bool {
	switch l {
	case LeituraSemOCR, LeituraSenha, LeituraIlegivel, LeituraPrazo, LeituraOCRFalhou:
		return true
	}
	return false
}

// Frase é o motivo em português mostrado ao usuário (vazia para "lido").
func (l Leitura) Frase() string {
	switch l {
	case LeituraSegundoLeitor:
		return "lido pelo segundo leitor"
	case LeituraOCR:
		return "texto obtido por OCR"
	case LeituraSemOCR:
		return "digitalizado; o OCR está desativado"
	case LeituraSenha:
		return "protegido por senha"
	case LeituraIlegivel:
		return "arquivo ilegível"
	case LeituraPrazo:
		return "prazo de leitura esgotado"
	case LeituraOCRFalhou:
		return "o OCR falhou em todas as páginas"
	}
	return ""
}

// Acao é o rótulo do botão que resolve o motivo, quando há um.
func (l Leitura) Acao() string {
	if l == LeituraSemOCR {
		return "Reprocessar com OCR"
	}
	return ""
}

// classificarErro traduz o erro de abertura/leitura de um documento.
func classificarErro(err error) Leitura {
	switch {
	case errors.Is(err, pdf.ErrInvalidPassword):
		return LeituraSenha
	case errors.Is(err, context.DeadlineExceeded):
		return LeituraPrazo
	}
	return LeituraIlegivel
}

// classificarPaginas decide a leitura de um documento que terminou sem erro,
// a partir do mapa de páginas.
func classificarPaginas(paginas map[string]any, segundoLeitor bool) Leitura {
	comTexto, todasOCR := 0, true
	var semOCR, prazo, ocrErro, ilegivel bool
	for _, v := range paginas {
		p, _ := v.(map[string]any)
		if p == nil {
			continue
		}
		content, _ := p["content"].(string)
		_, temSemOCR := p["sem_ocr"]
		_, temPrazo := p["ocr_prazo"]
		_, temErroOCR := p["ocr_erro"]
		semOCR = semOCR || temSemOCR
		prazo = prazo || temPrazo
		ocrErro = ocrErro || temErroOCR
		marcadorErro := strings.HasPrefix(content, "[ERRO") || strings.HasPrefix(content, "[texto ilegível na página:")
		if marcadorErro {
			ilegivel = true
		}
		if strings.TrimSpace(content) == "" || marcadorErro || temErroOCR || temSemOCR {
			continue
		}
		comTexto++
		if o, _ := p["ocr"].(bool); !o {
			todasOCR = false
		}
	}
	switch {
	case comTexto > 0:
		if todasOCR {
			return LeituraOCR
		}
		if segundoLeitor {
			return LeituraSegundoLeitor
		}
		return LeituraLido
	case semOCR:
		return LeituraSemOCR
	case prazo:
		return LeituraPrazo
	case ocrErro:
		return LeituraOCRFalhou
	case ilegivel:
		return LeituraIlegivel
	}
	return LeituraLido
}

// docLeitura registra o motivo de leitura do documento i.
func (a *Atividade) docLeitura(i int, l Leitura, detalhe string) {
	a.mudar(i, func(d *AtividadeDoc) {
		d.Leitura, d.Detalhe = string(l), detalhe
	})
}

// NaoLidos conta os documentos que ficaram sem texto aproveitável.
func (a *Atividade) NaoLidos() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, d := range a.docs {
		if Leitura(d.Leitura).NaoLido() {
			n++
		}
	}
	return n
}
