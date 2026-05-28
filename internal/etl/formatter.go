package etl

import (
	"fmt"
	"strings"
	"time"

	"eprocia/internal/models"
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
func GenerateConsolidatedOutput(files []models.ProcessFile, fileContents map[string][]PageData, cleaningLevel string) string {
	var sb strings.Builder

	// Header/Context info block
	sb.WriteString("<contexto>\n")
	sb.WriteString("    PROCESSO JUDICIAL - ORDEM CRONOLÓGICA\n")
	sb.WriteString(fmt.Sprintf("    Total de Documentos: %d\n", len(files)))
	sb.WriteString(fmt.Sprintf("    Data de Consolidação: %s\n\n", time.Now().Format("02/01/2006 15:04:05")))
	sb.WriteString("    <summary>\n")
	for _, f := range files {
		pages := fileContents[f.Name]
		sb.WriteString(fmt.Sprintf("    - Evento %d - %s: %d páginas\n", f.EventNum, f.DocName, len(pages)))
	}
	sb.WriteString("    </summary>\n")
	sb.WriteString("</contexto>\n\n")

	// Group files by EventNum
	eventsMap := make(map[int][]models.ProcessFile)
	var eventNums []int
	for _, f := range files {
		if _, exists := eventsMap[f.EventNum]; !exists {
			eventNums = append(eventNums, f.EventNum)
		}
		eventsMap[f.EventNum] = append(eventsMap[f.EventNum], f)
	}

	// Loop over each Event chronologically
	for _, num := range eventNums {
		sb.WriteString(fmt.Sprintf("<evento num=\"%d\">\n", num))
		eventFiles := eventsMap[num]
		for index, f := range eventFiles {
			sb.WriteString(fmt.Sprintf("    <documento index=\"%d\" nome=\"%s\">\n", index+1, f.DocName))
			pages := fileContents[f.Name]
			for _, page := range pages {
				cleanedText := CleanText(page.Text, cleaningLevel)
				sb.WriteString(fmt.Sprintf("        <pag num=\"%d\">\n", page.PageNum))
				sb.WriteString(cleanedText)
				sb.WriteString("\n        </pag>\n")
			}
			sb.WriteString("    </documento>\n")
		}
		sb.WriteString("</evento>\n")
	}

	return sb.String()
}
