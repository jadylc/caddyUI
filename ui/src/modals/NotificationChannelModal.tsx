import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import Modal from "react-bootstrap/Modal";
import {
	createNotificationChannel,
	type NotificationChannel,
	type NotificationEventSpec,
	updateNotificationChannel,
} from "src/api/backend";
import { Button } from "src/components";
import { showSuccess } from "src/notifications";

export const notificationChannelTypes = [
	{ value: "webhook", label: "Webhook" },
	{ value: "bark", label: "Bark" },
	{ value: "serverchan", label: "Server酱" },
	{ value: "gotify", label: "Gotify" },
	{ value: "telegram", label: "Telegram" },
	{ value: "wecom", label: "企业微信机器人" },
	{ value: "dingtalk", label: "钉钉机器人" },
	{ value: "feishu", label: "飞书机器人" },
];

export const notificationChannelLabel = (type: string) =>
	notificationChannelTypes.find((item) => item.value === type)?.label || type;

const createEmptyChannel = (): NotificationChannel => ({
	name: "",
	type: "webhook",
	url: "",
	method: "POST",
	headers: "",
	bodyTemplate: "",
	proxyUrl: "",
	token: "",
	secret: "",
	chatId: "",
	events: [],
	enabled: true,
});

const needsURL = (type: string) => ["webhook", "wecom", "dingtalk", "feishu", "gotify"].includes(type);
const needsToken = (type: string) => ["bark", "serverchan", "gotify", "telegram"].includes(type);
const needsChatID = (type: string) => type === "telegram";

interface ShowProps {
	channel?: NotificationChannel;
	events: NotificationEventSpec[];
	onSaved?: () => void | Promise<void>;
}

interface Props extends InnerModalProps, ShowProps {}

const showNotificationChannelModal = (props: ShowProps) => {
	EasyModal.show(NotificationChannelModal, props);
};

const initialChannel = (channel?: NotificationChannel): NotificationChannel => ({
	...createEmptyChannel(),
	...(channel || {}),
	events: channel?.events ? [...channel.events] : [],
	enabled: channel?.enabled ?? true,
});

const NotificationChannelModal = EasyModal.create(({ visible, remove, channel, events, onSaved }: Props) => {
	const [draft, setDraft] = useState<NotificationChannel>(() => initialChannel(channel));
	const [saving, setSaving] = useState(false);
	const [errorMsg, setErrorMsg] = useState("");
	const isEdit = !!draft.id;

	const updateDraft = (key: keyof NotificationChannel, value: any) => setDraft((prev) => ({ ...prev, [key]: value }));

	const toggleEvent = (event: string) => {
		const selectedEvents = draft.events || [];
		updateDraft(
			"events",
			selectedEvents.includes(event)
				? selectedEvents.filter((item) => item !== event)
				: [...selectedEvents, event],
		);
	};

	const save = async () => {
		if (saving) return;
		setSaving(true);
		setErrorMsg("");
		try {
			if (draft.id) {
				await updateNotificationChannel(draft);
			} else {
				await createNotificationChannel(draft);
			}
			await onSaved?.();
			showSuccess("推送渠道已保存");
			remove();
		} catch (err: any) {
			setErrorMsg(err.message || "保存推送渠道失败");
		} finally {
			setSaving(false);
		}
	};

	return (
		<Modal show={visible} onHide={saving ? undefined : remove} backdrop="static" keyboard={false} size="lg">
			<form
				onSubmit={(event) => {
					event.preventDefault();
					save();
				}}
			>
				<Modal.Header closeButton={!saving}>
					<Modal.Title>{isEdit ? "编辑推送渠道" : "新增推送渠道"}</Modal.Title>
				</Modal.Header>
				<Modal.Body>
					<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg("")} dismissible>
						{errorMsg}
					</Alert>
					<div className="row g-3">
						<div className="col-md-4">
							<label className="form-label required" htmlFor="notifyName">
								名称
							</label>
							<input
								id="notifyName"
								className="form-control"
								value={draft.name}
								required
								onChange={(event) => updateDraft("name", event.target.value)}
							/>
						</div>
						<div className="col-md-4">
							<label className="form-label required" htmlFor="notifyType">
								渠道
							</label>
							<select
								id="notifyType"
								className="form-control"
								value={draft.type}
								onChange={(event) => updateDraft("type", event.target.value)}
							>
								{notificationChannelTypes.map((item) => (
									<option value={item.value} key={item.value}>
										{item.label}
									</option>
								))}
							</select>
						</div>
						<div className="col-md-4 d-flex align-items-end">
							<label className="form-check mb-2" htmlFor="notifyEnabled">
								<input
									id="notifyEnabled"
									className="form-check-input"
									type="checkbox"
									checked={draft.enabled}
									onChange={(event) => updateDraft("enabled", event.target.checked)}
								/>
								<span className="form-check-label">启用</span>
							</label>
						</div>
						{needsURL(draft.type) ? (
							<div className="col-md-6">
								<label className="form-label required" htmlFor="notifyUrl">
									请求地址
								</label>
								<input
									id="notifyUrl"
									className="form-control"
									value={draft.url || ""}
									required
									onChange={(event) => updateDraft("url", event.target.value)}
								/>
							</div>
						) : null}
						{needsToken(draft.type) ? (
							<div className="col-md-6">
								<label className="form-label required" htmlFor="notifyToken">
									Token / Key
								</label>
								<input
									id="notifyToken"
									className="form-control"
									type="password"
									value={draft.token || ""}
									required
									onChange={(event) => updateDraft("token", event.target.value)}
								/>
							</div>
						) : null}
						{needsChatID(draft.type) ? (
							<div className="col-md-6">
								<label className="form-label required" htmlFor="notifyChatId">
									Chat ID
								</label>
								<input
									id="notifyChatId"
									className="form-control"
									value={draft.chatId || ""}
									required
									onChange={(event) => updateDraft("chatId", event.target.value)}
								/>
							</div>
						) : null}
						<div className="col-md-6">
							<label className="form-label" htmlFor="notifyProxyUrl">
								代理地址
							</label>
							<input
								id="notifyProxyUrl"
								className="form-control"
								value={draft.proxyUrl || ""}
								onChange={(event) => updateDraft("proxyUrl", event.target.value)}
								placeholder="http://127.0.0.1:7890"
							/>
							<small className="text-muted">选填，仅用于该推送渠道请求；支持 http、https、socks5。</small>
						</div>
						{draft.type === "webhook" ? (
							<>
								<div className="col-md-3">
									<label className="form-label" htmlFor="notifyMethod">
										方法
									</label>
									<select
										id="notifyMethod"
										className="form-control"
										value={draft.method || "POST"}
										onChange={(event) => updateDraft("method", event.target.value)}
									>
										<option value="POST">POST</option>
										<option value="PUT">PUT</option>
										<option value="PATCH">PATCH</option>
									</select>
								</div>
								<div className="col-md-9">
									<label className="form-label" htmlFor="notifyHeaders">
										请求头
									</label>
									<input
										id="notifyHeaders"
										className="form-control"
										value={draft.headers || ""}
										onChange={(event) => updateDraft("headers", event.target.value)}
										placeholder="JSON 或每行 Header: Value"
									/>
								</div>
							</>
						) : null}
						<div className="col-12">
							<label className="form-label" htmlFor="notifyTemplate">
								消息模板
							</label>
							<textarea
								id="notifyTemplate"
								className="form-control"
								rows={3}
								value={draft.bodyTemplate || ""}
								onChange={(event) => updateDraft("bodyTemplate", event.target.value)}
								placeholder="{{title}}\n{{message}}\n{{domain}}"
							/>
						</div>
					</div>
					<div className="mt-3">
						<div className="form-label">消息类型</div>
						<div className="row">
							{events.map((event) => (
								<div className="col-md-4" key={event.id}>
									<label className="form-check" title={event.description}>
										<input
											className="form-check-input"
											type="checkbox"
											checked={(draft.events || []).includes(event.id)}
											onChange={() => toggleEvent(event.id)}
										/>
										<span className="form-check-label">{event.name}</span>
									</label>
								</div>
							))}
						</div>
						<small className="text-muted">不勾选任何类型表示接收全部推送。</small>
					</div>
				</Modal.Body>
				<Modal.Footer>
					<Button onClick={remove} disabled={saving}>
						取消
					</Button>
					<Button type="submit" actionType="primary" isLoading={saving} disabled={saving}>
						保存渠道
					</Button>
				</Modal.Footer>
			</form>
		</Modal>
	);
});

export { showNotificationChannelModal };
