package etl

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// htmlMaximo limita o HTML descomprimido que ExtractTextFromHTML guarda em
// memória para detectar o charset: um documento do eproc tem poucos MB, e uma
// entrada maior (bomba de ZIP) vira erro do documento.
const htmlMaximo = 32 << 20

// ExtractTextFromHTML extracts plain text from an HTML document reader, ignoring heads, scripts, and styles.
func ExtractTextFromHTML(r io.Reader) (string, error) {
	dados, err := io.ReadAll(io.LimitReader(r, htmlMaximo+1))
	if err != nil {
		return "", err
	}
	if len(dados) > htmlMaximo {
		return "", fmt.Errorf("HTML maior que %d MiB", htmlMaximo>>20)
	}
	z := html.NewTokenizer(bytes.NewReader(decodificarHTML(dados)))
	var sb strings.Builder
	ignoreContent := false
	profPre := 0

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return normalizarLinhas(sb.String()), nil
			}
			return "", z.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			name := strings.ToLower(t.Data)
			// <head> sem </head> é válido em HTML5: o body reabre o texto. Tag
			// autofechada não tem conteúdo a ignorar.
			switch {
			case name == "body":
				ignoreContent = false
			case (name == "script" || name == "style" || name == "head") && tt == html.StartTagToken:
				ignoreContent = true
			case name == "pre" && tt == html.StartTagToken:
				profPre++
			}
			marcarFronteira(&sb, name, true)

		case html.EndTagToken:
			t := z.Token()
			name := strings.ToLower(t.Data)
			if name == "script" || name == "style" || name == "head" {
				ignoreContent = false
			}
			if name == "pre" && profPre > 0 {
				profPre--
			}
			marcarFronteira(&sb, name, false)

		case html.TextToken:
			switch {
			case ignoreContent:
			case profPre > 0:
				// <pre> guarda as quebras de linha (um lançamento por linha).
				sb.WriteString(strings.ReplaceAll(z.Token().Data, "\r\n", "\n"))
			default:
				sb.WriteString(colapsarEspacos(z.Token().Data))
			}
		}
	}
}

// decodificarHTML converte os bytes do HTML para UTF-8 pelo charset declarado
// (meta/BOM). Sem declaração e com bytes que não são UTF-8 válido, assume
// Windows-1252, o que o eproc costuma emitir.
func decodificarHTML(dados []byte) []byte {
	enc, _, certo := charset.DetermineEncoding(dados, "")
	if !certo {
		// DetermineEncoding só olha o primeiro 1 KB: o documento inteiro decide.
		if utf8.Valid(dados) {
			enc = encoding.Nop
		} else {
			enc = charmap.Windows1252
		}
	}
	out, err := enc.NewDecoder().Bytes(dados)
	if err != nil {
		return dados
	}
	return out
}

// elementosDeBloco quebram a linha ao abrir e ao fechar.
var elementosDeBloco = map[string]bool{
	"p": true, "div": true, "br": true, "tr": true, "li": true, "table": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"section": true, "article": true, "blockquote": true, "pre": true, "hr": true,
}

// colapsarEspacos troca cada sequência de espaços e quebras do fonte por um
// espaço só, mantendo o espaço das bordas: texto inline segue na mesma linha.
func colapsarEspacos(d string) string {
	f := strings.Fields(d)
	if len(f) == 0 {
		if d == "" {
			return ""
		}
		return " "
	}
	out := strings.Join(f, " ")
	if r, _ := utf8.DecodeRuneInString(d); unicode.IsSpace(r) {
		out = " " + out
	}
	if r, _ := utf8.DecodeLastRuneInString(d); unicode.IsSpace(r) {
		out += " "
	}
	return out
}

// marcarFronteira escreve a quebra de linha (elemento de bloco) ou o espaço
// (célula de tabela) que a tag representa.
func marcarFronteira(sb *strings.Builder, nome string, abertura bool) {
	switch {
	case elementosDeBloco[nome]:
		sb.WriteByte('\n')
	case abertura && (nome == "td" || nome == "th"):
		sb.WriteByte(' ')
	}
}

// normalizarLinhas aplica trim a cada linha e descarta as vazias, de modo que
// sequências de quebras viram uma só. Texto não vazio termina em "\n".
func normalizarLinhas(s string) string {
	var out strings.Builder
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out.WriteString(l)
			out.WriteByte('\n')
		}
	}
	return out.String()
}
