package etl

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/charmap"
)

// ExtractTextFromHTML extracts plain text from an HTML document reader, ignoring heads, scripts, and styles.
func ExtractTextFromHTML(r io.Reader) (string, error) {
	dados, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	z := html.NewTokenizer(bytes.NewReader(decodificarHTML(dados)))
	var sb strings.Builder
	ignoreContent := false

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				// Normalize multiple newlines and spaces to save tokens
				return sb.String(), nil
			}
			return "", z.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			name := strings.ToLower(t.Data)
			if name == "script" || name == "style" || name == "head" {
				ignoreContent = true
			}

		case html.EndTagToken:
			t := z.Token()
			name := strings.ToLower(t.Data)
			if name == "script" || name == "style" || name == "head" {
				ignoreContent = false
			}

		case html.TextToken:
			if !ignoreContent {
				text := strings.TrimSpace(z.Token().Data)
				if text != "" {
					sb.WriteString(text)
					sb.WriteString("\n")
				}
			}
		}
	}
}

// decodificarHTML converte os bytes do HTML para UTF-8 pelo charset declarado
// (meta/BOM). Sem declaração e com bytes que não são UTF-8 válido, assume
// Windows-1252, o que o eproc costuma emitir.
func decodificarHTML(dados []byte) []byte {
	enc, _, certo := charset.DetermineEncoding(dados, "")
	if !certo && !utf8.Valid(dados) {
		enc = charmap.Windows1252
	}
	out, err := enc.NewDecoder().Bytes(dados)
	if err != nil {
		return dados
	}
	return out
}
