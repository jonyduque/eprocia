package registro

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"eprocia/internal/models"
)

// relogio é um relógio de teste: só anda quando o teste manda.
type relogio struct{ ns atomic.Int64 }

func novoRelogio() *relogio {
	r := &relogio{}
	r.ns.Store(time.Date(2026, 11, 24, 10, 0, 0, 0, time.Local).UnixNano())
	return r
}

func (r *relogio) agora() time.Time { return time.Unix(0, r.ns.Load()) }
func (r *relogio) em(base time.Time) func(time.Duration) {
	return func(d time.Duration) { r.ns.Store(base.Add(d).UnixNano()) }
}

// bufSeguro é um io.Writer que o teste pode ler enquanto a gravadora escreve.
type bufSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *bufSeguro) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *bufSeguro) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func saneadorTeste() *Saneador {
	return NovoSaneador(`C:\Users\fulano`, `C:\Users\fulano\AppData\Local\Temp`)
}

// gravadorTeste cria uma gravadora sobre destino e a encerra no fim do teste.
func gravadorTeste(t *testing.T, destino interface {
	Write([]byte) (int, error)
}, agora func() time.Time) *gravador {
	t.Helper()
	g := novoGravador(configGravador{destino: destino, agora: agora, saneador: saneadorTeste()})
	t.Cleanup(g.encerrar)
	return g
}

func linhasJSON(t *testing.T, texto string) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(strings.NewReader(texto))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("linha não é JSON válido: %v: %q", err, sc.Text())
		}
		out = append(out, m)
	}
	return out
}

func TestManipulador_LinhaJSONComCamposFixos(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	slog.New(g.manipulador(slog.LevelInfo)).With("item_id", "u1").Info("zip concluído", "doc_id", 3)
	g.encerrar()

	ls := linhasJSON(t, saida.String())
	if len(ls) != 1 {
		t.Fatalf("linhas = %d, quer 1: %q", len(ls), saida.String())
	}
	l := ls[0]
	ts, _ := l["ts"].(string)
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("ts %q não é RFC 3339: %v", ts, err)
	}
	if !strings.ContainsAny(ts[19:], "Z+-") {
		t.Errorf("ts %q sem fuso", ts)
	}
	if l["nivel"] != "INFO" {
		t.Errorf("nivel = %v", l["nivel"])
	}
	if l["msg"] != "zip concluído" {
		t.Errorf("msg = %v", l["msg"])
	}
	if l["item_id"] != "u1" {
		t.Errorf("item_id = %v", l["item_id"])
	}
	if l["doc_id"] != float64(3) {
		t.Errorf("doc_id = %v", l["doc_id"])
	}
	for _, k := range []string{"time", "level"} {
		if _, ok := l[k]; ok {
			t.Errorf("chave %q não deveria existir", k)
		}
	}
}

func TestManipulador_Saneia(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	log := slog.New(g.manipulador(slog.LevelDebug))
	errPDF := fmt.Errorf("abrir: %w", &fs.PathError{Op: "open", Path: `C:\Users\fulano\AppData\Local\Temp\a.pdf`, Err: errors.New("acesso negado")})
	log.Info(`falha em C:\Users\fulano\Processos\x.zip`,
		"caminho", `C:\Users\fulano\Documents\y.pdf`,
		"erro", errPDF,
		slog.Group("doc", "origem", `C:\Users\fulano\Downloads\z.zip`))
	log.WithGroup("g").With("pasta", `C:\Users\fulano\Desktop`).Warn("aviso", "e", errPDF)
	g.encerrar()

	texto := saida.String()
	if len(linhasJSON(t, texto)) != 2 {
		t.Fatalf("esperava 2 linhas: %q", texto)
	}
	if strings.Contains(strings.ToLower(texto), "fulano") {
		t.Errorf("o log contém o nome do usuário: %s", texto)
	}
}

func TestManipulador_SaneiaAnyStruct(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	log := slog.New(g.manipulador(slog.LevelInfo))
	log.Info("config", slog.Any("config", models.Config{MonitoredPath: `C:\Users\fulano\Processos`}))
	log.Info("mapa", slog.Any("m", map[string]string{"p": `C:\Users\fulano\x`}))
	g.encerrar()

	texto := saida.String()
	if len(linhasJSON(t, texto)) != 2 {
		t.Fatalf("esperava 2 linhas: %q", texto)
	}
	if strings.Contains(strings.ToLower(texto), "fulano") {
		t.Errorf("o log contém o nome do usuário: %s", texto)
	}
}

// writerPreso avisa no primeiro Write e trava até liberar ser fechado.
type writerPreso struct {
	entrou  chan struct{}
	liberar chan struct{}
	uma     sync.Once
}

func (w *writerPreso) Write(p []byte) (int, error) {
	w.uma.Do(func() { close(w.entrou) })
	<-w.liberar
	return len(p), nil
}

func TestGravador_NaoBloqueia(t *testing.T) {
	w := &writerPreso{entrou: make(chan struct{}), liberar: make(chan struct{})}
	g := novoGravador(configGravador{destino: w, agora: time.Now, saneador: saneadorTeste(), intervalo: 10 * time.Millisecond})
	t.Cleanup(func() {
		close(w.liberar)
		g.encerrar()
	})
	log := slog.New(g.manipulador(slog.LevelDebug))

	log.Info("primeira")
	select {
	case <-w.entrou:
	case <-time.After(3 * time.Second):
		t.Fatal("a gravadora não chegou a escrever")
	}
	// A gravadora está presa no Write: enche o canal até a primeira recusa.
	for g.tentarEnviar([]byte("{}\n")) {
	}

	antes := g.perdidas.Load()
	medir := func(nome string, f func(), max time.Duration) {
		t.Helper()
		ini := time.Now()
		f()
		if d := time.Since(ini); d > max {
			t.Errorf("%s levou %v, máximo %v", nome, d, max)
		}
	}
	medir("Info", func() { log.Info("cheio") }, 5*time.Millisecond)
	medir("Warn", func() { log.Warn("cheio") }, 200*time.Millisecond)
	medir("Error", func() { log.Error("cheio") }, 200*time.Millisecond)
	if d := g.perdidas.Load() - antes; d != 3 {
		t.Errorf("perdidas subiu %d, quer 3", d)
	}
}

// writerFalho devolve erro até aceitar ser ligado.
type writerFalho struct {
	aceitar atomic.Bool
	buf     bufSeguro
}

func (w *writerFalho) Write(p []byte) (int, error) {
	if !w.aceitar.Load() {
		return 0, errors.New("disco cheio")
	}
	return w.buf.Write(p)
}

func TestGravador_FalhaDeEscritaPausa60s(t *testing.T) {
	r := novoRelogio()
	base := r.agora()
	irPara := r.em(base)
	w := &writerFalho{}
	g := gravadorTeste(t, w, r.agora)
	log := slog.New(g.manipulador(slog.LevelDebug))

	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("pânico ao registrar: %v", p)
			}
		}()
		log.Info("a")
		log.Error("b")
		g.descarregar() // a escrita falha aqui
		log.Info("c")
		g.descarregar()
	}()

	w.aceitar.Store(true)
	irPara(59 * time.Second)
	log.Info("d")
	g.descarregar()
	if s := w.buf.String(); s != "" {
		t.Fatalf("aos 59 s nada deveria ser gravado: %q", s)
	}

	irPara(61 * time.Second)
	log.Info("e")
	g.descarregar()
	ls := linhasJSON(t, w.buf.String())
	if len(ls) < 2 {
		t.Fatalf("aos 61 s a gravação deveria voltar: %q", w.buf.String())
	}
	if ls[0]["msg"] != "linhas de log perdidas" || ls[0]["nivel"] != "WARN" {
		t.Errorf("primeira linha = %v, quer o WARN de linhas perdidas", ls[0])
	}
	if n, _ := ls[0]["linhas_perdidas"].(float64); n <= 0 {
		t.Errorf("linhas_perdidas = %v, quer > 0", ls[0]["linhas_perdidas"])
	}
	if ls[len(ls)-1]["msg"] != "e" {
		t.Errorf("última linha = %v, quer e", ls[len(ls)-1])
	}
}

func TestGravador_DescargaPeriodicaENoEncerrar(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	log := slog.New(g.manipulador(slog.LevelInfo))
	log.Info("periodica")
	prazo := time.Now().Add(3 * time.Second)
	for !strings.Contains(saida.String(), "periodica") {
		if time.Now().After(prazo) {
			t.Fatal("a linha não foi descarregada em 3 s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	log.Info("no encerrar")
	g.encerrar()
	if !strings.Contains(saida.String(), "no encerrar") {
		t.Error("Encerrar não descarregou o resto")
	}
	g.encerrar()
}

func TestGravador_LogDepoisDeEncerrar(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	log := slog.New(g.manipulador(slog.LevelInfo))
	g.encerrar()
	log.Info("depois")
	log.Error("depois")
	if strings.Contains(saida.String(), "depois") {
		t.Error("linha depois de Encerrar foi gravada")
	}

	saida2 := &bufSeguro{}
	g2 := gravadorTeste(t, saida2, time.Now)
	log2 := slog.New(g2.manipulador(slog.LevelInfo))
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			for j := range 100 {
				log2.Info("concorrente", "g", i, "j", j)
				if j%10 == 0 {
					log2.Error("concorrente", "g", i)
				}
			}
		})
	}
	wg.Go(g2.encerrar)
	wg.Wait()
	linhasJSON(t, saida2.String())
}

func TestGravador_Concorrencia(t *testing.T) {
	saida := &bufSeguro{}
	g := gravadorTeste(t, saida, time.Now)
	log := slog.New(g.manipulador(slog.LevelInfo))
	const goroutines, porGoroutine = 50, 200
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			for j := range porGoroutine {
				log.Info("x", "g", i, "j", j)
			}
		})
	}
	wg.Wait()
	g.encerrar()

	gravadas, perdidas := 0, 0
	for _, l := range linhasJSON(t, saida.String()) {
		switch l["msg"] {
		case "x":
			gravadas++
		case "linhas de log perdidas":
			n, _ := l["linhas_perdidas"].(float64)
			perdidas += int(n)
		}
	}
	perdidas += int(g.perdidas.Load())
	if gravadas+perdidas != goroutines*porGoroutine {
		t.Errorf("gravadas %d + perdidas %d = %d, quer %d", gravadas, perdidas, gravadas+perdidas, goroutines*porGoroutine)
	}
}
