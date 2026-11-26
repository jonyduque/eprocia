package etl

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Geradores dos PDFs sintéticos de pdf_classes_test.go. Todo conteúdo é fictício.

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
