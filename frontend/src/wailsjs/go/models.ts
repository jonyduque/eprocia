export namespace atualizacao {
	
	export class Estado {
	    ativo: boolean;
	    fase: string;
	    versao: string;
	    progresso: number;
	    mensagem: string;
	    modo: string;
	    motivo: string;
	    atual: string;
	
	    static createFrom(source: any = {}) {
	        return new Estado(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ativo = source["ativo"];
	        this.fase = source["fase"];
	        this.versao = source["versao"];
	        this.progresso = source["progresso"];
	        this.mensagem = source["mensagem"];
	        this.modo = source["modo"];
	        this.motivo = source["motivo"];
	        this.atual = source["atual"];
	    }
	}
	export class Info {
	    versao: string;
	    notasURL: string;
	    modo: string;
	    motivo: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.versao = source["versao"];
	        this.notasURL = source["notasURL"];
	        this.modo = source["modo"];
	        this.motivo = source["motivo"];
	    }
	}

}

export namespace etl {
	
	export class AtividadeDoc {
	    nome: string;
	    evento: number;
	    estado: string;
	    paginas: number;
	    feitas: number;
	    ocr: number;
	    erro?: string;
	    leitura?: string;
	    detalhe?: string;
	
	    static createFrom(source: any = {}) {
	        return new AtividadeDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.evento = source["evento"];
	        this.estado = source["estado"];
	        this.paginas = source["paginas"];
	        this.feitas = source["feitas"];
	        this.ocr = source["ocr"];
	        this.erro = source["erro"];
	        this.leitura = source["leitura"];
	        this.detalhe = source["detalhe"];
	    }
	}
	export class InstantaneoAtividade {
	    item_id: string;
	    docs: AtividadeDoc[];
	    limite_docs: number;
	    limite_ocr: number;
	    ocr_ativos: number;
	
	    static createFrom(source: any = {}) {
	        return new InstantaneoAtividade(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item_id = source["item_id"];
	        this.docs = this.convertValues(source["docs"], AtividadeDoc);
	        this.limite_docs = source["limite_docs"];
	        this.limite_ocr = source["limite_ocr"];
	        this.ocr_ativos = source["ocr_ativos"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QueueItem {
	    id: string;
	    name: string;
	    path: string;
	    status: string;
	    progress: number;
	    message: string;
	    output_path: string;
	    docs_count: number;
	    pages_count: number;
	    estimated_tokens: number;
	    processed_docs: number;
	    total_docs: number;
	    processed_pages: number;
	    total_pages: number;
	    unread_docs: number;
	    relatorio?: models.RelatorioZip;
	    error?: string;
	    timestamp: number;
	    ocr_mode?: string;
	    cleaning_level?: string;
	    forcar_ocr?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.message = source["message"];
	        this.output_path = source["output_path"];
	        this.docs_count = source["docs_count"];
	        this.pages_count = source["pages_count"];
	        this.estimated_tokens = source["estimated_tokens"];
	        this.processed_docs = source["processed_docs"];
	        this.total_docs = source["total_docs"];
	        this.processed_pages = source["processed_pages"];
	        this.total_pages = source["total_pages"];
	        this.unread_docs = source["unread_docs"];
	        this.relatorio = this.convertValues(source["relatorio"], models.RelatorioZip);
	        this.error = source["error"];
	        this.timestamp = source["timestamp"];
	        this.ocr_mode = source["ocr_mode"];
	        this.cleaning_level = source["cleaning_level"];
	        this.forcar_ocr = source["forcar_ocr"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace models {
	
	export class Config {
	    monitored_path: string;
	    ocr_mode: string;
	    cleaning_level: string;
	    auto_start: boolean;
	    watchdog_enabled: boolean;
	    theme: string;
	    omitir_tipos: string[];
	    forcar_ocr: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.monitored_path = source["monitored_path"];
	        this.ocr_mode = source["ocr_mode"];
	        this.cleaning_level = source["cleaning_level"];
	        this.auto_start = source["auto_start"];
	        this.watchdog_enabled = source["watchdog_enabled"];
	        this.theme = source["theme"];
	        this.omitir_tipos = source["omitir_tipos"];
	        this.forcar_ocr = source["forcar_ocr"];
	    }
	}
	export class DocAtencao {
	    nome: string;
	    evento: number;
	    sem_ocr?: number[];
	    com_erro?: number[];
	    ocr_falhou_com_reserva?: number[];
	    total_sem_ocr?: number;
	    total_com_erro?: number;
	    total_ocr_falhou_com_reserva?: number;
	
	    static createFrom(source: any = {}) {
	        return new DocAtencao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.evento = source["evento"];
	        this.sem_ocr = source["sem_ocr"];
	        this.com_erro = source["com_erro"];
	        this.ocr_falhou_com_reserva = source["ocr_falhou_com_reserva"];
	        this.total_sem_ocr = source["total_sem_ocr"];
	        this.total_com_erro = source["total_com_erro"];
	        this.total_ocr_falhou_com_reserva = source["total_ocr_falhou_com_reserva"];
	    }
	}
	export class DocsRelatorio {
	    total: number;
	    lidos: number;
	    nao_lidos: number;
	    omitidos: number;
	    duplicados_h7: number;
	    pdf: number;
	    html: number;
	    tipos_omitidos?: string[];
	    por_leitura?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new DocsRelatorio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.lidos = source["lidos"];
	        this.nao_lidos = source["nao_lidos"];
	        this.omitidos = source["omitidos"];
	        this.duplicados_h7 = source["duplicados_h7"];
	        this.pdf = source["pdf"];
	        this.html = source["html"];
	        this.tipos_omitidos = source["tipos_omitidos"];
	        this.por_leitura = source["por_leitura"];
	    }
	}
	export class LimpezaRelatorio {
	    nivel: string;
	    bytes_antes: number;
	    bytes_depois: number;
	    bytes_duplicatas: number;
	    tokens_antes: number;
	    tokens_depois: number;
	
	    static createFrom(source: any = {}) {
	        return new LimpezaRelatorio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nivel = source["nivel"];
	        this.bytes_antes = source["bytes_antes"];
	        this.bytes_depois = source["bytes_depois"];
	        this.bytes_duplicatas = source["bytes_duplicatas"];
	        this.tokens_antes = source["tokens_antes"];
	        this.tokens_depois = source["tokens_depois"];
	    }
	}
	export class OCRRelatorio {
	    processos: number;
	    blocos: number;
	    paginas_em_bloco: number;
	    refacoes: number;
	    prazos: number;
	    bloco_medio_ms: number;
	    bloco_mediana_ms: number;
	    bloco_p95_ms: number;
	    pagina_estimada_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new OCRRelatorio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.processos = source["processos"];
	        this.blocos = source["blocos"];
	        this.paginas_em_bloco = source["paginas_em_bloco"];
	        this.refacoes = source["refacoes"];
	        this.prazos = source["prazos"];
	        this.bloco_medio_ms = source["bloco_medio_ms"];
	        this.bloco_mediana_ms = source["bloco_mediana_ms"];
	        this.bloco_p95_ms = source["bloco_p95_ms"];
	        this.pagina_estimada_ms = source["pagina_estimada_ms"];
	    }
	}
	export class PaginasRelatorio {
	    total: number;
	    lida_nativa: number;
	    lida_ocr: number;
	    vazia: number;
	    sem_ocr: number;
	    com_erro: number;
	    ocr_sem_ganho: number;
	    ocr_falhou_com_reserva: number;
	    de_nao_lidos: number;
	    de_omitidos: number;
	    duplicadas: number;
	    por_motivo?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new PaginasRelatorio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.lida_nativa = source["lida_nativa"];
	        this.lida_ocr = source["lida_ocr"];
	        this.vazia = source["vazia"];
	        this.sem_ocr = source["sem_ocr"];
	        this.com_erro = source["com_erro"];
	        this.ocr_sem_ganho = source["ocr_sem_ganho"];
	        this.ocr_falhou_com_reserva = source["ocr_falhou_com_reserva"];
	        this.de_nao_lidos = source["de_nao_lidos"];
	        this.de_omitidos = source["de_omitidos"];
	        this.duplicadas = source["duplicadas"];
	        this.por_motivo = source["por_motivo"];
	    }
	}
	export class TempoRelatorio {
	    total_ms: number;
	    abrir_ms: number;
	    pre_contagem_ms: number;
	    leitura_parede_ms: number;
	    leitura_soma_ms: number;
	    ocr_trabalho_ms: number;
	    ocr_parede_ms: number;
	    formatacao_ms: number;
	    gravacao_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new TempoRelatorio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_ms = source["total_ms"];
	        this.abrir_ms = source["abrir_ms"];
	        this.pre_contagem_ms = source["pre_contagem_ms"];
	        this.leitura_parede_ms = source["leitura_parede_ms"];
	        this.leitura_soma_ms = source["leitura_soma_ms"];
	        this.ocr_trabalho_ms = source["ocr_trabalho_ms"];
	        this.ocr_parede_ms = source["ocr_parede_ms"];
	        this.formatacao_ms = source["formatacao_ms"];
	        this.gravacao_ms = source["gravacao_ms"];
	    }
	}
	export class RelatorioZip {
	    documentos: DocsRelatorio;
	    paginas: PaginasRelatorio;
	    tempo: TempoRelatorio;
	    ocr: OCRRelatorio;
	    limpeza: LimpezaRelatorio;
	    bytes_txt: number;
	    tokens: number;
	    atencao?: DocAtencao[];
	
	    static createFrom(source: any = {}) {
	        return new RelatorioZip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.documentos = this.convertValues(source["documentos"], DocsRelatorio);
	        this.paginas = this.convertValues(source["paginas"], PaginasRelatorio);
	        this.tempo = this.convertValues(source["tempo"], TempoRelatorio);
	        this.ocr = this.convertValues(source["ocr"], OCRRelatorio);
	        this.limpeza = this.convertValues(source["limpeza"], LimpezaRelatorio);
	        this.bytes_txt = source["bytes_txt"];
	        this.tokens = source["tokens"];
	        this.atencao = this.convertValues(source["atencao"], DocAtencao);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

