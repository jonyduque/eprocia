package etl

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"eprocia/internal/models"
	"eprocia/internal/ocr"
	"eprocia/internal/registro"
)

// medidasOCR acumula os processos de OCR de um ZIP. Os contadores são
// atômicos; só a lista de durações dos blocos (para mediana e p95) fica sob
// mutex. O valor zero está pronto para uso.
type medidasOCR struct {
	processos, blocos, paginasEmBloco, paginas atomic.Int64
	refacoes, prazos                           atomic.Int64
	trabalhoNs, blocosNs                       atomic.Int64
	// Parede: o menor início e o maior fim, em ns desde a época (0 = nenhum).
	inicioMin, fimMax atomic.Int64

	mu            sync.Mutex
	duracoesBloco []time.Duration
}

// registrar conta um processo de OCR de paginas páginas que levou dur. Um
// processo com mais de uma página é um bloco.
func (m *medidasOCR) registrar(paginas int, dur time.Duration, err error) {
	if m == nil {
		return
	}
	m.processos.Add(1)
	m.paginas.Add(int64(paginas))
	m.trabalhoNs.Add(int64(dur))
	if paginas > 1 {
		m.blocos.Add(1)
		m.paginasEmBloco.Add(int64(paginas))
		m.blocosNs.Add(int64(dur))
		m.mu.Lock()
		m.duracoesBloco = append(m.duracoesBloco, dur)
		m.mu.Unlock()
	}
	if errors.Is(err, ocr.ErrPrazo) {
		m.prazos.Add(1)
	}
	fim := time.Now().UnixNano()
	inicio := fim - int64(dur)
	for {
		v := m.inicioMin.Load()
		if (v != 0 && v <= inicio) || m.inicioMin.CompareAndSwap(v, inicio) {
			break
		}
	}
	for {
		v := m.fimMax.Load()
		if v >= fim || m.fimMax.CompareAndSwap(v, fim) {
			break
		}
	}
}

// refacao conta um bloco de paginas páginas que falhou e vai ser refeito
// página a página. As páginas saem da contagem do bloco falho, porque cada uma
// volta a contar na refação; o tempo do bloco falho fica no trabalho.
func (m *medidasOCR) refacao(paginas int) {
	if m == nil {
		return
	}
	m.refacoes.Add(1)
	m.paginas.Add(-int64(paginas))
}

// resumo devolve a parte de OCR do relatório e os tempos de trabalho e de
// parede, em ms. Sem processo, tudo é zero (nenhuma divisão por zero).
func (m *medidasOCR) resumo() (o models.OCRRelatorio, trabalhoMs, paredeMs int64) {
	if m == nil {
		return
	}
	o.Processos = int(m.processos.Load())
	o.Blocos = int(m.blocos.Load())
	o.PaginasEmBloco = int(m.paginasEmBloco.Load())
	o.Refacoes = int(m.refacoes.Load())
	o.Prazos = int(m.prazos.Load())
	trabalho := m.trabalhoNs.Load()
	if n := m.paginas.Load(); n > 0 {
		o.PaginaEstimadaMs = time.Duration(trabalho / n).Milliseconds()
	}
	if o.Blocos > 0 {
		o.BlocoMedioMs = time.Duration(m.blocosNs.Load() / int64(o.Blocos)).Milliseconds()
	}
	m.mu.Lock()
	duracoes := slices.Clone(m.duracoesBloco)
	m.mu.Unlock()
	if n := len(duracoes); n > 0 {
		slices.Sort(duracoes)
		mediana := duracoes[n/2]
		if n%2 == 0 {
			mediana = (duracoes[n/2-1] + duracoes[n/2]) / 2
		}
		o.BlocoMedianaMs = mediana.Milliseconds()
		// p95 pelo posto mais próximo: o menor valor com 95% dos blocos até ele.
		o.BlocoP95Ms = duracoes[(95*n+99)/100-1].Milliseconds()
	}
	if ini, fim := m.inicioMin.Load(), m.fimMax.Load(); ini != 0 && fim > ini {
		paredeMs = time.Duration(fim - ini).Milliseconds()
	}
	return o, time.Duration(trabalho).Milliseconds(), paredeMs
}

// comMedidas faz o despacho de OCR contar em m os blocos refeitos página a página.
func (p *PDFParser) comMedidas(m *medidasOCR) *PDFParser {
	p.medidas = m
	return p
}

// ocrDoZip monta os executores de OCR de cada documento de um ZIP: todos
// dividem o semáforo sem, marcam as páginas em OCR na atividade só depois de
// conseguir a vaga e registram cada processo em medidas.
type ocrDoZip struct {
	sem     chan struct{}
	atv     *Atividade
	modo    string
	medidas *medidasOCR
}

func (o *ocrDoZip) doDoc(i int) (func(context.Context, string, int) (string, error), ocrLote) {
	runner := func(ctx context.Context, pdfPath string, pageNum int) (string, error) {
		select {
		case o.sem <- struct{}{}:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		defer func() { <-o.sem }()
		o.atv.ocrInicio(i, 1)
		defer o.atv.ocrFim(i, 1)
		inicio := time.Now()
		texto, err := executarOCR(ctx, pdfPath, pageNum, o.modo)
		o.medidas.registrar(1, time.Since(inicio), err)
		return texto, err
	}
	lote := func(ctx context.Context, pdfPath string, paginas []int, aoConcluir func(int)) ([]string, error) {
		select {
		case o.sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-o.sem }()
		o.atv.ocrInicio(i, len(paginas))
		var prontas atomic.Int32
		defer func() { o.atv.ocrFim(i, len(paginas)-int(prontas.Load())) }()
		inicio := time.Now()
		textos, err := executarOCRPaginas(ctx, pdfPath, paginas, o.modo, func(pagina int) {
			// A página sai da contagem de OCR do documento assim que fica pronta;
			// o processo segue contado em OCRAtivos até o lote terminar.
			if int(prontas.Add(1)) <= len(paginas) {
				o.atv.mudar(i, func(d *AtividadeDoc) { d.OCR = max(d.OCR-1, 0) })
			}
			if aoConcluir != nil {
				aoConcluir(pagina)
			}
		})
		dur := time.Since(inicio)
		o.medidas.registrar(len(paginas), dur, err)
		registro.Logger(ctx).Debug("bloco de OCR", "paginas", len(paginas), "duracao_ms", dur.Milliseconds())
		return textos, err
	}
	return runner, lote
}

// Estado final de um documento no relatório.
const (
	estatPendente = iota // não terminou (cancelamento)
	estatLido
	estatNaoLido
	estatOmitido
)

// estatDoc é o resumo de um documento, escrito só pela goroutine dele e lido
// depois de wg.Wait().
type estatDoc struct {
	estado     int
	leitura    Leitura
	duracao    time.Duration // da vaga ao fim
	paginas    int           // de um não lido: as do mapa, como no pages_count da fila
	categorias map[string]int
	porMotivo  map[string]int
	semGanho   int
	comReserva int
	// Números das páginas que pedem atenção.
	semOCR, comErro, falhouReserva []int
}

// estatDeLido resume as páginas (já com categoria) de um documento lido.
func estatDeLido(paginas map[string]any) estatDoc {
	e := estatDoc{estado: estatLido, categorias: map[string]int{}}
	for _, v := range paginas {
		p, _ := v.(map[string]any)
		if p == nil {
			continue
		}
		idx, _ := p["index"].(int)
		cat, _ := p["categoria"].(string)
		if cat == "" {
			cat = categoriaPagina(p)
		}
		e.categorias[cat]++
		switch cat {
		case CatSemOCR:
			e.semOCR = append(e.semOCR, idx)
		case CatComErro:
			e.comErro = append(e.comErro, idx)
		}
		if _, ok := p["ocr_sem_ganho"]; ok {
			e.semGanho++
		}
		if _, ok := p["ocr_aviso"]; ok {
			e.comReserva++
			e.falhouReserva = append(e.falhouReserva, idx)
		}
		if m, ok := p["motivo_ocr"].(string); ok {
			if e.porMotivo == nil {
				e.porMotivo = map[string]int{}
			}
			e.porMotivo[m]++
		}
	}
	slices.Sort(e.semOCR)
	slices.Sort(e.comErro)
	slices.Sort(e.falhouReserva)
	return e
}

// maxPaginasAtencao limita cada lista de páginas de DocAtencao.
const maxPaginasAtencao = 50

// primeirasPaginas devolve no máximo maxPaginasAtencao números (os primeiros).
func primeirasPaginas(paginas []int) []int {
	if len(paginas) > maxPaginasAtencao {
		return slices.Clip(paginas[:maxPaginasAtencao])
	}
	return paginas
}

// fasesZip são os tempos medidos pelo pipeline fora do OCR.
type fasesZip struct {
	abrir, preContagem, leituraParede, formatacao, gravacao time.Duration
}

// montarRelatorio soma, numa só goroutine e depois de wg.Wait(), as
// estatísticas dos documentos.
func montarRelatorio(files []models.ProcessFile, contagens []int, tiposOmitidos []string, estats []estatDoc,
	s SaidaConsolidada, med *medidasOCR, fases fasesZip) *models.RelatorioZip {
	r := &models.RelatorioZip{Limpeza: s.Limpeza, BytesTxt: int64(len(s.Texto)), Tokens: s.Tokens}
	d, p := &r.Documentos, &r.Paginas
	d.Total = len(files)
	d.DuplicadosH7 = s.DocsDuplicados
	p.Duplicadas = s.PaginasDuplicadas
	var leituraSoma time.Duration
	for i, f := range files {
		if f.Extension == ".pdf" {
			d.PDF++
		} else {
			d.HTML++
		}
		e := estats[i]
		leituraSoma += e.duracao
		switch e.estado {
		case estatOmitido:
			d.Omitidos++
			p.DeOmitidos += contagens[i]
			if !slices.Contains(d.TiposOmitidos, tiposOmitidos[i]) {
				d.TiposOmitidos = append(d.TiposOmitidos, tiposOmitidos[i])
			}
		case estatNaoLido:
			d.NaoLidos++
			p.DeNaoLidos += e.paginas
			if d.PorLeitura == nil {
				d.PorLeitura = map[string]int{}
			}
			d.PorLeitura[string(e.leitura)]++
		case estatLido:
			d.Lidos++
			p.LidaNativa += e.categorias[CatLidaNativa]
			p.LidaOCR += e.categorias[CatLidaOCR]
			p.Vazia += e.categorias[CatVazia]
			p.SemOCR += e.categorias[CatSemOCR]
			p.ComErro += e.categorias[CatComErro]
			p.OCRSemGanho += e.semGanho
			p.OCRFalhouComReserva += e.comReserva
			for m, n := range e.porMotivo {
				if p.PorMotivo == nil {
					p.PorMotivo = map[string]int{}
				}
				p.PorMotivo[m] += n
			}
			if len(e.semOCR)+len(e.comErro)+len(e.falhouReserva) > 0 {
				r.Atencao = append(r.Atencao, models.DocAtencao{Nome: f.DocName, Evento: f.EventNum,
					SemOCR: primeirasPaginas(e.semOCR), ComErro: primeirasPaginas(e.comErro),
					OCRFalhouComReserva:      primeirasPaginas(e.falhouReserva),
					TotalSemOCR:              len(e.semOCR),
					TotalComErro:             len(e.comErro),
					TotalOCRFalhouComReserva: len(e.falhouReserva)})
			}
		}
	}
	p.Total = p.LidaNativa + p.LidaOCR + p.Vazia + p.SemOCR + p.ComErro
	// Atencao sai na ordem dos documentos (files já vem ordenado por evento).
	slices.Sort(d.TiposOmitidos)

	var trabalho, parede int64
	r.OCR, trabalho, parede = med.resumo()
	r.Tempo = models.TempoRelatorio{
		AbrirMs:         fases.abrir.Milliseconds(),
		PreContagemMs:   fases.preContagem.Milliseconds(),
		LeituraParedeMs: fases.leituraParede.Milliseconds(),
		LeituraSomaMs:   leituraSoma.Milliseconds(),
		OCRTrabalhoMs:   trabalho,
		OCRParedeMs:     parede,
		FormatacaoMs:    fases.formatacao.Milliseconds(),
		GravacaoMs:      fases.gravacao.Milliseconds(),
	}
	return r
}

// semOCRDosLidos soma as páginas sem_ocr dos documentos lidos: o N da linha
// do cabeçalho.
func semOCRDosLidos(estats []estatDoc) int {
	n := 0
	for _, e := range estats {
		if e.estado == estatLido {
			n += e.categorias[CatSemOCR]
		}
	}
	return n
}

// definirRelatorio guarda o relatório do ZIP; aceita receptor nil.
func (a *Atividade) definirRelatorio(r *models.RelatorioZip) {
	if a == nil {
		return
	}
	a.relatorio.Store(r)
}

// Relatorio devolve o relatório do ZIP, ou nil se ainda não há (ou a é nil).
func (a *Atividade) Relatorio() *models.RelatorioZip {
	if a == nil {
		return nil
	}
	return a.relatorio.Load()
}
