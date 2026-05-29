package etl

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dslipak/pdf"
)

// PageData represents the extracted data of a single page.
type PageData struct {
	PageNum int    `json:"page_num"`
	Text    string `json:"text"`
	IsOcr   bool   `json:"is_ocr"`
}

// PDFParser wraps the PDF file processing.
type PDFParser struct {
	filePath string
}

// NewPDFParser creates a new PDFParser instance.
func NewPDFParser(filePath string) *PDFParser {
	return &PDFParser{filePath: filePath}
}

// ParsePages parses the PDF page-by-page.
// If native text is insufficient and OCR is enabled, it uses the OCR runner callback.
func (p *PDFParser) ParsePages(ctx context.Context, ocrEnabled bool, ocrRunner func(context.Context, string, int) (string, error)) ([]PageData, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pdf: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat pdf: %w", err)
	}

	reader, err := pdf.NewReader(file, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to create pdf reader: %w", err)
	}

	numPages := reader.NumPage()
	var pagesData []PageData

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// Extract native text
		text, err := ExtractNativeText(page)
		if err != nil {
			text = ""
		}

		cleanText := strings.TrimSpace(text)
		needsOcr := false

		if ocrEnabled {
			// Heuristic: If native text is empty or very short, check if it contains images
			if len(cleanText) < 100 {
				needsOcr = HasImages(page)
			}
		}

		var finalPageData PageData
		finalPageData.PageNum = pageNum

		if needsOcr && ocrRunner != nil {
			ocrText, err := ocrRunner(ctx, p.filePath, pageNum)
			if err == nil {
				finalPageData.Text = ocrText
				finalPageData.IsOcr = true
			} else {
				// Fallback to native text if OCR fails
				finalPageData.Text = cleanText
				finalPageData.IsOcr = false
			}
		} else {
			finalPageData.Text = cleanText
			finalPageData.IsOcr = false
		}

		pagesData = append(pagesData, finalPageData)
	}

	return pagesData, nil
}

// ExtractNativeText extracts plain text from a pdf.Page using GetTextByRow as preferred layout option.
func ExtractNativeText(page pdf.Page) (string, error) {
	rows, err := page.GetTextByRow()
	if err != nil {
		// Fallback to plain text if row parsing fails
		b, err := page.GetPlainText(nil)
		if err != nil {
			return "", err
		}
		return b, nil
	}

	var sb strings.Builder
	for _, row := range rows {
		var words []string
		for _, word := range row.Content {
			words = append(words, word.S)
		}
		if len(words) > 0 {
			sb.WriteString(strings.Join(words, " "))
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// HasImages checks if a page contains any image XObject of significant resolution.
func HasImages(page pdf.Page) bool {
	resources := page.Resources()
	if resources.IsNull() {
		return false
	}
	xobject := resources.Key("XObject")
	if xobject.IsNull() {
		return false
	}
	for _, k := range xobject.Keys() {
		obj := xobject.Key(k)
		subtype := obj.Key("Subtype")
		if !subtype.IsNull() && subtype.Name() == "Image" {
			width := obj.Key("Width").Int64()
			height := obj.Key("Height").Int64()
			// Filter out small decoration images (e.g. logos, icons)
			if width > 100 && height > 100 {
				return true
			}
		}
	}
	return false
}
