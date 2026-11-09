package etl

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLinhasBasicas(t *testing.T) {
	in := "  a   b\t\tc  \n\n\x01d  e\n_ _ _ _ _ _\n-----\nItem ........ 12"
	want := []string{"a b c", "d e", "Item ........ 12"}
	if got := linhasBasicas(in); !slices.Equal(got, want) {
		t.Errorf("linhasBasicas = %q, quer %q", got, want)
	}
}

func TestLinhaSeparadora(t *testing.T) {
	for _, l := range []string{"-----", "_ _ _ _ _", "=====", ".....", "*****", "• • • • •"} {
		if !linhaSeparadora(l) {
			t.Errorf("%q deveria ser separadora", l)
		}
	}
	for _, l := range []string{"----", "", "--- a ---", "Item ........ 12"} {
		if linhaSeparadora(l) {
			t.Errorf("%q não deveria ser separadora", l)
		}
	}
}

func TestLinhasBasicas_NBSP(t *testing.T) {
	got := linhasBasicas("a  b")
	if !slices.Equal(got, []string{"a b"}) {
		t.Errorf("got %q", got)
	}
}

func TestCleanPage_BasicaNaoJuntaLinhas(t *testing.T) {
	got := cleanPage("linha  um\nlinha dois", NivelBasico, &ctxDocumento{})
	if want := "linha um\nlinha dois"; got != want {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestCleanPage_NivelDesconhecidoViraBasica(t *testing.T) {
	got := cleanPage("a  b\nc", "xyz", nil)
	if want := "a b\nc"; got != want {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestLinhasBasicas_VFeNEL(t *testing.T) {
	got := linhasBasicas("a\vb\fc\u0085d")
	if !slices.Equal(got, []string{"a b c d"}) {
		t.Errorf("got %q", got)
	}
}

// blocoEproc é a chancela do eproc em 5 linhas, como o HTML sai depois da
// Tarefa 2, mais o rodapé CNJ.
var blocoEproc = []string{
	"Documento eletrônico assinado por FULANO DE TAL, Juiz de Direito, na forma do artigo 1º, inciso III,",
	"da Lei 11.419, de 19 de dezembro de 2006. A conferência da autenticidade do documento",
	"está disponível no endereço eletrônico https://eproc.exemplo.jus.br/externo?acao=autenticidade,",
	"mediante o preenchimento do código verificador 123456789012 e do código CRC",
	"ABCD1234.",
	"5000000-00.2026.8.26.0001 123456789012 .V2",
}

var miolo3 = []string{"Vistos.", "Defiro o pedido.", "Intimem-se."}

func comMiolo(fim ...string) []string {
	return append(slices.Clone(miolo3), fim...)
}

func TestChancela_HTMLSoNome(t *testing.T) {
	got := resumirChancelas(comMiolo(blocoEproc...), true)
	if want := comMiolo("[assinado: FULANO DE TAL]"); !slices.Equal(got, want) {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestChancela_EprocPDFTrazCargo(t *testing.T) {
	got := resumirChancelas(comMiolo(blocoEproc...), false)
	if want := comMiolo("[assinado: FULANO DE TAL, Juiz de Direito]"); !slices.Equal(got, want) {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestChancela_BlocoNumaLinhaSo(t *testing.T) {
	eproc := strings.Join(blocoEproc[:5], " ")
	if got := resumirChancelas(comMiolo(eproc, blocoEproc[5]), true); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL]")) {
		t.Errorf("eproc: got %q", got)
	}
	esaj := "Este documento é cópia do original, assinado digitalmente por FULANO DE TAL, protocolado em 01/01/2026 às 10:00 , sob o número X1. Para conferir o original, acesse o site https://esaj.exemplo.jus.br, informe o processo 5000000-00.2026.8.26.0001 e código Y2."
	if got := resumirChancelas(comMiolo(esaj), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL, 01/01/2026]")) {
		t.Errorf("e-SAJ: got %q", got)
	}
}

func TestChancela_ESAJ(t *testing.T) {
	esaj := []string{
		"Este documento é cópia do original, assinado digitalmente por FULANO DE TAL e Tribunal de Justiça Fictício,",
		"protocolado em 01/01/2026 às 10:00 , sob o número WXYZ0000001.",
		"Para conferir o original, acesse o site https://esaj.exemplo.jus.br/conferencia, informe o processo",
		"5000000-00.2026.8.26.0001 e código ABC123.",
	}
	want := "[assinado: FULANO DE TAL, 01/01/2026]"
	if got := resumirChancelas(comMiolo(esaj...), false); !slices.Equal(got, comMiolo(want)) {
		t.Errorf("rodapé: got %q", got)
	}
	topo := append(slices.Clone(esaj), miolo3...)
	if got := resumirChancelas(topo, false); !slices.Equal(got, append([]string{want}, miolo3...)) {
		t.Errorf("topo: got %q", got)
	}
}

func TestChancela_SoESAJNoTopo(t *testing.T) {
	in := []string{"Assinado digitalmente por FULANO DE TAL - Juiz de Direito"}
	for i := range 12 {
		in = append(in, "linha "+strconv.Itoa(i))
	}
	if got := resumirChancelas(in, false); !slices.Equal(got, in) {
		t.Errorf("chancela de cartório no topo devia ficar: %q", got)
	}
}

func TestChancela_Cartorio(t *testing.T) {
	ass := "Assinado digitalmente por FULANO DE TAL - Juiz de Direito"
	prot := "Protocolo nº 123456 Documento registrado eletronicamente em 01/01/2026"
	if got := resumirChancelas(comMiolo(ass, prot), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL, Juiz de Direito]")) {
		t.Errorf("par: got %q", got)
	}
	if got := resumirChancelas(comMiolo(prot, ass), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL, Juiz de Direito]")) {
		t.Errorf("par invertido: got %q", got)
	}
	if got := resumirChancelas(comMiolo(prot), false); !slices.Equal(got, comMiolo("[assinado: 01/01/2026]")) {
		t.Errorf("protocolo sozinho: got %q", got)
	}
}

func TestChancela_CartorioExigeFormaDeNome(t *testing.T) {
	for _, fim := range [][]string{
		{"assinado digitalmente por ele, razão pela qual o contrato é nulo."},
		{"Assinado digitalmente por ele, razão pela qual o contrato é nulo."},
		{"Assinado digitalmente por quem não tinha poderes para tanto."},
		{"Assinado digitalmente por X"},
	} {
		if got := resumirChancelas(comMiolo(fim...), false); !slices.Equal(got, comMiolo(fim...)) {
			t.Errorf("%q devia ficar intacta: %q", fim, got)
		}
	}
	ok := "Assinado digitalmente por FULANO DE TAL - Tabelião"
	if got := resumirChancelas(comMiolo(ok), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL, Tabelião]")) {
		t.Errorf("got %q", got)
	}
	so := "Assinado digitalmente por FULANO DE TAL"
	if got := resumirChancelas(comMiolo(so), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL]")) {
		t.Errorf("só nome: got %q", got)
	}
}

func TestChancela_CartorioCargoNaLinhaSeguinte(t *testing.T) {
	got := resumirChancelas(comMiolo("Assinado digitalmente por FULANO DE TAL -", "Juiz de Direito"), false)
	if want := comMiolo("[assinado: FULANO DE TAL, Juiz de Direito]"); !slices.Equal(got, want) {
		t.Errorf("got %q, quer %q", got, want)
	}
	// A linha seguinte é outra chancela: não vira cargo.
	got = resumirChancelas(comMiolo("Assinado digitalmente por FULANO DE TAL -", "Este laudo foi assinado eletronicamente"), false)
	if want := comMiolo("[assinado: FULANO DE TAL]"); !slices.Equal(got, want) {
		t.Errorf("got %q, quer %q", got, want)
	}
}

func TestChancela_CargoLinhaRecusadaFicaIntacta(t *testing.T) {
	for _, l2 := range []string{
		"Intimem-se.",
		"5000000-00.2026.8.26.0001",
		"Protocolo nº 123",
		"Cumpra-se com urgência a determinação anterior deste juízo",
	} {
		got := resumirChancelas(comMiolo("Assinado digitalmente por FULANO DE TAL -", l2), false)
		if want := comMiolo("[assinado: FULANO DE TAL]", l2); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, quer %q", l2, got, want)
		}
	}
	// Linha 2 é a chancela secundária do cartório: não vira cargo nem empresta a
	// data; some porque a linha 1 já deixou o marcador.
	got := resumirChancelas(comMiolo("Assinado digitalmente por FULANO DE TAL -", "Documento registrado eletronicamente em 01/01/2026"), false)
	if want := comMiolo("[assinado: FULANO DE TAL]"); !slices.Equal(got, want) {
		t.Errorf("protocolo: got %q, quer %q", got, want)
	}
	got = resumirChancelas(comMiolo("Assinado digitalmente por FULANO DE TAL -", "Juiz de Direito"), false)
	if want := comMiolo("[assinado: FULANO DE TAL, Juiz de Direito]"); !slices.Equal(got, want) {
		t.Errorf("cargo: got %q", got)
	}
}

func TestChancela_CartorioNomeComMaiuscula(t *testing.T) {
	for _, l := range []string{
		"Assinado digitalmente por meio do certificado ICP-Brasil o contrato",
		"Assinado digitalmente por quem não tinha poderes para tanto",
		"Assinado digitalmente por ambas as partes em 01/01/2026",
	} {
		if got := resumirChancelas(comMiolo(l), false); !slices.Equal(got, comMiolo(l)) {
			t.Errorf("%q devia ficar intacta: %q", l, got)
		}
	}
	for l, want := range map[string]string{
		"Assinado digitalmente por Dr. FULANO DE TAL - Tabelião": "[assinado: Dr. FULANO DE TAL, Tabelião]",
		"ASSINADO DIGITALMENTE POR FULANO DE TAL - TABELIÃO":     "[assinado: FULANO DE TAL, TABELIÃO]",
		"Assinado digitalmente por FULANO DE TAL em 01/01/2026":  "[assinado: FULANO DE TAL, 01/01/2026]",
	} {
		if got := resumirChancelas(comMiolo(l), false); !slices.Equal(got, comMiolo(want)) {
			t.Errorf("%q: got %q, quer %q", l, got, want)
		}
	}
}

func TestChancela_LaudoComCaudaFica(t *testing.T) {
	for _, l := range []string{
		"Este laudo foi assinado eletronicamente pelo perito Dr. FULANO DE TAL, CRM 12345.",
		"Você pode conferir a autenticidade do documento em https://exemplo.jus.br com o código XYZ123 até 01/01/2026, conforme o perito FULANO DE TAL.",
	} {
		got := resumirChancelas(comMiolo(l), false)
		if !slices.Equal(got, comMiolo(l)) {
			t.Errorf("%q devia ficar intacta: %q", l, got)
		}
	}
	if got := resumirChancelas(comMiolo("Este laudo foi assinado eletronicamente."), false); !slices.Equal(got, miolo3) {
		t.Errorf("frase fixa com ponto devia sumir: %q", got)
	}
}

func TestChancela_ParaConferirNoMaximoUmaQuebra(t *testing.T) {
	fim := []string{
		"Para conferir o original, acesse o site https://esaj.exemplo.jus.br, informe o processo",
		"5000000-00.2026.8.26.0001",
		"Conclusão do parecer.",
		"Assim, opino pela procedência e código 3 do anexo.",
	}
	got := resumirChancelas(comMiolo(fim...), false)
	if !slices.Contains(got, "Conclusão do parecer.") || !slices.Contains(got, fim[3]) {
		t.Errorf("atravessou mais de uma quebra: %q", got)
	}
}

func TestPadrao_MarcadoresDiferentesFicam(t *testing.T) {
	a := "Assinado digitalmente por FULANO DE TAL - Juiz de Direito"
	b := "Assinado digitalmente por BELTRANO DE TAL - Escrivão"
	saida, _ := GenerateConsolidatedOutput(docPadrao(pg("", "um", a), pg("", "dois", b)), NivelPadrao)
	if !strings.Contains(pagina(t, saida, 1), "[assinado: FULANO DE TAL, Juiz de Direito]") ||
		!strings.Contains(pagina(t, saida, 2), "[assinado: BELTRANO DE TAL, Escrivão]") {
		t.Errorf("assinaturas diferentes devem ficar: %s", saida)
	}
}

func TestChancela_Signatario(t *testing.T) {
	sig := []string{
		"Documento eletrônico 0000001 assinado eletronicamente nos termos do Art.1º §2º inciso III da Lei 11.419/2006",
		"Signatário(a): FULANO DE TAL",
	}
	if got := resumirChancelas(comMiolo(sig...), false); !slices.Equal(got, comMiolo("[assinado: FULANO DE TAL]")) {
		t.Errorf("got %q", got)
	}
}

func TestChancela_LaudoEConferenciaSomem(t *testing.T) {
	fim := []string{
		"Este laudo foi assinado eletronicamente",
		"Você pode conferir a autenticidade do documento em https://exemplo.jus.br/conferir com o código XYZ123",
	}
	if got := resumirChancelas(comMiolo(fim...), false); !slices.Equal(got, miolo3) {
		t.Errorf("got %q", got)
	}
}

func TestChancela_FraseNoMeioFica(t *testing.T) {
	frase := "o código verificador não confere com o documento assinado por FULANO DE TAL"
	cartorio := "Assinado digitalmente por FULANO DE TAL - Juiz de Direito"
	var ls []string
	for i := 1; i <= 20; i++ {
		switch i {
		case 10:
			ls = append(ls, frase)
		case 11:
			ls = append(ls, cartorio)
		default:
			ls = append(ls, "linha "+strconv.Itoa(i))
		}
	}
	pag := strings.Join(ls, "\n")
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		got := cleanPage(pag, nivel, &ctxDocumento{jaVistas: map[string]bool{}})
		if !strings.Contains(got, frase) || !strings.Contains(got, cartorio) {
			t.Errorf("%s: frase do miolo devia ficar: %q", nivel, got)
		}
		if strings.Contains(got, "[assinado") {
			t.Errorf("%s: miolo não vira marcador: %q", nivel, got)
		}
	}
}

func TestBasica_DocumentoAssinadoFica(t *testing.T) {
	l := "Documento assinado eletronicamente por FULANO"
	pag := "inicio\n" + l + "\nfim"
	for _, nivel := range []string{NivelBasico, NivelPadrao, NivelMaximo} {
		if got := cleanPage(pag, nivel, &ctxDocumento{jaVistas: map[string]bool{}}); !strings.Contains(got, l) {
			t.Errorf("%s devia manter %q: %q", nivel, l, got)
		}
	}
}

func TestCleanPage_ChancelaSoNaPadraoENaMaxima(t *testing.T) {
	pag := strings.Join(comMiolo("Assinado digitalmente por FULANO DE TAL - Juiz de Direito"), "\n")
	if got := cleanPage(pag, NivelBasico, nil); strings.Contains(got, "[assinado") {
		t.Errorf("Básica não troca chancela: %q", got)
	}
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		if got := cleanPage(pag, nivel, nil); !strings.Contains(got, "[assinado: FULANO DE TAL, Juiz de Direito]") || strings.Contains(got, "Assinado digitalmente") {
			t.Errorf("%s devia trocar a chancela: %q", nivel, got)
		}
	}
}

func docNomeado(nome string, paginas ...string) map[string]any {
	ev := docPadrao(paginas...)
	ev["1"].(map[string]any)["documentos"].(map[string]any)["d"].(map[string]any)["nome"] = nome
	return ev
}

func TestGenerate_HTMLSaiSoComNome(t *testing.T) {
	pag := strings.Join(comMiolo(blocoEproc...), "\n")
	for _, c := range []struct{ nome, want string }{
		{"Evento 1 - DESPADEC1.html", "[assinado: FULANO DE TAL]"},
		{"Evento 1 - DESPADEC1.HTM", "[assinado: FULANO DE TAL]"},
		{"Evento 1 - DESPADEC1.pdf", "[assinado: FULANO DE TAL, Juiz de Direito]"},
	} {
		saida, _ := GenerateConsolidatedOutput(docNomeado(c.nome, pag), NivelPadrao)
		if !strings.Contains(saida, c.want) {
			t.Errorf("%s: falta %q em %q", c.nome, c.want, saida)
		}
	}
}

func TestPadrao_BordaRepetidaNaoApagaMarcador(t *testing.T) {
	ass := "Assinado digitalmente por FULANO DE TAL - Juiz de Direito em 01/01/2026"
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		saida, _ := GenerateConsolidatedOutput(docPadrao(
			pg("TIMBRE", "um", ass), pg("TIMBRE", "dois", ass), pg("TIMBRE", "tres", ass)), nivel)
		for n := 1; n <= 3; n++ {
			p := pagina(t, saida, n)
			// Marcador idêntico fica só na primeira ocorrência do documento, como o timbre.
			if want := n == 1; strings.Contains(p, "[assinado: FULANO DE TAL, Juiz de Direito, 01/01/2026]") != want {
				t.Errorf("%s pág. %d: marcador presente = %v, quer %v: %q", nivel, n, !want, want, p)
			}
			if strings.Contains(p, "Assinado digitalmente") {
				t.Errorf("%s pág. %d manteve a chancela: %q", nivel, n, p)
			}
		}
	}
}

func TestCleanPage_MaximaTiraCabecalhoComEspacoIrregular(t *testing.T) {
	cab := "TRIBUNAL  DE JUSTICA\x01 FICTICIO"
	pags := []string{
		cab + "\n-----\n" + pg("", "um", ""),
		"TRIBUNAL DE JUSTICA FICTICIO\n_____\n" + pg("", "dois", ""),
		"TRIBUNAL\tDE JUSTICA FICTICIO\n=====\n" + pg("", "tres", ""),
	}
	ctx := &ctxDocumento{repetidas: repeatedEdgeLines(pags)}
	for i, p := range pags {
		got := cleanPage(p, NivelMaximo, ctx)
		if want := i == 0; strings.Contains(got, "TRIBUNAL") != want {
			t.Errorf("pagina %d: cabecalho presente = %v, quer %v (%q)", i, !want, want, got)
		}
	}
}
