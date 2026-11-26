export namespace main {

	export class ChildrenRequest {
	    parentId: number;
	    text: string;
	    extensions: string[];
	    offset: number;
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new ChildrenRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parentId = source["parentId"];
	        this.text = source["text"];
	        this.extensions = source["extensions"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}
	export class RenderNode {
	    id: number;
	    parentId: number;
	    name: string;
	    path: string;
	    extension: string;
	    kind: string;
	    depth: number;
	    matched: boolean;
	    hiddenMatches: number;
	    leafCount: number;

	    static createFrom(source: any = {}) {
	        return new RenderNode(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parentId = source["parentId"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.extension = source["extension"];
	        this.kind = source["kind"];
	        this.depth = source["depth"];
	        this.matched = source["matched"];
	        this.hiddenMatches = source["hiddenMatches"];
	        this.leafCount = source["leafCount"];
	    }
	}
	export class ChildrenResult {
	    nodes: RenderNode[];
	    total: number;
	    hasMore: boolean;

	    static createFrom(source: any = {}) {
	        return new ChildrenResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], RenderNode);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
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
	export class ExtensionCount {
	    extension: string;
	    count: number;

	    static createFrom(source: any = {}) {
	        return new ExtensionCount(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extension = source["extension"];
	        this.count = source["count"];
	    }
	}
	export class FolderEntry {
	    name: string;
	    path: string;

	    static createFrom(source: any = {}) {
	        return new FolderEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class FolderListing {
	    path: string;
	    parent: string;
	    folders: FolderEntry[];
	    truncated: boolean;

	    static createFrom(source: any = {}) {
	        return new FolderListing(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.parent = source["parent"];
	        this.folders = this.convertValues(source["folders"], FolderEntry);
	        this.truncated = source["truncated"];
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
	export class QueryRequest {
	    text: string;
	    extensions: string[];
	    limit: number;
	    rootId: number;
	    focusId: number;

	    static createFrom(source: any = {}) {
	        return new QueryRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.extensions = source["extensions"];
	        this.limit = source["limit"];
	        this.rootId = source["rootId"];
	        this.focusId = source["focusId"];
	    }
	}
	export class QueryResult {
	    nodes: RenderNode[];
	    matches: number;
	    truncated: boolean;
	    queryMs: number;

	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], RenderNode);
	        this.matches = source["matches"];
	        this.truncated = source["truncated"];
	        this.queryMs = source["queryMs"];
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

	export class ScanSummary {
	    cancelled: boolean;
	    root: string;
	    rootName: string;
	    files: number;
	    directories: number;
	    skipped: number;
	    durationMs: number;
	    extensions: ExtensionCount[];

	    static createFrom(source: any = {}) {
	        return new ScanSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.root = source["root"];
	        this.rootName = source["rootName"];
	        this.files = source["files"];
	        this.directories = source["directories"];
	        this.skipped = source["skipped"];
	        this.durationMs = source["durationMs"];
	        this.extensions = this.convertValues(source["extensions"], ExtensionCount);
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
