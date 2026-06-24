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
	deadHosts?: DeadHost[];
	redirectionHosts?: RedirectionHost[];
}

export interface ProxyLocation {
	path: string;
	forwardScheme: string;
	forwardHost: string;
	forwardPort: number;
}

export interface ForwardAuthConfig {
	enabled: boolean;
	provider: string;
	upstream: string;
	uri: string;
	copyHeaders: string[];
	useGlobal?: boolean;
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
	hstsEnabled: boolean;
	hstsSubdomains: boolean;
	trustForwardedProto: boolean;
	forwardAuth?: ForwardAuthConfig;
	upstreamInsecureSkipVerify?: boolean;
	// Expansions:
	owner?: User;
	accessList?: AccessList;
	certificate?: Certificate;
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
	provider?: string;
	credentialId?: string;
	saved?: boolean;
}

export interface CredentialSummary {
	id: string;
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
