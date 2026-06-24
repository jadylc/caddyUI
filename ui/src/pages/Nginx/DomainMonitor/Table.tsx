import { IconDotsVertical, IconEdit, IconPower, IconRefresh, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { DomainMonitor } from "src/api/backend";
import { CreatedOnFormatter, DateFormatter, EmptyData, HasPermission } from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { CERTIFICATES, MANAGE } from "src/modules/Permissions";
import styles from "./Table.module.css";

interface Props {
	data: DomainMonitor[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onCheck?: (id: number) => void;
	onRenew?: (id: number) => void;
	canRenew?: (item: DomainMonitor) => boolean;
	onEdit?: (id: number) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onNew?: () => void;
}

const metaValue = (item: DomainMonitor, camelKey: string, snakeKey?: string) =>
	item.meta?.[camelKey] ?? item.meta?.[snakeKey || camelKey];

const primaryDomain = (item: DomainMonitor) => {
	const domain = metaValue(item, "domainName", "domain_name");
	if (domain) {
		return domain;
	}
	return item.domainNames[0] || item.name;
};

const primaryDomainCell = (item: DomainMonitor) => (
	<div className={styles.primaryCell}>
		<div className={`${styles.primaryDomain} ${!item.enabled ? "text-red" : ""}`} title={primaryDomain(item)}>
			{primaryDomain(item)}
		</div>
		{item.name && item.name !== primaryDomain(item) ? (
			<div className={`${styles.mutedLine} ${!item.enabled ? "text-red" : ""}`} title={item.name}>
				{item.name}
			</div>
		) : null}
		<div className={styles.targetSummary}>{targetsCell(item.domainNames)}</div>
	</div>
);

const registrarProviderName = (provider?: string) => {
	switch ((provider || "").trim().toLowerCase()) {
		case "alidns":
		case "aliyun":
			return "阿里云";
		case "digitalplat":
			return "DigitalPlat";
		case "dnshe":
			return "DNSHE";
		case "cloudflare":
			return "Cloudflare";
		case "dnspod":
			return "DNSPod";
		default:
			return provider || "";
	}
};

const registrarProviderCell = (item: DomainMonitor) => {
	const provider = item.registrarProvider || metaValue(item, "registrarProvider", "registrar_provider");
	if (!provider) {
		return <span className="text-muted">-</span>;
	}
	return (
		<span
			className={`badge bg-azure-lt text-azure ${styles.registrarBadge}`}
			title={registrarProviderName(provider)}
		>
			{registrarProviderName(provider)}
		</span>
	);
};

const targetsCell = (domains: string[]) => (
	<div className={styles.targetsCell}>
		{domains.map((domain) => (
			<span className={`badge bg-blue-lt text-blue ${styles.targetBadge}`} title={domain} key={domain}>
				{domain}
			</span>
		))}
	</div>
);

const statusLabel = (item: DomainMonitor) => {
	if (!item.enabled) {
		return (
			<span className="status status-muted">
				<span className="status-dot" />
				<T id="domain-monitor.status-offline" />
			</span>
		);
	}
	const status = metaValue(item, "status");
	const lastError = metaValue(item, "lastError", "last_error");
	if (status === "error") {
		return (
			<span className="status status-red" title={lastError || ""}>
				<span className="status-dot" />
				<T id="domain-monitor.status-error" />
			</span>
		);
	}
	if (status === "warning") {
		return (
			<span className="status status-yellow" title={lastError || ""}>
				<span className="status-dot" />
				<T id="domain-monitor.status-warning" />
			</span>
		);
	}
	if (status === "ok") {
		return (
			<span className="status status-green">
				<span className="status-dot" />
				<T id="domain-monitor.status-ok" />
			</span>
		);
	}
	return (
		<span className="status status-muted">
			<span className="status-dot" />
			<T id="domain-monitor.status-unknown" />
		</span>
	);
};

const errorText = (item: DomainMonitor) => {
	const message = metaValue(item, "lastError", "last_error");
	if (!message) return null;
	return (
		<div
			className="text-danger small mt-1"
			style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
		>
			{message}
		</div>
	);
};

const sslCell = (item: DomainMonitor) => {
	if (!item.checkSsl) {
		return <span className="text-muted">-</span>;
	}
	const expiresOn = metaValue(item, "sslExpiresOn", "ssl_expires_on");
	const daysLeft = metaValue(item, "sslDaysLeft", "ssl_days_left");
	const issuer = metaValue(item, "sslIssuer", "ssl_issuer");
	const source = metaValue(item, "sslSource", "ssl_source");
	if (!expiresOn) {
		return <span className="text-muted">-</span>;
	}
	return (
		<div className={styles.metricCell}>
			<div className={styles.metricMain}>
				<DateFormatter value={expiresOn} highlightPast highlistNearlyExpired />
			</div>
			{typeof daysLeft === "number" ? (
				<div className={styles.metricMeta}>
					<T id="domain-monitor.days-left" data={{ days: daysLeft }} />
				</div>
			) : null}
			{issuer ? (
				<div className={styles.metricMeta} title={issuer}>
					<T id="domain-monitor.ssl-issuer" data={{ issuer }} />
				</div>
			) : null}
			{source === "caddy" ? (
				<div className={styles.metricMeta}>
					<T id="domain-monitor.ssl-source.caddy" />
				</div>
			) : null}
		</div>
	);
};

const domainExpiryCell = (item: DomainMonitor) => {
	if (!item.checkDomain) {
		return <span className="text-muted">-</span>;
	}
	const expiresOn = metaValue(item, "domainExpiresOn", "domain_expires_on");
	const daysLeft = metaValue(item, "domainDaysLeft", "domain_days_left");
	const unavailable = metaValue(item, "domainExpiryUnavailable", "domain_expiry_unavailable");
	const unavailableReason = metaValue(item, "domainExpiryUnavailableReason", "domain_expiry_unavailable_reason");
	if (!expiresOn) {
		if (unavailable) {
			return (
				<div className={styles.metricCell}>
					<div className="text-muted">不可用</div>
					{unavailableReason ? (
						<div className={styles.metricMeta} title={unavailableReason}>
							{unavailableReason}
						</div>
					) : null}
				</div>
			);
		}
		return <span className="text-muted">-</span>;
	}
	return (
		<div className={styles.metricCell}>
			<div className={styles.metricMain}>
				<DateFormatter value={expiresOn} highlightPast highlistNearlyExpired />
			</div>
			{typeof daysLeft === "number" ? (
				<div className={styles.metricMeta}>
					<T id="domain-monitor.days-left" data={{ days: daysLeft }} />
				</div>
			) : null}
		</div>
	);
};

const checkCell = (item: DomainMonitor) => {
	const lastChecked = metaValue(item, "lastChecked", "last_checked");
	return (
		<div className={styles.metricCell}>
			<div className={styles.metricMain}>
				{lastChecked ? <DateFormatter value={lastChecked} /> : <span className="text-muted">-</span>}
			</div>
			<div className={styles.metricMeta}>
				<T id="domain-monitor.check-every" data={{ interval: item.checkInterval }} />
			</div>
		</div>
	);
};

const dnsCell = (item: DomainMonitor) => {
	if (!item.checkDns) {
		return <span className="text-muted">-</span>;
	}
	const ips = (metaValue(item, "resolvedIps", "resolved_ips") || []) as string[];
	if (!ips.length) {
		return <span className="text-muted">-</span>;
	}
	return (
		<div className={styles.dnsCell}>
			{ips.slice(0, 4).map((ip) => (
				<span className={`badge bg-secondary-lt ${styles.ipBadge}`} title={ip} key={ip}>
					{ip}
				</span>
			))}
			{ips.length > 4 ? <span className="badge bg-secondary-lt">+{ips.length - 4}</span> : null}
		</div>
	);
};

export default function Table({
	data,
	isFetching,
	isFiltered,
	onCheck,
	onRenew,
	canRenew,
	onEdit,
	onDelete,
	onDisableToggle,
	onNew,
}: Props) {
	const columnHelper = createColumnHelper<DomainMonitor>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "name",
				header: intl.formatMessage({ id: "domain-monitor.primary-domain" }),
				sortingFn: (a, b) => primaryDomain(a.original).localeCompare(primaryDomain(b.original)),
				cell: (info) => primaryDomainCell(info.getValue()),
				meta: { className: `align-top ${styles.primaryColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "status",
				header: intl.formatMessage({ id: "column.status" }),
				enableSorting: false,
				cell: (info) => (
					<div className={styles.statusCell}>
						{statusLabel(info.getValue())}
						{errorText(info.getValue())}
					</div>
				),
				meta: { className: `align-top ${styles.statusColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row.createdOn, {
				id: "createdOn",
				header: "创建时间",
				sortingFn: (a, b) => (a.original.createdOn || "").localeCompare(b.original.createdOn || ""),
				cell: (info) => (
					<span className={styles.createdCell}>
						<CreatedOnFormatter value={info.getValue()} />
					</span>
				),
				meta: { className: `align-top ${styles.createdColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "registrarProvider",
				header: "域名商",
				enableSorting: false,
				cell: (info) => registrarProviderCell(info.getValue()),
				meta: { className: `align-top ${styles.registrarColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "domainExpiry",
				header: intl.formatMessage({ id: "domain-monitor.domain-expires" }),
				enableSorting: false,
				cell: (info) => domainExpiryCell(info.getValue()),
				meta: { className: `align-top ${styles.dateColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "ssl",
				header: intl.formatMessage({ id: "domain-monitor.ssl-expires" }),
				enableSorting: false,
				cell: (info) => sslCell(info.getValue()),
				meta: { className: `align-top ${styles.sslColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "dns",
				header: intl.formatMessage({ id: "domain-monitor.resolved-ips" }),
				enableSorting: false,
				cell: (info) => dnsCell(info.getValue()),
				meta: { className: `align-top ${styles.dnsColumn}` },
			}),
			columnHelper.accessor((row: DomainMonitor) => row, {
				id: "check",
				header: intl.formatMessage({ id: "domain-monitor.check" }),
				enableSorting: false,
				cell: (info) => checkCell(info.getValue()),
				meta: { className: `align-top ${styles.checkColumn}` },
			}),
			columnHelper.display({
				id: "id",
				cell: (info) => (
					<span className="dropdown">
						<button
							type="button"
							className="btn dropdown-toggle btn-action btn-sm px-1"
							data-bs-boundary="viewport"
							data-bs-popper-config='{"strategy":"fixed"}'
							data-bs-toggle="dropdown"
						>
							<IconDotsVertical />
						</button>
						<div className="dropdown-menu dropdown-menu-end">
							<span className="dropdown-header">
								<T
									id="object.actions-title"
									tData={{ object: "domain-monitor" }}
									data={{ id: info.row.original.id }}
								/>
							</span>
							<a
								className="dropdown-item"
								href="#"
								onClick={(e) => {
									e.preventDefault();
									onCheck?.(info.row.original.id);
								}}
							>
								<IconRefresh size={16} />
								<T id="domain-monitor.action.check" />
							</a>
							{canRenew?.(info.row.original) ? (
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onRenew?.(info.row.original.id);
									}}
								>
									<IconRefresh size={16} />
									<span>手动续期</span>
								</a>
							) : null}
							<a
								className="dropdown-item"
								href="#"
								onClick={(e) => {
									e.preventDefault();
									onEdit?.(info.row.original.id);
								}}
							>
								<IconEdit size={16} />
								<T id="action.edit" />
							</a>
							<HasPermission section={CERTIFICATES} permission={MANAGE} hideError>
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onDisableToggle?.(info.row.original.id, !info.row.original.enabled);
									}}
								>
									<IconPower size={16} />
									<T id={info.row.original.enabled ? "action.disable" : "action.enable"} />
								</a>
								<div className="dropdown-divider" />
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onDelete?.(info.row.original.id);
									}}
								>
									<IconTrash size={16} />
									<T id="action.delete" />
								</a>
							</HasPermission>
						</div>
					</span>
				),
				meta: { className: `text-end ${styles.actionColumn}` },
			}),
		],
		[columnHelper, onCheck, onRenew, canRenew, onEdit, onDisableToggle, onDelete],
	);
	const [sorting, setSorting] = useState<SortingState>([]);
	const tableInstance = useReactTable<DomainMonitor>({
		columns,
		data,
		state: { sorting },
		onSortingChange: setSorting,
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		rowCount: data.length,
		meta: { isFetching },
		enableSortingRemoval: false,
	});

	return (
		<TableLayout
			tableInstance={tableInstance}
			tableClassName={styles.monitorTable}
			emptyState={
				<EmptyData
					object="domain-monitor"
					objects="domain-monitor"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="azure"
					permissionSection={CERTIFICATES}
				/>
			}
		/>
	);
}
