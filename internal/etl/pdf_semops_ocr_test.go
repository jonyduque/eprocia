package etl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// Investigação do B-EF-03: página de /Contents único sem operadores de texto
// cuja imagem está dentro de um Form XObject (HasImages só olha o /XObject da
// página, então não a conta). Antes do B-EF-03 o texto vazio caía no
// GetTextByRow, que zerava as métricas (valida=false) e nenhuma regra nova
// agia: a página não ia ao OCR. Agora as métricas ficam válidas e
// motivoImagemDescoberta manda a página ao OCR. Medido: em c605d58^ este teste
// falha (sem OCR); em HEAD passa. A mudança é desejável (página só de imagem
// vai ao OCR) e o teste fica como trava.
func TestPDFParser_ParsePages_ImagemEmFormSemTextoVaiAoOCR(t *testing.T) {
	form := "q 595 0 0 842 0 0 cm /Im0 Do Q"
	conteudo := "/Fm0 Do"
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Fm0 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(conteudo), conteudo),
		fmt.Sprintf("5 0 obj\n<< /Type /XObject /Subtype /Form /BBox [0 0 595 842] /Resources << /XObject << /Im0 6 0 R >> >> /Length %d >>\nstream\n%s\nendstream\nendobj", len(form), form),
		"6 0 obj\n<< /Type /XObject /Subtype /Image /Width 1200 /Height 1700 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 4 >>\nstream\nabcd\nendstream\nendobj",
	}
	var chamadas atomic.Int32
	ocr := func(ctx context.Context, path string, n int) (string, error) {
		chamadas.Add(1)
		return "texto do OCR", nil
	}
	parsePagesWithDeadline(t, objects, ocr)
	if chamadas.Load() == 0 {
		t.Errorf("a página sem texto com imagem em Form não foi ao OCR")
	}
}
