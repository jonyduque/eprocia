export namespace models {
	
	export class Config {
	    monitored_path: string;
	    ocr_enabled: boolean;
	    cleaning_level: string;
	    auto_start: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.monitored_path = source["monitored_path"];
	        this.ocr_enabled = source["ocr_enabled"];
	        this.cleaning_level = source["cleaning_level"];
	        this.auto_start = source["auto_start"];
	    }
	}

}

