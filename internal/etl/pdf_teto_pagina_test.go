package etl

import (
	"fmt"
	"strings"
	"testing"
)

// trocarLimite troca um limite do pacote durante o teste.
func trocarLimite(t *testing.T, alvo *int, valor int) {
	t.Helper()
	antigo := *alvo
	*alvo = valor
	t.Cleanup(func() { *alvo = antigo })
}

// Um Form pequeno chamado muitas vezes, ou Forms grandes, não podem fazer a
// página ler mais bytes que o teto: o que passa dele é ignorado como limite.
func TestForm_TetoDeBytesPorPagina(t *testing.T) {
	conteudo := tF(72, 700, "X")
	tamanho := len(conteudo) + 1 // o stream leva o \n antes de endstream
	trocarLimite(t, &formBytesMaxPorPagina, 2*tamanho+tamanho/2)

	var sb strings.Builder
	for i := range 10 {
		fmt.Fprintf(&sb, "q 1 0 0 1 %d 0 cm /Fm1 Do Q\n", i)
	}
	p := passadaDe(t, paginaComXObjects(sb.String(), "/Fm1 6 0 R", formXObject(conteudo, "")))
	if p.trechosForm != 2 {
		t.Fatalf("trechosForm = %d, quer 2", p.trechosForm)
	}
	conferirIgnorados(t, p, map[motivoFormIgnorado]int{formIgnoradoLimite: 8})
}

// Um stream de Form ilegível também gasta o teto: senão milhares de chamadas
// a ele leriam sem fim.
func TestForm_StreamIlegivelGastaOTeto(t *testing.T) {
	ilegivel := "BT /F1 12 Tf (sem fecho"
	tamanho := len(ilegivel) + 1
	trocarLimite(t, &formBytesMaxPorPagina, 3*tamanho)

	var sb strings.Builder
	for range 10 {
		sb.WriteString("/Fm1 Do\n")
	}
	p := passadaDe(t, paginaComXObjects(sb.String(), "/Fm1 6 0 R", formXObject(ilegivel, "")))
	conferirIgnorados(t, p, map[motivoFormIgnorado]int{formIgnoradoStreamIlegivel: 3, formIgnoradoLimite: 7})
}

// Os trechos de texto da página (da própria página e dos Forms) param no teto.
func TestTexto_TetoDeTrechosPorPagina(t *testing.T) {
	trocarLimite(t, &trechosMaxPorPagina, 5)
	var sb strings.Builder
	for i := range 10 {
		sb.WriteString(tF(72, float64(800-20*i), fmt.Sprintf("LINHA%d", i)))
	}
	p := passadaDe(t, paginaComXObjects(sb.String(), ""))
	if len(p.trechos) != 5 {
		t.Fatalf("trechos = %d, quer 5", len(p.trechos))
	}
}

// /Contents em array: a soma dos streams passa pelo teto, não só cada um.
func TestCheckContentStream_TetoDaSomaDoArray(t *testing.T) {
	conteudo := tF(72, 700, "PARTE")
	tamanho := len(conteudo) + 1
	objetos := func(n int) []string {
		var refs []string
		objs := []string{
			"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
			"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		}
		var streams []string
		for i := range n {
			refs = append(refs, fmt.Sprintf("%d 0 R", 5+i))
			streams = append(streams, fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", 5+i, tamanho, conteudo))
		}
		objs = append(objs,
			fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents [%s] >>\nendobj", strings.Join(refs, " ")),
			"4 0 obj\n"+helvetica+"\nendobj")
		return append(objs, streams...)
	}
	trocarLimite(t, &paginaBytesMaximo, 3*tamanho)
	if _, err := checkContentStream(primeiraPagina(t, objetos(3))); err != nil {
		t.Fatalf("3 streams no teto: %v", err)
	}
	if _, err := checkContentStream(primeiraPagina(t, objetos(4))); err == nil {
		t.Fatalf("4 streams acima do teto passaram")
	}
}
