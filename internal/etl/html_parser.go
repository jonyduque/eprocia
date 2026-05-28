package etl

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// ExtractTextFromHTML extracts plain text from an HTML document reader, ignoring heads, scripts, and styles.
func ExtractTextFromHTML(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)
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
