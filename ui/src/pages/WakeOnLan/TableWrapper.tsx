import { IconHelp, IconPlus, IconSearch } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import { deleteWakeDevice, toggleWakeDevice, wakeDevice } from "src/api/backend";
import { Button, HasPermission, LoadingPage } from "src/components";
import { useWakeDevices } from "src/hooks";
import { T } from "src/locale";
import { showDeleteConfirmModal, showHelpModal, showWakeDeviceModal } from "src/modals";
import { ADMIN, MANAGE } from "src/modules/Permissions";
import { showError, showObjectSuccess, showSuccess } from "src/notifications";
import Table from "./Table";

export default function TableWrapper() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const [wakingId, setWakingId] = useState<number | null>(null);
	const { isFetching, isLoading, isError, error, data } = useWakeDevices();

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const invalidateWakeDevices = async (id?: number) => {
		await queryClient.invalidateQueries({ queryKey: ["wake-devices"] });
		if (id) {
			await queryClient.invalidateQueries({ queryKey: ["wake-device", id] });
		}
		await queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
	};

	const handleDelete = async (id: number) => {
		await deleteWakeDevice(id);
		showObjectSuccess("wake-on-lan.device", "deleted");
	};

	const handleDisableToggle = async (id: number, enabled: boolean) => {
		await toggleWakeDevice(id, enabled);
		await invalidateWakeDevices(id);
		showObjectSuccess("wake-on-lan.device", enabled ? "enabled" : "disabled");
	};

	const handleWake = async (id: number) => {
		if (wakingId) return;
		setWakingId(id);
		try {
			await wakeDevice(id);
			showSuccess(<T id="wake-on-lan.notification.woken" />);
		} catch (err: any) {
			showError(err.message || "Unknown error");
		} finally {
			setWakingId(null);
			await invalidateWakeDevices(id);
		}
	};

	const filtered =
		search && data
			? data.filter((item) =>
					`${item.name} ${item.macAddress} ${item.host || ""} ${item.description || ""}`
						.toLowerCase()
						.includes(search),
				)
			: null;

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-green" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="wake-on-lan" />
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
									onClick={() => showHelpModal("WakeOnLan", "green")}
									title="功能说明"
									ariaLabel="功能说明"
								>
									<IconHelp size={20} />
								</Button>
								<HasPermission section={ADMIN} permission={MANAGE} hideError>
									<Button size="sm" className="btn-green" onClick={() => showWakeDeviceModal("new")}>
										<IconPlus size={18} />
										<T id="object.add" tData={{ object: "wake-on-lan.device" }} />
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
					wakingId={wakingId}
					onWake={handleWake}
					onEdit={(id: number) => showWakeDeviceModal(id)}
					onDelete={(id: number) =>
						showDeleteConfirmModal({
							title: <T id="object.delete" tData={{ object: "wake-on-lan.device" }} />,
							onConfirm: () => handleDelete(id),
							invalidations: [["wake-devices"], ["wake-device", id]],
							children: <T id="object.delete.content" tData={{ object: "wake-on-lan.device" }} />,
						})
					}
					onDisableToggle={handleDisableToggle}
					onNew={() => showWakeDeviceModal("new")}
				/>
			</div>
		</div>
	);
}
