package etl

import (
	"slices"
	"sync"
)

// Estados de um documento na atividade de um ZIP.
const (
	DocAguardando = "aguardando" // esperando vaga entre os documentos em paralelo
	DocLendo      = "lendo"      // com vaga, lendo texto nativo ou HTML
	DocOCR        = "ocr"        // com páginas no OCR agora
	DocConcluido  = "concluido"
	DocErro       = "erro"
)

// AtividadeDoc é o estado de um documento do ZIP em processamento.
type AtividadeDoc struct {
	Nome    string `json:"nome"`
	Evento  int    `json:"evento"`
	Estado  string `json:"estado"`
	Paginas int    `json:"paginas"`
	Feitas  int    `json:"feitas"`
	// OCR conta as páginas deste documento num processo de OCR agora.
	OCR  int    `json:"ocr"`
	Erro string `json:"erro,omitempty"`
}

// InstantaneoAtividade é a cópia da atividade enviada ao frontend.
type InstantaneoAtividade struct {
	ItemID     string         `json:"item_id"`
	Docs       []AtividadeDoc `json:"docs"`
	LimiteDocs int            `json:"limite_docs"`
	LimiteOCR  int            `json:"limite_ocr"`
	// OCRAtivos conta os processos de OCR rodando (cada um com uma ou mais páginas).
	OCRAtivos int `json:"ocr_ativos"`
}

// Atividade acompanha, documento a documento, o processamento concorrente de
// um ZIP. Todos os métodos aceitam receptor nil e então não fazem nada, para
// o pipeline rodar sem observador.
type Atividade struct {
	mu         sync.Mutex
	itemID     string
	docs       []AtividadeDoc
	limiteDocs int
	limiteOCR  int
	ocrAtivos  int
	versao     uint64
}

// NovaAtividade cria a atividade de um item da fila.
func NovaAtividade(itemID string) *Atividade {
	return &Atividade{itemID: itemID}
}

func (a *Atividade) iniciar(files []AtividadeDoc, limiteDocs, limiteOCR int) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.docs = files
	a.limiteDocs, a.limiteOCR = limiteDocs, limiteOCR
	a.ocrAtivos = 0
	a.versao++
}

func (a *Atividade) mudar(i int, fn func(d *AtividadeDoc)) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if i < 0 || i >= len(a.docs) {
		return
	}
	fn(&a.docs[i])
	a.versao++
}

func (a *Atividade) docLendo(i int) {
	a.mudar(i, func(d *AtividadeDoc) { d.Estado = DocLendo })
}

func (a *Atividade) docPagina(i int) {
	a.mudar(i, func(d *AtividadeDoc) {
		d.Feitas++
		if d.Feitas > d.Paginas {
			d.Paginas = d.Feitas
		}
	})
}

// ocrInicio marca n páginas do documento i num processo de OCR que começou.
func (a *Atividade) ocrInicio(i, n int) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ocrAtivos++
	if i >= 0 && i < len(a.docs) {
		a.docs[i].OCR += n
		a.docs[i].Estado = DocOCR
	}
	a.versao++
}

func (a *Atividade) ocrFim(i, n int) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ocrAtivos = max(a.ocrAtivos-1, 0)
	if i >= 0 && i < len(a.docs) {
		d := &a.docs[i]
		d.OCR = max(d.OCR-n, 0)
		if d.OCR == 0 && d.Estado == DocOCR {
			d.Estado = DocLendo
		}
	}
	a.versao++
}

// docFim encerra o documento: concluído, ou com erro se msgErro não for vazia.
func (a *Atividade) docFim(i int, msgErro string) {
	a.mudar(i, func(d *AtividadeDoc) {
		d.OCR = 0
		if msgErro != "" {
			d.Estado, d.Erro = DocErro, msgErro
			return
		}
		d.Estado = DocConcluido
		if d.Feitas > d.Paginas {
			d.Paginas = d.Feitas
		}
	})
}

// Versao muda a cada alteração; quem envia ao frontend compara para não
// repetir um instantâneo igual.
func (a *Atividade) Versao() uint64 {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.versao
}

// Instantaneo devolve uma cópia independente do estado atual.
func (a *Atividade) Instantaneo() *InstantaneoAtividade {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return &InstantaneoAtividade{
		ItemID:     a.itemID,
		Docs:       slices.Clone(a.docs),
		LimiteDocs: a.limiteDocs,
		LimiteOCR:  a.limiteOCR,
		OCRAtivos:  a.ocrAtivos,
	}
}
