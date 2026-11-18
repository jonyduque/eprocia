package etl

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

var a4 = caixa{0, 0, 595, 842}

// comPrazo roda f e falha o teste se não voltar em 5 s.
func comPrazo(t *testing.T, f func()) {
	t.Helper()
	feito := make(chan struct{})
	go func() {
		defer close(feito)
		f()
	}()
	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal("não terminou no prazo")
	}
}

func img(w, h int, ctm ...float64) imagemColocada {
	var m matriz
	copy(m[:], ctm)
	return imagemColocada{ctm: m, largPx: w, altPx: h}
}

func perto(x, quer, tol float64) bool { return x >= quer-tol && x <= quer+tol }

func TestMedir_SemImagem(t *testing.T) {
	var textos []caixa
	for i := range 40 {
		y := 800 - float64(i)*18
		textos = append(textos, caixa{50, y, 500, y + 10})
	}
	m := medirPagina(a4, nil, textos)
	if !m.valida || m.fImg != 0 || m.fTxt != 0 || m.fDesc != 0 {
		t.Fatalf("medirPagina = %+v; quer valida e frações 0", m)
	}
}

func TestMedir_PaginaInteira(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1240, 1754, 595, 0, 0, 842, 0, 0)}, nil)
	if !m.valida || m.fImg < 0.99 || m.fDesc < 0.99 || m.fTxt != 0 {
		t.Fatalf("medirPagina = %+v; quer fImg e fDesc >= 0,99 e fTxt = 0", m)
	}
}

func TestMedir_Espelhada(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1240, 1754, 595, 0, 0, -842, 0, 842)}, nil)
	if !m.valida || m.fImg < 0.99 || m.fDesc < 0.99 || m.fTxt != 0 {
		t.Fatalf("medirPagina = %+v; quer fImg e fDesc >= 0,99 e fTxt = 0", m)
	}
}

func TestMedir_Girada90(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1754, 1240, 0, 842, -595, 0, 595, 0)}, nil)
	if m.fImg < 0.99 {
		t.Fatalf("fImg = %v, quer >= 0,99", m.fImg)
	}
}

func TestMedir_ImagemInutil(t *testing.T) {
	for _, im := range []imagemColocada{img(1, 1, 595, 0, 0, 842, 0, 0), img(100, 100, 595, 0, 0, 842, 0, 0)} {
		if m := medirPagina(a4, []imagemColocada{im}, nil); m.fImg != 0 || !m.valida {
			t.Errorf("%dx%d px: %+v; quer valida e fImg = 0", im.largPx, im.altPx, m)
		}
	}
}

func TestMedir_CTMSingular(t *testing.T) {
	ims := []imagemColocada{img(1240, 1754, 0, 0, 0, 0, 0, 0), img(1240, 1754, 595, 0, 595, 0, 0, 0)}
	for _, im := range ims {
		if m := medirPagina(a4, []imagemColocada{im}, nil); m.fImg != 0 || m.fDesc != 0 {
			t.Errorf("CTM %v: %+v; quer fImg = fDesc = 0", im.ctm, m)
		}
		if n := linhasAcimaDasImagens([]trecho{{texto: "X", y: 900, tamanho: 10}}, []imagemColocada{im}); n != 0 {
			t.Errorf("CTM %v: linhasAcima = %d, quer 0", im.ctm, n)
		}
	}
}

func TestMedir_MediaBoxInvalida(t *testing.T) {
	cheia := []imagemColocada{img(1240, 1754, 595, 0, 0, 842, 0, 0)}
	for _, mb := range []caixa{{0, 0, 0, 842}, {0, 0, 595, 0}, {595, 842, 0, 0}} {
		if m := medirPagina(mb, cheia, nil); m.valida {
			t.Errorf("MediaBox %v: valida = true", mb)
		}
	}
}

func TestMedir_MeiaPaginaTextoLonge(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1240, 877, 595, 0, 0, 421, 0, 0)}, []caixa{{50, 800, 300, 810}})
	if m.fImg < 0.48 || m.fImg > 0.52 || m.fDesc < 0.48 || m.fDesc > 0.52 {
		t.Fatalf("medirPagina = %+v; quer fImg e fDesc entre 0,48 e 0,52", m)
	}
}

func TestMedir_TextoEncostado(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1240, 877, 595, 0, 0, 421, 0, 0)}, []caixa{{0, 423, 595, 433}})
	if !(m.fDesc < m.fImg) {
		t.Fatalf("medirPagina = %+v; quer fDesc < fImg", m)
	}
}

func TestMedir_TextoSobreAImagem(t *testing.T) {
	var textos []caixa
	for i := range 30 {
		y := 40 + float64(i)*26
		textos = append(textos, caixa{0, y, 595, y + 10})
	}
	m := medirPagina(a4, []imagemColocada{img(1240, 1754, 595, 0, 0, 842, 0, 0)}, textos)
	if m.fImg < 0.99 || m.fDesc >= 0.60 {
		t.Fatalf("medirPagina = %+v; quer fImg >= 0,99 e fDesc < 0,60", m)
	}
}

func TestMedir_OrigemDeslocada(t *testing.T) {
	m := medirPagina(caixa{100, 100, 695, 942}, []imagemColocada{img(1240, 1754, 595, 0, 0, 842, 100, 100)}, nil)
	if m.fImg < 0.99 {
		t.Fatalf("fImg = %v, quer >= 0,99", m.fImg)
	}
}

func TestMedir_ImagemParteFora(t *testing.T) {
	m := medirPagina(a4, []imagemColocada{img(1860, 1754, 893, 0, 0, 842, 297, 0)}, nil)
	if m.fImg < 0.48 || m.fImg > 0.52 {
		t.Fatalf("fImg = %v, quer entre 0,48 e 0,52", m.fImg)
	}
}

func TestLinhasAcima(t *testing.T) {
	baixo := img(1240, 877, 595, 0, 0, 421, 0, 0)
	linhas := func(ys ...float64) []trecho {
		var ts []trecho
		for i, y := range ys {
			ts = append(ts, trecho{texto: fmt.Sprintf("L%d", i), x: 72, y: y, fimX: 100, tamanho: 10})
		}
		return ts
	}
	casos := []struct {
		nome    string
		imagens []imagemColocada
		trechos []trecho
		quer    int
	}{
		{"sem imagem", nil, linhas(800, 780, 700), 0},
		{"imagem na metade de baixo", []imagemColocada{baixo}, linhas(800, 780, 700, 300, 100), 3},
		{"linha de base no topo exato", []imagemColocada{baixo}, linhas(421, 300), 1},
		{"espelhada", []imagemColocada{img(1240, 877, 595, 0, 0, -421, 0, 421)}, linhas(800, 780, 700, 300, 100), 3},
		{"duas imagens", []imagemColocada{baixo, img(413, 207, 200, 0, 0, 100, 0, 500)}, linhas(800, 780, 700, 500, 300), 3},
		{"imagem inútil", []imagemColocada{img(1, 1, 595, 0, 0, 842, 0, 0)}, linhas(800, 780), 0},
		{"dois trechos na mesma linha", []imagemColocada{baixo}, append(linhas(800, 300), trecho{texto: "B", x: 300, y: 800, fimX: 320, tamanho: 10}), 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if n := linhasAcimaDasImagens(c.trechos, c.imagens); n != c.quer {
				t.Fatalf("linhasAcimaDasImagens = %d, quer %d", n, c.quer)
			}
		})
	}
}

func TestImagens_Registro(t *testing.T) {
	imagem := imagemXObject(400, 300)
	t.Run("Do na página", func(t *testing.T) {
		p := passadaDe(t, paginaComXObjects("q 200 0 0 100 50 60 cm /Im1 Do Q", "/Im1 6 0 R", imagem))
		if len(p.imagens) != 1 || p.imagens[0] != img(400, 300, 200, 0, 0, 100, 50, 60) {
			t.Fatalf("imagens = %+v", p.imagens)
		}
	})
	t.Run("dentro de Form com /Matrix", func(t *testing.T) {
		p := passadaDe(t, paginaComXObjects("q 1 0 0 1 10 20 cm /Fm1 Do Q", "/Fm1 6 0 R",
			formXObject("q 200 0 0 100 0 0 cm /Im0 Do Q", "/Matrix [1 0 0 1 5 5] /Resources << /XObject << /Im0 7 0 R >> >>"), imagem))
		if len(p.imagens) != 1 || p.imagens[0] != img(400, 300, 200, 0, 0, 100, 15, 25) {
			t.Fatalf("imagens = %+v", p.imagens)
		}
	})
	t.Run("inline", func(t *testing.T) {
		p := passadaDe(t, paginaComXObjects("q 595 0 0 800 0 42 cm\nBI /W 620 /H 830 /CS /G /BPC 8 /F /AHx ID\nFFFFFF00>\nEI Q", ""))
		if len(p.imagens) != 1 || p.imagens[0] != img(620, 830, 595, 0, 0, 800, 0, 42) {
			t.Fatalf("imagens = %+v", p.imagens)
		}
	})
	t.Run("inline com /Width e /Height", func(t *testing.T) {
		p := passadaDe(t, paginaComXObjects("BI /Width 30 /Height 20 /BPC 8 /CS /G ID\nFF>\nEI", ""))
		if len(p.imagens) != 1 || p.imagens[0].largPx != 30 || p.imagens[0].altPx != 20 {
			t.Fatalf("imagens = %+v", p.imagens)
		}
	})
	casosSem := []struct{ nome, pagina, form, dicForm string }{
		{"Do em /OC BDC na página", "/OC /MC0 BDC /Im1 Do EMC", "", ""},
		{"Form com /OC", "/Fm1 Do", "/Im1 Do", "/OC << /Type /OCG /Name (Oculta) >>"},
		{"Do em /OC BDC no Form", "/Fm1 Do", "/OC /MC0 BDC /Im1 Do EMC", ""},
		{"inline em /OC BDC", "/OC /MC0 BDC BI /W 620 /H 830 ID\nFF>\nEI EMC", "", ""},
	}
	for _, c := range casosSem {
		t.Run(c.nome, func(t *testing.T) {
			p := passadaDe(t, paginaComXObjects(c.pagina, "/Im1 6 0 R /Fm1 7 0 R", imagem, formXObject(c.form, c.dicForm)))
			if len(p.imagens) != 0 {
				t.Fatalf("imagens = %+v, quer nenhuma", p.imagens)
			}
		})
	}
	t.Run("dimensões inválidas viram 0 px e a imagem não entra", func(t *testing.T) {
		ruim := "<< /Type /XObject /Subtype /Image /Width /X /Height -5 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 1 >>\nstream\nA\nendstream"
		p := passadaDe(t, paginaComXObjects("q 595 0 0 842 0 0 cm /Im1 Do Q", "/Im1 6 0 R", ruim))
		if len(p.imagens) != 0 {
			t.Fatalf("imagens = %+v, quer nenhuma (0 px é imagem inútil)", p.imagens)
		}
		if m := p.metricas(); m.fImg != 0 {
			t.Fatalf("fImg = %v, quer 0", m.fImg)
		}
	})
}

func TestImagens_TetoPorPagina(t *testing.T) {
	stream := strings.Repeat("/Im1 Do\n", 5000)
	objs := paginaComXObjects(stream, "/Im1 6 0 R", imagemXObject(400, 300))
	var p passadaPagina
	comPrazo(t, func() { p = passadaDe(t, objs) })
	if len(p.imagens) != imagensMaxPorPagina {
		t.Fatalf("len(imagens) = %d, quer %d", len(p.imagens), imagensMaxPorPagina)
	}
}

func TestExtrairNativoSeguro_LevaFormsIgnorados(t *testing.T) {
	form := formXObject("BT /F1 12 Tf (sem fecho", "")
	stream := tF(72, 700, "DA PAGINA") + "/Fm1 Do"
	casos := []struct {
		nome, mediaBox string
		valida         bool
	}{
		{"com MediaBox", "/MediaBox [0 0 595 842]", true},
		{"sem MediaBox", "", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, m, err := extrairNativoSeguro(primeiraPagina(t, paginaComXObjectsMB(stream, "/Fm1 6 0 R", c.mediaBox, form)))
			if err != nil {
				t.Fatalf("extrairNativoSeguro: %v", err)
			}
			if m.valida != c.valida || m.formsIgnorados[formIgnoradoStreamIlegivel] != 1 {
				t.Fatalf("métricas = %+v; quer valida %v e 1 Form ilegível", m, c.valida)
			}
		})
	}
}

func TestMediaBoxDe_CicloDeParent(t *testing.T) {
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 /Parent 5 0 R >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 4 0 R >>\nendobj",
		"4 0 obj\n<< /Length 1 >>\nstream\n\nendstream\nendobj",
		"5 0 obj\n<< /Type /Pages /Kids [] /Count 0 /Parent 2 0 R >>\nendobj",
	}
	page := primeiraPagina(t, objs)
	var ok bool
	comPrazo(t, func() { _, ok = mediaBoxDe(page) })
	if ok {
		t.Fatal("mediaBoxDe = true num ciclo de /Parent sem /MediaBox")
	}
}

// Regressão da revisão da Task 2: o trecho AAA (fonte 20) abre o grupo de y=700
// numa ordem e o BBB (fonte 2) na outra; contar as linhas reagrupando uma cópia
// já reordenada pode dar mais linhas acima da imagem do que o texto tem.
func TestExtrairNativo_LinhasAcimaCasaComOTexto(t *testing.T) {
	stream := "q 595 0 0 600 0 0 cm /Im1 Do Q\n" +
		"BT /F1 20 Tf 300 700 Td (AAA) Tj ET\n" +
		"BT /F1 2 Tf 72 700 Td (BBB) Tj ET\n" +
		"BT /F1 2 Tf 72 695 Td (CCC) Tj ET\n"
	objs := paginaComXObjects(stream, "/Im1 6 0 R", imagemXObject(1240, 1250))
	texto, m, err := extrairNativoSeguro(primeiraPagina(t, objs))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for l := range strings.SplitSeq(texto, "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	if n != 1 || m.linhasAcima != 1 {
		t.Fatalf("texto %q (%d linhas), linhasAcima %d: quer 1 linha e linhasAcima 1", texto, n, m.linhasAcima)
	}
}

func TestImagens_TetoNaoContaInutil(t *testing.T) {
	stream := strings.Repeat("/Im1 Do\n", 5000) + "q 595 0 0 842 0 0 cm /Im2 Do Q\n"
	objs := paginaComXObjects(stream, "/Im1 6 0 R /Im2 7 0 R", imagemXObject(1, 1), imagemXObject(1240, 1754))
	var p passadaPagina
	comPrazo(t, func() { p = passadaDe(t, objs) })
	if m := p.metricas(); !perto(m.fImg, 1, 0.02) {
		t.Fatalf("fImg = %v (imagens %d), quer perto de 1", m.fImg, len(p.imagens))
	}
}

// BenchmarkExtrairNativo mede só extrairNativo sobre a página já aberta.
func BenchmarkExtrairNativo(b *testing.B) {
	for _, nome := range []string{"texto-puro", "logotipo"} {
		b.Run(nome, func(b *testing.B) {
			var objs []string
			for _, p := range paginasSinteticas() {
				if p.nome == nome {
					objs = p.objetos
				}
			}
			dados := buildPDF(objs)
			reader, n, err := abrirLeitorPDF(bytes.NewReader(dados), int64(len(dados)), 0)
			if err != nil {
				b.Fatal(err)
			}
			paginas, _, err := listarPaginas(reader, n)
			if err != nil || len(paginas) == 0 {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := extrairNativo(paginas[0]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
