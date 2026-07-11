import { IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { deleteHeaderPolicyTemplate, toggleHeaderPolicyTemplate } from "src/api/backend";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useHeaderPolicyTemplates } from "src/hooks";
import { T } from "src/locale";
import { showDeleteConfirmModal, showHeaderPolicyTemplateModal } from "src/modals";
import { MANAGE, PROXY_HOSTS } from "src/modules/Permissions";
import { showObjectSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const { isFetching, isLoading, isError, error, data } = useHeaderPolicyTemplates();

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const handleDelete = async (id: number) => {
		await deleteHeaderPolicyTemplate(id);
		showObjectSuccess("header-policy-template", "deleted");
	};

	const handleDisableToggle = async (id: number, enabled: boolean) => {
		await toggleHeaderPolicyTemplate(id, enabled);
		queryClient.invalidateQueries({ queryKey: ["header-policy-templates"] });
		queryClient.invalidateQueries({ queryKey: ["header-policy-template", id] });
		queryClient.invalidateQueries({ queryKey: ["proxy-hosts"] });
		queryClient.invalidateQueries({ queryKey: ["static-hosts"] });
		showObjectSuccess("header-policy-template", enabled ? "enabled" : "disabled");
	};

	const handleRefresh = () => {
		queryClient.invalidateQueries({ queryKey: ["header-policy-templates"], exact: false });
		queryClient.refetchQueries({ queryKey: ["header-policy-templates"], type: "active" });
	};

	const filtered =
		search && data
			? data.filter((item) =>
					`${item.name} ${item.description || ""} ${item.category || ""} ${(item.headers || [])
						.map((rule) => `${rule.name} ${rule.value}`)
						.join(" ")}`.toLowerCase().includes(search),
				)
			: null;

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-lime" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="header-policy-templates" />
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
								<Button size="sm" onClick={handleRefresh} disabled={isFetching}>
									<IconRefresh size={20} />
								</Button>
								<HasPermission section={PROXY_HOSTS} permission={MANAGE} hideError>
									<Button size="sm" className="btn-lime" onClick={() => showHeaderPolicyTemplateModal("new")}>
										<IconPlus size={18} />
										<T id="object.add" tData={{ object: "header-policy-template" }} />
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
					onEdit={(id: number) => showHeaderPolicyTemplateModal(id)}
					onDelete={(id: number) =>
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "header-policy-template" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [
								["header-policy-templates"],
								["header-policy-template", id],
								["proxy-hosts"],
								["static-hosts"],
							],
							children: <T id="object.delete.content" tData={{ object: "header-policy-template" }} />,
						})
					}
					onDisableToggle={handleDisableToggle}
					onNew={() => showHeaderPolicyTemplateModal("new")}
				/>
			</div>
		</div>
	);
}
