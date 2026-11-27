package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"eprocia/internal/historico"
	"eprocia/internal/models"
)

// eventoHistoricoFalha avisa o frontend, uma vez por sessão, que o histórico
// não pôde ser gravado. Vai sem dados: a interface mostra só a frase fixa, e o
// erro (com caminho e nome do usuário) fica no log.
const eventoHistoricoFalha = "historico:falha"

// arquivoHistorico é o historico.json na pasta de dados; "" sem pastaDados.
func (a *App) arquivoHistorico() string {
	if a.pastaDados == "" {
		return ""
	}
	return filepath.Join(a.pastaDados, "historico.json")
}

// existeNoDisco diz se o caminho está no disco; um erro que não é "não
// existe" (permissão, disco ocupado) conta como presente, para não marcar
// ausente o que só não pôde ser lido.
func existeNoDisco(caminho string) bool {
	_, err := os.Stat(caminho)
	return !errors.Is(err, fs.ErrNotExist)
}

// aoFalharHistorico é o AoFalhar do gravador. Pode rodar com q.emitMu travado
// (GravarAgora, Fechar): não chama a fila nem reagenda a gravação (com o
// disco falhando, viraria laço); só guarda a falha e emite o aviso, uma vez
// por sessão. O gravador já registrou o erro no log.
func (a *App) aoFalharHistorico(error) {
	a.falhaHistoricoMu.Lock()
	primeira := !a.historicoFalhou
	a.historicoFalhou = true
	a.falhaHistoricoMu.Unlock()
	if primeira {
		a.emitirEm(a.ctx, eventoHistoricoFalha)
	}
}

// novoGravadorHistorico cria o gravador; NovoGravador apaga os temporários
// historico-*.tmp da pasta, por isso, no processo novo da atualização, só
// depois de o antigo sair.
func (a *App) novoGravadorHistorico(arquivo string, protegido bool) *historico.Gravador {
	return historico.NovoGravador(arquivo, historico.Opcoes{Protegido: protegido, AoFalhar: a.aoFalharHistorico})
}

// iniciarHistorico carrega o histórico e o restaura na fila antes de o
// watcher ligar, para a varredura reaproveitar as linhas interrompidas. No
// processo novo da atualização (apos > 0), o gravador fica para
// ligarHistoricoAdiado. Desligado, não lê nem grava nada.
func (a *App) iniciarHistorico(cfg *models.Config) {
	a.historicoMu.Lock()
	defer a.historicoMu.Unlock()
	a.historicoAdiado = a.apos > 0
	if cfg == nil || !cfg.GuardarHistorico {
		return
	}
	arquivo := a.arquivoHistorico()
	if arquivo == "" {
		slog.Warn("histórico desligado: pasta de dados desconhecida")
		return
	}
	itens, err := historico.Carregar(arquivo)
	// Sem ler o arquivo, uma sessão sem entrada não pode gravar lista vazia
	// por cima dele: o gravador nasce protegido.
	protegido := err != nil
	switch {
	case errors.Is(err, historico.ErrVersaoNova):
		slog.Warn("histórico de uma versão mais nova do eprocIA: a lista abre vazia e o arquivo fica até haver um item")
	case err != nil:
		slog.Warn("não foi possível ler o histórico; a lista abre vazia", "erro", err)
	}
	if len(itens) > 0 {
		a.queue.Restaurar(itens)
	}
	if a.historicoAdiado {
		a.historicoProtegido = protegido
	} else {
		a.historico = a.novoGravadorHistorico(arquivo, protegido)
		a.queue.SetGravador(a.historico)
	}
	if len(itens) > 0 {
		go a.queue.ConferirArquivos(existeNoDisco)
	}
}

// ligarHistoricoAdiado, no processo novo da atualização, liga o gravador
// depois de o antigo sair (e antes de soltar a fila) e agenda a lista atual,
// para uma remoção feita na espera chegar ao arquivo. Lê a config de agora:
// o interruptor pode ter mudado na espera.
func (a *App) ligarHistoricoAdiado() {
	a.historicoMu.Lock()
	defer a.historicoMu.Unlock()
	if !a.historicoAdiado {
		return
	}
	a.historicoAdiado = false
	cfg := a.configAtual()
	if cfg == nil || !cfg.GuardarHistorico || a.queue == nil {
		return
	}
	arquivo := a.arquivoHistorico()
	if arquivo == "" {
		slog.Warn("histórico desligado: pasta de dados desconhecida")
		return
	}
	a.historico = a.novoGravadorHistorico(arquivo, a.historicoProtegido)
	a.queue.SetGravador(a.historico)
	a.queue.AgendarHistorico()
}

// aplicarHistorico trata o interruptor (SaveConfig). Ligar cria o gravador e
// grava na hora a lista atual (no processo novo da atualização, ainda à
// espera do antigo, quem liga é ligarHistoricoAdiado). Desligar fecha o
// gravador, apaga o arquivo e solta o gravador da fila; os itens na tela
// ficam até fechar.
func (a *App) aplicarHistorico(ligar bool) {
	if a.queue == nil {
		return
	}
	a.historicoMu.Lock()
	defer a.historicoMu.Unlock()
	arquivo := a.arquivoHistorico()
	if !ligar {
		a.historicoProtegido = false
		if a.historico != nil {
			_ = a.historico.FecharEApagar() // a falha já foi ao log
			a.queue.SetGravador(nil)
			a.historico = nil
			return
		}
		if arquivo != "" {
			if err := os.Remove(arquivo); err != nil && !errors.Is(err, fs.ErrNotExist) {
				slog.Warn("não foi possível apagar o histórico", "erro", err)
			}
		}
		return
	}
	// Fechado pela troca de versão: um gravador aberto aqui gravaria por cima
	// do arquivo que o processo novo carrega. Só a config muda; o Reabrir da
	// reversão cria o gravador, se a troca falhar.
	if a.historico != nil || a.historicoAdiado || a.historicoFechadoTroca {
		return
	}
	a.ligarGravadorLocked(arquivo)
	_ = a.queue.GravarHistoricoAgora() // a falha já foi ao log e ao aviso
}

// ligarGravadorLocked cria o gravador (sem proteção) e o entrega à fila.
// Exige a.historicoMu travado.
func (a *App) ligarGravadorLocked(arquivo string) bool {
	if arquivo == "" {
		slog.Warn("histórico desligado: pasta de dados desconhecida")
		return false
	}
	a.historico = a.novoGravadorHistorico(arquivo, false)
	a.queue.SetGravador(a.historico)
	return true
}

// historicoAtualizacao entrega o histórico ao atualizacao.Ambiente: fecha
// antes do reinício e reabre se o reinício falhar.
type historicoAtualizacao struct{ a *App }

// Fechar grava a lista final e fecha o gravador. Uma falha de escrita não
// impede a troca (decisão d2): vai ao log e devolve nil; o processo novo
// carrega a última escrita que deu certo.
func (h historicoAtualizacao) Fechar() error {
	if h.a.queue == nil {
		return nil
	}
	// A marca vem antes da escrita final: daí em diante o interruptor não
	// cria gravador aberto neste processo (aplicarHistorico).
	h.a.historicoMu.Lock()
	h.a.historicoFechadoTroca = true
	h.a.historicoMu.Unlock()
	if err := h.a.queue.FecharHistorico(); err != nil {
		slog.Warn("atualização: o histórico não foi gravado", "erro", err)
	}
	return nil
}

// Reabrir tira a marca do Fechar, volta a aceitar gravações e agenda a lista
// atual, para que uma remoção ou limpeza feita na espera do reinício chegue
// ao arquivo. Se o interruptor foi ligado na espera, cria o gravador agora.
func (h historicoAtualizacao) Reabrir() error {
	if h.a.queue == nil {
		return nil
	}
	h.a.historicoMu.Lock()
	h.a.historicoFechadoTroca = false
	g := h.a.historico
	if g == nil && !h.a.historicoAdiado {
		if cfg := h.a.configAtual(); cfg != nil && cfg.GuardarHistorico && h.a.ligarGravadorLocked(h.a.arquivoHistorico()) {
			g = h.a.historico
		}
	}
	h.a.historicoMu.Unlock()
	if g != nil {
		if err := g.Reabrir(); err != nil {
			slog.Warn("atualização: o histórico não foi reaberto", "erro", err)
		}
	}
	h.a.queue.AgendarHistorico()
	return nil
}

// HistoricoFalhou diz se alguma gravação do histórico falhou nesta sessão.
// O frontend consulta no onMount, depois de registrar o EventsOn de
// historico:falha: o evento pode ter saído antes e se perdido.
func (a *App) HistoricoFalhou() bool {
	a.falhaHistoricoMu.Lock()
	defer a.falhaHistoricoMu.Unlock()
	return a.historicoFalhou
}
