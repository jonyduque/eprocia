package etl

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// lexerTermina prevê se o lexer da dslipak/pdf chega ao fim do stream: um
// dicionário, array ou string sem fecho o faz girar para sempre no fim dos dados.
func TestLexerTermina(t *testing.T) {
	casos := []struct {
		nome    string
		dados   string
		termina bool
	}{
		{"texto simples", "BT /F1 12 Tf 72 712 Td (Olá) Tj ET", true},
		{"TJ com array", "BT [(a) -120 (b)] TJ ET", true},
		{"marcação com dicionário", "/P << /MCID 0 >> BDC (x) Tj EMC", true},
		{"hex", "<414243> Tj", true},
		{"hex com espaços", "< 41 42\n43 > Tj", true},
		{"string com parênteses e escapes", `(a (b) \) \\ \101 \
c) Tj`, true},
		{"comentário no fim", "q Q % fim", true},
		{"chave não-nome fecha", "<< 1 2 >> pop", true},
		{"colchete solto encerra", "] (x) Tj", true},
		{">> solto encerra", ">> (x) Tj", true},
		{"hex malformado encerra", "<4G> Tj", true},
		{"vazio", "", true},

		{"dicionário sem fecho", "/P << /MCID 0 BDC", false},
		{"dicionário só com abertura", "<<", false},
		{"array sem fecho", "BT [(a) 1 (b)", false},
		{"operador encerra array", "BT [(a) 1 (b) TJ", true},
		{"string sem fecho", "(abc Tj", false},
		{"string com escape no fim", `(abc\`, false},
		{"hex sem fecho", "<4142", false},
		{"hex com dígito ímpar no fim", "<4", false},
		{"dicionário dentro de array sem fecho", "[<< /A 1 >>", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			feito := make(chan bool, 1)
			go func() { feito <- lexerTermina([]byte(c.dados)) }()
			select {
			case got := <-feito:
				if got != c.termina {
					t.Errorf("lexerTermina(%q) = %v, esperava %v", c.dados, got, c.termina)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("lexerTermina(%q) travou", c.dados)
			}
		})
	}
}

// Um stream de conteúdo com dicionário sem fecho fazia a extração girar para
// sempre (processo real parado por horas); agora vira erro e a página segue
// sem texto nativo.
func TestExtractNativeText_DicionarioSemFechoNaoTrava(t *testing.T) {
	objects := paginaComFonte("BT /F1 12 Tf 72 712 Td (Texto) Tj ET\n/P << /MCID 0 BDC", helvetica)
	page := primeiraPagina(t, objects)
	feito := make(chan error, 1)
	go func() {
		_, err := ExtractNativeText(page)
		feito <- err
	}()
	select {
	case err := <-feito:
		if err == nil || !strings.Contains(err.Error(), "ilegível") {
			t.Errorf("esperava erro de stream ilegível, obtive %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExtractNativeText travou")
	}
}

// O /ToUnicode passa pelo mesmo lexer: um CMap com string sem fecho faz a
// fonte ser lida como sem ToUnicode, sem travar.
func TestExtractNativeText_ToUnicodeSemFechoNaoTrava(t *testing.T) {
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap 1 beginbfchar <01> <0041"
	fonte := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /ToUnicode 6 0 R >>"
	objects := paginaComFonte("BT /F1 12 Tf 72 712 Td (Texto) Tj ET", fonte,
		"<< /Length "+strconv.Itoa(len(cmap))+" >>\nstream\n"+cmap+"\nendstream")
	feito := make(chan struct{})
	go func() {
		ExtractNativeText(primeiraPagina(t, objects))
		close(feito)
	}()
	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal("ExtractNativeText travou no ToUnicode")
	}
}
