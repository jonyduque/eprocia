package atualizacao

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"eprocia/internal/registro"
)

const (
	// prazoNovoSair é quanto a reversão espera o processo novo encerrado sair.
	prazoNovoSair = 10 * time.Second
	// intervaloSinal é o passo da sondagem do arquivo de sinal.
	intervaloSinal = 200 * time.Millisecond
)

// ErrNaoAbriu: o processo novo (Versao) arrancou, ou tentou arrancar, e não
// deu sinal de vida; os nomes já foram revertidos.
type ErrNaoAbriu struct{ Versao string }

func (e ErrNaoAbriu) Error() string { return "a versão " + e.Versao + " não abriu" }

// ErrReversaoIncompleta: uma reversão não conseguiu restaurar os nomes.
var ErrReversaoIncompleta = errors.New("a reversão da troca ficou incompleta")

// ErrNaoAplicou: a troca falhou antes de o processo novo arrancar e nada
// mudou. Embrulha o erro de origem, que carrega caminhos: o texto do erro
// nunca vai para o log nem para a tela (a tela usa MsgNaoAplicou).
var ErrNaoAplicou = errors.New("a atualização não foi aplicada")

// ErrSemAnterior: Reiniciar não achou o eprocia.anterior.exe e não iniciou
// nada (não haveria como reverter). O eprocia.exe já é a versão nova, que
// abre na próxima vez que o eprocIA for aberto.
var ErrSemAnterior = errors.New("não há eprocia.anterior.exe para reverter")

// Novas tentativas de rename e remoção (comTentativas): logo depois de um
// processo sair, o Windows ainda pode segurar a imagem dele por um instante.
// São variáveis só para o teste encurtar a pausa.
var (
	tentativasArquivo = 10
	pausaTentativa    = 200 * time.Millisecond
)

// Troca instala a versão baixada na pasta da instalação por usuário (Pasta)
// em dois passos. Instalar põe o baixado no lugar do eprocia.exe (o em
// execução vira o eprocia.anterior.exe) sem iniciar nada. Reiniciar inicia o
// eprocia.exe novo com --apos-atualizar=<PIDAtual>, --limpar-anterior e
// Extras, e espera o sinal de vida em PastaDados até PrazoSinal (zero = o
// PrazoSinal do pacote); sem sinal, reverte os nomes.
//
// Erros: ErrNaoConferiu = a reconferência do baixado falhou (nada mudou, o
// baixado foi apagado); ErrNaoAplicou = o Instalar falhou com os renames
// desfeitos (nada mudou); ErrSemAnterior = Reiniciar sem eprocia.anterior.exe
// (nada iniciado); ErrNaoAbriu = o novo não deu sinal e os nomes voltaram;
// ErrReversaoIncompleta = um rename que desfazia falhou (antes, a troca
// tenta deixar um eprocia.exe na pasta por cópia). Os erros carregam
// caminhos: o texto deles nunca vai para o log nem para a tela; o log recebe
// texto fixo e o usuário, as Msg* por classe.
//
// Trava (nil = sem trava) é segurada pela reversão do Reiniciar do primeiro
// ao último rename. Instalar não a trava: quem chama Instalar já segura a
// mesma trava durante a chamada inteira, o que cobre o desfazer dela, e
// travar de novo seria impasse (sync.Mutex não é reentrante).
type Troca struct {
	Pasta, PastaDados string
	PIDAtual          int
	Extras            []string
	PrazoSinal        time.Duration
	Trava             sync.Locker
	// renomear é trocado nos testes; nil = os.Rename.
	renomear func(de, para string) error
}

func (t *Troca) ren(de, para string) error {
	if t.renomear != nil {
		return t.renomear(de, para)
	}
	return os.Rename(de, para)
}

// renRepetindo renomeia com novas tentativas em erro de compartilhamento.
func (t *Troca) renRepetindo(de, para string) error {
	return comTentativas(func() error { return t.ren(de, para) })
}

// comTentativas repete op enquanto ela falhar por violação de compartilhamento
// (32) ou acesso negado (5), até tentativasArquivo vezes (no mínimo uma), com
// pausaTentativa entre elas, e devolve o último erro; outro erro volta na hora.
func comTentativas(op func() error) error {
	var err error
	for i := range max(1, tentativasArquivo) {
		if i > 0 {
			time.Sleep(pausaTentativa)
		}
		err = op()
		if err == nil || !repetivel(err) {
			return err
		}
	}
	return err
}

// repetivel: ERROR_SHARING_VIOLATION (32) e ERROR_ACCESS_DENIED (5), pelo
// número, para o pacote compilar fora do Windows.
func repetivel(err error) bool {
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(5))
}

func (t *Troca) caminho(nome string) string { return filepath.Join(t.Pasta, nome) }

// Instalar põe o baixado b no lugar do eprocia.exe (que vira o
// eprocia.anterior.exe). Não inicia processo nem espera sinal: nil = a versão
// nova está em eprocia.exe e a em execução, em eprocia.anterior.exe.
func (t *Troca) Instalar(ctx context.Context, b Baixado) error {
	log := registro.Logger(ctx)
	exe, anterior := t.caminho(NomeExe), t.caminho(NomeAnterior)

	// O baixado pode ter esperado a fila por horas na pasta de instalação.
	if err := reconferir(b); err != nil {
		_ = os.Remove(b.Caminho)
		log.Error("troca do executável: o baixado não conferiu de novo; apagado")
		return err
	}
	if err := os.Remove(anterior); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error("troca do executável: não apagou o eprocia.anterior.exe velho", atributosErro(err)...)
		return fmt.Errorf("%w: apagar o eprocia.anterior.exe velho: %w", ErrNaoAplicou, err)
	}
	if err := t.ren(exe, anterior); err != nil {
		log.Error("troca do executável: não renomeou o eprocia.exe", atributosErro(err)...)
		return fmt.Errorf("%w: renomear o eprocia.exe: %w", ErrNaoAplicou, err)
	}
	if err := t.ren(b.Caminho, exe); err != nil {
		log.Error("troca do executável: não pôs o baixado no lugar; desfazendo", atributosErro(err)...)
		if errDesfazer := t.ren(anterior, exe); errDesfazer != nil {
			log.Error("troca do executável: não desfez o rename do eprocia.exe; copiando o anterior ou o baixado", atributosErro(errDesfazer)...)
			t.reporExe(log, anterior, b.Caminho)
			return ErrReversaoIncompleta
		}
		return fmt.Errorf("%w: pôr o baixado no lugar: %w", ErrNaoAplicou, err)
	}
	log.Info("troca do executável", "resultado", "instalada", "versao", b.Versao.String())
	return nil
}

// Reiniciar inicia o eprocia.exe já instalado (a versão nova) e espera o
// sinal. nil = o sinal chegou e o chamador encerra o processo antigo; o
// eprocia.anterior.exe fica para o processo novo apagar depois de o antigo
// sair. Sem sinal, reverte.
func (t *Troca) Reiniciar(ctx context.Context, nova Versao) error {
	if _, err := os.Lstat(t.caminho(NomeAnterior)); err != nil {
		registro.Logger(ctx).Error("reinício da troca: sem eprocia.anterior.exe; nada iniciado", atributosErro(err)...)
		return ErrSemAnterior
	}
	return t.iniciarEEsperar(ctx, nova.String())
}

// copiarParaExe copia origem para eprocia.exe por um temporário da mesma
// pasta (abrir só leitura funciona mesmo com o exe em execução); o rename final
// repete em erro de compartilhamento (antivírus segurando o exe recém-gravado).
func (t *Troca) copiarParaExe(origem string) (errCopia error) {
	o, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer o.Close()
	tmp := t.caminho(PrefixoTemp + rand.Text())
	d, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	defer func() {
		if errCopia != nil {
			_ = os.Remove(tmp)
		}
	}()
	_, err = io.Copy(d, o)
	if errFechar := d.Close(); err == nil {
		err = errFechar
	}
	if err != nil {
		return err
	}
	return t.renRepetindo(tmp, t.caminho(NomeExe))
}

// reporExe tenta, depois de um rename que desfazia falhar, deixar um
// eprocia.exe na pasta copiando cada origem, na ordem. Devolve se conseguiu.
func (t *Troca) reporExe(log *slog.Logger, origens ...string) bool {
	for _, origem := range origens {
		err := t.copiarParaExe(origem)
		if err == nil {
			log.Warn("troca do executável: eprocia.exe reposto por cópia", "origem", filepath.Base(origem))
			return true
		}
		log.Error("troca do executável: a cópia não repôs o eprocia.exe", append([]any{"origem", filepath.Base(origem)}, atributosErro(err)...)...)
	}
	log.Error("troca do executável: a pasta ficou sem eprocia.exe")
	return false
}

// reconferir reabre o baixado só leitura e confere de novo.
func reconferir(b Baixado) error {
	f, err := os.Open(b.Caminho)
	if err != nil {
		return fmt.Errorf("%w: reabrir o baixado", ErrNaoConferiu)
	}
	defer f.Close()
	return conferir(b, f)
}

// iniciarEEsperar inicia o eprocia.exe já trocado, com --apos-atualizar,
// --limpar-anterior e Extras, nessa ordem, e espera o sinal; sem sinal,
// encerra o novo e reverte.
func (t *Troca) iniciarEEsperar(ctx context.Context, versao string) error {
	log := registro.Logger(ctx)
	sinal := CaminhoSinal(t.PastaDados, t.PIDAtual)
	// Um sinal esquecido de outra troca passaria por sinal do novo.
	_ = os.Remove(sinal)

	args := append([]string{FlagAposAtualizar + strconv.Itoa(t.PIDAtual), FlagLimparAnterior}, t.Extras...)
	cmd := exec.Command(t.caminho(NomeExe), args...)
	cmd.SysProcAttr = atributosFilho()
	if err := cmd.Start(); err != nil {
		log.Error("troca do executável: o processo novo não iniciou; revertendo", atributosErro(err)...)
		return t.reverter(log, versao)
	}
	fim := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(fim)
	}()

	if t.esperarSinal(ctx, sinal, fim) {
		log.Info("troca do executável", "resultado", "sinal_recebido", "versao", versao)
		return nil
	}
	_ = cmd.Process.Kill()
	select {
	case <-fim:
	case <-time.After(prazoNovoSair):
		log.Warn("troca do executável: o processo novo não saiu depois de encerrado")
	}
	_ = os.Remove(sinal)
	return t.reverter(log, versao)
}

// esperarSinal sonda o sinal até ele chegar (verdadeiro), o processo novo
// sair, o prazo vencer ou o contexto acabar. No build de ensaio com
// EPROCIA_ENSAIO_SEM_SINAL=1 não lê o sinal e segue até o prazo.
func (t *Troca) esperarSinal(ctx context.Context, sinal string, fim <-chan struct{}) bool {
	prazo := t.PrazoSinal
	if prazo <= 0 {
		prazo = PrazoSinal
	}
	ignorar := ensaioIgnoraSinal()
	limite := time.NewTimer(prazo)
	defer limite.Stop()
	passo := time.NewTicker(intervaloSinal)
	defer passo.Stop()
	for {
		if !ignorar {
			if _, ok := lerSinal(sinal); ok {
				return true
			}
		}
		select {
		case <-fim:
			return false
		case <-limite.C:
			return false
		case <-ctx.Done():
			return false
		case <-passo.C:
		}
	}
}

// reverter desfaz a troca depois que o novo não deu sinal: eprocia.exe vira
// eprocia.falhou.exe e eprocia.anterior.exe volta a ser eprocia.exe. Segura
// a Trava do primeiro ao último rename, e cada rename tem novas tentativas.
func (t *Troca) reverter(log *slog.Logger, versao string) error {
	if t.Trava != nil {
		t.Trava.Lock()
		defer t.Trava.Unlock()
	}
	exe, anterior, falhou := t.caminho(NomeExe), t.caminho(NomeAnterior), t.caminho(NomeFalhou)
	if err := comTentativas(func() error { return os.Remove(falhou) }); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Warn("reversão da troca: não apagou o eprocia.falhou.exe velho", atributosErro(err)...)
	}
	if err := t.renRepetindo(exe, falhou); err != nil {
		// O eprocia.exe (o novo) continua no lugar.
		log.Error("reversão da troca: não renomeou o eprocia.exe novo", atributosErro(err)...)
		return ErrReversaoIncompleta
	}
	if err := t.renRepetindo(anterior, exe); err != nil {
		// Ordem: cópia do anterior; senão o novo de volta pelo nome; senão
		// cópia do novo.
		log.Error("reversão da troca: não restaurou o eprocia.exe anterior; copiando", atributosErro(err)...)
		if t.reporExe(log, anterior) {
			return ErrReversaoIncompleta
		}
		errRepor := t.renRepetindo(falhou, exe)
		if errRepor == nil {
			log.Warn("reversão da troca: o eprocia.exe novo voltou ao lugar")
			return ErrReversaoIncompleta
		}
		log.Error("reversão da troca: não repôs o eprocia.exe novo", atributosErro(errRepor)...)
		t.reporExe(log, falhou)
		return ErrReversaoIncompleta
	}
	log.Warn("troca do executável", "resultado", "revertida_sem_sinal", "versao", versao)
	return ErrNaoAbriu{Versao: versao}
}

// atributosErro descreve um erro de arquivo ou processo sem o texto dele, que
// carrega caminhos: só o tipo e o código do sistema, quando houver.
func atributosErro(err error) []any {
	attrs := []any{"tipo", fmt.Sprintf("%T", err)}
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		attrs = append(attrs, "codigo", uint64(errno))
	}
	return attrs
}
