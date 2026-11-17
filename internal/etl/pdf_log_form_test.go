package etl

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
)

// A linha de log do Form ignorado é a única saída nova da v0.4.2 sujeita à
// regra de privacidade: só o número da página, o motivo fixo e a contagem,
// nunca o nome do XObject nem o erro da biblioteca. Sem t.Parallel: troca a
// saída do log global.
func TestLogFormIgnorado_SoNumerosEMotivoFixo(t *testing.T) {
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(saida)
		log.SetFlags(flags)
	})

	objs := paginaComXObjects(corpoSintetico(10, 780)+"/FmSEGREDO Do /FmSEGREDODOIS Do", "/FmSEGREDO 6 0 R /FmSEGREDODOIS 7 0 R",
		formXObject("BT /F1 12 Tf (sem fecho", ""),
		formXObject("BT /F1 12 Tf (sem fecho", "/Filter /DCTDecode"))
	runner, _ := ocrFalso(ocrFalsoTexto, nil)
	parsePagesWithDeadline(t, objs, runner)

	linha := regexp.MustCompile(`^\[PDFParser\] Página 1: Form XObject ignorado \((stream ilegível|profundidade|limite por página|pânico)\): 1$`)
	vistos := map[string]bool{}
	for l := range strings.SplitSeq(buf.String(), "\n") {
		if strings.Contains(l, "SEGREDO") {
			t.Errorf("log com o nome do XObject: %q", l)
		}
		if !strings.Contains(l, "Form XObject ignorado") {
			continue
		}
		m := linha.FindStringSubmatch(l)
		if m == nil {
			t.Errorf("linha fora do formato: %q", l)
			continue
		}
		vistos[m[1]] = true
	}
	for _, motivo := range []string{"stream ilegível", "pânico"} {
		if !vistos[motivo] {
			t.Errorf("sem a linha do motivo %q no log:\n%s", motivo, buf.String())
		}
	}
}
