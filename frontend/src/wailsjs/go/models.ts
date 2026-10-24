export namespace etl {
	
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

