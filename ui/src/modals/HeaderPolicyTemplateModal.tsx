import { IconPlus, IconTrash } from "@tabler/icons-react";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, FieldArray, Form, Formik } from "formik";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { Button, Loading } from "src/components";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { useHeaderPolicyTemplate, useSetHeaderPolicyTemplate } from "src/hooks";
import { intl, T } from "src/locale";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const showHeaderPolicyTemplateModal = (id: number | "new") => {
	EasyModal.show(HeaderPolicyTemplateModal, { id });
};

interface Props extends InnerModalProps {
	id: number | "new";
}

const newHeaderRule = () => ({ action: "set", name: "", value: "" });

const templateValidation = (values: any) => {
	const errors: Record<string, any> = {};
	if (!String(values.name || "").trim()) {
		errors.name = intl.formatMessage({ id: "error.required" });
	}
	if (!values.headers?.filter((rule: any) => String(rule.name || "").trim()).length) {
		errors.headers = "至少需要一条 Header 规则";
	}
	return errors;
};

const categoryOptions = [
	{ value: "api", label: "API" },
	{ value: "static", label: "静态站" },
	{ value: "download", label: "下载站" },
	{ value: "security", label: "安全" },
	{ value: "cors", label: "CORS" },
	{ value: "custom", label: "自定义" },
];

const HeaderPolicyTemplateModal = EasyModal.create(({ id, visible, remove }: Props) => {
	const { data, isLoading, isError, error } = useHeaderPolicyTemplate(id);
	const mutation = useSetHeaderPolicyTemplate();
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
	const readOnly = !!data?.builtin;
	const initialValues = data ?? {
		id: 0,
		name: "",
		description: "",
		category: "custom",
		enabled: true,
		builtin: false,
		headers: [],
		meta: {},
	};

	const submit = async (values: any) => {
		if (readOnly) return;
		const payload = {
			...values,
			headers: (values.headers || []).filter((rule: any) => String(rule.name || "").trim()),
		};
		await mutation.mutateAsync(payload);
		showObjectSuccess("header-policy-template", "saved");
		remove();
	};

	return (
		<>
			<Formik initialValues={initialValues as any} enableReinitialize validate={templateValidation} onSubmit={submit}>
				{({ isSubmitting, values, errors, touched }) => (
					<Form>
						<DirtySync dirtyRef={dirtyRef} />
						<Modal show={visible} onHide={handleClose} size="lg" backdrop="static">
							<Modal.Header closeButton>
								<Modal.Title>
									<T id={id !== "new" ? "object.edit" : "object.add"} tData={{ object: "header-policy-template" }} />
								</Modal.Title>
							</Modal.Header>
							<Modal.Body>
								{isLoading ? <Loading /> : null}
								{isError ? <Alert variant="danger">{error?.message || "Unknown error"}</Alert> : null}
								{data ? (
									<div className="vstack gap-3">
										{readOnly ? <Alert variant="info">内置策略模板只读，可在站点中直接绑定。</Alert> : null}
										<div className="row g-3">
											<div className="col-md-6">
												<label className="form-label" htmlFor="policyTemplateName">
													名称
												</label>
												<Field
													id="policyTemplateName"
													name="name"
													className={`form-control ${errors.name && touched.name ? "is-invalid" : ""}`}
													disabled={readOnly}
												/>
												{errors.name && touched.name ? <div className="invalid-feedback">{String(errors.name)}</div> : null}
											</div>
											<div className="col-md-3">
												<label className="form-label" htmlFor="policyTemplateCategory">
													分类
												</label>
												<Field
													as="select"
													id="policyTemplateCategory"
													name="category"
													className="form-select"
													disabled={readOnly}
												>
													{categoryOptions.map((item) => (
														<option key={item.value} value={item.value}>
															{item.label}
														</option>
													))}
												</Field>
											</div>
											<div className="col-md-3">
												<div className="form-label">状态</div>
												<div className="form-check form-switch mt-2">
													<Field
														id="policyTemplateEnabled"
														name="enabled"
														type="checkbox"
														className="form-check-input"
														disabled={readOnly}
													/>
													<label className="form-check-label" htmlFor="policyTemplateEnabled">
														启用
													</label>
												</div>
											</div>
										</div>
										<div>
											<label className="form-label" htmlFor="policyTemplateDescription">
												描述
											</label>
											<Field
												as="textarea"
												id="policyTemplateDescription"
												name="description"
												className="form-control"
												rows={2}
												disabled={readOnly}
											/>
										</div>
										<div>
											<div className="d-flex align-items-center justify-content-between mb-2">
												<div className="form-label m-0">Header 规则</div>
												{readOnly ? null : (
													<FieldArray name="headers">
														{({ push }) => (
															<Button type="button" size="sm" onClick={() => push(newHeaderRule())}>
																<IconPlus size={16} />
																添加 Header
															</Button>
														)}
													</FieldArray>
												)}
											</div>
											<FieldArray name="headers">
												{({ remove }) => (
													<div className="vstack gap-2">
														{(values.headers || []).map((rule: any, index: number) => (
															<div className="row g-2 align-items-end" key={index}>
																<div className="col-md-2">
																	<label className="form-label" htmlFor={`policyHeader${index}Action`}>
																		操作
																	</label>
																	<Field
																		as="select"
																		id={`policyHeader${index}Action`}
																		name={`headers.${index}.action`}
																		className="form-select"
																		disabled={readOnly}
																	>
																		<option value="set">设置</option>
																		<option value="delete">删除</option>
																	</Field>
																</div>
																<div className="col-md-4">
																	<label className="form-label" htmlFor={`policyHeader${index}Name`}>
																		Header
																	</label>
																	<Field
																		id={`policyHeader${index}Name`}
																		name={`headers.${index}.name`}
																		className="form-control"
																		placeholder="Header-Name"
																		disabled={readOnly}
																	/>
																</div>
																<div className="col-md-5">
																	<label className="form-label" htmlFor={`policyHeader${index}Value`}>
																		值
																	</label>
																	<Field
																		id={`policyHeader${index}Value`}
																		name={`headers.${index}.value`}
																		className="form-control"
																		placeholder={rule.action === "delete" ? "" : "value"}
																		disabled={readOnly || rule.action === "delete"}
																	/>
																</div>
																<div className="col-md-1">
																	{readOnly ? null : (
																		<Button type="button" className="btn-icon" onClick={() => remove(index)}>
																			<IconTrash size={16} />
																		</Button>
																	)}
																</div>
															</div>
														))}
														{errors.headers ? <div className="text-danger small">{String(errors.headers)}</div> : null}
													</div>
												)}
											</FieldArray>
										</div>
									</div>
								) : null}
							</Modal.Body>
							<Modal.Footer>
								<Button actionType="secondary" onClick={handleClose}>
									<T id="cancel" />
								</Button>
								{readOnly ? null : (
									<Button type="submit" disabled={isSubmitting || mutation.isPending}>
										<T id="save" />
									</Button>
								)}
							</Modal.Footer>
						</Modal>
					</Form>
				)}
			</Formik>
			<ConfirmDiscardModal show={showConfirm} onConfirm={handleConfirm} onCancel={handleCancel} />
		</>
	);
});

export { HeaderPolicyTemplateModal, showHeaderPolicyTemplateModal };
