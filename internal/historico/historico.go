// Package historico guarda a lista da fila da última sessão com processamento
// em historico.json: formato, validação, carga e o gravador com debounce e
// escrita atômica. É um pacote folha: não importa etl nem net.
package historico

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"eprocia/internal/models"
	"eprocia/internal/saida"
)

// VersaoFormato é a versão do formato de historico.json que este programa
// lê e grava.
const VersaoFormato = 1

// Teto é o número máximo de itens guardados; acima dele saem os mais antigos.
const Teto = 500

// ErrVersaoNova é devolvido por Carregar quando o arquivo foi gravado por uma
// versão do formato maior que VersaoFormato.
var ErrVersaoNova = errors.New("histórico gravado por uma versão mais nova do eprocIA")

// Item é uma linha da fila guardada no histórico.
type Item struct {
	Name            string   `json:"name"`
	Path            string   `json:"path"`
	Status          string   `json:"status"`
	Message         string   `json:"message"`
	OutputPath      string   `json:"output_path"`
	DocsCount       int      `json:"docs_count"`
	PagesCount      int      `json:"pages_count"`
	EstimatedTokens int      `json:"estimated_tokens"`
	UnreadDocs      int      `json:"unread_docs"`
	Error           string   `json:"error"`
	Timestamp       int64    `json:"timestamp"`
	FinishedAt      int64    `json:"finished_at"`
	OcrMode         string   `json:"ocr_mode"`
	CleaningLevel   string   `json:"cleaning_level"`
	Kind            string   `json:"kind"`
	Paths           []string `json:"paths,omitempty"`
	// SaidaAnterior é o TXT que um reprocesso ainda em curso vai sobrescrever
	// (QueueItem.saidaAnterior): sem ele, um PDF interrompido no meio de um
	// reprocesso voltaria sem saída e gravaria <nome>_2.txt.
	SaidaAnterior string `json:"saida_anterior,omitempty"`
	// Relatorio é o relatório do último sucesso, só com contagens: Atencao
	// (os nomes dos documentos a conferir) nunca é gravado (decisão d3 de
	// 2026-11-26).
	Relatorio *models.RelatorioZip `json:"relatorio,omitempty"`
}

// arquivoHistorico é o conteúdo de historico.json.
type arquivoHistorico struct {
	Versao int    `json:"versao"`
	Itens  []Item `json:"itens"`
}

// bom é a marca de ordem de bytes do UTF-8 (Bloco de Notas).
var bom = []byte{0xEF, 0xBB, 0xBF}

// statusValidos são os status que um item guardado pode ter.
var statusValidos = map[string]bool{
	"pending": true, "processing": true, "success": true,
	"error": true, "cancelled": true, "interrupted": true,
}

// Carregar lê o histórico, valida item a item, aplica o Teto e devolve os
// itens do mais novo para o mais antigo. Arquivo ausente devolve lista vazia
// sem erro. Conteúdo corrompido (JSON inválido, vazio, versao menor que 1) é
// renomeado para <arquivo>.corrompido-AAAAMMDD-HHMMSS e a lista volta vazia,
// sem erro. Versão maior que VersaoFormato devolve ErrVersaoNova sem tocar no
// arquivo.
func Carregar(arquivo string) ([]Item, error) {
	dados, err := os.ReadFile(arquivo)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dados = bytes.TrimPrefix(dados, bom)
	var h arquivoHistorico
	if err := json.Unmarshal(dados, &h); err != nil || h.Versao < 1 {
		separarCorrompido(arquivo)
		return nil, nil
	}
	if h.Versao > VersaoFormato {
		return nil, ErrVersaoNova
	}
	itens := make([]Item, 0, len(h.Itens))
	for _, it := range h.Itens {
		if it.Kind == "" {
			it.Kind = kindDoCaminho(it.Path)
		}
		if itemValido(it) {
			// Saída anterior que não é do item só perde o campo: o item vale.
			if it.SaidaAnterior != "" && !saida.SaidaDoItem(it.Kind, it.Path, it.Paths, it.SaidaAnterior) {
				it.SaidaAnterior = ""
			}
			itens = append(itens, it)
		}
	}
	return Podar(itens, Teto), nil
}

// separarCorrompido tira o arquivo do caminho, para a lista abrir vazia sem
// perder o conteúdo.
func separarCorrompido(arquivo string) {
	novo := arquivo + ".corrompido-" + time.Now().Format("20060102-150405")
	if err := os.Rename(arquivo, novo); err != nil {
		slog.Warn("histórico corrompido; a lista abre vazia", "arquivo", arquivo, "erro", err)
		return
	}
	slog.Warn("histórico corrompido; a lista abre vazia", "arquivo", novo)
}

// kindDoCaminho deriva o tipo do item da extensão, para arquivo sem kind.
func kindDoCaminho(caminho string) string {
	if strings.EqualFold(filepath.Ext(caminho), ".pdf") {
		return "pdf"
	}
	return "zip"
}

// caminhoValido aceita caminho absoluto com extensão .zip ou .pdf (sem caixa).
func caminhoValido(caminho string) bool {
	if !filepath.IsAbs(caminho) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(caminho))
	return ext == ".zip" || ext == ".pdf"
}

// itemValido aplica as regras de um item guardado (kind já preenchido).
func itemValido(it Item) bool {
	if (it.Kind != "zip" && it.Kind != "pdf") || !caminhoValido(it.Path) || !statusValidos[it.Status] {
		return false
	}
	for _, p := range it.Paths {
		if !caminhoValido(p) {
			return false
		}
	}
	return it.OutputPath == "" || saida.SaidaDoItem(it.Kind, it.Path, it.Paths, it.OutputPath)
}

// Podar devolve no máximo teto itens, os primeiros (os mais novos).
func Podar(itens []Item, teto int) []Item {
	if teto < 0 || len(itens) <= teto {
		return itens
	}
	return itens[:teto]
}
