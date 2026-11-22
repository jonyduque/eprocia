package etl

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type casoJuntar struct {
	ocr, nativo string
	acima       int
	want        string
}

var casosJuntar = []casoJuntar{
	{"PETICAO INICIAL\nFULANO DE TAL requer", "Petição inicial", 0, "PETICAO INICIAL\nFULANO DE TAL requer"},
	{"TEXTO DA IMAGEM", "Figura 1: fachada do imóvel", 0, "TEXTO DA IMAGEM\n\nFigura 1: fachada do imóvel"},
	{"TEXTO DA IMAGEM", "Figura 1: fachada do imóvel", 1, "Figura 1: fachada do imóvel\n\nTEXTO DA IMAGEM"},
	// Linha nativa com uma palavra que o OCR não trouxe entra, mesmo com as
	// outras todas no OCR: o pedaço que falta (um valor, uma data) não se perde.
	{"alfa beta gama", "alfa beta gama delta epsilon", 0, "alfa beta gama\n\nalfa beta gama delta epsilon"},
	{"Valor total: R$", "Valor total: R$ 1.500,00", 0, "Valor total: R$\n\nValor total: R$ 1.500,00"},
	{"alfa beta gama delta epsilon", "Alfa beta gama delta épsilon", 0, "alfa beta gama delta epsilon"},
	{"alfa beta", "alfa beta gama delta epsilon", 0, "alfa beta\n\nalfa beta gama delta epsilon"},
	{"Página 12 de 40", "12", 0, "Página 12 de 40"},
	{"Texto qualquer", "12", 0, "Texto qualquer\n\n12"},
	{"TEXTO\n\n", "linha um aqui\nlinha dois aqui", 0, "TEXTO\n\nlinha um aqui\nlinha dois aqui"},
	{"TEXTO", "", 0, "TEXTO"},
	{
		"CABECALHO DO DOCUMENTO\nTEXTO DA IMAGEM\nRODAPE FINAL",
		"Cabeçalho do documento\nlinha que o OCR perdeu\noutra linha perdida", 3,
		"CABECALHO DO DOCUMENTO\nlinha que o OCR perdeu\noutra linha perdida\nTEXTO DA IMAGEM\nRODAPE FINAL",
	},
	{
		"CABECALHO DO DOCUMENTO\nTEXTO DA IMAGEM",
		"Cabeçalho do documento\nlegenda embaixo da figura", 1,
		"CABECALHO DO DOCUMENTO\nTEXTO DA IMAGEM\n\nlegenda embaixo da figura",
	},
	{
		"TEXTO DA IMAGEM\nASSINATURA DIGITAL DO CARTORIO",
		"selo perdido pelo leitor\nAssinatura digital do cartório", 0,
		"TEXTO DA IMAGEM\nselo perdido pelo leitor\nASSINATURA DIGITAL DO CARTORIO",
	},
	{
		"alfa beta\nalfa beta gama\ndelta",
		"alfa beta gama\nfalta esta linha", 0,
		"alfa beta\nalfa beta gama\nfalta esta linha\ndelta",
	},
	{
		"RODAPE FINAL\nCABECALHO DO DOCUMENTO",
		"Cabeçalho do documento\nfalta no topo\nfalta embaixo\nRodapé final", 2,
		"falta embaixo\nRODAPE FINAL\nCABECALHO DO DOCUMENTO\nfalta no topo",
	},
	// Um "prazo" do OCR não cobre duas linhas nativas: a segunda entra.
	{"prazo de defesa\nrecursal contado", "prazo de defesa\nprazo recursal", 0, "prazo de defesa\nprazo recursal\nrecursal contado"},
	// O dia lido errado ("06" no lugar de "05") faz a linha nativa entrar.
	{"Data do fato 06/11/2026 conforme autos", "Data do fato 05/11/2026 conforme autos", 0, "Data do fato 06/11/2026 conforme autos\n\nData do fato 05/11/2026 conforme autos"},
	// A palavra repetida na mesma linha precisa de uma ocorrência própria no OCR.
	{"Prazo de dez dias", "dias a dias", 0, "Prazo de dez dias\n\ndias a dias"},
	// Linha só de pedaços curtos com dígito: vale a contagem de "10", não o Contains.
	{"Folha 210 x", "10 x", 0, "Folha 210 x\n\n10 x"},
	// A linha ausente não consome o "prazo" do OCR: a presente seguinte fica presente.
	{"prazo de defesa", "prazo vencido hoje\nprazo de defesa", 0, "prazo vencido hoje\nprazo de defesa"},
}

func TestJuntar(t *testing.T) {
	for _, c := range casosJuntar {
		if got := juntarOCRComNativo(c.ocr, c.nativo, c.acima); got != c.want {
			t.Errorf("juntarOCRComNativo(%q, %q, %d) =\n%q\nesperava\n%q", c.ocr, c.nativo, c.acima, got, c.want)
		}
	}
}

const ocrSemPalavrasEmComum = "XXXX YYYY\nZZZZ WWWW"

var nativoSeisLinhas = []string{
	"primeira linha nativa",
	"segunda linha nativa",
	"terceira linha nativa",
	"quarta linha nativa",
	"quinta linha nativa",
	"sexta linha nativa",
}

func TestJuntar_NuncaPerdeLinhaNativa(t *testing.T) {
	nativo := strings.Join(nativoSeisLinhas, "\n")
	for _, acima := range []int{0, 2, 6} {
		var partes []string
		if acima > 0 {
			partes = append(partes, strings.Join(nativoSeisLinhas[:acima], "\n"))
		}
		partes = append(partes, ocrSemPalavrasEmComum)
		if acima < len(nativoSeisLinhas) {
			partes = append(partes, strings.Join(nativoSeisLinhas[acima:], "\n"))
		}
		want := strings.Join(partes, "\n\n")
		if got := juntarOCRComNativo(ocrSemPalavrasEmComum, nativo, acima); got != want {
			t.Errorf("acima=%d:\n%q\nesperava\n%q", acima, got, want)
		}
	}

	casos := slices.Clone(casosJuntar)
	for _, acima := range []int{0, 2, 6} {
		casos = append(casos, casoJuntar{ocr: ocrSemPalavrasEmComum, nativo: nativo, acima: acima})
	}
	contar := func(s string) map[string]int {
		m := map[string]int{}
		for _, f := range fichasOraculo(s) {
			m[f]++
		}
		return m
	}
	// P2 e P3 são limites que valem para qualquer ordem de consumo; o caso
	// exato (consumo em ordem, só das presentes) está em casosJuntar.
	for _, c := range casos {
		got := juntarOCRComNativo(c.ocr, c.nativo, c.acima)
		linhasGot := strings.Split(got, "\n")
		cOCR, cNat, cGot := contar(c.ocr), contar(c.nativo), contar(got)
		linhasOCR := linhasDoOCR(c.ocr)
		// P1: nenhuma ficha nativa perdida.
		for f, n := range cNat {
			if cGot[f] < n {
				t.Errorf("caso %q/%q: ficha nativa %q aparece %d vez(es) em %q, esperava ao menos %d", c.ocr, c.nativo, f, cGot[f], got, n)
			}
		}
		cont := func(ls []string, l string) int {
			n := 0
			for _, x := range ls {
				if x == l {
					n++
				}
			}
			return n
		}
		for _, l := range linhasNativas(c.nativo) {
			cLinha := contar(l)
			// P2: linha com ficha que o OCR não pode cobrir entra inteira.
			naoCobre := false
			// P3: linha cujas fichas o OCR cobre para a página nativa inteira.
			cobreDeSobra := len(cLinha) > 0
			for f, n := range cLinha {
				if n > cOCR[f] {
					naoCobre = true
				}
				if cOCR[f] < cNat[f] {
					cobreDeSobra = false
				}
			}
			if naoCobre && !slices.Contains(linhasGot, l) {
				t.Errorf("caso %q/%q: linha nativa %q, com ficha que o OCR não cobre, não entrou inteira em %q", c.ocr, c.nativo, l, got)
			}
			if cobreDeSobra && cont(linhasGot, l) > cont(linhasOCR, l) {
				t.Errorf("caso %q/%q: linha nativa %q já coberta pelo OCR entrou de novo em %q", c.ocr, c.nativo, l, got)
			}
		}
		// As linhas do OCR aparecem todas, na ordem.
		j := 0
		for _, l := range linhasDoOCR(c.ocr) {
			for j < len(linhasGot) && linhasGot[j] != l {
				j++
			}
			if j == len(linhasGot) {
				t.Errorf("caso %q/%q: linha do OCR %q fora de ordem ou perdida em %q", c.ocr, c.nativo, l, got)
				break
			}
			j++
		}
	}
}

// fichasOraculo é o oráculo do teste de propriedade, independente de
// pdf_juncao.go e lingua.go: o texto em NFC (sem isso um "Petição" em NFD se
// parte nas marcas, que não são letra), cortado em sequências de letras e
// dígitos com 3 runas ou mais ou com algum dígito, em minúsculas e sem acento.
func fichasOraculo(s string) []string {
	var out []string
	s = norm.NFC.String(s)
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if utf8.RuneCountInString(w) < 3 && !strings.ContainsFunc(w, unicode.IsDigit) {
			continue
		}
		d := norm.NFD.String(strings.ToLower(w))
		d = strings.Map(func(r rune) rune {
			if unicode.Is(unicode.Mn, r) {
				return -1
			}
			return r
		}, d)
		out = append(out, norm.NFC.String(d))
	}
	return out
}

// Entrada hostil: nada de pânico nem de demora com texto vazio, só espaços,
// só quebras, linhas enormes, UTF-8 inválido e linhasAcima fora dos limites.
func TestJuntar_EntradaHostil(t *testing.T) {
	enorme := strings.Repeat("palavra ", 20_000)
	muitas := strings.Repeat("alfa beta gama delta\n", 500)
	textos := []string{"", "   ", "\n\n\n", "\f", "\xff\xfe\xfd", "a\xffbcd efg\xc3", enorme, muitas, "\r\n\r\n", "12\n12\n12"}
	for _, o := range textos {
		for _, n := range textos {
			for _, acima := range []int{-5, 0, 1, 1 << 30} {
				_ = juntarOCRComNativo(o, n, acima)
				for _, m := range []motivoOCR{motivoPadrao, motivoIlegivel, motivoImagemDescoberta, motivoIsca} {
					_, _ = conteudoAposOCR(ocrTask{pageNum: 1, fallback: n, motivo: m, linhasAcima: acima}, o)
				}
			}
		}
	}

	// Pior caso da âncora: 2000 linhas do OCR com as mesmas palavras e 2000
	// linhas nativas alternando presente e faltando, cada faltando pede a
	// âncora da vizinha. Linhas do OCR vezes palavras da linha nativa, não mais.
	var nativo strings.Builder
	for i := range 2000 {
		if i%2 == 0 {
			nativo.WriteString("alfa beta gama delta\n")
		} else {
			nativo.WriteString("linha perdida pelo leitor\n")
		}
	}
	ocrGrande := strings.Repeat("alfa beta gama delta\n", 2000)
	inicio := time.Now()
	got := juntarOCRComNativo(ocrGrande, nativo.String(), 0)
	if d := time.Since(inicio); d > 10*time.Second {
		t.Errorf("pior caso da âncora levou %v", d)
	}
	if n := strings.Count(got, "linha perdida pelo leitor"); n != 1000 {
		t.Errorf("linhas encaixadas = %d, esperava 1000", n)
	}
	// Linha nativa sem palavras e OCR só com espaços: a linha não se perde.
	if got := juntarOCRComNativo("   \n  ", "12", 0); got != "12" {
		t.Errorf("OCR em branco: %q", got)
	}
}

func TestConteudoAposOCR(t *testing.T) {
	const reserva = "RESERVA NATIVA da página"
	ilegivel := strings.Repeat("§¤¶", 10)
	if !textoIlegivel(ilegivel) {
		t.Fatalf("premissa: textoIlegivel(%q) deveria ser true", ilegivel)
	}
	motivos := []motivoOCR{motivoPadrao, motivoIlegivel, motivoImagemDescoberta, motivoIsca}
	for _, m := range motivos {
		for _, acima := range []int{0, 1} {
			task := ocrTask{pageNum: 1, fallback: reserva, motivo: m, linhasAcima: acima}
			want := "TEXTO BOM DO OCR"
			if m == motivoImagemDescoberta {
				if acima == 0 {
					want = "TEXTO BOM DO OCR\n\n" + reserva
				} else {
					want = reserva + "\n\nTEXTO BOM DO OCR"
				}
			}
			if c, o := conteudoAposOCR(task, "TEXTO BOM DO OCR"); c != want || !o {
				t.Errorf("motivo %d, acima %d: (%q, %t), esperava (%q, true)", m, acima, c, o, want)
			}
			for _, ruim := range []string{"", "  \n\t", "\f", "\n\f\n", ilegivel} {
				if c, o := conteudoAposOCR(task, ruim); c != reserva || o {
					t.Errorf("motivo %d, OCR %q: (%q, %t), esperava a reserva sem OCR", m, ruim, c, o)
				}
			}
		}
		semReserva := ocrTask{pageNum: 1, motivo: m}
		if c, o := conteudoAposOCR(semReserva, "X"); c != "X" || !o {
			t.Errorf("sem reserva, motivo %d: (%q, %t)", m, c, o)
		}
		if c, o := conteudoAposOCR(semReserva, ""); c != "" || !o {
			t.Errorf("sem reserva, motivo %d, OCR vazio: (%q, %t)", m, c, o)
		}
	}
}

func TestParsePages_OCREmBrancoMantemAReserva(t *testing.T) {
	pages := parsePagesWithDeadline(t, objetosTextoCurtoEImagem(), func(context.Context, string, int) (string, error) {
		return "", nil
	})
	p := pages["1"].(map[string]any)
	if c, _ := p["content"].(string); !strings.Contains(c, "Texto nativo curto ficticio") {
		t.Errorf("content = %q, esperava o texto nativo curto", c)
	}
	if o, _ := p["ocr"].(bool); o {
		t.Errorf("ocr = true, esperava false")
	}
	if _, tem := p["ocr_erro"]; tem {
		t.Errorf("ocr_erro presente: %v", p)
	}
}

func TestParsePages_OCRComErroSegueComoHoje(t *testing.T) {
	pages := parsePagesWithDeadline(t, objetosTextoCurtoEImagem(), func(context.Context, string, int) (string, error) {
		return "", errors.New("tesseract falhou")
	})
	p := pages["1"].(map[string]any)
	if c, _ := p["content"].(string); !strings.Contains(c, "Texto nativo curto ficticio") {
		t.Errorf("content = %q, esperava a reserva", c)
	}
	if o, _ := p["ocr"].(bool); o {
		t.Errorf("ocr = true, esperava false")
	}
	if e, _ := p["ocr_erro"].(string); e == "" {
		t.Errorf("ocr_erro vazio: %v", p)
	}
}

func TestParsePages_OCREmBrancoMantemAReserva_SegundoLeitor(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) {
		return []string{"Despacho curto ficticio"}, nil
	})
	pages, err := parserPoppler(1).ParsePages(context.Background(), true,
		func(context.Context, string, int) (string, error) { return " \n", nil }, nil)
	if err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	p := paginaPoppler(t, pages, "1")
	if c := p["content"]; c != "Despacho curto ficticio" {
		t.Errorf("content = %q", c)
	}
	if o, _ := p["ocr"].(bool); o {
		t.Errorf("ocr = true, esperava false")
	}
}
