package historico

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// renomear troca o temporário pelo arquivo final; variável para o teste
// simular falha.
var renomear = os.Rename

// esperaRenomear é a espera antes da nova tentativa de rename.
var esperaRenomear = 200 * time.Millisecond

// aoGravarTeste, quando não nil, é chamado depois de cada rename que deu
// certo (contador dos testes).
var aoGravarTeste func()

// atrasoPadrao é o debounce de Agendar quando Opcoes.Atraso é zero.
const atrasoPadrao = time.Second

// Opcoes configura o Gravador.
type Opcoes struct {
	// Protegido: o arquivo atual é de uma versão mais nova do formato; listas
	// vazias são ignoradas até a primeira lista não vazia.
	Protegido bool
	// AoFalhar é chamado (fora do mutex) quando uma escrita falha.
	AoFalhar func(error)
	// Atraso é o debounce de Agendar; zero vale 1 s.
	Atraso time.Duration
}

// Gravador grava o histórico com debounce e escrita atômica (temporário na
// mesma pasta e rename por cima). Um mutex serializa escrita, rename e
// mudança de estado. Fechado, não grava nada até Reabrir.
type Gravador struct {
	arquivo  string
	atraso   time.Duration
	aoFalhar func(error)

	mu        sync.Mutex
	protegido bool
	fechado   bool
	pendente  []Item
	timer     *time.Timer
	geracao   uint64 // invalida um timer já disparado e ainda sem o mutex
	avisando  bool   // AoFalhar em curso: uma falha dentro dele não o chama de novo
}

// NovoGravador cria o gravador do arquivo, sem gravar nada.
func NovoGravador(arquivo string, o Opcoes) *Gravador {
	atraso := o.Atraso
	if atraso <= 0 {
		atraso = atrasoPadrao
	}
	apagarTemporarios(filepath.Dir(arquivo))
	return &Gravador{arquivo: arquivo, atraso: atraso, aoFalhar: o.AoFalhar, protegido: o.Protegido}
}

// apagarTemporarios remove os historico-*.tmp que uma escrita interrompida
// (queda de energia, processo encerrado) deixou na pasta.
func apagarTemporarios(pasta string) {
	sobras, _ := filepath.Glob(filepath.Join(pasta, "historico-*.tmp"))
	for _, f := range sobras {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("não foi possível apagar um temporário do histórico", "erro", err)
		}
	}
}

// aceitaLocked diz se a lista deve ser gravada e, com uma lista não vazia,
// tira a proteção. Chamar com g.mu.
func (g *Gravador) aceitaLocked(itens []Item) bool {
	if g.protegido {
		if len(itens) == 0 {
			return false
		}
		g.protegido = false
	}
	return true
}

// cancelarLocked desfaz o timer pendente. Chamar com g.mu.
func (g *Gravador) cancelarLocked() {
	g.geracao++
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
	g.pendente = nil
}

// Agendar guarda a lista e a grava no máximo um Atraso depois da primeira
// chamada ainda não gravada: com timer pendente, só troca a lista (não rearma),
// para uma rajada contínua não adiar a escrita sem limite. Chamadas dentro do
// Atraso geram uma escrita só, com a última lista.
func (g *Gravador) Agendar(itens []Item) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.fechado || !g.aceitaLocked(itens) {
		return
	}
	g.pendente = append([]Item{}, itens...)
	if g.timer != nil {
		return
	}
	gen := g.geracao
	g.timer = time.AfterFunc(g.atraso, func() { g.disparar(gen) })
}

// disparar grava a lista pendente, se o timer ainda for o vigente.
func (g *Gravador) disparar(gen uint64) {
	g.mu.Lock()
	if g.fechado || gen != g.geracao {
		g.mu.Unlock()
		return
	}
	itens := g.pendente
	g.pendente, g.timer = nil, nil
	err := g.escreverLocked(itens)
	g.mu.Unlock()
	g.avisar(err)
}

// GravarAgora grava a lista já, descartando um Agendar pendente. Fechado,
// volta nil sem gravar.
func (g *Gravador) GravarAgora(itens []Item) error {
	g.mu.Lock()
	if g.fechado || !g.aceitaLocked(itens) {
		g.mu.Unlock()
		return nil
	}
	g.cancelarLocked()
	err := g.escreverLocked(itens)
	g.mu.Unlock()
	g.avisar(err)
	return err
}

// Fechar grava a lista final e fecha o gravador, mesmo quando a escrita
// falha. Já fechado, volta nil sem gravar.
func (g *Gravador) Fechar(itens []Item) error {
	g.mu.Lock()
	if g.fechado {
		g.mu.Unlock()
		return nil
	}
	g.cancelarLocked()
	g.fechado = true
	if !g.aceitaLocked(itens) {
		g.mu.Unlock()
		return nil
	}
	err := g.escreverLocked(itens)
	g.mu.Unlock()
	g.avisar(err)
	return err
}

// FecharEApagar fecha o gravador (descartando o pendente) e apaga o arquivo.
// Vale também com o gravador já fechado.
func (g *Gravador) FecharEApagar() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancelarLocked()
	g.fechado = true
	if err := os.Remove(g.arquivo); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("não foi possível apagar o histórico", "erro", err)
		return err
	}
	return nil
}

// Reabrir volta a aceitar gravações depois de Fechar; não grava nada.
func (g *Gravador) Reabrir() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fechado = false
	return nil
}

// avisar registra a falha e chama AoFalhar, fora do mutex. Uma falha que
// acontece enquanto AoFalhar roda (ele chamou Agendar ou GravarAgora, ou outra
// goroutine falhou ao mesmo tempo) só vai para o log: sem isso, um AoFalhar
// que grava de novo com o disco falhando entraria em recursão sem fim.
func (g *Gravador) avisar(err error) {
	if err == nil {
		return
	}
	slog.Warn("não foi possível gravar o histórico", "erro", err)
	if g.aoFalhar == nil {
		return
	}
	g.mu.Lock()
	if g.avisando {
		g.mu.Unlock()
		return
	}
	g.avisando = true
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.avisando = false
		g.mu.Unlock()
	}()
	g.aoFalhar(err)
}

// escreverLocked grava {versao, itens} num temporário da pasta e o renomeia
// por cima do arquivo, com uma nova tentativa do rename. Falhando, apaga o
// temporário. Chamar com g.mu.
func (g *Gravador) escreverLocked(itens []Item) error {
	if itens == nil {
		itens = []Item{}
	}
	dados, err := json.MarshalIndent(arquivoHistorico{Versao: VersaoFormato, Itens: Podar(itens, Teto)}, "", "  ")
	if err != nil {
		return err
	}
	pasta := filepath.Dir(g.arquivo)
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(pasta, "historico-*.tmp")
	if err != nil {
		return err
	}
	nomeTmp := tmp.Name()
	_, err = tmp.Write(dados)
	if err == nil {
		err = tmp.Sync()
	}
	if errFechar := tmp.Close(); err == nil {
		err = errFechar
	}
	if err == nil {
		if err = renomear(nomeTmp, g.arquivo); err != nil {
			time.Sleep(esperaRenomear)
			err = renomear(nomeTmp, g.arquivo)
		}
	}
	if err != nil {
		_ = os.Remove(nomeTmp)
		return err
	}
	if aoGravarTeste != nil {
		aoGravarTeste()
	}
	return nil
}
