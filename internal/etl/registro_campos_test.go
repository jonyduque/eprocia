package etl

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"eprocia/internal/models"
	"eprocia/internal/ocr"
	"eprocia/internal/registro"
)

// ctxComLogJSON devolve um contexto com um logger JSON em memória, no nível
// dado, já com o item_id que a fila põe.
func ctxComLogJSON(nivel slog.Level) (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: nivel})).With("item_id", "item-teste")
	return registro.ComLogger(context.Background(), lg), &buf
}

// gravarZipComo grava um ZIP com o nome dado numa pasta temporária.
func gravarZipComo(t *testing.T, nome string, entradas []entradaZip) string {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, e := range entradas {
		w, err := zw.Create(e.nome)
		if err != nil {
			t.Fatalf("criar entrada %s: %v", e.nome, err)
		}
		if _, err := w.Write(e.conteudo); err != nil {
			t.Fatalf("escrever entrada %s: %v", e.nome, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fechar zip: %v", err)
	}
	caminho := filepath.Join(t.TempDir(), nome)
	if err := os.WriteFile(caminho, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("gravar zip: %v", err)
	}
	return caminho
}

func linhasComMsg(linhas []map[string]any, msg string) []map[string]any {
	var out []map[string]any
	for _, l := range linhas {
		if l["msg"] == msg {
			out = append(out, l)
		}
	}
	return out
}

func TestRegistro_CamposDeZipEDocumento(t *testing.T) {
	isolarTemp(t)
	const textoPDF = "Texto ficticio de FULANO DE TAL"
	nomes := []string{"Evento 1 - DOC1.pdf", "Evento 2 - PET2.html"}
	zipPath := gravarZipComo(t, "5000000-00.2026.8.26.0001.zip", []entradaZip{
		{nomes[0], pdfComTexto(textoPDF)},
		{nomes[1], []byte("<html><body><p>Peticao do evento dois</p></body></html>")},
	})
	ctx, buf := ctxComLogJSON(slog.LevelInfo)

	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfgSemOCR(), nil, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	out := buf.String()
	linhas := linhasJSON(t, out)

	ini := linhasComMsg(linhas, "zip iniciado")
	fim := linhasComMsg(linhas, "zip concluído")
	if len(ini) != 1 || len(fim) != 1 {
		t.Fatalf("zip iniciado = %d, zip concluído = %d; quero 1 e 1:\n%s", len(ini), len(fim), out)
	}
	for _, l := range []map[string]any{ini[0], fim[0]} {
		if l["item_id"] != "item-teste" {
			t.Errorf("linha %q sem item_id: %v", l["msg"], l)
		}
		if l["level"] != "INFO" {
			t.Errorf("linha %q com nível %v", l["msg"], l["level"])
		}
	}
	if ini[0]["zip"] != "5000000-00.2026.8.26.0001.zip" {
		t.Errorf("zip = %v", ini[0]["zip"])
	}
	if ini[0]["pdf"] != 1.0 || ini[0]["html"] != 1.0 {
		t.Errorf("pdf/html = %v/%v", ini[0]["pdf"], ini[0]["html"])
	}
	if b, _ := ini[0]["bytes"].(float64); b <= 0 {
		t.Errorf("bytes = %v", ini[0]["bytes"])
	}
	cfg, _ := ini[0]["config"].(map[string]any)
	if cfg["ocr_mode"] != "none" || cfg["cleaning_level"] != "none" {
		t.Errorf("config = %v", ini[0]["config"])
	}
	for _, campo := range []string{"duracao_ms", "bytes_txt", "tokens"} {
		if _, ok := fim[0][campo].(float64); !ok {
			t.Errorf("zip concluído sem %s: %v", campo, fim[0])
		}
	}
	if b, _ := fim[0]["bytes_txt"].(float64); b <= 0 {
		t.Errorf("bytes_txt = %v", fim[0]["bytes_txt"])
	}

	docs := linhasComMsg(linhas, "documento concluído")
	if len(docs) != len(nomes) {
		t.Fatalf("documento concluído = %d linhas, quero %d:\n%s", len(docs), len(nomes), out)
	}
	ids := map[float64]bool{}
	porNome := map[string]map[string]any{}
	for _, d := range docs {
		id, ok := d["doc_id"].(float64)
		if !ok {
			t.Errorf("documento sem doc_id: %v", d)
		}
		ids[id] = true
		if d["item_id"] != "item-teste" {
			t.Errorf("documento sem item_id: %v", d)
		}
		nome, _ := d["doc"].(string)
		porNome[nome] = d
		for _, campo := range []string{"evento", "extensao", "bytes", "paginas", "estado", "leitura", "duracao_ms"} {
			if _, ok := d[campo]; !ok {
				t.Errorf("documento %q sem %s: %v", nome, campo, d)
			}
		}
	}
	if len(ids) != len(nomes) {
		t.Errorf("doc_id repetido: %v", ids)
	}
	for _, n := range nomes {
		if porNome[n] == nil {
			t.Errorf("sem linha do documento %q", n)
		}
	}
	if d := porNome[nomes[0]]; d != nil {
		if d["leitor"] != "biblioteca" || d["leitura"] != string(LeituraLido) || d["estado"] != DocConcluido {
			t.Errorf("PDF: leitor/leitura/estado = %v/%v/%v", d["leitor"], d["leitura"], d["estado"])
		}
		if d["evento"] != 1.0 || d["extensao"] != ".pdf" {
			t.Errorf("PDF: evento/extensao = %v/%v", d["evento"], d["extensao"])
		}
	}

	for _, l := range linhas {
		if _, ok := l["pag"]; ok {
			t.Errorf("linha com pag em INFO: %v", l)
		}
	}
	if strings.Contains(out, "FULANO DE TAL") || strings.Contains(out, "Peticao do evento") {
		t.Errorf("log com o texto do documento:\n%s", out)
	}
}

func TestRegistro_ErroComEtapaEClasse(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "5000000-00.2026.8.26.0001.zip")
	if err := os.WriteFile(zipPath, []byte("bytes que nao formam um zip 0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errDireto := zip.OpenReader(zipPath)
	if errDireto == nil {
		t.Fatal("zip.OpenReader abriu o arquivo inválido")
	}
	ctx, buf := ctxComLogJSON(slog.LevelInfo)

	_, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfgSemOCR(), nil, nil)
	if err == nil {
		t.Fatal("esperava erro")
	}
	var ee ErroEtapa
	if !errors.As(err, &ee) || ee.Etapa != "abrir_zip" {
		t.Fatalf("erro sem ErroEtapa abrir_zip: %#v", err)
	}
	// O ErroEtapa não acrescenta texto: a mensagem é a de ReadZipFiles, que
	// embrulha a do zip.OpenReader.
	if quero := fmt.Sprintf("failed to open zip file: %v", errDireto); err.Error() != quero {
		t.Errorf("err.Error() = %q, quero %q", err.Error(), quero)
	}
	linhas := linhasComMsg(linhasJSON(t, buf.String()), "zip com erro")
	if len(linhas) != 1 {
		t.Fatalf("zip com erro = %d linhas:\n%s", len(linhas), buf.String())
	}
	l := linhas[0]
	if l["level"] != "WARN" {
		t.Errorf("zip com erro em %v, quero WARN", l["level"])
	}
	if l["etapa"] != "abrir_zip" || l["classe"] != string(LeituraIlegivel) || l["item_id"] != "item-teste" {
		t.Errorf("linha de erro: %v", l)
	}
	if e, _ := l["erro"].(string); e != err.Error() {
		t.Errorf("erro = %q, quero %q", e, err.Error())
	}
}

// O documento com erro sai em WARN e o que deu certo em INFO; as linhas da
// pré-contagem (segundo leitor falhou) levam doc e doc_id.
func TestRegistro_DocumentoComErroEmWarn(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZipComo(t, "5000000-00.2026.8.26.0001.zip", []entradaZip{
		{"Evento 1 - DOC1.pdf", []byte("isto nao e um PDF 0123456789")},
		{"Evento 2 - PET2.html", []byte("<html><body><p>Peticao do evento dois</p></body></html>")},
	})
	ctx, buf := ctxComLogJSON(slog.LevelInfo)
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfgSemOCR(), nil, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	linhas := linhasJSON(t, buf.String())
	niveis := map[string]any{}
	for _, d := range linhasComMsg(linhas, "documento concluído") {
		nome, _ := d["doc"].(string)
		niveis[nome] = d["level"]
	}
	if niveis["Evento 1 - DOC1.pdf"] != "WARN" || niveis["Evento 2 - PET2.html"] != "INFO" {
		t.Errorf("níveis de documento concluído = %v; quero PDF WARN e HTML INFO\n%s", niveis, buf.String())
	}
	for _, msg := range []string{"segundo leitor falhou"} {
		ls := linhasComMsg(linhas, msg)
		if len(ls) != 1 {
			t.Errorf("%q = %d linhas, quero 1:\n%s", msg, len(ls), buf.String())
			continue
		}
		if ls[0]["doc"] != "Evento 1 - DOC1.pdf" || ls[0]["doc_id"] != 0.0 || ls[0]["item_id"] != "item-teste" {
			t.Errorf("%q sem doc/doc_id/item_id: %v", msg, ls[0])
		}
	}
}

func TestRegistro_PanicComPilha(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZipComo(t, "5000000-00.2026.8.26.0001.zip",
		[]entradaZip{{"Evento 1 - DOC1.pdf", pdfComTexto("Texto ficticio de FULANO DE TAL")}})
	ctx, buf := ctxComLogJSON(slog.LevelInfo)
	progresso := func(_ float64, msg string, _, _, _, _ int) {
		if strings.HasPrefix(msg, "Processando ") {
			panic("panic sintetico")
		}
	}
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfgSemOCR(), progresso, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	achou := false
	for _, l := range linhasJSON(t, buf.String()) {
		if l["level"] != "ERROR" {
			continue
		}
		achou = true
		if p, _ := l["pilha"].(string); strings.TrimSpace(p) == "" {
			t.Errorf("linha ERROR sem pilha: %v", l)
		}
		if l["doc_id"] == nil || l["item_id"] != "item-teste" {
			t.Errorf("linha ERROR sem doc_id/item_id: %v", l)
		}
	}
	if !achou {
		t.Fatalf("sem linha ERROR do panic:\n%s", buf.String())
	}
}

// atributosErro leva etapa, classe, código de saída e stderr; o texto do erro
// continua sem o stderr.
func TestAtributosErro_ProcessoExterno(t *testing.T) {
	err := ErroEtapa{Etapa: "ocr", Err: fmt.Errorf("bloco: %w", &ocr.ErroProcesso{
		Programa: "tesseract", Codigo: 3, Stderr: "STDERR-SINTETICO", Err: errors.New("exit status 3")})}
	attrs := atributosErro(err)
	m := map[string]any{}
	for i := 0; i+1 < len(attrs); i += 2 {
		m[attrs[i].(string)] = attrs[i+1]
	}
	if m["etapa"] != "ocr" || m["classe"] != string(LeituraIlegivel) || m["codigo_saida"] != 3 || m["stderr"] != "STDERR-SINTETICO" {
		t.Errorf("atributos = %v", m)
	}
	e, ok := m["erro"].(error)
	if !ok || e.Error() != err.Error() || strings.Contains(e.Error(), "STDERR-SINTETICO") {
		t.Errorf("erro = %v", m["erro"])
	}
	if atributosErro(errors.New("x"))[0] != "erro" {
		t.Error("atributosErro sem erro primeiro")
	}
	if atributosErro(nil) != nil {
		t.Error("atributosErro(nil) não vazio")
	}
}

// A linha Warn da falha de OCR leva o stderr do processo externo.
func TestRegistro_FalhaDeOCRLevaStderrNoWarn(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) { return []string{""}, nil })
	ctx, buf := ctxComLogJSON(slog.LevelInfo)
	runner, _ := ocrFalso("", &ocr.ErroProcesso{Programa: "tesseract", Codigo: 1, Stderr: "STDERR-SINTETICO", Err: errors.New("exit status 1")})

	if _, err := parserPopplerEm("doc.pdf", 1).ParsePages(ctx, true, runner, nil); err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	linhas := linhasComMsg(linhasJSON(t, buf.String()), "página com falha de OCR")
	if len(linhas) != 1 {
		t.Fatalf("linhas Warn = %d:\n%s", len(linhas), buf.String())
	}
	if linhas[0]["stderr"] != "STDERR-SINTETICO" || linhas[0]["codigo_saida"] != 1.0 {
		t.Errorf("linha Warn sem stderr/código: %v", linhas[0])
	}
}

// Documento omitido por tipo: leitura "omitido", o tipo no atributo omitido e
// sem leitor (o arquivo nem é aberto).
func TestRegistro_DocumentoOmitido(t *testing.T) {
	isolarTemp(t)
	zipPath := gravarZipComo(t, "5000000-00.2026.8.26.0001.zip", []entradaZip{
		{"Evento 1 - EXTR1.pdf", pdfComTexto("Extrato ficticio")},
		{"Evento 2 - DOC1.pdf", pdfComTexto("Texto ficticio de FULANO DE TAL")},
	})
	cfg := cfgSemOCR()
	cfg.OmitirTipos = []string{"EXTR"}
	ctx, buf := ctxComLogJSON(slog.LevelInfo)
	if _, _, _, _, err := NewPipeline().ProcessZipComAtividade(ctx, zipPath, cfg, nil, nil); err != nil {
		t.Fatalf("ProcessZip: %v", err)
	}
	achou := false
	for _, d := range linhasComMsg(linhasJSON(t, buf.String()), "documento concluído") {
		if d["doc"] != "Evento 1 - EXTR1.pdf" {
			if d["leitor"] != "biblioteca" {
				t.Errorf("documento lido sem leitor: %v", d)
			}
			continue
		}
		achou = true
		if d["leitura"] != "omitido" || d["omitido"] != "EXTR" {
			t.Errorf("omitido: leitura/omitido = %v/%v", d["leitura"], d["omitido"])
		}
		if _, ok := d["leitor"]; ok {
			t.Errorf("omitido com leitor: %v", d)
		}
	}
	if !achou {
		t.Fatalf("sem linha do omitido:\n%s", buf.String())
	}
}

// O pânico no processador da fila vai em ERROR com pilha e item_id. Sem
// t.Parallel: troca slog.Default().
func TestRegistro_PanicDaFilaComPilha(t *testing.T) {
	padrao, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(padrao)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var mu sync.Mutex
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&escritorTravado{mu: &mu, w: &buf}, nil)))

	q, _ := novaFilaGravada(t, func(context.Context, Entrada, *models.Config, func(float64, string, int, int, int, int)) (string, int, int, int, error) {
		panic("panic sintetico da fila")
	})
	q.AddFiles([]string{"5000000-00.2026.8.26.0001.zip"})
	esperarFim(t, q, "5000000-00.2026.8.26.0001.zip")
	id := q.GetItems()[0].ID

	mu.Lock()
	out := buf.String()
	mu.Unlock()
	achou := false
	for _, l := range linhasJSON(t, out) {
		if l["level"] != "ERROR" {
			continue
		}
		achou = true
		if p, _ := l["pilha"].(string); strings.TrimSpace(p) == "" {
			t.Errorf("linha ERROR sem pilha: %v", l)
		}
		if l["item_id"] != id {
			t.Errorf("item_id = %v, quero %s", l["item_id"], id)
		}
	}
	if !achou {
		t.Fatalf("sem linha ERROR do pânico da fila:\n%s", out)
	}
}

type escritorTravado struct {
	mu *sync.Mutex
	w  io.Writer
}

func (e *escritorTravado) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.w.Write(p)
}
