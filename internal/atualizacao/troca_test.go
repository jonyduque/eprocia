//go:build windows

package atualizacao

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var (
	v042 = Versao{0, 4, 2}
	v044 = Versao{0, 4, 4}
	v050 = Versao{0, 5, 0}
)

const (
	prazoComSinal = 30 * time.Second
	prazoSemSinal = time.Second
)

// cenarioTroca: pasta de instalação com eprocia.exe (0.4.4) em execução,
// iniciado pelo teste, e pasta de dados onde os auxiliares se registram.
type cenarioTroca struct {
	pasta, dados string
	antigo       *exec.Cmd
}

// novoCenarioTroca monta o cenário; modo é o EPROCIA_TESTE_AUX que os
// processos iniciados pela troca herdam. O cleanup encerra o antigo e espera
// (ou encerra) cada filho registrado na pasta de dados.
func novoCenarioTroca(t *testing.T, modo string) *cenarioTroca {
	t.Helper()
	c := &cenarioTroca{pasta: t.TempDir(), dados: t.TempDir()}
	exe := filepath.Join(c.pasta, NomeExe)
	copiarAuxiliar(t, exe, v044)
	c.antigo = exec.Command(exe)
	c.antigo.Env = append(os.Environ(), "EPROCIA_TESTE_AUX=dorme", "EPROCIA_TESTE_MS=120000", "EPROCIA_TESTE_DADOS=")
	if err := c.antigo.Start(); err != nil {
		t.Fatalf("iniciar o eprocia.exe antigo: %v", err)
	}
	t.Cleanup(func() {
		_ = c.antigo.Process.Kill()
		_ = c.antigo.Wait()
		for _, pid := range filhos(t, c.dados) {
			if EsperarSair(pid, 10*time.Second) {
				continue
			}
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
				EsperarSair(pid, 10*time.Second)
			}
		}
	})
	t.Setenv("EPROCIA_TESTE_AUX", modo)
	t.Setenv("EPROCIA_TESTE_DADOS", c.dados)
	return c
}

func (c *cenarioTroca) troca(prazo time.Duration) *Troca {
	return &Troca{Pasta: c.pasta, PastaDados: c.dados, PIDAtual: c.antigo.Process.Pid, PrazoSinal: prazo}
}

func (c *cenarioTroca) caminho(nome string) string { return filepath.Join(c.pasta, nome) }

// baixado grava na pasta de instalação um auxiliar carimbado v, como o
// Baixar deixaria, e devolve o Baixado com o SHA certo.
func (c *cenarioTroca) baixado(t *testing.T, v Versao) Baixado {
	t.Helper()
	caminho := c.caminho(PrefixoTemp + "teste")
	copiarAuxiliar(t, caminho, v)
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return Baixado{
		Caminho: caminho,
		Asset:   Asset{Nome: "eprocia-" + v.String() + "-windows-amd64.exe", Tamanho: int64(len(dados))},
		Versao:  v,
		SHA256:  sha256.Sum256(dados),
	}
}

// instalado faz um Instalar bem-sucedido do 0.5.0 com tr e devolve o baixado.
func (c *cenarioTroca) instalado(t *testing.T, tr *Troca) Baixado {
	t.Helper()
	b := c.baixado(t, v050)
	if err := tr.Instalar(context.Background(), b); err != nil {
		t.Fatalf("Instalar = %v", err)
	}
	return b
}

var reArgs = regexp.MustCompile(`^args-(\d+)\.txt$`)

// filhos lista os PIDs dos auxiliares que se registraram em dados.
func filhos(t *testing.T, dados string) []int {
	t.Helper()
	entradas, err := os.ReadDir(dados)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entradas {
		if m := reArgs.FindStringSubmatch(e.Name()); m != nil {
			pid, _ := strconv.Atoi(m[1])
			pids = append(pids, pid)
		}
	}
	return pids
}

// unicoFilho exige exatamente um auxiliar iniciado e devolve o PID dele.
func unicoFilho(t *testing.T, dados string) int {
	t.Helper()
	pids := filhos(t, dados)
	if len(pids) != 1 {
		t.Fatalf("filhos iniciados = %v, quer exatamente um", pids)
	}
	return pids[0]
}

// nenhumFilho exige que nenhum auxiliar tenha sido iniciado.
func nenhumFilho(t *testing.T, dados string) {
	t.Helper()
	if pids := filhos(t, dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(dados)
}

func argsDe(t *testing.T, dados string, pid int) []string {
	t.Helper()
	return strings.Split(lerArquivo(t, filepath.Join(dados, "args-"+strconv.Itoa(pid)+".txt")), "\n")
}

func somaDe(t *testing.T, caminho string) [32]byte {
	t.Helper()
	return sha256.Sum256([]byte(lerArquivo(t, caminho)))
}

func versaoDe(t *testing.T, caminho string) Versao {
	t.Helper()
	v, err := VersaoPE(caminho)
	if err != nil {
		t.Fatalf("versão de %s: %v", filepath.Base(caminho), err)
	}
	return v
}

func naoExiste(t *testing.T, caminho string) {
	t.Helper()
	if _, err := os.Lstat(caminho); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s existe (err=%v)", filepath.Base(caminho), err)
	}
}

func semTemporarios(t *testing.T, pasta string) {
	t.Helper()
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		if temPrefixo(e.Name(), PrefixoTemp) {
			t.Errorf("sobrou %s", e.Name())
		}
	}
}

// renomearFalhando devolve um renomear que falha na chamada n (1 em diante).
func renomearFalhando(n int) func(de, para string) error {
	chamada := 0
	return func(de, para string) error {
		chamada++
		if chamada == n {
			return &os.LinkError{Op: "rename", Old: de, New: para, Err: errors.New("falha injetada")}
		}
		return os.Rename(de, para)
	}
}

// falhaInjetada é o erro dos renomear falsos.
func falhaInjetada(de, para string) error {
	return &os.LinkError{Op: "rename", Old: de, New: para, Err: errors.New("falha injetada")}
}

// outrosTemporarios lista os eprocia.exe.download-* da pasta, menos exceto.
func outrosTemporarios(t *testing.T, pasta, exceto string) []string {
	t.Helper()
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, e := range entradas {
		if temPrefixo(e.Name(), PrefixoTemp) && e.Name() != exceto {
			nomes = append(nomes, e.Name())
		}
	}
	return nomes
}

// conferirRevertido: eprocia.exe voltou a ser o de antes e o novo ficou como
// eprocia.falhou.exe.
func conferirRevertido(t *testing.T, c *cenarioTroca, antes [32]byte, falhou Versao) {
	t.Helper()
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não voltou a ser o de antes")
	}
	if v := versaoDe(t, c.caminho(NomeFalhou)); v != falhou {
		t.Errorf("eprocia.falhou.exe = %v, quer %v", v, falhou)
	}
}

func querNaoAbriu050(t *testing.T, err error) {
	t.Helper()
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Reiniciar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
}

// pausaCurta encurta a pausa entre as novas tentativas durante o teste.
func pausaCurta(t *testing.T) {
	t.Helper()
	antiga := pausaTentativa
	pausaTentativa = time.Millisecond
	t.Cleanup(func() { pausaTentativa = antiga })
}

func TestInstalar_Sucesso(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.Extras = []string{"--tray"}
	if err := tr.Instalar(context.Background(), b); err != nil {
		t.Fatalf("Instalar = %v", err)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer 0.5.0", v)
	}
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	naoExiste(t, b.Caminho)
	nenhumFilho(t, c.dados)
	naoExiste(t, CaminhoSinal(c.dados, tr.PIDAtual))
}

func TestInstalar_ArquivoAlteradoAntesDoRename(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	f, err := os.OpenFile(b.Caminho, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0x5A}, b.Asset.Tamanho/2); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = c.troca(prazoComSinal).Instalar(context.Background(), b)
	if !errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("Instalar = %v, quer ErrNaoConferiu", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe mudou")
	}
	naoExiste(t, c.caminho(NomeAnterior))
	naoExiste(t, b.Caminho)
	nenhumFilho(t, c.dados)
}

func TestInstalar_AnteriorVelhoApagado(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	copiarAuxiliar(t, c.caminho(NomeAnterior), v042)
	b := c.baixado(t, v050)
	if err := c.troca(prazoComSinal).Instalar(context.Background(), b); err != nil {
		t.Fatalf("Instalar = %v", err)
	}
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer 0.5.0", v)
	}
	nenhumFilho(t, c.dados)
}

func TestInstalar_FalhaNoSegundoRename(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.renomear = renomearFalhando(2)
	err := tr.Instalar(context.Background(), b)
	if err == nil {
		t.Fatal("Instalar = nil, quer erro")
	}
	var naoAbriu ErrNaoAbriu
	if errors.As(err, &naoAbriu) || errors.Is(err, ErrReversaoIncompleta) || errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("Instalar = %v, quer erro de \"nada mudou\"", err)
	}
	if !errors.Is(err, ErrNaoAplicou) {
		t.Errorf("Instalar = %v, quer ErrNaoAplicou", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe mudou")
	}
	naoExiste(t, c.caminho(NomeAnterior))
	if _, err := os.Stat(b.Caminho); err != nil {
		t.Errorf("o baixado sumiu: %v", err)
	}
	nenhumFilho(t, c.dados)
}

// falhaDupla faz falhar o rename do baixado para eprocia.exe e o desfazer.
func falhaDupla(b Baixado) func(de, para string) error {
	return func(de, para string) error {
		if filepath.Base(para) == NomeExe && (de == b.Caminho || filepath.Base(de) == NomeAnterior) {
			return falhaInjetada(de, para)
		}
		return os.Rename(de, para)
	}
}

func conferirCopiaAnterior(t *testing.T, c *cenarioTroca, b Baixado, antes [32]byte) {
	t.Helper()
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não é a cópia do 0.4.4")
	}
	if sobra := outrosTemporarios(t, c.pasta, filepath.Base(b.Caminho)); len(sobra) != 0 {
		t.Errorf("sobraram temporários: %v", sobra)
	}
	nenhumFilho(t, c.dados)
}

func TestInstalar_DesfazerFalhaCopiaAnterior(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.renomear = falhaDupla(b)
	if err := tr.Instalar(context.Background(), b); !errors.Is(err, ErrReversaoIncompleta) {
		t.Fatalf("Instalar = %v, quer ErrReversaoIncompleta", err)
	}
	conferirCopiaAnterior(t, c, b, antes)
}

// O rename do temporário da cópia de socorro para eprocia.exe repete em erro
// de compartilhamento (antivírus segurando o exe recém-gravado).
func TestInstalar_CopiaDeSocorroRepeteRename(t *testing.T) {
	pausaCurta(t)
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	dupla := falhaDupla(b)
	chamadas := 0
	tr.renomear = func(de, para string) error {
		if filepath.Base(para) == NomeExe && de != b.Caminho && temPrefixo(filepath.Base(de), PrefixoTemp) {
			chamadas++
			if chamadas <= 3 {
				return &os.LinkError{Op: "rename", Old: de, New: para, Err: syscall.Errno(32)}
			}
		}
		return dupla(de, para)
	}
	if err := tr.Instalar(context.Background(), b); !errors.Is(err, ErrReversaoIncompleta) {
		t.Fatalf("Instalar = %v, quer ErrReversaoIncompleta", err)
	}
	if chamadas != 4 {
		t.Errorf("renames do temporário = %d, quer 4", chamadas)
	}
	conferirCopiaAnterior(t, c, b, antes)
}

// A cópia do anterior falha: o socorro copia o baixado, como no reverter.
func TestInstalar_SocorroCopiaOBaixado(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	dupla := falhaDupla(b)
	primeiroTmp := ""
	tr.renomear = func(de, para string) error {
		if filepath.Base(para) == NomeExe && de != b.Caminho && temPrefixo(filepath.Base(de), PrefixoTemp) {
			if primeiroTmp == "" {
				primeiroTmp = de
			}
			if de == primeiroTmp {
				return falhaInjetada(de, para)
			}
		}
		return dupla(de, para)
	}
	if err := tr.Instalar(context.Background(), b); !errors.Is(err, ErrReversaoIncompleta) {
		t.Fatalf("Instalar = %v, quer ErrReversaoIncompleta", err)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer a cópia do baixado (0.5.0)", v)
	}
	if sobra := outrosTemporarios(t, c.pasta, filepath.Base(b.Caminho)); len(sobra) != 0 {
		t.Errorf("sobraram temporários: %v", sobra)
	}
	nenhumFilho(t, c.dados)
}

// Quem chama Instalar já segura a Trava; o desfazer não pode travá-la de novo.
func TestInstalar_DesfazerComTravaJaTravada(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.renomear = falhaDupla(b)
	trava := &sync.Mutex{}
	tr.Trava = trava
	trava.Lock()
	defer trava.Unlock()
	res := make(chan error, 1)
	go func() { res <- tr.Instalar(context.Background(), b) }()
	select {
	case err := <-res:
		if !errors.Is(err, ErrReversaoIncompleta) {
			t.Fatalf("Instalar = %v, quer ErrReversaoIncompleta", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Instalar não voltou em 2 s com a Trava já travada (impasse)")
	}
	conferirCopiaAnterior(t, c, b, antes)
}

func TestReiniciar_Sucesso(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	tr := c.troca(prazoComSinal)
	tr.Extras = []string{"--tray"}
	c.instalado(t, tr)
	if err := tr.Reiniciar(context.Background(), v050); err != nil {
		t.Fatalf("Reiniciar = %v", err)
	}
	pid := unicoFilho(t, c.dados)
	quer := []string{FlagAposAtualizar + strconv.Itoa(tr.PIDAtual), "--limpar-anterior", "--tray"}
	if args := argsDe(t, c.dados, pid); !slices.Equal(args, quer) {
		t.Errorf("args do filho = %q, quer %q", args, quer)
	}
	naoExiste(t, CaminhoSinal(c.dados, tr.PIDAtual))
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer 0.5.0", v)
	}
}

func TestReiniciar_SemAnteriorNaoInicia(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	err := c.troca(prazoComSinal).Reiniciar(context.Background(), v050)
	if !errors.Is(err, ErrSemAnterior) {
		t.Fatalf("Reiniciar = %v, quer ErrSemAnterior", err)
	}
	if errors.Is(err, ErrNaoAplicou) {
		t.Errorf("Reiniciar = %v, não quer ErrNaoAplicou", err)
	}
	nenhumFilho(t, c.dados)
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe mudou")
	}
}

func TestReiniciar_NovoSaiComErroReverte(t *testing.T) {
	c := novoCenarioTroca(t, "sai1")
	antes := somaDe(t, c.caminho(NomeExe))
	tr := c.troca(prazoComSinal)
	c.instalado(t, tr)
	inicio := time.Now()
	querNaoAbriu050(t, tr.Reiniciar(context.Background(), v050))
	if d := time.Since(inicio); d >= prazoComSinal {
		t.Errorf("esperou o prazo inteiro (%v) com o novo já fora", d)
	}
	conferirRevertido(t, c, antes, v050)
	naoExiste(t, c.caminho(NomeAnterior))
}

func TestReiniciar_PrazoReverte(t *testing.T) {
	c := novoCenarioTroca(t, "parado")
	antes := somaDe(t, c.caminho(NomeExe))
	tr := c.troca(prazoSemSinal)
	c.instalado(t, tr)
	querNaoAbriu050(t, tr.Reiniciar(context.Background(), v050))
	if !EsperarSair(unicoFilho(t, c.dados), 0) {
		t.Error("o filho parado ainda roda depois de Reiniciar devolver")
	}
	conferirRevertido(t, c, antes, v050)
	naoExiste(t, c.caminho(NomeAnterior))
}

func TestReiniciar_FalhaNoStartReverte(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	tr := c.troca(prazoComSinal)
	c.instalado(t, tr)
	// O eprocia.exe instalado deixa de ser PE: o Start falha.
	if err := os.WriteFile(c.caminho(NomeExe), []byte("não é um executável"), 0o644); err != nil {
		t.Fatal(err)
	}
	querNaoAbriu050(t, tr.Reiniciar(context.Background(), v050))
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não voltou a ser o de antes")
	}
	if _, err := os.Stat(c.caminho(NomeFalhou)); err != nil {
		t.Errorf("sem eprocia.falhou.exe: %v", err)
	}
	naoExiste(t, c.caminho(NomeAnterior))
	nenhumFilho(t, c.dados)
}

func TestReverter_RepeteEmViolacaoDeCompartilhamento(t *testing.T) {
	pausaCurta(t)
	casos := []struct {
		nome    string
		de      string
		para    string
		errno   syscall.Errno
		falhas  int
		inteira bool
	}{
		{"exe para falhou, 32", NomeExe, NomeFalhou, 32, 3, true},
		{"anterior para exe, 32", NomeAnterior, NomeExe, 32, 3, true},
		{"exe para falhou, 5", NomeExe, NomeFalhou, 5, 3, true},
		{"exe para falhou, sempre", NomeExe, NomeFalhou, 32, 1000, false},
		{"anterior para exe, sempre", NomeAnterior, NomeExe, 5, 1000, false},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenarioTroca(t, "sai1")
			antes := somaDe(t, c.caminho(NomeExe))
			tr := c.troca(prazoComSinal)
			c.instalado(t, tr)
			var chamadas int
			tr.renomear = func(de, para string) error {
				if filepath.Base(de) == caso.de && filepath.Base(para) == caso.para {
					chamadas++
					if chamadas <= caso.falhas {
						return &os.LinkError{Op: "rename", Old: de, New: para, Err: caso.errno}
					}
				}
				return os.Rename(de, para)
			}
			err := tr.Reiniciar(context.Background(), v050)
			if caso.inteira {
				querNaoAbriu050(t, err)
				conferirRevertido(t, c, antes, v050)
				if chamadas != caso.falhas+1 {
					t.Errorf("chamadas = %d, quer %d", chamadas, caso.falhas+1)
				}
				return
			}
			if !errors.Is(err, ErrReversaoIncompleta) {
				t.Fatalf("Reiniciar = %v, quer ErrReversaoIncompleta", err)
			}
			if _, err := os.Stat(c.caminho(NomeExe)); err != nil {
				t.Errorf("sem eprocia.exe: %v", err)
			}
			if chamadas != tentativasArquivo {
				t.Errorf("chamadas = %d, quer %d", chamadas, tentativasArquivo)
			}
		})
	}
	t.Run("outro erro não repete", func(t *testing.T) {
		c := novoCenarioTroca(t, "sai1")
		tr := c.troca(prazoComSinal)
		c.instalado(t, tr)
		var chamadas int
		tr.renomear = func(de, para string) error {
			if filepath.Base(de) == NomeExe && filepath.Base(para) == NomeFalhou {
				chamadas++
				return &os.LinkError{Op: "rename", Old: de, New: para, Err: syscall.Errno(2)}
			}
			return os.Rename(de, para)
		}
		if err := tr.Reiniciar(context.Background(), v050); !errors.Is(err, ErrReversaoIncompleta) {
			t.Fatalf("Reiniciar = %v, quer ErrReversaoIncompleta", err)
		}
		if chamadas != 1 {
			t.Errorf("chamadas = %d, quer 1", chamadas)
		}
	})
}

func TestReiniciar_ReversaoSeguraTrava(t *testing.T) {
	c := novoCenarioTroca(t, "sai1")
	tr := c.troca(prazoComSinal)
	c.instalado(t, tr)
	antes := somaDe(t, c.caminho(NomeAnterior))
	trava := &sync.Mutex{}
	tr.Trava = trava
	chegou, liberar := make(chan struct{}), make(chan struct{})
	var fimUltimo atomic.Bool
	tr.renomear = func(de, para string) error {
		if filepath.Base(de) == NomeExe && filepath.Base(para) == NomeFalhou {
			close(chegou)
			<-liberar
		}
		err := os.Rename(de, para)
		if filepath.Base(de) == NomeAnterior && filepath.Base(para) == NomeExe {
			fimUltimo.Store(true)
		}
		return err
	}
	res := make(chan error, 1)
	go func() { res <- tr.Reiniciar(context.Background(), v050) }()
	select {
	case <-chegou:
	case <-time.After(prazoComSinal):
		t.Fatal("a reversão não chegou ao rename do eprocia.exe")
	}
	travou := make(chan bool, 1)
	go func() {
		trava.Lock()
		travou <- fimUltimo.Load()
		trava.Unlock()
	}()
	select {
	case <-travou:
		t.Fatal("Lock voltou com a reversão em curso")
	case <-time.After(500 * time.Millisecond):
	}
	close(liberar)
	select {
	case fim := <-travou:
		if !fim {
			t.Error("Lock voltou antes do último rename da reversão")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Lock não voltou em 2 s depois de liberada a reversão")
	}
	querNaoAbriu050(t, <-res)
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não voltou a ser o 0.4.4")
	}
	if v := versaoDe(t, c.caminho(NomeFalhou)); v != v050 {
		t.Errorf("eprocia.falhou.exe = %v, quer 0.5.0", v)
	}
}

func TestReversao_Incompleta(t *testing.T) {
	casos := []struct {
		nome  string
		falha func(b Baixado, de, para string) bool
		quer  Versao
	}{
		// O rename do anterior falha; a cópia dele ocupa o eprocia.exe.
		{"copia o anterior", func(_ Baixado, de, para string) bool {
			return filepath.Base(de) == NomeAnterior && filepath.Base(para) == NomeExe
		}, v044},
		// Nem o anterior nem a cópia dele chegam a eprocia.exe; o novo volta.
		{"repoe o falhou", func(b Baixado, de, para string) bool {
			if filepath.Base(para) != NomeExe || de == b.Caminho {
				return false
			}
			return filepath.Base(de) == NomeAnterior || temPrefixo(filepath.Base(de), PrefixoTemp)
		}, v050},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenarioTroca(t, "sai1")
			tr := c.troca(prazoComSinal)
			b := c.instalado(t, tr)
			tr.renomear = func(de, para string) error {
				if caso.falha(b, de, para) {
					return falhaInjetada(de, para)
				}
				return os.Rename(de, para)
			}
			err := tr.Reiniciar(context.Background(), v050)
			if !errors.Is(err, ErrReversaoIncompleta) {
				t.Fatalf("Reiniciar = %v, quer ErrReversaoIncompleta", err)
			}
			if v := versaoDe(t, c.caminho(NomeExe)); v != caso.quer {
				t.Errorf("eprocia.exe = %v, quer %v", v, caso.quer)
			}
			semTemporarios(t, c.pasta)
		})
	}
}
