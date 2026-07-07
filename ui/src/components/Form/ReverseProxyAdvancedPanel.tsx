import { IconPlus, IconTrash } from "@tabler/icons-react";
import cn from "classnames";
import { Field, getIn, useFormikContext } from "formik";
import type { ReactNode } from "react";
import type { ProxyHeaderRule, ReverseProxyAdvancedConfig } from "src/api/backend";

interface Props {
	name?: string;
	compact?: boolean;
	inheritNote?: string;
}

const blankHeaderRule = (): ProxyHeaderRule => ({ action: "set", name: "", value: "" });

export const blankReverseProxyAdvanced = (): ReverseProxyAdvancedConfig => ({
	healthCheck: {
		enabled: false,
		uri: "/health",
		interval: "30s",
		timeout: "5s",
		status: "",
		passiveEnabled: false,
		failDuration: "30s",
		maxFails: 1,
		unhealthyStatus: "",
	},
	retry: {
		enabled: false,
		tryDuration: "5s",
		tryInterval: "250ms",
		retries: 2,
		methods: "",
	},
	headers: {
		hostMode: "",
		hostValue: "",
		request: [],
		response: [],
	},
	transport: {
		protocol: "",
		tlsServerName: "",
		tlsInsecureSkipVerify: false,
		dialTimeout: "",
		responseHeaderTimeout: "",
		readTimeout: "",
		writeTimeout: "",
	},
});

export const withReverseProxyDefaults = (value?: ReverseProxyAdvancedConfig, legacyInsecureSkipVerify = false) => {
	const blank = blankReverseProxyAdvanced();
	const healthCheck = { ...blank.healthCheck, ...(value?.healthCheck ?? {}) };
	if (!healthCheck.uri) healthCheck.uri = blank.healthCheck?.uri;
	if (!healthCheck.interval) healthCheck.interval = blank.healthCheck?.interval;
	if (!healthCheck.timeout) healthCheck.timeout = blank.healthCheck?.timeout;
	if (!healthCheck.failDuration) healthCheck.failDuration = blank.healthCheck?.failDuration;
	if (!healthCheck.maxFails) healthCheck.maxFails = blank.healthCheck?.maxFails;
	const retry = { ...blank.retry, ...(value?.retry ?? {}) };
	if (!retry.tryDuration) retry.tryDuration = blank.retry?.tryDuration;
	if (!retry.tryInterval) retry.tryInterval = blank.retry?.tryInterval;
	if (!retry.retries) retry.retries = blank.retry?.retries;
	return {
		healthCheck,
		retry,
		headers: {
			...blank.headers,
			...(value?.headers ?? {}),
			request: Array.isArray(value?.headers?.request) ? value.headers.request : [],
			response: Array.isArray(value?.headers?.response) ? value.headers.response : [],
		},
		transport: {
			...blank.transport,
			...(value?.transport ?? {}),
			tlsInsecureSkipVerify: legacyInsecureSkipVerify || !!value?.transport?.tlsInsecureSkipVerify,
		},
	} as ReverseProxyAdvancedConfig;
};

export const normalizeReverseProxyAdvanced = (value?: ReverseProxyAdvancedConfig) => {
	const cfg = withReverseProxyDefaults(value);
	const request = normalizeHeaderRules(cfg.headers?.request);
	const response = normalizeHeaderRules(cfg.headers?.response);
	const activeHealthEnabled = !!cfg.healthCheck?.enabled;
	const passiveHealthEnabled = !!cfg.healthCheck?.passiveEnabled;
	const retryEnabled = !!cfg.retry?.enabled;
	return {
		healthCheck: {
			enabled: activeHealthEnabled,
			uri: activeHealthEnabled ? cleanString(cfg.healthCheck?.uri) : "",
			interval: activeHealthEnabled ? cleanString(cfg.healthCheck?.interval) : "",
			timeout: activeHealthEnabled ? cleanString(cfg.healthCheck?.timeout) : "",
			status: activeHealthEnabled ? cleanString(cfg.healthCheck?.status) : "",
			passiveEnabled: passiveHealthEnabled,
			failDuration: passiveHealthEnabled ? cleanString(cfg.healthCheck?.failDuration) : "",
			maxFails: passiveHealthEnabled ? cleanNumber(cfg.healthCheck?.maxFails) : 0,
			unhealthyStatus: passiveHealthEnabled ? cleanString(cfg.healthCheck?.unhealthyStatus) : "",
		},
		retry: {
			enabled: retryEnabled,
			tryDuration: retryEnabled ? cleanString(cfg.retry?.tryDuration) : "",
			tryInterval: retryEnabled ? cleanString(cfg.retry?.tryInterval) : "",
			retries: retryEnabled ? cleanNumber(cfg.retry?.retries) : 0,
			methods: retryEnabled && cfg.retry?.methods === "get_head" ? "get_head" : "",
		},
		headers: {
			hostMode: ["host", "upstream", "custom"].includes(String(cfg.headers?.hostMode || ""))
				? cfg.headers?.hostMode
				: "",
			hostValue: cleanString(cfg.headers?.hostValue),
			request,
			response,
		},
		transport: {
			protocol:
				cfg.transport?.protocol === "h2c" || cfg.transport?.protocol === "http1" ? cfg.transport.protocol : "",
			tlsServerName: cleanString(cfg.transport?.tlsServerName),
			tlsInsecureSkipVerify: !!cfg.transport?.tlsInsecureSkipVerify,
			dialTimeout: cleanString(cfg.transport?.dialTimeout),
			responseHeaderTimeout: cleanString(cfg.transport?.responseHeaderTimeout),
			readTimeout: cleanString(cfg.transport?.readTimeout),
			writeTimeout: cleanString(cfg.transport?.writeTimeout),
		},
	} as ReverseProxyAdvancedConfig;
};

const cleanString = (value: any) => String(value || "").trim();

const cleanNumber = (value: any) => {
	const n = Number(value || 0);
	return Number.isInteger(n) && n > 0 ? n : 0;
};

const normalizeHeaderRules = (rules?: ProxyHeaderRule[]) =>
	(Array.isArray(rules) ? rules : [])
		.map((rule) => ({
			action: rule.action === "delete" ? "delete" : "set",
			name: cleanString(rule.name),
			value: cleanString(rule.value),
		}))
		.filter((rule) => rule.name);

const durationOptions = [
	{ value: "", label: "默认" },
	{ value: "250ms", label: "250ms" },
	{ value: "500ms", label: "500ms" },
	{ value: "1s", label: "1s" },
	{ value: "2s", label: "2s" },
	{ value: "5s", label: "5s" },
	{ value: "10s", label: "10s" },
	{ value: "30s", label: "30s" },
	{ value: "1m", label: "1m" },
];

const statusOptions = [
	{ value: "", label: "默认" },
	{ value: "200", label: "200" },
	{ value: "204", label: "204" },
	{ value: "301", label: "301" },
	{ value: "302", label: "302" },
	{ value: "401", label: "401" },
	{ value: "403", label: "403" },
	{ value: "404", label: "404" },
	{ value: "500", label: "500" },
	{ value: "502", label: "502" },
	{ value: "503", label: "503" },
	{ value: "504", label: "504" },
];

function SwitchField({ name, id, label, help }: { name: string; id: string; label: string; help?: string }) {
	return (
		<Field name={name} type="checkbox">
			{({ field }: any) => (
				<label className="row align-items-center" htmlFor={id}>
					<span className="col">
						{label}
						{help ? <small className="d-block text-secondary">{help}</small> : null}
					</span>
					<span className="col-auto">
						<span className="form-check form-check-single form-switch">
							<input
								{...field}
								id={id}
								className={cn("form-check-input", { "bg-lime": field.checked })}
								type="checkbox"
								checked={!!field.checked}
							/>
						</span>
					</span>
				</label>
			)}
		</Field>
	);
}

function TextField({
	name,
	id,
	label,
	placeholder,
	type = "text",
	min,
}: {
	name: string;
	id: string;
	label: string;
	placeholder?: string;
	type?: string;
	min?: number;
}) {
	return (
		<div className="mb-3">
			<label className="form-label" htmlFor={id}>
				{label}
			</label>
			<Field id={id} name={name} type={type} min={min} className="form-control" placeholder={placeholder} />
		</div>
	);
}

function SelectField({
	name,
	id,
	label,
	options,
}: {
	name: string;
	id: string;
	label: string;
	options: { value: string; label: string }[];
}) {
	return (
		<div className="mb-3">
			<label className="form-label" htmlFor={id}>
				{label}
			</label>
			<Field as="select" id={id} name={name} className="form-control">
				{options.map((item) => (
					<option key={item.value || "default"} value={item.value}>
						{item.label}
					</option>
				))}
			</Field>
		</div>
	);
}

function Section({
	title,
	summary,
	description,
	children,
	open = false,
}: {
	title: string;
	summary: string;
	description?: string;
	children: ReactNode;
	open?: boolean;
}) {
	return (
		<details className="border rounded-2 mb-2" open={open}>
			<summary className="d-flex align-items-start justify-content-between gap-3 p-3 cursor-pointer">
				<span>
					<span className="fw-medium d-block">{title}</span>
					{description ? <span className="text-secondary small">{description}</span> : null}
				</span>
				<span className="badge bg-secondary-lt text-secondary text-nowrap">{summary}</span>
			</summary>
			<div className="border-top p-3">{children}</div>
		</details>
	);
}

function HeaderRules({ name, label }: { name: string; label: string }) {
	const { values, setFieldValue } = useFormikContext<any>();
	const rules = (getIn(values, name) as ProxyHeaderRule[] | undefined) ?? [];
	const setRules = (next: ProxyHeaderRule[]) => setFieldValue(name, next);
	const handleChange = (idx: number, field: keyof ProxyHeaderRule, value: string) => {
		setRules(rules.map((rule, ruleIdx) => (ruleIdx === idx ? { ...rule, [field]: value } : rule)));
	};
	return (
		<div className="mb-3">
			<div className="d-flex align-items-center justify-content-between mb-2">
				<label className="form-label m-0">{label}</label>
				<button
					type="button"
					className="btn btn-sm"
					onClick={() => setRules([...rules, blankHeaderRule()])}
					title="添加头规则"
				>
					<IconPlus size={16} />
					添加
				</button>
			</div>
			{rules.length === 0 ? <div className="text-secondary small mb-2">保持默认</div> : null}
			<div className="d-flex flex-column gap-2">
				{rules.map((rule, idx) => (
					<div className="row g-2 align-items-end" key={idx}>
						<div className="col-md-2">
							<select
								className="form-control"
								value={rule.action === "delete" ? "delete" : "set"}
								onChange={(event) => handleChange(idx, "action", event.target.value)}
							>
								<option value="set">设置</option>
								<option value="delete">删除</option>
							</select>
						</div>
						<div className="col-md-4">
							<input
								type="text"
								className="form-control"
								placeholder="Header-Name"
								value={rule.name || ""}
								onChange={(event) => handleChange(idx, "name", event.target.value)}
							/>
						</div>
						<div className="col-md-5">
							<input
								type="text"
								className="form-control"
								placeholder={rule.action === "delete" ? "" : "value"}
								value={rule.value || ""}
								disabled={rule.action === "delete"}
								onChange={(event) => handleChange(idx, "value", event.target.value)}
							/>
						</div>
						<div className="col-md-1">
							<button
								type="button"
								className="btn btn-icon"
								onClick={() => setRules(rules.filter((_, ruleIdx) => ruleIdx !== idx))}
								title="删除头规则"
							>
								<IconTrash size={16} />
							</button>
						</div>
					</div>
				))}
			</div>
		</div>
	);
}

export function ReverseProxyAdvancedPanel({ name = "reverseProxy", compact = false, inheritNote }: Props) {
	const { values } = useFormikContext<any>();
	const cfg = withReverseProxyDefaults(getIn(values, name));
	const activeHealthEnabled = !!cfg.healthCheck?.enabled;
	const passiveHealthEnabled = !!cfg.healthCheck?.passiveEnabled;
	const retryEnabled = !!cfg.retry?.enabled;
	const customHostEnabled = cfg.headers?.hostMode === "custom";
	const reverseProxySuffix = ".reverseProxy";
	const ownerPath = name.endsWith(reverseProxySuffix) ? name.slice(0, -reverseProxySuffix.length) : "";
	const upstreamsPath = ownerPath ? `${ownerPath}.upstreams` : "upstreams";
	const forwardSchemePath = ownerPath ? `${ownerPath}.forwardScheme` : "forwardScheme";
	const upstreams = Array.isArray(getIn(values, upstreamsPath)) ? getIn(values, upstreamsPath) : [];
	const hasHTTPSUpstream =
		upstreams.some((upstream: any) => upstream?.forwardScheme === "https") ||
		(!upstreams.length && getIn(values, forwardSchemePath) === "https");
	const hasConnectionSettings =
		cfg.transport?.protocol ||
		cfg.transport?.tlsServerName ||
		cfg.transport?.tlsInsecureSkipVerify ||
		cfg.transport?.dialTimeout ||
		cfg.transport?.responseHeaderTimeout ||
		cfg.transport?.readTimeout ||
		cfg.transport?.writeTimeout;
	const healthSummary = activeHealthEnabled
		? `主动 ${cfg.healthCheck?.uri || "未填路径"}`
		: passiveHealthEnabled
			? "被动"
			: "默认";
	const retrySummary = cfg.retry?.enabled
		? `${cfg.retry.retries || 0} 次 / ${cfg.retry.tryDuration || "默认窗口"}`
		: "默认";
	const headerRuleCount = (cfg.headers?.request?.length || 0) + (cfg.headers?.response?.length || 0);
	const headerSummary = cfg.headers?.hostMode || headerRuleCount > 0 ? "已配置" : "默认";
	const transportSummary = hasConnectionSettings ? "已配置" : "默认";

	return (
		<div className={cn("reverse-proxy-advanced", compact && "small")}>
			{inheritNote ? <div className="alert alert-info py-2">{inheritNote}</div> : null}
			<div className="text-secondary small mb-3">
				普通反代保持默认即可；只有在多上游、内网证书、gRPC 或慢服务场景下再配置这些项。
			</div>
			<Section
				title="健康检查"
				summary={healthSummary}
				description="上游故障时自动摘除，恢复后再加入。"
				open={activeHealthEnabled || passiveHealthEnabled}
			>
				<div className="divide-y">
					<div className="pb-3">
						<SwitchField
							name={`${name}.healthCheck.enabled`}
							id={`${name}-health-enabled`}
							label="主动健康检查"
							help="Caddy 会定期请求健康路径，失败的上游会暂时摘除。"
						/>
					</div>
					{activeHealthEnabled ? (
						<div className="pt-3">
							<div className="row">
								<div className="col-md-4">
									<TextField
										name={`${name}.healthCheck.uri`}
										id={`${name}-health-uri`}
										label="检查路径"
										placeholder="/healthz"
									/>
								</div>
								<div className="col-md-3">
									<SelectField
										name={`${name}.healthCheck.interval`}
										id={`${name}-health-interval`}
										label="间隔"
										options={durationOptions}
									/>
								</div>
								<div className="col-md-3">
									<SelectField
										name={`${name}.healthCheck.timeout`}
										id={`${name}-health-timeout`}
										label="超时"
										options={durationOptions}
									/>
								</div>
								<div className="col-md-2">
									<SelectField
										name={`${name}.healthCheck.status`}
										id={`${name}-health-status`}
										label="成功状态"
										options={statusOptions}
									/>
								</div>
							</div>
						</div>
					) : null}
					<div className="pt-3">
						<SwitchField
							name={`${name}.healthCheck.passiveEnabled`}
							id={`${name}-passive-health-enabled`}
							label="被动失败检测"
							help="根据请求失败和响应状态临时摘除异常上游。"
						/>
						{passiveHealthEnabled ? (
							<div className="row mt-3">
								<div className="col-md-4">
									<SelectField
										name={`${name}.healthCheck.failDuration`}
										id={`${name}-fail-duration`}
										label="失败窗口"
										options={durationOptions}
									/>
								</div>
								<div className="col-md-4">
									<TextField
										name={`${name}.healthCheck.maxFails`}
										id={`${name}-max-fails`}
										label="最大失败次数"
										type="number"
										min={0}
									/>
								</div>
								<div className="col-md-4">
									<SelectField
										name={`${name}.healthCheck.unhealthyStatus`}
										id={`${name}-unhealthy-status`}
										label="失败状态"
										options={statusOptions}
									/>
								</div>
							</div>
						) : null}
					</div>
				</div>
			</Section>
			<Section
				title="重试策略"
				summary={retrySummary}
				description="上游短暂不可用时，在同一次请求内重试。"
				open={retryEnabled}
			>
				<div className="mb-3">
					<SwitchField
						name={`${name}.retry.enabled`}
						id={`${name}-retry-enabled`}
						label="启用重试"
						help="用于上游拨号失败、临时不可用等场景。"
					/>
				</div>
				{retryEnabled ? (
					<div className="row">
						<div className="col-md-3">
							<SelectField
								name={`${name}.retry.tryDuration`}
								id={`${name}-try-duration`}
								label="重试窗口"
								options={durationOptions}
							/>
						</div>
						<div className="col-md-3">
							<SelectField
								name={`${name}.retry.tryInterval`}
								id={`${name}-try-interval`}
								label="重试间隔"
								options={durationOptions}
							/>
						</div>
						<div className="col-md-3">
							<TextField
								name={`${name}.retry.retries`}
								id={`${name}-retries`}
								label="最大次数"
								type="number"
								min={0}
							/>
						</div>
						<div className="col-md-3">
							<SelectField
								name={`${name}.retry.methods`}
								id={`${name}-retry-methods`}
								label="仅重试方法"
								options={[
									{ value: "", label: "Caddy 默认" },
									{ value: "get_head", label: "GET / HEAD" },
								]}
							/>
						</div>
					</div>
				) : null}
			</Section>
			<Section
				title="请求/响应头"
				summary={headerSummary}
				description="改写发往上游的请求头，或追加返回客户端的响应头。"
				open={headerSummary !== "默认"}
			>
				<div className="row">
					<div className="col-md-4">
						<SelectField
							name={`${name}.headers.hostMode`}
							id={`${name}-host-mode`}
							label="Host 头"
							options={[
								{ value: "", label: "默认" },
								{ value: "host", label: "原始域名" },
								{ value: "upstream", label: "上游地址" },
								{ value: "custom", label: "自定义" },
							]}
						/>
					</div>
					{customHostEnabled ? (
						<div className="col-md-8">
							<TextField
								name={`${name}.headers.hostValue`}
								id={`${name}-host-value`}
								label="自定义 Host"
								placeholder="app.internal"
							/>
						</div>
					) : null}
				</div>
				<HeaderRules name={`${name}.headers.request`} label="发往上游的请求头" />
				<HeaderRules name={`${name}.headers.response`} label="返回客户端的响应头" />
			</Section>
			<Section
				title="上游连接"
				summary={transportSummary}
				description="控制 Caddy 连接上游时的协议、TLS 和超时。"
				open={!!hasConnectionSettings}
			>
				<div className="row">
					<div className={hasHTTPSUpstream ? "col-md-4" : "col-md-12"}>
						<SelectField
							name={`${name}.transport.protocol`}
							id={`${name}-transport-protocol`}
							label="上游 HTTP 版本"
							options={[
								{ value: "", label: "默认" },
								{ value: "h2c", label: "h2c（明文 HTTP/2）" },
								{ value: "http1", label: "HTTP/1.1" },
							]}
						/>
					</div>
					{hasHTTPSUpstream ? (
						<div className="col-md-8">
							<TextField
								name={`${name}.transport.tlsServerName`}
								id={`${name}-tls-server-name`}
								label="上游 TLS 主机名"
								placeholder="app.internal"
							/>
						</div>
					) : null}
				</div>
				{hasHTTPSUpstream ? (
					<div className="mb-3">
						<SwitchField
							name={`${name}.transport.tlsInsecureSkipVerify`}
							id={`${name}-tls-skip-verify`}
							label="跳过上游 HTTPS 证书校验"
							help="仅用于自签名或内网证书。公网或不可信网络不建议开启。"
						/>
					</div>
				) : null}
				<div className="row">
					<div className="col-md-3">
						<SelectField
							name={`${name}.transport.dialTimeout`}
							id={`${name}-dial-timeout`}
							label="连接超时"
							options={durationOptions}
						/>
					</div>
					<div className="col-md-3">
						<SelectField
							name={`${name}.transport.responseHeaderTimeout`}
							id={`${name}-response-header-timeout`}
							label="响应头超时"
							options={durationOptions}
						/>
					</div>
					<div className="col-md-3">
						<SelectField
							name={`${name}.transport.readTimeout`}
							id={`${name}-read-timeout`}
							label="读超时"
							options={durationOptions}
						/>
					</div>
					<div className="col-md-3">
						<SelectField
							name={`${name}.transport.writeTimeout`}
							id={`${name}-write-timeout`}
							label="写超时"
							options={durationOptions}
						/>
					</div>
				</div>
			</Section>
		</div>
	);
}
