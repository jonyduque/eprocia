package atualizacao

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"eprocia/internal/autostart"
	"eprocia/internal/registro"
)

// Rede é a parte do Cliente que o Atualizador usa (*Cliente a implementa).
type Rede interface {
	Verificar(ctx context.Context, atual Versao) (*Release, error)
	Baixar(ctx context.Context, rel *Release, asset Asset, pasta, nome string, progresso func(feito, total int64)) (Baixado, error)
}

// Gravador é o histórico de processamento, fechado antes do reinício e
// reaberto se o reinício falhar (nil = não há).
type Gravador interface {
	Fechar() error
	Reabrir() error
}

// Ambiente liga o Atualizador ao resto do app. Funções nil são ignoradas
// (Ociosa nil conta como ociosa). Sair esconde a janela e chama App.Quit,
// que leva ao shutdown e ao Encerrar.
type Ambiente struct {
	PararMonitor, ReligarMonitor, Reter, Soltar func()
	Ociosa, Pausada, Oculta                     func() bool
	Gravador                                    Gravador
	Sair                                        func()
	Emitir                                      func(Estado)
}

// Trocador é a parte da Troca que o Atualizador usa (*Troca a implementa).
type Trocador interface {
	Instalar(ctx context.Context, b Baixado) error
	Reiniciar(ctx context.Context, nova Versao) error
}

// Opcoes configura o Atualizador. Trocador nil cria uma *Troca na pasta da
// instalação; Shell nil = ShellExecuteEx; Reabrir nil = ReabrirConferido.
type Opcoes struct {
	Atual                        Versao
	Instalacao                   Instalacao
	PastaDados, PastaAtualizacao string
	PID                          int
	Rede                         Rede
	Trocador                     func(extras []string, trava sync.Locker) Trocador
	Shell                        func(caminho string) error
	Reabrir                      func(Baixado) (*Travado, error)
}

// pendente é o baixado ainda não entregue a Trocador.Instalar nem ao
// Reabrir: Encerrar e Cancelar o apagam. pasta é a subpasta do instalador.
type pendente struct{ arquivo, pasta string }

func (p pendente) apagar() {
	if p.arquivo != "" {
		_ = os.Remove(p.arquivo)
	}
	if p.pasta != "" {
		_ = os.RemoveAll(p.pasta)
	}
}

// evento é um Estado a emitir, numerado na ordem em que foi gravado.
type evento struct {
	seq uint64
	e   Estado
}

// Atualizador coordena a atualização: consulta, download, instalação,
// reinício e abertura do instalador, uma operação por vez.
//
// Travas, nesta ordem e sem ciclo:
//   - mu guarda o estado. Emitir, Sair e as funções do Ambiente nunca rodam
//     com mu preso (Sair leva ao Encerrar; Ociosa pode chamar AoFicarOciosa).
//   - trocaMu é segurado em volta de Trocador.Instalar e, pela Troca, durante
//     os renames da reversão. Quem o segura nunca pega mu: solta o trocaMu
//     antes de gravar a fase. E nunca se trava o trocaMu com mu preso: a
//     goroutine confere encerrado sob mu, solta mu e só então trava o
//     trocaMu (se o Encerrar ganhou, ela fica parada ali sem segurar mu).
//   - Encerrar faz o que precisa sob mu, solta mu e trava o trocaMu, que
//     nunca mais é solto: nenhuma sequência de renames começa depois dele, e
//     ele espera a que estiver no meio.
//   - emitMu só ordena as chamadas a Emitir (um estado mais velho nunca é
//     emitido depois de um mais novo); Emitir não pode chamar o Atualizador.
type Atualizador struct {
	o     Opcoes
	amb   Ambiente
	atual string
	ativo bool

	mu        sync.Mutex
	fase      string
	versao    string
	progresso int
	mensagem  string
	modo      string
	motivo    string
	consulta  Info     // última consulta boa (Modo vazio = nenhuma)
	rel       *Release // release da última consulta com versão maior
	instalada bool     // a versão nova está em eprocia.exe, à espera do reinício
	nova      Versao
	gen       uint64 // a operação em curso; muda a cada operação e a cada Cancelar
	ctxOp     context.Context
	cancelar  context.CancelFunc
	baixado   Baixado // o instalador baixado, à espera da fila
	pendente  pendente
	encerrado bool
	seq       uint64

	emitMu    sync.Mutex
	publicado uint64

	trocaMu  sync.Mutex
	encerrar sync.Once
	// antesDeTravarTroca roda no Encerrar entre o cancelamento e a trava
	// do trocaMu (só teste; nil em produção).
	antesDeTravarTroca func()
}

// NovoAtualizador cria o Atualizador. Com o.Atual inválida (0.0.0, build de
// desenvolvimento), fica desligado: Estado().Ativo falso e tudo recusado.
func NovoAtualizador(o Opcoes, amb Ambiente) *Atualizador {
	if o.Trocador == nil {
		o.Trocador = func(extras []string, trava sync.Locker) Trocador {
			return &Troca{Pasta: o.Instalacao.Pasta, PastaDados: o.PastaDados, PIDAtual: o.PID, Extras: extras, Trava: trava}
		}
	}
	if o.Shell == nil {
		o.Shell = ShellExecuteEx
	}
	if o.Reabrir == nil {
		o.Reabrir = ReabrirConferido
	}
	return &Atualizador{o: o, amb: amb, atual: o.Atual.String(), ativo: o.Atual.Valida(), ctxOp: context.Background()}
}

// ocupada: fases de uma operação em curso.
func ocupada(fase string) bool {
	switch fase {
	case FaseVerificando, FaseBaixando, FaseConferindo, FaseInstalando, FaseAguardandoFila,
		fasePreparando, FaseReiniciando, FaseAbrindoInstalador:
		return true
	}
	return false
}

func (a *Atualizador) estadoLocked() Estado {
	fase := a.fase
	if fase == fasePreparando {
		fase = FaseAbrindoInstalador
		if a.modo == string(ModoTrocar) {
			fase = FaseReiniciando
		}
	}
	return Estado{Ativo: a.ativo, Fase: fase, Versao: a.versao, Progresso: a.progresso,
		Mensagem: a.mensagem, Modo: a.modo, Motivo: a.motivo, Atual: a.atual}
}

// marcarLocked numera o estado atual para publicar depois de soltar mu.
func (a *Atualizador) marcarLocked() evento {
	a.seq++
	return evento{seq: a.seq, e: a.estadoLocked()}
}

// publicar emite ev fora de mu, descartando o que chegar fora de ordem.
func (a *Atualizador) publicar(ev evento) {
	if a.amb.Emitir == nil {
		return
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	if ev.seq <= a.publicado {
		return
	}
	a.publicado = ev.seq
	a.amb.Emitir(ev.e)
}

// validaLocked: a operação gen continua a em curso e não houve Encerrar.
func (a *Atualizador) validaLocked(gen uint64) bool { return a.gen == gen && !a.encerrado }

func chamar(f func()) {
	if f != nil {
		f()
	}
}

func (a *Atualizador) ociosa() bool {
	if a.amb.Ociosa == nil {
		return true
	}
	return a.amb.Ociosa()
}

func (a *Atualizador) pausada() bool { return a.amb.Pausada != nil && a.amb.Pausada() }
func (a *Atualizador) oculta() bool  { return a.amb.Oculta != nil && a.amb.Oculta() }

// Estado devolve o estado atual.
func (a *Atualizador) Estado() Estado {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.estadoLocked()
}

// NotasURL é a página da release da última consulta boa.
func (a *Atualizador) NotasURL() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.consulta.NotasURL
}

// Verificar consulta a release mais nova. ErrIncompleta vira o modo
// incompleta sem erro; erro de consulta devolve um erro cujo texto é só a
// frase da tela (errors.Is acha a origem).
func (a *Atualizador) Verificar(ctx context.Context) (Info, error) {
	if !a.ativo {
		return Info{}, ErrDesligado
	}
	a.mu.Lock()
	switch {
	case a.encerrado:
		a.mu.Unlock()
		return Info{}, ErrDesligado
	case a.instalada:
		a.mu.Unlock()
		return Info{}, ErrFaltaReiniciar
	case ocupada(a.fase):
		a.mu.Unlock()
		return Info{}, ErrEmCurso
	}
	a.gen++
	gen := a.gen
	ctxOp, cancel := context.WithCancel(ctx)
	a.ctxOp, a.cancelar = ctxOp, cancel
	a.fase, a.versao, a.progresso, a.mensagem, a.modo, a.motivo = FaseVerificando, "", 0, "", "", ""
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)

	rel, err := a.o.Rede.Verificar(ctxOp, a.o.Atual)
	cancel()
	log := registro.Logger(ctx)

	a.mu.Lock()
	if !a.validaLocked(gen) {
		a.mu.Unlock()
		return Info{}, context.Canceled
	}
	var info Info
	switch {
	case errors.Is(err, ErrIncompleta):
		info = Info{Modo: ModoIncompleta}
		if rel != nil {
			info.Versao, info.NotasURL = rel.Versao.String(), rel.NotasURL
		}
		a.fase, a.rel = FaseIncompleta, nil
	case err != nil:
		frase := MsgSemRede
		if errors.Is(err, ErrRecusada) {
			frase = MsgRecusada
		}
		a.fase, a.mensagem, a.rel, a.consulta = FaseErro, frase, nil, Info{}
		ev = a.marcarLocked()
		a.mu.Unlock()
		a.publicar(ev)
		log.Warn("consulta de atualização", "resultado", classeErro(err))
		return Info{}, &erroComFrase{frase: frase, causa: err}
	case rel == nil || rel.Versao.Comparar(a.o.Atual) <= 0:
		info = Info{Versao: a.atual, Modo: ModoEmDia}
		a.fase, a.rel = FaseEmDia, nil
	default:
		info = Info{Versao: rel.Versao.String(), NotasURL: rel.NotasURL, Modo: string(a.o.Instalacao.Modo)}
		if a.o.Instalacao.Modo == ModoSoAvisar {
			info.Motivo = a.o.Instalacao.Motivo
		}
		a.fase, a.rel = FaseDisponivel, rel
	}
	a.consulta = info
	a.versao, a.modo, a.motivo = info.Versao, info.Modo, info.Motivo
	ev = a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
	log.Info("consulta de atualização", "resultado", info.Modo, "versao", info.Versao)
	return info, nil
}

// Instalar valida e devolve; o download e a instalação (modo trocar) ou a
// abertura do instalador correm numa goroutine e se reportam por Emitir.
func (a *Atualizador) Instalar() error {
	if !a.ativo {
		return ErrDesligado
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.encerrado:
		return ErrDesligado
	case a.instalada:
		return ErrFaltaReiniciar
	case ocupada(a.fase):
		return ErrEmCurso
	case a.consulta.Modo == "":
		return ErrSemConsulta
	case a.rel == nil || (a.consulta.Modo != string(ModoTrocar) && a.consulta.Modo != string(ModoInstalador)):
		return ErrNaoInstalavel
	}
	a.gen++
	gen := a.gen
	ctx, cancel := context.WithCancel(context.Background())
	a.ctxOp, a.cancelar = ctx, cancel
	rel, modo := a.rel, Modo(a.consulta.Modo)
	a.fase, a.progresso, a.mensagem = FaseBaixando, 0, ""
	a.versao, a.modo, a.motivo = rel.Versao.String(), string(modo), ""
	ev := a.marcarLocked()
	go func() {
		a.publicar(ev)
		a.instalar(ctx, gen, rel, modo)
	}()
	return nil
}

// instalar baixa e, no modo trocar, instala; no modo instalador, segue para
// a espera da fila.
func (a *Atualizador) instalar(ctx context.Context, gen uint64, rel *Release, modo Modo) {
	log := registro.Logger(ctx)
	var p pendente
	asset, pasta, nome := rel.Exe, a.o.Instalacao.Pasta, PrefixoTemp+rand.Text()
	if modo == ModoInstalador {
		dir, err := NovaPastaInstalador(a.o.PastaAtualizacao)
		if err != nil {
			log.Error("download de atualização: não criou a pasta do instalador", atributosErro(err)...)
			a.falhar(gen, MsgInterrompido)
			return
		}
		asset, pasta, nome = rel.Instalador, dir, rel.Instalador.Nome
		p.pasta = dir
	}
	p.arquivo = filepath.Join(pasta, nome)

	a.mu.Lock()
	if !a.validaLocked(gen) {
		a.mu.Unlock()
		p.apagar()
		return
	}
	a.pendente = p
	a.mu.Unlock()

	b, err := a.o.Rede.Baixar(ctx, rel, asset, pasta, nome, a.aoProgredir(gen))

	a.mu.Lock()
	if !a.validaLocked(gen) {
		// Cancelar ou Encerrar: o arquivo nunca é entregue.
		a.mu.Unlock()
		p.apagar()
		return
	}
	if err != nil {
		frase := MsgInterrompido
		if errors.Is(err, ErrNaoConferiu) {
			frase = MsgNaoConferiu
		}
		a.pendente = pendente{}
		a.fase, a.mensagem, a.progresso = FaseErro, frase, 0
		ev := a.marcarLocked()
		a.mu.Unlock()
		a.publicar(ev)
		p.apagar()
		return
	}
	if modo == ModoInstalador {
		a.baixado = b
		a.fase = fasePreparando
		a.mu.Unlock()
		a.tentar(ctx, gen)
		return
	}

	// Modo trocar: entrega o baixado ao Trocador.Instalar, sem nada do
	// Ambiente (a troca dos arquivos não fecha o programa).
	a.pendente = pendente{}
	a.fase, a.progresso = FaseInstalando, 0
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)

	tr := a.o.Trocador(nil, &a.trocaMu)
	a.trocaMu.Lock() // nunca com mu preso; depois do Encerrar, fica aqui
	if ctx.Err() != nil {
		// O Encerrar cancelou entre a conferência e a trava: nenhuma
		// sequência de renames começa (sem pegar mu com o trocaMu preso).
		a.trocaMu.Unlock()
		_ = os.Remove(b.Caminho)
		return
	}
	err = tr.Instalar(ctx, b)
	a.trocaMu.Unlock()
	if errors.Is(err, ErrNaoAplicou) {
		// Nada mudou: o baixado que sobrou na pasta da instalação sai.
		_ = os.Remove(b.Caminho)
	}

	a.mu.Lock()
	if a.encerrado {
		a.mu.Unlock()
		return
	}
	if err == nil {
		a.instalada, a.nova = true, b.Versao
		a.fase, a.versao, a.mensagem = FaseInstalada, b.Versao.String(), ""
	} else {
		a.fase, a.mensagem = FaseErro, a.fraseErro(err)
	}
	ev = a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
	if err != nil {
		log.Error("instalação da atualização", "resultado", classeErro(err))
	}
}

// aoProgredir emite o progresso do download só quando o inteiro muda; com o
// arquivo inteiro, a fase passa a conferindo.
func (a *Atualizador) aoProgredir(gen uint64) func(feito, total int64) {
	return func(feito, total int64) {
		if total <= 0 {
			return
		}
		pct := int(min(max(feito*100/total, 0), 100))
		a.mu.Lock()
		if !a.validaLocked(gen) || a.fase != FaseBaixando {
			a.mu.Unlock()
			return
		}
		switch {
		case feito >= total:
			a.fase, a.progresso = FaseConferindo, 100
		case pct == a.progresso:
			a.mu.Unlock()
			return
		default:
			a.progresso = pct
		}
		ev := a.marcarLocked()
		a.mu.Unlock()
		a.publicar(ev)
	}
}

// falhar grava a fase erro com a frase, se a operação gen ainda valer.
func (a *Atualizador) falhar(gen uint64, frase string) {
	a.mu.Lock()
	if !a.validaLocked(gen) {
		a.mu.Unlock()
		return
	}
	a.pendente = pendente{}
	a.fase, a.mensagem, a.progresso = FaseErro, frase, 0
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
}

// Reiniciar valida e devolve; a espera da fila, a preparação e o reinício
// correm numa goroutine. Só na fase instalada.
func (a *Atualizador) Reiniciar() error {
	if !a.ativo {
		return ErrDesligado
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.encerrado:
		return ErrDesligado
	case !a.instalada:
		return ErrNaoInstalavel
	case a.fase != FaseInstalada:
		return ErrEmCurso
	}
	a.gen++
	gen := a.gen
	ctx, cancel := context.WithCancel(context.Background())
	a.ctxOp, a.cancelar = ctx, cancel
	// Da conferência inicial da fila: só esta goroutine segue.
	a.fase, a.mensagem = fasePreparando, ""
	go a.tentar(ctx, gen)
	return nil
}

// tentar segue para a preparação se a fila estiver ociosa; senão grava
// aguardando_fila e confere a fila de novo (um aviso que chegou antes da
// fase gravada não se perde). Chamada na fase interna preparando.
func (a *Atualizador) tentar(ctx context.Context, gen uint64) {
	if a.ociosa() {
		a.preparar(ctx, gen)
		return
	}
	a.aguardarFila(gen, pendente{}, false)
}

// aguardarFila grava aguardando_fila (devolvendo p ao pendente, se repor) e
// reconfere a fila fora de mu.
func (a *Atualizador) aguardarFila(gen uint64, p pendente, repor bool) {
	pausada := a.pausada()
	a.mu.Lock()
	if !a.validaLocked(gen) || a.fase != fasePreparando {
		enc := a.encerrado
		a.mu.Unlock()
		if repor && enc {
			p.apagar()
		}
		return
	}
	if repor {
		a.pendente = p
	}
	a.fase, a.mensagem = FaseAguardandoFila, ""
	if pausada {
		a.mensagem = MsgRetomeFila
	}
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
	if a.ociosa() {
		a.AoFicarOciosa()
	}
}

// AoFicarOciosa é o aviso da fila: em aguardando_fila, passa (uma vez só,
// sob mu) para a preparação; em qualquer outra fase, não faz nada.
func (a *Atualizador) AoFicarOciosa() {
	a.mu.Lock()
	if a.encerrado || a.fase != FaseAguardandoFila {
		a.mu.Unlock()
		return
	}
	a.fase = fasePreparando
	gen, ctx := a.gen, a.ctxOp
	a.mu.Unlock()
	go a.preparar(ctx, gen)
}

// preparar: Reabrir (modo instalador), PararMonitor, Reter, conferência da
// fila retida, Gravador.Fechar; depois Trocador.Reiniciar ou o shell e, com
// sucesso, Sair. Falha desfaz na ordem inversa.
func (a *Atualizador) preparar(ctx context.Context, gen uint64) {
	log := registro.Logger(ctx)
	a.mu.Lock()
	if !a.validaLocked(gen) || a.fase != fasePreparando {
		a.mu.Unlock()
		return
	}
	modo, b, nova, p := Modo(a.modo), a.baixado, a.nova, a.pendente
	if modo == ModoInstalador {
		a.pendente = pendente{} // entregue ao Reabrir
	}
	a.mu.Unlock()

	var trav *Travado
	if modo == ModoInstalador {
		t, err := a.o.Reabrir(b)
		if err != nil {
			p.apagar()
			log.Error("abertura do instalador: não conferiu de novo", "resultado", classeErro(err))
			a.falhar(gen, a.fraseErro(err))
			return
		}
		trav = t
	}
	fecharTrav := func() {
		if trav != nil {
			trav.Fechar()
		}
	}

	chamar(a.amb.PararMonitor)
	chamar(a.amb.Reter)
	if !a.ociosa() {
		// Um item entrou entre o aviso e a fila retida.
		chamar(a.amb.Soltar)
		chamar(a.amb.ReligarMonitor)
		fecharTrav()
		a.aguardarFila(gen, p, modo == ModoInstalador)
		return
	}
	if g := a.amb.Gravador; g != nil {
		if err := g.Fechar(); err != nil {
			log.Error("atualização: não fechou o histórico", atributosErro(err)...)
			chamar(a.amb.Soltar)
			chamar(a.amb.ReligarMonitor)
			fecharTrav()
			if modo == ModoInstalador {
				p.apagar()
				a.falhar(gen, MsgNaoAplicou(a.atual))
				return
			}
			// A versão nova continua instalada: volta a instalada.
			a.mu.Lock()
			if !a.validaLocked(gen) {
				a.mu.Unlock()
				return
			}
			a.fase, a.mensagem = FaseInstalada, MsgNaoAplicou(a.atual)
			ev := a.marcarLocked()
			a.mu.Unlock()
			a.publicar(ev)
			return
		}
	}

	a.mu.Lock()
	if !a.validaLocked(gen) {
		// Encerrar durante a preparação: nada começa.
		a.mu.Unlock()
		fecharTrav()
		p.apagar() // instalador: o Reabrir soltou o arquivo; trocar: p vazio
		return
	}
	if modo == ModoTrocar {
		a.fase, a.mensagem = FaseReiniciando, MsgAbrindo(nova.String())
	} else {
		a.fase, a.mensagem = FaseAbrindoInstalador, ""
	}
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)

	var err error
	if modo == ModoTrocar {
		var extras []string
		if a.oculta() {
			extras = []string{autostart.FlagTray}
		}
		// Sem cancelamento: o Encerrar não interrompe o Reiniciar (o novo já
		// espera o antigo sair); a reversão, se houver, espera o trocaMu.
		err = a.o.Trocador(extras, &a.trocaMu).Reiniciar(context.WithoutCancel(ctx), nova)
	} else {
		err = trav.Abrir(a.o.Shell)
	}

	a.mu.Lock()
	enc := a.encerrado
	a.mu.Unlock()
	if enc {
		return
	}
	if err == nil {
		chamar(a.amb.Sair)
		return
	}

	if g := a.amb.Gravador; g != nil {
		if errR := g.Reabrir(); errR != nil {
			log.Error("atualização: não reabriu o histórico", atributosErro(errR)...)
		}
	}
	chamar(a.amb.Soltar)
	chamar(a.amb.ReligarMonitor)
	if modo == ModoInstalador {
		p.apagar()
	}

	a.mu.Lock()
	if !a.validaLocked(gen) {
		a.mu.Unlock()
		return
	}
	if errors.Is(err, ErrSemAnterior) {
		// O eprocia.exe já é a versão nova: abre na próxima vez.
		a.fase, a.mensagem = FaseInstalada, MsgAbreNaProxima
	} else {
		a.fase, a.mensagem = FaseErro, a.fraseErro(err)
		a.instalada = false // a versão instalada se perdeu na reversão
		a.versao = a.consulta.Versao
	}
	ev = a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
	log.Error("reinício da atualização", "resultado", classeErro(err), "modo", string(modo))
}

// Cancelar: durante o download, volta a disponivel; em aguardando_fila do
// modo instalador, apaga o baixado e volta a disponivel; em aguardando_fila
// do modo trocar, volta a instalada sem tocar em arquivo. Nas outras fases,
// nada.
func (a *Atualizador) Cancelar() {
	a.mu.Lock()
	var p pendente
	switch {
	case a.fase == FaseBaixando || a.fase == FaseConferindo:
		// A goroutine do download apaga o que o Baixar deixar.
		a.fase, a.versao = FaseDisponivel, a.consulta.Versao
		a.pendente = pendente{}
	case a.fase == FaseAguardandoFila && a.instalada:
		a.fase = FaseInstalada
	case a.fase == FaseAguardandoFila:
		p, a.pendente, a.baixado = a.pendente, pendente{}, Baixado{}
		a.fase, a.versao = FaseDisponivel, a.consulta.Versao
	default:
		a.mu.Unlock()
		return
	}
	a.gen++
	if a.cancelar != nil {
		a.cancelar()
	}
	a.progresso, a.mensagem = 0, ""
	ev := a.marcarLocked()
	a.mu.Unlock()
	a.publicar(ev)
	p.apagar()
}

// Encerrar é chamado no shutdown (e pode vir duas vezes: a segunda volta na
// hora). Cancela a operação em curso, apaga o baixado não entregue e trava
// o trocaMu para sempre: espera um Trocador.Instalar ou uma reversão em
// curso e impede que outra comece. Não espera mais nada: em reiniciando, o
// Reiniciar (sem cancelamento) segue sem renomear, e o processo novo espera
// este sair. Da entrega em diante, nada é apagado nem renomeado.
func (a *Atualizador) Encerrar() {
	a.encerrar.Do(func() {
		a.mu.Lock()
		a.encerrado = true
		if a.cancelar != nil {
			a.cancelar()
		}
		p := a.pendente
		a.pendente = pendente{}
		a.mu.Unlock()
		p.apagar()
		if a.antesDeTravarTroca != nil {
			a.antesDeTravarTroca()
		}
		a.trocaMu.Lock() // nunca solto
	})
}

// fraseErro traduz o erro de uma troca, do download, do Reabrir ou do shell
// pela classe, nunca pelo texto (que carrega caminho ou URL).
func (a *Atualizador) fraseErro(err error) string {
	if e, ok := errors.AsType[ErrNaoAbriu](err); ok {
		return MsgNaoAbriu(e.Versao, a.atual)
	}
	switch {
	case errors.Is(err, ErrReversaoIncompleta):
		slog.Default().Error("atualização: a reversão da troca ficou incompleta; reinstalar o eprocIA")
		return MsgReversaoIncompleta
	case errors.Is(err, ErrNaoConferiu):
		return MsgNaoConferiu
	case errors.Is(err, ErrParado):
		return MsgInterrompido
	case errors.Is(err, ErrCancelado):
		return MsgInstalacaoCancelada(a.atual)
	}
	return MsgNaoAplicou(a.atual)
}

// classeErro é o nome fixo da classe de um erro, para o log.
func classeErro(err error) string {
	if _, ok := errors.AsType[ErrNaoAbriu](err); ok {
		return "nao_abriu"
	}
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrReversaoIncompleta):
		return "reversao_incompleta"
	case errors.Is(err, ErrNaoConferiu):
		return "nao_conferiu"
	case errors.Is(err, ErrSemAnterior):
		return "sem_anterior"
	case errors.Is(err, ErrNaoAplicou):
		return "nao_aplicou"
	case errors.Is(err, ErrParado):
		return "parado"
	case errors.Is(err, ErrCancelado):
		return "cancelado"
	case errors.Is(err, ErrRecusada):
		return "recusada"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "interrompida"
	}
	return "erro"
}
