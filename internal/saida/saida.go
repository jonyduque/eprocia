// Package saida guarda a regra única do nome do TXT de saída: onde ele fica,
// como se chama e quando uma saída pertence a um item da fila. É um pacote
// folha (só a biblioteca padrão), importado por etl, watchdog, main e pelo
// histórico.
package saida

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ErrSemNomeLivre é devolvido quando a base e os nomes _2 a _999 já existem.
var ErrSemNomeLivre = errors.New("não há nome livre para o TXT")

// sufixoMax é o maior N de <nome>_N.txt.
const sufixoMax = 999

// escrever grava o conteúdo no arquivo criado; variável para o teste simular
// falha de escrita.
var escrever = (*os.File).WriteString

// reCNJ é o número de processo no formato NNNNNNN-DD.AAAA.J.TR.OOOO.
var reCNJ = regexp.MustCompile(`\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}`)

// semExtensao tira a última extensão do caminho.
func semExtensao(caminho string) string {
	return strings.TrimSuffix(caminho, filepath.Ext(caminho))
}

// Base devolve <pasta>\<nome sem a última extensão>.txt.
func Base(entrada string) string {
	return semExtensao(entrada) + ".txt"
}

// NomeLote devolve a base do TXT de um item de PDFs, com pdfs já na ordem por
// evento: <pasta do primeiro>\<CNJ>.txt quando todos os nomes têm o mesmo
// número de processo, senão Base do primeiro. Com um PDF só, Base dele; ""
// para lista vazia.
func NomeLote(pdfs []string) string {
	if len(pdfs) == 0 {
		return ""
	}
	if len(pdfs) == 1 {
		return Base(pdfs[0])
	}
	comum := ""
	for i, p := range pdfs {
		cnj := reCNJ.FindString(filepath.Base(p))
		if cnj == "" || (i > 0 && cnj != comum) {
			return Base(pdfs[0])
		}
		comum = cnj
	}
	return filepath.Join(filepath.Dir(pdfs[0]), comum+".txt")
}

// GravarSemSobrescrever cria a base ou, se ela estiver ocupada, o primeiro
// <nome>_N.txt livre (N de 2 a 999) e grava o conteúdo nele. A base conta
// como ocupada também quando existe <nome>.zip na mesma pasta. A escolha e a
// criação são uma operação só (O_EXCL); uma falha de escrita remove o arquivo
// criado. Devolve o caminho gravado.
func GravarSemSobrescrever(base, conteudo string) (string, error) {
	nome := semExtensao(base)
	for n := 1; n <= sufixoMax; n++ {
		candidato := base
		if n > 1 {
			candidato = nome + "_" + strconv.Itoa(n) + ".txt"
		} else if _, err := os.Stat(nome + ".zip"); err == nil {
			continue
		}
		f, err := os.OpenFile(candidato, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := escrever(f, conteudo); err != nil {
			_ = f.Close()
			_ = os.Remove(candidato)
			return "", err
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(candidato)
			return "", err
		}
		return candidato, nil
	}
	return "", ErrSemNomeLivre
}

// Gravar grava direto no caminho, truncando o que houver (a gravação do ZIP).
func Gravar(caminho, conteudo string) error {
	f, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := escrever(f, conteudo); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// DaEntrada diz se saida é <radical>.txt ou <radical>_N.txt (N de 2 a 999,
// sem zero à esquerda) na pasta da entrada, sem diferença de caixa.
func DaEntrada(entrada, saida string) bool {
	if entrada == "" {
		return false
	}
	return daBase(Base(entrada), saida)
}

// daBase confere saida contra a base <pasta>\<radical>.txt.
func daBase(base, s string) bool {
	if base == "" || s == "" {
		return false
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(base)), filepath.Clean(filepath.Dir(s))) {
		return false
	}
	nome := filepath.Base(s)
	ext := filepath.Ext(nome)
	if !strings.EqualFold(ext, ".txt") {
		return false
	}
	radicalSaida := strings.TrimSuffix(nome, ext)
	radical := semExtensao(filepath.Base(base))
	if strings.EqualFold(radicalSaida, radical) {
		return true
	}
	i := strings.LastIndexByte(radicalSaida, '_')
	if i < 0 || !sufixoValido(radicalSaida[i+1:]) {
		return false
	}
	return strings.EqualFold(radicalSaida[:i], radical)
}

// sufixoValido aceita um inteiro decimal de 2 a 999 sem zero à esquerda.
func sufixoValido(s string) bool {
	if s == "" || len(s) > 3 || s[0] == '0' {
		return false
	}
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, _ := strconv.Atoi(s)
	return n >= 2 && n <= sufixoMax
}

// SaidaDoItem diz se a saída pertence ao item: ZIP só em Base(path); PDF em
// <radical>.txt ou <radical>_N.txt, contra NomeLote(paths) quando o item tem
// vários PDFs e contra path quando tem um só (ou paths vazio).
func SaidaDoItem(kind, path string, paths []string, saida string) bool {
	if saida == "" {
		return false
	}
	switch kind {
	case "zip":
		return path != "" && strings.EqualFold(filepath.Clean(saida), filepath.Clean(Base(path)))
	case "pdf":
		if len(paths) > 1 {
			return daBase(NomeLote(paths), saida)
		}
		return DaEntrada(path, saida)
	}
	return false
}
