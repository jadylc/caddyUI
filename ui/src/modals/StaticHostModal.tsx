import cn from "classnames";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import {
	AccessField,
	AccessLogFields,
	Button,
	DomainCertificateBindingsField,
	DomainNamesField,
	Loading,
	ResponseOptimizationFields,
	SSLOptionsFields,
	normalizeAccessLog,
	normalizeResponseOptimization,
	withAccessLogDefaults,
	withResponseOptimizationDefaults,
} from "src/components";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { useSetStaticHost, useStaticHost } from "src/hooks";
import { T } from "src/locale";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

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

const validateListenPorts = (value: any, sslForced?: boolean) => {
	const raw = Array.isArray(value) ? value.join(",") : String(value || "");
	if (!raw.trim()) return undefined;
	if (!/^\d+(,\d+)*$/.test(raw)) {
		return "监听端口必须是英文逗号分隔的数字，例如 80,8080";
	}
	for (const part of raw.split(",")) {
		const port = Number(part);
		if (!Number.isInteger(port) || port <= 0 || port > 65535) {
			return "监听端口必须是 1-65535 的数字";
		}
		if (!sslForced && port === 443) {
			return "仅 HTTP 模式不能监听 443；请留空/使用 80，或启用 SSL";
		}
	}
	return undefined;
};

const formatListenPorts = (listenPort?: number, listenPorts?: number[]) => {
	const source = listenPorts?.length ? listenPorts : listenPort ? [listenPort] : [];
	const ports = source.filter((port, index, values) => port > 0 && values.indexOf(port) === index);
	return ports.join(",");
};

const validateRootPath = (value: any) => {
	const raw = String(value || "").trim();
	if (!raw) return "站点目录不能为空";
	if (!raw.startsWith("/") && !/^[a-zA-Z]:[\\/]/.test(raw)) {
		return "站点目录必须是绝对路径，例如 /srv/sites/app";
	}
	if (/[\r\n\t]/.test(raw)) {
		return "站点目录不能包含控制字符";
	}
	return undefined;
};

const validateIndexFile = (value: any) => {
	const raw = String(value || "").trim();
	if (!raw) return "Fallback 文件不能为空";
	if (!raw.startsWith("/")) return "Fallback 文件必须以 / 开头";
	if (/[\s{}]/.test(raw)) return "Fallback 文件不能包含空白或占位符";
	if (raw.split("/").includes("..")) return "Fallback 文件不能包含 .. 路径段";
	return undefined;
};

const showStaticHostModal = (id: number | "new", initialData?: any) => {
	EasyModal.show(StaticHostModal, { id, initialData } as ShowProps);
};

interface Props extends InnerModalProps {
	id: number | "new";
	initialData?: any;
}

interface ShowProps {
	id: number | "new";
	initialData?: any;
}

const StaticHostModal = EasyModal.create(({ id, initialData, visible, remove }: Props) => {
	const { data, isLoading, error } = useStaticHost(id);
	const { mutate: setStaticHost } = useSetStaticHost();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);

	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;
		setIsSubmitting(true);
		setErrorMsg(null);

		const listenPorts = parseListenPorts(values.listenPort);
		const mode = values.mode === "spa" ? "spa" : "static";
		const payload = {
			...values,
			id: id === "new" ? undefined : id,
			listenPort: listenPorts[0] || 0,
			listenPorts,
			rootPath: String(values.rootPath || "").trim(),
			mode,
			indexFile: mode === "spa" ? String(values.indexFile || "/index.html").trim() : "",
			directoryBrowsing: mode === "static" && !!values.directoryBrowsing,
			accessListId: Number(values.accessListId || 0),
			response: normalizeResponseOptimization(values.response, "public, max-age=3600"),
			accessLog: normalizeAccessLog(values.accessLog),
			certificateId:
				values.sslForced && values.meta?.certificateBindings?.length
					? -1
					: values.certificateId === "new"
						? "new"
						: Number(values.certificateId || 0),
		};

		setStaticHost(payload, {
			onError: (err: any) => setErrorMsg(<T id={err.message} />),
			onSuccess: () => {
				showObjectSuccess("static-host", "saved");
				remove();
			},
			onSettled: () => {
				setIsSubmitting(false);
				setSubmitting(false);
			},
		});
	};

	return (
		<>
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
								serviceName: initialData?.serviceName ?? data?.serviceName ?? "",
								domainNames: initialData?.domainNames ?? data?.domainNames ?? [],
								listenPort: formatListenPorts(
									initialData?.listenPort ?? data?.listenPort,
									initialData?.listenPorts ?? data?.listenPorts,
								),
								rootPath: initialData?.rootPath ?? data?.rootPath ?? "",
								mode: initialData?.mode ?? data?.mode ?? "static",
								indexFile: initialData?.indexFile ?? data?.indexFile ?? "/index.html",
								directoryBrowsing:
									initialData?.directoryBrowsing ?? data?.directoryBrowsing ?? false,
								accessListId: initialData?.accessListId ?? data?.accessListId ?? 0,
								certificateId: initialData?.certificateId ?? data?.certificateId ?? 0,
								sslForced: initialData?.sslForced ?? data?.sslForced ?? false,
								http2Support: initialData?.http2Support ?? data?.http2Support ?? false,
								hstsEnabled: initialData?.hstsEnabled ?? data?.hstsEnabled ?? false,
								hstsSubdomains:
									initialData?.hstsSubdomains ?? data?.hstsSubdomains ?? false,
								response: withResponseOptimizationDefaults(
									initialData?.response ?? data?.response,
									"public, max-age=3600",
								),
								accessLog: withAccessLogDefaults(initialData?.accessLog ?? data?.accessLog),
								meta: initialData?.meta ?? data?.meta ?? {},
							} as any
						}
						validate={(values: any) => {
							const errors: Record<string, string> = {};
							const listenPortError = validateListenPorts(values.listenPort, values.sslForced);
							if (listenPortError) errors.listenPort = listenPortError;
							const rootPathError = validateRootPath(values.rootPath);
							if (rootPathError) errors.rootPath = rootPathError;
							if (values.mode === "spa") {
								const indexFileError = validateIndexFile(values.indexFile);
								if (indexFileError) errors.indexFile = indexFileError;
							}
							return errors;
						}}
						onSubmit={onSubmit}
					>
						{({ values, setFieldValue }) => (
							<Form>
								<DirtySync dirtyRef={dirtyRef} />
								<Modal.Header closeButton>
									<Modal.Title>
										<T
											id={id !== "new" ? "object.edit" : "object.add"}
											tData={{ object: "static-host" }}
										/>
									</Modal.Title>
								</Modal.Header>
								<Modal.Body className="p-0">
									<Alert
										variant="danger"
										show={!!errorMsg}
										onClose={() => setErrorMsg(null)}
										dismissible
									>
										{errorMsg}
									</Alert>
									<div className="card m-0 border-0">
										<div className="card-header">
											<ul className="nav nav-tabs card-header-tabs" data-bs-toggle="tabs">
												<li className="nav-item" role="presentation">
													<a
														href="#tab-static-details"
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
														href="#tab-static-access"
														className="nav-link"
														data-bs-toggle="tab"
														aria-selected="false"
														tabIndex={-1}
														role="tab"
													>
														<T id="column.access" />
													</a>
												</li>
												<li className="nav-item" role="presentation">
													<a
														href="#tab-static-response"
														className="nav-link"
														data-bs-toggle="tab"
														aria-selected="false"
														tabIndex={-1}
														role="tab"
													>
														运维
													</a>
												</li>
												<li className="nav-item" role="presentation">
													<a
														href="#tab-static-ssl"
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
												<div
													className="tab-pane active show"
													id="tab-static-details"
													role="tabpanel"
												>
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
																	placeholder="例如：博客 / 前端应用"
																	{...field}
																/>
															</div>
														)}
													</Field>
													<DomainNamesField isWildcardPermitted dnsProviderWildcardSupported />
													<Field name="listenPort">
														{({ field, form }: any) => (
															<div className="mb-3">
																<label className="form-label" htmlFor="listenPort">
																	<T id="host.listen-port" />
																</label>
																<input
																	id="listenPort"
																	type="text"
																	className={`form-control ${form.errors.listenPort && form.touched.listenPort ? "is-invalid" : ""}`}
																	placeholder={values.sslForced ? "443,11111" : "80,8080"}
																	{...field}
																/>
																{form.errors.listenPort ? (
																	<div className="invalid-feedback">
																		{form.errors.listenPort &&
																		form.touched.listenPort
																			? form.errors.listenPort
																			: null}
																	</div>
																) : null}
															</div>
														)}
													</Field>
													<Field name="rootPath">
														{({ field, form }: any) => (
															<div className="mb-3">
																<label className="form-label" htmlFor="rootPath">
																	站点目录
																</label>
																<input
																	id="rootPath"
																	type="text"
																	className={`form-control ${form.errors.rootPath && form.touched.rootPath ? "is-invalid" : ""}`}
																	required
																	placeholder="/srv/sites/app"
																	{...field}
																/>
																{form.errors.rootPath ? (
																	<div className="invalid-feedback">
																		{form.errors.rootPath && form.touched.rootPath
																			? form.errors.rootPath
																			: null}
																	</div>
																) : null}
															</div>
														)}
													</Field>
													<div className="mb-3">
														<div className="form-label">运行模式</div>
														<div className="btn-group w-100" role="group">
															<button
																type="button"
																className={cn("btn", values.mode !== "spa" && "btn-primary")}
																onClick={() => setFieldValue("mode", "static")}
															>
																静态文件
															</button>
															<button
																type="button"
																className={cn("btn", values.mode === "spa" && "btn-primary")}
																onClick={() => setFieldValue("mode", "spa")}
															>
																SPA 应用
															</button>
														</div>
													</div>
													{values.mode === "spa" ? (
														<Field name="indexFile">
															{({ field, form }: any) => (
																<div className="mb-3">
																	<label className="form-label" htmlFor="indexFile">
																		Fallback 文件
																	</label>
																	<input
																		id="indexFile"
																		type="text"
																		className={`form-control ${form.errors.indexFile && form.touched.indexFile ? "is-invalid" : ""}`}
																		required
																		placeholder="/index.html"
																		{...field}
																	/>
																	{form.errors.indexFile ? (
																		<div className="invalid-feedback">
																			{form.errors.indexFile && form.touched.indexFile
																				? form.errors.indexFile
																				: null}
																		</div>
																	) : null}
																</div>
															)}
														</Field>
													) : (
														<div className="my-3">
															<h4 className="py-2">
																<T id="options" />
															</h4>
															<div className="divide-y">
																<div>
																	<label className="row" htmlFor="directoryBrowsing">
																		<span className="col">目录浏览</span>
																		<span className="col-auto">
																			<Field name="directoryBrowsing" type="checkbox">
																				{({ field }: any) => (
																					<label className="form-check form-check-single form-switch">
																						<input
																							{...field}
																							id="directoryBrowsing"
																							className={cn(
																								"form-check-input",
																								{ "bg-cyan": field.checked },
																							)}
																							type="checkbox"
																						/>
																					</label>
																				)}
																			</Field>
																		</span>
																	</label>
																</div>
															</div>
														</div>
													)}
												</div>
												<div className="tab-pane" id="tab-static-access" role="tabpanel">
													<AccessField />
												</div>
												<div className="tab-pane" id="tab-static-response" role="tabpanel">
													<h4 className="py-2">响应优化</h4>
													<ResponseOptimizationFields
														color="bg-cyan"
														defaultCacheControl="public, max-age=3600"
													/>
													<h4 className="py-2 mt-4">访问日志</h4>
													<AccessLogFields color="bg-cyan" />
												</div>
												<div className="tab-pane" id="tab-static-ssl" role="tabpanel">
													<DomainCertificateBindingsField />
													<SSLOptionsFields color="bg-cyan" />
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
										className="ms-auto bg-cyan"
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
		</>
	);
});

export { showStaticHostModal };
