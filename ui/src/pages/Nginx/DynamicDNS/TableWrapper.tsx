import { IconHelp, IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { checkDynamicDNSItem, deleteDynamicDNSItem, toggleDynamicDNSItem } from "src/api/backend";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useDynamicDNSItems } from "src/hooks";
import { T } from "src/locale";
import { showDeleteConfirmModal, showDynamicDNSModal, showHelpModal } from "src/modals";
import { MANAGE, STREAMS } from "src/modules/Permissions";
import { showObjectSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const { isFetching, isLoading, isError, error, data } = useDynamicDNSItems();

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const handleDelete = async (id: number) => {
		await deleteDynamicDNSItem(id);
		showObjectSuccess("dynamic-dns", "deleted");
	};

	const handleDisableToggle = async (id: number, enabled: boolean) => {
		await toggleDynamicDNSItem(id, enabled);
		queryClient.invalidateQueries({ queryKey: ["dynamic-dns"] });
		queryClient.invalidateQueries({ queryKey: ["dynamic-dns", id] });
		showObjectSuccess("dynamic-dns", enabled ? "enabled" : "disabled");
	};

	const handleRefresh = () => {
		queryClient.invalidateQueries({ queryKey: ["dynamic-dns"], exact: false });
		queryClient.refetchQueries({ queryKey: ["dynamic-dns"], type: "active" });
	};

	const handleCheck = async (id: number) => {
		try {
			await checkDynamicDNSItem(id);
			showObjectSuccess("dynamic-dns", "saved");
		} catch {
			// The backend persists the failed check result before returning 400;
			// refresh the table so the row shows the concrete reason.
		} finally {
			queryClient.invalidateQueries({ queryKey: ["dynamic-dns"] });
			queryClient.invalidateQueries({ queryKey: ["dynamic-dns", id] });
		}
	};

	const filtered =
		search && data
			? data.filter((item) =>
					`${item.name} ${item.domainNames.join(" ")} ${item.dnsProvider || ""}`
						.toLowerCase()
						.includes(search),
				)
			: null;

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-cyan" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="dynamic-dns" />
							</h2>
						</div>
						<div className="col-md-auto col-sm-12">
							<div className="ms-auto d-flex flex-wrap btn-list">
								{data?.length ? (
									<div className="input-group input-group-flat w-auto">
										<span className="input-group-text input-group-text-sm">
											<IconSearch size={16} />
										</span>
										<input
											type="text"
											className="form-control form-control-sm"
											autoComplete="off"
											onChange={(e: any) => setSearch(e.target.value.toLowerCase().trim())}
										/>
									</div>
								) : null}
								<Button
									size="sm"
									onClick={() => showHelpModal("DynamicDNS", "cyan")}
									title="功能说明"
									ariaLabel="功能说明"
								>
									<IconHelp size={20} />
								</Button>
								<Button size="sm" onClick={handleRefresh} disabled={isFetching}>
									<IconRefresh size={20} />
								</Button>
								<HasPermission section={STREAMS} permission={MANAGE} hideError>
									<Button size="sm" className="btn-cyan" onClick={() => showDynamicDNSModal("new")}>
										<IconPlus size={18} />
										<T id="object.add" tData={{ object: "dynamic-dns" }} />
									</Button>
								</HasPermission>
							</div>
						</div>
					</div>
				</div>
				<Table
					data={filtered ?? data ?? []}
					isFetching={isFetching}
					isFiltered={!!filtered}
					onEdit={(id: number) => showDynamicDNSModal(id)}
					onDelete={(id: number) =>
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "dynamic-dns" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [["dynamic-dns"], ["dynamic-dns", id]],
							children: <T id="object.delete.content" tData={{ object: "dynamic-dns" }} />,
						})
					}
					onDisableToggle={handleDisableToggle}
					onCheck={handleCheck}
					onNew={() => showDynamicDNSModal("new")}
				/>
			</div>
		</div>
	);
}
