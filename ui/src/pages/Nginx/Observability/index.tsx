import {
	IconActivityHeartbeat,
	IconAlertCircle,
	IconAlertTriangle,
	IconClock,
	IconGauge,
	IconHelp,
	IconRefresh,
	IconServer,
} from "@tabler/icons-react";
import type React from "react";
import Alert from "react-bootstrap/Alert";
import { Button, LoadingPage } from "src/components";
import { useObservability } from "src/hooks";
import { showHelpModal } from "src/modals";
import type { ObservabilityLogEvent, ObservabilitySite, ObservabilitySiteUpstream } from "src/api/backend";
import styles from "./Observability.module.css";

const statusKeys = ["2xx", "3xx", "4xx", "5xx"] as const;

function fmtNumber(value?: number) {
	return Math.round(value || 0).toLocaleString();
}

function fmtDecimal(value?: number, digits = 2) {
	return (value || 0).toLocaleString(undefined, {
		minimumFractionDigits: value ? digits : 0,
		maximumFractionDigits: digits,
	});
}

function fmtMs(value?: number) {
	if (!value) return "-";
	return `${fmtDecimal(value, value >= 100 ? 0 : 1)} ms`;
}

function fmtP95(value?: number, capped?: boolean) {
	if (!value) return "-";
	if (capped && value >= 1000) return `> ${fmtDecimal(value / 1000, value % 1000 ? 1 : 0)} s`;
	return fmtMs(value);
}

function statusTone(key: string) {
	switch (key) {
		case "2xx":
			return styles.statusSuccess;
		case "3xx":
			return styles.statusRedirect;
		case "4xx":
			return styles.statusClientError;
		case "5xx":
			return styles.statusServerError;
		default:
			return styles.statusEmpty;
	}
}

function upstreamText(status: string) {
	switch (status) {
		case "healthy":
			return "正常";
		case "unhealthy":
			return "异常";
		default:
			return "未配置检测";
	}
}

function kindText(kind: string) {
	switch (kind) {
		case "static":
			return "静态";
		case "redirection":
			return "重定向";
		case "dead":
			return "错误页";
		default:
			return "代理";
	}
}

function statusTotal(classes?: Record<string, number>) {
	return statusKeys.reduce((total, key) => total + (classes?.[key] || 0), 0);
}

function StatusBuckets({ classes }: { classes?: Record<string, number> }) {
	const total = statusTotal(classes);
	return (
		<div className={styles.statusDistribution}>
			<div className={styles.statusTrack} role="img" aria-label={`共 ${fmtNumber(total)} 个响应`}>
				{statusKeys.map((key) => {
					const count = classes?.[key] || 0;
					const pct = total ? (count / total) * 100 : 0;
					return pct ? <span key={key} className={statusTone(key)} style={{ width: `${pct}%` }} /> : null;
				})}
			</div>
			<div className={styles.statusLegend}>
			{statusKeys.map((key) => {
				const count = classes?.[key] || 0;
				const pct = total ? Math.round((count / total) * 100) : 0;
				return (
					<div className={styles.statusLegendItem} key={key}>
						<span className={`${styles.statusDot} ${statusTone(key)}`} />
						<span>{key}</span>
						<strong>{fmtNumber(count)}</strong>
						<span className="text-muted">{pct}%</span>
					</div>
				);
			})}
			</div>
		</div>
	);
}

function MetricCard({
	title,
	value,
	subtitle,
	icon,
	color,
}: {
	title: string;
	value: string;
	subtitle?: string;
	icon: React.ReactNode;
	color: string;
}) {
	return (
		<div className="col-sm-6 col-xl-3">
			<div className="card card-sm">
				<div className="card-body">
					<div className="row align-items-center">
						<div className="col-auto">
							<span className={`avatar text-white ${color}`}>{icon}</span>
						</div>
						<div className="col">
							<div className="text-muted small">{title}</div>
							<div className="h2 mb-0">{value}</div>
							{subtitle ? <div className="text-muted small mt-1">{subtitle}</div> : null}
						</div>
					</div>
				</div>
			</div>
		</div>
	);
}

function UpstreamList({ upstreams }: { upstreams?: ObservabilitySiteUpstream[] }) {
	if (!upstreams?.length) {
		return <span className="text-muted">-</span>;
	}
	return (
		<div className={styles.siteUpstreams}>
			{upstreams.map((upstream) => (
				<UpstreamStatus address={upstream.address} status={upstream.status} key={upstream.address} />
			))}
		</div>
	);
}

function UpstreamStatus({ address, status }: { address: string; status: string }) {
	return (
		<div className={styles.upstreamStatus}>
			<span className={`${styles.upstreamDot} ${styles[`upstream_${status}`] || styles.upstream_unknown}`} />
			<span className={styles.upstreamAddress}>{address}</span>
			<span className="text-muted small">{upstreamText(status)}</span>
		</div>
	);
}

function ErrorRows({ events, emptyText }: { events?: ObservabilityLogEvent[]; emptyText: string }) {
	if (!events?.length) {
		return (
			<tr>
				<td colSpan={4} className="text-center text-muted py-4">
					{emptyText}
				</td>
			</tr>
		);
	}
	return (
		<>
			{events.map((event, idx) => (
				<tr key={`${event.time}-${event.source}-${idx}`}>
					<td className="text-nowrap">{event.time || "-"}</td>
					<td>
						{event.sites?.length ? (
							<span className={styles.errorSites}>{event.sites.join("、")}</span>
						) : (
							<span className="text-muted">全局</span>
						)}
					</td>
					<td className="text-nowrap">{event.source || "-"}</td>
					<td className="text-break">
						<div className={styles.errorMessage}>
							<IconAlertCircle size={16} />
							<span>{event.message}</span>
						</div>
					</td>
				</tr>
			))}
		</>
	);
}

function SiteRows({ sites }: { sites: ObservabilitySite[] }) {
	if (!sites.length) {
		return (
			<tr>
				<td colSpan={8} className="text-center text-muted py-4">
					暂无站点
				</td>
			</tr>
		);
	}
	return (
		<>
			{sites.map((site) => (
				<tr key={site.id}>
					<td>
						<div className="font-weight-medium">{site.name}</div>
						<div className="text-muted small">{kindText(site.kind)}</div>
					</td>
					<td className="text-break">{site.domains?.join("、") || "-"}</td>
					<td>
						{site.accessLogEnabled ? (
							<span className={styles.enabledText}>已启用</span>
						) : (
							<span className="text-muted">未启用</span>
						)}
					</td>
					<td className="text-end">{fmtNumber(site.requests)}</td>
					<td className="text-end">{fmtMs(site.p95Ms)}</td>
					{statusKeys.map((key) => (
						<td className={`text-end ${statusTone(key)}`} key={key}>
							{fmtNumber(site.statusClasses?.[key])}
						</td>
					))}
					<td>
						<UpstreamList upstreams={site.upstreams} />
					</td>
				</tr>
			))}
		</>
	);
}

export default function ObservabilityPage() {
	const { data, isLoading, isError, error, isFetching, refetch } = useObservability();

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "加载运行观测失败"}</Alert>;
	if (!data) return null;

	const healthyUpstreams = data.upstreams.filter((item) => item.status === "healthy").length;
	const unhealthyUpstreams = data.upstreams.filter((item) => item.status === "unhealthy").length;
	const unknownUpstreams = data.upstreams.filter((item) => item.status === "unknown").length;
	const checkedUpstreams = healthyUpstreams + unhealthyUpstreams;
	const upstreamSubtitle = data.upstreams.length
		? [
				unhealthyUpstreams ? `${fmtNumber(unhealthyUpstreams)} 个异常` : "",
				unknownUpstreams ? `${fmtNumber(unknownUpstreams)} 个未配置主动检查` : "",
				!unhealthyUpstreams && !unknownUpstreams ? "全部已验证正常" : "",
			]
				.filter(Boolean)
				.join("，")
		: "暂无上游";

	return (
		<div>
			<div className="row align-items-center mb-3">
				<div className="col">
					<h2 className="mb-0">运行观测</h2>
					<div className="text-muted small">采集时间：{data.collectedAt || "-"}</div>
				</div>
				<div className="col-auto d-flex gap-2">
					<Button
						size="sm"
						variant="ghost"
						onClick={() => showHelpModal("Observability", "azure")}
						title="功能说明"
						ariaLabel="功能说明"
					>
						<IconHelp size={18} />
					</Button>
					<Button size="sm" variant="ghost" isLoading={isFetching} onClick={() => refetch()}>
						<IconRefresh size={18} />
					</Button>
				</div>
			</div>

			{!data.available ? (
				<Alert variant="warning">运行指标暂不可用：{data.error || "无法读取运行指标"}</Alert>
			) : null}

			{data.healthAlerts?.length ? (
				<div className="mb-3">
					{data.healthAlerts.map((alert) => (
						<Alert variant="danger" key={alert.target} className="mb-2">
							<div className="fw-semibold">
								上游 {alert.target} 的主动健康检查持续失败（最近 24 小时命中 {alert.count} 次）
							</div>
							<div className="small mt-1">{alert.hint}</div>
						</Alert>
					))}
				</div>
			) : null}

			{data.warnings?.length ? (
				<Alert variant="warning" className="mb-3">
					<div className="fw-semibold mb-1">需要处理的配置问题</div>
					<ul className="mb-0 ps-3 small">
						{data.warnings.map((warning) => (
							<li key={warning}>{warning}</li>
						))}
					</ul>
				</Alert>
			) : null}

			<div className="row row-cards mb-3">
				<MetricCard
					title="业务 QPS"
					value={fmtDecimal(data.traffic.qps)}
					subtitle={`累计 ${fmtNumber(data.traffic.totalRequests)} 次请求，不含管理面板`}
					icon={<IconActivityHeartbeat size={24} />}
					color="bg-azure"
				/>
				<MetricCard
					title="P95 延迟"
					value={fmtP95(data.traffic.p95Ms, data.traffic.p95Capped)}
					subtitle={
						data.traffic.p95Capped
							? `高于可精确估算范围 · ${fmtNumber(data.traffic.inFlight)} 个处理中请求`
							: `${fmtNumber(data.traffic.inFlight)} 个处理中请求`
					}
					icon={<IconGauge size={24} />}
					color="bg-green"
				/>
				<MetricCard
					title="上游检测"
					value={checkedUpstreams ? `${fmtNumber(healthyUpstreams)} / ${fmtNumber(checkedUpstreams)} 正常` : "未启用"}
					subtitle={upstreamSubtitle}
					icon={<IconServer size={24} />}
					color={unhealthyUpstreams ? "bg-red" : unknownUpstreams ? "bg-secondary" : "bg-indigo"}
				/>
				<MetricCard
					title="错误趋势"
					value={`${fmtNumber(data.errorTrend.last1h)} / 1h`}
					subtitle={`15m ${fmtNumber(data.errorTrend.last15m)} · 24h ${fmtNumber(data.errorTrend.last24h)}`}
					icon={<IconAlertTriangle size={24} />}
					color={data.errorTrend.last1h ? "bg-red" : "bg-yellow"}
				/>
			</div>

			<div className="row row-cards">
				<div className="col-12">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">状态码分布</h3>
							<div className="card-actions text-muted small">共 {fmtNumber(statusTotal(data.traffic.statusClasses))} 个响应</div>
						</div>
						<div className="card-body">
							<StatusBuckets classes={data.traffic.statusClasses} />
						</div>
					</div>
				</div>
				<div className="col-12">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">上游状态</h3>
							<div className="card-actions text-muted small">
								主动检测 {fmtNumber(checkedUpstreams)} / {fmtNumber(data.upstreams.length)}
							</div>
						</div>
						<div className="card-body">
							{data.upstreams.length ? (
								<div className={styles.upstreamGrid}>
									{data.upstreams.map((upstream) => (
										<UpstreamStatus address={upstream.address} status={upstream.status} key={upstream.address} />
									))}
								</div>
							) : (
								<div className="text-center text-muted py-3">暂无上游状态数据</div>
							)}
						</div>
					</div>
				</div>

				<div className="col-12">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">站点详情</h3>
						</div>
						<div className="table-responsive">
							<table className={`table card-table table-vcenter ${styles.siteTable}`}>
								<thead>
									<tr>
										<th>站点</th>
										<th>域名</th>
										<th>访问日志</th>
										<th className="text-end">请求</th>
										<th className="text-end">
											<span className="d-inline-flex align-items-center gap-1">
												<IconClock size={16} />
												P95
											</span>
										</th>
										{statusKeys.map((key) => (
											<th className="text-end" key={key}>
												{key}
											</th>
										))}
										<th>上游</th>
									</tr>
								</thead>
								<tbody>
									<SiteRows sites={data.sites} />
								</tbody>
							</table>
						</div>
					</div>
				</div>

				<div className="col-12">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">最近错误</h3>
						</div>
						<div className="table-responsive">
							<table className={`table card-table table-vcenter ${styles.errorTable}`}>
								<thead>
									<tr>
										<th>时间</th>
										<th>关联站点</th>
										<th>来源</th>
										<th>内容</th>
									</tr>
								</thead>
								<tbody>
									<ErrorRows events={data.recentErrors} emptyText="暂无错误" />
								</tbody>
							</table>
						</div>
					</div>
				</div>
			</div>
		</div>
	);
}
