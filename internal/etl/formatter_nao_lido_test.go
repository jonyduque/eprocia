package etl

import (
	"strconv"
	"strings"
	"testing"
)

// docNL monta um documento de teste com as páginas dadas; naoLido e semOCR
// reproduzem o que o pipeline grava no mapa.
func docNL(index int, nome, naoLido string, semOCR bool, paginas ...string) map[string]any {
	pags := map[string]any{}
	for j, c := range paginas {
		p := map[string]any{"index": j + 1, "ocr": false, "content": c}
		if semOCR {
			p["sem_ocr"] = true
		}
		pags[strconv.Itoa(j+1)] = p
	}
	d := map[string]any{"index": index, "nome": nome, "paginas": pags, "total_paginas": len(paginas)}
	if naoLido != "" {
		d["nao_lido"] = naoLido
	}
	return d
}

func eventoNL(docs ...map[string]any) map[string]any {
	m := map[string]any{}
	for i, d := range docs {
		m["d"+strconv.Itoa(i)] = d
	}
	return map[string]any{"1": map[string]any{"index": 1, "documentos": m}}
}

func TestSaida_ContagemDeNaoLidos(t *testing.T) {
	ev := eventoNL(
		docNL(1, "Evento 1 - A.pdf", "", false, "texto a"),
		docNL(2, "Evento 1 - B.pdf", "arquivo ilegível", false, "[ERRO ao analisar PDF: x]"),
		docNL(3, "Evento 1 - C.pdf", "", false, "texto c"),
	)
	saida, _ := GenerateConsolidatedOutput(ev, NivelPadrao)
	if !strings.Contains(saida, "Total de Documentos: 3\nDocumentos não lidos: 1\n") {
		t.Errorf("faltou a contagem 1:\n%s", saida)
	}
	ev = eventoNL(docNL(1, "Evento 1 - A.pdf", "", false, "texto a"))
	saida, _ = GenerateConsolidatedOutput(ev, NivelPadrao)
	if !strings.Contains(saida, "Documentos não lidos: 0") {
		t.Errorf("faltou a contagem 0:\n%s", saida)
	}
}

func TestSaida_SummaryNaoLido(t *testing.T) {
	ev := eventoNL(docNL(1, "Evento 1 - B.pdf", "arquivo ilegível", false, "[ERRO ao analisar PDF: x]"))
	saida, _ := GenerateConsolidatedOutput(ev, NivelPadrao)
	var linha string
	for l := range strings.SplitSeq(saida, "\n") {
		if strings.HasPrefix(l, "- Evento 1 - ") {
			linha = l
		}
	}
	if !strings.HasSuffix(linha, ": NÃO LIDO") || strings.Contains(linha, "página") {
		t.Errorf("linha do summary = %q", linha)
	}
}

func TestSaida_CorpoNaoLido(t *testing.T) {
	ev := eventoNL(docNL(1, "Evento 1 - B.pdf", "arquivo ilegível", false, "[ERRO ao analisar PDF: x]"))
	saida, _ := GenerateConsolidatedOutput(ev, NivelPadrao)
	marca := "[documento não lido: arquivo ilegível; consulte o PDF original]"
	i, j := strings.Index(saida, marca), strings.Index(saida, "[ERRO ao analisar PDF: x]")
	if i < 0 || j < 0 || i > j {
		t.Errorf("ordem/presença erradas (%d, %d):\n%s", i, j, saida)
	}
	if n := strings.Count(saida, "[ERRO"); n != 1 {
		t.Errorf("[ERRO apareceu %d vezes:\n%s", n, saida)
	}
}

func TestSaida_CorpoNaoLidoHTML(t *testing.T) {
	ev := eventoNL(docNL(1, "Evento 1 - B.html", "arquivo ilegível", false, ""))
	saida, _ := GenerateConsolidatedOutput(ev, NivelPadrao)
	if !strings.Contains(saida, "[documento não lido: arquivo ilegível; consulte o arquivo original]") {
		t.Errorf("faltou o marcador de HTML:\n%s", saida)
	}
}

func TestSaida_NaoLidoSemOCR_SoOMarcador(t *testing.T) {
	pags := make([]string, 6)
	for i := range pags {
		pags[i] = "[página digitalizada: o OCR está desativado]"
	}
	ev := eventoNL(docNL(1, "Evento 1 - D.pdf", "digitalizado; o OCR está desativado", true, pags...))
	saida, _ := GenerateConsolidatedOutput(ev, NivelPadrao)
	if !strings.Contains(saida, "[documento não lido: digitalizado; o OCR está desativado; consulte o PDF original]") {
		t.Errorf("faltou o marcador:\n%s", saida)
	}
	if strings.Contains(saida, "<pag") || strings.Contains(saida, "[página digitalizada") {
		t.Errorf("páginas não deviam sair:\n%s", saida)
	}
}

func TestSaida_NaoLidosIguaisNaoViramDuplicata(t *testing.T) {
	erro := "[ERRO ao analisar PDF: " + strings.Repeat("falha longa ", 30) + "]"
	ev := eventoNL(
		docNL(1, "Evento 1 - A.pdf", "arquivo ilegível", false, erro),
		docNL(2, "Evento 1 - B.pdf", "arquivo ilegível", false, erro),
	)
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		saida, _ := GenerateConsolidatedOutput(ev, nivel)
		if strings.Contains(saida, "idêntic") {
			t.Errorf("%s: não lidos viraram duplicata:\n%s", nivel, saida)
		}
		if n := strings.Count(saida, "[ERRO"); n != 2 {
			t.Errorf("%s: [ERRO apareceu %d vezes", nivel, n)
		}
	}
}

func TestSaida_PaginaSemOCRNaoSomeNaPadrao(t *testing.T) {
	marca := "[página digitalizada: o OCR está desativado]"
	d := docNL(1, "Evento 1 - M.pdf", "", false,
		textoLongo("p1", 250), marca, textoLongo("p3", 250), marca, marca)
	for _, i := range []string{"2", "4", "5"} {
		d["paginas"].(map[string]any)[i].(map[string]any)["sem_ocr"] = true
	}
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		saida, _ := GenerateConsolidatedOutput(eventoNL(d), nivel)
		if n := strings.Count(saida, marca); n != 3 {
			t.Errorf("%s: marcador apareceu %d vezes, esperado 3:\n%s", nivel, n, saida)
		}
	}
}
