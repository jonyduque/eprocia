package etl

import (
	"crypto/sha256"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// Páginas sintéticas A4 (595 × 842 pt), com texto fictício, para medir a
// estrutura da página. Montadas como no protótipo do brainstorm
// (referencia/gerar_pdfs.py), sem renderizar nada: os streams das imagens são
// mínimos, porque nada os lê.

// textoSintetico é o corpo de uma petição fictícia, só em ASCII (a Helvetica
// sem /Encoding decodifica por pdfDocEncoding).
const textoSintetico = "EXCELENTISSIMO SENHOR DOUTOR JUIZ DE DIREITO DA VARA CIVEL DA COMARCA DE EXEMPLO. " +
	"FULANO DE TAL, brasileiro, casado, portador do CPF n. 000.000.000-00, residente na Rua das " +
	"Flores, n. 100, vem, respeitosamente, a presenca de Vossa Excelencia, por intermedio de seu " +
	"advogado que esta subscreve, propor a presente acao de cobranca em face de BELTRANO DE TAL, " +
	"pelos fatos e fundamentos a seguir expostos. O autor celebrou com o reu contrato de prestacao " +
	"de servicos no valor de R$ 12.345,67, que nao foi pago no vencimento, conforme se verifica dos " +
	"documentos anexos. Nos termos do art. 389 do Codigo Civil, nao cumprida a obrigacao, responde o " +
	"devedor por perdas e danos, mais juros e atualizacao monetaria. Diante do exposto, requer a " +
	"citacao do reu para, querendo, apresentar contestacao, sob pena de revelia, e a procedencia do " +
	"pedido para condenar o reu ao pagamento da quantia devida, com os acrescimos legais. Da-se a " +
	"causa o valor de R$ 12.345,67. Termos em que pede deferimento. Processo 5000000-00.2026.8.26.0001."

// legendaSintetica tem 100 bytes ou mais e nenhuma das palavras do OCR falso
// da Task 6 (texto, lido, pelo, ocr, linha, um, dois, imagem).
const legendaSintetica = "Documento assinado eletronicamente por FULANO DE TAL em 01/02/2026, conforme art. 1o, III, b, " +
	"da Lei 11.419/2006. Processo 5000000-00.2026.8.26.0001, evento 12, pagina 1 de 1."

// quebrarLinhas quebra o texto em linhas de até largura caracteres.
func quebrarLinhas(texto string, largura int) []string {
	var linhas []string
	atual := ""
	for p := range strings.FieldsSeq(texto) {
		if len(atual)+len(p)+1 > largura {
			linhas = append(linhas, atual)
			atual = p
		} else {
			atual = strings.TrimSpace(atual + " " + p)
		}
	}
	if atual != "" {
		linhas = append(linhas, atual)
	}
	return linhas
}

// opsTexto escreve as linhas com /F1 a partir de (x, y), uma por T*; prefixo
// vai logo depois do BT (cor, modo de renderização).
func opsTexto(linhas []string, x, y, tam, lead float64, prefixo string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "BT %s /F1 %g Tf %g TL %g %g Td\n", prefixo, tam, lead, x, y)
	for _, l := range linhas {
		fmt.Fprintf(&sb, "(%s) Tj T*\n", l)
	}
	sb.WriteString("ET\n")
	return sb.String()
}

// corpoSintetico são as primeiras n linhas do corpo, a 10 pt, a partir de y0.
func corpoSintetico(n int, y0 float64) string {
	base := quebrarLinhas(textoSintetico, 95)
	var linhas []string
	for len(linhas) < n {
		linhas = append(linhas, base...)
	}
	return opsTexto(linhas[:n], 50, y0, 10, 15, "")
}

func legendaEmbaixo() string {
	return opsTexto(quebrarLinhas(legendaSintetica, 110), 30, 20, 7, 9, "")
}

// imagemXObject devolve o corpo de um objeto imagem que declara w×h pixels, com
// um stream mínimo (nada lê o stream nestes testes), no padrão de
// objetosPaginasComImagem.
func imagemXObject(w, h int) string {
	return fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 1 >>\nstream\nA\nendstream", w, h)
}

// mosaico pinta n ladrilhos de tw×th px (a 0,5 pt por pixel) escolhidos de
// forma espalhada numa grade que cobre 990×1400 px a partir de (50, 782) pt,
// todos com a imagem do objeto 6. Devolve os operadores e o /XObject.
func mosaico(tw, th, n int) (ops, xobjects string) {
	type pos struct{ tx, ty int }
	var todos []pos
	for ty := 0; ty < 1400; ty += th {
		for tx := 0; tx < 990; tx += tw {
			todos = append(todos, pos{tx, ty})
		}
	}
	var o, x strings.Builder
	k := 0
	for i, p := range todos {
		if (i+1)*n/len(todos) == i*n/len(todos) {
			continue
		}
		k++
		fmt.Fprintf(&o, "q %g 0 0 %g %g %g cm /T%d Do Q\n", float64(tw)*0.5, float64(th)*0.5, 50+float64(p.tx)*0.5, 842-60-float64(p.ty+th)*0.5, k)
		fmt.Fprintf(&x, "/T%d 6 0 R ", k)
	}
	return o.String(), x.String()
}

// palavrasIsca devolve n palavras aleatórias (semente fixa) de 3 a 8 letras e dígitos.
func palavrasIsca(n int, semente uint64) []string {
	const alfa = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewPCG(semente, 7))
	ps := make([]string, n)
	for i := range ps {
		b := make([]byte, 3+r.IntN(6))
		for j := range b {
			b[j] = alfa[r.IntN(len(alfa))]
		}
		ps[i] = string(b)
	}
	return ps
}

// iscaEspalhada escreve as palavras uma a uma em posições aleatórias.
func iscaEspalhada(palavras []string, tam float64, prefixo string, semente uint64) string {
	r := rand.New(rand.NewPCG(semente, 11))
	var sb strings.Builder
	fmt.Fprintf(&sb, "BT %s /F1 %g Tf\n", prefixo, tam)
	for _, p := range palavras {
		fmt.Fprintf(&sb, "1 0 0 1 %.1f %.1f Tm (%s) Tj\n", 20+r.Float64()*540, 20+r.Float64()*800, p)
	}
	sb.WriteString("ET\n")
	return sb.String()
}

type paginaSintetica struct {
	nome            string
	objetos         []string // para buildPDF e primeiraPagina
	candidataImagem bool     // fDesc >= 0,30 e fTxt <= 0,10
	isca            bool     // a regra de isca inteira (língua e fImg >= 0,10)
}

func paginasSinteticas() []paginaSintetica {
	escaneada := imagemXObject(1240, 1754) // A4 a 150 dpi
	pagina := func(stream, xobjects string, extras ...string) []string {
		return paginaComXObjects(stream, xobjects, extras...)
	}

	extrato := []string{"EXTRATO DE CONTA CORRENTE - AGENCIA 0001 - CONTA 00000-0 - FULANO DE TAL"}
	saldo := 100000
	for i := range 40 {
		v := (i*7919)%100000 - 50000
		saldo += v
		hist := "PAGAMENTO BOLETO"
		if v > 0 {
			hist = "PIX RECEBIDO"
		}
		extrato = append(extrato, fmt.Sprintf("%02d/03/2026   %s   %06d   %d,%02d   %d,%02d", i%28+1, hist, 100000+i*7717%900000, v/100, abs(v%100), saldo/100, abs(saldo%100)))
	}

	hashes := []string{"CERTIDAO DE AUTENTICIDADE DOS DOCUMENTOS DO PROCESSO 5000000-00.2026.8.26.0001"}
	for i := range 35 {
		hashes = append(hashes, fmt.Sprintf("Evento %d - DOC%d.pdf  SHA-256: %x", i+1, i+1, sha256.Sum256([]byte{byte(i)})))
	}

	urls := []string{"Documento assinado eletronicamente. A conferencia da autenticidade do documento esta disponivel no endereco eletronico"}
	for i := range 30 {
		urls = append(urls, fmt.Sprintf("https://eproc.exemplo.jus.br/eproc/externo_controlador.php?acao=consulta_autenticidade_documentos&cod=%012d&crc=%08X informando o codigo verificador %012dv%d", 100000000000+i*7919, 0xA1B2C3+i*977, 200000000000+i*104729, i%9+1))
	}

	var comprovante []string
	for i := range 30 {
		switch i % 5 {
		case 0:
			comprovante = append(comprovante, fmt.Sprintf("Codigo de autenticacao: %08X%04d", 0x5F3A21+i*331, i))
		case 1:
			comprovante = append(comprovante, fmt.Sprintf("Valor pago: R$ %d,%02d", 100+i*37, i%100))
		case 2:
			comprovante = append(comprovante, fmt.Sprintf("Data do pagamento: %02d/02/2026", i%28+1))
		case 3:
			comprovante = append(comprovante, "Pagador: FULANO DE TAL")
		case 4:
			comprovante = append(comprovante, fmt.Sprintf("Banco 001 agencia 0001 conta %05d-%d", 10000+i*13, i%10))
		}
	}

	carimbo := opsTexto([]string{
		"Recebido em 01/02/2026 por FULANO DE TAL, matricula 0001234,",
		"no processo 5000000-00.2026.8.26.0001, protocolo 000123.",
	}, 300, 400, 8, 10, "")
	timbre3 := opsTexto([]string{
		"Termos em que pede deferimento.",
		"Cidade Exemplo, 01 de fevereiro de 2026.",
		"FULANO DE TAL - OAB/SP 000.000",
	}, 50, 700, 11, 16, "")

	mosLeg, xoLeg := mosaico(90, 40, 237)
	mosIsca, xoIsca := mosaico(60, 30, 400)
	inline := "q 595 0 0 800 0 42 cm\nBI /W 620 /H 830 /CS /G /BPC 8 /F /AHx ID\n" + strings.Repeat("FF", 64) + ">\nEI Q\n"
	isca := palavrasIsca(80, 42)
	var iscaLinhas []string
	for i := 0; i < len(isca); i += 10 {
		iscaLinhas = append(iscaLinhas, strings.Join(isca[i:i+10], " "))
	}

	return []paginaSintetica{
		{nome: "texto-puro", objetos: pagina(corpoSintetico(45, 780), "")},
		{nome: "extrato-nativo", objetos: pagina(opsTexto(extrato, 40, 790, 9, 18, ""), "")},
		{nome: "certidao-hashes", objetos: pagina(opsTexto(hashes, 30, 800, 7, 20, ""), "")},
		{nome: "rodapes-urls", objetos: pagina(opsTexto(urls, 20, 800, 6, 22, ""), "")},
		{nome: "logotipo", objetos: pagina("q 150 0 0 50 50 780 cm /Im1 Do Q\n"+corpoSintetico(45, 740), "/Im1 6 0 R", imagemXObject(300, 100))},
		{nome: "assinatura-digitalizada", objetos: pagina(corpoSintetico(40, 780)+"q 200 0 0 60 200 80 cm /Im1 Do Q\n", "/Im1 6 0 R", imagemXObject(400, 120))},
		{nome: "timbre-meia-pagina", objetos: pagina("q 595 0 0 842 0 0 cm /Im1 Do Q\n"+corpoSintetico(22, 740), "/Im1 6 0 R", imagemXObject(595, 842))},
		{nome: "timbre-pagina-cheia", objetos: pagina("q 595 0 0 842 0 0 cm /Im1 Do Q\n"+corpoSintetico(45, 740), "/Im1 6 0 R", imagemXObject(595, 842))},
		{nome: "digitalizado-camada-ocr", objetos: pagina("q 595 0 0 842 0 0 cm /Im1 Do Q\n"+opsTexto(strings.Split(strings.TrimSpace(strings.Repeat(strings.Join(quebrarLinhas(textoSintetico, 95), "\n")+"\n", 6)), "\n")[:45], 40, 790, 10, 16, "3 Tr"), "/Im1 6 0 R", escaneada)},
		{nome: "peticao-print", objetos: pagina(corpoSintetico(22, 780)+"q 400 0 0 300 100 60 cm /Im1 Do Q\n", "/Im1 6 0 R", imagemXObject(800, 600))},
		{nome: "comprovante-codigos", objetos: pagina(opsTexto(comprovante, 40, 760, 10, 20, "")+"q 250 0 0 500 320 200 cm /Im1 Do Q\n", "/Im1 6 0 R", imagemXObject(520, 1040))},
		{nome: "legenda-imagem-direta", candidataImagem: true, objetos: pagina("q 595 0 0 800 0 42 cm /Im1 Do Q\n"+legendaEmbaixo(), "/Im1 6 0 R", escaneada)},
		{nome: "legenda-imagem-form", candidataImagem: true, objetos: pagina("q 1 0 0 1 0 42 cm /Fm1 Do Q\n"+legendaEmbaixo(), "/Fm1 6 0 R",
			formXObject("q 595 0 0 800 0 0 cm /Im0 Do Q", "/Resources << /XObject << /Im0 7 0 R >> >>"), escaneada)},
		{nome: "legenda-imagem-inline", candidataImagem: true, objetos: pagina(inline+legendaEmbaixo(), "")},
		{nome: "legenda-acima-imagem", candidataImagem: true, objetos: pagina(opsTexto(quebrarLinhas(legendaSintetica, 110), 30, 800, 7, 20, "")+"q 595 0 0 700 0 0 cm /Im1 Do Q\n", "/Im1 6 0 R", escaneada)},
		{nome: "carimbo-sobre-digitalizacao", candidataImagem: true, objetos: pagina("q 595 0 0 842 0 0 cm /Im1 Do Q\n"+carimbo, "/Im1 6 0 R", escaneada)},
		{nome: "mosaico-legenda", candidataImagem: true, objetos: pagina(mosLeg+legendaEmbaixo(), xoLeg, imagemXObject(90, 40))},
		{nome: "mosaico-legenda-curta", candidataImagem: true, objetos: pagina(mosLeg+opsTexto([]string{"FULANO DE TAL - evento 12, pagina 1 de 1"}, 30, 20, 7, 9, ""), xoLeg, imagemXObject(90, 40))},
		{nome: "timbre-3-linhas", candidataImagem: true, objetos: pagina("q 595 0 0 842 0 0 cm /Im1 Do Q\n"+timbre3, "/Im1 6 0 R", escaneada)},
		{nome: "isca-cinza", isca: true, objetos: pagina(iscaEspalhada(isca, 5, "0.93 0.93 0.93 rg", 1)+mosIsca, xoIsca, imagemXObject(60, 30))},
		{nome: "isca-preta", isca: true, objetos: pagina(iscaEspalhada(isca, 9, "", 2)+mosIsca, xoIsca, imagemXObject(60, 30))},
		{nome: "isca-linhas", isca: true, objetos: pagina(opsTexto(iscaLinhas, 40, 800, 8, 19, "0.9 0.9 0.9 rg")+mosIsca, xoIsca, imagemXObject(60, 30))},
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func paginaSinteticaDe(t *testing.T, nome string) paginaSintetica {
	t.Helper()
	for _, p := range paginasSinteticas() {
		if p.nome == nome {
			return p
		}
	}
	t.Fatalf("página sintética %q não existe", nome)
	return paginaSintetica{}
}

// faixa é um valor esperado com tolerância; livre não é conferido.
type faixa struct {
	min, max float64
	livre    bool
}

func val(v float64) faixa       { return faixa{min: v - 0.05, max: v + 0.05} }
func entre(a, b float64) faixa  { return faixa{min: a - 0.05, max: b + 0.05} }
func acimaDe(v float64) faixa   { return faixa{min: v + 1e-9, max: 2} }
func pelomenos(v float64) faixa { return faixa{min: v - 0.05, max: 2} }
func ateh(v float64) faixa      { return faixa{min: -1, max: v + 0.05} }

var livre = faixa{livre: true}

func (f faixa) contem(x float64) bool { return f.livre || (x >= f.min && x <= f.max) }

// esperadoSintetico é a tabela do brief (Task 2): fImg, fTxt, fDesc e se a
// candidatura é conferida.
var esperadoSintetico = map[string]struct {
	fImg, fTxt, fDesc faixa
	candidataLivre    bool
}{
	"texto-puro":                  {val(0), livre, val(0), false},
	"extrato-nativo":              {val(0), livre, val(0), false},
	"certidao-hashes":             {val(0), livre, val(0), false},
	"rodapes-urls":                {val(0), livre, val(0), false},
	"logotipo":                    {entre(0.01, 0.02), entre(0.46, 0.51), entre(0.01, 0.02), false},
	"assinatura-digitalizada":     {entre(0.01, 0.02), entre(0.46, 0.51), entre(0.01, 0.02), false},
	"timbre-meia-pagina":          {val(1), val(0.25), val(0.71), false},
	"timbre-pagina-cheia":         {val(1), acimaDe(0.10), livre, false},
	"digitalizado-camada-ocr":     {val(1), val(0.55), val(0.39), false},
	"peticao-print":               {val(0.24), val(0.25), val(0.24), false},
	"comprovante-codigos":         {pelomenos(0.10), acimaDe(0.10), livre, false},
	"legenda-imagem-direta":       {val(0.96), val(0.02), val(0.95), false},
	"legenda-imagem-form":         {val(0.96), val(0.02), val(0.95), false},
	"legenda-imagem-inline":       {val(0.96), val(0.02), val(0.95), false},
	"legenda-acima-imagem":        {val(0.84), val(0.02), val(0.84), false},
	"carimbo-sobre-digitalizacao": {val(1), val(0.01), val(0.97), false},
	"mosaico-legenda":             {val(0.42), val(0.02), val(0.42), false},
	"mosaico-legenda-curta":       {val(0.42), ateh(0.02), val(0.42), false},
	"timbre-3-linhas":             {val(1), val(0.02), val(0.97), false},
	"isca-cinza":                  {pelomenos(0.10), livre, livre, true},
	"isca-preta":                  {pelomenos(0.10), livre, livre, true},
	"isca-linhas":                 {pelomenos(0.10), livre, livre, true},
}

func TestSinteticas_Metricas(t *testing.T) {
	paginas := paginasSinteticas()
	if len(paginas) != len(esperadoSintetico) {
		t.Fatalf("%d páginas sintéticas, %d esperadas", len(paginas), len(esperadoSintetico))
	}
	for _, p := range paginas {
		t.Run(p.nome, func(t *testing.T) {
			e, ok := esperadoSintetico[p.nome]
			if !ok {
				t.Fatalf("sem valores esperados")
			}
			_, m, err := extrairNativoSeguro(primeiraPagina(t, p.objetos))
			if err != nil {
				t.Fatalf("extrairNativoSeguro: %v", err)
			}
			t.Logf("fImg %.3f fTxt %.3f fDesc %.3f linhasAcima %d", m.fImg, m.fTxt, m.fDesc, m.linhasAcima)
			if !m.valida {
				t.Fatalf("valida = false")
			}
			if !e.fImg.contem(m.fImg) || !e.fTxt.contem(m.fTxt) || !e.fDesc.contem(m.fDesc) {
				t.Errorf("fImg %.3f fTxt %.3f fDesc %.3f fora da faixa %+v", m.fImg, m.fTxt, m.fDesc, e)
			}
			if candidata := m.fDesc >= 0.30 && m.fTxt <= 0.10; !e.candidataLivre && candidata != p.candidataImagem {
				t.Errorf("candidata = %v, quer %v", candidata, p.candidataImagem)
			}
		})
	}
}

func TestExtrairNativoSeguro_IgualAoExtractNativeText(t *testing.T) {
	for _, p := range paginasSinteticas() {
		t.Run(p.nome, func(t *testing.T) {
			page := primeiraPagina(t, p.objetos)
			texto, _, err := extrairNativoSeguro(page)
			antigo, errAntigo := ExtractNativeText(page)
			if texto != antigo || (err == nil) != (errAntigo == nil) {
				t.Fatalf("extrairNativoSeguro = %q, %v; ExtractNativeText = %q, %v", texto, err, antigo, errAntigo)
			}
		})
	}
}

func TestExtrairNativoSeguro_SemMediaBoxNaoEValida(t *testing.T) {
	p := paginaSinteticaDe(t, "legenda-imagem-direta")
	objs := append([]string(nil), p.objetos...)
	objs[2] = strings.Replace(objs[2], "/MediaBox [0 0 595 842] ", "", 1)
	if objs[2] == p.objetos[2] {
		t.Fatal("o /MediaBox não foi tirado")
	}
	texto, m, err := extrairNativoSeguro(primeiraPagina(t, objs))
	if err != nil {
		t.Fatalf("extrairNativoSeguro: %v", err)
	}
	comMB, _, _ := extrairNativoSeguro(primeiraPagina(t, p.objetos))
	if m.valida || m.linhasAcima != 0 || texto != comMB {
		t.Fatalf("sem MediaBox: valida %v, linhasAcima %d, texto %q (com MediaBox %q)", m.valida, m.linhasAcima, texto, comMB)
	}
}

func TestSinteticas_LinhasAcima(t *testing.T) {
	for _, p := range paginasSinteticas() {
		t.Run(p.nome, func(t *testing.T) {
			texto, m, err := extrairNativoSeguro(primeiraPagina(t, p.objetos))
			if err != nil {
				t.Fatalf("extrairNativoSeguro: %v", err)
			}
			switch {
			case p.nome == "legenda-acima-imagem":
				if m.linhasAcima != 2 {
					t.Errorf("linhasAcima = %d, quer 2", m.linhasAcima)
				}
			case p.candidataImagem:
				if m.linhasAcima != 0 {
					t.Errorf("linhasAcima = %d, quer 0", m.linhasAcima)
				}
			}
			naoVazias := 0
			for l := range strings.SplitSeq(texto, "\n") {
				if strings.TrimSpace(l) != "" {
					naoVazias++
				}
			}
			if m.linhasAcima > naoVazias {
				t.Errorf("linhasAcima = %d passa das %d linhas do texto", m.linhasAcima, naoVazias)
			}
		})
	}
}
