package etl

import (
	"fmt"
	"strings"
	"testing"
)

// streamObj monta o objeto n com um stream de conteúdo.
func streamObj(n int, conteudo string) string {
	return fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", n, len(conteudo)+1, conteudo)
}

// paginaComObjetoTrocado é uma página com /Contents em array (sem a volta ao
// GetTextByRow) cujo objeto 6 está gravado no arquivo como "9 0 obj": a
// tabela de objetos aponta para ele, e a biblioteca entra em pânico ao
// resolvê-lo ("loading {6 0}: found {9 0}"). extraPagina entra no dicionário
// da página e objeto6 é o corpo do objeto 6.
func paginaComObjetoTrocado(extraPagina, antes, depois, objeto6 string) []string {
	return []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R %s /Resources << /Font << /F1 5 0 R >> /XObject << /Fm1 6 0 R >> >> /Contents [4 0 R 7 0 R] >>\nendobj", extraPagina),
		streamObj(4, antes),
		"5 0 obj\n" + helvetica + "\nendobj",
		"9 0 obj\n" + objeto6 + "\nendobj",
		streamObj(7, depois),
	}
}

func conferirTextoInteiro(t *testing.T, objs []string) {
	t.Helper()
	runner, _ := ocrFalso(ocrFalsoTexto, nil)
	c := conteudoDe(paginaUm(t, parsePagesWithDeadline(t, objs, runner)))
	if strings.Contains(c, "[ERRO ao ler página") {
		t.Fatalf("página perdida: %q", c)
	}
	for _, l := range []string{"ANTES DO XOBJECT", "DEPOIS DO XOBJECT"} {
		if !strings.Contains(c, l) {
			t.Errorf("%q ausente em %q", l, c)
		}
	}
}

func TestObjetoQuebrado_XObjectNaoDerrubaAPagina(t *testing.T) {
	objs := paginaComObjetoTrocado("/MediaBox [0 0 595 842]",
		corpoSintetico(10, 780)+tF(72, 400, "ANTES DO XOBJECT")+"/Fm1 Do",
		tF(72, 300, "DEPOIS DO XOBJECT"),
		formXObject(tF(72, 600, "DO FORM"), ""))
	conferirTextoInteiro(t, objs)
	p := passadaDe(t, objs)
	conferirIgnorados(t, p, nil)
	if !p.temMediaBox {
		t.Errorf("temMediaBox = false")
	}
}

func TestObjetoQuebrado_MediaBoxNaoDerrubaAPagina(t *testing.T) {
	objs := paginaComObjetoTrocado("/MediaBox 6 0 R",
		corpoSintetico(10, 780)+tF(72, 400, "ANTES DO XOBJECT"),
		tF(72, 300, "DEPOIS DO XOBJECT"),
		"[0 0 595 842]")
	conferirTextoInteiro(t, objs)
	if p := passadaDe(t, objs); p.temMediaBox {
		t.Errorf("temMediaBox = true com o MediaBox quebrado")
	}
}

// O Form registra uma imagem útil e entra em pânico no "end" sem "begin": a
// página fica como se o Form não existisse, sem a imagem dele.
func TestForm_PanicoDescartaAsImagens(t *testing.T) {
	objs := paginaComXObjects(tF(72, 400, "ANTES")+"/Fm1 Do", "/Fm1 6 0 R",
		formXObject("q 595 0 0 842 0 0 cm /Im1 Do Q end", "/Resources << /XObject << /Im1 7 0 R >> >>"),
		imagemXObject(1240, 1754))
	p := passadaDe(t, objs)
	conferirIgnorados(t, p, map[motivoFormIgnorado]int{formIgnoradoPanico: 1})
	if len(p.imagens) != 0 {
		t.Errorf("imagens = %d, quer 0", len(p.imagens))
	}
	// Controle: sem o pânico, a mesma imagem conta.
	ok := paginaComXObjects(tF(72, 400, "ANTES")+"/Fm1 Do", "/Fm1 6 0 R",
		formXObject("q 595 0 0 842 0 0 cm /Im1 Do Q", "/Resources << /XObject << /Im1 7 0 R >> >>"),
		imagemXObject(1240, 1754))
	if n := len(passadaDe(t, ok).imagens); n != 1 {
		t.Errorf("controle: imagens = %d, quer 1", n)
	}
}
