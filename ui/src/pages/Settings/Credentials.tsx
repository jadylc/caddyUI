import { IconHelp, IconPlus } from "@tabler/icons-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Alert from "react-bootstrap/Alert";
import Modal from "react-bootstrap/Modal";
import {
	type CredentialDetail,
	type CredentialSummary,
	type ProviderSpec,
	createCredential,
	deleteCredential,
	getCredential,
	getCredentials,
	updateCredential,
} from "src/api/backend";
import { Button, LoadingPage, useLocalPagination } from "src/components";
import { ProviderCredentialFields } from "src/components/Form/ProviderCredentialFields";
import { showHelpModal } from "src/modals";
import { ConfirmDiscardModal } from "src/modals/ConfirmDiscardModal";
import { showError, showSuccess } from "src/notifications";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { useProviders } from "src/hooks/useProviders";

const emptyCredential: CredentialDetail = {
	id: "",
	name: "",
	provider: "digitalplat",
	hasSecret: false,
	usageCount: 0,
	secrets: {},
};

// 类型选项与字段 schema 全部由后端 provider 注册表下发（见 useProviders），
// 新增 provider 时前端无需改动。
const providerOptionsOf = (providers: ProviderSpec[]) =>
	providers.map((item) => ({ value: item.id, label: item.name }));

const providerNameOf = (provider: string, providers: ProviderSpec[]) =>
	providers.find((item) => item.id === provider)?.name || provider;

// 取某个 provider 的字段 schema，供表单渲染。
const providerFieldsOf = (provider: string, providers: ProviderSpec[]) => {
	return providers.find((item) => item.id === provider)?.fields;
};

const credentialPayload = (draft: CredentialDetail): CredentialDetail => {
	const secrets: Record<string, string> = {};
	for (const [key, value] of Object.entries(draft.secrets || {})) {
		if (value) {
			secrets[key] = value;
		}
	}
	return {
		id: draft.id,
		name: draft.name.trim(),
		provider: draft.provider,
		hasSecret: draft.hasSecret,
		usageCount: draft.usageCount,
		secrets: Object.keys(secrets).length ? secrets : undefined,
		// 历史类型化字段不再提交：后端以 secrets 为准回填，避免出现重复存储。
		aliyunKey: undefined,
		aliyunSecret: undefined,
		cfToken: undefined,
		dnspodToken: undefined,
		heApiKey: undefined,
		digitalPlatApiKey: undefined,
		dnsheApiKey: undefined,
		dnsheApiSecret: undefined,
	};
};

export default function CredentialsSettingsPage() {
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<CredentialDetail>(emptyCredential);
	const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({});
	const [showForm, setShowForm] = useState(false);
	const [saving, setSaving] = useState(false);
	const [loadingID, setLoadingID] = useState("");

	const closeForm = () => {
		dirtyRef.current = false;
		setDraft(emptyCredential);
		setVisibleSecrets({});
		setShowForm(false);
	};

	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(closeForm);

	const { data, isLoading, isError, error } = useQuery({
		queryKey: ["credentials"],
		queryFn: getCredentials,
	});
	// provider 类型选项与字段 schema 全部来自后端注册表。
	const { data: providers } = useProviders();
	const providerList = providers || [];
	const credentials = data || [];
	const { pageItems: credentialPageItems, pagination: credentialPagination } = useLocalPagination(credentials);

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;

	const refresh = () => {
		queryClient.invalidateQueries({ queryKey: ["credentials"] });
		queryClient.invalidateQueries({ queryKey: ["dns-providers"] });
		queryClient.invalidateQueries({ queryKey: ["providers"] });
	};
	const updateDraft = (key: keyof CredentialDetail, value: any) => {
		dirtyRef.current = true;
		setDraft((prev) => ({ ...prev, [key]: value }));
	};
	const openNew = () => {
		dirtyRef.current = false;
		setDraft({
			...emptyCredential,
			fields: providerFieldsOf(emptyCredential.provider, providerList),
		});
		setVisibleSecrets({});
		setShowForm(true);
	};
	const toggleSecret = (key: string) => setVisibleSecrets((prev) => ({ ...prev, [key]: !prev[key] }));

	// 切换类型时清空密钥并按新类型的 schema 渲染字段。
	const selectProvider = (provider: string) => {
		dirtyRef.current = true;
		setDraft((prev) => ({
			...prev,
			provider,
			secrets: {},
			fields: providerFieldsOf(provider, providerList),
		}));
		setVisibleSecrets({});
	};

	const edit = async (id: string) => {
		setLoadingID(id);
		try {
			const detail = await getCredential(id);
			setDraft({
				...detail,
				fields: detail.fields || providerFieldsOf(detail.provider, providerList),
			});
			setVisibleSecrets({});
			dirtyRef.current = false;
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
			dirtyRef.current = false;
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
		<ProviderCredentialFields
			fields={draft.fields}
			values={draft.secrets}
			visibleSecrets={visibleSecrets}
			onToggleSecret={toggleSecret}
			idPrefix="credential"
			onChange={(key: string, value: string) => updateDraft("secrets", { ...draft.secrets, [key]: value })}
		/>
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
			<Modal show={showForm} onHide={handleClose} backdrop="static" keyboard centered size="lg">
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
								onChange={(event) => selectProvider(event.target.value)}
							>
								{providerOptionsOf(providerList).map((item) => (
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
					<Button onClick={handleClose} disabled={saving}>
						取消
					</Button>
					<Button actionType="primary" onClick={save} isLoading={saving} disabled={saving}>
						保存凭据
					</Button>
				</Modal.Footer>
			</Modal>
			<ConfirmDiscardModal show={showConfirm} onConfirm={handleConfirm} onCancel={handleCancel} />
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
						{credentialPageItems.map((item: CredentialSummary) => (
							<tr key={item.id}>
								<td>
									<div className="fw-semibold">{item.name}</div>
									<div className="text-muted small">{item.id}</div>
								</td>
								<td>{providerNameOf(item.provider, providerList)}</td>
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
						{!credentials.length ? (
							<tr>
								<td colSpan={5} className="text-muted text-center py-4">
									暂无凭据
								</td>
							</tr>
						) : null}
					</tbody>
				</table>
			</div>
			{credentialPagination}
		</div>
	);
}
