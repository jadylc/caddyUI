import { IconHelp, IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { deleteStaticHost, toggleStaticHost } from "src/api/backend";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useStaticHosts } from "src/hooks";
import { T } from "src/locale";
import { showDeleteConfirmModal, showHelpModal, showStaticHostModal } from "src/modals";
import { MANAGE, STATIC_HOSTS } from "src/modules/Permissions";
import { showObjectSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const { isFetching, isLoading, isError, error, data } = useStaticHosts(["access_list", "certificate"]);

	if (isLoading) {
		return <LoadingPage />;
	}

	if (isError) {
		return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;
	}

	const handleDelete = async (id: number) => {
		await deleteStaticHost(id);
		showObjectSuccess("static-host", "deleted");
	};

	const handleDisableToggle = async (id: number, enabled: boolean) => {
		await toggleStaticHost(id, enabled);
		queryClient.invalidateQueries({ queryKey: ["static-hosts"] });
		queryClient.invalidateQueries({ queryKey: ["static-host", id] });
		queryClient.invalidateQueries({ queryKey: ["host-report"] });
		showObjectSuccess("static-host", enabled ? "enabled" : "disabled");
	};

	const handleRefresh = () => {
		queryClient.invalidateQueries({ queryKey: ["static-hosts"], exact: false });
		queryClient.refetchQueries({ queryKey: ["static-hosts"], type: "active" });
	};

	let filtered = null;
	if (search && data) {
		filtered = data?.filter(
			(item) =>
				item.domainNames.some((domain: string) => domain.toLowerCase().includes(search)) ||
				(item.serviceName || "").toLowerCase().includes(search) ||
				(item.rootPath || "").toLowerCase().includes(search) ||
				(item.mode || "").toLowerCase().includes(search),
		);
	}

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-cyan" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="static-hosts" />
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
											id="static-host-search"
											type="text"
											className="form-control form-control-sm"
											autoComplete="off"
											onChange={(e: any) => setSearch(e.target.value.toLowerCase().trim())}
										/>
									</div>
								) : null}
								<Button
									size="sm"
									onClick={() => showHelpModal("StaticHosts", "cyan")}
									title="功能说明"
									ariaLabel="功能说明"
								>
									<IconHelp size={20} />
								</Button>
								<Button size="sm" onClick={handleRefresh} disabled={isFetching}>
									<IconRefresh size={20} />
								</Button>
								<HasPermission section={STATIC_HOSTS} permission={MANAGE} hideError>
									<Button size="sm" className="btn-cyan" onClick={() => showStaticHostModal("new")}>
										<IconPlus size={18} />
										<T id="object.add" tData={{ object: "static-host" }} />
									</Button>
								</HasPermission>
							</div>
						</div>
					</div>
				</div>
				<Table
					data={filtered ?? data ?? []}
					isFiltered={!!search}
					isFetching={isFetching}
					onEdit={(id: number) => showStaticHostModal(id)}
					onCopy={(host) => {
						const {
							id: _id,
							createdOn: _createdOn,
							modifiedOn: _modifiedOn,
							owner: _owner,
							accessList: _accessList,
							certificate: _certificate,
							...copy
						} = host;
						showStaticHostModal("new", copy);
					}}
					onDelete={(id: number) => {
						const host = data?.find((h) => h.id === id);
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "static-host" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [["static-hosts"], ["static-host", id]],
							children: (
								<>
									<T id="object.delete.content" tData={{ object: "static-host" }} />
									{host?.domainNames?.length ? (
										<div className="mt-2 fw-bold text-break">{host.domainNames.join(", ")}</div>
									) : null}
									{host?.rootPath ? (
										<div className="mt-1 text-muted small">
											<code>{host.rootPath}</code>
										</div>
									) : null}
								</>
							),
						});
					}}
					onDisableToggle={handleDisableToggle}
					onNew={() => showStaticHostModal("new")}
				/>
			</div>
		</div>
	);
}
