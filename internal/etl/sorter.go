package etl

import (
	"archive/zip"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"eprocia/internal/models"
)

var eventRegex = regexp.MustCompile(`(?i)evento\s+(\d+)`)

// docNumRegex captures the document number that closes an Eproc file name
// (e.g. the "3" in "Evento 1 - COMP3.pdf" or "Evento 12 - PROC_VIRTUAL_3.pdf").
var docNumRegex = regexp.MustCompile(`(\d+)\.[A-Za-z0-9]+$`)

// coverRegex matches the process cover page, which must always come first.
var coverRegex = regexp.MustCompile(`(?i)^capa do processo\.pdf$`)

const (
	// noEventNum is the sort key of files without an "Evento N" prefix; they go last.
	noEventNum = 999999
	// noDocNum is the sort key of files without a trailing document number.
	noDocNum = math.MaxInt32
)

// ParseFileInfo extracts metadata from a zip.File to build a ProcessFile.
//
// Ordering keys, per the spec (§3.2): the cover ("Capa do processo.pdf") is
// event 0; "Evento N" files use N as event and the trailing number as the
// document ID; anything else is sent to the end.
func ParseFileInfo(f *zip.File) models.ProcessFile {
	name := filepath.Base(f.Name)
	ext := strings.ToLower(filepath.Ext(name))

	eventNum := noEventNum
	docNum := noDocNum

	if coverRegex.MatchString(name) {
		eventNum, docNum = 0, 0
	} else if loc := eventRegex.FindStringSubmatchIndex(name); loc != nil {
		if val, err := strconv.Atoi(name[loc[2]:loc[3]]); err == nil {
			eventNum = val
		}
		// Search only after the "Evento N" prefix so the event number is never
		// mistaken for the document number (e.g. "Evento 5.pdf").
		if m := docNumRegex.FindStringSubmatch(name[loc[1]:]); m != nil {
			if val, err := strconv.Atoi(m[1]); err == nil {
				docNum = val
			}
		}
	}

	return models.ProcessFile{
		Name:       f.Name,
		EventNum:   eventNum,
		DocNum:     docNum,
		DocName:    name,
		Extension:  ext,
		ZipFileRef: f,
	}
}

// SortProcessFiles sorts process files chronologically: by event number, then
// by document number inside the event, falling back to a natural name compare.
func SortProcessFiles(files []models.ProcessFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].EventNum != files[j].EventNum {
			return files[i].EventNum < files[j].EventNum
		}
		if files[i].DocNum != files[j].DocNum {
			return files[i].DocNum < files[j].DocNum
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
