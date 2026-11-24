package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"eprocia/internal/registro"
)

// mensagensAcimaDeDebug são as únicas chamadas Info/Warn/Error permitidas nos
// arquivos de leitura de PDF, todas em Warn: o resto (por página, por
// atualização) fica em Debug, para um ZIP grande não encher o log em INFO.
var mensagensAcimaDeDebug = map[string]bool{
	"Form XObject ignorado":                                           true,
	"página com falha de OCR":                                         true,
	"OCR em bloco falhou; refazendo página a página":                  true,
	"falha ao abrir o PDF":                                            true,
	"árvore de páginas inválida":                                      true,
	"segundo leitor não extraiu o texto; todas as páginas vão ao OCR": true,
}

func analisarFonte(t *testing.T, arquivo string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), arquivo, nil, 0)
	if err != nil {
		t.Fatalf("ler %s: %v", arquivo, err)
	}
	return f
}

// chamadaLogPrint diz se a chamada é log.Print*.
func chamadaLogPrint(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "log" && strings.HasPrefix(sel.Sel.Name, "Print")
}

func TestRegistro_SemLogPorPaginaForaDeDebug(t *testing.T) {
	emWarn := map[string]bool{}
	for _, arquivo := range []string{"pdf_parser.go", "pdf_poppler.go", "pdf_ocr_despacho.go", "pdf_texto.go"} {
		f := analisarFonte(t, arquivo)
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if chamadaLogPrint(c) {
				t.Errorf("%s: chamada log.%s", arquivo, c.Fun.(*ast.SelectorExpr).Sel.Name)
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Info", "Warn", "Error", "InfoContext", "WarnContext", "ErrorContext":
			default:
				return true
			}
			msg := ""
			if len(c.Args) > 0 {
				if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					msg, _ = strconv.Unquote(lit.Value)
				}
			}
			if !mensagensAcimaDeDebug[msg] || sel.Sel.Name != "Warn" {
				t.Errorf("%s: %s(%q) fora da lista (só Warn com as mensagens fixas)", arquivo, sel.Sel.Name, msg)
			}
			emWarn[msg] = true
			return true
		})
	}
	for msg := range mensagensAcimaDeDebug {
		if !emWarn[msg] {
			t.Errorf("mensagem %q da lista sem chamada Warn", msg)
		}
	}

	// queue.go: a linha de progresso, a cada atualização, é Debug.
	f := analisarFonte(t, "queue.go")
	achou := false
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		if id, ok := as.Lhs[0].(*ast.Ident); !ok || id.Name != "onProgress" {
			return true
		}
		fl, ok := as.Rhs[0].(*ast.FuncLit)
		if !ok {
			return true
		}
		achou = true
		debug := false
		ast.Inspect(fl.Body, func(m ast.Node) bool {
			c, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			if chamadaLogPrint(c) {
				t.Errorf("queue.go: onProgress chama log.%s", c.Fun.(*ast.SelectorExpr).Sel.Name)
			}
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
				switch sel.Sel.Name {
				case "Debug", "DebugContext":
					debug = true
				case "Info", "Warn", "Error", "InfoContext", "WarnContext", "ErrorContext":
					t.Errorf("queue.go: onProgress chama %s", sel.Sel.Name)
				}
			}
			return true
		})
		if !debug {
			t.Errorf("queue.go: onProgress sem chamada Debug")
		}
		return true
	})
	if !achou {
		t.Fatal("queue.go: onProgress não encontrado")
	}
}

// Guarda de regressão: sem EPROCIA_LOG (slog.Default() intocado), o slog cai no
// pacote log (stderr hoje) em INFO e acima, e o Debug não sai. Sem t.Parallel:
// troca a saída do log global.
func TestRegistro_SemVariavelStderrComoHoje(t *testing.T) {
	padrao, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(padrao)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	log.SetOutput(&buf)

	slog.Debug("linha-debug")
	slog.Info("linha-info")
	slog.Warn("linha-warn")
	slog.Error("linha-error")

	out := buf.String()
	for _, m := range []string{"linha-info", "linha-warn", "linha-error"} {
		if !strings.Contains(out, m) {
			t.Errorf("sem %q na saída do log:\n%s", m, out)
		}
	}
	if strings.Contains(out, "linha-debug") {
		t.Errorf("Debug saiu sem EPROCIA_LOG:\n%s", out)
	}
}

// perfilFicticio é o caminho temporário sob o perfil que o construtor por
// caminho guarda em filePath.
const perfilFicticio = `C:\Users\fulano\AppData\Local\Temp\eprocia-123\doc.pdf`

func parserPopplerEm(caminho string, n int) *PDFParser {
	return NewPDFParserDeLeitor(bytes.NewReader(nil), 0, caminho,
		func() (string, error) { return "doc-ficticio.pdf", nil }).comPoppler(n)
}

func linhasJSON(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for l := range strings.SplitSeq(strings.TrimSpace(s), "\n") {
		if l == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("linha não é JSON: %q", l)
		}
		out = append(out, r)
	}
	return out
}

// Fora do caminho em bloco, o erro do OCR só sairia em Debug: a linha Warn
// do documento leva o primeiro erro.
func TestRegistro_FalhaDeOCRLevaOErroNoWarn(t *testing.T) {
	trocarPoppler(t, nil, func(context.Context, string, int) ([]string, error) { return []string{""}, nil })
	var buf bytes.Buffer
	ctx := registro.ComLogger(context.Background(),
		slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	runner, _ := ocrFalso("", errors.New("tesseract saiu com 1: ERRO-SINTETICO"))

	if _, err := parserPopplerEm("doc.pdf", 1).ParsePages(ctx, true, runner, nil); err != nil {
		t.Fatalf("ParsePages: %v", err)
	}
	achou := false
	for _, r := range linhasJSON(t, buf.String()) {
		if r["msg"] != "página com falha de OCR" {
			continue
		}
		achou = true
		if r["level"] != "WARN" {
			t.Errorf("nível %v", r["level"])
		}
		if e, _ := r["erro"].(string); !strings.Contains(e, "ERRO-SINTETICO") {
			t.Errorf("linha Warn sem o erro do OCR: %v", r)
		}
	}
	if !achou {
		t.Fatalf("sem a linha Warn da falha de OCR:\n%s", buf.String())
	}
}

// O segundo leitor que falha sem OCR loga o nome base, não o temporário sob o
// perfil. Sem t.Parallel: troca a saída do log global.
func TestRegistro_SegundoLeitorLogaSoONomeBase(t *testing.T) {
	saida, flags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	log.SetOutput(&buf)
	ctx := registro.ComLogger(context.Background(),
		slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if _, err := parserPopplerEm(perfilFicticio, 1).ParsePages(ctx, false, nil, nil); err == nil {
		t.Fatal("esperava erro do segundo leitor")
	}
	out := buf.String()
	if strings.Contains(out, "AppData") {
		t.Errorf("log com o caminho temporário:\n%s", out)
	}
	if !strings.Contains(out, "doc.pdf") {
		t.Errorf("log sem o nome do documento:\n%s", out)
	}
}
