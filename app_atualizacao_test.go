package main

import (
	"bytes"
	"context"
	"errors"
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
	app.cfg = &models.Config{MonitoredPath: m.monitorada, WatchdogEnabled: true, AutoStart: true, OcrMode: "none", CleaningLevel: "basic"}
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
	if n := m.app.AddFilesToQueue([]string{inexistente}); n != 1 {
		t.Fatalf("AddFilesToQueue = %d; quer 1", n)
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
