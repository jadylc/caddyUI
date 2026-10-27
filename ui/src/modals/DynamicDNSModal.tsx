import { useQueryClient } from "@tanstack/react-query";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import Select from "react-select";
import { createCredential } from "src/api/backend";
import type { DNSProvider, ProviderField } from "src/api/backend/models";
import { Button, Loading } from "src/components";
import { ProviderCredentialFields } from "src/components/Form/ProviderCredentialFields";
import { useDnsProviders, useDynamicDNSItem, useSetDynamicDNSItem } from "src/hooks";
import { intl, T } from "src/locale";
import { validateDuration, validateString } from "src/modules/Validations";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const showDynamicDNSModal = (id: number | "new") => {
	EasyModal.show(DynamicDNSModal, { id });
};

// provider 展示名由注册表下发，不在前端硬编码。
const dnsProviderLabel = (providers: DNSProvider[], provider: string) => {
	const match = providers.find((p) => (p.provider || p.id) === provider);
	return match?.name || provider;
};

interface Props extends InnerModalProps {
	id: number | "new";
}

const DynamicDNSModal = EasyModal.create(({ id, visible, remove }: Props) => {
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
	const queryClient = useQueryClient();
	const { data, isLoading, error } = useDynamicDNSItem(id);
	const { data: dnsProviders, isLoading: providersLoading } = useDnsProviders();
	const providerList = dnsProviders || [];
	const { mutate: setDynamicDNS } = useSetDynamicDNSItem();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);

	const providerOptions = providerList.map((p: DNSProvider) => ({
		value: p.credentialId || p.id,
		label: p.name,
		provider: p.provider || p.id,
		credentialId: p.credentialId,
		saved: p.saved,
		fields: p.fields,
	}));

	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;

		// Validate provider selection
		if (!values.credentialId && !values.dnsProvider) {
			setErrorMsg(<T id="dynamic-dns.error.provider-required" />);
			return;
		}

		setIsSubmitting(true);
		setErrorMsg(null);

		let credentialId = values.credentialId;
		if (!credentialId && values.dnsProvider) {
			try {
				const provider = values.dnsProvider;
				const providerSpec = providerList.find((p) => (p.provider || p.id) === provider);
				const credData: any = {
					name: `动态域名 - ${dnsProviderLabel(providerList, provider)}`,
					provider,
					// 统一提交 secrets：后端按 provider 注册表回填历史字段。
					secrets: (providerSpec?.fields || []).reduce(
						(acc: Record<string, string>, field: ProviderField) => {
							acc[field.key] = values.secrets?.[field.key] || "";
							return acc;
						},
						{},
					),
				};
				const result = await createCredential(credData);
				credentialId = result.id;
				queryClient.invalidateQueries({ queryKey: ["credentials"] });
				queryClient.invalidateQueries({ queryKey: ["dns-providers"] });
			} catch (err: any) {
				setErrorMsg(<T id={err.message} />);
				setIsSubmitting(false);
				setSubmitting(false);
				return;
			}
		}

		const payload = {
			...values,
			id: id === "new" ? undefined : id,
			credentialId,
			domainNames: String(values.domainNames || "")
				.split(/[\n,]+/)
				.map((domain: string) => domain.trim())
				.filter(Boolean),
			resolvers: String(values.resolvers || "")
				.split(/[\n,]+/)
				.map((resolver: string) => resolver.trim())
				.filter(Boolean),
			ipServiceUrl: String(values.ipServiceUrl || "").trim(),
			dnsProvider: undefined,
			aliyunKey: undefined,
			aliyunSecret: undefined,
			cfToken: undefined,
			dnspodToken: undefined,
			heApiKey: undefined,
		};

		setDynamicDNS(payload, {
			onError: (err: any) => setErrorMsg(<T id={err.message} />),
			onSuccess: () => {
				showObjectSuccess("dynamic-dns", "saved");
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
							credentialId: data.credentialId || "",
							dnsProvider: "",
							secrets: { ...((data.meta?.credentialSecrets as Record<string, string>) || {}) },
							ipv4: data.ipv4 ?? true,
							ipv6: data.ipv6 ?? false,
							checkInterval: data.checkInterval || "5m",
							ttl: data.ttl || "",
							ipServiceUrl: data.ipServiceUrl || "",
							resolvers: (data.resolvers || []).join("\n"),
							enabled: data.enabled ?? true,
							meta: data.meta || {},
						} as any
					}
					onSubmit={onSubmit}
				>
					{({ values, setFieldValue }: any) => {
						const selectedOption = providerOptions.find(
							(o) =>
								o.credentialId === values.credentialId ||
								(!o.credentialId && o.provider === values.dnsProvider),
						);
						const isSavedCredentialSelected = !!selectedOption?.credentialId;
						const activeProvider = selectedOption?.provider || values.dnsProvider;
						const activeFields = selectedOption?.fields;

						const handleProviderChange = (option: any) => {
							if (option?.credentialId) {
								setFieldValue("credentialId", option.credentialId);
								setFieldValue("dnsProvider", "");
							} else {
								setFieldValue("credentialId", "");
								setFieldValue("dnsProvider", option?.provider || option?.value);
							}
							// 切换到新的提供商时清空密钥，表单按新 schema 渲染。
							setFieldValue("secrets", {});
						};

						return (
							<Form>
								<DirtySync dirtyRef={dirtyRef} />
								<Modal.Header closeButton>
									<Modal.Title>
										<T
											id={data?.id ? "object.edit" : "object.add"}
											tData={{ object: "dynamic-dns" }}
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
												<label className="form-label" htmlFor="dynamicDnsName">
													<T id="name" />
												</label>
												<input
													id="dynamicDnsName"
													type="text"
													className={`form-control ${form.errors.name && form.touched.name ? "is-invalid" : ""}`}
													required
													{...field}
												/>
											</div>
										)}
									</Field>
									<div className="mb-3">
										<label className="form-label" htmlFor="dynamicDnsProvider">
											<T id="certificates.dns.provider" />
										</label>
										<Select
											inputId="dynamicDnsProvider"
											className="react-select-container"
											classNamePrefix="react-select"
											isLoading={providersLoading}
											options={providerOptions}
											value={selectedOption || null}
											onChange={handleProviderChange}
											placeholder={intl.formatMessage({
												id: "certificates.dns.provider.placeholder",
											})}
											menuPortalTarget={document.body}
										/>
									{activeProvider && !isSavedCredentialSelected ? (
										<>
											<ProviderCredentialFields
												fields={activeFields}
												values={values.secrets || {}}
												idPrefix="dynamicDns"
												onChange={(key: string, value: string) =>
													setFieldValue("secrets", { ...(values.secrets || {}), [key]: value })
												}
											/>
											<small className="text-danger mt-1">
												<T id="certificates.dns.credentials-warning" />
											</small>
										</>
									) : null}
										{isSavedCredentialSelected && (
											<small className="text-muted mt-1">
												<T id="dynamic-dns.credential-reusing" />
											</small>
										)}
										<a
											href="/certificates"
											className="text-muted mt-1"
											style={{ fontSize: "0.85em" }}
										>
											<T id="dynamic-dns.credential-add" />
										</a>
									</div>
									<Field name="domainNames" validate={(v: string) => !v?.trim() ? intl.formatMessage({ id: "error.required" }) : undefined}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="dynamicDnsDomains">
													<T id="dynamic-dns.domains" />
												</label>
												<textarea
													id="dynamicDnsDomains"
													className={`form-control ${form.errors.domainNames && form.touched.domainNames ? "is-invalid" : ""}`}
													rows={3}
													required
													{...field}
												/>
												{form.errors.domainNames && form.touched.domainNames ? (
													<div className="invalid-feedback">{form.errors.domainNames}</div>
												) : (
													<small className="text-muted">
														<T id="dynamic-dns.domains.help" />
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
														<label className="form-label" htmlFor="checkInterval">
															<T id="dynamic-dns.check-interval" />
														</label>
														<input
															id="checkInterval"
															className={`form-control ${form.errors.checkInterval && form.touched.checkInterval ? "is-invalid" : ""}`}
															required
															{...field}
														/>
														{form.errors.checkInterval && form.touched.checkInterval ? (
															<div className="invalid-feedback">{form.errors.checkInterval}</div>
														) : (
															<small className="text-muted">
																<T id="dynamic-dns.check-interval.help" />
															</small>
														)}
													</div>
												)}
											</Field>
										</div>
										<div className="col-md-6">
											<Field name="ttl">
												{({ field }: any) => (
													<div className="mb-3">
														<label className="form-label" htmlFor="ttl">
															<T id="dynamic-dns.ttl" />
														</label>
														<input
															id="ttl"
															className="form-control"
															placeholder="1h"
															{...field}
														/>
														<small className="text-muted">
															<T id="dynamic-dns.ttl.help" />
														</small>
													</div>
												)}
											</Field>
										</div>
									</div>
									<Field name="resolvers">
										{({ field }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="resolvers">
													<T id="dynamic-dns.resolvers" />
												</label>
												<textarea id="resolvers" className="form-control" rows={2} {...field} />
												<small className="text-muted">
													<T id="dynamic-dns.resolvers.help" />
												</small>
											</div>
										)}
									</Field>
									<Field name="ipServiceUrl">
										{({ field }: any) => {
											const presetOptions = [
												{ value: "", label: "自动" },
												{ value: "https://api.ipify.org", label: "api.ipify.org" },
												{ value: "https://ifconfig.me/ip", label: "ifconfig.me" },
												{ value: "https://ipinfo.io/ip", label: "ipinfo.io" },
												{ value: "https://api.ip.sb/ip", label: "api.ip.sb" },
												{ value: "https://icanhazip.com", label: "icanhazip.com" },
												{ value: "custom", label: "Custom..." },
											];
											const isCustom =
												field.value && !presetOptions.some((o) => o.value === field.value);
											return (
												<div className="mb-3">
													<label className="form-label" htmlFor="ipServiceUrl">
														<T id="dynamic-dns.ip-service-url" />
													</label>
													<Select
														inputId="ipServiceUrl"
														className="react-select-container"
														classNamePrefix="react-select"
														options={presetOptions}
														value={
															isCustom
																? presetOptions.find((o) => o.value === "custom")
																: presetOptions.find((o) => o.value === field.value) ||
																	presetOptions[0]
														}
														onChange={(option: any) => {
															if (option?.value === "custom") {
																setFieldValue("ipServiceUrl", "https://");
															} else {
																setFieldValue("ipServiceUrl", option?.value || "");
															}
														}}
														menuPortalTarget={document.body}
													/>
													{isCustom && (
														<input
															id="ipServiceUrl"
															type="url"
															className="form-control mt-2"
															placeholder="https://example.com/ip"
															value={field.value}
															onChange={(e) =>
																setFieldValue("ipServiceUrl", e.target.value)
															}
														/>
													)}
													<small className="text-muted">
														<T id="dynamic-dns.ip-service-url.hint" />
													</small>
												</div>
											);
										}}
									</Field>
									<div className="divide-y mb-3">
										{["ipv4", "ipv6", "enabled"].map((name) => (
											<label className="row" htmlFor={`dynamicDns-${name}`} key={name}>
												<span className="col">
													<T id={name === "enabled" ? "enabled" : `dynamic-dns.${name}`} />
												</span>
												<span className="col-auto">
													<Field name={name} type="checkbox">
														{({ field }: any) => (
															<label className="form-check form-check-single form-switch">
																<input
																	id={`dynamicDns-${name}`}
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

export { showDynamicDNSModal };
