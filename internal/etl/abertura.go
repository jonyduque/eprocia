package etl

import (
	"context"
	"errors"
	"fmt"

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

// infoPDF e textoPDF são o segundo leitor; os testes os trocam para não
// depender dos binários.
var infoPDF, textoPDF = ocr.InfoPDF, ocr.TextoPDF

// abrirPDF tenta a biblioteca e, se ela recusar o arquivo (PDF 2.0, PKCS#7,
// AES, xref inconsistente, pânico), conta as páginas com o pdfinfo. Senha
// errada não vai ao Poppler: ele também não abriria. Se os dois falharem, o
// erro embrulha o do Poppler (errors.Is chega a ocr.ErrPrazo/ErrSenha) e cita
// o texto da biblioteca.
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
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %v; não foi possível gravá-lo para o segundo leitor: %w", errLib, err)
	}
	n, err = infoPDF(ctx, caminho)
	if err != nil {
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %v; segundo leitor: %w", errLib, err)
	}
	return aberturaPDF{paginas: n, poppler: true}, nil
}
