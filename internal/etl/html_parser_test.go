package etl

import (
	"bytes"
	"io"
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

func TestExtractTextFromHTML_UTF8SemMetaDepoisDe1KB(t *testing.T) {
	src := "<html><head><style>" + strings.Repeat("a{color:red}", 120) + "</style></head><body><p>Contestação</p></body></html>"
	got, err := ExtractTextFromHTML(strings.NewReader(src))
	if err != nil || !strings.Contains(got, "Contestação") {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestExtractTextFromHTML_PreMantemQuebras(t *testing.T) {
	src := "<html><body><pre>01/01 Saque 10,00\r\n02/01 Depósito 20,00\n  03/01 Tarifa 1,00</pre></body></html>"
	got, _ := ExtractTextFromHTML(strings.NewReader(src))
	want := "01/01 Saque 10,00\n02/01 Depósito 20,00\n03/01 Tarifa 1,00\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractTextFromHTML_HeadSemFechar(t *testing.T) {
	src := "<html><head><title>x</title><body><p>Decisão</p></body></html>"
	got, _ := ExtractTextFromHTML(strings.NewReader(src))
	if got != "Decisão\n" {
		t.Errorf("got %q", got)
	}
}

// Uma entrada HTML enorme (bomba de ZIP) vira erro em vez de ocupar a memória.
func TestExtractTextFromHTML_MaiorQueOLimiteViraErro(t *testing.T) {
	r := io.LimitReader(zeroReader{}, htmlMaximo+2)
	if _, err := ExtractTextFromHTML(r); err == nil || !strings.Contains(err.Error(), "maior que") {
		t.Fatalf("esperava erro de tamanho, obtive %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}
