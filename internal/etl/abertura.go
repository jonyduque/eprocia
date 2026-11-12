package etl

import (
	"context"
	"errors"
	"fmt"
	"log"

	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
)

// aberturaPDF é o resultado de abrir um PDF na pré-contagem: pela biblioteca
// (leitor preenchido) ou, quando ela o recusa, pelo Poppler embutido.
type aberturaPDF struct {
	leitor  *pdf.Reader // nil quando o Poppler abriu
	paginas int
	poppler bool
}

// Resumos do erro do segundo leitor que chegam ao .txt e ao painel: frases
// fixas, sem caminho nem stderr (o erro inteiro vai só para o log).
const (
	resumoSegundoLeitorPrazo = "tempo esgotado"
	resumoSegundoLeitorSenha = "protegido por senha"
	resumoSegundoLeitorFalha = "não conseguiu ler o arquivo"
)

// infoPDF e textoPDF são o segundo leitor; os testes os trocam para não
// depender dos binários.
var infoPDF, textoPDF = ocr.InfoPDF, ocr.TextoPDF

// erroSegundoLeitor é a falha do Poppler como chega ao usuário: Error() é só
// uma frase fixa, porque o erro original pode trazer o caminho do temporário
// (com o nome de usuário do Windows) e o stderr do pdfinfo/pdftotext, e o
// marcador vai para o .txt colado num LLM. A causa fica para errors.Is/As.
type erroSegundoLeitor struct {
	resumo string
	causa  error
}

func (e *erroSegundoLeitor) Error() string { return "segundo leitor: " + e.resumo }
func (e *erroSegundoLeitor) Unwrap() error { return e.causa }

// resumirSegundoLeitor registra o erro inteiro no log e devolve a forma curta.
// resumo vazio escolhe a frase pela classe do erro.
func resumirSegundoLeitor(nome, resumo string, err error) error {
	if resumo == "" {
		switch {
		case errors.Is(err, ocr.ErrPrazo):
			resumo = resumoSegundoLeitorPrazo
		case errors.Is(err, ocr.ErrSenha):
			resumo = resumoSegundoLeitorSenha
		default:
			resumo = resumoSegundoLeitorFalha
		}
	}
	if nome != "" {
		log.Printf("[PDFParser] Segundo leitor falhou em %s: %v", nome, err)
	} else {
		log.Printf("[PDFParser] Segundo leitor falhou: %v", err)
	}
	return &erroSegundoLeitor{resumo: resumo, causa: err}
}

// abrirPDF tenta a biblioteca e, se ela recusar o arquivo (PDF 2.0, PKCS#7,
// AES, xref inconsistente, pânico), conta as páginas com o pdfinfo. Senha
// errada não vai ao Poppler: ele também não abriria. Se os dois falharem, o
// erro cita o texto da biblioteca (que não traz caminho) e o resumo do
// segundo leitor, e embrulha a causa (errors.Is chega a ocr.ErrPrazo/ErrSenha).
func abrirPDF(ctx context.Context, f *fontePDF) (aberturaPDF, error) {
	n, errLib := f.contarPaginas()
	if errLib == nil {
		return aberturaPDF{leitor: f.leitor, paginas: n}, nil
	}
	if errors.Is(errLib, pdf.ErrInvalidPassword) {
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %w", errLib)
	}
	caminho, err := f.materializar()
	if err != nil {
		// O erro do CreateTemp traz o caminho de %TEMP%.
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %v; %w", errLib,
			resumirSegundoLeitor("", "não foi possível gravar o arquivo temporário", err))
	}
	n, err = infoPDF(ctx, caminho)
	if err == nil && n <= 0 {
		err = fmt.Errorf("o pdfinfo informou %d páginas", n)
	}
	if err != nil {
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %v; %w", errLib, resumirSegundoLeitor("", "", err))
	}
	return aberturaPDF{paginas: n, poppler: true}, nil
}
