package watchdog

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"eprocia/internal/saida"
)

// janelaPendentes limita a varredura inicial a downloads recentes, para não
// reprocessar o histórico inteiro de uma pasta de Downloads.
const janelaPendentes = 24 * time.Hour

// prefixoTemporario é o dos PDFs temporários do pipeline (eprocia_pdf_*.pdf):
// nunca são entrada, mesmo que alguém monitore %TEMP%.
const prefixoTemporario = "eprocia_pdf_"

// Extensoes devolve as extensões monitoradas: só ".zip" ou, com a opção de
// PDF ligada, ".zip" e ".pdf".
func Extensoes(monitorarPDF bool) []string {
	if monitorarPDF {
		return []string{".zip", ".pdf"}
	}
	return []string{".zip"}
}

// normalizarExtensoes passa para minúsculas, tira repetições e ordena; nil ou
// vazio vale [".zip"].
func normalizarExtensoes(extensoes []string) []string {
	var out []string
	for _, e := range extensoes {
		e = strings.ToLower(e)
		if e != "" && !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return []string{".zip"}
	}
	slices.Sort(out)
	return out
}

// aceito diz se o arquivo tem uma das extensões (já normalizadas) e não é
// um temporário do pipeline.
func aceito(nome string, extensoes []string) bool {
	ext := strings.ToLower(filepath.Ext(nome))
	if !slices.Contains(extensoes, ext) {
		return false
	}
	return !(ext == ".pdf" && temporario(nome))
}

// temporario diz se o nome é de um PDF temporário do pipeline.
func temporario(nome string) bool {
	base := filepath.Base(nome)
	return len(base) >= len(prefixoTemporario) && strings.EqualFold(base[:len(prefixoTemporario)], prefixoTemporario)
}

// jaConvertido diz se a entrada (com mtime mod) já tem um TXT igual ou mais
// novo ao lado. ZIP: só saida.Base. PDF: <nome>.txt ou <nome>_N.txt
// (saida.DaEntrada), salvo <nome>.txt quando existe <nome>.zip (o TXT é do
// ZIP). Um TXT cujo radical é o nome de outra entrada da pasta (processo_2.txt
// com processo_2.pdf ou processo_2.zip ao lado) é a base dela e não conta para
// processo.pdf.
func jaConvertido(caminho string, mod time.Time) bool {
	if !strings.EqualFold(filepath.Ext(caminho), ".pdf") {
		txt, err := os.Stat(saida.Base(caminho))
		return err == nil && !txt.ModTime().Before(mod)
	}
	dir := filepath.Dir(caminho)
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	nomes := make(map[string]bool, len(entradas))
	for _, e := range entradas {
		nomes[strings.ToLower(e.Name())] = true
	}
	proprio := strings.ToLower(filepath.Base(caminho))
	for _, e := range entradas {
		if e.IsDir() {
			continue
		}
		candidato := filepath.Join(dir, e.Name())
		if !saida.DaEntrada(caminho, candidato) {
			continue
		}
		radical := strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if nomes[radical+".zip"] {
			continue
		}
		if pdf := radical + ".pdf"; pdf != proprio && nomes[pdf] {
			continue
		}
		if info, err := e.Info(); err == nil && !info.ModTime().Before(mod) {
			return true
		}
	}
	return false
}

// pendingZips lista as entradas de dir (ZIP e, se estiver em extensoes, PDF)
// modificadas depois de since que ainda não foram convertidas (jaConvertido).
// Devolve do mais antigo ao mais novo. ignorar (opcional) descarta entradas
// que já falharam com o mesmo mtime.
func pendingZips(dir string, since time.Time, extensoes []string, ignorar func(zip string, mod time.Time) bool) ([]string, error) {
	extensoes = normalizarExtensoes(extensoes)
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type pendente struct {
		caminho string
		mod     time.Time
	}
	var achados []pendente
	for _, e := range entradas {
		if e.IsDir() || !aceito(e.Name(), extensoes) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().After(since) {
			continue
		}
		caminho := filepath.Join(dir, e.Name())
		if jaConvertido(caminho, info.ModTime()) {
			continue
		}
		if ignorar != nil && ignorar(caminho, info.ModTime()) {
			continue
		}
		achados = append(achados, pendente{caminho, info.ModTime()})
	}

	sort.SliceStable(achados, func(i, j int) bool { return achados[i].mod.Before(achados[j].mod) })
	caminhos := make([]string, len(achados))
	for i, a := range achados {
		caminhos[i] = a.caminho
	}
	return caminhos, nil
}
