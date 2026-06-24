import type { Table as ReactTable } from "@tanstack/react-table";
import type { ReactNode } from "react";
import { T } from "src/locale";
import type { ADMIN, Permission, Section } from "src/modules/Permissions";

interface Props {
	tableInstance: ReactTable<any>;
	onNew?: () => void;
	isFiltered?: boolean;
	object: string;
	objects: string;
	color?: string;
	customAddBtn?: ReactNode;
	permissionSection?: Section | typeof ADMIN;
	permission?: Permission;
}
function EmptyData({ tableInstance, isFiltered, objects }: Props) {
	return (
		<tr>
			<td colSpan={tableInstance.getVisibleFlatColumns().length}>
				<div className="empty text-center my-2">
					{isFiltered ? (
						<h2 className="empty-title">
							<T id="empty-search" />
						</h2>
					) : (
						<>
							<h2 className="empty-title">
								<T id="object.empty" tData={{ objects }} />
							</h2>
							<p className="text-muted mb-0">
								<T id="empty-subtitle" />
							</p>
						</>
					)}
				</div>
			</td>
		</tr>
	);
}

export { EmptyData };
