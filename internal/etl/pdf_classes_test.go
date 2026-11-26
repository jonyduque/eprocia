package etl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"eprocia/internal/models"
	"eprocia/internal/ocr"

	"github.com/dslipak/pdf"
)

// Um PDF sintético por classe que a dslipak/pdf recusa, de ponta a ponta, com
// o pdfinfo e o pdftotext embutidos de verdade. Todo conteúdo é fictício.

const fraseClasses = "Peticao inicial ficticia de FULANO DE TAL no processo 5000000-00.2026.8.26.0001"

// tempOriginal é o %TEMP% do processo antes de qualquer t.Setenv.
var tempOriginal = os.TempDir()

// dirClasses é a pasta deste processo, criada na primeira chamada de
// tempClasses e apagada pelo TestMain no fim.
var (
	dirClasses      string
	criarDirClasses sync.Once
	errDirClasses   error
)

// tempClasses aponta o %TEMP% para uma pasta própria deste processo, só destes
// testes, e a devolve. A pasta continua fixa durante o processo porque o cache
// binDir/popplerDir do pacote ocr guarda, por processo, o diretório da
// primeira extração: com isolarTemp, o t.TempDir apagado no fim do primeiro
// teste deixaria um caminho morto para os testes seguintes deste arquivo e de
// pipeline_memoria_test.go. Também não serve o %TEMP% real: o eprocIA_bin de
// lá é o do eprocia.exe do dono. Cada processo tem a sua pasta (outra
// execução não a atrapalha) e o TestMain a apaga no fim.
func tempClasses(t *testing.T) string {
	t.Helper()
	criarDirClasses.Do(func() {
		dirClasses, errDirClasses = os.MkdirTemp(tempOriginal, "eprocia_testes_classes_*")
	})
	if errDirClasses != nil {
		t.Fatalf("falha ao criar a pasta dos testes de classes: %v", errDirClasses)
	}
	// Sobra de um teste anterior deste processo não pode reprovar este.
	velhos, _ := filepath.Glob(filepath.Join(dirClasses, "eprocia_pdf_*"))
	for _, v := range velhos {
		os.Remove(v)
	}
	t.Setenv("TMP", dirClasses)
	t.Setenv("TEMP", dirClasses)
	t.Setenv("TMPDIR", dirClasses)
	return dirClasses
}

// rodarClasse processa um ZIP com um único PDF usando o Poppler real e
// devolve o .txt e o estado do documento.
func rodarClasse(t *testing.T, conteudo []byte, cfg *models.Config) (string, AtividadeDoc) {
	t.Helper()
	if testing.Short() {
		t.Skip("usa o Poppler real")
	}
	tmp := tempClasses(t)
	trocarPoppler(t, ocr.InfoPDF, ocr.TextoPDF)
	zipPath := gravarZip(t, []entradaZip{{"Evento 1 - DOC1.pdf", conteudo}})
	atv := NovaAtividade("x")
	out, _, _, _, err := NewPipeline().ProcessZipComAtividade(context.Background(), zipPath, cfg,
		func(float64, string, int, int, int, int) {}, atv)
	if err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("falha ao ler a saída: %v", err)
	}
	semTemporarios(t, tmp)
	docs := atv.Instantaneo().Docs
	if len(docs) != 1 {
		t.Fatalf("esperava 1 documento, veio %d", len(docs))
	}
	return string(b), docs[0]
}

func conferirLeitura(t *testing.T, saida string, d AtividadeDoc, leitura Leitura, naoLidos int) {
	t.Helper()
	if d.Leitura != string(leitura) {
		t.Errorf("leitura = %q, esperava %q (detalhe %q)", d.Leitura, leitura, d.Detalhe)
	}
	if linha := fmt.Sprintf("Documentos não lidos: %d\n", naoLidos); !strings.Contains(saida, linha) {
		t.Errorf("o .txt não tem %q:\n%s", strings.TrimSpace(linha), saida)
	}
}

// objetosTexto são os objetos de um PDF de uma página com fraseClasses em
// Helvetica (que o pdftotext lê sem fonte embutida).
func objetosTexto() []string {
	stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", fraseClasses)
	return []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj",
	}
}

// objetosDigitalizado são os objetos de uma página só com a imagem da
// fixture escaneado.pdf (sem texto nativo).
func objetosDigitalizado(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "bench", "fixtures", "escaneado.pdf"))
	if err != nil {
		t.Fatalf("falha ao ler a fixture: %v", err)
	}
	ini := bytes.Index(b, []byte("6 0 obj\n<< /Type /XObject /Subtype /Image"))
	if ini < 0 {
		t.Fatal("imagem não encontrada em escaneado.pdf")
	}
	fimStream := ini + bytes.Index(b[ini:], []byte("endstream"))
	fim := fimStream + bytes.Index(b[fimStream:], []byte("endobj")) + len("endobj")
	imagem := "4" + string(b[ini+1:fim])
	conteudo := "q 595 0 0 222 0 620 cm /Im1 Do Q"
	return []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 4 0 R >> >> /Contents 5 0 R >>\nendobj",
		imagem,
		fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(conteudo), conteudo),
	}
}

// Os geradores reproduzem a recusa da biblioteca (senão o teste não prova o
// segundo leitor) e o Poppler real abre o que deve abrir.
func TestClasses_GeradoresRecusadosPelaBiblioteca(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o Poppler real")
	}
	casos := map[string][]byte{
		"Versao20":      pdfVersao20(objetosTexto()),
		"Envelope":      pdfEmEnvelope(buildPDF(objetosTexto())),
		"AESV2SemSenha": pdfAESV2(objetosTexto(), ""),
		"AESV2ComSenha": pdfAESV2(objetosTexto(), "segredo"),
		"Truncado":      pdfTruncado(),
		"Digitalizado":  pdfVersao20(objetosDigitalizado(t)),
	}
	base := buildPDF(objetosTexto())
	if _, err := pdf.NewReader(bytes.NewReader(base), int64(len(base))); err != nil {
		t.Fatalf("o PDF de base não abre na biblioteca: %v", err)
	}
	for nome, b := range casos {
		if _, err := pdf.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
			t.Errorf("%s: a biblioteca abriu o PDF", nome)
		} else {
			t.Logf("%s: biblioteca recusa: %v", nome, err)
		}
	}
	if b := casos["Envelope"]; !bytes.HasPrefix(b, []byte{0x30, 0x80, 0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02}) ||
		!bytes.Equal(b[52:57], []byte("%PDF-")) {
		t.Errorf("envelope sem o prefixo esperado: % X", b[:60])
	}

	tmp := tempClasses(t)
	gravar := func(nome string, b []byte) string {
		c := filepath.Join(t.TempDir(), nome+".pdf")
		if err := os.WriteFile(c, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return c
	}
	semSenha := gravar("sem-senha", casos["AESV2SemSenha"])
	if n, err := ocr.InfoPDF(context.Background(), semSenha); err != nil || n != 1 {
		t.Fatalf("pdfinfo no AESV2 sem senha: %d, %v", n, err)
	}
	textos, err := ocr.TextoPDF(context.Background(), semSenha, 1)
	if err != nil || !strings.Contains(textos[0], fraseClasses) {
		t.Errorf("pdftotext no AESV2 sem senha: %q, %v", textos, err)
	}
	comSenha := gravar("com-senha", casos["AESV2ComSenha"])
	if _, err := ocr.InfoPDF(context.Background(), comSenha); err == nil || !strings.Contains(err.Error(), ocr.ErrSenha.Error()) {
		t.Errorf("pdfinfo no AESV2 com senha: esperava Incorrect password, veio %v", err)
	}
	semTemporarios(t, tmp)
}

// O AESV2 com o filtro de 16 bytes, como a norma escreve, a biblioteca abre:
// ele não é uma classe recusada e sai como "lido".
func TestClasses_AESV2NaFormaDaNormaAbreNaBiblioteca(t *testing.T) {
	b := pdfAESV2Filtro(objetosTexto(), "", 16)
	r, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("a biblioteca recusou o AESV2 da norma: %v", err)
	}
	if r.NumPage() != 1 {
		t.Errorf("NumPage = %d", r.NumPage())
	}
	saida, d := rodarClasse(t, b, cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraLido, 0)
	if !strings.Contains(saida, fraseClasses) {
		t.Errorf("o .txt não tem a frase:\n%s", saida)
	}
}

type fraseClasse int

const (
	fraseNaoConfere fraseClasse = iota
	fraseNoTxt
	fraseFora
)

func TestClasses_PorClasse(t *testing.T) {
	casos := []struct {
		nome     string
		pdf      func(t *testing.T) []byte
		preparar func(t *testing.T)
		leitura  Leitura
		naoLidos int
		frase    fraseClasse
		marcador string
	}{
		{nome: "Versao20", pdf: func(*testing.T) []byte { return pdfVersao20(objetosTexto()) },
			leitura: LeituraSegundoLeitor, frase: fraseNoTxt},
		{nome: "Envelope", pdf: func(*testing.T) []byte { return pdfEmEnvelope(buildPDF(objetosTexto())) },
			leitura: LeituraSegundoLeitor, frase: fraseNoTxt},
		{nome: "AESV2SemSenha", pdf: func(*testing.T) []byte { return pdfAESV2(objetosTexto(), "") },
			leitura: LeituraSegundoLeitor, frase: fraseNoTxt},
		{nome: "AESV2ComSenha", pdf: func(*testing.T) []byte { return pdfAESV2(objetosTexto(), "segredo") },
			leitura: LeituraSenha, naoLidos: 1, frase: fraseFora,
			marcador: "[documento não lido: protegido por senha; consulte o PDF original]"},
		{nome: "Truncado", pdf: func(*testing.T) []byte { return pdfTruncado() },
			leitura: LeituraIlegivel, naoLidos: 1, frase: fraseNaoConfere},
		// Não há PDF sintético com o xref que faz a biblioteca entrar em pânico
		// em NumPage(): o pânico é simulado e o PDF sadio vai ao Poppler real.
		{nome: "TabelaInconsistente", pdf: func(*testing.T) []byte { return buildPDF(objetosTexto()) },
			preparar: func(t *testing.T) {
				original := contarPaginasPDF
				t.Cleanup(func() { contarPaginasPDF = original })
				contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) {
					panic("loading {8 0}: found {7 0}")
				}
			},
			leitura: LeituraSegundoLeitor, frase: fraseNoTxt},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if c.preparar != nil {
				c.preparar(t)
			}
			saida, d := rodarClasse(t, c.pdf(t), cfgPoppler("none"))
			conferirLeitura(t, saida, d, c.leitura, c.naoLidos)
			if c.marcador != "" && !strings.Contains(saida, c.marcador) {
				t.Errorf("o .txt não tem %q:\n%s", c.marcador, saida)
			}
			switch c.frase {
			case fraseNoTxt:
				if !strings.Contains(saida, fraseClasses) {
					t.Errorf("o .txt não tem a frase:\n%s", saida)
				}
			case fraseFora:
				if strings.Contains(saida, fraseClasses) {
					t.Errorf("o texto cifrado vazou para o .txt:\n%s", saida)
				}
			}
		})
	}
}

// palavraDaImagem é uma palavra longa do texto da imagem de escaneado.pdf
// que o OCR fast devolve.
const palavraDaImagem = "contestacao"

func TestClasses_Versao20Digitalizado(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o Poppler real")
	}
	b := pdfVersao20(objetosDigitalizado(t))
	t.Run("sem OCR", func(t *testing.T) {
		saida, d := rodarClasse(t, b, cfgPoppler("none"))
		conferirLeitura(t, saida, d, LeituraSemOCR, 1)
		if marcador := "[documento não lido: digitalizado; o OCR está desativado; consulte o PDF original]"; !strings.Contains(saida, marcador) {
			t.Errorf("o .txt não tem %q:\n%s", marcador, saida)
		}
	})
	t.Run("com OCR", func(t *testing.T) {
		saida, d := rodarClasse(t, b, cfgPoppler("fast"))
		conferirLeitura(t, saida, d, LeituraOCR, 0)
		// Só o corpo do documento: o cabeçalho do .txt já diz "PROCESSO".
		corpo := strings.ToLower(documentoDe(t, saida, "Evento 1 - DOC1.pdf"))
		if palavra := palavraDaImagem; !strings.Contains(corpo, palavra) {
			t.Errorf("o OCR não leu a imagem (sem %q no documento):\n%s", palavra, corpo)
		}
	})
}
