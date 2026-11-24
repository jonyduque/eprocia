package etl

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"testing"
)

// mensagensAcimaDeDebug são as únicas chamadas Info/Warn/Error permitidas nos
// arquivos de leitura de PDF, todas em Warn: o resto (por página, por
// atualização) fica em Debug, para um ZIP grande não encher o log em INFO.
var mensagensAcimaDeDebug = map[string]bool{
	"Form XObject ignorado":                          true,
	"página com falha de OCR":                        true,
	"OCR em bloco falhou; refazendo página a página": true,
	"falha ao abrir o PDF":                           true,
	"árvore de páginas inválida":                     true,
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
			return true
		})
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
