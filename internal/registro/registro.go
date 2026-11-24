package registro

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// LimiteLog é o tamanho a partir do qual eprocia.log gira.
	LimiteLog = 5 << 20
	// ArquivadosLog é quantos eprocia.N.log ficam guardados.
	ArquivadosLog = 5
	// IdadeMaximaLog é a idade a partir da qual um eprocia*.log é apagado.
	IdadeMaximaLog = 30 * 24 * time.Hour
)

// Opcoes configura Iniciar.
type Opcoes struct {
	// Valor é o de EPROCIA_LOG, lido uma vez por quem chama.
	Valor string
	// PastaDados é a pasta de dados do app (%AppData%\eprocIA); os logs ficam
	// em PastaDados\logs.
	PastaDados string
	// Saneador troca o perfil do usuário; nil usa SaneadorDoSistema.
	Saneador *Saneador
	// Agora é o relógio (retenção e pausa após falha); nil usa time.Now.
	Agora func() time.Time
	// Saida, só para teste, substitui o arquivo rotativo.
	Saida io.Writer
}

// estado é o registro do processo, guardado por Iniciar.
var estado struct {
	mu             sync.RWMutex
	g              *gravador
	h              *manipulador
	detalhado      bool
	pasta          string
	naoReconhecido bool
}

// Iniciar liga o log em arquivo conforme o valor de EPROCIA_LOG. Desligado,
// não cria nada e devolve (false, nil). Ligado, cria <PastaDados>\logs, apaga
// os logs com mais de 30 dias e abre eprocia.log. Uma falha de pasta ou de
// arquivo devolve (false, err) e deixa o registro desligado.
func Iniciar(o Opcoes) (ativo bool, err error) {
	Encerrar()
	modo, reconhecido := LerModo(o.Valor)
	if modo == ModoDesligado {
		return false, nil
	}
	if o.Saneador == nil {
		o.Saneador = SaneadorDoSistema()
	}
	if o.Agora == nil {
		o.Agora = time.Now
	}
	pasta := filepath.Join(o.PastaDados, "logs")
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return false, err
	}
	cfg := configGravador{destino: o.Saida, agora: o.Agora, saneador: o.Saneador}
	if o.Saida == nil {
		// A retenção não impede o log: um arquivo que não sai fica para a
		// próxima abertura.
		_ = ApagarAntigos(pasta, IdadeMaximaLog, o.Agora())
		a, err := AbrirRotativo(pasta, "eprocia", ".log", LimiteLog, ArquivadosLog)
		if err != nil {
			return false, err
		}
		cfg.destino, cfg.fechar = a, a.Close
	}
	nivel := slog.LevelInfo
	if modo == ModoDetalhado {
		nivel = slog.LevelDebug
	}
	g := novoGravador(cfg)

	// Outro Iniciar concorrente pode ter instalado um gravador depois do
	// Encerrar do começo: troca sob o lock e encerra o anterior fora dele.
	estado.mu.Lock()
	anterior := estado.g
	estado.g = g
	estado.h = g.manipulador(nivel)
	estado.detalhado = modo == ModoDetalhado
	estado.pasta = pasta
	estado.naoReconhecido = !reconhecido
	estado.mu.Unlock()
	if anterior != nil {
		anterior.encerrar()
	}
	return true, nil
}

// Ativo diz se o log em arquivo está ligado e aberto.
func Ativo() bool {
	estado.mu.RLock()
	defer estado.mu.RUnlock()
	return estado.g != nil
}

// Detalhado diz se o registro por página (DEBUG) está ligado.
func Detalhado() bool {
	estado.mu.RLock()
	defer estado.mu.RUnlock()
	return estado.g != nil && estado.detalhado
}

// Pasta é a pasta dos logs, vazia com o registro inativo.
func Pasta() string {
	estado.mu.RLock()
	defer estado.mu.RUnlock()
	if estado.g == nil {
		return ""
	}
	return estado.pasta
}

// Manipulador é o slog.Handler do log em arquivo, nil com o registro inativo.
func Manipulador() slog.Handler {
	estado.mu.RLock()
	defer estado.mu.RUnlock()
	if estado.h == nil {
		return nil
	}
	return estado.h
}

// ValorNaoReconhecido diz se EPROCIA_LOG tinha um valor desconhecido (que
// vale como 1); quem inicia o app registra o aviso.
func ValorNaoReconhecido() bool {
	estado.mu.RLock()
	defer estado.mu.RUnlock()
	return estado.g != nil && estado.naoReconhecido
}

// Descarregar grava no arquivo tudo o que já foi registrado.
func Descarregar() {
	estado.mu.RLock()
	g := estado.g
	estado.mu.RUnlock()
	if g != nil {
		g.descarregar()
	}
}

// Encerrar descarrega e fecha o arquivo; o que for registrado depois é
// descartado. Idempotente.
func Encerrar() {
	estado.mu.Lock()
	g := estado.g
	estado.g, estado.h = nil, nil
	estado.mu.Unlock()
	if g != nil {
		g.encerrar()
	}
}

// ResetarParaTeste encerra o registro e zera o estado do pacote.
func ResetarParaTeste() {
	Encerrar()
	estado.mu.Lock()
	defer estado.mu.Unlock()
	estado.detalhado, estado.pasta, estado.naoReconhecido = false, "", false
}

type chaveLogger struct{}

// ComLogger devolve um contexto que leva l, para Logger.
func ComLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, chaveLogger{}, l)
}

// Logger devolve o logger de ComLogger, ou slog.Default().
func Logger(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(chaveLogger{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}
