package etl

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// pageMarkerRegex matches a line that is *only* a page marker, such as
// "Página 12 de 35", "Pág. 3/10" or "página 4". Anchoring the whole line keeps
// legitimate sentences that merely mention the word "página" intact.
var pageMarkerRegex = regexp.MustCompile(`(?i)^p[áa]g(ina|\.)?\s*\d+(\s*(de|/)\s*\d+)?$`)

// escapeAttr escapes a value for safe use inside a double-quoted XML attribute.
func escapeAttr(s string) string {
	return attrEscaper.Replace(s)
}

var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// eventPrefixRegex matches the "Evento N - " opener of an Eproc file name.
var eventPrefixRegex = regexp.MustCompile(`(?i)^evento\s+\d+\s*-\s*`)

// summaryDocName turns "Evento 1 - INIC1.pdf" into "INIC1" for the summary
// line, which already prints the event number itself (PRD §6).
func summaryDocName(nome string) string {
	base := strings.TrimSuffix(nome, filepath.Ext(nome))
	return eventPrefixRegex.ReplaceAllString(base, "")
}

// digitosFinaisRegex pega os dígitos no fim do código do documento (EXTR2).
var digitosFinaisRegex = regexp.MustCompile(`\d+$`)

// tipoDocumento extrai o código de tipo do nome do arquivo: tira o prefixo
// "Evento N - ", a extensão e os dígitos finais, em maiúsculas
// ("Evento 3 - EXTR2.pdf" vira "EXTR").
func tipoDocumento(docName string) string {
	base := strings.TrimSuffix(docName, filepath.Ext(docName))
	base = eventPrefixRegex.ReplaceAllString(base, "")
	return strings.ToUpper(digitosFinaisRegex.ReplaceAllString(strings.TrimSpace(base), ""))
}

// pageCountLabel renders "1 página" / "N páginas".
func pageCountLabel(n int) string {
	if n == 1 {
		return "1 página"
	}
	return fmt.Sprintf("%d páginas", n)
}

// edgeLineCount is how many non-empty lines at the top and at the bottom of a
// page are candidates for repeated header/footer detection.
const edgeLineCount = 5

// valorEdgeRegex acha linhas com valor monetário ou percentual: nelas os
// dígitos importam, então a chave é a linha exata.
var valorEdgeRegex = regexp.MustCompile(`R\$|US\$|\d+,\d{2}|%`)

// paginaCurta informa se a página, com n linhas não vazias, é toda borda: com
// até 2*edgeLineCount linhas não sobra miolo, então fica fora da detecção.
func paginaCurta(n int) bool { return n <= 2*edgeLineCount }

// normalizeEdgeLine maps lines that differ only by numbers (página, folha, data)
// to the same key, so "fl. 1" and "fl. 2" count as the same footer.
func normalizeEdgeLine(trimmed string) string {
	if valorEdgeRegex.MatchString(trimmed) {
		return trimmed
	}
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return '#'
		}
		return r
	}, trimmed)
}

// edgeLines returns the normalized first and last edgeLineCount non-empty
// lines of a page, without duplicates.
func edgeLines(page string) map[string]bool {
	// Mesma limpeza e mesmas posições que cleanPage usa ao consultar o conjunto.
	nonEmpty := linhasBasicas(page)
	edges := make(map[string]bool)
	if paginaCurta(len(nonEmpty)) {
		return edges
	}
	for i, line := range nonEmpty {
		if i < edgeLineCount || i >= len(nonEmpty)-edgeLineCount {
			edges[normalizeEdgeLine(line)] = true
		}
	}
	return edges
}

// repeatedEdgeLines finds the normalized header/footer lines that appear at the
// edges of at least 60% of a document's pages (documents with 2+ pages only).
func repeatedEdgeLines(pages []string) map[string]bool {
	if len(pages) < 2 {
		return nil
	}
	counts := make(map[string]int)
	for _, page := range pages {
		for line := range edgeLines(page) {
			counts[line]++
		}
	}
	var repeated map[string]bool
	for line, n := range counts {
		if n*5 >= len(pages)*3 {
			if repeated == nil {
				repeated = make(map[string]bool)
			}
			repeated[line] = true
		}
	}
	return repeated
}

// cleanPage limpa uma página conforme o nível (none, basic, standard,
// aggressive). Níveis desconhecidos valem como basic. Em standard e aggressive
// tira marcadores de página e linhas só de símbolos e as bordas listadas em
// ctx.repetidas (ver repeatedEdgeLines): só as posições de borda contam, então a
// mesma frase no meio da página sobrevive. O timbre fica na primeira página em
// que aparece (ctx.jaVistas). As chancelas de assinatura nas bordas viram
// "[assinado: ...]" (resumirChancelas) antes das bordas repetidas, e o marcador
// nunca passa por elas. Só aggressive troca a página de certificado de
// plataforma pelo marcador (antes de tudo, sem tocar no estado do documento:
// o timbre fica para a próxima página de conteúdo), tira as letras soltas
// (H3b) e reflui os parágrafos (H9).
func cleanPage(text string, level string, ctx *ctxDocumento) string {
	if level == NivelNenhum {
		return text
	}
	lines := linhasBasicas(text)
	if level != NivelPadrao && level != NivelMaximo {
		return strings.Join(lines, "\n")
	}
	if level == NivelMaximo && paginaCertificado(lines) {
		return marcadorCertificado
	}

	var repeated map[string]bool
	html := false
	if ctx != nil {
		repeated = ctx.repetidas
		html = ctx.html
		if ctx.jaVistas == nil {
			ctx.jaVistas = map[string]bool{}
		}
	}
	// As posições continuam as de lines, as mesmas que edgeLines usa.
	saem, marcadores := planoChancelas(lines, html)
	var vistasNaPagina []string
	var cleanedLines []string
	for pos, line := range lines {
		if saem[pos] {
			if m, ok := marcadores[pos]; ok {
				if ctx != nil {
					if ctx.assinados[m] {
						continue
					}
					if ctx.assinados == nil {
						ctx.assinados = map[string]bool{}
					}
					ctx.assinados[m] = true
				}
				cleanedLines = append(cleanedLines, m)
			}
			continue
		}
		if pageMarkerRegex.MatchString(line) || linhaSoSimbolos(line) || (level == NivelMaximo && linhaLetraSolta(line)) {
			continue
		}
		if len(repeated) > 0 && !paginaCurta(len(lines)) && (pos < edgeLineCount || pos >= len(lines)-edgeLineCount) {
			if chave := normalizeEdgeLine(line); repeated[chave] {
				vistasNaPagina = append(vistasNaPagina, chave)
				if ctx.jaVistas[chave] {
					continue
				}
			}
		}
		cleanedLines = append(cleanedLines, line)
	}
	for _, chave := range vistasNaPagina {
		ctx.jaVistas[chave] = true
	}
	if level == NivelPadrao {
		return strings.Join(cleanedLines, "\n")
	}
	return strings.Join(refluir(cleanedLines), "\n")
}

// duplicadaMinRunas é o tamanho mínimo (em runas, após linhasBasicas) de uma
// página para entrar na detecção de duplicatas (H7).
const duplicadaMinRunas = 200

// chaveDoc identifica um documento pelo evento e pelo índice (o nome pode repetir).
type chaveDoc struct{ evento, doc int }

// origemPagina identifica a primeira ocorrência de um conteúdo; nome é o nome
// curto do documento (summaryDocName), usado só no texto da referência.
type origemPagina struct {
	chaveDoc
	nome string
	pag  int
}

// chavePagina localiza uma página na saída: evento, índice do documento e da página.
type chavePagina struct{ evento, doc, pag int }

// refPagina e refDocumento são os marcadores que H7 deixa no lugar do texto.
func (o origemPagina) refPagina() string {
	return fmt.Sprintf("[conteúdo idêntico a: evento %d, %s, pág. %d]", o.evento, o.nome, o.pag)
}

func refDocumento(evento int, nome string) string {
	return fmt.Sprintf("[documento idêntico a: evento %d, %s]", evento, nome)
}

// paginaOrdenada e docOrdenado guardam a ordem de saída: documentos por index
// dentro do evento e páginas por index dentro do documento.
type paginaOrdenada struct {
	index   int
	content string
}

type docOrdenado struct {
	nome    string
	index   int
	paginas []paginaOrdenada
	// omitido é o tipo do documento que o usuário mandou omitir ("" se não);
	// totalPaginas é o total de páginas dele, contado na origem.
	omitido      string
	totalPaginas int
}

// documentosOrdenados devolve os documentos de um evento, e as páginas de cada
// um, na ordem de saída. É a única fonte dessa ordem.
func documentosOrdenados(evData map[string]any) []docOrdenado {
	var docs []docOrdenado
	for _, val := range evData["documentos"].(map[string]any) {
		dData := val.(map[string]any)
		d := docOrdenado{nome: dData["nome"].(string), index: dData["index"].(int)}
		d.omitido, _ = dData["omitido"].(string)
		d.totalPaginas, _ = dData["total_paginas"].(int)
		for _, pv := range dData["paginas"].(map[string]any) {
			pData := pv.(map[string]any)
			// Campos ausentes são tolerados.
			c, _ := pData["content"].(string)
			d.paginas = append(d.paginas, paginaOrdenada{pData["index"].(int), c})
		}
		slices.SortFunc(d.paginas, func(a, b paginaOrdenada) int { return cmp.Compare(a.index, b.index) })
		docs = append(docs, d)
	}
	slices.SortFunc(docs, func(a, b docOrdenado) int { return cmp.Compare(a.index, b.index) })
	return docs
}

// duplicadasH7 é o resultado da pré-passada: páginas repetidas e documentos
// idênticos, por inteiro, a um documento anterior.
type duplicadasH7 struct {
	paginas    map[chavePagina]origemPagina
	documentos map[chaveDoc]origemPagina // origemPagina.pag não é usado
}

// chaveConteudo é o sha256 das linhas básicas unidas por espaço; "" quando a
// página tem menos de duplicadaMinRunas runas.
func chaveConteudo(content string) string {
	juntas := strings.Join(linhasBasicas(content), " ")
	if utf8.RuneCountInString(juntas) < duplicadaMinRunas {
		return ""
	}
	soma := sha256.Sum256([]byte(juntas))
	return string(soma[:])
}

// duplicadasPorConteudo percorre eventos, documentos e páginas na ordem de saída
// e devolve, para cada página repetida, a origem da primeira ocorrência. Um
// documento só é idêntico a outro se tem o mesmo número de páginas e a página i
// de um tem a mesma chave da página i do outro, para todo i (e todas longas).
func duplicadasPorConteudo(eventos map[string]any, eventNums []int) duplicadasH7 {
	primeiras := map[string]origemPagina{}
	chavesDoc := map[chaveDoc][]string{}
	nomes := map[chaveDoc]string{}
	res := duplicadasH7{
		paginas:    map[chavePagina]origemPagina{},
		documentos: map[chaveDoc]origemPagina{},
	}
	for _, num := range eventNums {
		for _, d := range documentosOrdenados(eventos[fmt.Sprintf("%d", num)].(map[string]any)) {
			if d.omitido != "" {
				// Omitido não é lido: fica fora da detecção e ninguém o referencia.
				continue
			}
			cd := chaveDoc{num, d.index}
			nomeCurto := summaryDocName(d.nome)
			nomes[cd] = nomeCurto
			chaves := make([]string, len(d.paginas))
			todasDup := len(d.paginas) > 0
			for i, p := range d.paginas {
				chave := chaveConteudo(p.content)
				chaves[i] = chave
				if chave == "" {
					todasDup = false
					continue
				}
				if o, ok := primeiras[chave]; ok {
					res.paginas[chavePagina{num, d.index, p.index}] = o
				} else {
					primeiras[chave] = origemPagina{cd, nomeCurto, p.index}
					todasDup = false
				}
			}
			chavesDoc[cd] = chaves
			if !todasDup {
				continue
			}
			origem := res.paginas[chavePagina{num, d.index, d.paginas[0].index}].chaveDoc
			if slices.Equal(chavesDoc[origem], chaves) {
				res.documentos[cd] = origemPagina{chaveDoc: origem, nome: nomes[origem]}
			}
		}
	}
	return res
}

// EstimateTokens approximates the LLM token count of text as one token per four
// characters, a local heuristic with no tokenizer or network access.
func EstimateTokens(text string) int {
	return utf8.RuneCountInString(text) / 4
}

// tokensLine renders the <contexto> line that carries the token estimate.
func tokensLine(n int) string {
	return fmt.Sprintf("Tokens estimados: %d\n", n)
}

// GenerateConsolidatedOutput formats all extracted process data into structured
// XML tags and returns it with the token estimate written in <contexto>, which
// equals EstimateTokens of the returned text.
func GenerateConsolidatedOutput(eventos map[string]any, cleaningLevel string) (string, int) {
	var sb strings.Builder

	// 1. Sort event numbers chronologically
	var eventNums []int
	for _, val := range eventos {
		evData := val.(map[string]any)
		eventNums = append(eventNums, evData["index"].(int))
	}
	slices.Sort(eventNums)

	// Count total documents
	totalDocs := 0
	for _, val := range eventos {
		evData := val.(map[string]any)
		documentosMap := evData["documentos"].(map[string]any)
		totalDocs += len(documentosMap)
	}

	// Header/Context info block
	sb.WriteString("<contexto>\n")
	sb.WriteString("PROCESSO JUDICIAL - ORDEM CRONOLÓGICA\n")
	sb.WriteString(fmt.Sprintf("Total de Documentos: %d\n", totalDocs))
	sb.WriteString(fmt.Sprintf("Data de Consolidação: %s\n", time.Now().Format("02/01/2006 15:04:05")))
	tokensAt := sb.Len()
	sb.WriteString("\n<summary>\n")

	// Sort events and documents to build the summary block
	for _, num := range eventNums {
		eventKey := fmt.Sprintf("%d", num)
		evData := eventos[eventKey].(map[string]any)
		documentosMap := evData["documentos"].(map[string]any)

		type docSummaryEntry struct {
			nome  string
			index int
			pages int
		}
		var docs []docSummaryEntry
		for _, val := range documentosMap {
			dData := val.(map[string]any)
			paginasMap := dData["paginas"].(map[string]any)
			paginas := len(paginasMap)
			if _, omitido := dData["omitido"].(string); omitido {
				paginas, _ = dData["total_paginas"].(int)
			}
			docs = append(docs, docSummaryEntry{
				nome:  dData["nome"].(string),
				index: dData["index"].(int),
				pages: paginas,
			})
		}
		slices.SortFunc(docs, func(a, b docSummaryEntry) int {
			return cmp.Compare(a.index, b.index)
		})

		for _, d := range docs {
			sb.WriteString(fmt.Sprintf("- Evento %d - %s: %s\n", num, summaryDocName(d.nome), pageCountLabel(d.pages)))
		}
	}
	sb.WriteString("</summary>\n")
	sb.WriteString("</contexto>\n\n")
	header := sb.String()
	sb.Reset()

	// H7: na Padrão e na Máxima, páginas de 200+ runas com o mesmo conteúdo
	// (após linhasBasicas) viram referência à primeira ocorrência, e o documento
	// idêntico a outro, página a página, vira uma referência só.
	var h7 duplicadasH7
	if cleaningLevel == NivelPadrao || cleaningLevel == NivelMaximo {
		h7 = duplicadasPorConteudo(eventos, eventNums)
	}

	// Loop over each Event chronologically
	for _, num := range eventNums {
		evData := eventos[fmt.Sprintf("%d", num)].(map[string]any)
		sb.WriteString(fmt.Sprintf("<evento num=\"%d\">\n", num))

		for _, d := range documentosOrdenados(evData) {
			sb.WriteString(fmt.Sprintf("<documento index=\"%d\" nome=\"%s\">\n", d.index, escapeAttr(d.nome)))

			if d.omitido != "" {
				sb.WriteString(fmt.Sprintf("[documento omitido: %s, %s]\n</documento>\n", d.omitido, pageCountLabel(d.totalPaginas)))
				continue
			}

			if o, ok := h7.documentos[chaveDoc{num, d.index}]; ok {
				sb.WriteString(refDocumento(o.evento, o.nome))
				sb.WriteString("\n</documento>\n")
				continue
			}

			ext := strings.ToLower(filepath.Ext(d.nome))
			ctx := &ctxDocumento{jaVistas: map[string]bool{}, html: ext == ".html" || ext == ".htm"}
			if cleaningLevel == NivelPadrao || cleaningLevel == NivelMaximo {
				contents := make([]string, len(d.paginas))
				for i, p := range d.paginas {
					contents[i] = p.content
				}
				ctx.repetidas = repeatedEdgeLines(contents)
			}

			// Limpa todas as páginas na ordem; só as que têm conteúdo saem.
			var saem []paginaOrdenada
			for _, p := range d.paginas {
				// cleanPage roda também nas duplicadas, e o resultado é descartado,
				// de propósito: o estado do documento (timbre em jaVistas, marcadores
				// em assinados) tem de avançar como se a página tivesse saído, porque
				// o texto já apareceu na origem.
				cleanedText := cleanPage(p.content, cleaningLevel, ctx)
				if o, ok := h7.paginas[chavePagina{num, d.index, p.index}]; ok {
					cleanedText = o.refPagina()
				}
				if strings.TrimSpace(cleanedText) == "" {
					continue
				}
				saem = append(saem, paginaOrdenada{p.index, cleanedText})
			}
			switch {
			case len(saem) == 0:
			case len(d.paginas) == 1:
				// Documento de uma página na origem: o conteúdo vai direto, sem
				// <pag>. Com 2+ páginas na origem o n original é mantido, porque
				// a peça é citada por página.
				sb.WriteString(saem[0].content)
				sb.WriteString("\n")
			default:
				for _, p := range saem {
					sb.WriteString(fmt.Sprintf("<pag n=\"%d\">\n", p.index))
					sb.WriteString(p.content)
					sb.WriteString("\n</pag>\n")
				}
			}
			sb.WriteString("</documento>\n")
		}
		sb.WriteString("</evento>\n")
	}
	body := sb.String()

	// The estimate covers the whole output, including its own line, whose
	// length depends on the number. The map from estimate to estimate is
	// monotonic and starts below its fixed point, so it climbs to it in a few
	// rounds (the line adds under 10 tokens); the bound is only a safety net.
	base := utf8.RuneCountInString(header) + utf8.RuneCountInString(body)
	tokens := base / 4
	for range 16 {
		next := (base + utf8.RuneCountInString(tokensLine(tokens))) / 4
		if next == tokens {
			break
		}
		tokens = next
	}

	line := tokensLine(tokens)
	var out strings.Builder
	out.Grow(len(header) + len(line) + len(body))
	out.WriteString(header[:tokensAt])
	out.WriteString(line)
	out.WriteString(header[tokensAt:])
	out.WriteString(body)
	return out.String(), tokens
}
