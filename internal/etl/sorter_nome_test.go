package etl

import (
	"slices"
	"testing"

	"eprocia/internal/models"
)

func TestParseNome(t *testing.T) {
	casos := []struct {
		nome    string
		evento  int
		doc     int
		docName string
		ext     string
	}{
		{"5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf", 12, 1, "5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf", ".pdf"},
		{"anexo.pdf", noEventNum, noDocNum, "anexo.pdf", ".pdf"},
		{"Capa do processo.pdf", 0, 0, "Capa do processo.pdf", ".pdf"},
		{`C:\Users\fulano\x\Evento 3 - INIC2.PDF`, 3, 2, "Evento 3 - INIC2.PDF", ".pdf"},
	}
	for _, c := range casos {
		f := ParseNome(c.nome)
		if f.Name != c.nome {
			t.Errorf("%q: Name = %q", c.nome, f.Name)
		}
		if f.EventNum != c.evento || f.DocNum != c.doc {
			t.Errorf("%q: evento/doc = %d/%d, quer %d/%d", c.nome, f.EventNum, f.DocNum, c.evento, c.doc)
		}
		if f.DocName != c.docName {
			t.Errorf("%q: DocName = %q, quer %q", c.nome, f.DocName, c.docName)
		}
		if f.Extension != c.ext {
			t.Errorf("%q: Extension = %q, quer %q", c.nome, f.Extension, c.ext)
		}
	}
}

func TestSortProcessFiles_EstavelComNomesIguais(t *testing.T) {
	a := ParseNome("Evento 3 - INIC1.pdf")
	a.Name = "a"
	b := ParseNome("Evento 3 - INIC1.pdf")
	b.Name = "b"
	for _, entrada := range [][]models.ProcessFile{{a, b}, {b, a}} {
		quer := []string{entrada[0].Name, entrada[1].Name}
		lista := slices.Clone(entrada)
		SortProcessFiles(lista)
		got := []string{lista[0].Name, lista[1].Name}
		if !slices.Equal(got, quer) {
			t.Errorf("ordem = %v, quer %v", got, quer)
		}
	}

	// Com dois elementos qualquer ordenação mantém a ordem; com muitos, uma
	// ordenação instável troca empates de lugar.
	var lista []models.ProcessFile
	for i := range 200 {
		f := ParseNome([]string{"Evento 5 - DOC1.pdf", "Evento 2 - DOC1.pdf", "Evento 9 - DOC1.pdf"}[(i*7)%3])
		f.Name = string(rune('A'+i%26)) + string(rune('a'+i/26))
		f.Tamanho = int64(i)
		lista = append(lista, f)
	}
	SortProcessFiles(lista)
	for i := 1; i < len(lista); i++ {
		if lista[i].DocName == lista[i-1].DocName && lista[i].Tamanho < lista[i-1].Tamanho {
			t.Fatalf("empate fora da ordem de entrada na posição %d", i)
		}
	}
}

func TestOrdenarPorEvento(t *testing.T) {
	e12 := `C:\Users\fulano\Downloads\5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf`
	anexo := `C:\Users\fulano\Downloads\anexo.pdf`
	e3 := `C:\Users\fulano\Desktop\5000000-00.2026.8.26.0001 - Evento 3 - INIC1.pdf`
	entrada := []string{e12, anexo, e3}
	original := slices.Clone(entrada)

	got := ordenarPorEvento(entrada)
	quer := []string{e3, e12, anexo}
	if !slices.Equal(got, quer) {
		t.Errorf("ordenarPorEvento = %v, quer %v", got, quer)
	}
	if !slices.Equal(entrada, original) {
		t.Errorf("a lista de entrada mudou: %v", entrada)
	}
}
