package etl

import (
	"strings"
	"testing"
)

var niveisDeLimpeza = []string{NivelNenhum, NivelBasico, NivelPadrao, NivelMaximo}

// marcarPagina grava um campo na página n (1-based) do documento de teste.
func marcarPagina(d map[string]any, n string, campo string, valor any) {
	d["paginas"].(map[string]any)[n].(map[string]any)[campo] = valor
}

// Os dois textos da falha do OCR pedem ao modelo que avise o usuário, e não
// trazem caminho nem detalhe do erro além da frase fixa.
func TestFalhaOCR_TextosPedemAvisoAoUsuario(t *testing.T) {
	for _, s := range []string{marcadorOCRPrazo, marcadorOCRFalha, avisoOCRComReserva, marcadorErroOCR(frasePaginaOCRFalha)} {
		if !strings.Contains(s, "avise o usuário") || !strings.Contains(s, "PDF original") {
			t.Errorf("texto sem o pedido de aviso: %q", s)
		}
		if !strings.Contains(s, "erro no OCR") {
			t.Errorf("texto não diz que houve erro no OCR: %q", s)
		}
		if strings.ContainsAny(s, "\n\\") || pareceCaminho.MatchString(s) {
			t.Errorf("texto com quebra de linha ou caminho: %q", s)
		}
	}
	if marcadorErroOCR(frasePaginaOCRPrazo) != marcadorOCRPrazo || marcadorErroOCR(frasePaginaOCRFalha) != marcadorOCRFalha {
		t.Errorf("marcadorErroOCR não bate com as constantes")
	}
	if !strings.HasPrefix(marcadorOCRFalha, "[ERRO de OCR: ") {
		t.Errorf("marcador sem o prefixo que classificarPaginas reconhece: %q", marcadorOCRFalha)
	}
}

// Página cujo OCR falhou e ficou com o texto nativo: o aviso sai uma vez, antes
// do texto, em todos os níveis de limpeza.
func TestFalhaOCR_AvisoAntesDoTextoDeReserva(t *testing.T) {
	for _, nivel := range niveisDeLimpeza {
		t.Run(nivel, func(t *testing.T) {
			d := docNL(1, "Evento 1 - A.pdf", "", false,
				"Texto da pagina um do documento ficticio.",
				"Texto nativo da pagina dois.",
				"Texto da pagina tres do documento ficticio.")
			marcarPagina(d, "2", "ocr_aviso", true)
			saida, _ := GenerateConsolidatedOutput(eventoNL(d), nivel)
			quer := "<pag n=\"2\">\n" + avisoOCRComReserva + "\nTexto nativo da pagina dois.\n</pag>\n"
			if !strings.Contains(saida, quer) {
				t.Errorf("página 2 sem o aviso antes do texto:\n%s", saida)
			}
			if n := strings.Count(saida, avisoOCRComReserva); n != 1 {
				t.Errorf("aviso aparece %d vezes, quer 1:\n%s", n, saida)
			}
		})
	}
}

// Documento de uma página: o aviso e o texto saem direto, sem <pag>, e o
// documento continua lido.
func TestFalhaOCR_AvisoEmDocumentoDeUmaPagina(t *testing.T) {
	d := docNL(1, "Evento 1 - A.pdf", "", false, "Texto nativo da pagina unica.")
	marcarPagina(d, "1", "ocr_aviso", true)
	saida, _ := GenerateConsolidatedOutput(eventoNL(d), NivelPadrao)
	quer := "nome=\"Evento 1 - A.pdf\">\n" + avisoOCRComReserva + "\nTexto nativo da pagina unica.\n</documento>\n"
	if !strings.Contains(saida, quer) {
		t.Errorf("documento sem o aviso antes do texto:\n%s", saida)
	}
	if !strings.Contains(saida, "Documentos não lidos: 0\n") {
		t.Errorf("documento contou como não lido:\n%s", saida)
	}
}

// A página com aviso não vira referência a outra página nem serve de origem:
// o texto dela pode estar incompleto.
func TestFalhaOCR_PaginaComAvisoNaoEDuplicata(t *testing.T) {
	longo := strings.Repeat("Paragrafo ficticio repetido para passar do minimo de duplicata. ", 6)
	for _, comAviso := range []string{"d1", "d2"} {
		t.Run(comAviso, func(t *testing.T) {
			d1 := docNL(1, "Evento 1 - A.pdf", "", false, longo, "Outra pagina do primeiro documento.")
			d2 := docNL(2, "Evento 1 - B.pdf", "", false, longo, "Outra pagina do segundo documento.")
			if comAviso == "d1" {
				marcarPagina(d1, "1", "ocr_aviso", true)
			} else {
				marcarPagina(d2, "1", "ocr_aviso", true)
			}
			saida, _ := GenerateConsolidatedOutput(eventoNL(d1, d2), NivelPadrao)
			if n := strings.Count(saida, "Paragrafo ficticio repetido"); n != 12 {
				t.Errorf("texto longo aparece %d vezes, quer 12 (as duas páginas inteiras):\n%s", n, saida)
			}
			if n := strings.Count(saida, avisoOCRComReserva); n != 1 {
				t.Errorf("aviso aparece %d vezes, quer 1", n)
			}
		})
	}
}

// Página sem texto de reserva: o marcador sai inteiro, em todos os níveis,
// mesmo repetido em várias páginas do documento.
func TestFalhaOCR_MarcadorSemReservaSaiInteiro(t *testing.T) {
	for _, nivel := range niveisDeLimpeza {
		t.Run(nivel, func(t *testing.T) {
			d := docNL(1, "Evento 1 - A.pdf", "", false,
				"Texto da pagina um do documento ficticio.",
				marcadorOCRFalha, marcadorOCRFalha, marcadorOCRPrazo, marcadorOCRFalha)
			for _, n := range []string{"2", "3", "5"} {
				marcarPagina(d, n, "ocr_erro", frasePaginaOCRFalha)
			}
			marcarPagina(d, "4", "ocr_erro", frasePaginaOCRPrazo)
			marcarPagina(d, "4", "ocr_prazo", true)
			saida, _ := GenerateConsolidatedOutput(eventoNL(d), nivel)
			if n := strings.Count(saida, marcadorOCRFalha+"\n</pag>"); n != 3 {
				t.Errorf("marcador de falha aparece %d vezes, quer 3:\n%s", n, saida)
			}
			if !strings.Contains(saida, "<pag n=\"4\">\n"+marcadorOCRPrazo+"\n</pag>\n") {
				t.Errorf("marcador de prazo ausente na página 4:\n%s", saida)
			}
		})
	}
}

// Documento não lido por falha do OCR em página que tinha texto de reserva: a
// linha do documento vem seguida do marcador com o pedido de aviso, e o texto
// de reserva não sai.
func TestFalhaOCR_DocumentoNaoLidoLevaOPedidoDeAviso(t *testing.T) {
	d := docNL(1, "Evento 1 - A.pdf", LeituraOCRFalhou.Frase(), false, "Texto nativo curto ficticio")
	marcarPagina(d, "1", "ocr_erro", frasePaginaOCRFalha)
	marcarPagina(d, "1", "ocr_aviso", true)
	saida, _ := GenerateConsolidatedOutput(eventoNL(d), NivelPadrao)
	quer := "[documento não lido: " + LeituraOCRFalhou.Frase() + "; consulte o PDF original]\n" + marcadorOCRFalha + "\n</documento>\n"
	if !strings.Contains(saida, quer) {
		t.Errorf("documento não lido sem o marcador com o pedido de aviso:\n%s", saida)
	}
	if strings.Contains(saida, "Texto nativo curto ficticio") || strings.Contains(saida, avisoOCRComReserva) {
		t.Errorf("documento não lido trouxe o texto de reserva ou o aviso de página:\n%s", saida)
	}
}

// O marcador que já é o conteúdo da primeira página não sai duas vezes.
func TestFalhaOCR_DocumentoNaoLidoSemReservaNaoRepeteOMarcador(t *testing.T) {
	d := docNL(1, "Evento 1 - A.pdf", LeituraPrazo.Frase(), false, marcadorOCRPrazo)
	marcarPagina(d, "1", "ocr_erro", frasePaginaOCRPrazo)
	marcarPagina(d, "1", "ocr_prazo", true)
	saida, _ := GenerateConsolidatedOutput(eventoNL(d), NivelPadrao)
	if n := strings.Count(saida, marcadorOCRPrazo); n != 1 {
		t.Errorf("marcador aparece %d vezes, quer 1:\n%s", n, saida)
	}
}
