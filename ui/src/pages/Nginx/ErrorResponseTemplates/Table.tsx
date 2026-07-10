import { IconDotsVertical, IconEdit, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getPaginationRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { ErrorResponseRule, ErrorResponseTemplate } from "src/api/backend";
import { CreatedOnFormatter, EmptyData, HasPermission } from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { MANAGE, PROXY_HOSTS } from "src/modules/Permissions";

interface Props {
	data: ErrorResponseTemplate[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onEdit?: (id: number) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onNew?: () => void;
}

const statusLabel = (statuses: string[]) => {
	const value = statuses?.[0] || "default";
	if (value === "default") return "默认";
	return value.toUpperCase();
};

const actionLabel = (rule: ErrorResponseRule) => {
	switch (rule.action) {
		case "redirect":
			return "跳转";
		case "reverse_proxy":
			return "反代";
		case "respond":
			return "响应";
		default:
			return "静态文件";
	}
};

const ruleSummary = (rules: ErrorResponseRule[]) =>
	(rules || []).map((rule) => `${statusLabel(rule.statuses)} -> ${actionLabel(rule)}`).join("，");

export default function Table({ data, isFetching, isFiltered, onEdit, onDelete, onDisableToggle, onNew }: Props) {
	const columnHelper = createColumnHelper<ErrorResponseTemplate>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: ErrorResponseTemplate) => row, {
				id: "name",
				header: intl.formatMessage({ id: "name" }),
				sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
				cell: (info) => (
					<div>
						<span className="font-weight-medium">{info.getValue().name}</span>
						{info.getValue().description ? (
							<div className="text-muted small">{info.getValue().description}</div>
						) : null}
					</div>
				),
			}),
			columnHelper.accessor((row: ErrorResponseTemplate) => row.rules, {
				id: "rules",
				header: "规则",
				enableSorting: false,
				cell: (info) => <span className="text-muted">{ruleSummary(info.getValue()) || "-"}</span>,
			}),
			columnHelper.accessor((row: ErrorResponseTemplate) => row.staticRoot || "", {
				id: "staticRoot",
				header: "错误页目录",
				sortingFn: (a, b) => (a.original.staticRoot || "").localeCompare(b.original.staticRoot || ""),
				cell: (info) => <code className="text-muted">{info.getValue() || "-"}</code>,
			}),
			columnHelper.accessor((row: ErrorResponseTemplate) => row.hostCount || 0, {
				id: "hostCount",
				header: "引用站点",
				cell: (info) => <span className="badge bg-secondary-lt">{info.getValue()}</span>,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: ErrorResponseTemplate) => row.enabled, {
				id: "enabled",
				header: intl.formatMessage({ id: "column.status" }),
				cell: (info) => (
					<span className={`badge ${info.getValue() ? "bg-success" : "bg-secondary"}`}>
						<T id={info.getValue() ? "status.enabled" : "status.disabled"} />
					</span>
				),
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: ErrorResponseTemplate) => row.modifiedOn || row.createdOn, {
				id: "modifiedOn",
				header: "修改时间",
				sortingFn: (a, b) =>
					(a.original.modifiedOn || a.original.createdOn || "").localeCompare(
						b.original.modifiedOn || b.original.createdOn || "",
					),
				cell: (info) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
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
									tData={{ object: "error-response-template" }}
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
		[columnHelper, onEdit, onDisableToggle, onDelete],
	);
	const [sorting, setSorting] = useState<SortingState>([{ id: "modifiedOn", desc: true }]);
	const tableInstance = useReactTable<ErrorResponseTemplate>({
		columns,
		data,
		state: { sorting },
		onSortingChange: setSorting,
		getCoreRowModel: getCoreRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
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
					object="error-response-template"
					objects="error-response-templates"
					tableInstance={tableInstance}
					onNew={onNew}
					isFiltered={isFiltered}
					color="orange"
					permissionSection={PROXY_HOSTS}
				/>
			}
		/>
	);
}
