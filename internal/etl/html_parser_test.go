package etl

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractTextFromHTML_Latin1ComMeta(t *testing.T) {
	src := []byte("<html><head><meta charset=\"iso-8859-1\"></head><body><p>Poder Judici\xe1rio</p></body></html>")
	got, err := ExtractTextFromHTML(bytes.NewReader(src))
	if err != nil || !strings.Contains(got, "Poder Judiciário") || strings.ContainsRune(got, '�') {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestExtractTextFromHTML_SemMetaBytesInvalidosViram1252(t *testing.T) {
	src := []byte("<html><body><p>R\xc9U</p></body></html>")
	got, _ := ExtractTextFromHTML(bytes.NewReader(src))
	if !strings.Contains(got, "RÉU") {
		t.Fatalf("got %q", got)
	}
}

func TestExtractTextFromHTML_UTF8Intacto(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader("<p>Ação — §1º</p>"))
	if !strings.Contains(got, "Ação — §1º") {
		t.Fatalf("got %q", got)
	}
}
