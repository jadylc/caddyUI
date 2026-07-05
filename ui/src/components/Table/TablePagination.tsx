import { IconChevronLeft, IconChevronRight, IconChevronsLeft, IconChevronsRight } from "@tabler/icons-react";
import type { Table as ReactTable } from "@tanstack/react-table";
import { useEffect, useMemo, useState } from "react";

const pageSizeOptions = [10, 25, 50, 100];

interface PaginationControlsProps {
	pageIndex: number;
	pageSize: number;
	total: number;
	onPageChange: (pageIndex: number) => void;
	onPageSizeChange: (pageSize: number) => void;
}

function clampPageIndex(pageIndex: number, pageCount: number) {
	return Math.max(0, Math.min(pageIndex, Math.max(0, pageCount - 1)));
}

function PaginationControls({ pageIndex, pageSize, total, onPageChange, onPageSizeChange }: PaginationControlsProps) {
	const pageCount = Math.max(1, Math.ceil(total / pageSize));
	const safePageIndex = clampPageIndex(pageIndex, pageCount);
	const start = total === 0 ? 0 : safePageIndex * pageSize + 1;
	const end = Math.min(total, (safePageIndex + 1) * pageSize);
	const canPrevious = safePageIndex > 0;
	const canNext = safePageIndex < pageCount - 1;

	const setPage = (nextPage: number) => {
		onPageChange(clampPageIndex(nextPage, pageCount));
	};

	return (
		<div className="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-2 px-3 py-2 border-top">
			<div className="text-secondary small">
				显示 {start}-{end} / 共 {total} 条
			</div>
			<div className="d-flex flex-wrap align-items-center gap-2">
				<label className="d-flex align-items-center gap-2 text-secondary small mb-0">
					每页
					<select
						className="form-select form-select-sm w-auto"
						value={pageSize}
						onChange={(event) => onPageSizeChange(Number(event.target.value))}
					>
						{pageSizeOptions.map((option) => (
							<option value={option} key={option}>
								{option}
							</option>
						))}
					</select>
				</label>
				<div className="btn-list flex-nowrap">
					<button
						type="button"
						className="btn btn-sm btn-icon"
						onClick={() => setPage(0)}
						disabled={!canPrevious}
						aria-label="第一页"
						title="第一页"
					>
						<IconChevronsLeft size={16} />
					</button>
					<button
						type="button"
						className="btn btn-sm btn-icon"
						onClick={() => setPage(safePageIndex - 1)}
						disabled={!canPrevious}
						aria-label="上一页"
						title="上一页"
					>
						<IconChevronLeft size={16} />
					</button>
					<span className="d-inline-flex align-items-center px-2 text-secondary small text-nowrap">
						第 {safePageIndex + 1} / {pageCount} 页
					</span>
					<button
						type="button"
						className="btn btn-sm btn-icon"
						onClick={() => setPage(safePageIndex + 1)}
						disabled={!canNext}
						aria-label="下一页"
						title="下一页"
					>
						<IconChevronRight size={16} />
					</button>
					<button
						type="button"
						className="btn btn-sm btn-icon"
						onClick={() => setPage(pageCount - 1)}
						disabled={!canNext}
						aria-label="最后一页"
						title="最后一页"
					>
						<IconChevronsRight size={16} />
					</button>
				</div>
			</div>
		</div>
	);
}

interface TablePaginationProps<TFields> {
	tableInstance: ReactTable<TFields>;
}

function TablePagination<TFields>({ tableInstance }: TablePaginationProps<TFields>) {
	const { pageIndex, pageSize } = tableInstance.getState().pagination;
	const total = tableInstance.getPrePaginationRowModel().rows.length;

	useEffect(() => {
		const pageCount = Math.max(1, Math.ceil(total / pageSize));
		if (pageIndex >= pageCount) {
			tableInstance.setPageIndex(pageCount - 1);
		}
	}, [pageIndex, pageSize, tableInstance, total]);

	return (
		<PaginationControls
			pageIndex={pageIndex}
			pageSize={pageSize}
			total={total}
			onPageChange={(nextPage) => tableInstance.setPageIndex(nextPage)}
			onPageSizeChange={(nextPageSize) => tableInstance.setPageSize(nextPageSize)}
		/>
	);
}

function useLocalPagination<T>(items: T[] = [], initialPageSize = 10) {
	const [pageIndex, setPageIndex] = useState(0);
	const [pageSize, setPageSizeState] = useState(initialPageSize);
	const total = items.length;
	const pageCount = Math.max(1, Math.ceil(total / pageSize));

	useEffect(() => {
		setPageIndex((current) => clampPageIndex(current, pageCount));
	}, [pageCount]);

	const setPageSize = (nextPageSize: number) => {
		setPageSizeState(nextPageSize);
		setPageIndex(0);
	};

	const pageItems = useMemo(() => {
		const start = pageIndex * pageSize;
		return items.slice(start, start + pageSize);
	}, [items, pageIndex, pageSize]);

	const pagination = (
		<PaginationControls
			pageIndex={pageIndex}
			pageSize={pageSize}
			total={total}
			onPageChange={setPageIndex}
			onPageSizeChange={setPageSize}
		/>
	);

	return { pageItems, pagination, pageIndex, pageSize, total };
}

export { PaginationControls, TablePagination, useLocalPagination };
