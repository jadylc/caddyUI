import { IconHelp, IconPlus } from "@tabler/icons-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "react-bootstrap/Alert";
import { deleteNotificationChannel, getNotificationChannels, testNotificationChannel } from "src/api/backend";
import { Button, LoadingPage } from "src/components";
import { notificationChannelLabel, showHelpModal, showNotificationChannelModal } from "src/modals";
import { showError, showSuccess } from "src/notifications";

export default function NotificationSettingsPage() {
	const queryClient = useQueryClient();
	const { data, isLoading, isError, error } = useQuery({
		queryKey: ["notification-channels"],
		queryFn: getNotificationChannels,
	});

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const refresh = () => queryClient.invalidateQueries({ queryKey: ["notification-channels"] });
	const events = data?.events || [];

	const remove = async (id?: number) => {
		if (!id) return;
		try {
			await deleteNotificationChannel(id);
			await refresh();
			showSuccess("推送渠道已删除");
		} catch (err: any) {
			showError(err.message);
		}
	};

	const test = async (id?: number) => {
		if (!id) return;
		try {
			await testNotificationChannel(id);
			await refresh();
			showSuccess("测试推送已发送");
		} catch (err: any) {
			showError(err.message);
		}
	};

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-azure" />
			<div className="card-header">
				<div className="row w-full align-items-center">
					<div className="col">
						<h2 className="mb-0">推送通知</h2>
					</div>
					<div className="col-auto">
						<div className="ms-auto d-flex flex-wrap btn-list">
							<Button
								size="sm"
								onClick={() => showHelpModal("Notifications", "azure")}
								title="功能说明"
								ariaLabel="功能说明"
							>
								<IconHelp size={20} />
							</Button>
							<Button
								size="sm"
								className="btn-azure"
								onClick={() => showNotificationChannelModal({ events, onSaved: refresh })}
							>
								<IconPlus size={18} />
								新增渠道
							</Button>
						</div>
					</div>
				</div>
			</div>
			<div className="table-responsive">
				<table className="table card-table table-vcenter">
					<thead>
						<tr>
							<th>名称</th>
							<th>渠道</th>
							<th>消息类型</th>
							<th>状态</th>
							<th>最近发送</th>
							<th className="w-1" />
						</tr>
					</thead>
					<tbody>
						{(data?.channels || []).map((item) => (
							<tr key={item.id}>
								<td>
									<div>{item.name}</div>
									{item.lastError ? <div className="text-danger small">{item.lastError}</div> : null}
								</td>
								<td>
									<div>{notificationChannelLabel(item.type)}</div>
									{item.proxyUrl ? <div className="text-muted small">代理已启用</div> : null}
								</td>
								<td>
									{item.events?.length
										? item.events
												.map((event) => events.find((spec) => spec.id === event)?.name || event)
												.join("、")
										: "全部"}
								</td>
								<td>{item.enabled ? "启用" : "停用"}</td>
								<td>{item.lastSentAt || "-"}</td>
								<td>
									<div className="btn-list flex-nowrap">
										<Button
											size="sm"
											onClick={() =>
												showNotificationChannelModal({
													channel: item,
													events,
													onSaved: refresh,
												})
											}
										>
											编辑
										</Button>
										<Button size="sm" onClick={() => test(item.id)}>
											测试
										</Button>
										<Button size="sm" className="btn-red" onClick={() => remove(item.id)}>
											删除
										</Button>
									</div>
								</td>
							</tr>
						))}
						{!data?.channels?.length ? (
							<tr>
								<td colSpan={6} className="text-muted text-center py-4">
									暂无推送渠道
								</td>
							</tr>
						) : null}
					</tbody>
				</table>
			</div>
		</div>
	);
}
