import { IconDotsVertical, IconEdit, IconPlayerPlay, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { DynamicDNS } from "src/api/backend";
import { CreatedOnFormatter, EmptyData, HasPermission } from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { MANAGE, STREAMS } from "src/modules/Permissions";

const metaValue = (meta: Record<string, any> | undefined, camelKey: string, snakeKey?: string) =>
	meta?.[camelKey] ?? meta?.[snakeKey || camelKey];

const dynamicDNSProvider = (item: DynamicDNS) =>
	item.dnsProvider || metaValue(item.meta, "dnsProvider", "dns_provider") || "-";

interface Props {
	data: DynamicDNS[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onEdit?: (id: number) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onCheck?: (id: number) => void;
	onNew?: () => void;
}

export default function Table({
	data,
	isFetching,
	isFiltered,
	onEdit,
	onDelete,
	onDisableToggle,
	onCheck,
	onNew,
}: Props) {
	const columnHelper = createColumnHelper<DynamicDNS>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: DynamicDNS) => row, {
				id: "name",
				header: intl.formatMessage({ id: "name" }),
				sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
				cell: (info) => <span className="font-weight-medium">{info.getValue().name}</span>,
			}),
			columnHelper.accessor((row: DynamicDNS) => row.createdOn, {
				id: "createdOn",
				header: "创建时间",
				sortingFn: (a, b) => (a.original.createdOn || "").localeCompare(b.original.createdOn || ""),
				cell: (info) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: DynamicDNS) => row.domainNames, {
				id: "domainNames",
				header: intl.formatMessage({ id: "dynamic-dns.domains" }),
				enableSorting: false,
				cell: (info) => (
					<>
						{info.getValue().map((domain) => (
							<span className="badge badge-lg domain-name" key={domain}>
								{domain}
							</span>
						))}
					</>
				),
			}),
			columnHelper.accessor((row: DynamicDNS) => row, {
				id: "versions",
				header: intl.formatMessage({ id: "dynamic-dns.ip-versions" }),
				enableSorting: false,
				cell: (info) =>
					[info.getValue().ipv4 ? "IPv4" : "", info.getValue().ipv6 ? "IPv6" : ""]
						.filter(Boolean)
						.join(" / "),
			}),
			columnHelper.accessor((row: DynamicDNS) => row, {
				id: "dnsProvider",
				header: "解析商",
				sortingFn: (a, b) => dynamicDNSProvider(a.original).localeCompare(dynamicDNSProvider(b.original)),
				cell: (info) => <span className="badge bg-secondary-lt">{dynamicDNSProvider(info.getValue())}</span>,
			}),
			columnHelper.accessor((row: DynamicDNS) => row.checkInterval, {
				id: "checkInterval",
				header: intl.formatMessage({ id: "dynamic-dns.check-interval" }),
			}),
			columnHelper.accessor((row: DynamicDNS) => row, {
				id: "enabled",
				header: intl.formatMessage({ id: "column.status" }),
				enableSorting: false,
				cell: (info) => {
					const item = info.getValue();
					const lastError = metaValue(item.meta, "lastError", "last_error");
					if (!item.enabled) {
						return (
							<span className="badge bg-secondary">
								<T id="dynamic-dns.status-offline" />
							</span>
						);
					}
					if (lastError) {
						return (
							<>
								<span className="badge bg-warning text-dark" title={lastError}>
									<T id="dynamic-dns.status-error" />
								</span>
								<div
									className="text-danger small mt-1"
									style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
								>
									{lastError}
								</div>
							</>
						);
					}
					return (
						<span className="badge bg-success">
							<T id="dynamic-dns.status-online" />
						</span>
					);
				},
			}),
			columnHelper.display({
				id: "id",
				cell: (info) => (
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
									tData={{ object: "dynamic-dns" }}
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
							<HasPermission section={STREAMS} permission={MANAGE} hideError>
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onCheck?.(info.row.original.id);
									}}
								>
									<IconPlayerPlay size={16} />
									检查
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
				),
				meta: { className: "text-end w-1" },
			}),
		],
		[columnHelper, onEdit, onCheck, onDisableToggle, onDelete],
	);
	const [sorting, setSorting] = useState<SortingState>([]);
	const tableInstance = useReactTable<DynamicDNS>({
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
			emptyState={
				<EmptyData
					object="dynamic-dns"
					objects="dynamic-dns"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="cyan"
					permissionSection={STREAMS}
				/>
			}
		/>
	);
}
