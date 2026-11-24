package ocr

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"testing"
)

// leitorSimples esconde o WriterTo do leitor de baixo (io.Copy prefere o
// WriterTo da origem) e anota o maior pedido de leitura que recebeu.
type leitorSimples struct {
	r         io.Reader
	maiorPedi int
	lidos     int
}

func (l *leitorSimples) Read(p []byte) (int, error) {
	l.maiorPedi = max(l.maiorPedi, len(p))
	n, err := l.r.Read(p)
	l.lidos += n
	return n, err
}

func dadosDe(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

func TestLimitado_ReadFromRespeitaLimiteELeAteEOF(t *testing.T) {
	dados := dadosDe(100 << 10)
	src := &leitorSimples{r: bytes.NewReader(dados)}
	l := &limitado{max: maxStderr}

	n, err := io.Copy(l, src)
	if err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if n != int64(len(dados)) || src.lidos != len(dados) {
		t.Fatalf("leu %d (origem %d), quer %d: tem de ler até o EOF", n, src.lidos, len(dados))
	}
	if !bytes.Equal(l.buf.Bytes(), dados[:maxStderr]) {
		t.Fatalf("guardou %d bytes, quer os primeiros %d", l.buf.Len(), maxStderr)
	}
	if !l.cortou {
		t.Fatal("cortou deveria ser true")
	}
	// Sem ReadFrom, io.Copy pediria leituras de 32 KiB (o buffer que aloca).
	if src.maiorPedi > 1024 {
		t.Fatalf("maior leitura pedida %d bytes; quer no máximo 1024 (sem o buffer de 32 KiB do io.Copy)", src.maiorPedi)
	}
}

func TestLimitado_ReadFromAbaixoDoLimiteNaoCorta(t *testing.T) {
	dados := dadosDe(300)
	l := &limitado{max: maxStderr}
	if _, err := io.Copy(l, &leitorSimples{r: bytes.NewReader(dados)}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(l.buf.Bytes(), dados) || l.cortou {
		t.Fatalf("guardou %d bytes, cortou=%v; quer 300 e false", l.buf.Len(), l.cortou)
	}
}

// Pelo caminho do exec: um pipe do sistema (os.File.WriteTo cai no
// genericWriteTo, que usa o ReadFrom do destino).
func TestLimitado_ReadFromPorPipe(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	dados := dadosDe(256 << 10)
	go func() {
		pw.Write(dados)
		pw.Close()
	}()
	l := &limitado{max: maxStderr}
	n, err := io.Copy(l, pr)
	pr.Close()
	if err != nil || n != int64(len(dados)) {
		t.Fatalf("io.Copy = %d, %v; quer %d, nil", n, err, len(dados))
	}
	if !bytes.Equal(l.buf.Bytes(), dados[:maxStderr]) || !l.cortou {
		t.Fatalf("guardou %d bytes, cortou=%v", l.buf.Len(), l.cortou)
	}
}

// O stdout do Poppler usa o mesmo tipo com teto alto: tudo abaixo do teto fica.
func TestLimitado_ReadFromTetoAlto(t *testing.T) {
	dados := dadosDe(3<<20 + 17)
	l := &limitado{max: 64 << 20}
	if _, err := io.Copy(l, &leitorSimples{r: bytes.NewReader(dados)}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(l.buf.Bytes(), dados) || l.cortou {
		t.Fatalf("guardou %d de %d bytes, cortou=%v", l.buf.Len(), len(dados), l.cortou)
	}
}

func TestLimitado_ReadFromAlocaPouco(t *testing.T) {
	dados := dadosDe(64 << 10)
	src := &leitorSimples{}
	l := &limitado{}
	const vezes = 20
	copiar := func() {
		src.r = bytes.NewReader(dados)
		*l = limitado{max: maxStderr}
		io.Copy(l, src)
	}
	copiar() // aquece
	var antes, depois runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)
	for range vezes {
		copiar()
	}
	runtime.ReadMemStats(&depois)
	porCopia := (depois.TotalAlloc - antes.TotalAlloc) / vezes
	// O buffer do limitado (até maxStderr) e o leitor; nunca os 32 KiB do io.Copy.
	if porCopia > 8<<10 {
		t.Fatalf("%d bytes alocados por cópia; quer no máximo 8 KiB", porCopia)
	}
}
