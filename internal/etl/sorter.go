package etl

import (
	"archive/zip"
	"cmp"
	"math"
	"path/filepath"
	"regexp"
	"slices"
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

// ParseNome extrai as chaves de ordenação do nome de um documento (entrada de
// ZIP ou caminho de um PDF avulso). Name fica como recebido; DocName é o nome
// base.
//
// Ordering keys, per the spec (§3.2): the cover ("Capa do processo.pdf") is
// event 0; "Evento N" files use N as event and the trailing number as the
// document ID; anything else is sent to the end.
func ParseNome(nome string) models.ProcessFile {
	name := filepath.Base(nome)
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
		Name:      nome,
		EventNum:  eventNum,
		DocNum:    docNum,
		DocName:   name,
		Extension: ext,
	}
}

// ParseFileInfo extracts metadata from a zip.File to build a ProcessFile.
func ParseFileInfo(f *zip.File) models.ProcessFile {
	pf := ParseNome(f.Name)
	pf.ZipFileRef = f
	pf.Abrir = f.Open
	pf.Tamanho = int64(f.UncompressedSize64)
	return pf
}

// ordenarPorEvento devolve uma cópia dos caminhos na ordem de
// SortProcessFiles sobre o nome base de cada um; empates ficam na ordem
// recebida.
func ordenarPorEvento(caminhos []string) []string {
	arquivos := make([]models.ProcessFile, len(caminhos))
	for i, c := range caminhos {
		arquivos[i] = ParseNome(filepath.Base(c))
		arquivos[i].Name = c
	}
	SortProcessFiles(arquivos)
	ordem := make([]string, len(arquivos))
	for i, f := range arquivos {
		ordem[i] = f.Name
	}
	return ordem
}

// SortProcessFiles sorts process files chronologically: by event number, then
// by document number inside the event, falling back to a natural name compare.
// Estável: nomes iguais (PDFs avulsos de pastas diferentes) ficam na ordem
// recebida, a mesma na fila e no pipeline.
func SortProcessFiles(files []models.ProcessFile) {
	slices.SortStableFunc(files, func(a, b models.ProcessFile) int {
		return cmp.Or(
			cmp.Compare(a.EventNum, b.EventNum),
			cmp.Compare(a.DocNum, b.DocNum),
			compareNaturalInt(a.DocName, b.DocName),
		)
	})
}

// compareNaturalInt adapta CompareNatural (menor-que) ao contrato -1/0/+1 de slices.SortFunc.
func compareNaturalInt(a, b string) int {
	switch {
	case CompareNatural(a, b):
		return -1
	case CompareNatural(b, a):
		return 1
	}
	return 0
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
