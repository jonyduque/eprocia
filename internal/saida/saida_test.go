package saida

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const dl = `C:\Users\fulano\Downloads`

func TestBase(t *testing.T) {
	casos := map[string]string{
		dl + `\processo.zip`: dl + `\processo.txt`,
		dl + `\processo.ZIP`: dl + `\processo.txt`,
		dl + `\processo.pdf`: dl + `\processo.txt`,
		dl + `\processo.PDF`: dl + `\processo.txt`,
		dl + `\a.b.pdf`:      dl + `\a.b.txt`,
	}
	for entrada, quer := range casos {
		if got := Base(entrada); got != quer {
			t.Errorf("Base(%q) = %q, quer %q", entrada, got, quer)
		}
	}
}

func TestNomeLote(t *testing.T) {
	dir1 := dl + `\dir1`
	dir2 := dl + `\dir2`
	cnj := "5000000-00.2026.8.26.0001"
	pdfs := []string{
		dir1 + `\` + cnj + ` - Evento 3 - INIC1.pdf`,
		dir1 + `\` + cnj + ` - Evento 12 - SENT1.pdf`,
		dir1 + `\` + cnj + ` - Evento 7 - LAUDO1.pdf`,
	}
	if got, quer := NomeLote(pdfs), dir1+`\`+cnj+`.txt`; got != quer {
		t.Errorf("CNJ comum: %q, quer %q", got, quer)
	}

	outro := []string{pdfs[0], dir1 + `\5000000-00.2026.8.26.0002 - Evento 12 - SENT1.pdf`, pdfs[2]}
	if got, quer := NomeLote(outro), Base(outro[0]); got != quer {
		t.Errorf("CNJ diferente: %q, quer %q", got, quer)
	}

	sem := []string{dir1 + `\peticao.pdf`, dir1 + `\laudo.pdf`}
	if got, quer := NomeLote(sem), Base(sem[0]); got != quer {
		t.Errorf("sem CNJ: %q, quer %q", got, quer)
	}

	um := []string{pdfs[1]}
	if got, quer := NomeLote(um), Base(um[0]); got != quer {
		t.Errorf("um PDF: %q, quer %q", got, quer)
	}

	pastas := []string{
		dir2 + `\` + cnj + ` - Evento 3 - INIC1.pdf`,
		dir1 + `\` + cnj + ` - Evento 7 - LAUDO1.pdf`,
	}
	if got, quer := NomeLote(pastas), dir2+`\`+cnj+`.txt`; got != quer {
		t.Errorf("duas pastas: %q, quer %q", got, quer)
	}

	if got := NomeLote(nil); got != "" {
		t.Errorf("lista vazia: %q, quer vazio", got)
	}
}

func criar(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func conferir(t *testing.T, caminho, quer string) {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != quer {
		t.Errorf("%s: conteúdo %q, quer %q", caminho, b, quer)
	}
}

func TestGravarSemSobrescrever_NomeLivre(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "processo.txt")
	got, err := GravarSemSobrescrever(base, "FULANO DE TAL")
	if err != nil {
		t.Fatal(err)
	}
	if got != base {
		t.Errorf("caminho %q, quer %q", got, base)
	}
	conferir(t, base, "FULANO DE TAL")
}

func TestGravarSemSobrescrever_BaseOcupadaGrava2(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "processo.txt")
	criar(t, base, "antigo")
	got, err := GravarSemSobrescrever(base, "novo")
	if err != nil {
		t.Fatal(err)
	}
	if quer := filepath.Join(dir, "processo_2.txt"); got != quer {
		t.Errorf("caminho %q, quer %q", got, quer)
	}
	conferir(t, got, "novo")
	conferir(t, base, "antigo")
}

func TestGravarSemSobrescrever_2e3OcupadosGrava4(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "processo.txt")
	criar(t, base, "a")
	criar(t, filepath.Join(dir, "processo_2.txt"), "b")
	criar(t, filepath.Join(dir, "processo_3.txt"), "c")
	got, err := GravarSemSobrescrever(base, "novo")
	if err != nil {
		t.Fatal(err)
	}
	if quer := filepath.Join(dir, "processo_4.txt"); got != quer {
		t.Errorf("caminho %q, quer %q", got, quer)
	}
	conferir(t, got, "novo")
}

func TestGravarSemSobrescrever_ZipAoLadoGrava2(t *testing.T) {
	dir := t.TempDir()
	criar(t, filepath.Join(dir, "processo.zip"), "zip")
	base := filepath.Join(dir, "processo.txt")
	got, err := GravarSemSobrescrever(base, "novo")
	if err != nil {
		t.Fatal(err)
	}
	if quer := filepath.Join(dir, "processo_2.txt"); got != quer {
		t.Errorf("caminho %q, quer %q", got, quer)
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a base não devia existir: %v", err)
	}
}

func TestGravarSemSobrescrever_TudoOcupadoDaErro(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "processo.txt")
	criar(t, base, "x")
	for n := 2; n <= 999; n++ {
		criar(t, filepath.Join(dir, fmt.Sprintf("processo_%d.txt", n)), "x")
	}
	got, err := GravarSemSobrescrever(base, "novo")
	if !errors.Is(err, ErrSemNomeLivre) {
		t.Fatalf("erro %v, quer ErrSemNomeLivre", err)
	}
	if got != "" {
		t.Errorf("caminho %q, quer vazio", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "processo_1000.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("processo_1000.txt não devia existir: %v", err)
	}
}

func TestGravarSemSobrescrever_NaoTocaNoExistente(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "processo.txt")
	criar(t, base, "conteúdo do ZIP de FULANO DE TAL")
	antes := hash(t, base)
	if _, err := GravarSemSobrescrever(base, "outro"); err != nil {
		t.Fatal(err)
	}
	if depois := hash(t, base); depois != antes {
		t.Error("o TXT existente mudou")
	}
}

func hash(t *testing.T, caminho string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func TestGravarSemSobrescrever_FalhaNaoDeixaArquivo(t *testing.T) {
	t.Run("pasta inexistente", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "nao-existe", "processo.txt")
		got, err := GravarSemSobrescrever(base, "x")
		if err == nil {
			t.Fatal("quer erro")
		}
		if errors.Is(err, ErrSemNomeLivre) {
			t.Errorf("erro %v não devia ser ErrSemNomeLivre", err)
		}
		if got != "" {
			t.Errorf("caminho %q, quer vazio", got)
		}
		if _, err := os.Stat(filepath.Join(dir, "nao-existe")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a pasta não devia ser criada: %v", err)
		}
	})
	t.Run("escrita falha", func(t *testing.T) {
		falha := errors.New("disco cheio")
		antigo := escrever
		escrever = func(*os.File, string) (int, error) { return 0, falha }
		t.Cleanup(func() { escrever = antigo })

		dir := t.TempDir()
		base := filepath.Join(dir, "processo.txt")
		criar(t, base, "antigo")
		got, err := GravarSemSobrescrever(base, "novo")
		if !errors.Is(err, falha) {
			t.Fatalf("erro %v, quer %v", err, falha)
		}
		if got != "" {
			t.Errorf("caminho %q, quer vazio", got)
		}
		entradas, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entradas) != 1 || entradas[0].Name() != "processo.txt" {
			var nomes []string
			for _, e := range entradas {
				nomes = append(nomes, e.Name())
			}
			t.Errorf("arquivos na pasta: %v, quer só processo.txt", nomes)
		}
		conferir(t, base, "antigo")
	})
}

func TestGravar(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "processo.txt")
	criar(t, caminho, "conteúdo antigo e mais longo")
	if err := Gravar(caminho, "novo"); err != nil {
		t.Fatal(err)
	}
	conferir(t, caminho, "novo")
}

func TestDaEntrada(t *testing.T) {
	entrada := dl + `\processo.pdf`
	aceita := []string{
		dl + `\processo.txt`,
		dl + `\processo_7.txt`,
		dl + `\PROCESSO.TXT`,
		dl + `\processo_999.txt`,
	}
	for _, s := range aceita {
		if !DaEntrada(entrada, s) {
			t.Errorf("DaEntrada(%q) = false, quer true", s)
		}
	}
	recusa := []string{
		`D:\outra\processo.txt`,
		dl + `\processo_1.txt`,
		dl + `\processo_02.txt`,
		dl + `\processo_1000.txt`,
		dl + `\processo_abc.txt`,
		dl + `\outro.txt`,
		"",
	}
	for _, s := range recusa {
		if DaEntrada(entrada, s) {
			t.Errorf("DaEntrada(%q) = true, quer false", s)
		}
	}
}

func TestSaidaDoItem(t *testing.T) {
	zip := dl + `\processo.zip`
	if !SaidaDoItem("zip", zip, nil, dl+`\processo.txt`) {
		t.Error("zip com processo.txt devia valer")
	}
	if SaidaDoItem("zip", zip, nil, dl+`\processo_2.txt`) {
		t.Error("zip com processo_2.txt não devia valer")
	}

	pdf := dl + `\processo.pdf`
	if !SaidaDoItem("pdf", pdf, []string{pdf}, dl+`\processo.txt`) {
		t.Error("pdf com processo.txt devia valer")
	}
	if !SaidaDoItem("pdf", pdf, []string{pdf}, dl+`\processo_3.txt`) {
		t.Error("pdf com processo_3.txt devia valer")
	}
	if !SaidaDoItem("pdf", pdf, nil, dl+`\processo_2.txt`) {
		t.Error("pdf sem paths com processo_2.txt devia valer")
	}

	cnj := "5000000-00.2026.8.26.0001"
	lote := []string{
		dl + `\` + cnj + ` - Evento 3 - INIC1.pdf`,
		dl + `\` + cnj + ` - Evento 7 - LAUDO1.pdf`,
		dl + `\` + cnj + ` - Evento 12 - SENT1.pdf`,
	}
	if !SaidaDoItem("pdf", lote[0], lote, dl+`\`+cnj+`.txt`) {
		t.Error("lote com <CNJ>.txt devia valer")
	}
	if !SaidaDoItem("pdf", lote[0], lote, dl+`\`+cnj+`_2.txt`) {
		t.Error("lote com <CNJ>_2.txt devia valer")
	}
	if SaidaDoItem("pdf", lote[0], lote, Base(lote[0])) {
		t.Error("lote com o nome do primeiro PDF não devia valer")
	}

	if SaidaDoItem("zip", zip, nil, "") || SaidaDoItem("pdf", pdf, []string{pdf}, "") {
		t.Error("saída vazia não devia valer")
	}
	if SaidaDoItem("html", pdf, []string{pdf}, dl+`\processo.txt`) || SaidaDoItem("", zip, nil, dl+`\processo.txt`) {
		t.Error("kind desconhecido não devia valer")
	}
}
