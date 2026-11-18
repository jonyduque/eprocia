package etl

import (
	"fmt"
	"strings"
	"testing"
)

// tF abrevia um bloco de texto com a fonte /F1 a 12 pt na posição (x, y).
func tF(x, y float64, s string) string {
	return fmt.Sprintf("BT /F1 12 Tf %g %g Td (%s) Tj ET\n", x, y, s)
}

// paginaComXObjects é paginaComFonte com a Helvetica em /F1 e com
// /XObject << xobjects >> nos recursos da página; extras viram os objetos 6, 7...
func paginaComXObjects(stream, xobjects string, extras ...string) []string {
	return paginaComXObjectsMB(stream, xobjects, "/MediaBox [0 0 595 842]", extras...)
}

// paginaComXObjectsMB é paginaComXObjects com o /MediaBox dado (vazio: sem
// /MediaBox nenhum).
func paginaComXObjectsMB(stream, xobjects, mediaBox string, extras ...string) []string {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R %s /Resources << /Font << /F1 5 0 R >> /XObject << %s >> >> /Contents 4 0 R >>\nendobj", mediaBox, xobjects),
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream)+1, stream),
		"5 0 obj\n" + helvetica + "\nendobj",
	}
	for i, e := range extras {
		objects = append(objects, fmt.Sprintf("%d 0 obj\n%s\nendobj", 6+i, e))
	}
	return objects
}

// formXObject devolve o corpo de um objeto Form (dicionário com /Type /XObject
// /Subtype /Form /BBox [0 0 595 842], mais dicExtra, e o stream), pronto para
// ser passado em extras.
func formXObject(conteudo, dicExtra string) string {
	return fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 595 842] %s /Length %d >>\nstream\n%s\nendstream", dicExtra, len(conteudo)+1, conteudo)
}

// passadaDe roda interpretarPagina sobre a primeira página.
func passadaDe(t *testing.T, objects []string) passadaPagina {
	t.Helper()
	p, err := interpretarPagina(primeiraPagina(t, objects))
	if err != nil {
		t.Fatalf("interpretarPagina: %v", err)
	}
	return p
}

// conferirIgnorados compara a contagem de Forms ignorados por motivo.
func conferirIgnorados(t *testing.T, p passadaPagina, esperado map[motivoFormIgnorado]int) {
	t.Helper()
	for m := range numMotivosForm {
		if p.formsIgnorados[m] != esperado[m] {
			t.Errorf("formsIgnorados[%s] = %d, quer %d", m, p.formsIgnorados[m], esperado[m])
		}
	}
}

// ordemDe falha se as partes não aparecem em texto nessa ordem.
func ordemDe(t *testing.T, texto string, partes ...string) {
	t.Helper()
	pos := -1
	for _, p := range partes {
		i := strings.Index(texto, p)
		if i < 0 {
			t.Fatalf("%q não aparece em %q", p, texto)
		}
		if i <= pos {
			t.Fatalf("%q fora de ordem em %q", p, texto)
		}
		pos = i
	}
}

func objsCorpoEmForm() []string {
	corpo := tF(72, 700, "PRIMEIRA LINHA DO CORPO") + tF(72, 680, "SEGUNDA LINHA DO CORPO") + tF(72, 660, "TERCEIRA LINHA DO CORPO")
	return paginaComXObjects(tF(72, 60, "Legenda da figura")+"/Fm1 Do", "/Fm1 6 0 R", formXObject(corpo, ""))
}

func TestForm_CorpoEmFormMaisLegenda(t *testing.T) {
	texto := extrairComPrazo(t, objsCorpoEmForm())
	ordemDe(t, texto, "PRIMEIRA LINHA DO CORPO", "SEGUNDA LINHA DO CORPO", "TERCEIRA LINHA DO CORPO", "Legenda da figura")
}

func TestForm_MatrizDoForm(t *testing.T) {
	objs := paginaComXObjects("/Fm1 Do "+tF(72, 400, "ACIMA"), "/Fm1 6 0 R",
		formXObject(tF(72, 700, "ABAIXO"), "/Matrix [1 0 0 1 0 -600]"))
	ordemDe(t, extrairComPrazo(t, objs), "ACIMA", "ABAIXO")
}

func TestForm_EstadoRestaurado(t *testing.T) {
	objs := paginaComXObjects("/Fm1 Do "+tF(72, 400, "ESQUERDA"), "/Fm1 6 0 R",
		formXObject("1 0 0 1 300 -300 cm "+tF(72, 700, "DIREITA"), ""))
	if texto := extrairComPrazo(t, objs); texto != "ESQUERDA DIREITA" {
		t.Fatalf("texto = %q, quer %q", texto, "ESQUERDA DIREITA")
	}
}

func TestForm_RecursosDoForm(t *testing.T) {
	fonteZ := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding << /Type /Encoding /Differences [65 /Z] >> >>"
	objs := paginaComXObjects(tF(72, 700, "AAA")+"/Fm1 Do", "/Fm1 6 0 R",
		formXObject(tF(72, 600, "AAA"), "/Resources << /Font << /F1 7 0 R >> >>"), fonteZ)
	texto := extrairComPrazo(t, objs)
	if texto != "AAA\nZZZ" {
		t.Fatalf("texto = %q, quer %q", texto, "AAA\nZZZ")
	}
}

func TestForm_SemRecursosUsaOsDaPagina(t *testing.T) {
	objs := paginaComXObjects("/Fm1 Do", "/Fm1 6 0 R", formXObject(tF(72, 600, "HERDADO"), ""))
	if texto := extrairComPrazo(t, objs); texto != "HERDADO" {
		t.Fatalf("texto = %q, quer HERDADO", texto)
	}
}

func TestForm_CamadaOpcional(t *testing.T) {
	casos := []struct {
		nome, pagina, form, dicForm, quer string
	}{
		{"(a) Form com /OC", tF(72, 700, "PAGINA") + "/Fm1 Do", tF(72, 600, "DO FORM"), "/OC << /Type /OCG /Name (Oculta) >>", "PAGINA"},
		{"(b) Do dentro de /OC BDC", tF(72, 700, "PAGINA") + "/OC /MC0 BDC /Fm1 Do EMC", tF(72, 600, "DO FORM"), "", "PAGINA"},
		{"(c) texto do Form em /OC BDC", "/Fm1 Do", "/OC /MC0 BDC " + tF(72, 700, "OCULTO") + "EMC " + tF(72, 680, "VISIVEL"), "", "VISIVEL"},
		{"(d) texto da página em /OC BDC", "/OC /MC0 BDC " + tF(72, 700, "NA PAGINA") + "EMC /Fm1 Do", tF(72, 600, "DO FORM"), "", "NA PAGINA\nDO FORM"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			objs := paginaComXObjects(c.pagina, "/Fm1 6 0 R", formXObject(c.form, c.dicForm))
			if texto := extrairComPrazo(t, objs); texto != c.quer {
				t.Fatalf("texto = %q, quer %q", texto, c.quer)
			}
			conferirIgnorados(t, passadaDe(t, objs), nil)
		})
	}
}

func TestForm_ForaDaPagina(t *testing.T) {
	paginaA := "q 1 0 0 1 0 2000 cm /Fm1 Do Q " + tF(72, 700, "DENTRO")
	formA := formXObject(tF(72, 700, "FORA"), "")

	t.Run("(a) acima do MediaBox", func(t *testing.T) {
		if texto := extrairComPrazo(t, paginaComXObjects(paginaA, "/Fm1 6 0 R", formA)); texto != "DENTRO" {
			t.Fatalf("texto = %q, quer DENTRO", texto)
		}
	})
	t.Run("(b) MediaBox deslocado", func(t *testing.T) {
		objs := paginaComXObjectsMB("/Fm1 Do", "/Fm1 6 0 R", "/MediaBox [100 100 695 942]",
			formXObject(tF(50, 500, "FORA")+tF(650, 900, "DENTRO"), ""))
		if texto := extrairComPrazo(t, objs); texto != "DENTRO" {
			t.Fatalf("texto = %q, quer DENTRO", texto)
		}
	})
	t.Run("(c) sem MediaBox", func(t *testing.T) {
		objs := paginaComXObjectsMB(paginaA, "/Fm1 6 0 R", "", formA)
		ordemDe(t, extrairComPrazo(t, objs), "FORA", "DENTRO")
		if p := passadaDe(t, objs); p.temMediaBox {
			t.Fatalf("temMediaBox = true sem /MediaBox")
		}
	})
}

func TestForm_PintadoDuasVezes(t *testing.T) {
	form := formXObject(tF(72, 700, "CARIMBO"), "")
	casos := []struct {
		nome, pagina string
		vezes        int
	}{
		{"(a) mesmo lugar", "/Fm1 Do /Fm1 Do", 1},
		{"(b) lugares diferentes", "/Fm1 Do q 1 0 0 1 0 -100 cm /Fm1 Do Q", 2},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			texto := extrairComPrazo(t, paginaComXObjects(c.pagina, "/Fm1 6 0 R", form))
			if n := strings.Count(texto, "CARIMBO"); n != c.vezes {
				t.Fatalf("CARIMBO aparece %d vezes em %q, quer %d", n, texto, c.vezes)
			}
		})
	}
}

func TestForm_Recursao(t *testing.T) {
	t.Run("(a) Form chama a si mesmo", func(t *testing.T) {
		objs := paginaComXObjects("/Fm1 Do", "/Fm1 6 0 R",
			formXObject(tF(72, 700, "EU")+"/Fm1 Do", "/Resources << /Font << /F1 5 0 R >> /XObject << /Fm1 6 0 R >> >>"))
		texto := extrairComPrazo(t, objs)
		if n := strings.Count(texto, "EU"); n != 1 {
			t.Fatalf("EU aparece %d vezes em %q, quer 1", n, texto)
		}
		if p := passadaDe(t, objs); p.formsIgnorados[formIgnoradoProfundidade] < 1 {
			t.Fatalf("formsIgnorados[profundidade] = %d, quer >= 1", p.formsIgnorados[formIgnoradoProfundidade])
		}
	})
	t.Run("(b) Fm1 chama Fm2, que chama Fm1", func(t *testing.T) {
		objs := paginaComXObjects("/Fm1 Do", "/Fm1 6 0 R",
			formXObject(tF(72, 700, "UM")+"/Fm2 Do", "/Resources << /Font << /F1 5 0 R >> /XObject << /Fm2 7 0 R >> >>"),
			formXObject(tF(72, 600, "DOIS")+"/Fm1 Do", "/Resources << /Font << /F1 5 0 R >> /XObject << /Fm1 6 0 R >> >>"))
		texto := extrairComPrazo(t, objs)
		for _, s := range []string{"UM", "DOIS"} {
			if n := strings.Count(texto, s); n != 1 {
				t.Fatalf("%s aparece %d vezes em %q, quer 1", s, n, texto)
			}
		}
		if p := passadaDe(t, objs); p.formsIgnorados[formIgnoradoProfundidade] < 1 {
			t.Fatalf("formsIgnorados[profundidade] = %d, quer >= 1", p.formsIgnorados[formIgnoradoProfundidade])
		}
	})
}

func TestForm_Limite256PorPagina(t *testing.T) {
	var sb strings.Builder
	for i := range 300 {
		fmt.Fprintf(&sb, "q 1 0 0 1 %d 0 cm /Fm1 Do Q\n", i)
	}
	p := passadaDe(t, paginaComXObjects(sb.String(), "/Fm1 6 0 R", formXObject(tF(72, 700, "X"), "")))
	if p.trechosForm != formMaxPorPagina {
		t.Fatalf("trechosForm = %d, quer %d", p.trechosForm, formMaxPorPagina)
	}
	conferirIgnorados(t, p, map[motivoFormIgnorado]int{formIgnoradoLimite: 300 - formMaxPorPagina})
}

func TestForm_StreamIlegivel(t *testing.T) {
	objs := paginaComXObjects(tF(72, 700, "DA PAGINA")+"/Fm1 Do", "/Fm1 6 0 R", formXObject("BT /F1 12 Tf (sem fecho", ""))
	if texto := extrairComPrazo(t, objs); texto != "DA PAGINA" {
		t.Fatalf("texto = %q, quer DA PAGINA", texto)
	}
	conferirIgnorados(t, passadaDe(t, objs), map[motivoFormIgnorado]int{formIgnoradoStreamIlegivel: 1})
}

func TestForm_FiltroNaoSuportado(t *testing.T) {
	objs := paginaComXObjects(tF(72, 700, "DA PAGINA")+"/Fm1 Do", "/Fm1 6 0 R",
		formXObject("BT /F1 12 Tf (sem fecho", "/Filter /DCTDecode"))
	if texto := extrairComPrazo(t, objs); texto != "DA PAGINA" {
		t.Fatalf("texto = %q, quer DA PAGINA", texto)
	}
	conferirIgnorados(t, passadaDe(t, objs), map[motivoFormIgnorado]int{formIgnoradoPanico: 1})
}

func TestForm_DoQueNaoEForm(t *testing.T) {
	imagem := "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 1 >>\nstream\nA\nendstream"
	objs := paginaComXObjects(tF(72, 700, "TEXTO")+"/Im9 Do /Im1 Do", "/Im1 6 0 R", imagem)
	if texto := extrairComPrazo(t, objs); texto != "TEXTO" {
		t.Fatalf("texto = %q, quer TEXTO", texto)
	}
}

func TestForm_ContaTrechos(t *testing.T) {
	if p := passadaDe(t, objsCorpoEmForm()); p.trechosForm != 3 {
		t.Fatalf("com Form: trechosForm = %d, quer 3", p.trechosForm)
	}
	p := passadaDe(t, paginaComFonte(tF(72, 700, "SEM FORM"), helvetica))
	if p.trechosForm != 0 {
		t.Fatalf("sem Form: trechosForm = %d, quer 0", p.trechosForm)
	}
	conferirIgnorados(t, p, nil)
	if len(p.trechos) != 1 || !p.temMediaBox {
		t.Fatalf("sem Form: %d trechos, temMediaBox %v; quer 1 e true", len(p.trechos), p.temMediaBox)
	}
}

// paginaComMediaBox monta uma página com os fragmentos dados no dicionário da
// página e no do /Pages pai.
func paginaComMediaBox(daPagina, doPai string) []string {
	return []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 %s >>\nendobj", doPai),
		fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R %s /Resources << >> /Contents 4 0 R >>\nendobj", daPagina),
		"4 0 obj\n<< /Length 1 >>\nstream\n\nendstream\nendobj",
	}
}

func TestMediaBoxDe(t *testing.T) {
	casos := []struct {
		nome, daPagina, doPai string
		quer                  caixa
		ok                    bool
	}{
		{"próprio", "/MediaBox [0 0 595 842]", "", caixa{0, 0, 595, 842}, true},
		{"herdado do /Parent", "", "/MediaBox [0 0 612 792]", caixa{0, 0, 612, 792}, true},
		{"ausente", "", "", caixa{}, false},
		{"área zero", "/MediaBox [0 0 0 0]", "", caixa{}, false},
		{"invertido", "/MediaBox [595 842 0 0]", "", caixa{0, 0, 595, 842}, true},
		{"três números", "/MediaBox [0 0 595]", "", caixa{}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, ok := mediaBoxDe(primeiraPagina(t, paginaComMediaBox(c.daPagina, c.doPai)))
			if ok != c.ok || (ok && got != c.quer) {
				t.Fatalf("mediaBoxDe = %+v, %v; quer %+v, %v", got, ok, c.quer, c.ok)
			}
		})
	}
}

func TestForm_MatrizInvalida(t *testing.T) {
	conteudo := tF(72, 700, "PRIMEIRA") + tF(72, 600, "SEGUNDA")
	for _, mat := range []string{"/Matrix [1 0 0 1 /X 0]", "/Matrix [1 0 0]", "/Matrix [1 0 0.25 /X 0 0]", "/Matrix [1 0 0 1 0 0 0]"} {
		t.Run(mat, func(t *testing.T) {
			objs := paginaComXObjects("/Fm1 Do", "/Fm1 6 0 R", formXObject(conteudo, mat))
			if texto := extrairComPrazo(t, objs); texto != "PRIMEIRA\nSEGUNDA" {
				t.Fatalf("texto = %q, quer %q", texto, "PRIMEIRA\nSEGUNDA")
			}
		})
	}
}

// O "end" sem "begin" faz o Interpret da biblioteca entrar em pânico depois de
// o stream ter passado pelo lerStream.
func TestForm_PanicoNoMeioRestauraEstado(t *testing.T) {
	objs := paginaComXObjects(tF(72, 400, "ANTES")+"/Fm1 Do "+tF(300, 400, "DEPOIS"), "/Fm1 6 0 R",
		formXObject("1 0 0 1 300 -300 cm "+tF(72, 700, "PERDIDO")+"end", ""))
	if texto := extrairComPrazo(t, objs); texto != "ANTES DEPOIS" {
		t.Fatalf("texto = %q, quer %q", texto, "ANTES DEPOIS")
	}
	conferirIgnorados(t, passadaDe(t, objs), map[motivoFormIgnorado]int{formIgnoradoPanico: 1})
}

func TestForm_QSoltoEBDCSemFecho(t *testing.T) {
	t.Run("Q a mais", func(t *testing.T) {
		objs := paginaComXObjects(tF(72, 400, "ANTES")+"q 1 0 0 1 0 0 cm /Fm1 Do "+tF(300, 400, "DEPOIS")+"Q", "/Fm1 6 0 R",
			formXObject("Q Q 1 0 0 1 0 -300 cm "+tF(72, 600, "DOFORM"), ""))
		if texto := extrairComPrazo(t, objs); texto != "ANTES DEPOIS\nDOFORM" {
			t.Fatalf("texto = %q, quer %q", texto, "ANTES DEPOIS\nDOFORM")
		}
	})
	t.Run("BDC sem EMC", func(t *testing.T) {
		objs := paginaComXObjects(tF(72, 400, "ANTES")+"/Fm1 Do /Fm2 Do "+tF(300, 400, "DEPOIS"), "/Fm1 6 0 R /Fm2 7 0 R",
			formXObject("/OC /Camada BDC "+tF(72, 600, "OCULTO"), ""), formXObject(tF(72, 500, "SEGUNDO"), ""))
		if texto := extrairComPrazo(t, objs); texto != "SEGUNDO\nANTES DEPOIS" {
			t.Fatalf("texto = %q, quer %q", texto, "SEGUNDO\nANTES DEPOIS")
		}
	})
}
