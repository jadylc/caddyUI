import {
	IconActivityHeartbeat,
	IconAlertTriangle,
	IconClock,
	IconGauge,
	IconRefresh,
	IconServer,
} from "@tabler/icons-react";
import type React from "react";
import Alert from "react-bootstrap/Alert";
import { Button, LoadingPage } from "src/components";
import { useObservability } from "src/hooks";
import type { ObservabilityLogEvent, ObservabilitySite, ObservabilitySiteUpstream } from "src/api/backend";

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

function statusClassBadge(key: string) {
	switch (key) {
		case "2xx":
			return "bg-success";
		case "3xx":
			return "bg-info";
		case "4xx":
			return "bg-warning text-dark";
		case "5xx":
			return "bg-danger";
		default:
			return "bg-secondary";
	}
}

function upstreamBadge(status: string) {
	switch (status) {
		case "healthy":
			return "bg-success";
		case "unhealthy":
			return "bg-danger";
		default:
			return "bg-secondary";
	}
}

function upstreamText(status: string) {
	switch (status) {
		case "healthy":
			return "正常";
		case "unhealthy":
			return "异常";
		default:
			return "未知";
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
		<div className="d-flex flex-column gap-2">
			{statusKeys.map((key) => {
				const count = classes?.[key] || 0;
				const pct = total ? Math.round((count / total) * 100) : 0;
				return (
					<div key={key}>
						<div className="d-flex justify-content-between small mb-1">
							<span className={`badge ${statusClassBadge(key)}`}>{key}</span>
							<span className="text-muted">
								{fmtNumber(count)} / {pct}%
							</span>
						</div>
						<div className="progress progress-sm">
							<div className={`progress-bar ${statusClassBadge(key)}`} style={{ width: `${pct}%` }} />
						</div>
					</div>
				);
			})}
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
		<div className="d-flex flex-wrap gap-1">
			{upstreams.map((upstream) => (
				<span className={`badge ${upstreamBadge(upstream.status)}`} key={upstream.address}>
					{upstream.address} · {upstreamText(upstream.status)}
				</span>
			))}
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
						<span className={`badge ${event.level === "error" ? "bg-danger" : "bg-warning text-dark"}`}>
							{event.level === "error" ? "错误" : "告警"}
						</span>
					</td>
					<td className="text-nowrap">{event.source || "-"}</td>
					<td className="text-break">{event.message}</td>
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
							<span className="badge bg-success">已启用</span>
						) : (
							<span className="badge bg-secondary">未启用</span>
						)}
					</td>
					<td className="text-end">{fmtNumber(site.requests)}</td>
					<td className="text-end">{fmtMs(site.p95Ms)}</td>
					{statusKeys.map((key) => (
						<td className="text-end" key={key}>
							<span className={`badge ${statusClassBadge(key)}`}>{fmtNumber(site.statusClasses?.[key])}</span>
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

	const unhealthyUpstreams = data.upstreams.filter((item) => !item.healthy).length;

	return (
		<div>
			<div className="row align-items-center mb-3">
				<div className="col">
					<h2 className="mb-0">运行观测</h2>
					<div className="text-muted small">采集时间：{data.collectedAt || "-"}</div>
				</div>
				<div className="col-auto">
					<Button size="sm" variant="ghost" isLoading={isFetching} onClick={() => refetch()}>
						<IconRefresh size={18} />
					</Button>
				</div>
			</div>

			{!data.available ? (
				<Alert variant="warning">运行指标暂不可用：{data.error || "无法读取运行指标"}</Alert>
			) : null}

			<div className="row row-cards mb-3">
				<MetricCard
					title="当前 QPS"
					value={fmtDecimal(data.traffic.qps)}
					subtitle={`累计 ${fmtNumber(data.traffic.totalRequests)} 次请求`}
					icon={<IconActivityHeartbeat size={24} />}
					color="bg-azure"
				/>
				<MetricCard
					title="P95 延迟"
					value={fmtMs(data.traffic.p95Ms)}
					subtitle={`${fmtNumber(data.traffic.inFlight)} 个处理中请求`}
					icon={<IconGauge size={24} />}
					color="bg-green"
				/>
				<MetricCard
					title="上游状态"
					value={`${fmtNumber(data.upstreams.length - unhealthyUpstreams)} / ${fmtNumber(data.upstreams.length)}`}
					subtitle={unhealthyUpstreams ? `${unhealthyUpstreams} 个异常` : "全部正常或暂无上游"}
					icon={<IconServer size={24} />}
					color={unhealthyUpstreams ? "bg-red" : "bg-indigo"}
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
				<div className="col-lg-5">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">状态码分布</h3>
						</div>
						<div className="card-body">
							<StatusBuckets classes={data.traffic.statusClasses} />
						</div>
					</div>
				</div>
				<div className="col-lg-7">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">上游健康</h3>
						</div>
						<div className="table-responsive">
							<table className="table card-table table-vcenter">
								<thead>
									<tr>
										<th>上游</th>
										<th className="w-1">状态</th>
									</tr>
								</thead>
								<tbody>
									{data.upstreams.length ? (
										data.upstreams.map((upstream) => (
											<tr key={upstream.address}>
												<td className="text-break">{upstream.address}</td>
												<td>
													<span className={`badge ${upstreamBadge(upstream.status)}`}>
														{upstreamText(upstream.status)}
													</span>
												</td>
											</tr>
										))
									) : (
										<tr>
											<td colSpan={2} className="text-center text-muted py-4">
												暂无上游健康数据
											</td>
										</tr>
									)}
								</tbody>
							</table>
						</div>
					</div>
				</div>

				<div className="col-12">
					<div className="card">
						<div className="card-header">
							<h3 className="card-title">站点详情</h3>
						</div>
						<div className="table-responsive">
							<table className="table card-table table-vcenter">
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
							<table className="table card-table table-vcenter">
								<thead>
									<tr>
										<th>时间</th>
										<th>级别</th>
										<th>来源</th>
										<th>内容</th>
									</tr>
								</thead>
								<tbody>
									<ErrorRows events={data.recentErrors} emptyText="暂无错误或告警" />
								</tbody>
							</table>
						</div>
					</div>
				</div>
			</div>
		</div>
	);
}
