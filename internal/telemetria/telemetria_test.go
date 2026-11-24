package telemetria

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"eprocia/internal/models"
)

func TestTelemetria_SemRede(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "eprocia/internal/telemetria").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for linha := range strings.SplitSeq(strings.ReplaceAll(string(out), "\r", ""), "\n") {
		switch {
		case linha == "net", linha == "net/http", strings.HasPrefix(linha, "net/"),
			strings.HasPrefix(linha, "golang.org/x/sys/windows"):
			t.Errorf("telemetria depende de %q", linha)
		}
	}
	if !strings.Contains(string(out), "eprocia/internal/telemetria") {
		t.Errorf("saída inesperada do go list: %s", out)
	}
}

func TestColetarMaquina(t *testing.T) {
	m := ColetarMaquina()
	if m.CPUs <= 0 || m.Arquitetura == "" {
		t.Errorf("CPUs = %d, Arquitetura = %q", m.CPUs, m.Arquitetura)
	}
	if runtime.GOOS != "windows" {
		return
	}
	if m.MemoriaTotal == 0 || m.Build == 0 || m.Windows == "" {
		t.Errorf("máquina incompleta: %+v", m)
	}
}

func execucaoDeTeste(nivel string, antes int64) models.ExecucaoZip {
	return models.ExecucaoZip{
		ItemID:     "11111111-2222-3333-4444-555555555555",
		Reprocesso: true,
		Origem:     "arrastada",
		Config: models.Config{
			MonitoredPath: `C:\Users\fulano\Processos`,
			OcrMode:       "normal",
			CleaningLevel: nivel,
			OmitirTipos:   []string{"EXTR"},
			ForcarOCR:     true,
		},
		BytesZip: 12345,
		Status:   "success",
		Relatorio: &models.RelatorioZip{
			Documentos: models.DocsRelatorio{
				Total: 3, Lidos: 2, NaoLidos: 0, Omitidos: 1, DuplicadosH7: 1, PDF: 2, HTML: 1,
				TiposOmitidos: []string{"EXTR"},
				PorLeitura:    map[string]int{"senha": 1},
			},
			Paginas: models.PaginasRelatorio{
				Total: 10, LidaNativa: 6, LidaOCR: 2, Vazia: 1, SemOCR: 1, ComErro: 0,
				OCRSemGanho: 1, OCRFalhouComReserva: 1, DeNaoLidos: 2, DeOmitidos: 3, Duplicadas: 1,
				PorMotivo: map[string]int{"padrao": 2, "forcado": 1},
			},
			Tempo:    models.TempoRelatorio{TotalMs: 900, AbrirMs: 5, OCRTrabalhoMs: 1200},
			OCR:      models.OCRRelatorio{Processos: 2, Blocos: 1, BlocoMedianaMs: 300},
			Limpeza:  models.LimpezaRelatorio{Nivel: nivel, BytesAntes: antes, BytesDepois: antes / 2, BytesDuplicatas: 7},
			BytesTxt: 4096,
			Tokens:   1024,
			Atencao: []models.DocAtencao{{
				Nome: "5000000-00.2026.8.26.0001 - Evento 3 - PET1.pdf", Evento: 3, SemOCR: []int{2}, TotalSemOCR: 1,
			}},
		},
	}
}

func jsonDe(t *testing.T, r Registro) string {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTelemetria_SemNomes(t *testing.T) {
	m := Maquina{CPUs: 8, MemoriaTotal: 16 << 30, Windows: "10.0", Build: 26100, Arquitetura: "amd64"}
	r := Montar(execucaoDeTeste("standard", 1000), m, "0.5.0", time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC))
	s := jsonDe(t, r)
	for _, proibido := range []string{"5000000", "PET1", "EXTR", "fulano", "Processos", "Users"} {
		if strings.Contains(s, proibido) {
			t.Errorf("JSON contém %q: %s", proibido, s)
		}
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	cfg, _ := v["config"].(map[string]any)
	if cfg["tipos_omitidos"] != float64(1) {
		t.Errorf("config.tipos_omitidos = %v, quer 1: %s", cfg["tipos_omitidos"], s)
	}
	if cfg["origem"] != "arrastada" || cfg["ocr_mode"] != "normal" || cfg["cleaning_level"] != "standard" || cfg["forcar_ocr"] != true {
		t.Errorf("config = %v", cfg)
	}
	limp, _ := v["limpeza"].(map[string]any)
	if limp["reducao_pct"] != float64(50) {
		t.Errorf("limpeza.reducao_pct = %v, quer 50: %s", limp["reducao_pct"], s)
	}
	for _, chave := range []string{"versao", "ts", "item_id", "reprocesso", "maquina", "tamanhos", "tempos_ms", "paginas", "ocr", "falhas", "resultado"} {
		if _, ok := v[chave]; !ok {
			t.Errorf("sem a chave %q: %s", chave, s)
		}
	}
	if v["item_id"] != "11111111-2222-3333-4444-555555555555" || v["reprocesso"] != true || v["versao"] != "0.5.0" {
		t.Errorf("cabeçalho = %v %v %v", v["item_id"], v["reprocesso"], v["versao"])
	}
	tam, _ := v["tamanhos"].(map[string]any)
	if tam["bytes_zip"] != float64(12345) || tam["pdf"] != float64(2) || tam["html"] != float64(1) ||
		tam["paginas"] != float64(10) || tam["bytes_txt"] != float64(4096) || tam["tokens"] != float64(1024) {
		t.Errorf("tamanhos = %v", tam)
	}
	pag, _ := v["paginas"].(map[string]any)
	pm, _ := pag["por_motivo"].(map[string]any)
	if pag["ocr_sem_ganho"] != float64(1) || pag["sem_ocr"] != float64(1) || pm["forcado"] != float64(1) {
		t.Errorf("paginas = %v", pag)
	}
	fal, _ := v["falhas"].(map[string]any)
	pl, _ := fal["por_leitura"].(map[string]any)
	if pl["senha"] != float64(1) {
		t.Errorf("falhas = %v", fal)
	}
	res, _ := v["resultado"].(map[string]any)
	if res["status"] != "success" {
		t.Errorf("resultado = %v", res)
	}
	maq, _ := v["maquina"].(map[string]any)
	if maq["cpus"] != float64(8) || maq["build"] != float64(26100) {
		t.Errorf("maquina = %v", maq)
	}
	if !strings.Contains(s, `"ts":"2026-11-24T10:00:00Z"`) {
		t.Errorf("ts: %s", s)
	}
}

func TestTelemetria_NoneSemReducao(t *testing.T) {
	for _, c := range []struct {
		nivel string
		antes int64
	}{{"none", 1000}, {"standard", 0}} {
		s := jsonDe(t, Montar(execucaoDeTeste(c.nivel, c.antes), Maquina{}, "x", time.Now()))
		if strings.Contains(s, "reducao_pct") {
			t.Errorf("nível %q, antes %d: tem reducao_pct: %s", c.nivel, c.antes, s)
		}
	}
}

func TestTelemetria_ErroSemRelatorio(t *testing.T) {
	e := models.ExecucaoZip{ItemID: "u", Origem: "monitorada", Status: "error", ClasseErro: "config",
		Config: models.Config{OcrMode: "none", ForcarOCR: true}}
	s := jsonDe(t, Montar(e, Maquina{}, "x", time.Now()))
	if !strings.Contains(s, `"classe_erro":"config"`) || !strings.Contains(s, `"forcar_ocr":false`) {
		t.Errorf("registro de erro: %s", s)
	}
}

// linhasJSON lê um .jsonl e confere que cada linha é JSON.
func linhasJSON(t *testing.T, caminho string) int {
	t.Helper()
	f, err := os.Open(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var v map[string]any
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			t.Fatalf("linha não é JSON: %q", sc.Text())
		}
		n++
	}
	return n
}

func TestGravador_UmaLinhaPorRegistroERotacao(t *testing.T) {
	dir := t.TempDir()
	r := Montar(execucaoDeTeste("standard", 1000), Maquina{}, "x", time.Now())
	tam := int64(len(jsonDe(t, r)) + 1)

	g, err := abrirGravador(dir, 3*tam+10)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		g.Gravar(r)
	}
	g.Fechar()
	ativo := filepath.Join(dir, "telemetria", "telemetria.jsonl")
	if n := linhasJSON(t, ativo); n != 3 {
		t.Fatalf("linhas = %d, quer 3", n)
	}

	// Mais um passa do limite: o ativo vira telemetria.1.jsonl.
	g, err = abrirGravador(dir, 3*tam+10)
	if err != nil {
		t.Fatal(err)
	}
	g.Gravar(r)
	g.Fechar()
	if n := linhasJSON(t, filepath.Join(dir, "telemetria", "telemetria.1.jsonl")); n != 3 {
		t.Errorf("arquivado com %d linhas, quer 3", n)
	}
	if n := linhasJSON(t, ativo); n != 1 {
		t.Errorf("ativo com %d linhas, quer 1", n)
	}

	// Arquivo antigo não é apagado por idade.
	velho := time.Now().AddDate(-2, 0, 0)
	arq := filepath.Join(dir, "telemetria", "telemetria.1.jsonl")
	if err := os.Chtimes(arq, velho, velho); err != nil {
		t.Fatal(err)
	}
	g, err = NovoGravador(dir)
	if err != nil {
		t.Fatal(err)
	}
	g.Gravar(r)
	g.Fechar()
	if _, err := os.Stat(arq); err != nil {
		t.Errorf("arquivado antigo sumiu: %v", err)
	}
}

type destinoFalho struct{ escritas int }

func (d *destinoFalho) Write(p []byte) (int, error) {
	d.escritas++
	return 0, errors.New("disco cheio (sintético)")
}
func (d *destinoFalho) Close() error { return nil }

func TestGravador_FalhaDescartaEAvisaUmaVez(t *testing.T) {
	antes, w, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antes)
		log.SetOutput(w)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	d := &destinoFalho{}
	g := iniciarGravador(d)
	r := Montar(execucaoDeTeste("standard", 1000), Maquina{}, "x", time.Now())
	feito := make(chan struct{})
	go func() {
		g.Gravar(r)
		g.Gravar(r)
		g.Fechar()
		close(feito)
	}()
	select {
	case <-feito:
	case <-time.After(10 * time.Second):
		t.Fatal("Gravar/Fechar bloqueou")
	}
	if d.escritas != 2 {
		t.Errorf("escritas tentadas = %d, quer 2", d.escritas)
	}
	avisos := 0
	for l := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if strings.Contains(l, `"level":"WARN"`) {
			avisos++
			if !strings.Contains(l, "telemetria") {
				t.Errorf("WARN sem menção à telemetria: %s", l)
			}
		}
	}
	if avisos != 1 {
		t.Errorf("WARNs = %d, quer 1:\n%s", avisos, buf.String())
	}
}

// destinoCorta grava só a metade da primeira escrita e devolve erro; as
// seguintes gravam inteiras.
type destinoCorta struct {
	buf     bytes.Buffer
	cortado bool
}

func (d *destinoCorta) Write(p []byte) (int, error) {
	if !d.cortado {
		d.cortado = true
		n, _ := d.buf.Write(p[:len(p)/2])
		return n, errors.New("disco cheio (sintético)")
	}
	return d.buf.Write(p)
}
func (d *destinoCorta) Close() error { return nil }

func TestGravador_LinhaInteiraAposGravacaoParcial(t *testing.T) {
	antes, w, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antes)
		log.SetOutput(w)
		log.SetFlags(flags)
	})
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))

	d := &destinoCorta{}
	g := iniciarGravador(d)
	r := Montar(execucaoDeTeste("standard", 1000), Maquina{}, "x", time.Now())
	g.Gravar(r)
	g.Gravar(r)
	g.Fechar()

	linhas := strings.Split(strings.TrimSuffix(d.buf.String(), "\n"), "\n")
	if len(linhas) != 2 {
		t.Fatalf("linhas = %d, quer 2 (meia + inteira):\n%s", len(linhas), d.buf.String())
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(linhas[1]), &v); err != nil {
		t.Errorf("linha depois da parcial não é JSON: %q: %v", linhas[1], err)
	}
}
