export namespace main {
	
	export class ConfigGroupData {
	    name: string;
	    provider: string;
	    api_url: string;
	    model_id: string;
	    api_key: string;
	    middle_route: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigGroupData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.api_url = source["api_url"];
	        this.model_id = source["model_id"];
	        this.api_key = source["api_key"];
	        this.middle_route = source["middle_route"];
	    }
	}
	export class ConfigData {
	    path: string;
	    mapped_model_id: string;
	    auth_key: string;
	    config_groups: ConfigGroupData[];
	
	    static createFrom(source: any = {}) {
	        return new ConfigData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.mapped_model_id = source["mapped_model_id"];
	        this.auth_key = source["auth_key"];
	        this.config_groups = this.convertValues(source["config_groups"], ConfigGroupData);
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
	
	export class ConfigInfo {
	    path: string;
	    name: string;
	    provider: string;
	    model: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.active = source["active"];
	    }
	}
	export class PlatformInfo {
	    os: string;
	    arch: string;
	    privileged: boolean;
	    has_sudo: boolean;
	    cap_support: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlatformInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.privileged = source["privileged"];
	        this.has_sudo = source["has_sudo"];
	        this.cap_support = source["cap_support"];
	    }
	}
	export class ProviderInfo {
	    id: string;
	    name: string;
	    default_url: string;
	    default_route: string;
	    models: string[];
	    api_key_hint: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.default_url = source["default_url"];
	        this.default_route = source["default_route"];
	        this.models = source["models"];
	        this.api_key_hint = source["api_key_hint"];
	    }
	}
	export class StatusInfo {
	    running: boolean;
	    port: number;
	    host: string;
	    config: string;
	    uptime: string;
	    model: string;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.host = source["host"];
	        this.config = source["config"];
	        this.uptime = source["uptime"];
	        this.model = source["model"];
	        this.provider = source["provider"];
	    }
	}
	export class SystemInfo {
	    platform: PlatformInfo;
	    go_version: string;
	    app_version: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = this.convertValues(source["platform"], PlatformInfo);
	        this.go_version = source["go_version"];
	        this.app_version = source["app_version"];
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
	export class TestResult {
	    success: boolean;
	    latency: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.latency = source["latency"];
	        this.message = source["message"];
	    }
	}

}

