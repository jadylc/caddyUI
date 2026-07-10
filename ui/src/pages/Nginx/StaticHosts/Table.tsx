import { IconCopy, IconDotsVertical, IconEdit, IconFileText, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getPaginationRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { StaticHost } from "src/api/backend";
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
import { MANAGE, STATIC_HOSTS } from "src/modules/Permissions";

interface Props {
	data: StaticHost[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onEdit?: (id: number) => void;
	onCopy?: (host: StaticHost) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onAccessLog?: (host: StaticHost) => void;
	onNew?: () => void;
}

const getListenPorts = (host: StaticHost) => {
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

const staticHostCertError = (host: StaticHost) => host.meta?.lastError || host.meta?.last_error || "";

const modeLabel = (host: StaticHost) => {
	if (host.mode === "spa") return "SPA";
	return "静态";
};

export default function Table({
	data,
	isFetching,
	onEdit,
	onCopy,
	onDelete,
	onDisableToggle,
	onAccessLog,
	onNew,
	isFiltered,
}: Props) {
	const columnHelper = createColumnHelper<StaticHost>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: StaticHost) => row, {
				id: "serviceName",
				header: "服务名称",
				sortingFn: (a, b) => (a.original.serviceName || "").localeCompare(b.original.serviceName || ""),
				cell: (info) => {
					const value = info.getValue().serviceName;
					return value ? (
						<span className="font-weight-medium">{value}</span>
					) : (
						<span className="text-muted">-</span>
					);
				},
			}),
			columnHelper.accessor((row: StaticHost) => row, {
				id: "domainNames",
				header: intl.formatMessage({ id: "column.source" }),
				sortingFn: (a, b) => {
					const aVal = a.original.domainNames?.[0] ?? "";
					const bVal = b.original.domainNames?.[0] ?? "";
					return aVal.localeCompare(bVal);
				},
				cell: (info) => {
					const value = info.getValue();
					return (
						<DomainsFormatter domains={value.domainNames} linkScheme={value.sslForced ? "https" : "http"} />
					);
				},
			}),
			columnHelper.accessor((row: StaticHost) => row.modifiedOn || row.createdOn, {
				id: "modifiedOn",
				header: "修改时间",
				sortingFn: (a, b) =>
					(a.original.modifiedOn || a.original.createdOn || "").localeCompare(
						b.original.modifiedOn || b.original.createdOn || "",
					),
				cell: (info) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: StaticHost) => row, {
				id: "listenPorts",
				header: intl.formatMessage({ id: "column.listen-ports" }),
				sortingFn: (a, b) => {
					const aVal = getListenPorts(a.original).join(",");
					const bVal = getListenPorts(b.original).join(",");
					return aVal.localeCompare(bVal, undefined, { numeric: true });
				},
				cell: (info) => {
					const ports = getListenPorts(info.getValue());
					if (!ports.length) {
						return <span className="text-muted">默认</span>;
					}
					return (
						<div className="d-flex flex-wrap gap-1">
							{ports.map((port) => (
								<span className="badge bg-cyan-lt" key={port}>
									:{port}
								</span>
							))}
						</div>
					);
				},
			}),
			columnHelper.accessor((row: StaticHost) => row.rootPath, {
				id: "rootPath",
				header: "站点目录",
				sortingFn: "alphanumeric",
				cell: (info) => <code className="text-break">{info.getValue()}</code>,
			}),
			columnHelper.accessor((row: StaticHost) => row, {
				id: "mode",
				header: "模式",
				sortingFn: (a, b) => modeLabel(a.original).localeCompare(modeLabel(b.original)),
				cell: (info) => {
					const value = info.getValue();
					return (
						<div className="d-flex flex-wrap gap-1">
							<span className={value.mode === "spa" ? "badge bg-purple-lt" : "badge bg-cyan-lt"}>
								{modeLabel(value)}
							</span>
							{value.directoryBrowsing ? <span className="badge bg-blue-lt">目录浏览</span> : null}
						</div>
					);
				},
			}),
			columnHelper.accessor((row: StaticHost) => row, {
				id: "certificate",
				enableSorting: false,
				header: intl.formatMessage({ id: "column.ssl" }),
				cell: (info) => {
					const value = info.getValue();
					return (
						<>
							<CertificateFormatter
								certificate={value.certificate}
								certificateId={value.certificateId}
								sslForced={value.sslForced}
								meta={value.meta}
							/>
							{staticHostCertError(value) ? (
								<div
									className="text-danger small mt-1"
									style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
								>
									{staticHostCertError(value)}
								</div>
							) : null}
						</>
					);
				},
			}),
			columnHelper.accessor((row: StaticHost) => row, {
				id: "accessList",
				enableSorting: false,
				header: intl.formatMessage({ id: "column.access" }),
				cell: (info) => {
					const value = info.getValue();
					if (!value.accessList) {
						return <T id="public" />;
					}
					return <AccessListFormatter access={value.accessList} />;
				},
			}),
			columnHelper.accessor((row: StaticHost) => row.enabled, {
				id: "enabled",
				header: intl.formatMessage({ id: "column.status" }),
				cell: (info) => <TrueFalseFormatter value={info.getValue()} trueLabel="online" falseLabel="offline" />,
			}),
			columnHelper.display({
				id: "id",
				cell: (info) => {
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
										tData={{ object: "static-host" }}
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
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onAccessLog?.(info.row.original);
									}}
								>
									<IconFileText size={16} />
									查看日志
								</a>
								<HasPermission section={STATIC_HOSTS} permission={MANAGE} hideError>
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
		[columnHelper, onEdit, onCopy, onAccessLog, onDisableToggle, onDelete],
	);

	const [sorting, setSorting] = useState<SortingState>([]);

	const tableInstance = useReactTable<StaticHost>({
		columns,
		data,
		state: { sorting },
		onSortingChange: setSorting,
		getCoreRowModel: getCoreRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
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
					object="static-host"
					objects="static-hosts"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="cyan"
					permissionSection={STATIC_HOSTS}
				/>
			}
		/>
	);
}
