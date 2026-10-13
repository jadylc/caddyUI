import { createColumnHelper, getCoreRowModel, getPaginationRowModel, useReactTable } from "@tanstack/react-table";
import { useMemo } from "react";
import type { WafAuditEntry } from "src/api/backend";
import { TableLayout } from "src/components/Table/TableLayout";
import { T } from "src/locale";

interface Props {
	data: WafAuditEntry[];
	isFetching?: boolean;
	onSelectItem?: (entry: WafAuditEntry) => void;
}

const formatTime = (value?: string) => {
	if (!value) return "—";
	return value.includes("T") ? value.replace("T", " ").replace(/\.?\d*Z?$/, "") : value;
};

export default function Table({ data, isFetching, onSelectItem }: Props) {
	const columnHelper = createColumnHelper<WafAuditEntry>();
	const columns = useMemo(
		() => [
			columnHelper.accessor("timestamp", {
				id: "timestamp",
				header: "时间",
				cell: (info: any) => formatTime(info.getValue()),
				size: 180,
			}),
			columnHelper.accessor("host", {
				id: "host",
				header: "主机",
				cell: (info: any) => info.getValue() || "—",
			}),
			columnHelper.accessor("method", {
				id: "method",
				header: "方法",
				cell: (info: any) => info.getValue() || "—",
				size: 80,
			}),
			columnHelper.accessor("uri", {
				id: "uri",
				header: "URI",
				cell: (info: any) => info.getValue() || "—",
			}),
			columnHelper.accessor("severity", {
				id: "severity",
				header: "严重度",
				cell: (info: any) => info.getValue() || "—",
				size: 90,
			}),
			columnHelper.accessor("action", {
				id: "action",
				header: "动作",
				cell: (info: any) => info.getValue() || "—",
				size: 90,
			}),
			columnHelper.accessor("ruleId", {
				id: "ruleId",
				header: "规则 ID",
				cell: (info: any) => info.getValue() || "—",
				size: 110,
			}),
			columnHelper.accessor("message", {
				id: "message",
				header: "消息",
				cell: (info: any) => info.getValue() || "—",
			}),
			columnHelper.display({
				id: "id",
				cell: (info: any) => {
					return (
						<button
							type="button"
							className="btn btn-action btn-sm px-1"
							onClick={(e) => {
								e.preventDefault();
								onSelectItem?.(info.row.original);
							}}
						>
							<T id="action.view-details" />
						</button>
					);
				},
				meta: {
					className: "text-end w-1",
				},
			}),
		],
		[columnHelper, onSelectItem],
	);

	const tableInstance = useReactTable<WafAuditEntry>({
		columns,
		data,
		getCoreRowModel: getCoreRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
		rowCount: data.length,
		meta: {
			isFetching,
		},
		enableSortingRemoval: false,
	});

	return <TableLayout tableInstance={tableInstance} />;
}