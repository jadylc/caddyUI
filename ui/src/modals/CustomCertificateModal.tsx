import { IconAlertTriangle } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { type Certificate, createCertificate, uploadCertificate, validateCertificate } from "src/api/backend";
import { Button } from "src/components";
import { intl, T } from "src/locale";
import { validateString } from "src/modules/Validations";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { showObjectSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

const showCustomCertificateModal = () => {
	EasyModal.show(CustomCertificateModal);
};

const CustomCertificateModal = EasyModal.create(({ visible, remove }: InnerModalProps) => {
	const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
	const queryClient = useQueryClient();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);

	const onSubmit = async (values: any, { setSubmitting }: any) => {
		if (isSubmitting) return;
		setIsSubmitting(true);
		setErrorMsg(null);

		try {
			const { niceName, provider, certificate, certificateKey, intermediateCertificate } = values;
			const formData = new FormData();

			formData.append("certificate", certificate);
			formData.append("certificate_key", certificateKey);
			if (intermediateCertificate !== null) {
				formData.append("intermediate_certificate", intermediateCertificate);
			}

			// Validate
			await validateCertificate(formData);

			// Create certificate, as other without anything else
			const cert = await createCertificate({ niceName, provider } as Certificate);

			// Upload the certificates to the created certificate
			await uploadCertificate(cert.id, formData);

			// Success
			showObjectSuccess("certificate", "saved");
			remove();
		} catch (err: any) {
			setErrorMsg(<T id={err.message} />);
		}

		queryClient.invalidateQueries({ queryKey: ["certificates"] });
		setIsSubmitting(false);
		setSubmitting(false);
	};

	return (
		<><Modal show={visible} onHide={handleClose} backdrop="static" keyboard size="lg">
			<Formik
				initialValues={
					{
						niceName: "",
						provider: "other",
						certificate: null,
						certificateKey: null,
						intermediateCertificate: null,
					} as any
				}
				onSubmit={onSubmit}
			>
				{() => (
					<Form>
						<DirtySync dirtyRef={dirtyRef} />
						<Modal.Header closeButton>
							<Modal.Title>
								<T id="object.add" tData={{ object: "certificates.custom" }} />
							</Modal.Title>
						</Modal.Header>
						<Modal.Body className="p-0">
							<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
								{errorMsg}
							</Alert>
							<div className="card m-0 border-0">
								<div className="card-body">
									<p className="text-warning">
										<IconAlertTriangle size={16} className="me-1" />
										<T id="certificates.custom.warning" />
									</p>
									<Field name="niceName" validate={validateString(1, 255)}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label htmlFor="niceName" className="form-label">
													<T id="column.name" />
												</label>
												<input
													id="niceName"
													type="text"
													required
													autoComplete="off"
													className={`form-control ${form.errors.niceName && form.touched.niceName ? "is-invalid" : ""}`}
													{...field}
												/>
												{form.errors.niceName && form.touched.niceName && (
													<div className="invalid-feedback">
														{form.errors.niceName}
													</div>
												)}
											</div>
										)}
									</Field>
									<Field name="certificateKey" validate={(v: any) => !v ? intl.formatMessage({ id: "error.required" }) : undefined}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label htmlFor="certificateKey" className="form-label">
													<T id="certificate.custom-certificate-key" />
												</label>
												<input
													id="certificateKey"
													type="file"
													required
													autoComplete="off"
													className={`form-control ${form.errors.certificateKey && form.touched.certificateKey ? "is-invalid" : ""}`}
													onChange={(event) => {
														form.setFieldValue(
															field.name,
															event.currentTarget.files?.length
																? event.currentTarget.files[0]
																: null,
														);
													}}
												/>
												{form.errors.certificateKey && form.touched.certificateKey && (
													<div className="invalid-feedback">
														{form.errors.certificateKey}
													</div>
												)}
											</div>
										)}
									</Field>
									<Field name="certificate" validate={(v: any) => !v ? intl.formatMessage({ id: "error.required" }) : undefined}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label htmlFor="certificate" className="form-label">
													<T id="certificate.custom-certificate" />
												</label>
												<input
													id="certificate"
													type="file"
													required
													autoComplete="off"
													className={`form-control ${form.errors.certificate && form.touched.certificate ? "is-invalid" : ""}`}
													onChange={(event) => {
														form.setFieldValue(
															field.name,
															event.currentTarget.files?.length
																? event.currentTarget.files[0]
																: null,
														);
													}}
												/>
												{form.errors.certificate && form.touched.certificate && (
													<div className="invalid-feedback">
														{form.errors.certificate}
													</div>
												)}
											</div>
										)}
									</Field>
									<Field name="intermediateCertificate">
										{({ field, form }: any) => (
											<div className="mb-3">
												<label htmlFor="intermediateCertificate" className="form-label">
													<T id="certificate.custom-intermediate" />
												</label>
												<input
													id="intermediateCertificate"
													type="file"
													autoComplete="off"
													className="form-control"
													onChange={(event) => {
														form.setFieldValue(
															field.name,
															event.currentTarget.files?.length
																? event.currentTarget.files[0]
																: null,
														);
													}}
												/>
												{form.errors.intermediateCertificate ? (
													<div className="invalid-feedback">
														{form.errors.intermediateCertificate &&
														form.touched.intermediateCertificate
															? form.errors.intermediateCertificate
															: null}
													</div>
												) : null}
											</div>
										)}
									</Field>
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
								className="ms-auto bg-pink"
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
		</Modal>
		<ConfirmDiscardModal show={showConfirm} onConfirm={handleConfirm} onCancel={handleCancel} />
	</>);
});

export { showCustomCertificateModal };
