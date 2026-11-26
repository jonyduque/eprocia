package registro

import (
	"bufio"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// TamanhoCanal é quantas linhas formatadas esperam pela gravadora.
	TamanhoCanal = 4096
	// IntervaloDescarga é de quanto em quanto tempo o buffer vai ao disco.
	IntervaloDescarga = time.Second
	// EsperaAvisoCheio é quanto WARN e ERROR esperam com o canal cheio.
	EsperaAvisoCheio = 50 * time.Millisecond
	// PausaAposFalha é quanto a gravadora fica sem gravar depois de uma falha.
	PausaAposFalha = 60 * time.Second
)

// msgPerdidas é a mensagem da linha que conta as linhas descartadas.
const msgPerdidas = "linhas de log perdidas"

// gravadoresVivos conta as goroutines gravadoras em execução, para os testes
// confirmarem que nenhuma vaza.
var gravadoresVivos atomic.Int64

// configGravador monta uma gravadora; intervalo zero vale IntervaloDescarga.
type configGravador struct {
	destino   io.Writer
	fechar    func() error
	agora     func() time.Time
	saneador  *Saneador
	intervalo time.Duration
}

// gravador é o lado assíncrono do log: os manipuladores formatam a linha e a
// põem no canal; uma única goroutine (rodar) a grava. Só essa goroutine toca
// buf, destino e os campos marcados abaixo.
type gravador struct {
	canal     chan []byte
	encerrado atomic.Bool
	// perdidas conta as linhas descartadas ainda não registradas no arquivo.
	perdidas atomic.Int64

	agora    func() time.Time
	saneador *Saneador

	pedidos chan chan struct{}
	parar   chan struct{}
	feito   chan struct{}
	umaVez  sync.Once

	// Só da goroutine gravadora.
	destino          io.Writer
	fechar           func() error
	buf              *bufio.Writer
	intervalo        time.Duration
	pausaAte         time.Time
	linhasNoBuffer   int64 // linhas do canal no buffer ainda não descarregadas
	perdidasNoBuffer int64 // soma das linhas_perdidas no buffer
	avisoGiro        error // falha de giro a registrar
}

func novoGravador(c configGravador) *gravador {
	if c.agora == nil {
		c.agora = time.Now
	}
	if c.intervalo <= 0 {
		c.intervalo = IntervaloDescarga
	}
	g := &gravador{
		canal:     make(chan []byte, TamanhoCanal),
		agora:     c.agora,
		saneador:  c.saneador,
		pedidos:   make(chan chan struct{}),
		parar:     make(chan struct{}),
		feito:     make(chan struct{}),
		destino:   c.destino,
		fechar:    c.fechar,
		buf:       bufio.NewWriterSize(c.destino, 64*1024),
		intervalo: c.intervalo,
	}
	if a, ok := c.destino.(*ArquivoRotativo); ok {
		a.aoFalharGiro = func(err error) { g.avisoGiro = err }
	}
	gravadoresVivos.Add(1)
	go g.rodar()
	return g
}

// enviar põe a linha no canal sem bloquear: DEBUG e INFO são descartadas na
// hora com o canal cheio; WARN e ERROR esperam até EsperaAvisoCheio.
// Uma linha vazia (formatação que falhou) é descartada sem contar.
func (g *gravador) enviar(b []byte, nivel slog.Level) {
	if len(b) == 0 || g.encerrado.Load() {
		return
	}
	select {
	case g.canal <- b:
		return
	default:
	}
	if nivel < slog.LevelWarn {
		g.perdidas.Add(1)
		return
	}
	t := time.NewTimer(EsperaAvisoCheio)
	defer t.Stop()
	select {
	case g.canal <- b:
	case <-t.C:
		g.perdidas.Add(1)
	}
}

// tentarEnviar é o gancho de teste: envia sem esperar e sem contar perda.
func (g *gravador) tentarEnviar(b []byte) bool {
	select {
	case g.canal <- b:
		return true
	default:
		return false
	}
}

// descarregar grava no destino tudo o que já chegou ao canal. Depois de
// encerrar, não faz nada.
func (g *gravador) descarregar() {
	resp := make(chan struct{})
	select {
	case g.pedidos <- resp:
		<-resp
	case <-g.feito:
	}
}

// encerrar descarta o que chegar daqui em diante, grava o que já está no
// canal, descarrega e fecha o destino. Idempotente. O canal nunca é fechado.
func (g *gravador) encerrar() {
	g.encerrado.Store(true)
	g.umaVez.Do(func() { close(g.parar) })
	<-g.feito
}

func (g *gravador) rodar() {
	defer close(g.feito)
	defer gravadoresVivos.Add(-1)
	tique := time.NewTicker(g.intervalo)
	defer tique.Stop()
	for {
		select {
		case b := <-g.canal:
			g.gravar(b)
		case <-tique.C:
			g.descarregarBuffer()
		case resp := <-g.pedidos:
			g.drenar()
			g.descarregarBuffer()
			close(resp)
		case <-g.parar:
			g.drenar()
			if g.retomar() {
				g.escreverPerdidas()
			}
			g.descarregarBuffer()
			if g.fechar != nil {
				_ = g.fechar()
			}
			return
		}
	}
}

func (g *gravador) drenar() {
	for {
		select {
		case b := <-g.canal:
			g.gravar(b)
		default:
			return
		}
	}
}

// retomar diz se a gravação está liberada. Ao fim de uma pausa, recomeça o
// bufio.Writer, que guarda o primeiro erro para sempre.
func (g *gravador) retomar() bool {
	if g.pausaAte.IsZero() {
		return true
	}
	if g.agora().Before(g.pausaAte) {
		return false
	}
	g.pausaAte = time.Time{}
	g.buf.Reset(g.destino)
	return true
}

func (g *gravador) gravar(b []byte) {
	if !g.retomar() {
		g.perdidas.Add(1)
		return
	}
	if !g.escreverPerdidas() {
		g.perdidas.Add(1)
		return
	}
	g.linhasNoBuffer++
	if _, err := g.buf.Write(b); err != nil {
		g.falhou()
		return
	}
	g.escreverAvisoGiro()
}

// escreverPerdidas grava, antes da próxima linha, o WARN com o total de
// linhas descartadas. Devolve false se a gravação falhou.
func (g *gravador) escreverPerdidas() bool {
	n := g.perdidas.Swap(0)
	if n == 0 {
		return true
	}
	g.perdidasNoBuffer += n
	if _, err := g.buf.Write(g.linhaInterna(slog.LevelWarn, msgPerdidas, slog.Int64("linhas_perdidas", n))); err != nil {
		g.falhou()
		return false
	}
	return true
}

// escreverAvisoGiro registra, uma vez por sequência de falhas, que o arquivo
// não pôde girar (aberto em outro programa, por exemplo).
func (g *gravador) escreverAvisoGiro() {
	if g.avisoGiro == nil {
		return
	}
	err := g.avisoGiro
	g.avisoGiro = nil
	if _, e := g.buf.Write(g.linhaInterna(slog.LevelWarn, "falha ao girar o arquivo de log", slog.String("erro", err.Error()))); e != nil {
		g.falhou()
	}
}

func (g *gravador) descarregarBuffer() {
	if !g.retomar() {
		return
	}
	if g.perdidas.Load() > 0 && !g.escreverPerdidas() {
		return
	}
	if g.buf.Buffered() == 0 {
		return
	}
	if err := g.buf.Flush(); err != nil {
		g.falhou()
		return
	}
	g.linhasNoBuffer, g.perdidasNoBuffer = 0, 0
	g.escreverAvisoGiro()
}

// falhou começa a pausa: as linhas no buffer contam como perdidas.
func (g *gravador) falhou() {
	g.perdidas.Add(g.linhasNoBuffer + g.perdidasNoBuffer)
	g.linhasNoBuffer, g.perdidasNoBuffer = 0, 0
	g.pausaAte = g.agora().Add(PausaAposFalha)
}

// linhaInterna formata uma linha da própria gravadora, com o relógio dela.
func (g *gravador) linhaInterna(nivel slog.Level, msg string, attrs ...slog.Attr) []byte {
	r := slog.NewRecord(g.agora(), nivel, msg, 0)
	r.AddAttrs(attrs...)
	return formatar(g.saneador, nil, r)
}
