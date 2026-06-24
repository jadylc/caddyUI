import { IconHelp, IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { checkDomainMonitor, deleteDomainMonitor, renewDomainMonitor, toggleDomainMonitor } from "src/api/backend";
import type { CredentialSummary, DomainMonitor } from "src/api/backend/models";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useCredentials, useDomainMonitors } from "src/hooks";
import { T } from "src/locale";
import { showDeleteConfirmModal, showDomainMonitorModal, showHelpModal } from "src/modals";
import { CERTIFICATES, MANAGE } from "src/modules/Permissions";
import { showError, showObjectSuccess, showSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const { isFetching, isLoading, isError, error, data } = useDomainMonitors();
	const { data: credentials } = useCredentials();

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const invalidate = (id?: number) => {
		queryClient.invalidateQueries({ queryKey: ["domain-monitor"] });
		if (id) {
			queryClient.invalidateQueries({ queryKey: ["domain-monitor", id] });
		}
	};

	const handleDelete = async (id: number) => {
		await deleteDomainMonitor(id);
		showObjectSuccess("domain-monitor", "deleted");
	};

	const handleDisableToggle = async (id: number, enabled: boolean) => {
		await toggleDomainMonitor(id, enabled);
		invalidate(id);
		showObjectSuccess("domain-monitor", enabled ? "enabled" : "disabled");
	};

	const handleCheck = async (id: number) => {
		try {
			await checkDomainMonitor(id);
			invalidate(id);
			showSuccess(<T id="domain-monitor.notification.checked" />);
		} catch (err: any) {
			showError(err.message);
		}
	};

	const handleRenew = async (id: number) => {
		try {
			await renewDomainMonitor(id);
			invalidate(id);
			showSuccess("续期已完成");
		} catch (err: any) {
			showError(err.message);
		}
	};

	const handleRefresh = () => {
		queryClient.invalidateQueries({ queryKey: ["domain-monitor"] });
		queryClient.refetchQueries({ queryKey: ["domain-monitor"], type: "active" });
	};

	const filtered =
		search && data
			? data.filter((item) => `${item.name} ${item.domainNames.join(" ")}`.toLowerCase().includes(search))
			: null;
	const dnsheCredentialIds = new Set(
		(credentials || [])
			.filter((item: CredentialSummary) => item.provider === "dnshe")
			.map((item: CredentialSummary) => item.id),
	);
	const canRenew = (item: DomainMonitor) => !!item.credentialId && dnsheCredentialIds.has(item.credentialId);

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-azure" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="domain-monitor" />
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
									onClick={() => showHelpModal("DomainMonitor", "azure")}
									title="功能说明"
									ariaLabel="功能说明"
								>
									<IconHelp size={20} />
								</Button>
								<Button size="sm" onClick={handleRefresh} disabled={isFetching}>
									<IconRefresh size={20} />
								</Button>
								<HasPermission section={CERTIFICATES} permission={MANAGE} hideError>
									<Button
										size="sm"
										className="btn-azure"
										onClick={() => showDomainMonitorModal("new")}
									>
										<IconPlus size={18} />
										<T id="object.add" tData={{ object: "domain-monitor" }} />
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
					onCheck={handleCheck}
					onRenew={handleRenew}
					canRenew={canRenew}
					onEdit={(id: number) => showDomainMonitorModal(id)}
					onDelete={(id: number) =>
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "domain-monitor" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [["domain-monitor"], ["domain-monitor", id]],
							children: <T id="object.delete.content" tData={{ object: "domain-monitor" }} />,
						})
					}
					onDisableToggle={handleDisableToggle}
					onNew={() => showDomainMonitorModal("new")}
				/>
			</div>
		</div>
	);
}
