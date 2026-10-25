export namespace main {

	export class DemoSession {
	    id: string;
	    state: string;
	    captured: number;
	    total: number;

	    static createFrom(source: any = {}) {
	        return new DemoSession(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.captured = source["captured"];
	        this.total = source["total"];
	    }
	}

}
