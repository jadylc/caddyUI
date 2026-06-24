import { IconCopy, IconDotsVertical, IconEdit, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { ProxyHost } from "src/api/backend";
import {
	AccessListFormatter,
	CertificateFormatter,
	CreatedOnFormatter,
	DomainsFormatter,
	EmptyData,
	HasPermission,
	TrueFalseFormatter,
} from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { MANAGE, PROXY_HOSTS } from "src/modules/Permissions";

interface Props {
	data: ProxyHost[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onEdit?: (id: number) => void;
	onCopy?: (host: ProxyHost) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onNew?: () => void;
}

const getListenPorts = (host: ProxyHost) => {
	const rawPorts = host.listenPorts?.length ? [...host.listenPorts] : host.listenPort ? [host.listenPort] : [];
	for (const domain of host.domainNames || []) {
		const match = String(domain).match(/:(\d+)$/);
		if (match) {
			rawPorts.push(Number(match[1]));
		}
	}
	const seen = new Set<number>();
	return rawPorts.filter((port) => {
		if (!Number.isFinite(port) || port <= 0 || seen.has(port)) {
			return false;
		}
		seen.add(port);
		return true;
	});
};

const proxyHostCertError = (host: ProxyHost) => host.meta?.lastError || host.meta?.last_error || "";

const forwardAuthName = (forwardAuth: any) => {
	const provider = String(forwardAuth?.provider || "").toLowerCase();
	if (provider === "authentik") return "authentik";
	return "Authelia";
};

export default function Table({
	data,
	isFetching,
	onEdit,
	onCopy,
	onDelete,
	onDisableToggle,
	onNew,
	isFiltered,
}: Props) {
	const columnHelper = createColumnHelper<ProxyHost>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: any) => row, {
				id: "serviceName",
				header: "服务名称",
				sortingFn: (a, b) => (a.original.serviceName || "").localeCompare(b.original.serviceName || ""),
				cell: (info: any) => {
					const value = info.getValue().serviceName;
					return value ? (
						<span className="font-weight-medium">{value}</span>
					) : (
						<span className="text-muted">-</span>
					);
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "domainNames",
				header: intl.formatMessage({ id: "column.source" }),
				sortingFn: (a, b) => {
					const aVal = a.original.domainNames?.[0] ?? "";
					const bVal = b.original.domainNames?.[0] ?? "";
					return aVal.localeCompare(bVal);
				},
				cell: (info: any) => {
					const value = info.getValue();
					return (
						<DomainsFormatter domains={value.domainNames} linkScheme={value.sslForced ? "https" : "http"} />
					);
				},
			}),
			columnHelper.accessor((row: any) => row.createdOn, {
				id: "createdOn",
				header: "创建时间",
				sortingFn: (a, b) => (a.original.createdOn || "").localeCompare(b.original.createdOn || ""),
				cell: (info: any) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: any) => row, {
				id: "listenPorts",
				header: intl.formatMessage({ id: "column.listen-ports" }),
				sortingFn: (a, b) => {
					const aVal = getListenPorts(a.original).join(",");
					const bVal = getListenPorts(b.original).join(",");
					return aVal.localeCompare(bVal, undefined, { numeric: true });
				},
				cell: (info: any) => {
					const ports = getListenPorts(info.getValue());
					if (!ports.length) {
						return <span className="text-muted">默认</span>;
					}
					return (
						<div className="d-flex flex-wrap gap-1">
							{ports.map((port) => (
								<span className="badge bg-blue-lt" key={port}>
									:{port}
								</span>
							))}
						</div>
					);
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "forwardHost",
				header: intl.formatMessage({ id: "column.destination" }),
				sortingFn: (a, b) => {
					const aVal = `${a.original.forwardHost}:${a.original.forwardPort}`;
					const bVal = `${b.original.forwardHost}:${b.original.forwardPort}`;
					return aVal.localeCompare(bVal);
				},
				cell: (info: any) => {
					const value = info.getValue();
					return `${value.forwardScheme}://${value.forwardHost}:${value.forwardPort}`;
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "certificate",
				enableSorting: false,
				header: intl.formatMessage({ id: "column.ssl" }),
				cell: (info: any) => {
					const value = info.getValue();
					return (
						<>
							<CertificateFormatter
								certificate={value.certificate}
								certificateId={value.certificateId}
								sslForced={value.sslForced}
								meta={value.meta}
							/>
							{proxyHostCertError(value) ? (
								<div
									className="text-danger small mt-1"
									style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
								>
									{proxyHostCertError(value)}
								</div>
							) : null}
						</>
					);
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "accessList",
				enableSorting: false,
				header: intl.formatMessage({ id: "column.access" }),
				cell: (info: any) => {
					const value = info.getValue();
					const forwardAuth = value.forwardAuth;
					if (!value.accessList && !forwardAuth?.enabled) {
						return <T id="public" />;
					}
					return (
						<div className="d-flex flex-column align-items-start gap-1">
							{value.accessList ? <AccessListFormatter access={value.accessList} /> : null}
							{forwardAuth?.enabled ? (
								<span className="badge bg-lime-lt">认证：{forwardAuthName(forwardAuth)}</span>
							) : null}
						</div>
					);
				},
			}),
			columnHelper.accessor((row: any) => row.enabled, {
				id: "enabled",
				header: intl.formatMessage({ id: "column.status" }),
				cell: (info: any) => {
					return <TrueFalseFormatter value={info.getValue()} trueLabel="online" falseLabel="offline" />;
				},
			}),
			columnHelper.display({
				id: "id",
				cell: (info: any) => {
					return (
						<span className="dropdown">
							<button
								type="button"
								className="btn dropdown-toggle btn-action btn-sm px-1"
								data-bs-boundary="viewport"
								data-bs-toggle="dropdown"
							>
								<IconDotsVertical />
							</button>
							<div className="dropdown-menu dropdown-menu-end">
								<span className="dropdown-header">
									<T
										id="object.actions-title"
										tData={{ object: "proxy-host" }}
										data={{ id: info.row.original.id }}
									/>
								</span>
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
								<HasPermission section={PROXY_HOSTS} permission={MANAGE} hideError>
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onCopy?.(info.row.original);
										}}
									>
										<IconCopy size={16} />
										<T id="action.copy" />
									</a>
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
					);
				},
				meta: {
					className: "text-end w-1",
				},
			}),
		],
		[columnHelper, onEdit, onCopy, onDisableToggle, onDelete],
	);

	const [sorting, setSorting] = useState<SortingState>([]);

	const tableInstance = useReactTable<ProxyHost>({
		columns,
		data,
		state: { sorting },
		onSortingChange: setSorting,
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		rowCount: data.length,
		meta: {
			isFetching,
		},
		enableSortingRemoval: false,
	});

	return (
		<TableLayout
			tableInstance={tableInstance}
			emptyState={
				<EmptyData
					object="proxy-host"
					objects="proxy-hosts"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="lime"
					permissionSection={PROXY_HOSTS}
				/>
			}
		/>
	);
}
