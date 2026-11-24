package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
