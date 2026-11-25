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
	"testing"
	"time"
)

var (
	v042 = Versao{0, 4, 2}
	v043 = Versao{0, 4, 3}
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

func TestAplicar_Sucesso(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.Extras = []string{"--tray"}
	if err := tr.Aplicar(context.Background(), b); err != nil {
		t.Fatalf("Aplicar = %v", err)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer 0.5.0", v)
	}
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	naoExiste(t, b.Caminho)
	pid := unicoFilho(t, c.dados)
	quer := []string{FlagAposAtualizar + strconv.Itoa(tr.PIDAtual), "--tray"}
	if args := argsDe(t, c.dados, pid); !slices.Equal(args, quer) {
		t.Errorf("args do filho = %q, quer %q", args, quer)
	}
	naoExiste(t, CaminhoSinal(c.dados, tr.PIDAtual))
}

func TestAplicar_ArquivoAlteradoAntesDoRename(t *testing.T) {
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
	err = c.troca(prazoComSinal).Aplicar(context.Background(), b)
	if !errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("Aplicar = %v, quer ErrNaoConferiu", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe mudou")
	}
	naoExiste(t, c.caminho(NomeAnterior))
	naoExiste(t, b.Caminho)
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

func TestAplicar_AnteriorVelhoApagado(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	copiarAuxiliar(t, c.caminho(NomeAnterior), v042)
	b := c.baixado(t, v050)
	if err := c.troca(prazoComSinal).Aplicar(context.Background(), b); err != nil {
		t.Fatalf("Aplicar = %v", err)
	}
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v050 {
		t.Errorf("eprocia.exe = %v, quer 0.5.0", v)
	}
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

func TestAplicar_NovoSaiComErroReverte(t *testing.T) {
	c := novoCenarioTroca(t, "sai1")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	inicio := time.Now()
	err := c.troca(prazoComSinal).Aplicar(context.Background(), b)
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Aplicar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
	if d := time.Since(inicio); d >= prazoComSinal {
		t.Errorf("esperou o prazo inteiro (%v) com o novo já fora", d)
	}
	conferirRevertido(t, c, antes, v050)
	naoExiste(t, c.caminho(NomeAnterior))
}

func TestAplicar_PrazoReverte(t *testing.T) {
	c := novoCenarioTroca(t, "parado")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	err := c.troca(prazoSemSinal).Aplicar(context.Background(), b)
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Aplicar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
	pid := unicoFilho(t, c.dados)
	if !EsperarSair(pid, 0) {
		t.Error("o filho parado ainda roda depois de Aplicar devolver")
	}
	conferirRevertido(t, c, antes, v050)
}

func TestAplicar_FalhaNoSegundoRename(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.renomear = renomearFalhando(2)
	err := tr.Aplicar(context.Background(), b)
	if err == nil {
		t.Fatal("Aplicar = nil, quer erro")
	}
	var naoAbriu ErrNaoAbriu
	if errors.As(err, &naoAbriu) || errors.Is(err, ErrReversaoIncompleta) || errors.Is(err, ErrNaoConferiu) {
		t.Fatalf("Aplicar = %v, quer erro de \"nada mudou\"", err)
	}
	if !errors.Is(err, ErrNaoAplicou) {
		t.Errorf("Aplicar = %v, quer ErrNaoAplicou", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe mudou")
	}
	naoExiste(t, c.caminho(NomeAnterior))
	if _, err := os.Stat(b.Caminho); err != nil {
		t.Errorf("o baixado sumiu: %v", err)
	}
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

// cenarioVoltar: eprocia.exe N (0.4.4) em execução e eprocia.anterior.exe
// N-1 (0.4.3).
func cenarioVoltar(t *testing.T, modo string) (c *cenarioTroca, somaN, somaAnterior [32]byte) {
	t.Helper()
	c = novoCenarioTroca(t, modo)
	copiarAuxiliar(t, c.caminho(NomeAnterior), v043)
	return c, somaDe(t, c.caminho(NomeExe)), somaDe(t, c.caminho(NomeAnterior))
}

func TestVoltar_Sucesso(t *testing.T) {
	c, _, _ := cenarioVoltar(t, "sinal")
	tr := c.troca(prazoComSinal)
	if err := tr.Voltar(context.Background()); err != nil {
		t.Fatalf("Voltar = %v", err)
	}
	if v := versaoDe(t, c.caminho(NomeExe)); v != v043 {
		t.Errorf("eprocia.exe = %v, quer 0.4.3", v)
	}
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	pid := unicoFilho(t, c.dados)
	if v := lerArquivo(t, filepath.Join(c.dados, "versao-"+strconv.Itoa(pid)+".txt")); v != "0.4.3" {
		t.Errorf("o filho registrou %q, quer 0.4.3", v)
	}
	if args := argsDe(t, c.dados, pid); !slices.Equal(args, []string{FlagAposAtualizar + strconv.Itoa(tr.PIDAtual)}) {
		t.Errorf("args do filho = %q", args)
	}
	naoExiste(t, CaminhoSinal(c.dados, tr.PIDAtual))
	semTemporarios(t, c.pasta)
}

func TestVoltar_FalhaNosPassos2e3(t *testing.T) {
	for _, n := range []int{2, 3} {
		t.Run("rename "+strconv.Itoa(n), func(t *testing.T) {
			c, somaN, somaAnterior := cenarioVoltar(t, "sinal")
			tr := c.troca(prazoComSinal)
			tr.renomear = renomearFalhando(n)
			err := tr.Voltar(context.Background())
			if err == nil {
				t.Fatal("Voltar = nil, quer erro")
			}
			var naoAbriu ErrNaoAbriu
			if errors.As(err, &naoAbriu) || errors.Is(err, ErrReversaoIncompleta) {
				t.Fatalf("Voltar = %v, quer erro de \"nada mudou\"", err)
			}
			if !errors.Is(err, ErrNaoAplicou) {
				t.Errorf("Voltar = %v, quer ErrNaoAplicou", err)
			}
			if somaDe(t, c.caminho(NomeExe)) != somaN {
				t.Error("eprocia.exe mudou")
			}
			if somaDe(t, c.caminho(NomeAnterior)) != somaAnterior {
				t.Error("eprocia.anterior.exe mudou")
			}
			semTemporarios(t, c.pasta)
			if pids := filhos(t, c.dados); len(pids) != 0 {
				t.Errorf("processos iniciados: %v", pids)
			}
		})
	}
}

func TestVoltar_SemSinalReverte(t *testing.T) {
	c, somaN, _ := cenarioVoltar(t, "parado")
	err := c.troca(prazoSemSinal).Voltar(context.Background())
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.4.3"}) {
		t.Fatalf("Voltar = %v, quer ErrNaoAbriu{0.4.3}", err)
	}
	if !EsperarSair(unicoFilho(t, c.dados), 0) {
		t.Error("o filho parado ainda roda depois de Voltar devolver")
	}
	conferirRevertido(t, c, somaN, v043)
	naoExiste(t, c.caminho(NomeAnterior))
	semTemporarios(t, c.pasta)
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

func TestAplicar_DesfazerFalhaCopiaAnterior(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	tr.renomear = func(de, para string) error {
		if filepath.Base(para) == NomeExe && (de == b.Caminho || filepath.Base(de) == NomeAnterior) {
			return falhaInjetada(de, para)
		}
		return os.Rename(de, para)
	}
	if err := tr.Aplicar(context.Background(), b); !errors.Is(err, ErrReversaoIncompleta) {
		t.Fatalf("Aplicar = %v, quer ErrReversaoIncompleta", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não é a cópia do 0.4.4")
	}
	if sobra := outrosTemporarios(t, c.pasta, filepath.Base(b.Caminho)); len(sobra) != 0 {
		t.Errorf("sobraram temporários: %v", sobra)
	}
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

func TestVoltar_DesfazerFalhaCopiaAnterior(t *testing.T) {
	c, somaN, somaAnterior := cenarioVoltar(t, "sinal")
	tr := c.troca(prazoComSinal)
	chamada := 0
	tr.renomear = func(de, para string) error {
		chamada++
		if chamada == 3 || (filepath.Base(de) == NomeAnterior && filepath.Base(para) == NomeExe) {
			return falhaInjetada(de, para)
		}
		return os.Rename(de, para)
	}
	if err := tr.Voltar(context.Background()); !errors.Is(err, ErrReversaoIncompleta) {
		t.Fatalf("Voltar = %v, quer ErrReversaoIncompleta", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != somaN {
		t.Error("eprocia.exe não é a cópia da versão N")
	}
	// O eprocia.anterior.exe é a N em execução (não pode ser substituído); a
	// N-1 fica num único temporário, que a limpeza da abertura apaga.
	if v := versaoDe(t, c.caminho(NomeAnterior)); v != v044 {
		t.Errorf("eprocia.anterior.exe = %v, quer 0.4.4", v)
	}
	sobra := outrosTemporarios(t, c.pasta, "")
	if len(sobra) != 1 || somaDe(t, c.caminho(sobra[0])) != somaAnterior {
		t.Errorf("temporários = %v, quer só o da N-1", sobra)
	}
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

func TestAplicar_FalhaNoStartReverte(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	tr := c.troca(prazoComSinal)
	// Depois do segundo rename, o eprocia.exe deixa de ser PE: o Start falha.
	tr.renomear = func(de, para string) error {
		if err := os.Rename(de, para); err != nil {
			return err
		}
		if de == b.Caminho {
			return os.WriteFile(para, []byte("não é um executável"), 0o644)
		}
		return nil
	}
	err := tr.Aplicar(context.Background(), b)
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Aplicar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != antes {
		t.Error("eprocia.exe não voltou a ser o de antes")
	}
	if _, err := os.Stat(c.caminho(NomeFalhou)); err != nil {
		t.Errorf("sem eprocia.falhou.exe: %v", err)
	}
	naoExiste(t, c.caminho(NomeAnterior))
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
	}
}

func TestAplicar_ContextoCanceladoReverte(t *testing.T) {
	c := novoCenarioTroca(t, "parado")
	antes := somaDe(t, c.caminho(NomeExe))
	b := c.baixado(t, v050)
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	time.AfterFunc(time.Second, cancelar)
	inicio := time.Now()
	err := c.troca(prazoComSinal).Aplicar(ctx, b)
	var naoAbriu ErrNaoAbriu
	if !errors.As(err, &naoAbriu) || naoAbriu != (ErrNaoAbriu{"0.5.0"}) {
		t.Fatalf("Aplicar = %v, quer ErrNaoAbriu{0.5.0}", err)
	}
	if d := time.Since(inicio); d >= prazoComSinal/2 {
		t.Errorf("demorou %v com o contexto cancelado em 1 s", d)
	}
	if !EsperarSair(unicoFilho(t, c.dados), 0) {
		t.Error("o filho parado ainda roda depois de Aplicar devolver")
	}
	conferirRevertido(t, c, antes, v050)
}

func TestVoltar_AnteriorSemVersaoNaoMuda(t *testing.T) {
	c := novoCenarioTroca(t, "sinal")
	if err := os.WriteFile(c.caminho(NomeAnterior), []byte("não é um executável"), 0o644); err != nil {
		t.Fatal(err)
	}
	somaN, somaAnterior := somaDe(t, c.caminho(NomeExe)), somaDe(t, c.caminho(NomeAnterior))
	tr := c.troca(prazoComSinal)
	tr.renomear = func(de, para string) error {
		t.Errorf("rename %s -> %s com o anterior sem versão", filepath.Base(de), filepath.Base(para))
		return os.Rename(de, para)
	}
	if err := tr.Voltar(context.Background()); !errors.Is(err, ErrNaoAplicou) {
		t.Fatalf("Voltar = %v, quer ErrNaoAplicou", err)
	}
	if somaDe(t, c.caminho(NomeExe)) != somaN || somaDe(t, c.caminho(NomeAnterior)) != somaAnterior {
		t.Error("os arquivos mudaram")
	}
	if pids := filhos(t, c.dados); len(pids) != 0 {
		t.Errorf("processos iniciados: %v", pids)
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
			b := c.baixado(t, v050)
			tr := c.troca(prazoComSinal)
			tr.renomear = func(de, para string) error {
				if caso.falha(b, de, para) {
					return falhaInjetada(de, para)
				}
				return os.Rename(de, para)
			}
			err := tr.Aplicar(context.Background(), b)
			if !errors.Is(err, ErrReversaoIncompleta) {
				t.Fatalf("Aplicar = %v, quer ErrReversaoIncompleta", err)
			}
			if v := versaoDe(t, c.caminho(NomeExe)); v != caso.quer {
				t.Errorf("eprocia.exe = %v, quer %v", v, caso.quer)
			}
			semTemporarios(t, c.pasta)
		})
	}
}
