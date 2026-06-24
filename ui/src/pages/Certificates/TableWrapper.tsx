import { IconHelp, IconPlus, IconRefresh, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { deleteCertificate, downloadCertificate } from "src/api/backend";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useCertificates } from "src/hooks";
import { T } from "src/locale";
import {
	showCustomCertificateModal,
	showDeleteConfirmModal,
	showDNSCertificateModal,
	showHelpModal,
	showHTTPCertificateModal,
	showRenewCertificateModal,
} from "src/modals";
import { CERTIFICATES, MANAGE } from "src/modules/Permissions";
import { showError, showObjectSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const { isFetching, isLoading, isError, error, data } = useCertificates([
		"dead_hosts",
		"proxy_hosts",
		"redirection_hosts",
		"streams",
	]);

	if (isLoading) {
		return <LoadingPage />;
	}

	if (isError) {
		return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;
	}

	const handleDelete = async (id: number) => {
		await deleteCertificate(id);
		showObjectSuccess("certificate", "deleted");
	};

	const handleDownload = async (id: number) => {
		try {
			await downloadCertificate(id);
		} catch (err: any) {
			showError(err.message);
		}
	};

	const handleEdit = (certificate: any) => {
		if ((certificate?.meta?.signMethod || certificate?.meta?.sign_method) === "DNS-01") {
			showDNSCertificateModal(certificate);
			return;
		}
		showHTTPCertificateModal(certificate);
	};
	const handleRefresh = () => {
		queryClient.invalidateQueries({ queryKey: ["certificates"], exact: false });
		queryClient.refetchQueries({ queryKey: ["certificates"], type: "active" });
	};

	let filtered = null;
	if (search && data) {
		filtered = data?.filter(
			(item) =>
				item.domainNames.some((domain: string) => domain.toLowerCase().includes(search)) ||
				item.niceName.toLowerCase().includes(search),
		);
	}

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-pink" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="certificates" />
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
											id="advanced-table-search"
											type="text"
											className="form-control form-control-sm"
											autoComplete="off"
											onChange={(e: any) => setSearch(e.target.value.toLowerCase().trim())}
										/>
									</div>
								) : null}
								<Button
									size="sm"
									onClick={() => showHelpModal("Certificates", "pink")}
									title="功能说明"
									ariaLabel="功能说明"
								>
									<IconHelp size={20} />
								</Button>
								<Button size="sm" onClick={handleRefresh} disabled={isFetching}>
									<IconRefresh size={20} />
								</Button>
								<HasPermission section={CERTIFICATES} permission={MANAGE} hideError>
									<div className="dropdown">
										<button
											type="button"
											className="btn btn-sm dropdown-toggle btn-pink"
											data-bs-toggle="dropdown"
										>
											<IconPlus size={18} />
											<T id="object.add" tData={{ object: "certificate" }} />
										</button>
										<div className="dropdown-menu">
											<a
												className="dropdown-item"
												href="#"
												onClick={(e) => {
													e.preventDefault();
													showHTTPCertificateModal();
												}}
											>
												<T id="lets-encrypt-via-http" />
											</a>
											<a
												className="dropdown-item"
												href="#"
												onClick={(e) => {
													e.preventDefault();
													showDNSCertificateModal();
												}}
											>
												<T id="lets-encrypt-via-dns" />
											</a>
											<div className="dropdown-divider" />
											<a
												className="dropdown-item"
												href="#"
												onClick={(e) => {
													e.preventDefault();
													showCustomCertificateModal();
												}}
											>
												<T id="certificates.custom" />
											</a>
										</div>
									</div>
								</HasPermission>
							</div>
						</div>
					</div>
				</div>
				<Table
					data={filtered ?? data ?? []}
					isFiltered={!!search}
					isFetching={isFetching}
					onEdit={handleEdit}
					onRenew={showRenewCertificateModal}
					onDownload={handleDownload}
					onDelete={(id: number) =>
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "certificate" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [["certificates"], ["certificate", id]],
							children: <T id="object.delete.content" tData={{ object: "certificate" }} />,
						})
					}
				/>
			</div>
		</div>
	);
}
