import { IconDotsVertical, IconEdit, IconEye, IconPower, IconTrash } from "@tabler/icons-react";
import {
	createColumnHelper,
	getCoreRowModel,
	getPaginationRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useMemo, useState } from "react";
import type { HeaderPolicyTemplate, ProxyHeaderRule } from "src/api/backend";
import { CreatedOnFormatter, EmptyData, HasPermission } from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { MANAGE, PROXY_HOSTS } from "src/modules/Permissions";

interface Props {
	data: HeaderPolicyTemplate[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onEdit?: (id: number) => void;
	onDelete?: (id: number) => void;
	onDisableToggle?: (id: number, enabled: boolean) => void;
	onNew?: () => void;
}

const categoryLabel = (category?: string) => {
	switch (category) {
		case "api":
			return "API";
		case "static":
			return "静态站";
		case "download":
			return "下载站";
		case "security":
			return "安全";
		case "cors":
			return "CORS";
		default:
			return "自定义";
	}
};

const headerSummary = (headers?: ProxyHeaderRule[]) =>
	(headers || [])
		.slice(0, 4)
		.map((rule) => (rule.action === "delete" ? `删除 ${rule.name}` : `${rule.name}: ${rule.value}`))
		.join("，");

export default function Table({ data, isFetching, isFiltered, onEdit, onDelete, onDisableToggle, onNew }: Props) {
	const columnHelper = createColumnHelper<HeaderPolicyTemplate>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row) => row, {
				id: "name",
				header: intl.formatMessage({ id: "name" }),
				sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
				cell: (info) => (
					<div>
						<div className="d-flex align-items-center gap-2">
							<span className="font-weight-medium">{info.getValue().name}</span>
							{info.getValue().builtin ? <span className="badge bg-blue-lt">内置</span> : null}
						</div>
						{info.getValue().description ? (
							<div className="text-muted small">{info.getValue().description}</div>
						) : null}
					</div>
				),
			}),
			columnHelper.accessor((row) => row.category || "custom", {
				id: "category",
				header: "分类",
				cell: (info) => <span className="badge bg-secondary-lt">{categoryLabel(info.getValue())}</span>,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row) => row.headers || [], {
				id: "headers",
				header: "Header",
				enableSorting: false,
				cell: (info) => (
					<span className="text-muted">
						{headerSummary(info.getValue()) || "-"}
						{info.getValue().length > 4 ? `，+${info.getValue().length - 4}` : ""}
					</span>
				),
			}),
			columnHelper.accessor((row) => row.hostCount || 0, {
				id: "hostCount",
				header: "引用站点",
				cell: (info) => <span className="badge bg-secondary-lt">{info.getValue()}</span>,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row) => row.enabled, {
				id: "enabled",
				header: intl.formatMessage({ id: "column.status" }),
				cell: (info) => (
					<span className={`badge ${info.getValue() ? "bg-success" : "bg-secondary"}`}>
						<T id={info.getValue() ? "status.enabled" : "status.disabled"} />
					</span>
				),
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row) => row.modifiedOn || row.createdOn, {
				id: "modifiedOn",
				header: "修改时间",
				sortingFn: (a, b) =>
					(a.original.modifiedOn || a.original.createdOn || "").localeCompare(
						b.original.modifiedOn || b.original.createdOn || "",
					),
				cell: (info) => <CreatedOnFormatter value={info.getValue() || ""} />,
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
									tData={{ object: "header-policy-template" }}
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
								{info.row.original.builtin ? <IconEye size={16} /> : <IconEdit size={16} />}
								<T id={info.row.original.builtin ? "action.view" : "action.edit"} />
							</a>
							{info.row.original.builtin ? null : (
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
							)}
						</div>
					</span>
				),
				meta: { className: "text-end w-1" },
			}),
		],
		[columnHelper, onEdit, onDisableToggle, onDelete],
	);
	const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }]);
	const tableInstance = useReactTable<HeaderPolicyTemplate>({
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
					object="header-policy-template"
					objects="header-policy-templates"
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
