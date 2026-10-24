package etl

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// arquivosModernizados são os fontes cobertos por B-GM-01..04, relativos a
// este pacote.
var arquivosModernizados = []string{
	"formatter.go",
	"pdf_parser.go",
	"pipeline.go",
	"queue.go",
	"sorter.go",
	"../config/config.go",
	"../watchdog/watchdog.go",
	"../../main.go",
}

// chamadasObsoletas mapeia pacote.função para o substituto esperado.
var chamadasObsoletas = map[string]string{
	"sort.Slice":       "slices.SortFunc",
	"sort.Ints":        "slices.Sort",
	"atomic.AddInt32":  "atomic.Int32.Add",
	"atomic.LoadInt32": "atomic.Int32.Load",
	"os.IsNotExist":    "errors.Is(err, fs.ErrNotExist)",
}

// ehWaitGroup reconhece os WaitGroups do projeto pelo nome (wg, w.wg, q.wg).
func ehWaitGroup(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name == "wg"
	case *ast.SelectorExpr:
		return x.Sel.Name == "wg"
	}
	return false
}

func TestModernizacao_SemIdiomasObsoletos(t *testing.T) {
	fset := token.NewFileSet()
	for _, rel := range arquivosModernizados {
		arq, err := parser.ParseFile(fset, filepath.FromSlash(rel), nil, 0)
		if err != nil {
			t.Fatalf("falha ao ler %s: %v", rel, err)
		}
		ast.Inspect(arq, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.InterfaceType:
				if len(x.Methods.List) == 0 {
					t.Errorf("%s: interface{} no lugar de any", fset.Position(x.Pos()))
				}
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					break
				}
				if pkg, ok := sel.X.(*ast.Ident); ok {
					if novo, ok := chamadasObsoletas[pkg.Name+"."+sel.Sel.Name]; ok {
						t.Errorf("%s: %s.%s no lugar de %s", fset.Position(x.Pos()), pkg.Name, sel.Sel.Name, novo)
					}
				}
				if (sel.Sel.Name == "Add" || sel.Sel.Name == "Done") && ehWaitGroup(sel.X) {
					t.Errorf("%s: WaitGroup.%s manual no lugar de WaitGroup.Go", fset.Position(x.Pos()), sel.Sel.Name)
				}
			case *ast.BinaryExpr:
				if c, ok := x.X.(*ast.CallExpr); ok && x.Op == token.QUO {
					if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "UnixNano" {
						t.Errorf("%s: UnixNano()/Millisecond no lugar de UnixMilli()", fset.Position(x.Pos()))
					}
				}
			}
			return true
		})
	}
}

// go fix -diff vazio garante os demais modernizadores (max, maps.Copy,
// slices.Backward, for range n, WaitGroup.Go) sem listá-los um a um.
func TestModernizacao_GoFixSemDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("roda o go fix; pulado com -short")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go não está no PATH")
	}
	cmd := exec.Command(goBin, "fix", "-diff", "eprocia/internal/...")
	saida, _ := cmd.CombinedOutput()
	if s := strings.TrimSpace(string(saida)); s != "" {
		t.Errorf("go fix -diff propõe mudanças em internal/:\n%s", s)
	}
}
