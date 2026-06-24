import { IconBolt, IconDotsVertical, IconEdit, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import cn from "classnames";
import { useMemo, useState } from "react";
import type { WakeDevice } from "src/api/backend";
import { CreatedOnFormatter, EmptyData, HasPermission } from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { formatDateTime, intl, T } from "src/locale";
import { ADMIN, MANAGE } from "src/modules/Permissions";

const metaValue = (meta: Record<string, any> | undefined, camelKey: string, snakeKey?: string) =>
	meta?.[camelKey] ?? meta?.[snakeKey || camelKey];

interface Props {
	data: WakeDevice[];
	isFiltered?: boolean;
	isFetching?: boolean;
	wakingId?: number | null;
	onWake?: (id: number) => void;
	onEdit?: (id: number) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onNew?: () => void;
}

export default function Table({
	data,
	isFetching,
	isFiltered,
	wakingId,
	onWake,
	onEdit,
	onDelete,
	onDisableToggle,
	onNew,
}: Props) {
	const columnHelper = createColumnHelper<WakeDevice>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: WakeDevice) => row, {
				id: "name",
				header: intl.formatMessage({ id: "name" }),
				sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
				cell: (info) => <span className="font-weight-medium">{info.getValue().name}</span>,
			}),
			columnHelper.accessor((row: WakeDevice) => row.createdOn, {
				id: "createdOn",
				header: "创建时间",
				sortingFn: (a, b) => (a.original.createdOn || "").localeCompare(b.original.createdOn || ""),
				cell: (info) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: WakeDevice) => row.macAddress, {
				id: "macAddress",
				header: intl.formatMessage({ id: "wake-on-lan.mac-address" }),
				cell: (info) => <code>{info.getValue()}</code>,
			}),
			columnHelper.accessor((row: WakeDevice) => row.host, {
				id: "host",
				header: intl.formatMessage({ id: "wake-on-lan.host" }),
				cell: (info) => info.getValue() || <span className="text-muted">-</span>,
			}),
			columnHelper.accessor((row: WakeDevice) => row, {
				id: "target",
				header: intl.formatMessage({ id: "wake-on-lan.target" }),
				enableSorting: false,
				cell: (info) => {
					const item = info.getValue();
					return `${item.broadcastAddress || "255.255.255.255"}:${item.port || 9}`;
				},
			}),
			columnHelper.accessor((row: WakeDevice) => row, {
				id: "status",
				header: intl.formatMessage({ id: "column.status" }),
				enableSorting: false,
				cell: (info) => {
					const item = info.getValue();
					const lastError = metaValue(item.meta, "lastError", "last_error");
					const lastErrorAt = metaValue(item.meta, "lastErrorAt", "last_error_at");
					const lastWokenAt = metaValue(item.meta, "lastWokenAt", "last_woken_at");
					if (!item.enabled) {
						return (
							<span className="badge bg-secondary">
								<T id="wake-on-lan.status-disabled" />
							</span>
						);
					}
					if (lastError) {
						return (
							<>
								<span className="badge bg-warning text-dark" title={lastError}>
									<T id="wake-on-lan.status-error" />
								</span>
								<div
									className="text-danger small mt-1"
									style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
								>
									{lastError}
								</div>
								{lastErrorAt ? (
									<div className="text-muted small">{formatDateTime(lastErrorAt)}</div>
								) : null}
							</>
						);
					}
					if (!lastWokenAt) {
						return (
							<span className="badge bg-secondary-lt">
								<T id="wake-on-lan.status-ready" />
							</span>
						);
					}
					return (
						<div>
							<span className="badge bg-success">
								<T id="wake-on-lan.status-woken" />
							</span>
							<div className="text-muted small mt-1">{formatDateTime(lastWokenAt)}</div>
						</div>
					);
				},
			}),
			columnHelper.display({
				id: "id",
				cell: (info) => {
					const item = info.row.original;
					const isWaking = wakingId === item.id;
					return (
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
										tData={{ object: "wake-on-lan.device" }}
										data={{ id: item.id }}
									/>
								</span>
								<HasPermission section={ADMIN} permission={MANAGE} hideError>
									<a
										className={cn("dropdown-item", (!item.enabled || isWaking) && "disabled")}
										href="#"
										onClick={(e) => {
											e.preventDefault();
											if (!item.enabled || isWaking) return;
											onWake?.(item.id);
										}}
									>
										<IconBolt size={16} />
										<T id={isWaking ? "wake-on-lan.action.waking" : "wake-on-lan.action.wake"} />
									</a>
								</HasPermission>
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onEdit?.(item.id);
									}}
								>
									<IconEdit size={16} />
									<T id="action.edit" />
								</a>
								<HasPermission section={ADMIN} permission={MANAGE} hideError>
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onDisableToggle?.(item.id, !item.enabled);
										}}
									>
										<IconPower size={16} />
										<T id={item.enabled ? "action.disable" : "action.enable"} />
									</a>
									<div className="dropdown-divider" />
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onDelete?.(item.id);
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
				meta: { className: "text-end w-1" },
			}),
		],
		[columnHelper, onEdit, onDisableToggle, onDelete, onWake, wakingId],
	);
	const [sorting, setSorting] = useState<SortingState>([]);
	const tableInstance = useReactTable<WakeDevice>({
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
					object="wake-on-lan.device"
					objects="wake-on-lan.devices"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="green"
					permissionSection={ADMIN}
				/>
			}
		/>
	);
}
