package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eprocia/internal/registro"
)

// A linha de log do Form ignorado é a única saída da v0.4.2 sujeita à regra de
// privacidade: só o número da página, o motivo fixo e a contagem, nunca o nome
// do XObject nem o erro da biblioteca. Ela sai em WARN; o resto da página, em
// DEBUG.
func TestLogFormIgnorado_SoNumerosEMotivoFixo(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := registro.ComLogger(context.Background(), lg)

	objs := paginaComXObjects(corpoSintetico(10, 780)+"/FmSEGREDO Do /FmSEGREDODOIS Do", "/FmSEGREDO 6 0 R /FmSEGREDODOIS 7 0 R",
		formXObject("BT /F1 12 Tf (sem fecho", ""),
		formXObject("BT /F1 12 Tf (sem fecho", "/Filter /DCTDecode"))
	caminho := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(caminho, buildPDF(objs), 0644); err != nil {
		t.Fatal(err)
	}
	runner, _ := ocrFalso(ocrFalsoTexto, nil)
	parser := parserDeArquivo(t, caminho)
	feito := make(chan error, 1)
	go func() {
		_, err := parser.ParsePages(ctx, true, runner, nil)
		feito <- err
	}()
	select {
	case err := <-feito:
		if err != nil {
			t.Fatalf("ParsePages: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("ParsePages travou (loop infinito na extração de texto nativo)")
	}

	motivos := map[string]bool{"stream ilegível": true, "profundidade": true, "limite por página": true, "pânico": true}
	vistos := map[string]bool{}
	for l := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if strings.Contains(l, "SEGREDO") {
			t.Errorf("log com o nome do XObject: %q", l)
		}
		if l == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("linha não é JSON: %q", l)
		}
		if r["level"] == "DEBUG" {
			continue
		}
		pag, _ := r["pag"].(float64)
		if r["msg"] != "Form XObject ignorado" {
			if pag == 1 {
				t.Errorf("linha acima de DEBUG para a página 1: %q", l)
			}
			continue
		}
		motivo, _ := r["motivo"].(string)
		if r["level"] != "WARN" || pag != 1 || !motivos[motivo] || r["n"] != float64(1) {
			t.Errorf("linha fora do formato: %q", l)
			continue
		}
		vistos[motivo] = true
	}
	for _, motivo := range []string{"stream ilegível", "pânico"} {
		if !vistos[motivo] {
			t.Errorf("sem a linha WARN do motivo %q no log:\n%s", motivo, buf.String())
		}
	}
}
