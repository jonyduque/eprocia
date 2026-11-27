package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"eprocia/internal/atualizacao"
	"eprocia/internal/models"
	"eprocia/internal/watchdog"
)

// redeProibida falha o teste se o App fizer qualquer consulta ou download.
type redeProibida struct{ t *testing.T }

func (r redeProibida) Verificar(context.Context, atualizacao.Versao) (*atualizacao.Release, error) {
	r.t.Error("a Rede foi chamada (Verificar) sem clique")
	return nil, errors.New("proibido")
}

func (r redeProibida) Baixar(context.Context, *atualizacao.Release, atualizacao.Asset, string, string, func(int64, int64)) (atualizacao.Baixado, error) {
	r.t.Error("a Rede foi chamada (Baixar) sem clique")
	return atualizacao.Baixado{}, errors.New("proibido")
}

// falsosAtualizacao controla e registra esperarSair, travarDepois,
// gravarVersao, autostart e emitir.
type falsosAtualizacao struct {
	mu          sync.Mutex
	esperas     int
	travas      int
	versoes     []string
	autostart   []bool
	emitidos    []string
	liberaSair  chan struct{} // close libera todas as chamadas de esperarSair
	liberaTrava chan struct{} // close libera travarDepois
}

func (f *falsosAtualizacao) contar(p *int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *p
}

func (f *falsosAtualizacao) gravadas() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.versoes...)
}

func (f *falsosAtualizacao) eventos() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.emitidos...)
}

type cenario struct {
	apos           int
	limparAnterior bool
	oculta         bool
	modo           atualizacao.Modo
	rede           atualizacao.Rede
	// sairLivre e travaLivre: os falsos devolvem na hora.
	sairLivre, travaLivre bool
}

type montagem struct {
	app             *App
	f               *falsosAtualizacao
	monitorada      string
	pastaInstalacao string
	opcoes          atualizacao.Opcoes
}

// novoAppAtualizacao monta o App sem Wails, sem registro do Windows e sem
// %APPDATA%: tudo em t.TempDir() e todos os falsos preparados.
func novoAppAtualizacao(t *testing.T, c cenario) *montagem {
	t.Helper()
	f := &falsosAtualizacao{liberaSair: make(chan struct{}), liberaTrava: make(chan struct{})}
	if c.sairLivre {
		close(f.liberaSair)
	}
	if c.travaLivre {
		close(f.liberaTrava)
	}
	m := &montagem{f: f, monitorada: t.TempDir(), pastaInstalacao: t.TempDir()}
	app := NewApp()
	app.autostart = func(ativo bool) error {
		f.mu.Lock()
		f.autostart = append(f.autostart, ativo)
		f.mu.Unlock()
		return nil
	}
	app.emitir = func(evento string, _ ...any) {
		f.mu.Lock()
		f.emitidos = append(f.emitidos, evento)
		f.mu.Unlock()
	}
	app.guardarConfig(&models.Config{MonitoredPath: m.monitorada, WatchdogEnabled: true, AutoStart: true, OcrMode: "none", CleaningLevel: "basic"})
	app.falhas = watchdog.CarregarFalhas(filepath.Join(t.TempDir(), "falhas.json"))
	app.pastaDados = t.TempDir()
	app.apos = c.apos
	app.limparAnterior = c.limparAnterior
	app.iniciarOculta(c.oculta)
	app.esperarSair = func(int, time.Duration) bool {
		f.mu.Lock()
		f.esperas++
		f.mu.Unlock()
		<-f.liberaSair
		return true
	}
	app.travarDepois = func() bool {
		f.mu.Lock()
		f.travas++
		f.mu.Unlock()
		<-f.liberaTrava
		return true
	}
	app.gravarVersao = func(v string) error {
		f.mu.Lock()
		f.versoes = append(f.versoes, v)
		f.mu.Unlock()
		return nil
	}
	m.app = app

	modo := c.modo
	if modo == "" {
		modo = atualizacao.ModoTrocar
	}
	rede := c.rede
	if rede == nil {
		rede = redeProibida{t}
	}
	atual, err := atualizacao.ParseVersao(versaoApp())
	if err != nil {
		t.Fatal(err)
	}
	m.opcoes = atualizacao.Opcoes{
		Atual:            atual,
		Instalacao:       atualizacao.Instalacao{Modo: modo, Pasta: m.pastaInstalacao},
		PastaDados:       app.pastaDados,
		PastaAtualizacao: t.TempDir(),
		PID:              os.Getpid(),
		Rede:             rede,
		Shell:            func(string) error { t.Error("shell chamado"); return nil },
	}
	t.Cleanup(func() {
		// Solta o que ainda espera, para nenhuma goroutine sobreviver ao teste.
		fecharSeAberto(f.liberaSair)
		fecharSeAberto(f.liberaTrava)
		if app.atualizador != nil {
			app.atualizador.Encerrar()
		}
		app.watcher.Stop()
		if app.queue != nil {
			app.queue.Shutdown()
		}
		app.watcher.Stop()
	})
	return m
}

func fecharSeAberto(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

// iniciar faz o que o startup faz depois do contexto do Wails.
func (m *montagem) iniciar() {
	m.app.iniciarFila(nil)
	m.app.iniciarAtualizacao(m.opcoes)
}

func esperarAte(t *testing.T, prazo time.Duration, o string, cond func() bool) {
	t.Helper()
	fim := time.Now().Add(prazo)
	for !cond() {
		if time.Now().After(fim) {
			t.Fatalf("esperou %v: %s", prazo, o)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// continua confere que cond segue verdadeira durante todo o prazo.
func continua(t *testing.T, prazo time.Duration, o string, cond func() bool) {
	t.Helper()
	fim := time.Now().Add(prazo)
	for time.Now().Before(fim) {
		if !cond() {
			t.Fatalf("deixou de valer antes de %v: %s", prazo, o)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func existe(caminho string) bool {
	_, err := os.Stat(caminho)
	return err == nil
}

func criarArquivo(t *testing.T, caminho string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func statusDe(app *App, id string) string {
	for _, it := range app.GetQueue() {
		if it.ID == id {
			return it.Status
		}
	}
	return ""
}

func TestAtualizacao_IniciarFilaSemWailsNemRegistro(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	m.app.iniciarFila(nil)
	if m.app.queue == nil || m.app.pipeline == nil {
		t.Fatal("iniciarFila não criou a fila e o pipeline")
	}
	if got := m.app.watcher.GetCurrentPath(); got != m.monitorada {
		t.Fatalf("watcher em %q; quer %q", got, m.monitorada)
	}
	m.f.mu.Lock()
	chamadas := append([]bool(nil), m.f.autostart...)
	m.f.mu.Unlock()
	if len(chamadas) != 1 || chamadas[0] != true {
		t.Fatalf("autostart chamado com %v; quer [true]", chamadas)
	}
}

func TestAtualizacao_SemRedeSemClique(t *testing.T) {
	for _, apos := range []int{0, 4242} {
		t.Run(map[int]string{0: "sem_apos", 4242: "com_apos"}[apos], func(t *testing.T) {
			m := novoAppAtualizacao(t, cenario{apos: apos, limparAnterior: apos > 0, sairLivre: true, travaLivre: true})
			m.iniciar()
			m.app.domPronto(nil)
			m.app.darSinal()
			m.app.arranqueApos()
			// Dá tempo às goroutines de rodar; a Rede falha o teste se chamada.
			time.Sleep(300 * time.Millisecond)
		})
	}
}

func TestAtualizacao_ProcessoNovoRetido(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{apos: 4242})
	m.iniciar()
	if got := m.app.watcher.GetCurrentPath(); got != "" {
		t.Fatalf("watcher ligado em %q com a fila retida", got)
	}
	m.app.domPronto(nil)
	inexistente := filepath.Join(t.TempDir(), "nao-existe.zip")
	if n, err := m.app.AddFilesToQueue([]string{inexistente}); err != nil || n != 1 {
		t.Fatalf("AddFilesToQueue = %d, %v; quer 1, nil", n, err)
	}
	itens := m.app.GetQueue()
	if len(itens) != 1 {
		t.Fatalf("fila com %d itens", len(itens))
	}
	id := itens[0].ID
	continua(t, 300*time.Millisecond, "item pending e watcher desligado antes de o antigo sair", func() bool {
		return statusDe(m.app, id) == "pending" && m.app.watcher.GetCurrentPath() == ""
	})
	sinal := atualizacao.CaminhoSinal(m.app.pastaDados, 4242)
	dados, err := os.ReadFile(sinal)
	if err != nil {
		t.Fatalf("sinal não gravado: %v", err)
	}
	if !strings.Contains(string(dados), versaoApp()) {
		t.Fatalf("sinal = %q; quer a versão %q", dados, versaoApp())
	}

	close(m.f.liberaSair)
	esperarAte(t, 5*time.Second, "item sair de pending", func() bool { return statusDe(m.app, id) != "pending" })
	continua(t, 300*time.Millisecond, "watcher desligado sem a trava", func() bool {
		return m.app.watcher.GetCurrentPath() == ""
	})
	close(m.f.liberaTrava)
	esperarAte(t, 5*time.Second, "watcher ligar", func() bool { return m.app.watcher.GetCurrentPath() == m.monitorada })
}

func TestAtualizacao_SinalNoStartupComTray(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{apos: 4242, oculta: true, sairLivre: true, travaLivre: true})
	m.iniciar()
	m.app.sinalNoStartup()
	sinal := atualizacao.CaminhoSinal(m.app.pastaDados, 4242)
	if !existe(sinal) {
		t.Fatal("sem sinal logo depois do startup com --tray")
	}
	esperarAte(t, 5*time.Second, "arranqueApos", func() bool { return m.f.contar(&m.f.travas) == 1 })
	if err := os.Remove(sinal); err != nil {
		t.Fatal(err)
	}
	m.app.domPronto(nil)
	time.Sleep(200 * time.Millisecond)
	if existe(sinal) {
		t.Fatal("domPronto gravou outro sinal")
	}
	if n := m.f.contar(&m.f.esperas); n != 1 {
		t.Fatalf("esperarSair chamado %d vezes; quer 1 (um arranqueApos só)", n)
	}
}

func TestAtualizacao_DomProntoUmaVez(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{apos: 4242, sairLivre: true, travaLivre: true})
	m.iniciar()
	m.app.sinalNoStartup() // sem --tray: não grava nada
	sinal := atualizacao.CaminhoSinal(m.app.pastaDados, 4242)
	if existe(sinal) {
		t.Fatal("sinal gravado no startup sem --tray")
	}
	m.app.domPronto(nil)
	if !existe(sinal) {
		t.Fatal("domPronto não gravou o sinal")
	}
	esperarAte(t, 5*time.Second, "arranqueApos", func() bool { return m.f.contar(&m.f.travas) == 1 })
	if err := os.Remove(sinal); err != nil {
		t.Fatal(err)
	}
	m.app.domPronto(nil)
	time.Sleep(200 * time.Millisecond)
	if existe(sinal) {
		t.Fatal("a segunda chamada de domPronto gravou outro sinal")
	}
	if n := m.f.contar(&m.f.esperas); n != 1 {
		t.Fatalf("esperarSair chamado %d vezes; quer 1", n)
	}
}

func TestAtualizacao_GravarVersaoSoTrocarEDepoisDoAntigo(t *testing.T) {
	t.Run("trocar", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{apos: 4242, limparAnterior: true, modo: atualizacao.ModoTrocar, travaLivre: true})
		m.iniciar()
		m.app.domPronto(nil)
		esperarAte(t, 5*time.Second, "as duas esperas", func() bool { return m.f.contar(&m.f.esperas) == 2 })
		time.Sleep(200 * time.Millisecond)
		if v := m.f.gravadas(); len(v) != 0 {
			t.Fatalf("gravarVersao antes de o antigo sair: %v", v)
		}
		close(m.f.liberaSair)
		esperarAte(t, 5*time.Second, "gravarVersao", func() bool { return len(m.f.gravadas()) == 1 })
		time.Sleep(200 * time.Millisecond)
		if v := m.f.gravadas(); len(v) != 1 || v[0] != versaoApp() {
			t.Fatalf("gravarVersao = %v; quer [%s]", v, versaoApp())
		}
	})
	for _, modo := range []atualizacao.Modo{atualizacao.ModoInstalador, atualizacao.ModoSoAvisar} {
		t.Run(string(modo), func(t *testing.T) {
			m := novoAppAtualizacao(t, cenario{apos: 4242, limparAnterior: true, modo: modo, sairLivre: true, travaLivre: true})
			m.iniciar()
			m.app.domPronto(nil)
			esperarAte(t, 5*time.Second, "watcher ligar", func() bool { return m.app.watcher.GetCurrentPath() == m.monitorada })
			time.Sleep(200 * time.Millisecond)
			if v := m.f.gravadas(); len(v) != 0 {
				t.Fatalf("gravarVersao no modo %s: %v", modo, v)
			}
		})
	}
}

func TestAtualizacao_LimpaAnteriorDepoisDoAntigoSair(t *testing.T) {
	t.Run("com_limpar_anterior", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{apos: 4242, limparAnterior: true, travaLivre: true})
		exe := filepath.Join(m.pastaInstalacao, atualizacao.NomeExe)
		anterior := filepath.Join(m.pastaInstalacao, atualizacao.NomeAnterior)
		criarArquivo(t, exe)
		criarArquivo(t, anterior)
		m.iniciar()

		feito := make(chan struct{})
		go func() { m.app.domPronto(nil); close(feito) }()
		select {
		case <-feito:
		case <-time.After(2 * time.Second):
			t.Fatal("domPronto esperou a limpeza ou o antigo sair")
		}
		if !existe(atualizacao.CaminhoSinal(m.app.pastaDados, 4242)) {
			t.Fatal("sinal não gravado antes de o antigo sair")
		}
		esperarAte(t, 5*time.Second, "as duas esperas", func() bool { return m.f.contar(&m.f.esperas) == 2 })
		continua(t, 300*time.Millisecond, "anterior presente antes de o antigo sair", func() bool { return existe(anterior) })
		close(m.f.liberaSair)
		esperarAte(t, 5*time.Second, "anterior apagado", func() bool { return !existe(anterior) })
		if !existe(exe) {
			t.Fatal("eprocia.exe apagado")
		}
	})
	t.Run("sem_limpar_anterior", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{apos: 4242, travaLivre: true})
		anterior := filepath.Join(m.pastaInstalacao, atualizacao.NomeAnterior)
		criarArquivo(t, anterior)
		m.iniciar()
		m.app.domPronto(nil)
		esperarAte(t, 5*time.Second, "a espera", func() bool { return m.f.contar(&m.f.esperas) == 1 })
		close(m.f.liberaSair)
		esperarAte(t, 5*time.Second, "watcher ligar", func() bool { return m.app.watcher.GetCurrentPath() == m.monitorada })
		time.Sleep(300 * time.Millisecond)
		if !existe(anterior) {
			t.Fatal("anterior apagado sem --limpar-anterior")
		}
		if n := m.f.contar(&m.f.esperas); n != 1 {
			t.Fatalf("esperarSair chamado %d vezes; quer 1", n)
		}
	})
}

func TestAtualizacao_AberturaNormalApagaAnterior(t *testing.T) {
	disparos := map[string]func(*App){
		"dom_pronto": func(a *App) { a.domPronto(nil) },
		"startup_tray": func(a *App) {
			a.iniciarOculta(true)
			a.sinalNoStartup()
		},
	}
	for nome, disparar := range disparos {
		t.Run(nome, func(t *testing.T) {
			m := novoAppAtualizacao(t, cenario{})
			anterior := filepath.Join(m.pastaInstalacao, atualizacao.NomeAnterior)
			criarArquivo(t, anterior)
			m.iniciar()
			disparar(m.app)
			esperarAte(t, 5*time.Second, "anterior apagado", func() bool { return !existe(anterior) })
			esperarAte(t, 5*time.Second, "gravarVersao", func() bool { return len(m.f.gravadas()) == 1 })

			criarArquivo(t, anterior)
			m.app.domPronto(nil)
			time.Sleep(300 * time.Millisecond)
			if !existe(anterior) {
				t.Fatal("a segunda chamada iniciou outra limpeza")
			}
			if v := m.f.gravadas(); len(v) != 1 {
				t.Fatalf("gravarVersao = %v; quer uma chamada", v)
			}
			if n := m.f.contar(&m.f.esperas); n != 0 {
				t.Fatalf("esperarSair chamado %d vezes na abertura normal", n)
			}
		})
	}
	t.Run("sem_arquivo", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{})
		m.iniciar()
		m.app.domPronto(nil)
		time.Sleep(300 * time.Millisecond)
		if v := m.f.gravadas(); len(v) != 0 {
			t.Fatalf("gravarVersao sem eprocia.anterior.exe: %v", v)
		}
	})
	for _, modo := range []atualizacao.Modo{atualizacao.ModoInstalador, atualizacao.ModoSoAvisar} {
		t.Run(string(modo), func(t *testing.T) {
			m := novoAppAtualizacao(t, cenario{modo: modo})
			anterior := filepath.Join(m.pastaInstalacao, atualizacao.NomeAnterior)
			criarArquivo(t, anterior)
			m.iniciar()
			m.app.domPronto(nil)
			time.Sleep(300 * time.Millisecond)
			if !existe(anterior) {
				t.Fatalf("modo %s apagou o anterior", modo)
			}
			if v := m.f.gravadas(); len(v) != 0 {
				t.Fatalf("gravarVersao no modo %s: %v", modo, v)
			}
		})
	}
}

// bufferSeguro guarda o log de várias goroutines.
type bufferSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *bufferSeguro) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *bufferSeguro) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestAtualizacao_FalhaAoApagarAnteriorSoLoga(t *testing.T) {
	antes, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antes)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	buf := &bufferSeguro{}
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))

	m := novoAppAtualizacao(t, cenario{})
	criarArquivo(t, filepath.Join(m.pastaInstalacao, atualizacao.NomeAnterior))
	m.app.apagarAnterior = func(string) error {
		return &fs.PathError{Op: "remove", Path: `C:\Users\fulano\AppData\Local\Programs\eprocIA\eprocia.anterior.exe`, Err: errors.New("acesso negado")}
	}
	m.iniciar()
	m.app.domPronto(nil)
	const linha = "limpeza da versão anterior: o eprocia.anterior.exe ficou"
	esperarAte(t, 5*time.Second, "linha do log", func() bool { return strings.Contains(buf.String(), linha) })
	if strings.Contains(buf.String(), "fulano") {
		t.Fatalf("o log tem o caminho do perfil:\n%s", buf.String())
	}
	if ev := m.f.eventos(); len(ev) != 0 {
		t.Fatalf("emitido para o frontend: %v", ev)
	}
}

func TestTravaAposAtualizar(t *testing.T) {
	t.Run("ocupada", func(t *testing.T) {
		var mu sync.Mutex
		livre, tentativas := false, 0
		travar := func() (func(), bool, error) {
			mu.Lock()
			defer mu.Unlock()
			tentativas++
			if !livre {
				return nil, false, nil
			}
			return func() {}, true, nil
		}
		envolvida, depois := travaAposAtualizar(travar)
		liberar, ok, err := envolvida()
		if !ok || err != nil || liberar == nil {
			t.Fatalf("envolvida = (%v, %v, %v); quer ok com a trava ocupada", liberar != nil, ok, err)
		}
		pronto := make(chan bool, 1)
		go func() { pronto <- depois() }()
		select {
		case <-pronto:
			t.Fatal("depois devolveu com a trava ocupada")
		case <-time.After(1500 * time.Millisecond):
		}
		mu.Lock()
		livre = true
		mu.Unlock()
		select {
		case v := <-pronto:
			if !v {
				t.Fatal("depois devolveu false")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("depois não devolveu com a trava livre")
		}
	})
	t.Run("livre", func(t *testing.T) {
		tentativas, liberou := 0, false
		travar := func() (func(), bool, error) {
			tentativas++
			return func() { liberou = true }, true, nil
		}
		envolvida, depois := travaAposAtualizar(travar)
		liberar, ok, err := envolvida()
		if !ok || err != nil {
			t.Fatalf("envolvida = (%v, %v); quer a trava real", ok, err)
		}
		liberar()
		if !liberou {
			t.Fatal("a envolvida não devolveu o liberar da trava real")
		}
		inicio := time.Now()
		if !depois() {
			t.Fatal("depois devolveu false")
		}
		if d := time.Since(inicio); d > 100*time.Millisecond {
			t.Fatalf("depois demorou %v com a trava obtida de primeira", d)
		}
		if tentativas != 1 {
			t.Fatalf("travar chamado %d vezes; quer 1", tentativas)
		}
	})
}

// redeDownloadPreso responde com uma versão maior e prende o download até o
// ctx dele ser cancelado.
type redeDownloadPreso struct {
	comecou, cancelado chan struct{}
}

func (r *redeDownloadPreso) Verificar(_ context.Context, atual atualizacao.Versao) (*atualizacao.Release, error) {
	nova := atualizacao.Versao{atual[0], atual[1], atual[2] + 1}
	return &atualizacao.Release{Tag: "v" + nova.String(), Versao: nova, Exe: atualizacao.Asset{Nome: "eprocia.exe", Tamanho: 10}}, nil
}

func (r *redeDownloadPreso) Baixar(ctx context.Context, _ *atualizacao.Release, _ atualizacao.Asset, _, _ string, _ func(int64, int64)) (atualizacao.Baixado, error) {
	close(r.comecou)
	<-ctx.Done()
	close(r.cancelado)
	return atualizacao.Baixado{}, ctx.Err()
}

func TestAtualizacao_ShutdownEncerra(t *testing.T) {
	rede := &redeDownloadPreso{comecou: make(chan struct{}), cancelado: make(chan struct{})}
	m := novoAppAtualizacao(t, cenario{rede: rede})
	m.iniciar()
	if _, err := m.app.atualizador.Verificar(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.app.atualizador.Instalar(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-rede.comecou:
	case <-time.After(5 * time.Second):
		t.Fatal("download não começou")
	}
	m.app.shutdown(nil)
	select {
	case <-rede.cancelado:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown não cancelou o download")
	}
}

// A goroutine do arranqueApos lê a config enquanto o frontend a grava
// (SaveConfig, SetTheme): o -race reprova acesso sem sincronização.
func TestAtualizacao_ConfigSemCorrida(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{apos: 4242, sairLivre: true, travaLivre: true})
	m.iniciar()
	outra := t.TempDir()
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 20 {
			m.app.arranqueApos()
		}
	})
	wg.Go(func() {
		for i := range 20 {
			pasta := m.monitorada
			if i%2 == 1 {
				pasta = outra
			}
			cfg := &models.Config{MonitoredPath: pasta, WatchdogEnabled: true, AutoStart: true, OcrMode: "none", CleaningLevel: "basic"}
			if err := m.app.SaveConfig(cfg); err != nil {
				t.Error(err)
			}
			if err := m.app.SetTheme("dark"); err != nil {
				t.Error(err)
			}
		}
	})
	wg.Wait()
}

// Spec §3.1: no processo novo, salvar a config não liga o watcher antes de
// o antigo sair e da trava; o arranque liga depois, na pasta salva.
func TestAtualizacao_SaveConfigNaoLigaMonitorAntesDaTrava(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{apos: 4242})
	m.iniciar()
	m.app.domPronto(nil)
	nova := t.TempDir()
	cfg := &models.Config{MonitoredPath: nova, WatchdogEnabled: true, AutoStart: true, OcrMode: "none", CleaningLevel: "basic"}
	if err := m.app.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := m.app.watcher.GetCurrentPath(); got != "" {
		t.Fatalf("SaveConfig ligou o watcher em %q antes de o antigo sair", got)
	}
	close(m.f.liberaSair)
	continua(t, 300*time.Millisecond, "watcher desligado sem a trava", func() bool {
		return m.app.watcher.GetCurrentPath() == ""
	})
	if err := m.app.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := m.app.watcher.GetCurrentPath(); got != "" {
		t.Fatalf("SaveConfig ligou o watcher em %q antes da trava", got)
	}
	close(m.f.liberaTrava)
	esperarAte(t, 5*time.Second, "watcher ligar na pasta salva", func() bool { return m.app.watcher.GetCurrentPath() == nova })
}

// A trava adiada desiste depois do prazo (contado a partir de depois, que
// roda depois do esperarSair): sem ela, o watcher não liga.
func TestTravaAposAtualizar_DesisteNoPrazo(t *testing.T) {
	antesPrazo, antesIntervalo := prazoTravaAdiada, intervaloTravaAdiada
	t.Cleanup(func() { prazoTravaAdiada, intervaloTravaAdiada = antesPrazo, antesIntervalo })
	prazoTravaAdiada, intervaloTravaAdiada = 300*time.Millisecond, 20*time.Millisecond

	travar := func() (func(), bool, error) { return nil, false, nil }
	envolvida, depois := travaAposAtualizar(travar)
	if _, ok, err := envolvida(); !ok || err != nil {
		t.Fatalf("envolvida = (%v, %v); quer ok com a trava ocupada", ok, err)
	}
	pronto := make(chan bool, 1)
	inicio := time.Now()
	go func() { pronto <- depois() }()
	select {
	case v := <-pronto:
		if v {
			t.Fatal("depois devolveu true sem obter a trava")
		}
		if d := time.Since(inicio); d < 250*time.Millisecond {
			t.Fatalf("depois desistiu em %v, antes do prazo", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("depois não desistiu no prazo")
	}
}

func TestAtualizacao_SemTravaNoPrazoNaoLigaMonitor(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{apos: 4242, sairLivre: true})
	m.app.travarDepois = func() bool { return false }
	m.iniciar()
	m.app.arranqueApos()
	cfg := &models.Config{MonitoredPath: m.monitorada, WatchdogEnabled: true, AutoStart: true, OcrMode: "none", CleaningLevel: "basic"}
	if err := m.app.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := m.app.watcher.GetCurrentPath(); got != "" {
		t.Fatalf("watcher ligado em %q sem a trava", got)
	}
}

// redeFalsa devolve uma release fixa e conta as chamadas.
type redeFalsa struct {
	mu        sync.Mutex
	rel       *atualizacao.Release
	consultas int
	baixados  int
	caminho   string
}

func (r *redeFalsa) Verificar(context.Context, atualizacao.Versao) (*atualizacao.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consultas++
	return r.rel, nil
}

func (r *redeFalsa) Baixar(_ context.Context, rel *atualizacao.Release, a atualizacao.Asset, _, _ string, _ func(int64, int64)) (atualizacao.Baixado, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baixados++
	return atualizacao.Baixado{Caminho: r.caminho, Asset: a, Versao: rel.Versao}, nil
}

func (r *redeFalsa) chamadas() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.consultas, r.baixados
}

// trocadorFalso conta as criações e os reinícios. antes, se não nil, roda no
// começo do Reiniciar; erro é o que o Reiniciar devolve.
type trocadorFalso struct {
	mu        sync.Mutex
	criados   int
	reiniciou int
	antes     func()
	erro      error
}

func (tf *trocadorFalso) Instalar(context.Context, atualizacao.Baixado) error { return nil }

func (tf *trocadorFalso) Reiniciar(context.Context, atualizacao.Versao) error {
	if tf.antes != nil {
		tf.antes()
	}
	tf.mu.Lock()
	tf.reiniciou++
	tf.mu.Unlock()
	return tf.erro
}

func (tf *trocadorFalso) contar() (int, int) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return tf.criados, tf.reiniciou
}

func releaseMaior(t *testing.T) *atualizacao.Release {
	t.Helper()
	v, err := atualizacao.ParseVersao("99.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return &atualizacao.Release{Tag: "v99.0.0", Versao: v, NotasURL: "https://github.com/jonyduque/eprocia/releases/tag/v99.0.0"}
}

func TestInstalarAtualizacao_SemVerificar(t *testing.T) {
	rede := &redeFalsa{rel: releaseMaior(t)}
	m := novoAppAtualizacao(t, cenario{rede: rede})
	m.iniciar()
	if err := m.app.InstalarAtualizacao(); err == nil {
		t.Fatal("InstalarAtualizacao sem Verificar devolveu nil")
	}
	if c, b := rede.chamadas(); c != 0 || b != 0 {
		t.Fatalf("a Rede foi chamada: consultas=%d baixados=%d", c, b)
	}
}

func TestVerificarAtualizacao_DevolveInfo(t *testing.T) {
	rede := &redeFalsa{rel: releaseMaior(t)}
	m := novoAppAtualizacao(t, cenario{rede: rede})
	m.iniciar()
	info, err := m.app.VerificarAtualizacao()
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Modo != "trocar" {
		t.Fatalf("Info = %+v; quer Modo trocar", info)
	}
	if got := m.app.EstadoAtualizacao().Fase; got != atualizacao.FaseDisponivel {
		t.Fatalf("fase = %q; quer disponivel", got)
	}
	temEvento := false
	for _, e := range m.f.eventos() {
		if e == "atualizacao:estado" {
			temEvento = true
		}
	}
	if !temEvento {
		t.Fatalf("eventos %v sem atualizacao:estado", m.f.eventos())
	}
}

func TestAbrirNovidades_SoPrefixo(t *testing.T) {
	casos := []struct {
		url  string
		abre bool
	}{
		{"https://github.com/jonyduque/eprocia/releases/tag/v0.5.0", true},
		{"https://github.com/outro/x/releases/", false},
		{"https://github.com/jonyduque/eprocia-x/releases/", false},
		{"http://github.com/jonyduque/eprocia/releases/tag/v1", false},
		{"https://github.com/jonyduque/eprocia/releases/../../x", false},
		{"https://github.com/jonyduque/eprocia/releases/tag/../../../x", false},
		{"https://github.com.evil.example/jonyduque/eprocia/releases/tag/v1", false},
		{"https://github.com/jonyduque/eprocia/releases/tag/v1\\..\\x", false},
		{"https://usuario@github.com/jonyduque/eprocia/releases/tag/v1", false},
		{"https://usuario:senha@github.com/jonyduque/eprocia/releases/tag/v1", false},
		{"https://evil.example/", false},
		{"", false},
	}
	for _, c := range casos {
		t.Run(c.url, func(t *testing.T) {
			rel := releaseMaior(t)
			rel.NotasURL = c.url
			m := novoAppAtualizacao(t, cenario{rede: &redeFalsa{rel: rel}})
			m.iniciar()
			var abertas []string
			m.app.abrirURL = func(u string) { abertas = append(abertas, u) }
			if _, err := m.app.VerificarAtualizacao(); err != nil {
				t.Fatal(err)
			}
			err := m.app.AbrirNovidades()
			if c.abre && (err != nil || len(abertas) != 1 || abertas[0] != c.url) {
				t.Fatalf("err=%v abertas=%v; quer abrir %q", err, abertas, c.url)
			}
			if !c.abre && (err == nil || len(abertas) != 0) {
				t.Fatalf("err=%v abertas=%v; quer erro sem abrir", err, abertas)
			}
		})
	}
}

func TestAbrirPaginaVersoes(t *testing.T) {
	app := NewApp()
	var abertas []string
	app.abrirURL = func(u string) { abertas = append(abertas, u) }
	if err := app.AbrirPaginaVersoes(); err != nil {
		t.Fatal(err)
	}
	if len(abertas) != 1 || abertas[0] != "https://github.com/jonyduque/eprocia/releases" {
		t.Fatalf("abertas = %v", abertas)
	}
}

func TestGetVersao(t *testing.T) {
	if got := NewApp().GetVersao(); got != versaoApp() {
		t.Fatalf("GetVersao = %q; quer %q", got, versaoApp())
	}
}

func TestAtualizacao_SemAtualizadorNaoQuebra(t *testing.T) {
	app := NewApp()
	if app.EstadoAtualizacao().Ativo {
		t.Fatal("estado ativo sem Atualizador")
	}
	if _, err := app.VerificarAtualizacao(); err == nil {
		t.Fatal("VerificarAtualizacao sem Atualizador devolveu nil")
	}
	if app.InstalarAtualizacao() == nil || app.ReiniciarAtualizacao() == nil || app.AbrirNovidades() == nil {
		t.Fatal("esperava erro sem Atualizador")
	}
	app.CancelarAtualizacao()
}

func TestReiniciarAtualizacao_ForaDeInstalada(t *testing.T) {
	rede := &redeFalsa{rel: releaseMaior(t), caminho: filepath.Join(t.TempDir(), "novo.exe")}
	tf := &trocadorFalso{}
	m := novoAppAtualizacao(t, cenario{rede: rede})
	m.opcoes.Trocador = func([]string, sync.Locker) atualizacao.Trocador {
		tf.mu.Lock()
		tf.criados++
		tf.mu.Unlock()
		return tf
	}
	m.iniciar()
	if err := m.app.ReiniciarAtualizacao(); err == nil {
		t.Fatal("ReiniciarAtualizacao fora de instalada devolveu nil")
	}
	if c, r := tf.contar(); c != 0 || r != 0 {
		t.Fatalf("Trocador criado=%d reiniciou=%d; quer 0/0", c, r)
	}

	if _, err := m.app.VerificarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	if err := m.app.InstalarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	esperarAte(t, 5*time.Second, "fase instalada", func() bool {
		return m.app.EstadoAtualizacao().Fase == atualizacao.FaseInstalada
	})
	if err := m.app.ReiniciarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	esperarAte(t, 5*time.Second, "Trocador.Reiniciar", func() bool {
		_, r := tf.contar()
		return r == 1
	})
}

type redeCancelada struct{}

func (redeCancelada) Verificar(context.Context, atualizacao.Versao) (*atualizacao.Release, error) {
	return nil, context.Canceled
}

func (redeCancelada) Baixar(context.Context, *atualizacao.Release, atualizacao.Asset, string, string, func(int64, int64)) (atualizacao.Baixado, error) {
	return atualizacao.Baixado{}, context.Canceled
}

func TestVerificarAtualizacao_CanceladaNaoEErro(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{rede: redeCancelada{}})
	m.iniciar()
	info, err := m.app.VerificarAtualizacao()
	if err != nil || info != nil {
		t.Fatalf("info=%v err=%v; quer nil, nil", info, err)
	}
}

func TestParaJS_FrasesDaTela(t *testing.T) {
	casos := map[error]string{
		atualizacao.ErrFaltaReiniciar: atualizacao.MsgFaltaReiniciar,
		atualizacao.ErrEmCurso:        atualizacao.MsgEmCurso,
		atualizacao.ErrSemConsulta:    atualizacao.MsgSemConsulta,
		atualizacao.ErrNaoInstalavel:  atualizacao.MsgNaoInstalavel,
		atualizacao.ErrDesligado:      atualizacao.MsgDesligado,
	}
	for origem, frase := range casos {
		got := paraJS(fmt.Errorf("embrulho C:\\Users\\fulano: %w", origem))
		if got == nil || got.Error() != frase {
			t.Errorf("%v: %v; quer %q", origem, got, frase)
		}
	}
	if got := paraJS(errors.New("C:\\Users\\fulano\\x falhou")); got == nil || got.Error() != atualizacao.MsgSemRede {
		t.Errorf("erro desconhecido: %v", got)
	}
	want := map[string]string{
		atualizacao.MsgFaltaReiniciar: "Reinicie o eprocIA para concluir a atualização",
		atualizacao.MsgEmCurso:        "Já há uma atualização em andamento",
		atualizacao.MsgSemConsulta:    "Procure a atualização antes de instalar",
		atualizacao.MsgNaoInstalavel:  "Esta instalação não pode ser atualizada por aqui",
		atualizacao.MsgDesligado:      "O eprocIA está fechando",
	}
	for got, w := range want {
		if got != w {
			t.Errorf("frase %q; quer %q", got, w)
		}
	}
}

// trocadorPreso segura o Reiniciar até libera fechar: a fase fica em
// reiniciando enquanto o teste confere a fila.
type trocadorPreso struct{ libera chan struct{} }

func (tp trocadorPreso) Instalar(context.Context, atualizacao.Baixado) error { return nil }

func (tp trocadorPreso) Reiniciar(context.Context, atualizacao.Versao) error {
	<-tp.libera
	return errors.New("liberado pelo teste")
}

// Durante o reinício, nenhum caminho do JS põe item na fila: a guarda do
// frontend (aceitaEntrada) pode chegar tarde, e o backend recusa também.
func TestAtualizacao_EntradaRecusadaNoReinicio(t *testing.T) {
	rede := &redeFalsa{rel: releaseMaior(t), caminho: filepath.Join(t.TempDir(), "novo.exe")}
	tp := trocadorPreso{libera: make(chan struct{})}
	m := novoAppAtualizacao(t, cenario{rede: rede})
	m.opcoes.Trocador = func([]string, sync.Locker) atualizacao.Trocador { return tp }
	m.iniciar()
	t.Cleanup(func() { fecharSeAberto(tp.libera) })

	// Um item que termina (em erro: o ZIP não existe), para o Reprocessar.
	primeiro := filepath.Join(t.TempDir(), "5000000-00.2026.8.26.0001.zip")
	if n, err := m.app.AddFilesToQueue([]string{primeiro}); err != nil || n != 1 {
		t.Fatalf("AddFilesToQueue antes do reinício = %d, %v; quer 1, nil", n, err)
	}
	id := m.app.GetQueue()[0].ID
	esperarAte(t, 5*time.Second, "o item terminar", func() bool { return statusDe(m.app, id) == "error" })

	if _, err := m.app.VerificarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	if err := m.app.InstalarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	esperarAte(t, 5*time.Second, "fase instalada", func() bool {
		return m.app.EstadoAtualizacao().Fase == atualizacao.FaseInstalada
	})
	if err := m.app.ReiniciarAtualizacao(); err != nil {
		t.Fatal(err)
	}
	esperarAte(t, 5*time.Second, "fase reiniciando", func() bool {
		return m.app.EstadoAtualizacao().Fase == atualizacao.FaseReiniciando
	})

	outro := filepath.Join(t.TempDir(), "outro.zip")
	querRecusa := func(nome string, err error) {
		t.Helper()
		if err == nil || err.Error() != atualizacao.MsgSoltarDepois {
			t.Errorf("%s durante o reinício: %v; quer %q", nome, err, atualizacao.MsgSoltarDepois)
		}
	}
	n, err := m.app.AddFilesToQueue([]string{outro})
	querRecusa("AddFilesToQueue", err)
	if n != 0 {
		t.Errorf("AddFilesToQueue = %d; quer 0", n)
	}
	n, err = m.app.AddDroppedFiles([]string{outro})
	querRecusa("AddDroppedFiles", err)
	if n != 0 {
		t.Errorf("AddDroppedFiles = %d; quer 0", n)
	}
	_, err = m.app.ProcessFile(outro)
	querRecusa("ProcessFile", err)
	querRecusa("ReprocessQueueItem", m.app.ReprocessQueueItem(id, "none", "basic", false))

	if itens := m.app.GetQueue(); len(itens) != 1 {
		t.Fatalf("fila com %d itens; quer só o primeiro", len(itens))
	}
	if got := statusDe(m.app, id); got != "error" {
		t.Errorf("o item recusado voltou à fila: status %q", got)
	}
}

// A frase da recusa é a mesma do frontend (ATUALIZACAO.soltarDepois).
func TestMsgSoltarDepois_IgualAoFrontend(t *testing.T) {
	textos, err := os.ReadFile(filepath.Join("frontend", "src", "lib", "textos.js"))
	if err != nil {
		t.Fatal(err)
	}
	quer := "soltarDepois: '" + atualizacao.MsgSoltarDepois + "'"
	if atualizacao.MsgSoltarDepois == "" || !strings.Contains(string(textos), quer) {
		t.Errorf("textos.js sem %q", quer)
	}
}

func TestLerArranque(t *testing.T) {
	errSnapshot := errors.New("snapshot")
	comLimpar := []string{"eprocia.exe", "--apos-atualizar=4242", "--limpar-anterior"}
	casos := []struct {
		nome    string
		args    []string
		ppid    int
		exe     string
		errPai  error
		quer    arranque
		chamado int
	}{
		{"pai é o antigo renomeado", comLimpar, 4242, "eprocia.anterior.exe", nil,
			arranque{apos: 4242, limparAnterior: true, pidFlag: 4242, paiPID: 4242, paiExe: "eprocia.anterior.exe"}, 1},
		{"pid não é o pai", comLimpar, 9999, "pwsh.exe", nil,
			arranque{pidFlag: 4242, recusa: "o pid informado não é o processo pai", paiPID: 9999, paiExe: "pwsh.exe"}, 1},
		{"pai não é eprocIA", comLimpar, 4242, "pwsh.exe", nil,
			arranque{pidFlag: 4242, recusa: "o processo pai não é o eprocIA", paiPID: 4242, paiExe: "pwsh.exe"}, 1},
		{"erro do pai", comLimpar, 0, "", errSnapshot,
			arranque{apos: 4242, limparAnterior: true, pidFlag: 4242, semConfirmar: motivoPaiIlegivel, errPai: errSnapshot}, 1},
		{"pai fora do instantâneo", comLimpar, 4242, "", nil,
			arranque{apos: 4242, limparAnterior: true, pidFlag: 4242, paiPID: 4242, semConfirmar: motivoPaiForaDoRetrato}, 1},
		{"sem limpar", []string{"eprocia.exe", "--apos-atualizar=4242"}, 4242, "eprocia.exe", nil,
			arranque{apos: 4242, pidFlag: 4242, paiPID: 4242, paiExe: "eprocia.exe"}, 1},
		{"só limpar", []string{"eprocia.exe", "--limpar-anterior"}, 4242, "eprocia.exe", nil, arranque{}, 0},
		{"sem flag", []string{"eprocia.exe"}, 4242, "eprocia.exe", nil, arranque{}, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chamadas := 0
			pai := func() (int, string, error) {
				chamadas++
				return c.ppid, c.exe, c.errPai
			}
			got := lerArranque(c.args, pai)
			if got != c.quer {
				t.Errorf("lerArranque(%q) = %+v, quer %+v", c.args, got, c.quer)
			}
			if chamadas != c.chamado {
				t.Errorf("pai chamado %d vezes, quer %d", chamadas, c.chamado)
			}
		})
	}
}

func TestLogarRecusaApos(t *testing.T) {
	antigo, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antigo)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	app := &App{arranque: arranque{pidFlag: 4242, recusa: atualizacao.MotivoPaiNaoEprocia, paiPID: 4242, paiExe: "pwsh.exe"}}
	app.logarRecusaApos()
	linha := buf.String()
	if strings.Count(linha, "\n") != 1 {
		t.Fatalf("quer uma linha, veio %q", linha)
	}
	for _, quer := range []string{"level=WARN", "--apos-atualizar ignorado", `motivo="o processo pai não é o eprocIA"`, "pid_flag=4242", "ppid=4242", "pai_exe=pwsh.exe"} {
		if !strings.Contains(linha, quer) {
			t.Errorf("log %q sem %q", linha, quer)
		}
	}

	for _, a := range []arranque{{}, {apos: 4242, pidFlag: 4242, paiPID: 4242, paiExe: "eprocia.exe"}} {
		buf.Reset()
		app := &App{arranque: a}
		app.logarRecusaApos()
		if buf.Len() != 0 {
			t.Errorf("arranque %+v logou %q", a, buf.String())
		}
	}
}

func TestLogarAposSemConfirmar(t *testing.T) {
	antigo, saida, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antigo)
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	casos := []struct {
		a    arranque
		quer []string
	}{
		{arranque{apos: 4242, pidFlag: 4242, semConfirmar: motivoPaiIlegivel, errPai: errors.New("snapshot")},
			[]string{"level=INFO", "--apos-atualizar aceito sem conferir o processo pai", `motivo="não foi possível ler o processo pai"`, "pid_flag=4242", "erro=snapshot"}},
		{arranque{apos: 4242, pidFlag: 4242, paiPID: 4242, semConfirmar: motivoPaiForaDoRetrato},
			[]string{"level=INFO", "--apos-atualizar aceito sem conferir o processo pai", `motivo="o processo pai não está no instantâneo"`, "pid_flag=4242", "ppid=4242"}},
	}
	for _, c := range casos {
		buf.Reset()
		app := &App{arranque: c.a}
		app.logarAposSemConfirmar()
		linha := buf.String()
		if strings.Count(linha, "\n") != 1 {
			t.Fatalf("quer uma linha, veio %q", linha)
		}
		for _, quer := range c.quer {
			if !strings.Contains(linha, quer) {
				t.Errorf("log %q sem %q", linha, quer)
			}
		}
	}
	if strings.Contains(buf.String(), "erro=") {
		t.Errorf("sem erro do pai, o log não traz erro: %q", buf.String())
	}

	for _, a := range []arranque{{}, {apos: 4242, pidFlag: 4242, paiPID: 4242, paiExe: "eprocia.exe"},
		{pidFlag: 4242, recusa: atualizacao.MotivoPaiNaoEprocia, paiPID: 4242, paiExe: "pwsh.exe"}} {
		buf.Reset()
		app := &App{arranque: a}
		app.logarAposSemConfirmar()
		if buf.Len() != 0 {
			t.Errorf("arranque %+v logou %q", a, buf.String())
		}
	}
}
