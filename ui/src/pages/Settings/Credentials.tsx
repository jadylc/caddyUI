import { IconEye, IconEyeOff, IconHelp, IconPlus } from "@tabler/icons-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import Modal from "react-bootstrap/Modal";
import {
	type CredentialDetail,
	type CredentialSummary,
	createCredential,
	deleteCredential,
	getCredential,
	getCredentials,
	updateCredential,
} from "src/api/backend";
import { Button, LoadingPage } from "src/components";
import { showHelpModal } from "src/modals";
import { showError, showSuccess } from "src/notifications";

const emptyCredential: CredentialDetail = {
	id: "",
	name: "",
	provider: "digitalplat",
	hasSecret: false,
	usageCount: 0,
	aliyunKey: "",
	aliyunSecret: "",
	cfToken: "",
	dnspodToken: "",
	heApiKey: "",
	digitalPlatApiKey: "",
	dnsheApiKey: "",
	dnsheApiSecret: "",
};

const providerOptions = [
	{ value: "digitalplat", label: "DigitalPlat" },
	{ value: "dnshe", label: "DNSHE" },
	{ value: "alidns", label: "阿里云 DNS" },
	{ value: "cloudflare", label: "Cloudflare" },
	{ value: "dnspod", label: "DNSPod" },
	{ value: "he", label: "Hurricane Electric" },
];

const providerName = (provider: string) => providerOptions.find((item) => item.value === provider)?.label || provider;

interface SecretInputProps {
	id: string;
	value: string;
	visible: boolean;
	onChange: (value: string) => void;
	onToggle: () => void;
}

function SecretInput({ id, value, visible, onChange, onToggle }: SecretInputProps) {
	return (
		<div className="input-group">
			<input
				id={id}
				className="form-control"
				type={visible ? "text" : "password"}
				value={value}
				onChange={(event) => onChange(event.target.value)}
			/>
			<button
				type="button"
				className="btn btn-icon"
				aria-label={visible ? "隐藏明文" : "显示明文"}
				title={visible ? "隐藏明文" : "显示明文"}
				onClick={onToggle}
			>
				{visible ? <IconEyeOff size={18} /> : <IconEye size={18} />}
			</button>
		</div>
	);
}

const credentialPayload = (draft: CredentialDetail): CredentialDetail => {
	const base = {
		...emptyCredential,
		id: draft.id,
		name: draft.name.trim(),
		provider: draft.provider,
		hasSecret: draft.hasSecret,
		usageCount: draft.usageCount,
	};
	switch (draft.provider) {
		case "alidns":
			return { ...base, aliyunKey: draft.aliyunKey || "", aliyunSecret: draft.aliyunSecret || "" };
		case "cloudflare":
			return { ...base, cfToken: draft.cfToken || "" };
		case "dnspod":
			return { ...base, dnspodToken: draft.dnspodToken || "" };
		case "he":
			return { ...base, heApiKey: draft.heApiKey || "" };
		case "dnshe":
			return { ...base, dnsheApiKey: draft.dnsheApiKey || "", dnsheApiSecret: draft.dnsheApiSecret || "" };
		default:
			return { ...base, digitalPlatApiKey: draft.digitalPlatApiKey || "" };
	}
};

export default function CredentialsSettingsPage() {
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<CredentialDetail>(emptyCredential);
	const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({});
	const [showForm, setShowForm] = useState(false);
	const [saving, setSaving] = useState(false);
	const [loadingID, setLoadingID] = useState("");
	const { data, isLoading, isError, error } = useQuery({
		queryKey: ["credentials"],
		queryFn: getCredentials,
	});

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const refresh = () => {
		queryClient.invalidateQueries({ queryKey: ["credentials"] });
		queryClient.invalidateQueries({ queryKey: ["dns-providers"] });
	};
	const updateDraft = (key: keyof CredentialDetail, value: any) => setDraft((prev) => ({ ...prev, [key]: value }));
	const closeForm = () => {
		setDraft(emptyCredential);
		setVisibleSecrets({});
		setShowForm(false);
	};
	const openNew = () => {
		setDraft(emptyCredential);
		setVisibleSecrets({});
		setShowForm(true);
	};
	const toggleSecret = (key: string) => setVisibleSecrets((prev) => ({ ...prev, [key]: !prev[key] }));

	const edit = async (id: string) => {
		setLoadingID(id);
		try {
			setDraft(await getCredential(id));
			setVisibleSecrets({});
			setShowForm(true);
		} catch (err: any) {
			showError(err.message);
		} finally {
			setLoadingID("");
		}
	};

	const save = async () => {
		setSaving(true);
		try {
			const payload = credentialPayload(draft);
			if (payload.id) {
				await updateCredential(payload);
			} else {
				await createCredential(payload);
			}
			closeForm();
			refresh();
			showSuccess("凭据已保存");
		} catch (err: any) {
			showError(err.message);
		} finally {
			setSaving(false);
		}
	};

	const remove = async (item: CredentialSummary) => {
		if (!window.confirm(`确定删除凭据「${item.name}」吗？`)) return;
		try {
			await deleteCredential(item.id);
			if (draft.id === item.id) {
				closeForm();
			}
			refresh();
			showSuccess("凭据已删除");
		} catch (err: any) {
			showError(err.message);
		}
	};

	const providerFields = (
		<>
			{draft.provider === "alidns" ? (
				<>
					<div className="col-md-6">
						<label className="form-label" htmlFor="credentialAliyunKey">
							AccessKey ID
						</label>
						<input
							id="credentialAliyunKey"
							className="form-control"
							value={draft.aliyunKey || ""}
							onChange={(event) => updateDraft("aliyunKey", event.target.value)}
						/>
					</div>
					<div className="col-md-6">
						<label className="form-label" htmlFor="credentialAliyunSecret">
							AccessKey Secret
						</label>
						<SecretInput
							id="credentialAliyunSecret"
							value={draft.aliyunSecret || ""}
							visible={!!visibleSecrets.aliyunSecret}
							onChange={(value) => updateDraft("aliyunSecret", value)}
							onToggle={() => toggleSecret("aliyunSecret")}
						/>
					</div>
				</>
			) : null}
			{draft.provider === "cloudflare" ? (
				<div className="col-md-12">
					<label className="form-label" htmlFor="credentialCFToken">
						API Token
					</label>
					<SecretInput
						id="credentialCFToken"
						value={draft.cfToken || ""}
						visible={!!visibleSecrets.cfToken}
						onChange={(value) => updateDraft("cfToken", value)}
						onToggle={() => toggleSecret("cfToken")}
					/>
				</div>
			) : null}
			{draft.provider === "dnspod" ? (
				<div className="col-md-12">
					<label className="form-label" htmlFor="credentialDNSPodToken">
						DNSPod Token
					</label>
					<SecretInput
						id="credentialDNSPodToken"
						value={draft.dnspodToken || ""}
						visible={!!visibleSecrets.dnspodToken}
						onChange={(value) => updateDraft("dnspodToken", value)}
						onToggle={() => toggleSecret("dnspodToken")}
					/>
					<small className="text-muted">格式：APP_ID,APP_TOKEN</small>
				</div>
			) : null}
			{draft.provider === "he" ? (
				<div className="col-md-12">
					<label className="form-label" htmlFor="credentialHEApiKey">
						Hurricane Electric API Key
					</label>
					<SecretInput
						id="credentialHEApiKey"
						value={draft.heApiKey || ""}
						visible={!!visibleSecrets.heApiKey}
						onChange={(value) => updateDraft("heApiKey", value)}
						onToggle={() => toggleSecret("heApiKey")}
					/>
				</div>
			) : null}
			{draft.provider === "digitalplat" ? (
				<div className="col-md-12">
					<label className="form-label" htmlFor="credentialDigitalPlatApiKey">
						API Key
					</label>
					<SecretInput
						id="credentialDigitalPlatApiKey"
						value={draft.digitalPlatApiKey || ""}
						visible={!!visibleSecrets.digitalPlatApiKey}
						onChange={(value) => updateDraft("digitalPlatApiKey", value)}
						onToggle={() => toggleSecret("digitalPlatApiKey")}
					/>
				</div>
			) : null}
			{draft.provider === "dnshe" ? (
				<>
					<div className="col-md-6">
						<label className="form-label" htmlFor="credentialDNSHEApiKey">
							API Key
						</label>
						<SecretInput
							id="credentialDNSHEApiKey"
							value={draft.dnsheApiKey || ""}
							visible={!!visibleSecrets.dnsheApiKey}
							onChange={(value) => updateDraft("dnsheApiKey", value)}
							onToggle={() => toggleSecret("dnsheApiKey")}
						/>
					</div>
					<div className="col-md-6">
						<label className="form-label" htmlFor="credentialDNSHEApiSecret">
							API Secret
						</label>
						<SecretInput
							id="credentialDNSHEApiSecret"
							value={draft.dnsheApiSecret || ""}
							visible={!!visibleSecrets.dnsheApiSecret}
							onChange={(value) => updateDraft("dnsheApiSecret", value)}
							onToggle={() => toggleSecret("dnsheApiSecret")}
						/>
					</div>
				</>
			) : null}
		</>
	);

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-azure" />
			<div className="card-header">
				<div className="row w-full align-items-center">
					<div className="col">
						<h2 className="mb-0">凭据管理</h2>
					</div>
					<div className="col-auto">
						<div className="ms-auto d-flex flex-wrap btn-list">
							<Button
								size="sm"
								onClick={() => showHelpModal("Credentials", "azure")}
								title="功能说明"
								ariaLabel="功能说明"
							>
								<IconHelp size={20} />
							</Button>
							<Button size="sm" className="btn-azure" onClick={openNew}>
								<IconPlus size={18} />
								新增凭据
							</Button>
						</div>
					</div>
				</div>
			</div>
			<Modal show={showForm} onHide={closeForm} backdrop="static" keyboard={!saving} centered size="lg">
				<Modal.Header closeButton>
					<Modal.Title>{draft.id ? "编辑凭据" : "新增凭据"}</Modal.Title>
				</Modal.Header>
				<Modal.Body>
					<div className="row">
						<div className="col-md-6">
							<label className="form-label" htmlFor="credentialName">
								名称
							</label>
							<input
								id="credentialName"
								className="form-control"
								value={draft.name}
								onChange={(event) => updateDraft("name", event.target.value)}
							/>
						</div>
						<div className="col-md-6">
							<label className="form-label" htmlFor="credentialProvider">
								类型
							</label>
							<select
								id="credentialProvider"
								className="form-select"
								value={draft.provider}
								onChange={(event) => updateDraft("provider", event.target.value)}
							>
								{providerOptions.map((item) => (
									<option value={item.value} key={item.value}>
										{item.label}
									</option>
								))}
							</select>
						</div>
					</div>
					<div className="row mt-3">{providerFields}</div>
				</Modal.Body>
				<Modal.Footer>
					<Button onClick={closeForm} disabled={saving}>
						取消
					</Button>
					<Button actionType="primary" onClick={save} isLoading={saving} disabled={saving}>
						保存凭据
					</Button>
				</Modal.Footer>
			</Modal>
			<div className="table-responsive">
				<table className="table card-table table-vcenter">
					<thead>
						<tr>
							<th>名称</th>
							<th>类型</th>
							<th>状态</th>
							<th>引用</th>
							<th className="w-1" />
						</tr>
					</thead>
					<tbody>
						{(data || []).map((item: CredentialSummary) => (
							<tr key={item.id}>
								<td>
									<div className="fw-semibold">{item.name}</div>
									<div className="text-muted small">{item.id}</div>
								</td>
								<td>{providerName(item.provider)}</td>
								<td>{item.hasSecret ? "已配置密钥" : "未配置密钥"}</td>
								<td>{item.usageCount ? `${item.usageCount} 处引用` : "未引用"}</td>
								<td>
									<div className="btn-list flex-nowrap">
										<Button
											size="sm"
											onClick={() => edit(item.id)}
											isLoading={loadingID === item.id}
										>
											编辑
										</Button>
										<Button size="sm" className="btn-red" onClick={() => remove(item)}>
											删除
										</Button>
									</div>
								</td>
							</tr>
						))}
						{!data?.length ? (
							<tr>
								<td colSpan={5} className="text-muted text-center py-4">
									暂无凭据
								</td>
							</tr>
						) : null}
					</tbody>
				</table>
			</div>
		</div>
	);
}
