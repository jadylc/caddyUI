import { IconPlus, IconTrash } from "@tabler/icons-react";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, FieldArray, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { Button, Loading } from "src/components";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { useErrorResponseTemplate, useSetErrorResponseTemplate } from "src/hooks";
import { intl, T } from "src/locale";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const showErrorResponseTemplateModal = (id: number | "new") => {
	EasyModal.show(ErrorResponseTemplateModal, { id });
};

interface Props extends InnerModalProps {
	id: number | "new";
}

const newRule = (id: number) => ({
	id,
	statuses: ["404"],
	action: "static_file",
	staticFile: "/404.html",
	redirectUrl: "",
	redirectCode: 302,
	upstream: "",
	body: "",
});

const templateValidation = (values: any) => {
	const errors: Record<string, any> = {};
	if (!String(values.name || "").trim()) {
		errors.name = intl.formatMessage({ id: "error.required" });
	}
	if (!values.rules?.length) {
		errors.rules = "至少需要一条规则";
	}
	return errors;
};

const ruleTargetField = (rule: any, index: number) => {
	switch (rule.action) {
		case "redirect":
			return (
				<div className="row g-2">
					<div className="col-md-8">
						<label className="form-label" htmlFor={`errorRule${index}RedirectUrl`}>
							跳转地址
						</label>
						<Field
							id={`errorRule${index}RedirectUrl`}
							name={`rules.${index}.redirectUrl`}
							className="form-control"
							placeholder="https://example.com/error"
						/>
					</div>
					<div className="col-md-4">
						<label className="form-label" htmlFor={`errorRule${index}RedirectCode`}>
							跳转状态码
						</label>
						<Field
							as="select"
							id={`errorRule${index}RedirectCode`}
							name={`rules.${index}.redirectCode`}
							className="form-select"
						>
							<option value={301}>301</option>
							<option value={302}>302</option>
							<option value={307}>307</option>
							<option value={308}>308</option>
						</Field>
					</div>
				</div>
			);
		case "reverse_proxy":
			return (
				<div>
					<label className="form-label" htmlFor={`errorRule${index}Upstream`}>
						错误页服务
					</label>
					<Field
						id={`errorRule${index}Upstream`}
						name={`rules.${index}.upstream`}
						className="form-control"
						placeholder="http://errors.internal:8080"
					/>
				</div>
			);
		case "respond":
			return (
				<div>
					<label className="form-label" htmlFor={`errorRule${index}Body`}>
						响应内容
					</label>
					<Field
						as="textarea"
						id={`errorRule${index}Body`}
						rows={3}
						name={`rules.${index}.body`}
						className="form-control"
					/>
				</div>
			);
		default:
			return (
				<div>
					<label className="form-label" htmlFor={`errorRule${index}StaticFile`}>
						错误页文件
					</label>
					<Field
						id={`errorRule${index}StaticFile`}
						name={`rules.${index}.staticFile`}
						className="form-control"
						placeholder="/404.html"
					/>
				</div>
			);
	}
};

const ErrorResponseTemplateModal = EasyModal.create(({ id, visible, remove }: Props) => {
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
	const { data, isLoading, error } = useErrorResponseTemplate(id);
	const { mutate: setTemplate } = useSetErrorResponseTemplate();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);

	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;
		setIsSubmitting(true);
		setErrorMsg(null);

		const payload = {
			...values,
			id: id === "new" ? undefined : id,
			name: String(values.name || "").trim(),
			description: String(values.description || "").trim(),
			staticRoot: String(values.staticRoot || "").trim(),
			enabled: values.enabled ?? true,
			meta: values.meta || {},
			rules: (values.rules || []).map((rule: any, index: number) => ({
				id: index + 1,
				statuses: rule.statuses?.length ? rule.statuses : ["default"],
				action: rule.action || "static_file",
				staticFile: String(rule.staticFile || "").trim(),
				redirectUrl: String(rule.redirectUrl || "").trim(),
				redirectCode: Number(rule.redirectCode || 302),
				upstream: String(rule.upstream || "").trim(),
				body: String(rule.body || "").trim(),
			})),
		};

		setTemplate(payload, {
			onError: (err: any) => setErrorMsg(err.message || "Unknown error"),
			onSuccess: () => {
				showObjectSuccess("error-response-template", "saved");
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
			<Modal show={visible} onHide={handleClose} backdrop="static" keyboard size="xl">
				{!isLoading && error && (
					<Alert variant="danger" className="m-3">
						{error?.message || "Unknown error"}
					</Alert>
				)}
				{isLoading && <Loading noLogo />}
				{!isLoading && data && (
					<Formik initialValues={data as any} validate={templateValidation} onSubmit={onSubmit}>
						{({ values, setFieldValue, errors, touched }: any) => (
							<Form>
								<DirtySync dirtyRef={dirtyRef} />
								<Modal.Header closeButton>
									<Modal.Title>
										<T
											id={data?.id ? "object.edit" : "object.add"}
											tData={{ object: "error-response-template" }}
										/>
									</Modal.Title>
								</Modal.Header>
								<Modal.Body>
									<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
										{errorMsg}
									</Alert>
									<div className="row g-3">
										<div className="col-md-6">
											<label className="form-label" htmlFor="errorTemplateName">
												<T id="name" />
											</label>
											<Field
												id="errorTemplateName"
												name="name"
												className={`form-control ${errors.name && touched.name ? "is-invalid" : ""}`}
											/>
											{errors.name && touched.name ? (
												<div className="invalid-feedback">{errors.name}</div>
											) : null}
										</div>
										<div className="col-md-6">
											<label className="form-label" htmlFor="errorTemplateStaticRoot">
												错误页目录
											</label>
											<Field
												id="errorTemplateStaticRoot"
												name="staticRoot"
												className="form-control"
												placeholder="/config/error-pages/default"
											/>
										</div>
										<div className="col-12">
											<label className="form-label" htmlFor="errorTemplateDescription">
												<T id="description" />
											</label>
											<Field id="errorTemplateDescription" name="description" className="form-control" />
										</div>
										<div className="col-12">
											<label className="form-check form-switch" htmlFor="errorTemplateEnabled">
												<Field
													id="errorTemplateEnabled"
													type="checkbox"
													name="enabled"
													className="form-check-input"
												/>
												<span className="form-check-label">
													<T id="status.enabled" />
												</span>
											</label>
										</div>
									</div>

									<div className="d-flex align-items-center justify-content-between mt-4 mb-2">
										<h3 className="mb-0">规则</h3>
										<Button
											type="button"
											size="sm"
											onClick={() => {
												const next = [...(values.rules || []), newRule((values.rules || []).length + 1)];
												setFieldValue("rules", next);
											}}
										>
											<IconPlus size={16} />
											添加规则
										</Button>
									</div>

									<FieldArray name="rules">
										{({ remove }) => (
											<div className="d-grid gap-3">
												{(values.rules || []).map((rule: any, index: number) => (
													<div className="border rounded p-3" key={rule.id || index}>
														<div className="row g-2 align-items-end">
															<div className="col-md-3">
																<label className="form-label" htmlFor={`errorRule${index}Status`}>
																	状态
																</label>
																<select
																	id={`errorRule${index}Status`}
																	className="form-select"
																	value={rule.statuses?.[0] || "default"}
																	onChange={(e) => setFieldValue(`rules.${index}.statuses`, [e.target.value])}
																>
																	<option value="403">403</option>
																	<option value="404">404</option>
																	<option value="4xx">4xx</option>
																	<option value="5xx">5xx</option>
																	<option value="default">默认</option>
																</select>
															</div>
															<div className="col-md-3">
																<label className="form-label" htmlFor={`errorRule${index}Action`}>
																	动作
																</label>
																<Field
																	as="select"
																	id={`errorRule${index}Action`}
																	name={`rules.${index}.action`}
																	className="form-select"
																>
																	<option value="static_file">静态文件</option>
																	<option value="redirect">跳转</option>
																	<option value="reverse_proxy">反代</option>
																	<option value="respond">响应</option>
																</Field>
															</div>
															<div className="col-md-5">{ruleTargetField(rule, index)}</div>
															<div className="col-md-1 text-end">
																<Button
																	type="button"
																	size="sm"
																	className="btn-icon"
																	disabled={(values.rules || []).length <= 1}
																	onClick={() => remove(index)}
																>
																	<IconTrash size={16} />
																</Button>
															</div>
														</div>
													</div>
												))}
											</div>
										)}
									</FieldArray>
								</Modal.Body>
								<Modal.Footer>
									<Button onClick={handleClose} disabled={isSubmitting}>
										<T id="cancel" />
									</Button>
									<Button type="submit" actionType="primary" disabled={isSubmitting} isLoading={isSubmitting}>
										<T id="save" />
									</Button>
								</Modal.Footer>
							</Form>
						)}
					</Formik>
				)}
			</Modal>
			<ConfirmDiscardModal show={showConfirm} onCancel={handleCancel} onConfirm={handleConfirm} />
		</>
	);
});

export { showErrorResponseTemplateModal };
