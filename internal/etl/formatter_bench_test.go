package etl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// processoSintetico monta um mapa de eventos fictício com nEventos eventos de
// um documento de pagsPorDoc páginas de 45 linhas cada: cabeçalho e rodapé
// repetidos, marcador de página, chancela do e-SAJ no fim, linhas com espaço
// duplo, tab, NBSP, espaço Unicode e byte inválido, páginas idênticas (H7),
// uma página com ocr_aviso, uma sem_ocr, um documento omitido e um não lido.
func processoSintetico(nEventos, pagsPorDoc int) map[string]any {
	eventos := map[string]any{}
	for e := 1; e <= nEventos; e++ {
		pags := map[string]any{}
		for p := 1; p <= pagsPorDoc; p++ {
			var b strings.Builder
			b.WriteString("PODER JUDICIÁRIO\nTRIBUNAL DE JUSTIÇA DO ESTADO FICTÍCIO\nComarca de Exemplo - Vara Cível\n")
			for l := range 38 {
				semente := (e*31 + p*7 + l) % 97
				if p%10 == 0 && e%2 == 0 {
					// Página repetida entre eventos: entra no H7.
					semente = l
				}
				switch semente % 13 {
				case 0:
					fmt.Fprintf(&b, "  Linha %d com  espaço duplo e\ttab \n", semente)
				case 1:
					fmt.Fprintf(&b, "Valor de R$ 1.%03d,00 em favor da parte autora (NBSP)\n", semente)
				case 2:
					fmt.Fprintf(&b, "Texto com espaço largo %d\r\n", semente)
				case 3:
					fmt.Fprintf(&b, "Byte inválido \xff no meio %d\n", semente)
				case 4:
					b.WriteString("----------\n")
				case 5:
					b.WriteString("\n")
				case 6:
					fmt.Fprintf(&b, "a parte requerida, citada em %d, apresentou contestação\n", semente)
				case 7:
					b.WriteString("x\n")
				default:
					fmt.Fprintf(&b, "Considerando os fatos narrados na petição inicial, item %d, o pedido é procedente em parte.\n", semente)
				}
			}
			fmt.Fprintf(&b, "Página %d de %d\n", p, pagsPorDoc)
			b.WriteString("Este documento é cópia do original, assinado digitalmente por FULANO DE TAL, protocolado em 01/02/2026\n")
			b.WriteString("Rua Fictícia, 100 - Centro - CEP 00000-000\nfl. " + strconv.Itoa(p))
			pag := map[string]any{"index": p, "ocr": false, "content": b.String()}
			if e == 3 && p == 2 {
				pag["ocr_aviso"] = true
			}
			if e == 3 && p == 5 {
				pag["ocr_aviso"] = true
				pag["content"] = pags["4"].(map[string]any)["content"]
			}
			if e == 4 && p == 3 {
				pag["sem_ocr"] = true
				pag["content"] = "[página digitalizada sem OCR]"
			}
			pags[strconv.Itoa(p)] = pag
		}
		doc := map[string]any{"index": 1, "nome": fmt.Sprintf("Evento %d - DOC%d.pdf", e, e), "paginas": pags, "total_paginas": pagsPorDoc}
		docs := map[string]any{"d1": doc}
		if e == 2 {
			docs["d2"] = map[string]any{"index": 2, "nome": "Evento 2 - PROC1.pdf", "paginas": map[string]any{}, "omitido": "PROC", "total_paginas": 3}
			docs["d3"] = map[string]any{"index": 3, "nome": "Evento 2 - X1.pdf", "nao_lido": "arquivo ilegível",
				"paginas": map[string]any{"1": map[string]any{"index": 1, "content": "[ERRO ao analisar PDF: x]"}}}
		}
		eventos[strconv.Itoa(e)] = map[string]any{"index": e, "documentos": docs}
	}
	return eventos
}

var linhaDataConsolidacao = regexp.MustCompile(`Data de Consolidação: [^\n]*\n`)

// A saída do formatador sobre o processo sintético (com ocr_aviso, sem_ocr,
// duplicadas, omitido e não lido) não muda: as somas foram gravadas antes de
// as linhas básicas passarem a ser calculadas uma vez por página.
func TestGenerateConsolidatedOutput_SaidaFixada(t *testing.T) {
	quer := map[string]string{
		NivelNenhum: "f2931e18285a782a43c19e4f594e98192ab0d5abdb13faff45e7e3dc7bc37bf5",
		NivelBasico: "5e09ead4041472e03034764701f2d2d4f0b272cd18bf27548a006b209d8b6889",
		NivelPadrao: "a4a9f4eced0b0161e71c04bf840a903b8a9466e63744ae704658c257ff51a304",
		NivelMaximo: "39286b388256492b47d8a3a82d233dcb1678a22958597701c9456c0129f4bf12",
	}
	for _, nivel := range niveisDeLimpeza {
		saida, _ := GenerateConsolidatedOutput(processoSintetico(6, 12), nivel)
		saida = linhaDataConsolidacao.ReplaceAllString(saida, "")
		soma := sha256.Sum256([]byte(saida))
		if got := hex.EncodeToString(soma[:]); got != quer[nivel] {
			t.Errorf("%s: soma %s, quer %s", nivel, got, quer[nivel])
		}
	}
}

// linhasBasicasReferencia é a versão de linhasBasicas sem o caminho rápido.
func linhasBasicasReferencia(text string) []string {
	var out []string
	for linha := range strings.SplitSeq(text, "\n") {
		linha = strings.Map(func(r rune) rune {
			switch {
			case r == '\t' || r == ' ' || r == '\v' || r == '\f' || r == '\u0085':
				return ' '
			case r == '\r' || unicode.IsControl(r):
				return -1
			}
			return r
		}, linha)
		linha = strings.Join(strings.Fields(linha), " ")
		if linha == "" || linhaSeparadora(linha) {
			continue
		}
		out = append(out, linha)
	}
	return out
}

var sementesLinhasBasicas = []string{
	"", "a", " a", "a ", "a  b", "a b c", "a\tb", "a b", "a b", "a　b", "a​b",
	"a\x01b", "a\u0085b", "a\u0080b", "a\rb", "a\vb\fc", "\xff", "a\xc3", "�", "ção é ótima",
	"-----", "- - - - -", "Item ........ 12", "linha um\nlinha  dois\n\n\tlinha três ",
	"Página 1 de 3\r\nR$ 1.000,00 ", "a b", "a b", "a b", "a b",
}

func TestLinhasBasicas_IgualAReferencia(t *testing.T) {
	for _, s := range sementesLinhasBasicas {
		if got, want := linhasBasicas(s), linhasBasicasReferencia(s); !slices.Equal(got, want) {
			t.Errorf("linhasBasicas(%q) = %q, quer %q", s, got, want)
		}
	}
}

func FuzzLinhasBasicas(f *testing.F) {
	for _, s := range sementesLinhasBasicas {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if got, want := linhasBasicas(s), linhasBasicasReferencia(s); !slices.Equal(got, want) {
			t.Errorf("linhasBasicas(%q) = %q, quer %q", s, got, want)
		}
	})
}

// Linha já limpa é reaproveitada sem cópia: a única alocação é a fatia.
func TestLinhasBasicas_LinhaLimpaNaoAloca(t *testing.T) {
	pag := strings.Repeat("Considerando os fatos narrados na petição inicial, o pedido é procedente.\n", 45)
	if n := testing.AllocsPerRun(20, func() { linhasBasicas(pag) }); n > 1 {
		t.Errorf("linhasBasicas fez %v alocações numa página limpa, quer no máximo 1", n)
	}
}

func BenchmarkGenerateConsolidatedOutput(b *testing.B) {
	eventos := processoSintetico(20, 50) // 1000 páginas
	for _, nivel := range []string{NivelPadrao, NivelMaximo} {
		b.Run(nivel, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				GenerateConsolidatedOutput(eventos, nivel)
			}
		})
	}
}
