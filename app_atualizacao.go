package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"eprocia/internal/atualizacao"
	"eprocia/internal/config"
	"eprocia/internal/etl"
	"eprocia/internal/watchdog"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventoAtualizacao leva o Estado do Atualizador ao frontend.
const eventoAtualizacao = "atualizacao:estado"

// intervaloTravaAdiada é o passo das novas tentativas da trava de instância
// no processo novo da atualização.
var intervaloTravaAdiada = 500 * time.Millisecond

// prazoTravaAdiada é quanto depois tenta obter a trava; contado de quando é
// chamado, depois de o antigo sair (ou do prazo de esperá-lo).
var prazoTravaAdiada = atualizacao.PrazoAntigoSair

// travaAposAtualizar adia a trava de instância do processo aberto pela troca
// (--apos-atualizar): o antigo a segura até sair, e o novo não pode sair por
// achá-la ocupada. A envolvida devolve ok com a trava ocupada; depois tenta de
// novo a cada intervaloTravaAdiada, por até prazoTravaAdiada, e devolve true
// ao obtê-la (na hora, se a trava veio de primeira ou se ela estiver
// indisponível) ou false no fim do prazo. A trava obtida depois fica com o
// processo até ele terminar.
func travaAposAtualizar(travar func() (func(), bool, error)) (envolvida func() (func(), bool, error), depois func() bool) {
	ocupada := false
	envolvida = func() (func(), bool, error) {
		liberar, ok, err := travar()
		if err == nil && !ok {
			ocupada = true
			return func() {}, true, nil
		}
		return liberar, ok, err
	}
	depois = func() bool {
		if !ocupada {
			return true
		}
		fim := time.Now().Add(prazoTravaAdiada)
		for {
			_, ok, err := travar()
			if err != nil {
				slog.Warn("trava de instância indisponível depois da atualização")
				return true
			}
			if ok {
				return true
			}
			if !time.Now().Before(fim) {
				return false
			}
			time.Sleep(intervaloTravaAdiada)
		}
	}
	return envolvida, depois
}

// emitirEm manda um evento ao frontend por a.emitir (testes) ou pelo Wails;
// com ctx nil, não emite nada (o runtime do Wails encerraria o processo).
func (a *App) emitirEm(ctx context.Context, evento string, dados ...any) {
	if a.emitir != nil {
		a.emitir(evento, dados...)
		return
	}
	if ctx != nil {
		runtime.EventsEmit(ctx, evento, dados...)
	}
}

// ligarMonitor liga o watcher conforme a config guardada (WatchdogEnabled e
// MonitoredPath), como o startup sempre fez; nada faz com o monitor retido
// (processo novo da atualização à espera do antigo e da trava).
func (a *App) ligarMonitor(ctx context.Context) {
	a.monitorMu.Lock()
	defer a.monitorMu.Unlock()
	if a.monitorRetido.Load() {
		return
	}
	cfg := a.configAtual()
	if cfg == nil || !cfg.WatchdogEnabled || cfg.MonitoredPath == "" {
		return
	}
	if err := a.watcher.Start(cfg.MonitoredPath); err != nil {
		slog.Warn("falha ao iniciar o monitoramento da pasta", "pasta", cfg.MonitoredPath, "erro", err)
		a.emitirEm(ctx, "watchdog:error", err.Error())
	}
}

// iniciarFila é a parte do startup que não precisa da janela: pipeline,
// fila, telemetria, config, falhas, inicialização com o Windows e watcher.
// Com apos > 0 (processo novo da atualização), a fila nasce retida e o
// watcher fica desligado até o antigo sair (arranqueApos). Com ctx nil, a
// fila não emite eventos.
func (a *App) iniciarFila(ctx context.Context) {
	a.pipeline = etl.NewPipeline()
	a.queue = etl.NewQueue(a.pipeline)
	if a.apos > 0 {
		a.queue.Reter()
		a.monitorRetido.Store(true)
	}
	a.queue.SetContext(ctx)
	a.queue.SetOnFinished(a.aoTerminar)
	// A fila nasce depois de iniciarRegistro (main): a telemetria se liga
	// aqui, e só com o log ativo; desligado, nada dela executa.
	a.ligarTelemetriaSeAtivo(a.pastaDados)

	// main já leu a config para escolher o fundo da janela.
	cfg := a.configAtual()
	if cfg == nil {
		cfg = carregarConfig(config.Load)
		a.guardarConfig(cfg)
	}
	a.queue.SetConfig(cfg)
	if a.falhas == nil {
		if caminho, err := config.GetConfigPath(); err == nil {
			a.falhas = watchdog.CarregarFalhas(filepath.Join(filepath.Dir(caminho), "falhas.json"))
		}
	}
	a.watcher.SetFalhas(a.falhas)
	a.sincronizarAutostart(cfg)
	if a.apos == 0 {
		a.ligarMonitor(ctx)
	}
}

// opcoesAtualizacao monta as Opcoes reais do Atualizador para este exe.
func (a *App) opcoesAtualizacao() atualizacao.Opcoes {
	atual, err := atualizacao.ParseVersao(versaoApp())
	if err != nil {
		atual = atualizacao.Versao{} // build de desenvolvimento: desligado
	}
	inst := atualizacao.Detectar(caminhoExecutavel(), atualizacao.RegistroWindows(), atualizacao.PodeEscrever)
	pid := os.Getpid()
	return atualizacao.Opcoes{
		Atual:            atual,
		Instalacao:       inst,
		PastaDados:       a.pastaDados,
		PastaAtualizacao: atualizacao.PastaAtualizacao(os.Getenv("LOCALAPPDATA")),
		PID:              pid,
		Rede:             atualizacao.NovoCliente(atualizacao.RegrasGitHub(), nil, atual),
		Shell:            atualizacao.ShellExecuteEx,
		Trocador: func(extras []string, trava sync.Locker) atualizacao.Trocador {
			return &atualizacao.Troca{Pasta: inst.Pasta, PastaDados: a.pastaDados, PIDAtual: pid, Extras: extras, PrazoSinal: atualizacao.PrazoSinal, Trava: trava}
		},
	}
}

// iniciarAtualizacao cria o Atualizador (sem nenhuma consulta: a rede só é
// usada a pedido do usuário) e, nos modos que trocam arquivos, apaga as
// sobras de uma troca anterior. Chamada depois de iniciarFila.
func (a *App) iniciarAtualizacao(o atualizacao.Opcoes) {
	a.modo = o.Instalacao.Modo
	a.pastaInstalacao = o.Instalacao.Pasta
	if a.modo == atualizacao.ModoTrocar || a.modo == atualizacao.ModoInstalador {
		atualizacao.Limpar(o.Instalacao.Pasta, o.PastaDados, o.PastaAtualizacao)
	}
	amb := atualizacao.Ambiente{
		PararMonitor:   a.watcher.Stop,
		ReligarMonitor: func() { a.ligarMonitor(a.ctx) },
		Reter:          a.queue.Reter,
		Soltar:         a.queue.Soltar,
		Ociosa:         a.queue.Ociosa,
		Pausada:        a.queue.IsPaused,
		Oculta:         a.oculta.Load,
		Sair: func() {
			a.ocultarJanela()
			a.Quit()
		},
		Emitir: func(e atualizacao.Estado) { a.emitirEm(a.ctx, eventoAtualizacao, e) },
	}
	a.atualizador = atualizacao.NovoAtualizador(o, amb)
	a.queue.SetAoFicarOciosa(a.atualizador.AoFicarOciosa)
}

// sinalNoStartup é o último passo do startup. Com a janela nascendo oculta
// (--tray), o carregamento do frontend não é garantido: o sinal de vida (com
// apos) e a limpeza do anterior partem daqui, sem esperar o OnDomReady.
func (a *App) sinalNoStartup() {
	if !a.oculta.Load() {
		return
	}
	if a.apos > 0 {
		a.darSinal()
	}
	a.limparAnteriorEmSegundoPlano()
}

// domPronto é o OnDomReady do Wails: sinal de vida (com apos) e limpeza do
// anterior, cada um uma vez só.
func (a *App) domPronto(_ context.Context) {
	if a.apos > 0 {
		a.darSinal()
	}
	a.limparAnteriorEmSegundoPlano()
}

// darSinal grava, uma vez só, o sinal de vida para o processo antigo e
// inicia arranqueApos numa goroutine.
func (a *App) darSinal() {
	a.sinalUmaVez.Do(func() {
		if err := atualizacao.GravarSinal(a.pastaDados, a.apos, versaoApp()); err != nil {
			slog.Warn("atualização: sinal de vida não gravado")
		}
		go a.arranqueApos()
	})
}

// arranqueApos espera o processo antigo sair, solta a fila, grava a versão
// no registro de desinstalação (modo trocar), espera a trava de instância e
// só então libera e liga o watcher: nunca dois processos monitoram a mesma
// pasta. Sem a trava no prazo, o watcher fica desligado.
func (a *App) arranqueApos() {
	if !a.esperarAntigo() {
		slog.Warn("atualização: o processo anterior não saiu no prazo; seguindo")
	}
	if a.queue != nil {
		a.queue.Soltar()
	}
	if a.modo == atualizacao.ModoTrocar {
		a.gravarVersaoAtual()
	}
	if a.travarDepois != nil && !a.travarDepois() {
		slog.Warn("atualização: trava de instância não obtida no prazo; monitoramento da pasta desligado")
		return
	}
	a.monitorRetido.Store(false)
	a.ligarMonitor(a.ctx)
}

func (a *App) esperarAntigo() bool {
	esperar := a.esperarSair
	if esperar == nil {
		esperar = atualizacao.EsperarSair
	}
	return esperar(a.apos, atualizacao.PrazoAntigoSair)
}

func (a *App) gravarVersaoAtual() {
	if a.gravarVersao == nil {
		return
	}
	if err := a.gravarVersao(versaoApp()); err != nil {
		slog.Warn("atualização: versão não gravada no registro de desinstalação")
	}
}

// limparAnteriorEmSegundoPlano (d5) apaga o eprocia.anterior.exe numa
// goroutine, uma vez só e só no modo trocar; quem chama nunca espera. Com
// apos e --limpar-anterior, espera o antigo sair (sem sair no prazo, tenta
// assim mesmo); com apos sem a flag, não faz nada; sem apos (o usuário
// instalou e não reiniciou), apaga o arquivo se existir e grava a versão.
// Falha só vai ao log, com texto fixo; a próxima abertura tenta de novo.
func (a *App) limparAnteriorEmSegundoPlano() {
	a.limpezaUmaVez.Do(func() {
		if a.modo != atualizacao.ModoTrocar || (a.apos > 0 && !a.limparAnterior) {
			return
		}
		go func() {
			if a.apos > 0 {
				if !a.esperarAntigo() {
					slog.Warn("limpeza da versão anterior: o processo anterior não saiu no prazo; tentando assim mesmo")
				}
				a.apagarVersaoAnterior()
				return
			}
			if _, err := os.Stat(filepath.Join(a.pastaInstalacao, atualizacao.NomeAnterior)); err != nil {
				return
			}
			a.apagarVersaoAnterior()
			a.gravarVersaoAtual()
		}()
	})
}

func (a *App) apagarVersaoAnterior() {
	apagar := a.apagarAnterior
	if apagar == nil {
		apagar = atualizacao.LimparAnterior
	}
	if err := apagar(a.pastaInstalacao); err != nil {
		slog.Warn("limpeza da versão anterior: o eprocia.anterior.exe ficou")
	}
}

const (
	urlVersoes        = "https://github.com/jonyduque/eprocia/releases"
	msgAtualizacaoOff = "A atualização pelo aplicativo não está disponível"
)

var (
	errAtualizacaoOff = errors.New(msgAtualizacaoOff)
	errSemNotas       = errors.New("Não há página de novidades para abrir")
)

// paraJS devolve ao frontend só uma frase fixa: o erro do Atualizador já
// traz texto fixo (as Msg* ou os sentinelas), e qualquer outro vira a frase
// genérica, nunca err.Error() com caminho ou URL.
func paraJS(err error) error {
	if err == nil {
		return nil
	}
	for sentinela, frase := range map[error]string{
		atualizacao.ErrSemConsulta:    atualizacao.MsgSemConsulta,
		atualizacao.ErrNaoInstalavel:  atualizacao.MsgNaoInstalavel,
		atualizacao.ErrEmCurso:        atualizacao.MsgEmCurso,
		atualizacao.ErrDesligado:      atualizacao.MsgDesligado,
		atualizacao.ErrFaltaReiniciar: atualizacao.MsgFaltaReiniciar,
	} {
		if errors.Is(err, sentinela) {
			return errors.New(frase)
		}
	}
	switch err.Error() {
	case atualizacao.MsgSemRede, atualizacao.MsgRecusada:
		return err
	}
	return errors.New(atualizacao.MsgSemRede)
}

// recusaEntrada é a guarda do backend para tudo que põe item na fila pelo
// JS: durante o reinício ou a abertura do instalador (fase interna de
// preparação incluída), a frase fixa de soltar depois; senão nil.
func (a *App) recusaEntrada() error {
	if a.atualizador != nil && !a.atualizador.AceitaEntrada() {
		return errors.New(atualizacao.MsgSoltarDepois)
	}
	return nil
}

// GetVersao é a versão do aplicativo em execução.
func (a *App) GetVersao() string { return versaoApp() }

// EstadoAtualizacao é o instantâneo do estado da atualização, sem rede.
func (a *App) EstadoAtualizacao() atualizacao.Estado {
	if a.atualizador == nil {
		return atualizacao.Estado{}
	}
	return a.atualizador.Estado()
}

// VerificarAtualizacao consulta a release mais nova (só a pedido do usuário).
func (a *App) VerificarAtualizacao() (*atualizacao.Info, error) {
	if a.atualizador == nil {
		return nil, errAtualizacaoOff
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	info, err := a.atualizador.Verificar(ctx)
	if errors.Is(err, context.Canceled) {
		// Operação substituída ou cancelada pelo usuário: não é erro.
		return nil, nil
	}
	if err != nil {
		return nil, paraJS(err)
	}
	return &info, nil
}

// InstalarAtualizacao baixa e instala a versão da última consulta.
func (a *App) InstalarAtualizacao() error {
	if a.atualizador == nil {
		return errAtualizacaoOff
	}
	return paraJS(a.atualizador.Instalar())
}

// CancelarAtualizacao cancela o download ou a espera da fila.
func (a *App) CancelarAtualizacao() {
	if a.atualizador != nil {
		a.atualizador.Cancelar()
	}
}

// ReiniciarAtualizacao reinicia para a versão instalada (só na fase instalada).
func (a *App) ReiniciarAtualizacao() error {
	if a.atualizador == nil {
		return errAtualizacaoOff
	}
	return paraJS(a.atualizador.Reiniciar())
}

func (a *App) abrirNoNavegador(url string) {
	if a.abrirURL != nil {
		a.abrirURL(url)
		return
	}
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// AbrirNovidades abre as notas da release da última consulta, só se a URL for
// do repositório do eprocIA.
func (a *App) AbrirNovidades() error {
	if a.atualizador == nil {
		return errAtualizacaoOff
	}
	url := a.atualizador.NotasURL()
	if !urlNovidadesValida(url) {
		return errSemNotas
	}
	a.abrirNoNavegador(url)
	return nil
}

// AbrirPaginaVersoes abre a página de versões do repositório.
func (a *App) AbrirPaginaVersoes() error {
	a.abrirNoNavegador(urlVersoes)
	return nil
}

// urlNovidadesValida aceita só https://github.com/jonyduque/eprocia/releases/...
// (host exato, sem usuário, sem ".." nem barra invertida no original).
func urlNovidadesValida(bruta string) bool {
	if strings.Contains(bruta, "\\") || strings.Contains(bruta, "..") {
		return false
	}
	u, err := url.Parse(bruta)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil {
		return false
	}
	return strings.HasPrefix(path.Clean(u.Path)+"/", "/jonyduque/eprocia/releases/")
}
