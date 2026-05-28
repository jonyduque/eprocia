package etl

import (
	"archive/zip"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"eprocia/internal/models"
)

var eventRegex = regexp.MustCompile(`(?i)evento\s+(\d+)`)

// ParseFileInfo extracts metadata from a zip.File to build a ProcessFile.
func ParseFileInfo(f *zip.File) models.ProcessFile {
	name := filepath.Base(f.Name)
	ext := strings.ToLower(filepath.Ext(name))

	// Try to match "Evento X"
	matches := eventRegex.FindStringSubmatch(name)
	eventNum := 999999 // Fallback to a high value so unmatched files go to the end
	if len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			eventNum = val
		}
	}

	return models.ProcessFile{
		Name:       f.Name,
		EventNum:   eventNum,
		DocName:    name,
		Extension:  ext,
		ZipFileRef: f,
	}
}

// SortProcessFiles sorts process files chronologically (by event number) and naturally.
func SortProcessFiles(files []models.ProcessFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].EventNum != files[j].EventNum {
			return files[i].EventNum < files[j].EventNum
		}
		return CompareNatural(files[i].DocName, files[j].DocName)
	})
}

// CompareNatural compares two strings naturally (runs of digits compared as numbers).
func CompareNatural(a, b string) bool {
	chunksA := splitIntoChunks(a)
	chunksB := splitIntoChunks(b)

	for i := 0; i < len(chunksA) && i < len(chunksB); i++ {
		ca := chunksA[i]
		cb := chunksB[i]

		if ca == cb {
			continue
		}

		isNumA := isDigit(ca[0])
		isNumB := isDigit(cb[0])

		if isNumA && isNumB {
			valA, _ := strconv.Atoi(ca)
			valB, _ := strconv.Atoi(cb)
			if valA != valB {
				return valA < valB
			}
		} else {
			// Case insensitive comparison for text chunks
			caLower := strings.ToLower(ca)
			cbLower := strings.ToLower(cb)
			if caLower != cbLower {
				return caLower < cbLower
			}
		}
	}

	if len(chunksA) != len(chunksB) {
		return len(chunksA) < len(chunksB)
	}

	// Tie-break with case-sensitive comparison of the original strings
	return a < b
}

func splitIntoChunks(s string) []string {
	var chunks []string
	if len(s) == 0 {
		return chunks
	}

	var current strings.Builder
	inDigit := isDigit(s[0])
	current.WriteByte(s[0])

	for i := 1; i < len(s); i++ {
		char := s[i]
		charIsDigit := isDigit(char)
		if charIsDigit != inDigit {
			chunks = append(chunks, current.String())
			current.Reset()
			inDigit = charIsDigit
		}
		current.WriteByte(char)
	}
	chunks = append(chunks, current.String())
	return chunks
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
