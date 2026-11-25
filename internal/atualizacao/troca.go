package atualizacao

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// Troca troca o executável da instalação por usuário (Pasta) e inicia o novo
// com --apos-atualizar=<PIDAtual> mais Extras, esperando o sinal de vida em
// PastaDados até PrazoSinal (zero = o PrazoSinal do pacote).
//
// Erros: nil = o sinal chegou e o chamador encerra o processo antigo;
// ErrNaoConferiu = a reconferência do baixado falhou (nada mudou, o baixado
// foi apagado); ErrNaoAbriu = o novo não deu sinal e os nomes voltaram;
// ErrReversaoIncompleta = uma reversão falhou; qualquer outro erro vem de
// antes de o processo novo arrancar, com os renames desfeitos (nada mudou).
// Os erros carregam caminhos: o log e o usuário recebem só texto fixo.
type Troca struct {
	Pasta, PastaDados string
	PIDAtual          int
	Extras            []string
	PrazoSinal        time.Duration
	// renomear é trocado nos testes; nil = os.Rename.
	renomear func(de, para string) error
}

func (t *Troca) ren(de, para string) error {
	if t.renomear != nil {
		return t.renomear(de, para)
	}
	return os.Rename(de, para)
}

func (t *Troca) caminho(nome string) string { return filepath.Join(t.Pasta, nome) }

// Aplicar põe o baixado b no lugar do eprocia.exe (que vira o
// eprocia.anterior.exe), inicia o novo e espera o sinal.
func (t *Troca) Aplicar(ctx context.Context, b Baixado) error {
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
		return fmt.Errorf("apagar o eprocia.anterior.exe velho: %w", err)
	}
	if err := t.ren(exe, anterior); err != nil {
		log.Error("troca do executável: não renomeou o eprocia.exe", atributosErro(err)...)
		return fmt.Errorf("renomear o eprocia.exe: %w", err)
	}
	if err := t.ren(b.Caminho, exe); err != nil {
		log.Error("troca do executável: não pôs o baixado no lugar; desfazendo", atributosErro(err)...)
		if errDesfazer := t.ren(anterior, exe); errDesfazer != nil {
			log.Error("troca do executável: não desfez o rename do eprocia.exe; a pasta ficou sem eprocia.exe", atributosErro(errDesfazer)...)
			return ErrReversaoIncompleta
		}
		return fmt.Errorf("pôr o baixado no lugar: %w", err)
	}
	return t.iniciarEEsperar(ctx, b.Versao.String())
}

// Voltar troca o eprocia.exe pelo eprocia.anterior.exe (que vira o
// eprocia.exe atual), inicia a versão anterior e espera o sinal.
func (t *Troca) Voltar(ctx context.Context) error {
	log := registro.Logger(ctx)
	exe, anterior := t.caminho(NomeExe), t.caminho(NomeAnterior)
	versao := ""
	if v, err := VersaoPE(anterior); err == nil {
		versao = v.String()
	}
	tmp := t.caminho(PrefixoTemp + rand.Text())

	if err := t.ren(anterior, tmp); err != nil {
		log.Error("voltar versão: não renomeou o eprocia.anterior.exe", atributosErro(err)...)
		return fmt.Errorf("renomear o eprocia.anterior.exe: %w", err)
	}
	if err := t.ren(exe, anterior); err != nil {
		log.Error("voltar versão: não renomeou o eprocia.exe; desfazendo", atributosErro(err)...)
		if errDesfazer := t.ren(tmp, anterior); errDesfazer != nil {
			log.Error("voltar versão: não restaurou o eprocia.anterior.exe", atributosErro(errDesfazer)...)
			return ErrReversaoIncompleta
		}
		return fmt.Errorf("renomear o eprocia.exe: %w", err)
	}
	if err := t.ren(tmp, exe); err != nil {
		log.Error("voltar versão: não pôs a versão anterior no lugar; desfazendo", atributosErro(err)...)
		if errDesfazer := t.ren(anterior, exe); errDesfazer != nil {
			log.Error("voltar versão: não restaurou o eprocia.exe; a pasta ficou sem eprocia.exe", atributosErro(errDesfazer)...)
			return ErrReversaoIncompleta
		}
		if errDesfazer := t.ren(tmp, anterior); errDesfazer != nil {
			log.Error("voltar versão: não restaurou o eprocia.anterior.exe", atributosErro(errDesfazer)...)
			return ErrReversaoIncompleta
		}
		return fmt.Errorf("pôr a versão anterior no lugar: %w", err)
	}
	return t.iniciarEEsperar(ctx, versao)
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

// iniciarEEsperar inicia o eprocia.exe já trocado e espera o sinal; sem
// sinal, encerra o novo e reverte.
func (t *Troca) iniciarEEsperar(ctx context.Context, versao string) error {
	log := registro.Logger(ctx)
	sinal := CaminhoSinal(t.PastaDados, t.PIDAtual)
	// Um sinal esquecido de outra troca passaria por sinal do novo.
	_ = os.Remove(sinal)

	args := append([]string{FlagAposAtualizar + strconv.Itoa(t.PIDAtual)}, t.Extras...)
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
// eprocia.falhou.exe e eprocia.anterior.exe volta a ser eprocia.exe.
func (t *Troca) reverter(log *slog.Logger, versao string) error {
	exe, anterior, falhou := t.caminho(NomeExe), t.caminho(NomeAnterior), t.caminho(NomeFalhou)
	if err := os.Remove(falhou); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Warn("reversão da troca: não apagou o eprocia.falhou.exe velho", atributosErro(err)...)
	}
	if err := t.ren(exe, falhou); err != nil {
		// O eprocia.exe (o novo) continua no lugar.
		log.Error("reversão da troca: não renomeou o eprocia.exe novo", atributosErro(err)...)
		return ErrReversaoIncompleta
	}
	if err := t.ren(anterior, exe); err != nil {
		log.Error("reversão da troca: não restaurou o eprocia.exe anterior; repondo o novo", atributosErro(err)...)
		if errRepor := t.ren(falhou, exe); errRepor != nil {
			log.Error("reversão da troca: não repôs o eprocia.exe novo; a pasta ficou sem eprocia.exe", atributosErro(errRepor)...)
		}
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
