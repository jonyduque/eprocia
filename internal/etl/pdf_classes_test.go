package etl

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

// tempClasses aponta o %TEMP% para uma pasta fixa, só destes testes, e a
// devolve. Não serve isolarTemp: o pacote ocr guarda em cache, por processo, a
// pasta onde extraiu os binários, e o t.TempDir de um teste some no fim dele
// (o seguinte acharia o cache apontando para uma pasta apagada). Também não
// serve o %TEMP% real: o eprocIA_bin de lá é o do eprocia.exe do dono.
func tempClasses(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(tempOriginal, "eprocia_testes_classes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("falha ao criar %s: %v", dir, err)
	}
	// Sobras de uma execução interrompida não podem reprovar esta.
	velhos, _ := filepath.Glob(filepath.Join(dir, "eprocia_pdf_*"))
	for _, v := range velhos {
		os.Remove(v)
	}
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Setenv("TMPDIR", dir)
	return dir
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

// pdfVersao20 é a saída de buildPDF com o cabeçalho %PDF-2.0.
func pdfVersao20(objects []string) []byte {
	return comoPDF20(buildPDF(objects))
}

// pdfEmEnvelope embrulha o PDF num envelope CMS SignedData (BER de tamanho
// indefinido, como o do MP): 52 bytes de prefixo, o PDF como um OCTET STRING
// primitivo e os bytes de fechamento.
func pdfEmEnvelope(p []byte) []byte {
	if len(p) > 0xFFFF {
		panic("pdfEmEnvelope: PDF grande demais para o comprimento de 2 bytes")
	}
	prefixo := []byte{
		0x30, 0x80, // ContentInfo
		0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02, // signedData
		0xA0, 0x80, // [0]
		0x30, 0x80, // SignedData
		0x02, 0x01, 0x01, // version 1
		0x31, 0x0B, 0x30, 0x09, 0x06, 0x05, 0x2B, 0x0E, 0x03, 0x02, 0x1A, 0x05, 0x00, // digestAlgorithms {sha1}
		0x30, 0x80, // encapContentInfo
		0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x01, // data
		0xA0, 0x80, // [0]
		0x04, 0x82, byte(len(p) >> 8), byte(len(p)), // OCTET STRING
	}
	if len(prefixo) != 52 {
		panic(fmt.Sprintf("pdfEmEnvelope: prefixo com %d bytes", len(prefixo)))
	}
	fechamento := []byte{
		0x00, 0x00, // fim do [0] do conteúdo
		0x00, 0x00, // fim do encapContentInfo
		0x31, 0x00, // signerInfos vazio
		0x00, 0x00, // fim do SignedData
		0x00, 0x00, // fim do [0]
		0x00, 0x00, // fim do ContentInfo
	}
	out := append(append(append([]byte{}, prefixo...), p...), fechamento...)
	return out
}

// pdfTruncado são os 64 primeiros bytes de um PDF sadio.
func pdfTruncado() []byte {
	return buildPDF(objetosTexto())[:64]
}

var (
	reCabecalhoObj = regexp.MustCompile(`^(\d+) (\d+) obj\n`)
	reLength       = regexp.MustCompile(`/Length \d+`)
)

// padSenha completa a senha com o preenchimento do algoritmo 2 (ISO 32000-1).
func padSenha(s string) []byte {
	b := []byte(s)
	if len(b) > 32 {
		b = b[:32]
	}
	return append(b, passwordPadTeste[:32-len(b)]...)
}

var passwordPadTeste = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

// rc4Iterado aplica o RC4 com a chave e, nas 19 voltas seguintes, com a chave
// XOR i (algoritmos 3 e 5, R >= 3).
func rc4Iterado(chave, dados []byte) []byte {
	out := append([]byte{}, dados...)
	for i := 0; i <= 19; i++ {
		k := make([]byte, len(chave))
		for j := range chave {
			k[j] = chave[j] ^ byte(i)
		}
		c, err := rc4.NewCipher(k)
		if err != nil {
			panic(err)
		}
		c.XORKeyStream(out, out)
	}
	return out
}

func md5Iterado(b []byte, voltas int) []byte {
	h := md5.Sum(b)
	k := h[:]
	for range voltas {
		h = md5.Sum(k[:16])
		k = h[:]
	}
	return k[:16]
}

// pdfAESV2 cifra os objetos (no formato de buildPDF) com o handler padrão,
// /V 4 /R 4 e o filtro /AESV2 com chave de 128 bits. Só os streams são
// cifrados: os objetos não podem ter strings literais nos dicionários. O
// filtro declara /Length 128 (em bits), como escrevem produtores reais: a
// biblioteca só aceita 16 (bytes) e recusa o arquivo com "encryption version
// V=4", que é o erro visto no PDF real; o Poppler aceita os dois.
func pdfAESV2(objects []string, senhaUsuario string) []byte {
	return pdfAESV2Filtro(objects, senhaUsuario, 128)
}

// pdfAESV2Filtro é pdfAESV2 com o /Length do filtro /StdCF escolhido.
func pdfAESV2Filtro(objects []string, senhaUsuario string, comprimentoFiltro int) []byte {
	const senhaDono = "proprietario-ficticio"
	var p int32 = -4
	id := md5.Sum([]byte("identificador ficticio do documento"))

	// Algoritmo 3: /O.
	o := rc4Iterado(md5Iterado(padSenha(senhaDono), 50), padSenha(senhaUsuario))
	// Algoritmo 2: chave do arquivo.
	var entrada []byte
	entrada = append(entrada, padSenha(senhaUsuario)...)
	entrada = append(entrada, o...)
	entrada = binary.LittleEndian.AppendUint32(entrada, uint32(p))
	entrada = append(entrada, id[:]...)
	chave := md5Iterado(entrada, 50)
	// Algoritmo 5: /U.
	hu := md5.Sum(append(append([]byte{}, passwordPadTeste...), id[:]...))
	u := append(rc4Iterado(chave, hu[:]), make([]byte, 16)...)

	cifrar := func(num, ger int, claro []byte) []byte {
		k := md5.Sum(append(append(append([]byte{}, chave...),
			byte(num), byte(num>>8), byte(num>>16), byte(ger), byte(ger>>8)), "sAlT"...))
		bloco, err := aes.NewCipher(k[:16])
		if err != nil {
			panic(err)
		}
		falta := aes.BlockSize - len(claro)%aes.BlockSize
		dados := append(append([]byte{}, claro...), bytes.Repeat([]byte{byte(falta)}, falta)...)
		iv := md5.Sum(fmt.Appendf(nil, "iv %d %d", num, ger))
		out := append([]byte{}, iv[:]...)
		cifrado := make([]byte, len(dados))
		cipher.NewCBCEncrypter(bloco, iv[:]).CryptBlocks(cifrado, dados)
		return append(out, cifrado...)
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.6\n")
	offsets := make([]int, 0, len(objects)+1)
	for _, obj := range objects {
		m := reCabecalhoObj.FindStringSubmatch(obj)
		if m == nil {
			panic("pdfAESV2: objeto sem cabeçalho: " + obj)
		}
		num, _ := strconv.Atoi(m[1])
		ger, _ := strconv.Atoi(m[2])
		offsets = append(offsets, buf.Len())
		ini := strings.Index(obj, "stream\n")
		if ini < 0 {
			if strings.Contains(obj, "(") {
				panic("pdfAESV2: string literal fora de stream")
			}
			buf.WriteString(obj)
			buf.WriteString("\n")
			continue
		}
		fim := strings.LastIndex(obj, "\nendstream")
		dicionario, claro := obj[:ini], obj[ini+len("stream\n"):fim]
		if strings.Contains(dicionario, "(") {
			panic("pdfAESV2: string literal fora de stream")
		}
		cifrado := cifrar(num, ger, []byte(claro))
		buf.WriteString(reLength.ReplaceAllString(dicionario, fmt.Sprintf("/Length %d", len(cifrado))))
		buf.WriteString("stream\n")
		buf.Write(cifrado)
		buf.WriteString("\nendstream\nendobj\n")
	}
	numEncrypt := len(objects) + 1
	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "%d 0 obj\n<< /Filter /Standard /V 4 /R 4 /Length 128 "+
		"/CF << /StdCF << /CFM /AESV2 /AuthEvent /DocOpen /Length %d >> >> /StmF /StdCF /StrF /StdCF "+
		"/O <%s> /U <%s> /P %d >>\nendobj\n", numEncrypt, comprimentoFiltro, hex.EncodeToString(o), hex.EncodeToString(u), p)

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R /Encrypt %d 0 R /ID [<%x> <%x>] >>\nstartxref\n%d\n%%%%EOF",
		len(offsets)+1, numEncrypt, id, id, xref)
	return buf.Bytes()
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
	}
	if _, err := pdf.NewReader(bytes.NewReader(buildPDF(objetosTexto())), int64(len(buildPDF(objetosTexto())))); err != nil {
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

func TestClasses_Versao20(t *testing.T) {
	saida, d := rodarClasse(t, pdfVersao20(objetosTexto()), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraSegundoLeitor, 0)
	if !strings.Contains(saida, fraseClasses) {
		t.Errorf("o .txt não tem a frase:\n%s", saida)
	}
}

func TestClasses_Envelope(t *testing.T) {
	saida, d := rodarClasse(t, pdfEmEnvelope(buildPDF(objetosTexto())), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraSegundoLeitor, 0)
	if !strings.Contains(saida, fraseClasses) {
		t.Errorf("o .txt não tem a frase:\n%s", saida)
	}
}

func TestClasses_AESV2SemSenha(t *testing.T) {
	saida, d := rodarClasse(t, pdfAESV2(objetosTexto(), ""), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraSegundoLeitor, 0)
	if !strings.Contains(saida, fraseClasses) {
		t.Errorf("o .txt não tem a frase:\n%s", saida)
	}
}

func TestClasses_AESV2ComSenha(t *testing.T) {
	saida, d := rodarClasse(t, pdfAESV2(objetosTexto(), "segredo"), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraSenha, 1)
	if marcador := "[documento não lido: protegido por senha; consulte o PDF original]"; !strings.Contains(saida, marcador) {
		t.Errorf("o .txt não tem %q:\n%s", marcador, saida)
	}
	if strings.Contains(saida, fraseClasses) {
		t.Errorf("o texto cifrado vazou para o .txt:\n%s", saida)
	}
}

func TestClasses_Truncado(t *testing.T) {
	saida, d := rodarClasse(t, pdfTruncado(), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraIlegivel, 1)
}

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
		if baixo := strings.ToLower(saida); !strings.Contains(baixo, "processo") && !strings.Contains(baixo, "parte") {
			t.Errorf("o OCR não leu a imagem:\n%s", saida)
		}
	})
}

// Não há PDF sintético com o xref que faz a biblioteca entrar em pânico em
// NumPage(): o pânico é simulado e o PDF sadio vai ao Poppler real.
func TestClasses_TabelaInconsistente(t *testing.T) {
	original := contarPaginasPDF
	t.Cleanup(func() { contarPaginasPDF = original })
	contarPaginasPDF = func(io.ReaderAt, int64) (*pdf.Reader, int, error) {
		panic("loading {8 0}: found {7 0}")
	}
	saida, d := rodarClasse(t, buildPDF(objetosTexto()), cfgPoppler("none"))
	conferirLeitura(t, saida, d, LeituraSegundoLeitor, 0)
	if !strings.Contains(saida, fraseClasses) {
		t.Errorf("o .txt não tem a frase:\n%s", saida)
	}
}
