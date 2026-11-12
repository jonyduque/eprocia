package etl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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
	// Falha ao gravar o temporário que o Poppler lê (o erro do CreateTemp traz
	// o caminho de %TEMP%).
	resumoSegundoLeitorTemporario = "não foi possível gravar o arquivo temporário"
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
	lib := resumirErroBiblioteca(errLib)
	if lib != errLib.Error() {
		log.Printf("[PDFParser] Biblioteca recusou o PDF: %v", errLib)
	}
	caminho, err := f.materializar()
	if err != nil {
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %s; %w", lib,
			resumirSegundoLeitor("", resumoSegundoLeitorTemporario, err))
	}
	n, err = infoPDF(ctx, caminho)
	if err == nil && n <= 0 {
		err = fmt.Errorf("o pdfinfo informou %d páginas", n)
	}
	if err != nil {
		return aberturaPDF{}, fmt.Errorf("falha ao abrir o PDF: %s; %w", lib, resumirSegundoLeitor("", "", err))
	}
	return aberturaPDF{paginas: n, poppler: true}, nil
}

// tetoErroBiblioteca é o maior trecho do erro da biblioteca, em runas, que vai
// ao .txt e ao painel.
const tetoErroBiblioteca = 120

// resumirErroBiblioteca encurta o erro da dslipak/pdf para o .txt e o painel:
// alguns embutem objetos do arquivo (o dicionário /Encrypt com os bytes de /O e
// /U, a tabela xref). O erro de cifra fica até o primeiro ';', controles e
// bytes inválidos viram espaço e o resto tem teto de 120 runas. Os erros curtos
// de hoje saem iguais.
func resumirErroBiblioteca(err error) string {
	s := err.Error()
	if strings.HasPrefix(s, "unsupported PDF: encryption version") {
		s, _, _ = strings.Cut(s, ";")
	}
	s = strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if utf8.RuneCountInString(s) > tetoErroBiblioteca {
		s = string([]rune(s)[:tetoErroBiblioteca]) + "…"
	}
	return s
}

// Textos fixos de erro que chegam ao .txt, ao painel e à notificação.
const (
	// Erro de E/S ou de início de processo: o texto original traz o caminho.
	textoErroDisco = "falha de leitura ou gravação em disco"
	// Rede de segurança: o texto resumido ainda parecia trazer um caminho.
	textoErroInterno = "erro interno; detalhe no log"
)

// Marcadores da página cujo OCR falhou sem texto nativo de reserva. ocr_erro
// guarda a mesma frase, sem os colchetes.
const (
	frasePaginaOCRPrazo = "tempo esgotado"
	frasePaginaOCRFalha = "o OCR não conseguiu ler a página"
	marcadorOCRPrazo    = "[ERRO de OCR: " + frasePaginaOCRPrazo + "]"
	marcadorOCRFalha    = "[ERRO de OCR: " + frasePaginaOCRFalha + "]"
)

// pareceCaminho reconhece um caminho do Windows no texto: letra de unidade
// seguida de barra (C:\ ou C:/, sem outra letra antes, para não pegar "http://")
// ou o prefixo UNC \\.
var pareceCaminho = regexp.MustCompile(`(^|[^A-Za-z])[A-Za-z]:[\\/]|\\\\`)

// vetarCaminho devolve s, ou o texto fixo de erro interno quando s parece trazer
// um caminho.
func vetarCaminho(s string) string {
	if pareceCaminho.MatchString(s) {
		return textoErroInterno
	}
	return s
}

// textoErroSaida é a porta de todo texto de erro que sai do pipeline para o
// .txt, o Detalhe do painel ou a notificação: nunca um caminho (o temporário
// traz o nome de usuário do Windows), stderr ou objetos do PDF. Quem chama
// registra o erro inteiro no log.
func textoErroSaida(err error) string {
	if segundo, ok := errors.AsType[*erroSegundoLeitor](err); ok {
		return segundo.Error()
	}
	if ehErroDeDisco(err) {
		return textoErroDisco
	}
	return vetarCaminho(resumirErroBiblioteca(err))
}

// ehErroDeDisco reconhece um erro de E/S ou de início de processo, cujo texto
// traz o caminho do arquivo ou do executável.
func ehErroDeDisco(err error) bool {
	if _, ok := errors.AsType[*fs.PathError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*os.LinkError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*os.SyscallError](err); ok {
		return true
	}
	_, ok := errors.AsType[*exec.Error](err)
	return ok
}

// fraseOCR é o texto fixo de uma página cujo OCR falhou.
func fraseOCR(err error) string {
	if errors.Is(err, ocr.ErrPrazo) {
		return frasePaginaOCRPrazo
	}
	return frasePaginaOCRFalha
}
