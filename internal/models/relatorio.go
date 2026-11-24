package models

// LimpezaRelatorio resume o efeito da limpeza sobre o texto dos documentos
// lidos: bytes e tokens estimados antes e depois, e os bytes das páginas que
// viraram referência de duplicata (H7). Fora das somas ficam páginas sem OCR,
// documentos omitidos e não lidos.
type LimpezaRelatorio struct {
	Nivel           string `json:"nivel"`
	BytesAntes      int64  `json:"bytes_antes"`
	BytesDepois     int64  `json:"bytes_depois"`
	BytesDuplicatas int64  `json:"bytes_duplicatas"`
	TokensAntes     int    `json:"tokens_antes"`
	TokensDepois    int    `json:"tokens_depois"`
}

// RelatorioZip resume o processamento de um ZIP: documentos, páginas por
// categoria, tempos por fase, OCR, limpeza e o .txt. É o contrato JSON do
// painel (queue:item) e da telemetria. É imutável depois de
// Atividade.definirRelatorio: copiarItem divide o mesmo ponteiro entre as
// cópias do item, então ninguém o altera depois de definido.
type RelatorioZip struct {
	Documentos DocsRelatorio    `json:"documentos"`
	Paginas    PaginasRelatorio `json:"paginas"`
	Tempo      TempoRelatorio   `json:"tempo"`
	OCR        OCRRelatorio     `json:"ocr"`
	Limpeza    LimpezaRelatorio `json:"limpeza"`
	BytesTxt   int64            `json:"bytes_txt"`
	Tokens     int              `json:"tokens"`
	Atencao    []DocAtencao     `json:"atencao,omitempty"`
}

// DocsRelatorio conta os documentos do ZIP. PorLeitura conta os não lidos por
// Leitura (que, para um documento que falhou, é a classe de classificarErro).
type DocsRelatorio struct {
	Total         int            `json:"total"`
	Lidos         int            `json:"lidos"`
	NaoLidos      int            `json:"nao_lidos"`
	Omitidos      int            `json:"omitidos"`
	DuplicadosH7  int            `json:"duplicados_h7"`
	PDF           int            `json:"pdf"`
	HTML          int            `json:"html"`
	TiposOmitidos []string       `json:"tipos_omitidos,omitempty"`
	PorLeitura    map[string]int `json:"por_leitura,omitempty"`
}

// PaginasRelatorio conta as páginas dos documentos lidos por categoria (a soma
// das cinco é Total) e, à parte, os sub-contadores, as páginas de documentos
// não lidos e omitidos, as duplicadas (H7) e as páginas por motivo de OCR.
// OCRFalhouComReserva conta as páginas com ocr_aviso, inclusive a que só foi
// ao OCR por Forçar OCR e cujo OCR falhou, mesmo com o nativo vazio.
type PaginasRelatorio struct {
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
	Duplicadas          int            `json:"duplicadas"`
	PorMotivo           map[string]int `json:"por_motivo,omitempty"`
}

// TempoRelatorio traz os tempos por fase, em ms. OCRTrabalhoMs soma os
// processos de OCR, que rodam em paralelo, e pode passar de TotalMs.
type TempoRelatorio struct {
	TotalMs         int64 `json:"total_ms"`
	AbrirMs         int64 `json:"abrir_ms"`
	PreContagemMs   int64 `json:"pre_contagem_ms"`
	LeituraParedeMs int64 `json:"leitura_parede_ms"`
	LeituraSomaMs   int64 `json:"leitura_soma_ms"`
	OCRTrabalhoMs   int64 `json:"ocr_trabalho_ms"`
	OCRParedeMs     int64 `json:"ocr_parede_ms"`
	FormatacaoMs    int64 `json:"formatacao_ms"`
	GravacaoMs      int64 `json:"gravacao_ms"`
}

// OCRRelatorio resume os processos de OCR. Um bloco é um processo com mais de
// uma página; PaginaEstimadaMs é o tempo de trabalho dividido pelas páginas
// que foram ao OCR (estimativa: o tempo real é medido só por processo). As
// páginas de um bloco que falhou e foi refeito página a página contam uma vez
// só (na refação), mas o tempo do bloco falho fica no trabalho.
type OCRRelatorio struct {
	Processos        int   `json:"processos"`
	Blocos           int   `json:"blocos"`
	PaginasEmBloco   int   `json:"paginas_em_bloco"`
	Refacoes         int   `json:"refacoes"`
	Prazos           int   `json:"prazos"`
	BlocoMedioMs     int64 `json:"bloco_medio_ms"`
	BlocoMedianaMs   int64 `json:"bloco_mediana_ms"`
	BlocoP95Ms       int64 `json:"bloco_p95_ms"`
	PaginaEstimadaMs int64 `json:"pagina_estimada_ms"`
}

// DocAtencao é um documento lido com páginas que pedem atenção, com os
// números de página de cada caso: cada lista traz no máximo os 50 primeiros, e
// o total do caso fica no campo Total correspondente.
type DocAtencao struct {
	Nome                string `json:"nome"`
	Evento              int    `json:"evento"`
	SemOCR              []int  `json:"sem_ocr,omitempty"`
	ComErro             []int  `json:"com_erro,omitempty"`
	OCRFalhouComReserva []int  `json:"ocr_falhou_com_reserva,omitempty"`
	// Totais de cada caso: as listas acima trazem só os primeiros números.
	TotalSemOCR              int `json:"total_sem_ocr,omitempty"`
	TotalComErro             int `json:"total_com_erro,omitempty"`
	TotalOCRFalhouComReserva int `json:"total_ocr_falhou_com_reserva,omitempty"`
}

// ExecucaoZip descreve uma execução de um item da fila que terminou (sucesso,
// erro ou cancelamento), para a telemetria. Config é a config efetiva do item
// (com o ajuste do Reprocessar e o Forçar OCR efetivo); Relatorio é nil fora
// do sucesso. Origem é "monitorada", "arrastada" ou "manual". EtapaErro é a
// etapa (etl.Etapa*) em que o ZIP inteiro falhou, só quando houve erro e ele
// trazia a etapa.
type ExecucaoZip struct {
	ItemID     string
	Reprocesso bool
	Origem     string
	Config     Config
	BytesZip   int64
	Status     string
	ClasseErro string
	EtapaErro  string
	Relatorio  *RelatorioZip
}
