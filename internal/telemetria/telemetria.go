// Package telemetria grava, só com EPROCIA_LOG definido, um registro local
// por ZIP processado: contagens, tempos e classes, sem nenhum nome (de ZIP,
// de documento, de pasta) nem texto de página. Não há envio: o pacote não
// importa net (TestTelemetria_SemRede), e por isso não importa internal/etl
// (que puxa net pelo runtime do Wails) nem golang.org/x/sys/windows.
package telemetria

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"eprocia/internal/models"
	"eprocia/internal/registro"
)

// Maquina descreve o computador, sem nome do computador nem do usuário.
type Maquina struct {
	CPUs         int    `json:"cpus"`
	MemoriaTotal uint64 `json:"memoria_total"`
	Windows      string `json:"windows,omitempty"`
	Build        uint32 `json:"build,omitempty"`
	Arquitetura  string `json:"arquitetura"`
}

// Registro é uma linha de telemetria.jsonl. Só leva números e valores de
// enumerações conhecidas: nunca copia Atencao, TiposOmitidos nem nome algum.
type Registro struct {
	Versao     string                `json:"versao"`
	TS         string                `json:"ts"`
	ItemID     string                `json:"item_id"`
	Reprocesso bool                  `json:"reprocesso"`
	Maquina    Maquina               `json:"maquina"`
	Config     ConfigRegistro        `json:"config"`
	Tamanhos   Tamanhos              `json:"tamanhos"`
	TemposMs   models.TempoRelatorio `json:"tempos_ms"`
	Paginas    PaginasRegistro       `json:"paginas"`
	OCR        models.OCRRelatorio   `json:"ocr"`
	Limpeza    LimpezaRegistro       `json:"limpeza"`
	Falhas     Falhas                `json:"falhas"`
	Resultado  Resultado             `json:"resultado"`
}

// ConfigRegistro é a config efetiva do ZIP, sem a pasta monitorada e sem os
// nomes dos tipos omitidos (só a quantidade).
type ConfigRegistro struct {
	OcrMode       string `json:"ocr_mode"`
	CleaningLevel string `json:"cleaning_level"`
	ForcarOCR     bool   `json:"forcar_ocr"`
	TiposOmitidos int    `json:"tipos_omitidos"`
	Origem        string `json:"origem"`
}

// Tamanhos traz o tamanho do ZIP, a contagem de documentos e de páginas e o
// tamanho do .txt.
type Tamanhos struct {
	BytesZip int64 `json:"bytes_zip"`
	PDF      int   `json:"pdf"`
	HTML     int   `json:"html"`
	Paginas  int   `json:"paginas"`
	BytesTxt int64 `json:"bytes_txt"`
	Tokens   int   `json:"tokens"`
}

// PaginasRegistro conta as páginas por categoria, por motivo de OCR e os
// sub-contadores.
type PaginasRegistro struct {
	Total               int            `json:"total"`
	LidaNativa          int            `json:"lida_nativa"`
	LidaOCR             int            `json:"lida_ocr"`
	Vazia               int            `json:"vazia"`
	SemOCR              int            `json:"sem_ocr"`
	ComErro             int            `json:"com_erro"`
	OCRSemGanho         int            `json:"ocr_sem_ganho"`
	OCRFalhouComReserva int            `json:"ocr_falhou_com_reserva"`
	DeNaoLidos          int            `json:"de_nao_lidos"`
	DeOmitidos          int            `json:"de_omitidos"`
	PorMotivo           map[string]int `json:"por_motivo,omitempty"`
}

// LimpezaRegistro resume a limpeza. ReducaoPct só existe com nível diferente
// de none e bytes antes > 0.
type LimpezaRegistro struct {
	BytesAntes      int64    `json:"bytes_antes"`
	BytesDepois     int64    `json:"bytes_depois"`
	BytesDuplicatas int64    `json:"bytes_duplicatas"`
	TokensAntes     int      `json:"tokens_antes"`
	TokensDepois    int      `json:"tokens_depois"`
	ReducaoPct      *float64 `json:"reducao_pct,omitempty"`
	Duplicadas      int      `json:"duplicadas"`
	DocsDuplicados  int      `json:"docs_duplicados"`
	Omitidos        int      `json:"omitidos"`
}

// Falhas conta os documentos não lidos por Leitura.
type Falhas struct {
	PorLeitura map[string]int `json:"por_leitura,omitempty"`
}

// Resultado é o desfecho do ZIP.
type Resultado struct {
	Status     string `json:"status"`
	ClasseErro string `json:"classe_erro,omitempty"`
}

var (
	modosOCR  = []string{"none", "fast", "normal", "best"}
	niveis    = []string{"none", "basic", "standard", "aggressive"}
	origens   = []string{"monitorada", "arrastada", "manual"}
	statusZip = []string{"success", "error", "cancelled"}
	// reClasse aceita só identificadores curtos (classes, motivos e leituras
	// são enumerações em snake_case); qualquer outro texto é descartado, para
	// que nada livre (um nome, um caminho) chegue ao arquivo por engano.
	reClasse = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

// enumOu devolve v se for um dos valores conhecidos, senão "".
func enumOu(v string, validos []string) string {
	if slices.Contains(validos, v) {
		return v
	}
	return ""
}

func classe(v string) string {
	if reClasse.MatchString(v) {
		return v
	}
	return ""
}

func copiarContagens(m map[string]int) map[string]int {
	if len(m) == 0 {
		return nil
	}
	c := make(map[string]int, len(m))
	for k, v := range m {
		if reClasse.MatchString(k) {
			c[k] = v
		}
	}
	if len(c) == 0 {
		return nil
	}
	return c
}

// Montar monta o registro de uma execução. Copia campo a campo só números e
// enumerações; Atencao, TiposOmitidos, a pasta monitorada e qualquer nome
// ficam de fora.
func Montar(e models.ExecucaoZip, m Maquina, versao string, agora time.Time) Registro {
	cfg := e.Config
	ocrMode := enumOu(cfg.OcrMode, modosOCR)
	r := Registro{
		Versao:     versao,
		TS:         agora.Format(time.RFC3339),
		ItemID:     e.ItemID,
		Reprocesso: e.Reprocesso,
		Maquina:    m,
		Config: ConfigRegistro{
			OcrMode:       ocrMode,
			CleaningLevel: enumOu(cfg.CleaningLevel, niveis),
			ForcarOCR:     cfg.ForcarOCR && cfg.OcrMode != "none",
			TiposOmitidos: len(cfg.OmitirTipos),
			Origem:        enumOu(e.Origem, origens),
		},
		Tamanhos:  Tamanhos{BytesZip: e.BytesZip},
		Resultado: Resultado{Status: enumOu(e.Status, statusZip), ClasseErro: classe(e.ClasseErro)},
	}
	rel := e.Relatorio
	if rel == nil {
		return r
	}
	r.Tamanhos.PDF = rel.Documentos.PDF
	r.Tamanhos.HTML = rel.Documentos.HTML
	r.Tamanhos.Paginas = rel.Paginas.Total
	r.Tamanhos.BytesTxt = rel.BytesTxt
	r.Tamanhos.Tokens = rel.Tokens
	r.TemposMs = rel.Tempo
	r.OCR = rel.OCR
	p := rel.Paginas
	r.Paginas = PaginasRegistro{
		Total: p.Total, LidaNativa: p.LidaNativa, LidaOCR: p.LidaOCR, Vazia: p.Vazia,
		SemOCR: p.SemOCR, ComErro: p.ComErro, OCRSemGanho: p.OCRSemGanho,
		OCRFalhouComReserva: p.OCRFalhouComReserva, DeNaoLidos: p.DeNaoLidos,
		DeOmitidos: p.DeOmitidos, PorMotivo: copiarContagens(p.PorMotivo),
	}
	l := rel.Limpeza
	r.Limpeza = LimpezaRegistro{
		BytesAntes: l.BytesAntes, BytesDepois: l.BytesDepois, BytesDuplicatas: l.BytesDuplicatas,
		TokensAntes: l.TokensAntes, TokensDepois: l.TokensDepois,
		Duplicadas: p.Duplicadas, DocsDuplicados: rel.Documentos.DuplicadosH7,
		Omitidos: rel.Documentos.Omitidos,
	}
	nivel := l.Nivel
	if nivel == "" {
		nivel = cfg.CleaningLevel
	}
	if nivel != "none" && l.BytesAntes > 0 {
		pct := math.Round((1-float64(l.BytesDepois)/float64(l.BytesAntes))*1000) / 10
		r.Limpeza.ReducaoPct = &pct
	}
	r.Falhas = Falhas{PorLeitura: copiarContagens(rel.Documentos.PorLeitura)}
	return r
}

const (
	// limiteArquivo é o tamanho em que telemetria.jsonl gira.
	limiteArquivo = 10 << 20
	// arquivados é quantos telemetria.N.jsonl ficam; não há apagamento por idade.
	arquivados = 5
	// tamanhoCanal é quantos registros esperam a goroutine gravadora.
	tamanhoCanal = 64
)

// Gravador grava os registros em telemetria.jsonl, uma linha por registro,
// numa goroutine própria: Gravar nunca bloqueia. Canal cheio descarta o
// registro e conta; falha de gravação descarta o registro e vira um WARN no
// log, uma vez até a próxima gravação bem-sucedida.
type Gravador struct {
	destino io.WriteCloser
	ch      chan []byte
	feito   chan struct{}

	mu      sync.RWMutex
	fechado bool

	perdidos atomic.Int64
}

// NovoGravador abre (ou cria) <pastaDados>\telemetria\telemetria.jsonl, com
// rotação a 10 MB e 5 arquivados.
func NovoGravador(pastaDados string) (*Gravador, error) {
	return abrirGravador(pastaDados, limiteArquivo)
}

func abrirGravador(pastaDados string, limite int64) (*Gravador, error) {
	dir := filepath.Join(pastaDados, "telemetria")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	a, err := registro.AbrirRotativo(dir, "telemetria", ".jsonl", limite, arquivados)
	if err != nil {
		return nil, err
	}
	return iniciarGravador(a), nil
}

func iniciarGravador(destino io.WriteCloser) *Gravador {
	g := &Gravador{destino: destino, ch: make(chan []byte, tamanhoCanal), feito: make(chan struct{})}
	go g.rodar()
	return g
}

// Gravar enfileira r sem bloquear; depois de Fechar, descarta.
func (g *Gravador) Gravar(r Registro) {
	b, err := json.Marshal(r)
	if err != nil {
		g.perdidos.Add(1)
		return
	}
	b = append(b, '\n')
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.fechado {
		g.perdidos.Add(1)
		return
	}
	select {
	case g.ch <- b:
	default:
		g.perdidos.Add(1)
	}
}

func (g *Gravador) rodar() {
	defer close(g.feito)
	avisou := false
	// parcial marca uma escrita que deixou meia linha no arquivo: a próxima
	// começa com "\n", para que o registro seguinte saia numa linha inteira.
	parcial := false
	for b := range g.ch {
		if parcial {
			b = append([]byte{'\n'}, b...)
		}
		n, err := g.destino.Write(b)
		switch {
		case err == nil:
			parcial = false
		case n > 0 && n < len(b):
			parcial = true
		case n >= len(b):
			parcial = false
		}
		if err != nil {
			g.perdidos.Add(1)
			if !avisou {
				slog.Warn("falha ao gravar a telemetria; registro descartado", "erro", err)
				avisou = true
			}
			continue
		}
		avisou = false
	}
}

// Fechar grava o que está no canal e fecha o arquivo. Pode ser chamado mais
// de uma vez.
func (g *Gravador) Fechar() {
	g.mu.Lock()
	if g.fechado {
		g.mu.Unlock()
		return
	}
	g.fechado = true
	close(g.ch)
	g.mu.Unlock()
	<-g.feito
	if err := g.destino.Close(); err != nil {
		slog.Warn("falha ao fechar a telemetria", "erro", err)
	}
	if n := g.perdidos.Load(); n > 0 {
		slog.Info(fmt.Sprintf("telemetria: %d registros descartados nesta sessão", n), "registros_perdidos", n)
	}
}
