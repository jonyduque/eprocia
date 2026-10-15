package etl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)


// pageMarkerRegex matches a line that is *only* a page marker, such as
// "Página 12 de 35", "Pág. 3/10" or "página 4". Anchoring the whole line keeps
// legitimate sentences that merely mention the word "página" intact.
var pageMarkerRegex = regexp.MustCompile(`(?i)^p[áa]g(ina|\.)?\s*\d+(\s*(de|/)\s*\d+)?$`)

// signaturePrefixes are boilerplate openers of digital-signature footers.
// Only lines that *start* with them are dropped, never prose that mentions them.
var signaturePrefixes = []string{
	"documento assinado",
	"assinatura eletrônica",
	"assinatura eletronica",
}

// isBoilerplateLine reports whether a trimmed line is a page marker or a
// digital-signature footer (aggressive cleaning only).
func isBoilerplateLine(trimmed string) bool {
	if pageMarkerRegex.MatchString(trimmed) {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range signaturePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

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

// pageCountLabel renders "1 página" / "N páginas".
func pageCountLabel(n int) string {
	if n == 1 {
		return "1 página"
	}
	return fmt.Sprintf("%d páginas", n)
}

// CleanText cleans text based on the configured cleaning level (none, basic, aggressive).
func CleanText(text string, level string) string {
	if level == "none" {
		return text
	}

	lines := strings.Split(text, "\n")
	var cleanedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Aggressive cleaning: drop whole-line headers/footers/signatures only.
		if level == "aggressive" && isBoilerplateLine(trimmed) {
			continue
		}

		cleanedLines = append(cleanedLines, trimmed)
	}

	if level == "aggressive" {
		// Compress to a single line or space-separated to maximize token savings
		return strings.Join(cleanedLines, " ")
	}
	return strings.Join(cleanedLines, "\n")
}


// GenerateConsolidatedOutput formats all extracted process data into structured XML tags.
func GenerateConsolidatedOutput(eventos map[string]any, cleaningLevel string) string {
	var sb strings.Builder

	// 1. Sort event numbers chronologically
	var eventNums []int
	for _, val := range eventos {
		evData := val.(map[string]any)
		eventNums = append(eventNums, evData["index"].(int))
	}
	sort.Ints(eventNums)

	// Count total documents
	totalDocs := 0
	for _, val := range eventos {
		evData := val.(map[string]any)
		documentosMap := evData["documentos"].(map[string]any)
		totalDocs += len(documentosMap)
	}

	// Header/Context info block
	sb.WriteString("<contexto>\n")
	sb.WriteString("    PROCESSO JUDICIAL - ORDEM CRONOLÓGICA\n")
	sb.WriteString(fmt.Sprintf("    Total de Documentos: %d\n", totalDocs))
	sb.WriteString(fmt.Sprintf("    Data de Consolidação: %s\n\n", time.Now().Format("02/01/2006 15:04:05")))
	sb.WriteString("    <summary>\n")

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
			docs = append(docs, docSummaryEntry{
				nome:  dData["nome"].(string),
				index: dData["index"].(int),
				pages: len(paginasMap),
			})
		}
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].index < docs[j].index
		})

		for _, d := range docs {
			sb.WriteString(fmt.Sprintf("    - Evento %d - %s: %s\n", num, summaryDocName(d.nome), pageCountLabel(d.pages)))
		}
	}
	sb.WriteString("    </summary>\n")
	sb.WriteString("</contexto>\n\n")

	// Loop over each Event chronologically
	for _, num := range eventNums {
		eventKey := fmt.Sprintf("%d", num)
		evData := eventos[eventKey].(map[string]any)
		sb.WriteString(fmt.Sprintf("<evento num=\"%d\">\n", num))

		documentosMap := evData["documentos"].(map[string]any)
		type docEntry struct {
			nome    string
			index   int
			paginas map[string]any
		}
		var docs []docEntry
		for _, val := range documentosMap {
			dData := val.(map[string]any)
			docs = append(docs, docEntry{
				nome:    dData["nome"].(string),
				index:   dData["index"].(int),
				paginas: dData["paginas"].(map[string]any),
			})
		}
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].index < docs[j].index
		})

		for _, d := range docs {
			sb.WriteString(fmt.Sprintf("    <documento index=\"%d\" nome=\"%s\">\n", d.index, escapeAttr(d.nome)))

			// Sort pages of this document numerically
			type pageEntry struct {
				index   int
				ocr     bool
				image   string
				content string
			}
			var pages []pageEntry
			for _, val := range d.paginas {
				pData := val.(map[string]any)
				
				// Handle potential missing fields gracefully
				ocrVal, _ := pData["ocr"].(bool)
				imgVal, _ := pData["image"].(string)
				contentVal, _ := pData["content"].(string)

				pages = append(pages, pageEntry{
					index:   pData["index"].(int),
					ocr:     ocrVal,
					image:   imgVal,
					content: contentVal,
				})
			}
			sort.Slice(pages, func(i, j int) bool {
				return pages[i].index < pages[j].index
			})

			for _, p := range pages {
				cleanedText := CleanText(p.content, cleaningLevel)
				sb.WriteString(fmt.Sprintf("        <pag num=\"%d\">\n", p.index))
				sb.WriteString(cleanedText)
				sb.WriteString("\n        </pag>\n")
			}
			sb.WriteString("    </documento>\n")
		}
		sb.WriteString("</evento>\n")
	}

	return sb.String()
}
