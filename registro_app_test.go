package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eprocia/internal/etl"
	"eprocia/internal/models"
	"eprocia/internal/registro"
)

// isolarRegistro guarda slog.Default(), log.Writer() e log.Flags() e os
// restaura no fim do teste, nessa ordem, antes de zerar o registro.
func isolarRegistro(t *testing.T) {
	t.Helper()
	antes, w, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antes)
		log.SetOutput(w)
		log.SetFlags(flags)
		registro.ResetarParaTeste()
	})
}

func cfgDeTeste(t *testing.T) *models.Config {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("sem perfil do usuário: %v", err)
	}
	return &models.Config{
		MonitoredPath:   filepath.Join(home, "Downloads"),
		OcrMode:         "normal",
		CleaningLevel:   "standard",
		WatchdogEnabled: true,
		Theme:           "system",
		OmitirTipos:     []string{},
	}
}

// linhasLog lê logs\eprocia.log de dir, uma linha JSON por item.
func linhasLog(t *testing.T, dir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "logs", "eprocia.log"))
	if err != nil {
		t.Fatalf("log não criado: %v", err)
	}
	defer f.Close()
	var linhas []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("linha não é JSON: %q: %v", sc.Text(), err)
		}
		linhas = append(linhas, m)
	}
	return linhas
}

func acharMsg(linhas []map[string]any, msg string) map[string]any {
	for _, l := range linhas {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestIniciarRegistro_SemVariavel(t *testing.T) {
	dir := t.TempDir() // antes de isolarRegistro: o cleanup do registro fecha o log antes de apagar a pasta
	isolarRegistro(t)
	antes, w := slog.Default(), log.Writer()

	if iniciarRegistro("", dir, cfgDeTeste(t)) {
		t.Fatal("iniciarRegistro sem variável devolveu true")
	}
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 0 {
		t.Errorf("pasta de dados não ficou vazia: %d entradas", len(entradas))
	}
	if slog.Default() != antes {
		t.Error("slog.Default() trocado com o log desligado")
	}
	if log.Writer() != w {
		t.Error("log.Writer() trocado com o log desligado")
	}
	app := &App{}
	if app.LogAtivo() {
		t.Error("LogAtivo() = true com o log desligado")
	}
	if err := app.AbrirPastaLogs(); err == nil || !strings.Contains(err.Error(), "não está ativo") {
		t.Errorf("AbrirPastaLogs() = %v, quer erro com \"não está ativo\"", err)
	}
}

func TestIniciarRegistro_Normal(t *testing.T) {
	dir := t.TempDir() // antes de isolarRegistro: o cleanup do registro fecha o log antes de apagar a pasta
	isolarRegistro(t)

	if !iniciarRegistro("1", dir, cfgDeTeste(t)) {
		t.Fatal("iniciarRegistro(\"1\") devolveu false")
	}
	if !(&App{}).LogAtivo() {
		t.Error("LogAtivo() = false com o log ligado")
	}
	registro.Encerrar()

	linhas := linhasLog(t, dir)
	ini := acharMsg(linhas, "eprocIA iniciado")
	if ini == nil {
		t.Fatalf("sem a linha de início: %v", linhas)
	}
	var wj struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	b, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &wj); err != nil {
		t.Fatal(err)
	}
	if wj.Info.ProductVersion == "" || ini["versao"] != wj.Info.ProductVersion {
		t.Errorf("versao = %v, quer %q", ini["versao"], wj.Info.ProductVersion)
	}
	if _, ok := ini["cpus"].(float64); !ok {
		t.Errorf("cpus ausente: %v", ini["cpus"])
	}
	for _, chave := range []string{"eprocia_log", "windows", "semaforo_docs", "semaforo_ocr", "pasta_logs"} {
		if _, ok := ini[chave]; !ok {
			t.Errorf("chave %q ausente na linha de início", chave)
		}
	}
	if ini["eprocia_log"] != "1" {
		t.Errorf("eprocia_log = %v, quer \"1\"", ini["eprocia_log"])
	}
	// t.TempDir() fica no temporário do usuário: a pasta chega saneada.
	pl, _ := ini["pasta_logs"].(string)
	if strings.Contains(strings.ToLower(pl), strings.ToLower(dir)) ||
		!(strings.HasPrefix(pl, "%TEMP%") || strings.HasPrefix(pl, "%USERPROFILE%")) {
		t.Errorf("pasta_logs = %q, quer saneada (começando por %%TEMP%% ou %%USERPROFILE%%)", pl)
	}
	cfg, _ := ini["config"].(map[string]any)
	mp, _ := cfg["monitored_path"].(string)
	if !strings.HasPrefix(mp, "%USERPROFILE%") {
		t.Errorf("config.monitored_path = %q, quer começando por %%USERPROFILE%%", mp)
	}
	for _, l := range linhas {
		if l["nivel"] == "DEBUG" {
			t.Errorf("linha DEBUG com EPROCIA_LOG=1: %v", l)
		}
	}
}

func TestIniciarRegistro_ValorDesconhecido(t *testing.T) {
	dir := t.TempDir() // antes de isolarRegistro: o cleanup do registro fecha o log antes de apagar a pasta
	isolarRegistro(t)

	if !iniciarRegistro("xyz", dir, cfgDeTeste(t)) {
		t.Fatal("iniciarRegistro(\"xyz\") devolveu false")
	}
	registro.Encerrar()

	aviso := acharMsg(linhasLog(t, dir), "valor de EPROCIA_LOG não reconhecido")
	if aviso == nil || aviso["nivel"] != "WARN" {
		t.Fatalf("sem o WARN de valor não reconhecido: %v", aviso)
	}
}

// TestIniciarRegistro_LidoSoNaPartida documenta o contrato: o registro não
// relê a variável depois de iniciado. A leitura única em si é garantida por
// main.go, que chama os.Getenv("EPROCIA_LOG") uma vez, na partida.
func TestIniciarRegistro_LidoSoNaPartida(t *testing.T) {
	dir := t.TempDir() // antes de isolarRegistro: o cleanup do registro fecha o log antes de apagar a pasta
	isolarRegistro(t)

	if !iniciarRegistro("1", dir, cfgDeTeste(t)) {
		t.Fatal("iniciarRegistro(\"1\") devolveu false")
	}
	t.Setenv("EPROCIA_LOG", "detalhado")
	if registro.Detalhado() {
		t.Error("registro.Detalhado() mudou com a variável trocada depois da partida")
	}
}

func TestPrepararInstancia(t *testing.T) {
	liberado := 0
	casos := []struct {
		nome        string
		ok          bool
		err         error
		querSair    bool
		querChamada int
	}{
		{"outra instância", false, nil, true, 0},
		{"trava obtida", true, nil, false, 1},
		{"trava indisponível", false, errors.New("falha sintética"), false, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chamadas := 0
			travar := func() (func(), bool, error) {
				if c.ok {
					return func() { liberado++ }, true, nil
				}
				return nil, false, c.err
			}
			sair, liberar := prepararInstancia(travar, func() { chamadas++ })
			if sair != c.querSair {
				t.Errorf("sair = %v, quer %v", sair, c.querSair)
			}
			if chamadas != c.querChamada {
				t.Errorf("aoTravar chamado %d vezes, quer %d", chamadas, c.querChamada)
			}
			if liberar == nil {
				t.Fatal("liberar nil")
			}
			liberar()
		})
	}
	if liberado != 1 {
		t.Errorf("liberar da trava chamado %d vezes, quer 1", liberado)
	}
}

// pdfSintetico monta um PDF de texto nativo de uma página.
func pdfSintetico(texto string) []byte {
	stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 712 Td\n(%s) Tj\nET", texto)
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << >> /Contents 4 0 R >>\nendobj",
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj", len(stream), stream),
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = buf.Len()
		buf.WriteString(o + "\n")
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objs)+1, xref)
	return buf.Bytes()
}

// zipSintetico grava, numa pasta própria, um ZIP com um PDF de nome falso.
func zipSintetico(t *testing.T, nomeZip, nomeDoc, texto string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(nomeDoc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(pdfSintetico(texto)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	caminho := filepath.Join(t.TempDir(), nomeZip)
	if err := os.WriteFile(caminho, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func esperarSucesso(t *testing.T, q *etl.Queue) *etl.QueueItem {
	t.Helper()
	limite := time.Now().Add(30 * time.Second)
	for time.Now().Before(limite) {
		for _, it := range q.GetItems() {
			switch it.Status {
			case "success":
				return it
			case "error", "cancelled":
				t.Fatalf("item terminou %s: %s", it.Status, it.Error)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("o ZIP não terminou")
	return nil
}

func TestIniciarRegistro_SemVariavelSemTelemetria(t *testing.T) {
	const (
		nomeZip = "5000000-00.2026.8.26.0001.zip"
		nomeDoc = "Evento 1 - PET1.pdf"
		texto   = "TEXTO SIGILOSO DE FULANO DE TAL"
	)
	for _, valor := range []string{"", "1"} {
		t.Run("EPROCIA_LOG="+valor, func(t *testing.T) {
			dir := t.TempDir() // antes de isolarRegistro: o cleanup do registro fecha o log antes de apagar a pasta
			isolarRegistro(t)
			zipPath := zipSintetico(t, nomeZip, nomeDoc, texto)

			cfg := cfgDeTeste(t)
			cfg.OcrMode, cfg.CleaningLevel = "none", "standard"
			iniciarRegistro(valor, dir, cfg)

			// O mesmo caminho do App.startup: fila criada depois do registro e
			// a mesma ligação condicional da telemetria (ligarTelemetriaSeAtivo).
			q := etl.NewQueue(etl.NewPipeline())
			q.SetConfig(cfg)
			app := &App{queue: q}
			app.ligarTelemetriaSeAtivo(dir)
			if (app.fecharTelemetria != nil) != (valor != "") {
				t.Errorf("fecharTelemetria definido = %v com EPROCIA_LOG=%q", app.fecharTelemetria != nil, valor)
			}
			q.AddFilesComOrigem([]string{zipPath}, "manual")
			esperarSucesso(t, q)
			q.Shutdown() // espera o fim do item, aviso de fim de ZIP incluído
			if app.fecharTelemetria != nil {
				app.fecharTelemetria()
			}
			registro.Encerrar()

			if valor == "" {
				for _, sub := range []string{"telemetria", "logs"} {
					if _, err := os.Stat(filepath.Join(dir, sub)); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("%s existe com EPROCIA_LOG vazia (err = %v)", sub, err)
					}
				}
				return
			}

			tel, err := os.ReadFile(filepath.Join(dir, "telemetria", "telemetria.jsonl"))
			if err != nil {
				t.Fatalf("telemetria não criada: %v", err)
			}
			linhas := strings.Split(strings.TrimSpace(string(tel)), "\n")
			if len(linhas) != 1 {
				t.Fatalf("telemetria com %d linhas, quer 1:\n%s", len(linhas), tel)
			}
			var reg map[string]any
			if err := json.Unmarshal([]byte(linhas[0]), &reg); err != nil {
				t.Fatal(err)
			}
			for _, nome := range []string{"5000000", "PET1", "SIGILOSO", "fulano"} {
				if strings.Contains(strings.ToLower(linhas[0]), strings.ToLower(nome)) {
					t.Errorf("telemetria contém %q: %s", nome, linhas[0])
				}
			}

			concluido := acharMsg(linhasLog(t, dir), "zip concluído")
			if concluido == nil {
				t.Fatal("log sem \"zip concluído\"")
			}
			if concluido["item_id"] == nil || concluido["item_id"] != reg["item_id"] {
				t.Errorf("item_id: log %v, telemetria %v", concluido["item_id"], reg["item_id"])
			}
			bruto, err := os.ReadFile(filepath.Join(dir, "logs", "eprocia.log"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(bruto), nomeZip) || !strings.Contains(string(bruto), nomeDoc) {
				t.Errorf("log sem o nome do ZIP ou do documento")
			}
			if strings.Contains(string(bruto), "SIGILOSO") {
				t.Errorf("log contém o texto do PDF")
			}
		})
	}
}
