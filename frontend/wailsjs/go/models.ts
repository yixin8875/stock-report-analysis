export namespace models {
	
	export class AIChannel {
	    id: number;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    model: string;
	    isDefault: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AIChannel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.isDefault = source["isDefault"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class FailureReasonMetric {
	    reason: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new FailureReasonMetric(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reason = source["reason"];
	        this.count = source["count"];
	    }
	}
	export class ChannelMetric {
	    channelId: number;
	    channelName: string;
	    totalRuns: number;
	    successRuns: number;
	    failedRuns: number;
	    successRate: string;
	    avgDuration: number;
	    totalTokens: number;
	    promptTokens: number;
	    outputTokens: number;
	
	    static createFrom(source: any = {}) {
	        return new ChannelMetric(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channelId = source["channelId"];
	        this.channelName = source["channelName"];
	        this.totalRuns = source["totalRuns"];
	        this.successRuns = source["successRuns"];
	        this.failedRuns = source["failedRuns"];
	        this.successRate = source["successRate"];
	        this.avgDuration = source["avgDuration"];
	        this.totalTokens = source["totalTokens"];
	        this.promptTokens = source["promptTokens"];
	        this.outputTokens = source["outputTokens"];
	    }
	}
	export class AnalysisDashboard {
	    totalRuns: number;
	    successRuns: number;
	    failedRuns: number;
	    successRate: string;
	    avgDurationMs: number;
	    totalTokens: number;
	    promptTokens: number;
	    outputTokens: number;
	    byChannel: ChannelMetric[];
	    failureTop: FailureReasonMetric[];
	
	    static createFrom(source: any = {}) {
	        return new AnalysisDashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRuns = source["totalRuns"];
	        this.successRuns = source["successRuns"];
	        this.failedRuns = source["failedRuns"];
	        this.successRate = source["successRate"];
	        this.avgDurationMs = source["avgDurationMs"];
	        this.totalTokens = source["totalTokens"];
	        this.promptTokens = source["promptTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.byChannel = this.convertValues(source["byChannel"], ChannelMetric);
	        this.failureTop = this.convertValues(source["failureTop"], FailureReasonMetric);
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
	export class AnalysisHistory {
	    id: number;
	    articleId: number;
	    analysis: string;
	    promptUsed: string;
	    channelUsed: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.articleId = source["articleId"];
	        this.analysis = source["analysis"];
	        this.promptUsed = source["promptUsed"];
	        this.channelUsed = source["channelUsed"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Tag {
	    id: number;
	    name: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	    }
	}
	export class Article {
	    id: number;
	    title: string;
	    content: string;
	    source: string;
	    analysis: string;
	    promptUsed: string;
	    channelUsed: string;
	    status: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    analyzedAt?: any;
	    tags: Tag[];
	
	    static createFrom(source: any = {}) {
	        return new Article(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.source = source["source"];
	        this.analysis = source["analysis"];
	        this.promptUsed = source["promptUsed"];
	        this.channelUsed = source["channelUsed"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.analyzedAt = this.convertValues(source["analyzedAt"], null);
	        this.tags = this.convertValues(source["tags"], Tag);
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
	export class BatchFailure {
	    articleId: number;
	    title: string;
	    reason: string;
	    // Go type: time
	    at: any;
	
	    static createFrom(source: any = {}) {
	        return new BatchFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.articleId = source["articleId"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	        this.at = this.convertValues(source["at"], null);
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
	export class BatchStatus {
	    running: boolean;
	    paused: boolean;
	    total: number;
	    completed: number;
	    success: number;
	    failed: number;
	    inProgress: number;
	    concurrency: number;
	    failures: BatchFailure[];
	
	    static createFrom(source: any = {}) {
	        return new BatchStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.paused = source["paused"];
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.success = source["success"];
	        this.failed = source["failed"];
	        this.inProgress = source["inProgress"];
	        this.concurrency = source["concurrency"];
	        this.failures = this.convertValues(source["failures"], BatchFailure);
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
	
	
	export class MinerUConfig {
	    enabled: number;
	    baseUrl: string;
	    apiToken: string;
	    modelVersion: string;
	    isOCR: number;
	    pollIntervalMs: number;
	    timeoutSec: number;
	
	    static createFrom(source: any = {}) {
	        return new MinerUConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.baseUrl = source["baseUrl"];
	        this.apiToken = source["apiToken"];
	        this.modelVersion = source["modelVersion"];
	        this.isOCR = source["isOCR"];
	        this.pollIntervalMs = source["pollIntervalMs"];
	        this.timeoutSec = source["timeoutSec"];
	    }
	}
	export class Prompt {
	    id: number;
	    name: string;
	    content: string;
	    isDefault: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Prompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.isDefault = source["isDefault"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class PromptVersion {
	    id: number;
	    promptId: number;
	    versionNo: number;
	    name: string;
	    content: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PromptVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.promptId = source["promptId"];
	        this.versionNo = source["versionNo"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class QARoleMetric {
	    roleId: number;
	    roleName: string;
	    totalRuns: number;
	    successRuns: number;
	    failedRuns: number;
	    successRate: string;
	    avgDuration: number;
	    totalTokens: number;
	    promptTokens: number;
	    outputTokens: number;
	
	    static createFrom(source: any = {}) {
	        return new QARoleMetric(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roleId = source["roleId"];
	        this.roleName = source["roleName"];
	        this.totalRuns = source["totalRuns"];
	        this.successRuns = source["successRuns"];
	        this.failedRuns = source["failedRuns"];
	        this.successRate = source["successRate"];
	        this.avgDuration = source["avgDuration"];
	        this.totalTokens = source["totalTokens"];
	        this.promptTokens = source["promptTokens"];
	        this.outputTokens = source["outputTokens"];
	    }
	}
	export class QADashboard {
	    totalRuns: number;
	    successRuns: number;
	    failedRuns: number;
	    successRate: string;
	    avgDurationMs: number;
	    totalTokens: number;
	    promptTokens: number;
	    outputTokens: number;
	    byRole: QARoleMetric[];
	    failureTop: FailureReasonMetric[];
	
	    static createFrom(source: any = {}) {
	        return new QADashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRuns = source["totalRuns"];
	        this.successRuns = source["successRuns"];
	        this.failedRuns = source["failedRuns"];
	        this.successRate = source["successRate"];
	        this.avgDurationMs = source["avgDurationMs"];
	        this.totalTokens = source["totalTokens"];
	        this.promptTokens = source["promptTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.byRole = this.convertValues(source["byRole"], QARoleMetric);
	        this.failureTop = this.convertValues(source["failureTop"], FailureReasonMetric);
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
	export class QAEvidence {
	    id: number;
	    messageId: number;
	    chunkIndex: number;
	    quote: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new QAEvidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.messageId = source["messageId"];
	        this.chunkIndex = source["chunkIndex"];
	        this.quote = source["quote"];
	        this.reason = source["reason"];
	    }
	}
	export class QAMessage {
	    id: number;
	    sessionId: number;
	    articleId: number;
	    parentId: number;
	    roleType: string;
	    roleId: number;
	    roleName: string;
	    content: string;
	    status: string;
	    errorReason: string;
	    durationMs: number;
	    promptTokens: number;
	    completionTokens: number;
	    totalTokens: number;
	    // Go type: time
	    createdAt: any;
	    evidences: QAEvidence[];
	
	    static createFrom(source: any = {}) {
	        return new QAMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.articleId = source["articleId"];
	        this.parentId = source["parentId"];
	        this.roleType = source["roleType"];
	        this.roleId = source["roleId"];
	        this.roleName = source["roleName"];
	        this.content = source["content"];
	        this.status = source["status"];
	        this.errorReason = source["errorReason"];
	        this.durationMs = source["durationMs"];
	        this.promptTokens = source["promptTokens"];
	        this.completionTokens = source["completionTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.evidences = this.convertValues(source["evidences"], QAEvidence);
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
	export class QAPin {
	    id: number;
	    sessionId: number;
	    articleId: number;
	    sourceMessageId: number;
	    content: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new QAPin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.articleId = source["articleId"];
	        this.sourceMessageId = source["sourceMessageId"];
	        this.content = source["content"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	
	export class QASession {
	    id: number;
	    articleId: number;
	    title: string;
	    summary: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new QASession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.articleId = source["articleId"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class Role {
	    id: number;
	    name: string;
	    alias: string;
	    domainTags: string;
	    systemPrompt: string;
	    modelOverride: string;
	    temperature: number;
	    maxTokens: number;
	    enabled: number;
	    isDefault: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Role(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.alias = source["alias"];
	        this.domainTags = source["domainTags"];
	        this.systemPrompt = source["systemPrompt"];
	        this.modelOverride = source["modelOverride"];
	        this.temperature = source["temperature"];
	        this.maxTokens = source["maxTokens"];
	        this.enabled = source["enabled"];
	        this.isDefault = source["isDefault"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class RoleTemplate {
	    id: string;
	    name: string;
	    alias: string;
	    domainTags: string;
	    systemPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new RoleTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.alias = source["alias"];
	        this.domainTags = source["domainTags"];
	        this.systemPrompt = source["systemPrompt"];
	    }
	}
	
	export class TelegraphWatchMatch {
	    code: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new TelegraphWatchMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	    }
	}
	export class TelegraphArticleItem {
	    id: number;
	    title: string;
	    source: string;
	    status: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    analyzedAt?: any;
	    importanceScore: number;
	    impactDirection: string;
	    impactLevel: string;
	    watchMatched: number;
	    watchMatches: TelegraphWatchMatch[];
	    tags: Tag[];
	
	    static createFrom(source: any = {}) {
	        return new TelegraphArticleItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.analyzedAt = this.convertValues(source["analyzedAt"], null);
	        this.importanceScore = source["importanceScore"];
	        this.impactDirection = source["impactDirection"];
	        this.impactLevel = source["impactLevel"];
	        this.watchMatched = source["watchMatched"];
	        this.watchMatches = this.convertValues(source["watchMatches"], TelegraphWatchMatch);
	        this.tags = this.convertValues(source["tags"], Tag);
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
	export class TelegraphDashboard {
	    totalRuns: number;
	    totalFetched: number;
	    totalImported: number;
	    totalAnalyzed: number;
	    successRate: string;
	    avgDurationMs: number;
	    failureTop: FailureReasonMetric[];
	
	    static createFrom(source: any = {}) {
	        return new TelegraphDashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRuns = source["totalRuns"];
	        this.totalFetched = source["totalFetched"];
	        this.totalImported = source["totalImported"];
	        this.totalAnalyzed = source["totalAnalyzed"];
	        this.successRate = source["successRate"];
	        this.avgDurationMs = source["avgDurationMs"];
	        this.failureTop = this.convertValues(source["failureTop"], FailureReasonMetric);
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
	export class TelegraphDigest {
	    id: number;
	    // Go type: time
	    slotStart: any;
	    // Go type: time
	    slotEnd: any;
	    summary: string;
	    topItems: number;
	    avgScore: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TelegraphDigest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slotStart = this.convertValues(source["slotStart"], null);
	        this.slotEnd = this.convertValues(source["slotEnd"], null);
	        this.summary = source["summary"];
	        this.topItems = source["topItems"];
	        this.avgScore = source["avgScore"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class TelegraphSchedulerConfig {
	    enabled: number;
	    sourceUrl: string;
	    intervalMinutes: number;
	    fetchLimit: number;
	    channelId: number;
	    analysisPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new TelegraphSchedulerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sourceUrl = source["sourceUrl"];
	        this.intervalMinutes = source["intervalMinutes"];
	        this.fetchLimit = source["fetchLimit"];
	        this.channelId = source["channelId"];
	        this.analysisPrompt = source["analysisPrompt"];
	    }
	}
	export class TelegraphSchedulerStatus {
	    running: boolean;
	    // Go type: time
	    lastRunAt: any;
	    lastError: string;
	    lastFetched: number;
	    lastImported: number;
	    lastAnalyzed: number;
	
	    static createFrom(source: any = {}) {
	        return new TelegraphSchedulerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.lastError = source["lastError"];
	        this.lastFetched = source["lastFetched"];
	        this.lastImported = source["lastImported"];
	        this.lastAnalyzed = source["lastAnalyzed"];
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
	
	export class WatchStock {
	    code: string;
	    name: string;
	    aliases: string[];
	
	    static createFrom(source: any = {}) {
	        return new WatchStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.aliases = source["aliases"];
	    }
	}

}

