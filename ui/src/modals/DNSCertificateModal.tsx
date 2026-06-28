import { useQueryClient } from "@tanstack/react-query";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { type Certificate, createCertificate } from "src/api/backend";
import { Button, CertificateAuthorityFields, DNSProviderFields, DomainNamesField } from "src/components";
import { useConfirmClose } from "src/hooks/useConfirmClose";
import { T } from "src/locale";
import { showSuccess } from "src/notifications";
import { ConfirmDiscardModal, DirtySync } from "./ConfirmDiscardModal";

type DNSCertificateModalProps = InnerModalProps & {
	initialData?: any;
	onCreated?: (certificate: Certificate) => void;
};

const showDNSCertificateModal = (initialData?: any, onCreated?: (certificate: Certificate) => void) => {
	EasyModal.show(DNSCertificateModal, { initialData, onCreated });
};

const DNSCertificateModal = EasyModal.create(
	({ visible, remove, initialData, onCreated }: DNSCertificateModalProps) => {
		const { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef } = useConfirmClose(remove);
		const queryClient = useQueryClient();
		const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
		const [isSubmitting, setIsSubmitting] = useState(false);
		const isEdit = !!initialData;
		const issuerProvider =
			initialData?.meta?.issuerProvider ?? initialData?.meta?.issuer_provider ?? initialData?.provider ?? "auto";
		const initialMeta = initialData?.meta ?? {};
		const eabKeyId = initialMeta.eabKeyId ?? initialMeta.eab_key_id ?? "";
		const eabMacKey = initialMeta.eabMacKey ?? initialMeta.eab_mac_key ?? "";

		const onSubmit = async (values: any, { setSubmitting }: any) => {
			if (isSubmitting) return;
			setIsSubmitting(true);
			setErrorMsg(null);

			try {
				const cert = await createCertificate(values);
				onCreated?.(cert);
				showSuccess(<T id="certificates.request.submitted" />);
				remove();
			} catch (err: any) {
				setErrorMsg(<T id={err.message} />);
			}
			queryClient.invalidateQueries({ queryKey: ["credentials"] });
			queryClient.invalidateQueries({ queryKey: ["dns-providers"] });
			queryClient.invalidateQueries({ queryKey: ["certificates"], exact: false });
			queryClient.refetchQueries({ queryKey: ["certificates"], type: "active" });
			queryClient.invalidateQueries({ queryKey: ["runtime-logs"] });
			setIsSubmitting(false);
			setSubmitting(false);
		};

		return (
			<><Modal show={visible} onHide={handleClose} backdrop="static" keyboard size="lg">
				<Formik
					initialValues={
						{
							domainNames: initialData?.domainNames ?? [],
							provider: issuerProvider,
							meta: {
								...initialMeta,
								issuerProvider,
								eabKeyId,
								eabMacKey,
								dnsChallenge: true,
								keyType: initialMeta.keyType ?? "ecdsa",
							},
						} as any
					}
					onSubmit={onSubmit}
				>
					{() => (
						<Form>
							<DirtySync dirtyRef={dirtyRef} />
							<Modal.Header closeButton>
								<Modal.Title>
									{isEdit ? (
										"编辑 DNS 证书申请"
									) : (
										<T id="object.add" tData={{ object: "lets-encrypt-via-dns" }} />
									)}
								</Modal.Title>
							</Modal.Header>
							<Modal.Body className="p-0">
								<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
									{errorMsg}
								</Alert>
								<div className="card m-0 border-0">
									<div className="card-body">
										<DomainNamesField isWildcardPermitted dnsProviderWildcardSupported />
										<CertificateAuthorityFields />
										<Field name="meta.keyType">
											{({ field }: any) => (
												<div className="mb-3">
													<label htmlFor="keyType" className="form-label">
														<T id="certificates.key-type" />
													</label>
													<select id="keyType" className="form-select" {...field}>
														<option value="rsa">
															<T id="certificates.key-type-rsa" />
														</option>
														<option value="ecdsa">
															<T id="certificates.key-type-ecdsa" />
														</option>
													</select>
													<small className="form-text text-muted">
														<T id="certificates.key-type-description" />
													</small>
												</div>
											)}
										</Field>
										{isEdit &&
										(initialData?.meta?.credentialId || initialData?.meta?.credential_id) ? (
											<Alert variant="info" className="mb-3">
												当前会复用原 DNS 凭据；如需更换，再选择 DNS 提供商并填写新凭据。
											</Alert>
										) : null}
										<DNSProviderFields
											optional={
												isEdit &&
												!!(initialData?.meta?.credentialId || initialData?.meta?.credential_id)
											}
										/>
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
	},
);

export { showDNSCertificateModal };
