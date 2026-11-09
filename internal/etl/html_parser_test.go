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

func TestExtractTextFromHTML_InlineNaMesmaLinha(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader(
		"<div><b>AUTOR</b><span>:</span> <span>FULANO DE TAL</span></div><p>Segunda linha</p>"))
	if strings.TrimSpace(got) != "AUTOR: FULANO DE TAL\nSegunda linha" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractTextFromHTML_BrELinhaDeTabela(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader(
		"<p>um<br>dois</p><table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>"))
	if strings.TrimSpace(got) != "um\ndois\na b\nc d" {
		t.Fatalf("got %q", got)
	}
}
