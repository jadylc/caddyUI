import { useQueryClient } from "@tanstack/react-query";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import Select from "react-select";
import { createCredential } from "src/api/backend";
import type { CredentialSummary, ProviderField, ProviderSpec } from "src/api/backend/models";
import { Button, Loading } from "src/components";
import { ProviderCredentialFields } from "src/components/Form/ProviderCredentialFields";
import { useCredentials, useDomainMonitor, useProviders, useSetDomainMonitor } from "src/hooks";
import { intl, T } from "src/locale";
import { validateDuration, validateNumber, validateString } from "src/modules/Validations";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const NEW_REGISTRAR_CREDENTIAL_PREFIX = "__new_registrar__";
const CUSTOM_REGISTRAR_PROVIDER = "__custom__";

const showDomainMonitorModal = (id: number | "new") => {
	EasyModal.show(DomainMonitorModal, { id });
};

interface Props extends InnerModalProps {
	id: number | "new";
}

const splitLines = (value: string) =>
	String(value || "")
		.split(/[\n,]+/)
		.map((item: string) => item.trim())
		.filter(Boolean);

const splitNumbers = (value: string) =>
	splitLines(value)
		.map((item: string) => Number(item))
		.filter((item: number) => Number.isInteger(item) && item >= 0);

// 域名商展示名与选项全部来自后端注册表（见 useProviders），不在前端硬编码。
const registrarProviderName = (provider: string, specs?: ProviderSpec[]) => {
	const id = String(provider || "").trim();
	const spec = specs?.find((item) => item.id === id);
	return spec?.name || id;
};

const registrarProviderValue = (value: string, specs?: ProviderSpec[]) => {
	const provider = String(value || "").trim();
	const known =
		!provider ||
		(specs || []).some((item) => item.id === provider || item.name === provider);
	return {
		registrarProvider: known ? provider : provider ? CUSTOM_REGISTRAR_PROVIDER : "",
		registrarProviderCustom: known ? "" : provider,
	};
};

// 新增凭据选项的 value：`__new_registrar__:<providerId>`。
const newRegistrarCredentialValue = (providerId: string) => `${NEW_REGISTRAR_CREDENTIAL_PREFIX}:${providerId}`;

const DomainMonitorModal = EasyModal.create(({ id, visible, remove }: Props) => {
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
	const queryClient = useQueryClient();
	const { data, isLoading, error } = useDomainMonitor(id);
	const { data: credentials, isLoading: credentialsLoading } = useCredentials();
	const { data: providers } = useProviders();
	const { mutate: setDomainMonitor } = useSetDomainMonitor();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);

	const registrarSpecs: ProviderSpec[] = (providers || []).filter((item) => item.registrar);
	const registrarProviderIds = new Set(registrarSpecs.map((item: ProviderSpec) => item.id));
	const registrarProviderOptions = [
		{ value: "", label: "自动识别" },
		...registrarSpecs.map((item: ProviderSpec) => ({ value: item.id, label: item.name })),
		{ value: CUSTOM_REGISTRAR_PROVIDER, label: "手动填写" },
	];
	const registrarCredentials = (credentials || []).filter(
		(item: CredentialSummary) => registrarProviderIds.has(item.provider),
	);
	const credentialOptions = [
		{ value: "", label: "自动选择（每个域名商仅有一个凭据时使用）" },
		...registrarCredentials.map((item: CredentialSummary) => ({
			value: item.id,
			label: `${registrarProviderName(item.provider, registrarSpecs)} - ${item.name}`,
			credentialId: item.id,
			provider: item.provider,
		})),
		...registrarSpecs.map((item: ProviderSpec) => ({
			value: newRegistrarCredentialValue(item.id),
			label: `新增 ${item.name} 凭据`,
			provider: item.id,
			isNew: true,
		})),
	];

	// 判断当前选择对应的域名商（新增凭据走 `__new_registrar__:<id>` 约定）。
	const credentialProviderOf = (credentialChoice: string) => {
		if (credentialChoice.startsWith(`${NEW_REGISTRAR_CREDENTIAL_PREFIX}:`)) {
			return credentialChoice.slice(`${NEW_REGISTRAR_CREDENTIAL_PREFIX}:`.length);
		}
		return (
			registrarCredentials.find((item: CredentialSummary) => item.id === credentialChoice)?.provider || ""
		);
	};

	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;
		setIsSubmitting(true);
		setErrorMsg(null);

		const creatingNewCredential = String(values.credentialChoice || "").startsWith(
			`${NEW_REGISTRAR_CREDENTIAL_PREFIX}:`,
		);
		let credentialId = creatingNewCredential ? "" : values.credentialChoice || "";
		if (creatingNewCredential) {
			const provider = credentialProviderOf(values.credentialChoice);
			const spec = registrarSpecs.find((item: ProviderSpec) => item.id === provider);
			const secrets: Record<string, string> = {};
			for (const field of spec?.fields || []) {
				const value = String(values.credentialSecrets?.[field.key] || "").trim();
				if (value) {
					secrets[field.key] = value;
				}
			}
			const missing = (spec?.fields || []).filter(
				(field: ProviderField) => field.required && !secrets[field.key],
			);
			if (missing.length) {
				setErrorMsg(
					`请填写 ${registrarProviderName(provider, registrarSpecs)} 的 ${missing
						.map((item: ProviderField) => item.label)
						.join("、")}`,
				);
				setIsSubmitting(false);
				setSubmitting(false);
				return;
			}
			try {
				const result = await createCredential({
					name:
						String(values.registrarCredentialName || "").trim() ||
						`${registrarProviderName(provider, registrarSpecs)} - ${values.name || "域名监控"}`,
					provider,
					secrets,
				});
				credentialId = result.id;
				queryClient.invalidateQueries({ queryKey: ["credentials"] });
			} catch (err: any) {
				setErrorMsg(<T id={err.message} />);
				setIsSubmitting(false);
				setSubmitting(false);
				return;
			}
		}

		const selectedCredentialProvider = credentialProviderOf(values.credentialChoice);
		const renewableSpec = registrarSpecs.find((item: ProviderSpec) => item.id === selectedCredentialProvider);
		const usingRenewableCredential = !!renewableSpec?.renewable;
		const payload = {
			...values,
			id: id === "new" ? undefined : id,
			credentialId,
			registrarProvider:
				values.registrarProvider === CUSTOM_REGISTRAR_PROVIDER
					? String(values.registrarProviderCustom || "").trim()
					: String(values.registrarProvider || "").trim(),
			thresholdDays: Number(values.thresholdDays || 0),
			reminderDays: splitNumbers(values.reminderDays),
			autoRenew: usingRenewableCredential && !!values.autoRenew,
			renewBeforeDays: usingRenewableCredential ? Number(values.renewBeforeDays || 0) : 0,
			domainNames: splitLines(values.domainNames),
			resolvers: splitLines(values.resolvers),
			credentialChoice: undefined,
			registrarProviderCustom: undefined,
			registrarCredentialName: undefined,
			credentialSecrets: undefined,
		};

		setDomainMonitor(payload, {
			onError: (err: any) => setErrorMsg(<T id={err.message} />),
			onSuccess: () => {
				showObjectSuccess("domain-monitor", "saved");
				remove();
			},
			onSettled: () => {
				setIsSubmitting(false);
				setSubmitting(false);
			},
		});
	};

	return (
		<><Modal show={visible} onHide={handleClose} backdrop="static" keyboard size="lg">
			{!isLoading && error && (
				<Alert variant="danger" className="m-3">
					{error?.message || "Unknown error"}
				</Alert>
			)}
			{isLoading && <Loading noLogo />}
			{!isLoading && data && (
				<Formik
					initialValues={
						{
							name: data.name || "",
							domainNames: (data.domainNames || []).join("\n"),
							checkSsl: data.checkSsl ?? false,
							checkDns: data.checkDns ?? true,
							checkDomain: data.checkDomain ?? true,
							credentialId: data.credentialId || "",
							credentialChoice: data.credentialId || "",
							...registrarProviderValue(
								data.registrarProvider ||
									data.meta?.registrarProvider ||
									data.meta?.registrar_provider ||
									"",
								registrarSpecs,
							),
							registrarCredentialName: data.name ? `${data.name} 域名商凭据` : "域名商凭据",
							credentialSecrets: {},
							reminderDays: (data.reminderDays || [30, 15, 7, 3, 1]).join(","),
							autoRenew: data.autoRenew ?? false,
							renewBeforeDays: data.renewBeforeDays || 30,
							checkInterval: data.checkInterval || "24h",
							thresholdDays: data.thresholdDays || 30,
							resolvers: (data.resolvers || []).join("\n"),
							enabled: data.enabled ?? true,
							meta: data.meta || {},
						} as any
					}
					onSubmit={onSubmit}
				>
					{({ values, setFieldValue }: any) => {
						const selectedCredentialOption =
							credentialOptions.find((option) => option.value === values.credentialChoice) ||
							(values.credentialChoice
								? { value: values.credentialChoice, label: `已选择凭据 ${values.credentialChoice}` }
								: credentialOptions[0]);
						const creatingNewCredential = String(values.credentialChoice || "").startsWith(
							`${NEW_REGISTRAR_CREDENTIAL_PREFIX}:`,
						);
						const creatingRegistrarCredential = creatingNewCredential;
						const selectedCredentialProvider = credentialProviderOf(values.credentialChoice);
						const creatingSpec = registrarSpecs.find(
							(item: ProviderSpec) => item.id === selectedCredentialProvider,
						);
						const usingRenewableCredential = !!creatingSpec?.renewable;
						const handleCredentialChange = (option: any) => {
							const nextValue = option?.value || "";
							setFieldValue("credentialChoice", nextValue);
							setFieldValue("credentialId", option?.credentialId || "");
							const nextProvider = credentialProviderOf(nextValue);
							// 切换到新的域名商时清空密钥，表单按新 schema 渲染。
							setFieldValue("credentialSecrets", {});
							const nextSpec = registrarSpecs.find((item: ProviderSpec) => item.id === nextProvider);
							if (!nextSpec?.renewable) {
								setFieldValue("autoRenew", false);
							}
						};

						return (
							<Form>
								<DirtySync dirtyRef={dirtyRef} />
								<Modal.Header closeButton>
									<Modal.Title>
										<T
											id={data?.id ? "object.edit" : "object.add"}
											tData={{ object: "domain-monitor" }}
										/>
									</Modal.Title>
								</Modal.Header>
								<Modal.Body>
									<Alert
										variant="danger"
										show={!!errorMsg}
										onClose={() => setErrorMsg(null)}
										dismissible
									>
										{errorMsg}
									</Alert>
									<Field name="name" validate={validateString(1, 255)}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="domainMonitorName">
													<T id="name" />
												</label>
												<input
													id="domainMonitorName"
													type="text"
													className={`form-control ${form.errors.name && form.touched.name ? "is-invalid" : ""}`}
													required
													{...field}
												/>
											</div>
										)}
									</Field>
									<Field name="domainNames" validate={(v: string) => !v?.trim() ? intl.formatMessage({ id: "error.required" }) : undefined}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="domainMonitorDomains">
													<T id="domain-monitor.domains" />
												</label>
												<textarea
													id="domainMonitorDomains"
													className={`form-control ${form.errors.domainNames && form.touched.domainNames ? "is-invalid" : ""}`}
													rows={4}
													required
													{...field}
												/>
												{form.errors.domainNames && form.touched.domainNames ? (
													<div className="invalid-feedback">{form.errors.domainNames}</div>
												) : (
													<small className="text-muted">
														<T id="domain-monitor.domains.help" />
													</small>
												)}
											</div>
										)}
									</Field>
									<div className="row">
										<div className="col-md-6">
											<Field name="checkInterval" validate={validateDuration()}>
												{({ field, form }: any) => (
													<div className="mb-3">
														<label className="form-label" htmlFor="domainMonitorInterval">
															<T id="domain-monitor.check-interval" />
														</label>
														<input
															id="domainMonitorInterval"
															className={`form-control ${form.errors.checkInterval && form.touched.checkInterval ? "is-invalid" : ""}`}
															required
															{...field}
														/>
														{form.errors.checkInterval && form.touched.checkInterval ? (
															<div className="invalid-feedback">{form.errors.checkInterval}</div>
														) : (
															<small className="text-muted">
																<T id="domain-monitor.check-interval.help" />
															</small>
														)}
													</div>
												)}
											</Field>
										</div>
										<div className="col-md-6">
											<Field name="thresholdDays" validate={validateNumber(0, 365)}>
												{({ field, form }: any) => (
													<div className="mb-3">
														<label className="form-label" htmlFor="domainMonitorThreshold">
															<T id="domain-monitor.threshold-days" />
														</label>
														<input
															id="domainMonitorThreshold"
															className={`form-control ${form.errors.thresholdDays && form.touched.thresholdDays ? "is-invalid" : ""}`}
															type="number"
															min={0}
															required
															{...field}
														/>
														{form.errors.thresholdDays && form.touched.thresholdDays ? (
															<div className="invalid-feedback">{form.errors.thresholdDays}</div>
														) : (
															<small className="text-muted">
																<T id="domain-monitor.threshold-days.help" />
															</small>
														)}
													</div>
												)}
											</Field>
										</div>
									</div>
									<Field name="resolvers">
										{({ field }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="domainMonitorResolvers">
													<T id="domain-monitor.resolvers" />
												</label>
												<textarea
													id="domainMonitorResolvers"
													className="form-control"
													rows={2}
													{...field}
												/>
												<small className="text-muted">
													<T id="domain-monitor.resolvers.help" />
												</small>
											</div>
										)}
									</Field>
									<div className="mb-3">
										<div className="form-label">监控状态</div>
										<div className="divide-y">
											<label className="row" htmlFor="domainMonitor-enabled">
												<span className="col">启用监控</span>
												<span className="col-auto">
													<Field name="enabled" type="checkbox">
														{({ field }: any) => (
															<label className="form-check form-check-single form-switch">
																<input
																	id="domainMonitor-enabled"
																	className="form-check-input"
																	type="checkbox"
																	{...field}
																	checked={field.value}
																/>
															</label>
														)}
													</Field>
												</span>
											</label>
										</div>
									</div>
									<div className="mb-3">
										<div className="form-label">检查内容</div>
										<div className="divide-y">
											{["checkDns", "checkSsl"].map((name) => (
												<label className="row" htmlFor={`domainMonitor-${name}`} key={name}>
													<span className="col">
														<T id={`domain-monitor.${name}`} />
													</span>
													<span className="col-auto">
														<Field name={name} type="checkbox">
															{({ field }: any) => (
																<label className="form-check form-check-single form-switch">
																	<input
																		id={`domainMonitor-${name}`}
																		className="form-check-input"
																		type="checkbox"
																		{...field}
																		checked={field.value}
																	/>
																</label>
															)}
														</Field>
													</span>
												</label>
											))}
										</div>
									</div>
									<div className="mb-3">
										<div className="form-label">域名到期检查</div>
										<div className="divide-y mb-3">
											<label className="row" htmlFor="domainMonitor-checkDomain">
												<span className="col">
													<T id="domain-monitor.checkDomain" />
												</span>
												<span className="col-auto">
													<Field name="checkDomain" type="checkbox">
														{({ field }: any) => (
															<label className="form-check form-check-single form-switch">
																<input
																	id="domainMonitor-checkDomain"
																	className="form-check-input"
																	type="checkbox"
																	{...field}
																	checked={field.value}
																/>
															</label>
														)}
													</Field>
												</span>
											</label>
										</div>
										{values.checkDomain ? (
											<>
												<div className="row mb-3">
													<div className="col-md-6">
														<label
															className="form-label"
															htmlFor="domainMonitorRegistrarProvider"
														>
															域名商
														</label>
														<select
															id="domainMonitorRegistrarProvider"
															className="form-select"
															value={values.registrarProvider}
															onChange={(event) =>
																setFieldValue("registrarProvider", event.target.value)
															}
														>
															{registrarProviderOptions.map((option) => (
																<option value={option.value} key={option.value}>
																	{option.label}
																</option>
															))}
														</select>
														<small className="text-muted">
															留空时按已知后缀自动识别；识别不到可手动填写。
														</small>
													</div>
													{values.registrarProvider === CUSTOM_REGISTRAR_PROVIDER ? (
														<div className="col-md-6">
															<label
																className="form-label"
																htmlFor="domainMonitorRegistrarProviderCustom"
															>
																手动域名商
															</label>
															<input
																id="domainMonitorRegistrarProviderCustom"
																className="form-control"
																type="text"
																value={values.registrarProviderCustom}
																onChange={(event) =>
																	setFieldValue(
																		"registrarProviderCustom",
																		event.target.value,
																	)
																}
																placeholder="例如：阿里云 / Namecheap"
															/>
														</div>
													) : null}
												</div>
												<label
													className="form-label"
													htmlFor="domainMonitorRegistrarCredential"
												>
													域名商凭据
												</label>
												<Select
													inputId="domainMonitorRegistrarCredential"
													className="react-select-container"
													classNamePrefix="react-select"
													isLoading={credentialsLoading}
													options={credentialOptions}
													value={selectedCredentialOption}
													onChange={handleCredentialChange}
													placeholder="选择或新增域名商凭据"
													menuPortalTarget={document.body}
												/>
												<small className="text-muted">
													仅用于查询域名到期；支持续期的域名商凭据还可用于手动/自动续期。
													<br />
													留空时每个域名商仅有一个凭据会自动使用。
												</small>
												{creatingRegistrarCredential ? (
													<div className="row mt-3">
														<div className="col-md-12">
															<label
																className="form-label"
																htmlFor="domainMonitorRegistrarCredentialName"
															>
																凭据名称
															</label>
															<input
																id="domainMonitorRegistrarCredentialName"
																className="form-control"
																type="text"
																value={values.registrarCredentialName}
																onChange={(event) =>
																	setFieldValue(
																		"registrarCredentialName",
																		event.target.value,
																	)
																}
															/>
														</div>
														<div className="col-md-12">
															<ProviderCredentialFields
																fields={creatingSpec?.fields}
																values={values.credentialSecrets}
																idPrefix="domainMonitor"
																onChange={(key: string, value: string) =>
																	setFieldValue("credentialSecrets", {
																		...(values.credentialSecrets || {}),
																		[key]: value,
																	})
																}
															/>
														</div>
													</div>
												) : null}
												<div className="row mt-3">
													<div className="col-md-6">
														<label
															className="form-label"
															htmlFor="domainMonitorReminderDays"
														>
															域名提醒天数
														</label>
														<input
															id="domainMonitorReminderDays"
															className="form-control"
															value={values.reminderDays}
															onChange={(event) =>
																setFieldValue("reminderDays", event.target.value)
															}
															placeholder="30,15,7,3,1"
														/>
														<small className="text-muted">
															英文逗号分隔，达到任一提前天数时推送提醒。
														</small>
													</div>
													{usingRenewableCredential ? (
														<div className="col-md-6">
															<label
																className="form-label"
																htmlFor="domainMonitorRenewBeforeDays"
															>
																自动续期提前天数
															</label>
															<input
																id="domainMonitorRenewBeforeDays"
																className="form-control"
																type="number"
																min={0}
																value={values.renewBeforeDays}
																onChange={(event) =>
																	setFieldValue("renewBeforeDays", event.target.value)
																}
															/>
															<label
																className="form-check mt-2"
																htmlFor="domainMonitorAutoRenew"
															>
																<input
																	id="domainMonitorAutoRenew"
																	className="form-check-input"
																	type="checkbox"
																	checked={values.autoRenew}
																	onChange={(event) =>
																		setFieldValue("autoRenew", event.target.checked)
																	}
																/>
																<span className="form-check-label">启用自动续期</span>
															</label>
														</div>
													) : null}
												</div>
											</>
										) : null}
									</div>
									{!values.checkDomain && !values.checkSsl && !values.checkDns ? (
										<Alert variant="warning">
											<T id="domain-monitor.check-type.warning" />
										</Alert>
									) : null}
								</Modal.Body>
								<Modal.Footer>
									<Button data-bs-dismiss="modal" onClick={handleClose} disabled={isSubmitting}>
										<T id="cancel" />
									</Button>
									<Button
										type="submit"
										actionType="primary"
										className="ms-auto"
										isLoading={isSubmitting}
										disabled={isSubmitting}
									>
										<T id="save" />
									</Button>
								</Modal.Footer>
							</Form>
						);
					}}
				</Formik>
			)}
		</Modal>
		<ConfirmDiscardModal show={showConfirm} onConfirm={handleConfirm} onCancel={handleCancel} />
	</>);
});

export { showDomainMonitorModal };
