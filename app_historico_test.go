package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"eprocia/internal/atualizacao"
	"eprocia/internal/etl"
	"eprocia/internal/historico"
	"eprocia/internal/saida"
)

// cnjTeste é o número fictício dos nomes de arquivo dos testes.
const cnjTeste = "5000000-00.2026.8.26.0001"

// ligarHistoricoNaConfig liga GuardarHistorico na config da montagem (antes
// de m.iniciar).
func ligarHistoricoNaConfig(m *montagem) {
	novo := *m.app.configAtual()
	novo.GuardarHistorico = true
	m.app.guardarConfig(&novo)
}

func arquivoHistoricoDe(m *montagem) string {
	return filepath.Join(m.app.pastaDados, "historico.json")
}

// gravarHistoricoAMao escreve o historico.json como o programa escreveria.
func gravarHistoricoAMao(t *testing.T, arquivo string, itens []historico.Item) {
	t.Helper()
	dados, err := json.Marshal(map[string]any{"versao": 1, "itens": itens})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arquivo, dados, 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemZip(caminho, status string) historico.Item {
	return historico.Item{
		Name: filepath.Base(caminho), Path: caminho, Status: status, Kind: "zip",
		Timestamp: 1_700_000_000_000, FinishedAt: 1_700_000_060_000, OcrMode: "none", CleaningLevel: "basic",
	}
}

// estadoArquivo guarda bytes e ModTime para conferir que nada regravou.
type estadoArquivo struct {
	dados []byte
	mod   time.Time
}

func lerEstado(t *testing.T, caminho string) estadoArquivo {
	t.Helper()
	e, err := lerEstadoErr(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// lerEstadoErr é lerEstado sem t, para goroutines fora do teste (o erro volta
// a quem chamou).
func lerEstadoErr(caminho string) (estadoArquivo, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return estadoArquivo{}, err
	}
	info, err := os.Stat(caminho)
	if err != nil {
		return estadoArquivo{}, err
	}
	return estadoArquivo{dados, info.ModTime()}, nil
}

func mesmoEstado(t *testing.T, caminho string, antes estadoArquivo) bool {
	t.Helper()
	agora := lerEstado(t, caminho)
	return bytes.Equal(agora.dados, antes.dados) && agora.mod.Equal(antes.mod)
}

// carregados devolve os caminhos dos itens gravados (nil se não leu).
func carregados(t *testing.T, arquivo string) []string {
	t.Helper()
	itens, err := historico.Carregar(arquivo)
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}
	var caminhos []string
	for _, it := range itens {
		caminhos = append(caminhos, it.Path)
	}
	return caminhos
}

// carregadosAgora é carregados para as esperas: com o gravador renomeando ao
// mesmo tempo, o Windows pode recusar a leitura (arquivo em uso); isso conta
// como "ainda não" (ok false), não como falha.
func carregadosAgora(arquivo string) (caminhos []string, ok bool) {
	itens, err := historico.Carregar(arquivo)
	if err != nil {
		return nil, false
	}
	for _, it := range itens {
		caminhos = append(caminhos, it.Path)
	}
	return caminhos, true
}

func caminhosIguais(a []string, b ...string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func itemPorCaminho(app *App, caminho string) *etl.QueueItem {
	for _, it := range app.GetQueue() {
		if strings.EqualFold(it.Path, caminho) {
			return it
		}
	}
	return nil
}

// adicionarETerminar põe um ZIP inexistente na fila e espera o erro.
func adicionarETerminar(t *testing.T, app *App, caminho string) string {
	t.Helper()
	if n, err := app.AddFilesToQueue([]string{caminho}); err != nil || n != 1 {
		t.Fatalf("AddFilesToQueue = %d, %v; quer 1, nil", n, err)
	}
	var id string
	esperarAte(t, 5*time.Second, "o item terminar em erro", func() bool {
		it := itemPorCaminho(app, caminho)
		if it == nil || it.Status != "error" {
			return false
		}
		id = it.ID
		return true
	})
	return id
}

func contarEvento(m *montagem, nome string) int {
	n := 0
	for _, e := range m.f.eventos() {
		if e == nome {
			n++
		}
	}
	return n
}

func TestHistorico_RestauraAntesDoWatcher(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	zip := filepath.Join(m.monitorada, cnjTeste+".zip")
	if err := os.WriteFile(zip, []byte("PK fictício"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Caixa diferente da que o watcher entrega: a linha guardada é reaproveitada.
	guardado := strings.ToLower(zip)
	gravarHistoricoAMao(t, arquivoHistoricoDe(m), []historico.Item{itemZip(guardado, "processing")})

	m.iniciar()
	esperarAte(t, 10*time.Second, "a varredura entregar o ZIP", func() bool {
		itens := m.app.GetQueue()
		return len(itens) > 0 && itens[0].Status != "interrupted"
	})
	itens := m.app.GetQueue()
	if len(itens) != 1 {
		t.Fatalf("fila com %d linhas; quer 1", len(itens))
	}
	if itens[0].Path != guardado {
		t.Fatalf("linha com Path %q; quer a linha guardada (%q)", itens[0].Path, guardado)
	}
}

func TestHistorico_DesligadoNaoLe(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	arquivo := arquivoHistoricoDe(m)
	gravarHistoricoAMao(t, arquivo, []historico.Item{itemZip(filepath.Join(t.TempDir(), cnjTeste+".zip"), "success")})
	antes := lerEstado(t, arquivo)

	m.iniciar()
	if itens := m.app.GetQueue(); len(itens) != 0 {
		t.Fatalf("fila com %d itens com o histórico desligado", len(itens))
	}
	if _, err := m.app.AddFilesToQueue([]string{filepath.Join(t.TempDir(), "outro.zip")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	m.app.shutdown(nil)
	if !mesmoEstado(t, arquivo, antes) {
		t.Fatal("o historico.json mudou com o histórico desligado")
	}
}

func TestHistorico_SessaoVaziaNaoApaga(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	arquivo := arquivoHistoricoDe(m)
	dir := t.TempDir()
	guardados := []historico.Item{
		itemZip(filepath.Join(dir, "a.zip"), "success"),
		itemZip(filepath.Join(dir, "b.zip"), "error"),
	}
	gravarHistoricoAMao(t, arquivo, guardados)

	m.iniciar()
	m.app.shutdown(nil)
	itens, err := historico.Carregar(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(itens, guardados) {
		t.Fatalf("depois da sessão sem entrada:\n%+v\nquer:\n%+v", itens, guardados)
	}
}

func TestHistorico_VersaoNovaIntacta(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	arquivo := arquivoHistoricoDe(m)
	if err := os.WriteFile(arquivo, []byte(`{"versao": 2, "itens": [{"futuro": true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	antes := lerEstado(t, arquivo)
	m.iniciar()
	m.app.shutdown(nil)
	if !mesmoEstado(t, arquivo, antes) {
		t.Fatal("o arquivo de versão nova foi sobrescrito sem entrada na sessão")
	}
}

func TestHistorico_ConfereAusentes(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	zip := filepath.Join(t.TempDir(), cnjTeste+".zip")
	it := itemZip(zip, "success")
	it.OutputPath = saida.Base(zip) // não existe
	gravarHistoricoAMao(t, arquivoHistoricoDe(m), []historico.Item{it})

	m.iniciar()
	esperarAte(t, 2*time.Second, "OutputMissing no restaurado", func() bool {
		item := itemPorCaminho(m.app, zip)
		return item != nil && item.OutputMissing
	})
}

func TestHistorico_Interruptor(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{})
	arquivo := arquivoHistoricoDe(m)
	m.iniciar()
	dir := t.TempDir()
	primeiro := filepath.Join(dir, "a.zip")
	adicionarETerminar(t, m.app, primeiro)

	cfg := *m.app.configAtual()
	cfg.GuardarHistorico = true
	if err := m.app.SaveConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	if got := carregados(t, arquivo); !caminhosIguais(got, primeiro) {
		t.Fatalf("ao ligar, o arquivo tem %v; quer [%s]", got, primeiro)
	}

	desligar := cfg
	desligar.GuardarHistorico = false
	if err := m.app.SaveConfig(&desligar); err != nil {
		t.Fatal(err)
	}
	if existe(arquivo) {
		t.Fatal("desligar não apagou o historico.json")
	}
	if _, err := m.app.AddFilesToQueue([]string{filepath.Join(dir, "b.zip")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if existe(arquivo) {
		t.Fatal("o historico.json voltou com o histórico desligado")
	}
}

func TestHistorico_AposAtualizarAdia(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{apos: 4242})
	ligarHistoricoNaConfig(m)
	arquivo := arquivoHistoricoDe(m)
	zip := filepath.Join(t.TempDir(), cnjTeste+".zip")
	gravarHistoricoAMao(t, arquivo, []historico.Item{itemZip(zip, "success")})
	antes := lerEstado(t, arquivo)

	m.iniciar()
	item := itemPorCaminho(m.app, zip)
	if item == nil {
		t.Fatal("o processo novo não restaurou o histórico no startup")
	}
	m.app.domPronto(nil)
	m.app.RemoveQueueItem(item.ID)
	continua(t, 1200*time.Millisecond, "arquivo intacto antes de o antigo sair", func() bool {
		return mesmoEstado(t, arquivo, antes)
	})

	close(m.f.liberaSair)
	close(m.f.liberaTrava)
	esperarAte(t, 3*time.Second, "o arquivo ficar sem o item", func() bool {
		got, ok := carregadosAgora(arquivo)
		return ok && len(got) == 0
	})
}

func TestHistorico_TrocaFechaEReabre(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	arquivo := arquivoHistoricoDe(m)
	m.iniciar()
	ad := historicoAtualizacao{m.app}
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a.zip"), filepath.Join(dir, "b.zip"), filepath.Join(dir, "c.zip")

	idA := adicionarETerminar(t, m.app, a)
	if err := ad.Fechar(); err != nil {
		t.Fatalf("Fechar = %v", err)
	}
	if got := carregados(t, arquivo); !caminhosIguais(got, a) {
		t.Fatalf("Fechar gravou %v; quer [%s]", got, a)
	}
	fechado := lerEstado(t, arquivo)
	adicionarETerminar(t, m.app, b)
	m.app.RemoveQueueItem(idA)
	time.Sleep(1500 * time.Millisecond)
	if !mesmoEstado(t, arquivo, fechado) {
		t.Fatal("o gravador fechado gravou")
	}

	if err := ad.Reabrir(); err != nil {
		t.Fatalf("Reabrir = %v", err)
	}
	esperarAte(t, 1500*time.Millisecond, "o Reabrir gravar a lista atual", func() bool {
		got, ok := carregadosAgora(arquivo)
		return ok && caminhosIguais(got, b)
	})
	adicionarETerminar(t, m.app, c)
	esperarAte(t, 1500*time.Millisecond, "nova gravação depois do Reabrir", func() bool {
		got, ok := carregadosAgora(arquivo)
		return ok && caminhosIguais(got, c, b)
	})

	if err := ad.Fechar(); err != nil {
		t.Fatalf("segundo Fechar = %v", err)
	}
	fechado = lerEstado(t, arquivo)
	m.app.RemoveQueueItem(itemPorCaminho(m.app, c).ID)
	m.app.shutdown(nil)
	time.Sleep(1200 * time.Millisecond)
	if !mesmoEstado(t, arquivo, fechado) {
		t.Fatal("o shutdown regravou depois do fechamento da troca")
	}

	t.Run("sem_gravador", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{})
		m.iniciar()
		ad := historicoAtualizacao{m.app}
		if err := ad.Fechar(); err != nil {
			t.Fatalf("Fechar sem gravador = %v", err)
		}
		if err := ad.Reabrir(); err != nil {
			t.Fatalf("Reabrir sem gravador = %v", err)
		}
		if existe(arquivoHistoricoDe(m)) {
			t.Fatal("historico.json criado com o histórico desligado")
		}
	})
}

func TestHistorico_TrocaComFalhaSegue(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	arquivo := arquivoHistoricoDe(m)
	m.iniciar()
	// Uma pasta com o nome do arquivo: o rename por cima falha.
	if err := os.Mkdir(arquivo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.app.AddFilesToQueue([]string{filepath.Join(t.TempDir(), "a.zip")}); err != nil {
		t.Fatal(err)
	}
	ad := historicoAtualizacao{m.app}
	if err := ad.Fechar(); err != nil {
		t.Fatalf("Fechar com a escrita falhando = %v; quer nil (a troca segue)", err)
	}
	if n := contarEvento(m, eventoHistoricoFalha); n != 1 {
		t.Fatalf("%s emitido %d vezes depois da primeira falha; quer 1", eventoHistoricoFalha, n)
	}
	if err := ad.Reabrir(); err != nil {
		t.Fatalf("Reabrir = %v", err)
	}
	if err := m.app.queue.GravarHistoricoAgora(); err == nil {
		t.Fatal("GravarHistoricoAgora devolveu nil com o rename falhando")
	}
	time.Sleep(1500 * time.Millisecond)
	if n := contarEvento(m, eventoHistoricoFalha); n != 1 {
		t.Fatalf("%s emitido %d vezes; quer 1 por sessão", eventoHistoricoFalha, n)
	}
}

func TestHistorico_TrocaPeloAtualizador(t *testing.T) {
	for _, falha := range []bool{true, false} {
		nome := map[bool]string{true: "troca_falha", false: "troca_ok"}[falha]
		t.Run(nome, func(t *testing.T) {
			rede := &redeFalsa{rel: releaseMaior(t), caminho: filepath.Join(t.TempDir(), "novo.exe")}
			tf := &trocadorFalso{}
			if falha {
				tf.erro = errors.New("troca falhou no teste")
			}
			m := novoAppAtualizacao(t, cenario{rede: rede})
			ligarHistoricoNaConfig(m)
			m.opcoes.Trocador = func([]string, sync.Locker) atualizacao.Trocador { return tf }
			arquivo := arquivoHistoricoDe(m)
			m.iniciar()

			zip := filepath.Join(t.TempDir(), cnjTeste+".zip")
			id := adicionarETerminar(t, m.app, zip)

			type visto struct {
				lista     []string
				intacto   bool
				carregado bool
				erro      error
			}
			vistoCh := make(chan visto, 1)
			// Roda na goroutine do atualizador: nada de t.Fatal aqui; o erro
			// volta pelo vistoCh.
			tf.antes = func() {
				var v visto
				defer func() { vistoCh <- v }()
				itens, err := historico.Carregar(arquivo)
				if err != nil {
					v.erro = err
					return
				}
				v.carregado = true
				for _, it := range itens {
					v.lista = append(v.lista, it.Path)
				}
				if falha {
					antes, err := lerEstadoErr(arquivo)
					if err != nil {
						v.erro = err
						return
					}
					m.app.RemoveQueueItem(id)
					time.Sleep(1200 * time.Millisecond)
					depois, err := lerEstadoErr(arquivo)
					if err != nil {
						v.erro = err
						return
					}
					v.intacto = bytes.Equal(depois.dados, antes.dados) && depois.mod.Equal(antes.mod)
				}
			}

			if _, err := m.app.VerificarAtualizacao(); err != nil {
				t.Fatal(err)
			}
			if err := m.app.InstalarAtualizacao(); err != nil {
				t.Fatal(err)
			}
			esperarAte(t, 5*time.Second, "fase instalada", func() bool {
				return m.app.EstadoAtualizacao().Fase == atualizacao.FaseInstalada
			})
			if err := m.app.ReiniciarAtualizacao(); err != nil {
				t.Fatal(err)
			}
			var v visto
			select {
			case v = <-vistoCh:
			case <-time.After(5 * time.Second):
				t.Fatal("o Trocador.Reiniciar não foi chamado")
			}
			if v.erro != nil {
				t.Fatalf("no Reiniciar: %v", v.erro)
			}
			if !v.carregado || !caminhosIguais(v.lista, zip) {
				t.Fatalf("no Reiniciar o historico.json tem %v (lido: %t); quer [%s]", v.lista, v.carregado, zip)
			}

			if falha {
				if !v.intacto {
					t.Fatal("o gravador fechado gravou durante a espera do reinício")
				}
				esperarAte(t, 5*time.Second, "a reversão", func() bool {
					_, r := tf.contar()
					f := m.app.EstadoAtualizacao().Fase
					return r == 1 && f != atualizacao.FaseReiniciando && f != atualizacao.FaseInstalada
				})
				esperarAte(t, 1500*time.Millisecond, "o arquivo ficar sem o item removido na espera", func() bool {
					got, ok := carregadosAgora(arquivo)
					return ok && len(got) == 0
				})
				return
			}

			esperarAte(t, 5*time.Second, "Sair", func() bool { return m.app.saindo.Load() })
			antes := lerEstado(t, arquivo)
			m.app.RemoveQueueItem(id)
			m.app.shutdown(nil)
			time.Sleep(1200 * time.Millisecond)
			if !mesmoEstado(t, arquivo, antes) {
				t.Fatal("o shutdown do processo antigo regravou depois da troca")
			}
		})
	}
}

func TestCopyOutput_TXTAusente(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{})
	ligarHistoricoNaConfig(m)
	dir := t.TempDir()
	zip := filepath.Join(dir, cnjTeste+".zip")
	txt := saida.Base(zip)
	escreverTxt(t, txt, "texto")
	it := itemZip(zip, "success")
	it.OutputPath = txt
	gravarHistoricoAMao(t, arquivoHistoricoDe(m), []historico.Item{it})
	m.app.ctx = context.Background()
	m.app.clipboard = func(context.Context, string) error { return nil }

	m.iniciar()
	time.Sleep(500 * time.Millisecond) // a conferência do startup acha o TXT
	if item := itemPorCaminho(m.app, zip); item == nil || item.OutputMissing {
		t.Fatalf("restaurado = %+v; quer presente e sem OutputMissing", item)
	}
	if err := os.Remove(txt); err != nil {
		t.Fatal(err)
	}
	err := m.app.CopyOutput(txt)
	quer := "O TXT não está mais em " + dir + "; ele pode ter sido movido ou apagado"
	if err == nil || err.Error() != quer {
		t.Fatalf("CopyOutput = %v; quer %q", err, quer)
	}
	if item := itemPorCaminho(m.app, zip); item == nil || !item.OutputMissing {
		t.Fatalf("depois do CopyOutput: %+v; quer OutputMissing", item)
	}
}

// M1: no processo antigo, ligar o interruptor depois do Fechar da troca não
// cria gravador aberto (ele gravaria por cima do arquivo do processo novo);
// o Reabrir da reversão traz o gravador de volta.
func TestHistorico_InterruptorDuranteATroca(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := novoAppAtualizacao(t, cenario{})
	arquivo := arquivoHistoricoDe(m)
	m.iniciar()
	zip := filepath.Join(t.TempDir(), "a.zip")
	adicionarETerminar(t, m.app, zip)
	ad := historicoAtualizacao{m.app}
	if err := ad.Fechar(); err != nil {
		t.Fatal(err)
	}

	cfg := *m.app.configAtual()
	cfg.GuardarHistorico = true
	if err := m.app.SaveConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := m.app.AddFilesToQueue([]string{filepath.Join(t.TempDir(), "b.zip")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if existe(arquivo) {
		t.Fatal("ligar o interruptor depois do Fechar da troca gravou o histórico")
	}

	if err := ad.Reabrir(); err != nil {
		t.Fatal(err)
	}
	esperarAte(t, 2*time.Second, "o gravador voltar e gravar depois do Reabrir", func() bool {
		got, ok := carregadosAgora(arquivo)
		return ok && len(got) == 2
	})
}

// M3: sem pastaDados, ligar o histórico (no startup ou pelo interruptor) não
// grava nada e avisa no log.
func TestHistorico_SemPastaDados(t *testing.T) {
	antes, saidaLog, flags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(antes)
		log.SetOutput(saidaLog)
		log.SetFlags(flags)
	})
	buf := &bufferSeguro{}
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Setenv("APPDATA", t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativo := filepath.Join(cwd, "historico.json")
	if existe(relativo) {
		t.Fatalf("já há um %s antes do teste", relativo)
	}

	const linha = "histórico desligado: pasta de dados desconhecida"
	t.Run("startup", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{})
		ligarHistoricoNaConfig(m)
		m.app.pastaDados = ""
		m.iniciar()
		adicionarETerminar(t, m.app, filepath.Join(t.TempDir(), "a.zip"))
		m.app.shutdown(nil)
		if !strings.Contains(buf.String(), linha) {
			t.Fatalf("log sem %q:\n%s", linha, buf.String())
		}
	})
	t.Run("interruptor", func(t *testing.T) {
		m := novoAppAtualizacao(t, cenario{})
		m.app.pastaDados = ""
		m.iniciar()
		adicionarETerminar(t, m.app, filepath.Join(t.TempDir(), "a.zip"))
		n := strings.Count(buf.String(), linha)
		cfg := *m.app.configAtual()
		cfg.GuardarHistorico = true
		if err := m.app.SaveConfig(&cfg); err != nil {
			t.Fatal(err)
		}
		m.app.shutdown(nil)
		if strings.Count(buf.String(), linha) <= n {
			t.Fatalf("ligar o interruptor sem pastaDados não avisou:\n%s", buf.String())
		}
	})
	if existe(relativo) {
		_ = os.Remove(relativo)
		t.Fatal("gravou historico.json na pasta corrente sem pastaDados")
	}
}

// M4: no processo novo da atualização, o historico-*.tmp que o antigo pode
// estar gravando fica até o antigo sair; depois, o gravador novo o apaga.
func TestHistorico_TemporarioDoAntigoFicaAteEleSair(t *testing.T) {
	m := novoAppAtualizacao(t, cenario{apos: 4242, travaLivre: true})
	ligarHistoricoNaConfig(m)
	tmp := filepath.Join(m.app.pastaDados, "historico-123.tmp")
	criarArquivo(t, tmp)

	m.iniciar()
	m.app.domPronto(nil)
	continua(t, 500*time.Millisecond, "o .tmp do antigo continuar antes de ele sair", func() bool {
		return existe(tmp)
	})
	close(m.f.liberaSair)
	esperarAte(t, 3*time.Second, "o .tmp sumir depois de o antigo sair", func() bool { return !existe(tmp) })
}
