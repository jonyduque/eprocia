export namespace etl {
	
	export class AtividadeDoc {
	    nome: string;
	    evento: number;
	    estado: string;
	    paginas: number;
	    feitas: number;
	    ocr: number;
	    erro?: string;
	
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
	    error?: string;
	    timestamp: number;
	    ocr_mode?: string;
	    cleaning_level?: string;
	
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
	        this.error = source["error"];
	        this.timestamp = source["timestamp"];
	        this.ocr_mode = source["ocr_mode"];
	        this.cleaning_level = source["cleaning_level"];
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
	    }
	}

}

