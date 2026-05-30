package etl

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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

		if level == "aggressive" {
			// Aggressive cleaning: remove headers, footers, repetitive metadata
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "página") || strings.Contains(lower, "pagina") {
				continue
			}
			if strings.Contains(lower, "documento assinado") || strings.Contains(lower, "assinatura eletrônica") {
				continue
			}
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
			sb.WriteString(fmt.Sprintf("    - Evento %d - %s: %d páginas\n", num, d.nome, d.pages))
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
			sb.WriteString(fmt.Sprintf("    <documento index=\"%d\" nome=\"%s\">\n", d.index, d.nome))

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
