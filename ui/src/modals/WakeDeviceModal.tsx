import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { Button, Loading } from "src/components";
import { useSetWakeDevice, useWakeDevice } from "src/hooks";
import { intl, T } from "src/locale";
import { validateString } from "src/modules/Validations";
import { showObjectSuccess } from "src/notifications";

const showWakeDeviceModal = (id: number | "new") => {
	EasyModal.show(WakeDeviceModal, { id });
};

interface Props extends InnerModalProps {
	id: number | "new";
}

const validateMacAddress = (required = false) => {
	return (value?: string): string | undefined => {
		const text = String(value || "").trim();
		if (!text) {
			return required ? intl.formatMessage({ id: "error.required" }) : undefined;
		}
		const compact = text.replace(/[:.\-\s]/g, "");
		if (!/^[0-9a-fA-F]{12}$/.test(compact)) {
			return intl.formatMessage({ id: "wake-on-lan.error.mac" });
		}
	};
};

const validateOptionalIPv4 = (value?: string): string | undefined => {
	const text = String(value || "").trim();
	if (!text) return undefined;
	const parts = text.split(".");
	if (parts.length !== 4 || parts.some((part) => !/^\d+$/.test(part) || Number(part) > 255)) {
		return intl.formatMessage({ id: "wake-on-lan.error.broadcast" });
	}
};

const validateOptionalPort = (value?: string | number): string | undefined => {
	if (value === "" || typeof value === "undefined" || value === null) return undefined;
	const port = Number(value);
	if (!Number.isInteger(port) || port < 1 || port > 65535) {
		return intl.formatMessage({ id: "wake-on-lan.error.port" });
	}
};

const WakeDeviceModal = EasyModal.create(({ id, visible, remove }: Props) => {
	const { data, isLoading, error } = useWakeDevice(id);
	const { mutate: setWakeDevice } = useSetWakeDevice();
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
			macAddress: String(values.macAddress || "").trim(),
			broadcastAddress: String(values.broadcastAddress || "").trim() || "255.255.255.255",
			port: values.port ? Number(values.port) : 9,
			secureOn: String(values.secureOn || "").trim(),
			host: String(values.host || "").trim(),
			description: String(values.description || "").trim(),
			meta: values.meta || {},
		};

		setWakeDevice(payload, {
			onError: (err: any) => setErrorMsg(<T id={err.message} />),
			onSuccess: () => {
				showObjectSuccess("wake-on-lan.device", "saved");
				remove();
			},
			onSettled: () => {
				setIsSubmitting(false);
				setSubmitting(false);
			},
		});
	};

	return (
		<Modal show={visible} onHide={remove} backdrop="static" keyboard={false} size="lg">
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
							macAddress: data.macAddress || "",
							broadcastAddress: data.broadcastAddress || "255.255.255.255",
							port: data.port || 9,
							secureOn: data.secureOn || "",
							host: data.host || "",
							description: data.description || "",
							enabled: data.enabled ?? true,
							meta: data.meta || {},
						} as any
					}
					onSubmit={onSubmit}
				>
					<Form>
						<Modal.Header closeButton>
							<Modal.Title>
								<T
									id={data?.id ? "object.edit" : "object.add"}
									tData={{ object: "wake-on-lan.device" }}
								/>
							</Modal.Title>
						</Modal.Header>
						<Modal.Body>
							<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
								{errorMsg}
							</Alert>

							<h3 className="mb-3">
								<T id="wake-on-lan.section.basic" />
							</h3>
							<Field name="name" validate={validateString(1, 255)}>
								{({ field, form }: any) => (
									<div className="mb-3">
										<label className="form-label required" htmlFor="wakeDeviceName">
											<T id="name" />
										</label>
										<input
											id="wakeDeviceName"
											type="text"
											className={`form-control ${form.errors.name && form.touched.name ? "is-invalid" : ""}`}
											required
											{...field}
										/>
										{form.errors.name && form.touched.name ? (
											<div className="invalid-feedback">{form.errors.name}</div>
										) : null}
									</div>
								)}
							</Field>
							<Field name="macAddress" validate={validateMacAddress(true)}>
								{({ field, form }: any) => (
									<div className="mb-3">
										<label className="form-label required" htmlFor="wakeDeviceMac">
											<T id="wake-on-lan.mac-address" />
										</label>
										<input
											id="wakeDeviceMac"
											type="text"
											className={`form-control ${form.errors.macAddress && form.touched.macAddress ? "is-invalid" : ""}`}
											required
											placeholder="00:11:22:33:44:55"
											{...field}
										/>
										{form.errors.macAddress && form.touched.macAddress ? (
											<div className="invalid-feedback">{form.errors.macAddress}</div>
										) : null}
									</div>
								)}
							</Field>

							<h3 className="mb-3 mt-4">
								<T id="wake-on-lan.section.packet" />
							</h3>
							<div className="row">
								<div className="col-md-8">
									<Field name="broadcastAddress" validate={validateOptionalIPv4}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="wakeDeviceBroadcast">
													<T id="wake-on-lan.broadcast-address" />
												</label>
												<input
													id="wakeDeviceBroadcast"
													type="text"
													className={`form-control ${form.errors.broadcastAddress && form.touched.broadcastAddress ? "is-invalid" : ""}`}
													placeholder="255.255.255.255"
													{...field}
												/>
												{form.errors.broadcastAddress && form.touched.broadcastAddress ? (
													<div className="invalid-feedback">
														{form.errors.broadcastAddress}
													</div>
												) : (
													<small className="text-muted">
														<T id="wake-on-lan.broadcast-address.help" />
													</small>
												)}
											</div>
										)}
									</Field>
								</div>
								<div className="col-md-4">
									<Field name="port" validate={validateOptionalPort}>
										{({ field, form }: any) => (
											<div className="mb-3">
												<label className="form-label" htmlFor="wakeDevicePort">
													<T id="wake-on-lan.port" />
												</label>
												<input
													id="wakeDevicePort"
													type="number"
													min={1}
													max={65535}
													className={`form-control ${form.errors.port && form.touched.port ? "is-invalid" : ""}`}
													placeholder="9"
													{...field}
												/>
												{form.errors.port && form.touched.port ? (
													<div className="invalid-feedback">{form.errors.port}</div>
												) : null}
											</div>
										)}
									</Field>
								</div>
							</div>
							<Field name="secureOn" validate={validateMacAddress(false)}>
								{({ field, form }: any) => (
									<div className="mb-3">
										<label className="form-label" htmlFor="wakeDeviceSecureOn">
											<T id="wake-on-lan.secure-on" />
										</label>
										<input
											id="wakeDeviceSecureOn"
											type="password"
											className={`form-control ${form.errors.secureOn && form.touched.secureOn ? "is-invalid" : ""}`}
											autoComplete="off"
											placeholder="66:77:88:99:AA:BB"
											{...field}
										/>
										{form.errors.secureOn && form.touched.secureOn ? (
											<div className="invalid-feedback">{form.errors.secureOn}</div>
										) : (
											<small className="text-muted">
												<T id="wake-on-lan.secure-on.help" />
											</small>
										)}
									</div>
								)}
							</Field>

							<h3 className="mb-3 mt-4">
								<T id="wake-on-lan.section.extra" />
							</h3>
							<Field name="host">
								{({ field }: any) => (
									<div className="mb-3">
										<label className="form-label" htmlFor="wakeDeviceHost">
											<T id="wake-on-lan.host" />
										</label>
										<input id="wakeDeviceHost" type="text" className="form-control" {...field} />
									</div>
								)}
							</Field>
							<Field name="description">
								{({ field }: any) => (
									<div className="mb-3">
										<label className="form-label" htmlFor="wakeDeviceDescription">
											<T id="description" />
										</label>
										<textarea
											id="wakeDeviceDescription"
											className="form-control"
											rows={2}
											{...field}
										/>
									</div>
								)}
							</Field>
							<label className="row" htmlFor="wakeDeviceEnabled">
								<span className="col">
									<T id="enabled" />
								</span>
								<span className="col-auto">
									<Field name="enabled" type="checkbox">
										{({ field }: any) => (
											<label className="form-check form-check-single form-switch">
												<input
													id="wakeDeviceEnabled"
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
						</Modal.Body>
						<Modal.Footer>
							<Button data-bs-dismiss="modal" onClick={remove} disabled={isSubmitting}>
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
				</Formik>
			)}
		</Modal>
	);
});

export { showWakeDeviceModal };
