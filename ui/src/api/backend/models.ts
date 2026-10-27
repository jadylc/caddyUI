export interface AppVersion {
	major: number;
	minor: number;
	revision: number;
}

export interface UserPermissions {
	id?: number;
	createdOn?: string;
	modifiedOn?: string;
	userId?: number;
	visibility: string;
	proxyHosts: string;
	staticHosts: string;
	redirectionHosts: string;
	deadHosts: string;
	streams: string;
	accessLists: string;
	certificates: string;
}

export interface User {
	id: number;
	createdOn: string;
	modifiedOn: string;
	isDisabled: boolean;
	email: string;
	name: string;
	nickname: string;
	avatar: string;
	roles: string[];
	permissions?: UserPermissions;
}

export interface AuditLog {
	id: number;
	createdOn: string;
	modifiedOn: string;
	userId: number;
	objectType: string;
	objectId: number;
	action: string;
	meta: Record<string, any>;
	// Expansions:
	user?: User;
}

export interface AccessList {
	id?: number;
	createdOn?: string;
	modifiedOn?: string;
	ownerUserId: number;
	name: string;
	meta: Record<string, any>;
	satisfyAny: boolean;
	passAuth: boolean;
	proxyHostCount?: number;
	// Expansions:
	owner?: User;
	items?: AccessListItem[];
	clients?: AccessListClient[];
}

export interface AccessListItem {
	id?: number;
	createdOn?: string;
	modifiedOn?: string;
	accessListId?: number;
	username: string;
	password: string;
	meta?: Record<string, any>;
	hint?: string;
}

export type AccessListClient = {
	id?: number;
	createdOn?: string;
	modifiedOn?: string;
	accessListId?: number;
	address: string;
	directive: "allow" | "deny";
	meta?: Record<string, any>;
};

export interface Certificate {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	provider: string;
	niceName: string;
	domainNames: string[];
	expiresOn: string;
	meta: Record<string, any>;
	owner?: User;
	proxyHosts?: ProxyHost[];
	staticHosts?: StaticHost[];
	deadHosts?: DeadHost[];
	redirectionHosts?: RedirectionHost[];
}

export interface ProxyLocation {
	path: string;
	forwardScheme: string;
	forwardHost: string;
	forwardPort: number;
	forwardPath?: string;
	upstreams?: ProxyUpstream[];
	loadBalancingPolicy?: string;
	reverseProxyMode?: "inherit" | "custom";
	reverseProxy?: ReverseProxyAdvancedConfig;
}

export interface ProxyUpstream {
	forwardScheme: string;
	forwardHost: string;
	forwardPort: number;
	weight?: number;
}

export interface ForwardAuthConfig {
	enabled: boolean;
	provider: string;
	upstream: string;
	uri: string;
	copyHeaders: string[];
	useGlobal?: boolean;
}

export interface WAFConfig {
	enabled: boolean;
	mode: "on" | "detection_only";
}

export interface ReverseProxyAdvancedConfig {
	healthCheck?: ReverseProxyHealthCheckConfig;
	retry?: ReverseProxyRetryConfig;
	headers?: ReverseProxyHeadersConfig;
	transport?: ReverseProxyTransportConfig;
	interception?: ReverseProxyInterceptionConfig;
}

export interface ReverseProxyHealthCheckConfig {
	enabled?: boolean;
	uri?: string;
	interval?: string;
	timeout?: string;
	status?: string;
	passiveEnabled?: boolean;
	failDuration?: string;
	maxFails?: number;
	unhealthyStatus?: string;
}

export interface ReverseProxyRetryConfig {
	enabled?: boolean;
	tryDuration?: string;
	tryInterval?: string;
	retries?: number;
	methods?: "" | "get_head";
}

export interface ProxyHeaderRule {
	action?: "set" | "delete";
	name?: string;
	value?: string;
}

export interface ReverseProxyHeadersConfig {
	hostMode?: "" | "host" | "upstream" | "custom";
	hostValue?: string;
	request?: ProxyHeaderRule[];
	response?: ProxyHeaderRule[];
}

export interface ReverseProxyTransportConfig {
	protocol?: "" | "h2c" | "http1";
	tlsServerName?: string;
	tlsInsecureSkipVerify?: boolean;
	dialTimeout?: string;
	responseHeaderTimeout?: string;
	readTimeout?: string;
	writeTimeout?: string;
}

export type ReverseProxyInterceptionAction = "error_page_template" | "redirect" | "respond" | "headers";

export interface ReverseProxyInterceptionConfig {
	enabled?: boolean;
	rules?: ReverseProxyInterceptionRule[];
}

export interface ReverseProxyInterceptionRule {
	id?: number;
	statuses?: string[];
	headerName?: string;
	headerValue?: string;
	action?: ReverseProxyInterceptionAction;
	templateId?: number;
	redirectUrl?: string;
	redirectCode?: number;
	statusCode?: number;
	body?: string;
	headers?: ProxyHeaderRule[];
}

export interface ResponseOptimizationConfig {
	compressionEnabled?: boolean;
	cacheControlEnabled?: boolean;
	cacheControlValue?: string;
	securityHeadersEnabled?: boolean;
	policyTemplateId?: number;
}

export interface HeaderPolicyTemplate {
	id: number;
	createdOn?: string;
	modifiedOn?: string;
	ownerUserId?: number;
	name: string;
	description?: string;
	category?: string;
	enabled: boolean;
	builtin?: boolean;
	headers?: ProxyHeaderRule[];
	meta?: Record<string, any>;
	hostCount?: number;
	owner?: User;
}

export interface AccessLogConfig {
	enabled?: boolean;
	format?: "json" | "console";
	outputFile?: string;
	rollSize?: string;
	rollKeep?: number;
	rollKeepDays?: number;
}

export interface ErrorHandlingConfig {
	enabled?: boolean;
	templateId?: number;
}

export type ErrorResponseAction = "static_file" | "redirect" | "reverse_proxy" | "respond";

export interface ErrorResponseRule {
	id?: number;
	statuses: string[];
	action: ErrorResponseAction;
	staticFile?: string;
	redirectUrl?: string;
	redirectCode?: number;
	upstream?: string;
	body?: string;
}

export interface ErrorResponseTemplate {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	name: string;
	description?: string;
	enabled: boolean;
	staticRoot?: string;
	rules: ErrorResponseRule[];
	meta: Record<string, any>;
	hostCount?: number;
	owner?: User;
}

export interface SiteAccessLog {
	enabled: boolean;
	path: string;
	lines: string[];
}

export interface ObservabilityReload {
	successful: boolean;
	lastSuccessAt?: string;
}

export interface ObservabilityTraffic {
	totalRequests: number;
	qps: number;
	inFlight: number;
	p95Ms: number;
	p95Capped: boolean;
	statusClasses: Record<string, number>;
	methods: Record<string, number>;
}

export interface ObservabilityUpstream {
	address: string;
	healthy: boolean;
	status: "healthy" | "unhealthy" | "unknown";
}

export interface ObservabilitySiteUpstream {
	address: string;
	status: "healthy" | "unhealthy" | "unknown";
}

export interface ObservabilityLogEvent {
	time?: string;
	level: "error" | "warn" | string;
	message: string;
	source: string;
	sites?: string[];
}

export interface ObservabilityErrorTrend {
	last15m: number;
	last1h: number;
	last24h: number;
}

export interface ObservabilitySite {
	id: number;
	name: string;
	kind: string;
	domains: string[];
	accessLogEnabled: boolean;
	requests: number;
	p95Ms: number;
	statusClasses: Record<string, number>;
	upstreams?: ObservabilitySiteUpstream[];
	recentErrors?: ObservabilityLogEvent[];
}

export interface ObservabilityOverview {
	collectedAt: string;
	available: boolean;
	error?: string;
	reload: ObservabilityReload;
	traffic: ObservabilityTraffic;
	upstreams: ObservabilityUpstream[];
	sites: ObservabilitySite[];
	recentErrors: ObservabilityLogEvent[];
	errorTrend: ObservabilityErrorTrend;
}

export interface AutheliaSettings {
	enabled: boolean;
	upstream: string;
	uri: string;
	copyHeaders: string[];
	failOpen: boolean;
}

export interface AuthentikSettings {
	enabled: boolean;
	upstream: string;
	uri: string;
	copyHeaders: string[];
}

export interface SystemSettings {
	acmeContactEmail: string;
}

export interface ProxyHost {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	serviceName?: string;
	domainNames: string[];
	listenPort: number;
	listenPorts?: number[];
	forwardScheme: string;
	forwardHost: string;
	forwardPort: number;
	upstreams?: ProxyUpstream[];
	loadBalancingPolicy?: string;
	accessListId: number;
	certificateId: number;
	sslForced: boolean;
	cachingEnabled: boolean;
	blockExploits: boolean;
	meta: Record<string, any>;
	allowWebsocketUpgrade: boolean;
	http2Support: boolean;
	enabled: boolean;
	locations?: ProxyLocation[];
	rootRedirectPath?: string;
	rootRedirectCode?: number;
	hstsEnabled: boolean;
	hstsSubdomains: boolean;
	trustForwardedProto: boolean;
	forwardAuth?: ForwardAuthConfig;
	waf?: WAFConfig;
	upstreamInsecureSkipVerify?: boolean;
	reverseProxy?: ReverseProxyAdvancedConfig;
	response?: ResponseOptimizationConfig;
	accessLog?: AccessLogConfig;
	errorHandling?: ErrorHandlingConfig;
	// Expansions:
	owner?: User;
	accessList?: AccessList;
	certificate?: Certificate;
}

export interface StaticHost {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	serviceName?: string;
	domainNames: string[];
	listenPort: number;
	listenPorts?: number[];
	rootPath: string;
	mode: "static" | "spa";
	indexFile?: string;
	directoryBrowsing: boolean;
	rootRedirectPath?: string;
	rootRedirectCode?: number;
	accessListId: number;
	certificateId: number;
	sslForced: boolean;
	meta: Record<string, any>;
	http2Support: boolean;
	enabled: boolean;
	hstsEnabled: boolean;
	hstsSubdomains: boolean;
	waf?: WAFConfig;
	response?: ResponseOptimizationConfig;
	accessLog?: AccessLogConfig;
	errorHandling?: ErrorHandlingConfig;
	// Expansions:
	owner?: User;
	accessList?: AccessList;
	certificate?: Certificate;
}

export interface WafAuditEntry {
	transactionId?: string;
	timestamp?: string;
	host?: string;
	uri?: string;
	method?: string;
	ruleId?: string;
	message?: string;
	severity?: string;
	action?: string;
	remoteAddr?: string;
}

export interface WafAuditResponse {
	total: number;
	items: WafAuditEntry[];
	limit: number;
	offset: number;
}

export interface DeadHost {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	domainNames: string[];
	certificateId: number;
	sslForced: boolean;
	meta: Record<string, any>;
	http2Support: boolean;
	enabled: boolean;
	hstsEnabled: boolean;
	hstsSubdomains: boolean;
	// Expansions:
	owner?: User;
	certificate?: Certificate;
}

export interface RedirectionHost {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	domainNames: string[];
	forwardDomainName: string;
	preservePath: boolean;
	certificateId: number;
	sslForced: boolean;
	blockExploits: boolean;
	meta: Record<string, any>;
	http2Support: boolean;
	forwardScheme: string;
	forwardHttpCode: number;
	enabled: boolean;
	hstsEnabled: boolean;
	hstsSubdomains: boolean;
	// Expansions:
	owner?: User;
	certificate?: Certificate;
}

export interface Stream {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	incomingPort: number;
	forwardingHost: string;
	forwardingPort: number;
	tcpForwarding: boolean;
	udpForwarding: boolean;
	meta: Record<string, any>;
	enabled: boolean;
	certificateId: number;
	// Expansions:
	owner?: User;
	certificate?: Certificate;
}

export interface DynamicDNS {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	name: string;
	domainNames: string[];
	credentialId: string;
	ipv4: boolean;
	ipv6: boolean;
	checkInterval: string;
	ttl: string;
	resolvers: string[];
	ipServiceUrl: string;
	dnsProvider?: string;
	meta: Record<string, any>;
	enabled: boolean;
	owner?: User;
}

export interface WakeDevice {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	name: string;
	macAddress: string;
	broadcastAddress: string;
	port: number;
	secureOn?: string;
	host?: string;
	description?: string;
	meta: Record<string, any> & {
		lastWokenAt?: string;
		lastError?: string;
		lastErrorAt?: string;
	};
	enabled: boolean;
	owner?: User;
}

export interface DomainMonitorResult {
	domain: string;
	status: string;
	resolvedIps?: string[];
	domainName?: string;
	domainExpiresOn?: string;
	domainDaysLeft?: number;
	domainExpiryUnavailable?: boolean;
	domainExpiryUnavailableReason?: string;
	sslExpiresOn?: string;
	sslDaysLeft?: number;
	sslIssuer?: string;
	sslSource?: string;
	error?: string;
}

export interface DomainMonitor {
	id: number;
	createdOn: string;
	modifiedOn: string;
	ownerUserId: number;
	name: string;
	domainNames: string[];
	checkSsl: boolean;
	checkDns: boolean;
	checkDomain: boolean;
	credentialId?: string;
	registrarProvider?: string;
	reminderDays?: number[];
	autoRenew?: boolean;
	renewBeforeDays?: number;
	checkInterval: string;
	thresholdDays: number;
	resolvers: string[];
	meta: Record<string, any> & {
		status?: string;
		lastChecked?: string;
		nextCheck?: string;
		lastError?: string;
		domainName?: string;
		domainExpiresOn?: string;
		domainDaysLeft?: number;
		domainExpiryUnavailable?: boolean;
		domainExpiryUnavailableReason?: string;
		sslExpiresOn?: string;
		sslDaysLeft?: number;
		sslIssuer?: string;
		sslSource?: string;
		resolvedIps?: string[];
		results?: DomainMonitorResult[];
	};
	enabled: boolean;
	owner?: User;
}

export interface Setting {
	id: string;
	name?: string;
	description?: string;
	value: string;
	meta?: Record<string, any>;
}

export interface DNSProvider {
	id: string;
	name: string;
	credentials: string;
	fields?: ProviderField[];
	provider?: string;
	credentialId?: string;
	saved?: boolean;
}

// ProviderField 与后端 providerFieldView 对应，前端据此按 schema 渲染凭据表单。
export interface ProviderField {
	key: string;
	label: string;
	placeholder?: string;
	help?: string;
	secret: boolean;
	required: boolean;
	env_name?: string;
}

// ProviderSpec 与后端 providerSpecView 对应。
export interface ProviderSpec {
	id: string;
	name: string;
	dns: boolean;
	ddns: boolean;
	registrar: boolean;
	renewable: boolean;
	fields?: ProviderField[];
	credentials: string;
}

export interface CredentialSummary {
	id: string;
	createdOn?: string;
	modifiedOn?: string;
	name: string;
	provider: string;
	hasSecret: boolean;
	usageCount: number;
}

export interface CredentialDetail extends CredentialSummary {
	aliyunKey?: string;
	aliyunSecret?: string;
	cfToken?: string;
	dnspodToken?: string;
	heApiKey?: string;
	digitalPlatApiKey?: string;
	dnsheApiKey?: string;
	dnsheApiSecret?: string;
	// 新增 provider 的密钥统一放这里；历史 provider 的密钥也同步回填到这里。
	secrets?: Record<string, string>;
	fields?: ProviderField[];
}

export interface NotificationEventSpec {
	id: string;
	name: string;
	description: string;
}

export interface NotificationChannel {
	id?: number;
	createdOn?: string;
	modifiedOn?: string;
	name: string;
	type: string;
	url?: string;
	method?: string;
	headers?: string;
	bodyTemplate?: string;
	proxyUrl?: string;
	token?: string;
	secret?: string;
	chatId?: string;
	events?: string[];
	enabled: boolean;
	lastError?: string;
	lastSentAt?: string;
	meta?: Record<string, any>;
}

export interface NotificationChannelsResponse {
	channels: NotificationChannel[];
	events: NotificationEventSpec[];
}

export interface CertificateAuthority {
	id: string;
	name: string;
	issuerProvider: string;
	caDirectory?: string;
	eabKeyId?: string;
	eabMacKey?: string;
	zerosslApiKey?: string;
	needsUrl?: boolean;
	supportsEab?: boolean;
	internal?: boolean;
	saved?: boolean;
}

export interface CertificateBinding {
	domain: string;
	certificateId?: number;
	certificateDomain?: string;
	mode?: "auto" | "selected";
}
