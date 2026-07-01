import cn from "classnames";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik, useFormikContext } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import {
	AccessField,
	Button,
	DomainCertificateBindingsField,
	DomainNamesField,
	Loading,
	LocationsFields,
	SSLOptionsFields,
	UpstreamsFields,
} from "src/components";
import { type ForwardAuthProviderID, forwardAuthProviders } from "src/forwardAuthProviders";
import { useAutheliaSettings, useAuthentikSettings, useProxyHost, useSetProxyHost } from "src/hooks";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { T } from "src/locale";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const providerOptions = Object.values(forwardAuthProviders);

const parseListenPorts = (value: any): number[] => {
	const raw = Array.isArray(value) ? value.join(",") : String(value || "");
	if (!raw.trim()) {
		return [];
	}
	const seen = new Set<number>();
	const ports: number[] = [];
	for (const part of raw.split(",")) {
		const port = Number(part);
		if (!Number.isInteger(port) || port <= 0 || port > 65535) {
			return [];
		}
		if (!seen.has(port)) {
			seen.add(port);
			ports.push(port);
		}
	}
	return ports;
};

const validateListenPorts = (value: any) => {
	const raw = Array.isArray(value) ? value.join(",") : String(value || "");
	if (!raw.trim()) return undefined;
	if (!/^\d+(,\d+)*$/.test(raw)) {
		return "监听端口必须是英文逗号分隔的数字，例如 443,11111";
	}
	for (const part of raw.split(",")) {
		const port = Number(part);
		if (!Number.isInteger(port) || port <= 0 || port > 65535) {
			return "监听端口必须是 1-65535 的数字";
		}
	}
	return undefined;
};

const formatListenPorts = (listenPort?: number, listenPorts?: number[]) => {
	const source = listenPorts?.length ? listenPorts : listenPort ? [listenPort] : [];
	const ports = source.filter((port, index, values) => port > 0 && values.indexOf(port) === index);
	return ports.join(",");
};

const normalizeUpstreams = (upstreams: any, fallback?: any) => {
	const source = Array.isArray(upstreams) && upstreams.length ? upstreams : fallback ? [fallback] : [];
	return source
		.map((item: any) => {
			const forwardScheme = item?.forwardScheme === "https" ? "https" : "http";
			const forwardHost = String(item?.forwardHost || "").trim();
			const forwardPort = Number(item?.forwardPort || (forwardScheme === "https" ? 443 : 80));
			const weight = Number(item?.weight || 1);
			if (!forwardHost || !Number.isInteger(forwardPort) || forwardPort < 1 || forwardPort > 65535) {
				return null;
			}
			return {
				forwardScheme,
				forwardHost,
				forwardPort,
				weight: Number.isInteger(weight) && weight >= 0 ? weight : 1,
			};
		})
		.filter(Boolean);
};

const normalizeLocations = (locations: any[] = []) =>
	locations
		.filter((loc) => String(loc?.path || "").trim() !== "")
		.map((loc) => {
			const upstreams = normalizeUpstreams(loc.upstreams, {
				forwardScheme: loc.forwardScheme,
				forwardHost: loc.forwardHost,
				forwardPort: loc.forwardPort,
				weight: 1,
			});
			const first = upstreams[0] ?? {
				forwardScheme: loc.forwardScheme || "http",
				forwardHost: loc.forwardHost || "",
				forwardPort: Number(loc.forwardPort || 80),
			};
			return {
				...loc,
				path: String(loc.path || "").trim(),
				forwardScheme: first.forwardScheme,
				forwardHost: first.forwardHost,
				forwardPort: Number(first.forwardPort || 0),
				forwardPath: String(loc.forwardPath || "").trim(),
				upstreams,
				loadBalancingPolicy: String(loc.loadBalancingPolicy || ""),
			};
		});

const initialLocations = (locations: any[] = []) =>
	locations.map((loc) => {
		const upstreams = normalizeUpstreams(loc.upstreams, {
			forwardScheme: loc.forwardScheme,
			forwardHost: loc.forwardHost,
			forwardPort: loc.forwardPort,
			weight: 1,
		});
		const first = upstreams[0] ?? {
			forwardScheme: loc.forwardScheme || "http",
			forwardHost: loc.forwardHost || "",
			forwardPort: Number(loc.forwardPort || 80),
		};
		return {
			...loc,
			forwardScheme: first.forwardScheme,
			forwardHost: first.forwardHost,
			forwardPort: Number(first.forwardPort || 80),
			upstreams,
			loadBalancingPolicy: loc.loadBalancingPolicy || "",
		};
	});

const showProxyHostModal = (id: number | "new", initialData?: any) => {
	EasyModal.show(ProxyHostModal, { id, initialData } as ShowProps);
};

interface Props extends InnerModalProps {
	id: number | "new";
	initialData?: any;
}
interface ShowProps {
	id: number | "new";
	initialData?: any;
}

function ForwardAuthFields() {
	const { values } = useFormikContext<any>();
	if (values.forwardAuth?.enabled !== true) return null;
	return <ForwardAuthContent />;
}

function ForwardAuthContent() {
	const { values } = useFormikContext<any>();
	const { data: autheliaSettings } = useAutheliaSettings();
	const { data: authentikSettings } = useAuthentikSettings();
	const provider = (values.forwardAuth?.provider === "authentik" ? "authentik" : "authelia") as ForwardAuthProviderID;
	const providerSpec = forwardAuthProviders[provider];
	const providerSettings = provider === "authentik" ? authentikSettings : autheliaSettings;
	const customCopyHeaders = Array.isArray(values.forwardAuth?.copyHeaders)
		? values.forwardAuth.copyHeaders
		: String(values.forwardAuth?.copyHeaders || "")
				.split(/[\n,]+/)
				.map((header: string) => header.trim())
				.filter(Boolean);
	const usesCustom =
		!values.forwardAuth?.useGlobal &&
		(!!String(values.forwardAuth?.upstream || "").trim() ||
			!!String(values.forwardAuth?.uri || "").trim() ||
			customCopyHeaders.length > 0);
	const endpoint = usesCustom
		? `${String(values.forwardAuth?.upstream || providerSpec.upstreamPlaceholder).replace(/\/$/, "")}${values.forwardAuth?.uri || providerSpec.uriPlaceholder}`
		: `${String(providerSettings?.upstream || providerSpec.upstreamPlaceholder).replace(/\/$/, "")}${providerSettings?.uri || providerSpec.uriPlaceholder}`;
	return (
		<div className="mt-3">
			<div className="mb-3">
				<label className="form-label" htmlFor="forwardAuthProvider">
					<T id="forward-auth.provider" />
				</label>
				<Field as="select" name="forwardAuth.provider" id="forwardAuthProvider" className="form-control">
					{providerOptions.map((item) => (
						<option key={item.id} value={item.id}>
							{item.name}
						</option>
					))}
				</Field>
			</div>
			<Alert variant={usesCustom || providerSettings?.enabled ? "info" : "warning"} className="py-2 mb-2">
				{usesCustom ? (
					<T id="forward-auth.summary" tData={{ endpoint }} />
				) : providerSettings?.enabled ? (
					<T id={providerSpec.summaryEnabledId} tData={{ endpoint }} />
				) : (
					<T id={providerSpec.summaryDisabledId} />
				)}
			</Alert>
			<div className="small text-secondary">
				{providerSpec.supportsFailOpen ? (
					<>
						<T id="authelia.fail-open" />:{" "}
						{autheliaSettings?.failOpen ? <T id="enabled" /> : <T id="disabled" />}
						{" · "}
					</>
				) : null}
				<a href={providerSpec.settingsPath} className="text-decoration-none">
					<T id={providerSpec.settingsLabelId} />
				</a>
			</div>
		</div>
	);
}

const ProxyHostModal = EasyModal.create(({ id, initialData, visible, remove }: Props) => {
	const { data, isLoading, error } = useProxyHost(id);
	const { mutate: setProxyHost } = useSetProxyHost();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);

	const defaultForwardAuth = {
		enabled: false,
		provider: "authelia",
		upstream: "",
		uri: "",
		copyHeaders: [],
		useGlobal: true,
	};
	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;
		setIsSubmitting(true);
		setErrorMsg(null);

		const rawForwardAuth = values.forwardAuth ?? {};
		const customCopyHeaders = Array.isArray(rawForwardAuth.copyHeaders)
			? rawForwardAuth.copyHeaders
			: String(rawForwardAuth.copyHeaders || "")
					.split(/[\n,]+/)
					.map((header: string) => header.trim())
					.filter(Boolean);
		const usesCustomForwardAuth =
			!rawForwardAuth.useGlobal &&
			(!!String(rawForwardAuth.upstream || "").trim() ||
				!!String(rawForwardAuth.uri || "").trim() ||
				customCopyHeaders.length > 0);
		const forwardAuthProvider = rawForwardAuth.provider === "authentik" ? "authentik" : "authelia";
		const forwardAuth =
			rawForwardAuth.enabled === true
				? usesCustomForwardAuth
					? {
							...rawForwardAuth,
							enabled: true,
							provider: forwardAuthProvider,
							copyHeaders: customCopyHeaders,
						}
					: { enabled: true, provider: forwardAuthProvider, useGlobal: true }
				: { enabled: false, provider: "", upstream: "", uri: "", copyHeaders: [], useGlobal: false };
		const listenPorts = parseListenPorts(values.listenPort);
		const upstreams = normalizeUpstreams(values.upstreams, {
			forwardScheme: values.forwardScheme,
			forwardHost: values.forwardHost,
			forwardPort: values.forwardPort,
			weight: 1,
		});
		const primaryUpstream = upstreams[0] ?? {
			forwardScheme: values.forwardScheme || "http",
			forwardHost: values.forwardHost || "",
			forwardPort: Number(values.forwardPort || 0),
		};

		const payload = {
			...values,
			id: id === "new" ? undefined : id,
			listenPort: listenPorts[0] || 0,
			listenPorts,
			forwardScheme: primaryUpstream.forwardScheme,
			forwardHost: primaryUpstream.forwardHost,
			forwardPort: Number(primaryUpstream.forwardPort || 0),
			upstreams,
			loadBalancingPolicy: String(values.loadBalancingPolicy || ""),
			locations: normalizeLocations(values.locations),
			accessListId: Number(values.accessListId || 0),
			forwardAuth,
			certificateId:
				values.sslForced && values.meta?.certificateBindings?.length
					? -1
					: values.certificateId === "new"
						? "new"
						: Number(values.certificateId || 0),
		};

		setProxyHost(payload, {
			onError: (err: any) => setErrorMsg(<T id={err.message} />),
			onSuccess: () => {
				showObjectSuccess("proxy-host", "saved");
				remove();
			},
			onSettled: () => {
				setIsSubmitting(false);
				setSubmitting(false);
			},
		});
	};

	return (<>
		<Modal show={visible} onHide={handleClose} backdrop="static" keyboard size="lg">
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
							// Details tab
							serviceName: initialData?.serviceName ?? data?.serviceName ?? "",
							domainNames: initialData?.domainNames ?? data?.domainNames ?? [],
							listenPort: formatListenPorts(
								initialData?.listenPort ?? data?.listenPort,
								initialData?.listenPorts ?? data?.listenPorts,
							),
							forwardScheme: initialData?.forwardScheme ?? data?.forwardScheme ?? "http",
							forwardHost: initialData?.forwardHost ?? data?.forwardHost ?? "",
							forwardPort: initialData?.forwardPort ?? data?.forwardPort ?? undefined,
							upstreams: normalizeUpstreams(initialData?.upstreams ?? data?.upstreams, {
								forwardScheme: initialData?.forwardScheme ?? data?.forwardScheme ?? "http",
								forwardHost: initialData?.forwardHost ?? data?.forwardHost ?? "",
								forwardPort: initialData?.forwardPort ?? data?.forwardPort ?? 80,
								weight: 1,
							}),
							loadBalancingPolicy:
								initialData?.loadBalancingPolicy ?? data?.loadBalancingPolicy ?? "",
							accessListId: initialData?.accessListId ?? data?.accessListId ?? 0,
							upstreamInsecureSkipVerify:
								initialData?.upstreamInsecureSkipVerify ??
								data?.upstreamInsecureSkipVerify ??
								false,
							// Locations tab
							locations: initialLocations(initialData?.locations ?? data?.locations ?? []),
							// SSL tab
							certificateId: initialData?.certificateId ?? data?.certificateId ?? 0,
							sslForced: initialData?.sslForced ?? data?.sslForced ?? false,
							http2Support: initialData?.http2Support ?? data?.http2Support ?? false,
							hstsEnabled: initialData?.hstsEnabled ?? data?.hstsEnabled ?? false,
							hstsSubdomains: initialData?.hstsSubdomains ?? data?.hstsSubdomains ?? false,
							trustForwardedProto: initialData?.trustForwardedProto ?? data?.trustForwardedProto ?? false,
							forwardAuth: {
								...defaultForwardAuth,
								...(data?.forwardAuth ?? {}),
								...(initialData?.forwardAuth ?? {}),
								copyHeaders: Array.isArray(
									initialData?.forwardAuth?.copyHeaders ??
										data?.forwardAuth?.copyHeaders ??
										defaultForwardAuth.copyHeaders,
								)
									? (
											initialData?.forwardAuth?.copyHeaders ??
											data?.forwardAuth?.copyHeaders ??
											defaultForwardAuth.copyHeaders
										).join("\n")
									: (initialData?.forwardAuth?.copyHeaders ?? data?.forwardAuth?.copyHeaders ?? ""),
							},
							meta: initialData?.meta ?? data?.meta ?? {},
						} as any
					}
					onSubmit={onSubmit}
				>
					{() => (
						<Form>
							<DirtySync dirtyRef={dirtyRef} />
							<Modal.Header closeButton>
								<Modal.Title>
									<T
										id={id !== "new" ? "object.edit" : "object.add"}
										tData={{ object: "proxy-host" }}
									/>
								</Modal.Title>
							</Modal.Header>
							<Modal.Body className="p-0">
								<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
									{errorMsg}
								</Alert>
								<div className="card m-0 border-0">
									<div className="card-header">
										<ul className="nav nav-tabs card-header-tabs" data-bs-toggle="tabs">
											<li className="nav-item" role="presentation">
												<a
													href="#tab-details"
													className="nav-link active"
													data-bs-toggle="tab"
													aria-selected="true"
													role="tab"
												>
													<T id="column.details" />
												</a>
											</li>
											<li className="nav-item" role="presentation">
												<a
													href="#tab-locations"
													className="nav-link"
													data-bs-toggle="tab"
													aria-selected="false"
													tabIndex={-1}
													role="tab"
												>
													<T id="column.custom-locations" />
												</a>
											</li>
											<li className="nav-item" role="presentation">
												<a
													href="#tab-ssl"
													className="nav-link"
													data-bs-toggle="tab"
													aria-selected="false"
													tabIndex={-1}
													role="tab"
												>
													<T id="column.ssl" />
												</a>
											</li>
										</ul>
									</div>
									<div className="card-body">
										<div className="tab-content">
											<div className="tab-pane active show" id="tab-details" role="tabpanel">
												<Field name="serviceName">
													{({ field }: any) => (
														<div className="mb-3">
															<label className="form-label" htmlFor="serviceName">
																服务名称
															</label>
															<input
																id="serviceName"
																type="text"
																className="form-control"
																placeholder="例如：AList / Home Assistant"
																{...field}
															/>
														</div>
													)}
												</Field>
												<DomainNamesField isWildcardPermitted dnsProviderWildcardSupported />
												<Field name="listenPort" validate={validateListenPorts}>
													{({ field, form }: any) => (
														<div className="mb-3">
															<label className="form-label" htmlFor="listenPort">
																<T id="host.listen-port" />
															</label>
															<input
																id="listenPort"
																type="text"
																className={`form-control ${form.errors.listenPort && form.touched.listenPort ? "is-invalid" : ""}`}
																placeholder="443,11111"
																{...field}
															/>
															<small className="text-secondary">
																<T id="host.listen-port.help" />
															</small>
															{form.errors.listenPort ? (
																<div className="invalid-feedback">
																	{form.errors.listenPort && form.touched.listenPort
																		? form.errors.listenPort
																		: null}
																</div>
															) : null}
														</div>
													)}
												</Field>
												<h4 className="py-2">
													<T id="load-balancing.upstreams" />
												</h4>
												<UpstreamsFields />
												<AccessField />
												<div className="my-3">
													<h4 className="py-2">
														<T id="options" />
													</h4>
													<div className="divide-y">
															<div>
															<label className="row" htmlFor="upstreamInsecureSkipVerify">
																<span className="col">
																	<T id="host.flags.upstream-insecure-skip-verify" />
																	<small className="d-block text-secondary">
																		<T id="host.flags.upstream-insecure-skip-verify.help" />
																	</small>
																</span>
																<span className="col-auto">
																	<Field name="upstreamInsecureSkipVerify" type="checkbox">
																		{({ field }: any) => (
																			<label className="form-check form-check-single form-switch">
																				<input
																					{...field}
																					id="upstreamInsecureSkipVerify"
																					className={cn("form-check-input", {
																						"bg-lime": field.checked,
																					})}
																					type="checkbox"
																				/>
																			</label>
																		)}
																	</Field>
																</span>
															</label>
														</div>
														<div>
															<label className="row" htmlFor="forwardAuthEnabled">
																<span className="col">
																	<T id="forward-auth.toggle" />
																</span>
																<span className="col-auto">
																	<Field name="forwardAuth.enabled" type="checkbox">
																		{({ field }: any) => (
																			<label className="form-check form-check-single form-switch">
																				<input
																					{...field}
																					id="forwardAuthEnabled"
																					className={cn("form-check-input", {
																						"bg-lime": field.checked,
																					})}
																					type="checkbox"
																				/>
																			</label>
																		)}
																	</Field>
																</span>
															</label>
															<ForwardAuthFields />
														</div>
													</div>
												</div>
											</div>
											<div className="tab-pane" id="tab-locations" role="tabpanel">
												<LocationsFields
													initialValues={initialData?.locations ?? data?.locations ?? []}
												/>
											</div>
											<div className="tab-pane" id="tab-ssl" role="tabpanel">
												<DomainCertificateBindingsField />
												<SSLOptionsFields color="bg-lime" forProxyHost={true} />
											</div>
										</div>
									</div>
								</div>
							</Modal.Body>
							<Modal.Footer>
								<Button data-bs-dismiss="modal" onClick={handleClose} disabled={isSubmitting}>
									<T id="cancel" />
								</Button>
								<Button
									type="submit"
									actionType="primary"
									className="ms-auto bg-lime"
									data-bs-dismiss="modal"
									isLoading={isSubmitting}
									disabled={isSubmitting}
								>
									<T id="save" />
								</Button>
							</Modal.Footer>
						</Form>
					)}
				</Formik>
			)}
		</Modal>
		<ConfirmDiscardModal show={showConfirm} onConfirm={handleConfirm} onCancel={handleCancel} />
	</>);
});

export { showProxyHostModal };
